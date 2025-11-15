package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

// DB returns the underlying *sql.DB instance
func (d *Database) DB() *sql.DB {
	return d.db
}

func New(path string) (*Database, error) {
	// Create the directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %v", err)
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %v", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite works best with a single connection
	dbInstance := &Database{db: db}

	// Run migrations
	if err := dbInstance.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %v", err)
	}

	return dbInstance, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Begin starts a new transaction
func (d *Database) Begin() (*sql.Tx, error) {
	return d.db.Begin()
}

// Exec executes a query without returning any rows
func (d *Database) Exec(query string, args ...interface{}) (sql.Result, error) {
	return d.db.Exec(query, args...)
}

// Query executes a query that returns rows
func (d *Database) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return d.db.Query(query, args...)
}

// QueryRow executes a query that is expected to return at most one row
func (d *Database) QueryRow(query string, args ...interface{}) *sql.Row {
	return d.db.QueryRow(query, args...)
}

// migrate runs the database migrations
func (d *Database) migrate() error {
	// Check if migrations table exists
	var tableExists int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='_migrations'`,
	).Scan(&tableExists)

	if err != nil {
		return fmt.Errorf("failed to check migrations table: %v", err)
	}

	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Create migrations table if it doesn't exist
	if tableExists == 0 {
		if _, err := tx.Exec(`
			CREATE TABLE _migrations (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL UNIQUE,
				run_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
		`); err != nil {
			return fmt.Errorf("failed to create migrations table: %v", err)
		}
	}

	// Run migrations in order
	for _, migration := range getMigrations() {
		// Check if migration already ran
		var count int
		err := tx.QueryRow(
			`SELECT COUNT(*) FROM _migrations WHERE name = ?`,
			migration.name,
		).Scan(&count)

		if err != nil {
			return fmt.Errorf("failed to check migration status: %v", err)
		}

		if count == 0 {
			// Run migration
			if _, err := tx.Exec(migration.statement); err != nil {
				return fmt.Errorf("failed to run migration %s: %v", migration.name, err)
			}

			// Record migration
			if _, err := tx.Exec(
				`INSERT INTO _migrations (name) VALUES (?)`,
				migration.name,
			); err != nil {
				return fmt.Errorf("failed to record migration %s: %v", migration.name, err)
			}
		}
	}

	return tx.Commit()
}

type migration struct {
	name      string
	statement string
}

func getMigrations() []migration {
	return []migration{
		{
			name: "initial_schema",
			statement: `
				-- Users table
				CREATE TABLE IF NOT EXISTS users (
					id TEXT PRIMARY KEY,
					username TEXT NOT NULL UNIQUE,
					email TEXT NOT NULL UNIQUE,
					hashed_password TEXT NOT NULL,
					is_active BOOLEAN DEFAULT 1,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				);

				CREATE TRIGGER IF NOT EXISTS update_users_timestamp
				AFTER UPDATE ON users
				BEGIN
					UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
				END;

				-- Groups table
				CREATE TABLE IF NOT EXISTS groups (
					id TEXT PRIMARY KEY,
					name TEXT NOT NULL,
					description TEXT,
					created_by TEXT NOT NULL,
					is_hierarchical BOOLEAN DEFAULT 0,
					parent_group_id TEXT,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (created_by) REFERENCES users(id),
					FOREIGN KEY (parent_group_id) REFERENCES groups(id)
				);

				CREATE TRIGGER IF NOT EXISTS update_groups_timestamp
				AFTER UPDATE ON groups
				BEGIN
					UPDATE groups SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
				END;

				-- Group members
				CREATE TABLE IF NOT EXISTS group_members (
					id TEXT PRIMARY KEY,
					group_id TEXT NOT NULL,
					user_id INTEGER NOT NULL,
					role TEXT NOT NULL DEFAULT 'member',
					is_inherited BOOLEAN DEFAULT 0,
					joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
					FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
					UNIQUE(group_id, user_id)
				);

				-- Events
				CREATE TABLE IF NOT EXISTS events (
					id TEXT PRIMARY KEY,
					title TEXT NOT NULL,
					description TEXT,
					start_time TIMESTAMP NOT NULL,
					end_time TIMESTAMP NOT NULL,
					user_id INTEGER NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
				);

				CREATE TRIGGER IF NOT EXISTS update_events_timestamp
				AFTER UPDATE ON events
				BEGIN
					UPDATE events SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
				END;

				-- Group events
				CREATE TABLE IF NOT EXISTS group_events (
					id TEXT PRIMARY KEY,
					group_id TEXT NOT NULL,
					event_id TEXT NOT NULL,
					added_by INTEGER NOT NULL,
					is_hierarchical BOOLEAN DEFAULT 0,
					status TEXT DEFAULT 'pending',
					added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
					FOREIGN KEY (event_id) REFERENCES events(id) ON DELETE CASCADE,
					FOREIGN KEY (added_by) REFERENCES users(id) ON DELETE CASCADE,
					UNIQUE(group_id, event_id)
				);

				-- Group invitations
				CREATE TABLE IF NOT EXISTS group_invitations (
					id TEXT PRIMARY KEY,
					group_id TEXT NOT NULL,
					user_id INTEGER NOT NULL,
					invited_by INTEGER NOT NULL,
					status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected')),
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					responded_at TIMESTAMP,
					FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
					FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
					FOREIGN KEY (invited_by) REFERENCES users(id) ON DELETE CASCADE,
					UNIQUE(group_id, user_id)
				);

				-- Event statuses
				CREATE TABLE IF NOT EXISTS group_event_status (
					id TEXT PRIMARY KEY,
					event_id TEXT NOT NULL,
					group_id TEXT NOT NULL,
					user_id INTEGER NOT NULL,
					status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled')),
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					responded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (event_id) REFERENCES events(id) ON DELETE CASCADE,
					FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
					FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
					UNIQUE(event_id, group_id, user_id)
				);

				CREATE TRIGGER IF NOT EXISTS update_group_event_status_timestamp
				AFTER UPDATE ON group_event_status
				BEGIN
					UPDATE group_event_status SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
				END;
			`,
		},
		// Add more migrations here as needed
	}
}
