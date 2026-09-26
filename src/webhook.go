package main

import (
	"bytes"
	// "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nicklaw5/helix/v2"
	log "github.com/sirupsen/logrus"
)

var VALID_GAMES = []string{
	"just chatting",
	"science & technology",
	"software and game development",
	"talk shows & podcasts",
	"information security",
}

type eventSubNotification struct {
	Challenge    string                     `json:"challenge"`
	Event        json.RawMessage            `json:"event"`
	Subscription helix.EventSubSubscription `json:"subscription"`
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, val) {
			return true
		}
	}
	return false
}

func (app *App) handleWebhook(w http.ResponseWriter, r *http.Request) {
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
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&vals); err != nil {
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
		json.NewDecoder(bytes.NewReader(vals.Event)).Decode(&offlineEvent)
		if r.Header.Get("Twitch-Eventsub-Message-Retry") != "0" {
			return
		}
		
		log.Printf("offline event for: %s", offlineEvent.BroadcasterUserName)
		err := app.store.UpdateStatus(offlineEvent.BroadcasterUserName, false, "", "", []string{})
		if err != nil {
			log.Errorf("failed to update status: %v", err)
		}

	case "stream.online":
		var onlineEvent helix.EventSubStreamOnlineEvent
		json.NewDecoder(bytes.NewReader(vals.Event)).Decode(&onlineEvent)
		if r.Header.Get("Twitch-Eventsub-Message-Retry") != "0" {
			return
		}

		log.Printf("online event for: %s", onlineEvent.BroadcasterUserName)
		stream, err := app.fetchStreamInfo(onlineEvent.BroadcasterUserID)
		if err != nil {
			log.Errorf("failed to fetch stream info: %v", err)
			return
		}

		isOnline := contains(VALID_GAMES, stream.GameName)
		err = app.store.UpdateStatus(onlineEvent.BroadcasterUserName, isOnline, stream.GameName, strings.ToUpper(stream.Language), stream.Tags)
		if err != nil {
			log.Errorf("failed to update status: %v", err)
		}

	case "channel.update":
		var updateEvent helix.EventSubChannelUpdateEvent
		json.NewDecoder(bytes.NewReader(vals.Event)).Decode(&updateEvent)
		
		log.Printf("channel update for: %s", updateEvent.BroadcasterUserName)
		stream, err := app.fetchStreamInfo(updateEvent.BroadcasterUserID)
		if err != nil {
			log.Errorf("failed to fetch stream info: %v", err)
			return
		}

		isOnline := contains(VALID_GAMES, stream.GameName)
		err = app.store.UpdateStatus(updateEvent.BroadcasterUserName, isOnline, stream.GameName, strings.ToUpper(stream.Language), stream.Tags)
		if err != nil {
			log.Errorf("failed to update status: %v", err)
		}
	}
}

func (app *App) fetchStreamInfo(userID string) (helix.Stream, error) {
	for i := 1; i <= 5; i++ {
		streams, err := app.client.GetStreams(&helix.StreamsParams{UserIDs: []string{userID}})
		if err == nil && streams.ErrorStatus == 0 {
			if len(streams.Data.Streams) > 0 {
				return streams.Data.Streams[0], nil
			}
			// Twitch API can take a few seconds to populate stream info after webhook
		} else if i == 5 {
			return helix.Stream{}, err
		}
		
		if i == 5 {
			break
		}
		time.Sleep(3 * time.Second)
	}
	return helix.Stream{}, fmt.Errorf("retries exhausted, no stream returned")
}
