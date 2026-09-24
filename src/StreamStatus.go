package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nicklaw5/helix/v2"
	"github.com/nikoksr/notify"
	log "github.com/sirupsen/logrus"
)

var VALID_GAMES = []string{
	"just chatting",
	"science & technology",
	"software and game development",
	"talk shows & podcasts",
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, val) {
			return true
		}
	}
	return false
}

type StreamerState struct {
	Online   bool     `json:"online"`
	Game     string   `json:"game"`
	Language string   `json:"language"`
	Tags     []string `json:"tags"`
}

type StreamersRepo struct {
	GitHubToken        string
	Owner              string
	Repo               string
	StatusFileSHA      string
	StatusMap          map[string]StreamerState
	notificationClient *notify.Notify
	client             *helix.Client
	mutex              *sync.Mutex
}

// GitHub API response for fetching file content
type ghFileContent struct {
	Sha     string `json:"sha"`
	Content string `json:"content"`
}

// fetchRemoteStatus fetches the current status.json from the data branch via the GitHub API
func (s *StreamersRepo) fetchRemoteStatus() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/status.json?ref=data", s.Owner, s.Repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.GitHubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("status.json not found on data branch")
	} else if resp.StatusCode != 200 {
		return fmt.Errorf("github api returned %d", resp.StatusCode)
	}

	var content ghFileContent
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return err
	}

	s.StatusFileSHA = content.Sha
	decoded, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return err
	}

	var parsedMap map[string]StreamerState
	if err := json.Unmarshal(decoded, &parsedMap); err != nil {
		return err
	}

	s.StatusMap = parsedMap
	return nil
}

// pushRemoteStatus pushes the current StatusMap to the data branch via the GitHub API
func (s *StreamersRepo) pushRemoteStatus() error {
	jsonData, err := json.MarshalIndent(s.StatusMap, "", "  ")
	if err != nil {
		return err
	}
	encodedContent := base64.StdEncoding.EncodeToString(jsonData)

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/status.json", s.Owner, s.Repo)
	
	payload := map[string]interface{}{
		"message": "🤖 STATUSS: update streaming status",
		"content": encodedContent,
		"branch":  "data",
	}
	if s.StatusFileSHA != "" {
		payload["sha"] = s.StatusFileSHA
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.GitHubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github api put returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var putResp struct {
		Content struct {
			Sha string `json:"sha"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&putResp); err == nil {
		s.StatusFileSHA = putResp.Content.Sha
	}

	log.Println("Successfully updated status.json on GitHub.")
	return nil
}

type eventSubNotification struct {
	Challenge    string                     `json:"challenge"`
	Event        json.RawMessage            `json:"event"`
	Subscription helix.EventSubSubscription `json:"subscription"`
}

func (s *StreamersRepo) eventsubStatus(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		return
	}
	defer r.Body.Close()

	if !helix.VerifyEventSubNotification(os.Getenv("SS_SECRETKEY"), r.Header, string(body)) {
		log.Println("invalid signature on message")
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("I am, unfortunately for you, a teapot.\n"))
		return
	}

	var vals eventSubNotification
	err = json.NewDecoder(bytes.NewReader(body)).Decode(&vals)
	if err != nil {
		log.Println(err)
		return
	}

	if vals.Challenge != "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(vals.Challenge))
		return
	}
	w.Write([]byte("OK"))

	switch vals.Subscription.Type {
	case "stream.offline":
		var offlineEvent helix.EventSubStreamOfflineEvent
		_ = json.NewDecoder(bytes.NewReader(vals.Event)).Decode(&offlineEvent)
		if r.Header.Get("Twitch-Eventsub-Message-Retry") != "0" {
			log.Warnf("ignoring duplicate event from Twitch for %s (%s)", offlineEvent.BroadcasterUserName, offlineEvent.BroadcasterUserID)
			return
		}
		log.Printf("got offline event for: %s (%s)", offlineEvent.BroadcasterUserName, offlineEvent.BroadcasterUserID)

		s.mutex.Lock()
		state := s.StatusMap[offlineEvent.BroadcasterUserName]
		state.Online = false
		s.StatusMap[offlineEvent.BroadcasterUserName] = state
		
		err = s.pushRemoteStatus()
		s.mutex.Unlock()

		if err != nil {
			log.Errorf("error pushing offline status: %s", err)
			s.notifyError(err)
		}

	case "stream.online":
		var onlineEvent helix.EventSubStreamOnlineEvent
		_ = json.NewDecoder(bytes.NewReader(vals.Event)).Decode(&onlineEvent)
		if r.Header.Get("Twitch-Eventsub-Message-Retry") != "0" {
			log.Warnf("ignoring duplicate event from Twitch for %s (%s)", onlineEvent.BroadcasterUserName, onlineEvent.BroadcasterUserID)
			return
		}
		log.Printf("got online event for: %s (%s)", onlineEvent.BroadcasterUserName, onlineEvent.BroadcasterUserID)

		var stream helix.Stream
		for i := 1; i <= 3; i++ {
			err = nil
			stream, err = s.fetchStreamInfo(onlineEvent.BroadcasterUserID)
			if err != nil {
				log.Errorf("Error fetching stream info for %s: %s", onlineEvent.BroadcasterUserName, err)
				if i == 3 {
					s.notifyError(err)
					return
				}
			} else {
				break
			}
		}

		s.mutex.Lock()
		state := StreamerState{
			Online:   contains(VALID_GAMES, stream.GameName),
			Game:     stream.GameName,
			Language: strings.ToUpper(stream.Language),
			Tags:     stream.Tags,
		}
		s.StatusMap[onlineEvent.BroadcasterUserName] = state
		err = s.pushRemoteStatus()
		s.mutex.Unlock()

		if err != nil {
			log.Errorf("error pushing online status: %s", err)
			s.notifyError(err)
		}

	case "channel.update":
		var updateEvent helix.EventSubChannelUpdateEvent
		_ = json.NewDecoder(bytes.NewReader(vals.Event)).Decode(&updateEvent)
		log.Printf("got channel update event for: %s (%s)", updateEvent.BroadcasterUserName, updateEvent.BroadcasterUserID)

		s.mutex.Lock()
		state := s.StatusMap[updateEvent.BroadcasterUserName]
		state.Online = contains(VALID_GAMES, updateEvent.CategoryName)
		state.Game = updateEvent.CategoryName
		state.Language = strings.ToUpper(updateEvent.Language)
		s.StatusMap[updateEvent.BroadcasterUserName] = state
		
		err = s.pushRemoteStatus()
		s.mutex.Unlock()

		if err != nil {
			log.Errorf("error pushing update status: %s", err)
			s.notifyError(err)
		}

	default:
		log.Errorf("error: event type %s has not been implemented", r.Header.Get("Twitch-Eventsub-Subscription-Type"))
	}
}

func (s *StreamersRepo) fetchStreamInfo(user_id string) (helix.Stream, error) {
	var stream helix.Stream
	log.Infof("trying to get stream info for uid %s", user_id)
	streams, err := s.client.GetStreams(&helix.StreamsParams{UserIDs: []string{user_id}})
	if err != nil {
		return stream, err
	}
	if streams.ErrorStatus != 0 {
		return stream, fmt.Errorf("error fetching stream info status=%d", streams.ErrorStatus)
	}
	if len(streams.Data.Streams) > 0 {
		stream = streams.Data.Streams[0]
	}
	return stream, nil
}

func (s *StreamersRepo) notifyError(err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.notificationClient.Send(ctx, "StreamStatus Error", err.Error())
}
