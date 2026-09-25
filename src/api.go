package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/nicklaw5/helix/v2"
	log "github.com/sirupsen/logrus"
)

// StreamStatusResponse is the shape expected by the frontend sort.js
type StreamStatusResponse struct {
	Online   bool     `json:"online"`
	Game     string   `json:"game"`
	Language string   `json:"language"`
	Tags     []string `json:"tags"`
}

func (app *App) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*") // Adjust for production CORS
	w.Header().Set("Content-Type", "application/json")

	streamers, err := app.store.GetStreamers()
	if err != nil {
		http.Error(w, "failed to get streamers", http.StatusInternalServerError)
		return
	}

	response := make(map[string]StreamStatusResponse)
	for _, s := range streamers {
		response[strings.ToLower(s.Username)] = StreamStatusResponse{
			Online:   s.IsOnline,
			Game:     s.Game,
			Language: s.Language,
			Tags:     s.Tags,
		}
	}

	json.NewEncoder(w).Encode(response)
}

func (app *App) handleAddStreamer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate Admin Token
	token := r.Header.Get("Authorization")
	expectedToken := "Bearer " + os.Getenv("SS_ADMIN_TOKEN")
	if token != expectedToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	username := req.Username
	if username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	// 1. Add to database
	if err := app.store.AddStreamer(username); err != nil {
		http.Error(w, "failed to save streamer", http.StatusInternalServerError)
		return
	}

	// 2. Fetch UserID from Twitch
	usersResp, err := app.client.GetUsers(&helix.UsersParams{Logins: []string{username}})
	if err != nil || len(usersResp.Data.Users) == 0 {
		log.Errorf("Failed to fetch user %s from twitch: %v", username, err)
		http.Error(w, "failed to find user on twitch", http.StatusNotFound)
		return
	}
	userID := usersResp.Data.Users[0].ID

	// 3. Subscribe to EventSub Webhooks
	callbackURL := "https://streamstatus.wupinyin.co.uk/webhook/callbacks"
	secret := os.Getenv("SS_SECRETKEY")

	events := []string{"stream.online", "stream.offline", "channel.update"}
	for _, eventType := range events {
		resp, err := app.client.CreateEventSubSubscription(&helix.EventSubSubscription{
			Type:    eventType,
			Version: "1",
			Condition: helix.EventSubCondition{
				BroadcasterUserID: userID,
			},
			Transport: helix.EventSubTransport{
				Method:   "webhook",
				Callback: callbackURL,
				Secret:   secret,
			},
		})
		if err != nil || resp.StatusCode >= 400 {
			log.Errorf("Failed to subscribe to %s for %s: %v %v", eventType, username, err, resp.ErrorMessage)
		}
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "Streamer %s added and webhooks subscribed successfully\n", username)
}
