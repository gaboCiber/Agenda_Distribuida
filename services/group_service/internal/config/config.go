package config

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server configuration
	ServerAddress   string
	ShutdownTimeout time.Duration
	Environment     string

	// Database configuration
	DatabasePath string

	// Redis configuration
	RedisURL string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Set default values
	cfg := &Config{
		ServerAddress:   getEnv("SERVER_ADDRESS", "0.0.0.0:8003"),
		ShutdownTimeout: 10 * time.Second,
		Environment:     getEnv("ENVIRONMENT", "development"),
		DatabasePath:    getEnv("DATABASE_PATH", "./data/groups.db"),
		RedisURL:        getEnv("REDIS_URL", "redis://agenda-bus-redis:6379/0"),
	}

	return cfg, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
