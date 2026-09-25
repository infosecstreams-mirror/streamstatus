package main

// Streamer represents a Twitch streamer and their current status
type Streamer struct {
	Username string `json:"username"`
	IsOnline bool   `json:"is_online"`
	Game     string `json:"game"`
	Language string `json:"language"`
	Tags     []string `json:"tags"`
}

// Store defines the data backend interface for managing streamers and their status.
// This allows us to easily swap between a Postgres database, a local JSON/CSV file, or memory.
type Store interface {
	// GetStreamers returns all tracked streamers
	GetStreamers() ([]Streamer, error)
	
	// AddStreamer adds a new streamer to the backend
	AddStreamer(username string) error
	
	// RemoveStreamer removes a streamer from the backend
	RemoveStreamer(username string) error
	
	// UpdateStatus updates the live status and metadata of a streamer
	UpdateStatus(username string, isOnline bool, game string, language string, tags []string) error
	
	// PruneInactiveStreamers removes streamers that have exceeded the inactivity threshold
	PruneInactiveStreamers() error
}
