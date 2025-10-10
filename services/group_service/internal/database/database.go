package database

import (
	"database/sql"
	"fmt"
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
			created_by TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
	}

	for _, migration := range migrations {
		_, err := db.Exec(migration)
		if err != nil {
			return fmt.Errorf("migration failed: %v\n%s", err, migration)
		}
	}

	return nil
}
