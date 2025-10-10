package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the database configuration
type Config struct {
	Driver string
	DSN    string // Data Source Name
}

// Init initializes the database connection and runs migrations
func Init(cfg Config) (*sql.DB, error) {
	// Create the database directory if it doesn't exist
	if cfg.Driver == "sqlite3" {
		if err := os.MkdirAll(filepath.Dir(cfg.DSN), 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Open database connection
	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign key constraints for SQLite
	if cfg.Driver == "sqlite3" {
		if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
		}

		// Enable WAL mode for better concurrency
		if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
		}
	}

	// Verify the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if err := Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return db, nil
}
