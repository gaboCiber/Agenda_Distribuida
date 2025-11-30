package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the application's configuration.
type Config struct {
	RedisAddrs       []string
	DBServiceURL     string
	PingInterval     time.Duration
	FailureThreshold int
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	redisAddrsStr := getEnv("REDIS_ADDRS", "localhost:6379,localhost:6380")
	redisAddrs := strings.Split(redisAddrsStr, ",")

	dbServiceURL := getEnv("DB_SERVICE_URL", "http://localhost:8000")

	pingIntervalSeconds, err := strconv.Atoi(getEnv("PING_INTERVAL", "1"))
	if err != nil {
		return nil, fmt.Errorf("invalid PING_INTERVAL: %w", err)
	}

	failureThreshold, err := strconv.Atoi(getEnv("FAILURE_THRESHOLD", "3"))
	if err != nil {
		return nil, fmt.Errorf("invalid FAILURE_THRESHOLD: %w", err)
	}

	return &Config{
		RedisAddrs:       redisAddrs,
		DBServiceURL:     dbServiceURL,
		PingInterval:     time.Duration(pingIntervalSeconds) * time.Second,
		FailureThreshold: failureThreshold,
	}, nil
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
