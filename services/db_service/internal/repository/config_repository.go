package repository

import (
	"context"
	"database/sql"
	"errors"
)

// Config represents a row in the config table
type Config struct {
	Name  string
	Value string
}

// ConfigRepository defines CRUD operations for the config table
type ConfigRepository struct {
	db *sql.DB
}

// NewConfigRepository creates a new ConfigRepository
func NewConfigRepository(db *sql.DB) *ConfigRepository {
	return &ConfigRepository{db: db}
}

// Create inserts a new config record
func (r *ConfigRepository) Create(ctx context.Context, config Config) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO config (name, value) VALUES (?, ?)", config.Name, config.Value)
	return err
}

// GetByName retrieves a config record by name
func (r *ConfigRepository) GetByName(ctx context.Context, name string) (Config, error) {
	var config Config
	row := r.db.QueryRowContext(ctx, "SELECT name, value FROM config WHERE name = ?", name)
	err := row.Scan(&config.Name, &config.Value)
	if errors.Is(err, sql.ErrNoRows) {
		return Config{}, nil
	}
	return config, err
}

// List returns all config records
func (r *ConfigRepository) List(ctx context.Context) ([]Config, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT name, value FROM config")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []Config
	for rows.Next() {
		var config Config
		if err := rows.Scan(&config.Name, &config.Value); err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, rows.Err()
}

// Update modifies the value of an existing config record
func (r *ConfigRepository) Update(ctx context.Context, config Config) error {
	_, err := r.db.ExecContext(ctx, "UPDATE config SET value = ? WHERE name = ?", config.Value, config.Name)
	return err
}

// Delete removes a config record by name
func (r *ConfigRepository) Delete(ctx context.Context, name string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM config WHERE name = ?", name)
	return err
}
