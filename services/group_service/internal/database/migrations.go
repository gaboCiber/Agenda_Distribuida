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
		// Add hierarchical support
		{
			Name: "add_hierarchical_support",
			Up: `
				-- Add new columns to groups table
				ALTER TABLE groups ADD COLUMN is_hierarchical BOOLEAN NOT NULL DEFAULT FALSE;
				ALTER TABLE groups ADD COLUMN parent_group_id TEXT REFERENCES groups(id) ON DELETE SET NULL;

				-- Add new columns to group_members table
				ALTER TABLE group_members ADD COLUMN is_inherited BOOLEAN NOT NULL DEFAULT FALSE;

				-- Add new column to group_events table
				ALTER TABLE group_events ADD COLUMN is_hierarchical BOOLEAN NOT NULL DEFAULT FALSE;

				-- Create indexes for better performance
				CREATE INDEX IF NOT EXISTS idx_groups_parent ON groups(parent_group_id);
				CREATE INDEX IF NOT EXISTS idx_group_members_inherited ON group_members(is_inherited);

				-- Create a view for hierarchical groups
				CREATE VIEW IF NOT EXISTS hierarchical_groups AS
				SELECT 
				    g1.id AS parent_group_id,
				    g1.name AS parent_group_name,
				    g2.id AS child_group_id,
				    g2.name AS child_group_name
				FROM 
				    groups g1
				JOIN 
				    groups g2 ON g2.parent_group_id = g1.id
				WHERE 
				    g1.is_hierarchical = TRUE;

				-- Create a view for inherited members
				CREATE VIEW IF NOT EXISTS inherited_members AS
				SELECT 
				    gm.*,
				    g.parent_group_id AS inherited_from_group_id
				FROM 
				    group_members gm
				JOIN 
				    groups g ON gm.group_id = g.id
				WHERE 
				    gm.is_inherited = TRUE;
			`,
		},
		// Add event status table
		{
			Name: "add_event_status_table",
			Up: `
				CREATE TABLE IF NOT EXISTS group_event_status (
					id TEXT PRIMARY KEY,
					group_id TEXT NOT NULL,
					event_id TEXT NOT NULL,
					user_id TEXT NOT NULL,
					status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected')),
					responded_at TIMESTAMP,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(event_id, user_id),
					FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
					FOREIGN KEY (group_id, event_id) REFERENCES group_events(group_id, event_id) ON DELETE CASCADE
				);

				-- Create indexes for better performance
				CREATE INDEX IF NOT EXISTS idx_group_event_status_event ON group_event_status(event_id);
				CREATE INDEX IF NOT EXISTS idx_group_event_status_group ON group_event_status(group_id);
				CREATE INDEX IF NOT EXISTS idx_group_event_status_group_event ON group_event_status(group_id, event_id);
			`,
		},
		// Add status column to group_events table
		{
			Name: "add_status_to_group_events",
			Up: `
				-- Add status column with default 'pending' and check constraint
				ALTER TABLE group_events 
				ADD COLUMN status TEXT NOT NULL DEFAULT 'pending' 
				CHECK (status IN ('pending', 'accepted', 'rejected'));

				-- Update existing records to have 'accepted' status for backward compatibility
				UPDATE group_events SET status = 'accepted';
			`,
		},
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
