package main

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore initializes a connection to the Postgres database and ensures the schema exists.
func NewPostgresStore(connectionString string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &PostgresStore{db: db}
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

func (s *PostgresStore) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS streamers (
		username VARCHAR(255) PRIMARY KEY,
		is_online BOOLEAN DEFAULT FALSE,
		game VARCHAR(255) DEFAULT '',
		language VARCHAR(10) DEFAULT '',
		tags TEXT DEFAULT '',
		last_seen TIMESTAMP DEFAULT NOW()
	);
	ALTER TABLE streamers ADD COLUMN IF NOT EXISTS last_seen TIMESTAMP DEFAULT NOW();`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStore) GetStreamers() ([]Streamer, error) {
	rows, err := s.db.Query("SELECT username, is_online, game, language, tags FROM streamers ORDER BY username ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streamers []Streamer
	for rows.Next() {
		var streamer Streamer
		var tagsStr string
		if err := rows.Scan(&streamer.Username, &streamer.IsOnline, &streamer.Game, &streamer.Language, &tagsStr); err != nil {
			return nil, err
		}
		if tagsStr != "" {
			streamer.Tags = strings.Split(tagsStr, ",")
		} else {
			streamer.Tags = []string{}
		}
		streamers = append(streamers, streamer)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return streamers, nil
}

func (s *PostgresStore) AddStreamer(username string) error {
	_, err := s.db.Exec("INSERT INTO streamers (username) VALUES ($1) ON CONFLICT (username) DO NOTHING", username)
	if err != nil {
		log.Errorf("Failed to add streamer %s: %s", username, err)
		return err
	}
	return nil
}

func (s *PostgresStore) RemoveStreamer(username string) error {
	_, err := s.db.Exec("DELETE FROM streamers WHERE username = $1", username)
	return err
}

func (s *PostgresStore) UpdateStatus(username string, isOnline bool, game string, language string, tags []string) error {
	tagsStr := strings.Join(tags, ",")
	query := `
		INSERT INTO streamers (username, is_online, game, language, tags, last_seen)
		VALUES ($1, $2, $3, $4, $5, CASE WHEN $2 = TRUE THEN NOW() ELSE NOW() END)
		ON CONFLICT (username) DO UPDATE 
		SET is_online = EXCLUDED.is_online,
		    game = EXCLUDED.game,
		    language = EXCLUDED.language,
		    tags = EXCLUDED.tags,
		    last_seen = CASE WHEN EXCLUDED.is_online = TRUE THEN NOW() ELSE streamers.last_seen END;
	`
	_, err := s.db.Exec(query, username, isOnline, game, language, tagsStr)
	return err
}

func (s *PostgresStore) PruneInactiveStreamers() error {
	// Remove streamers who haven't been online in over a year
	_, err := s.db.Exec("DELETE FROM streamers WHERE last_seen < NOW() - INTERVAL '1 year'")
	return err
}
