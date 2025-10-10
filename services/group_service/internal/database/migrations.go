package database

import "database/sql"

// Migration represents a database migration
type Migration struct {
	Name string
	Up   string
}

// Migrate runs all database migrations
func Migrate(db *sql.DB) error {
	// Check if migrations table exists
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS _migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			run_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		return err
	}

	// Define all migrations in order
	migrations := []Migration{
		{
			Name: "initial_schema",
			Up: `
				CREATE TABLE IF NOT EXISTS groups (
					id TEXT PRIMARY KEY,
					name TEXT NOT NULL,
					description TEXT,
					created_by TEXT NOT NULL,
					created_at TIMESTAMP NOT NULL,
					updated_at TIMESTAMP NOT NULL
				);

				CREATE TABLE IF NOT EXISTS group_members (
					id TEXT PRIMARY KEY,
					group_id TEXT NOT NULL,
					user_id TEXT NOT NULL,
					role TEXT NOT NULL DEFAULT 'member',
					joined_at TIMESTAMP NOT NULL,
					FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE,
					UNIQUE(group_id, user_id)
				);

				CREATE TABLE IF NOT EXISTS group_events (
					id TEXT PRIMARY KEY,
					group_id TEXT NOT NULL,
					event_id TEXT NOT NULL,
					added_by TEXT NOT NULL,
					added_at TIMESTAMP NOT NULL,
					FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE,
					UNIQUE(group_id, event_id)
				);

				CREATE TABLE IF NOT EXISTS group_invitations (
					id TEXT PRIMARY KEY,
					group_id TEXT NOT NULL,
					user_id TEXT NOT NULL,
					invited_by TEXT NOT NULL,
					status TEXT NOT NULL DEFAULT 'pending',
					created_at TIMESTAMP NOT NULL,
					responded_at TIMESTAMP,
					FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE,
					UNIQUE(group_id, user_id, status)
				);

				-- Create indexes for better performance
				CREATE INDEX IF NOT EXISTS idx_group_members_group ON group_members(group_id);
				CREATE INDEX IF NOT EXISTS idx_group_members_user ON group_members(user_id);
				CREATE INDEX IF NOT EXISTS idx_group_events_group ON group_events(group_id);
				CREATE INDEX IF NOT EXISTS idx_group_invitations_user ON group_invitations(user_id);
				CREATE INDEX IF NOT EXISTS idx_group_invitations_group ON group_invitations(group_id);
			`,
		},
		// Add future migrations here:
		// {
		// 	Name: "add_new_feature",
		// 	Up: `
		// 		ALTER TABLE groups ADD COLUMN new_column TEXT;
		// 	`,
		// },
	}

	// Run each migration in a transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	for _, migration := range migrations {
		// Check if migration was already run
		var count int
		err := tx.QueryRow(
			"SELECT COUNT(*) FROM _migrations WHERE name = ?",
			migration.Name,
		).Scan(&count)

		if err != nil {
			tx.Rollback()
			return err
		}

		// Skip if migration was already applied
		if count > 0 {
			continue
		}

		// Run migration
		if _, err := tx.Exec(migration.Up); err != nil {
			tx.Rollback()
			return err
		}

		// Record migration
		if _, err := tx.Exec(
			"INSERT INTO _migrations (name) VALUES (?)",
			migration.Name,
		); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}
