package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Database wraps the SQL database connection
type Database struct {
	*sql.DB
}

// NewSQLiteDB creates a new SQLite database connection
func NewSQLiteDB(dbPath string) (*Database, error) {
	// Create the directory if it doesn't exist
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %v", err)
	}

	// Open the database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %v", err)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	return &Database{db}, nil
}

// runMigrations runs database migrations
func runMigrations(db *sql.DB) error {
	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	// Ensure we rollback in case of error
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Create tables if they don't exist
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			is_hierarchical BOOLEAN DEFAULT FALSE,
			parent_group_id TEXT REFERENCES groups(id) ON DELETE CASCADE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS group_members (
			group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (group_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS group_events (
			id TEXT PRIMARY KEY,
			group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			event_id TEXT NOT NULL,
			added_by TEXT NOT NULL,
			added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(group_id, event_id)
		)`,
		`CREATE TABLE IF NOT EXISTS group_invitations (
			id TEXT PRIMARY KEY,
			group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL,
			invited_by TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(group_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS group_event_status (
			id TEXT PRIMARY KEY,
			group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			event_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected')),
			responded_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(event_id, user_id),
			FOREIGN KEY (group_id, event_id) REFERENCES group_events(group_id, event_id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_group_event_status_event ON group_event_status(event_id)`,
		`CREATE INDEX IF NOT EXISTS idx_group_event_status_group ON group_event_status(group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_group_event_status_group_event ON group_event_status(group_id, event_id)`,
	}

	// Execute migrations
	for i, migration := range migrations {
		if _, err := tx.Exec(migration); err != nil {
			return fmt.Errorf("failed to execute migration #%d: %v\nSQL: %s", i+1, err, migration)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migrations: %v", err)
	}

	// Verify tables were created
	tables := []string{"groups", "group_members", "group_events", "group_invitations", "group_event_status"}
	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		if err != nil {
			log.Printf("failed to check if table %s exists: %v", table, err)
			continue
		}
		if count == 0 {
			log.Printf("table %s was not created successfully", table)
			continue
		}
	}

	return nil
}
