package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// MockStore is a mock implementation of the Store interface for testing
type MockStore struct {
	streamers []Streamer
	err       error
}

func (m *MockStore) GetStreamers() ([]Streamer, error) {
	return m.streamers, m.err
}

func (m *MockStore) AddStreamer(username string) error {
	return nil
}

func (m *MockStore) RemoveStreamer(username string) error {
	return nil
}

func (m *MockStore) UpdateStatus(username string, isOnline bool, game string, language string, tags []string) error {
	return nil
}

func (m *MockStore) PruneInactiveStreamers() error {
	return nil
}

func TestHandleGetStatus(t *testing.T) {
	mockStore := &MockStore{
		streamers: []Streamer{
			{Username: "teststreamer", IsOnline: true, Game: "Hacking", Language: "en", Tags: []string{"cyber"}},
		},
	}

	app := &App{store: mockStore}

	req, err := http.NewRequest("GET", "/api/status", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(app.handleGetStatus)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response map[string]StreamStatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if val, ok := response["teststreamer"]; !ok {
		t.Errorf("expected teststreamer in response")
	} else if !val.Online {
		t.Errorf("expected teststreamer to be online")
	}
}

func TestHandleGetStreamersList(t *testing.T) {
	mockStore := &MockStore{
		streamers: []Streamer{
			{Username: "online_user", IsOnline: true},
			{Username: "offline_user", IsOnline: false},
		},
	}

	app := &App{store: mockStore}

	// Test GET /api/streamers
	req, err := http.NewRequest("GET", "/api/streamers", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(app.handleGetStreamersList)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var streamers []Streamer
	if err := json.NewDecoder(rr.Body).Decode(&streamers); err != nil {
		t.Fatal(err)
	}

	if len(streamers) != 2 {
		t.Errorf("expected 2 streamers, got %d", len(streamers))
	}

	// Test GET /api/streamers?status=online
	reqOnline, _ := http.NewRequest("GET", "/api/streamers?status=online", nil)
	rrOnline := httptest.NewRecorder()
	handler.ServeHTTP(rrOnline, reqOnline)

	var onlineStreamers []Streamer
	json.NewDecoder(rrOnline.Body).Decode(&onlineStreamers)
	if len(onlineStreamers) != 1 || onlineStreamers[0].Username != "online_user" {
		t.Errorf("expected 1 online streamer (online_user), got %v", onlineStreamers)
	}
}
