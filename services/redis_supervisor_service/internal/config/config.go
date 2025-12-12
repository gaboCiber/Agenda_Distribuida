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
	RedisAddrs      []string
	DBServiceURL    string
	RaftNodesURLs   []string
	PingInterval    time.Duration
	FailureThreshold int
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	redisAddrsStr := getEnv("REDIS_ADDRS", "localhost:6379,localhost:6380,localhost:6381")
	redisAddrs := strings.Split(redisAddrsStr, ",")

	dbServiceURL := getEnv("DB_SERVICE_URL", "http://localhost:8000")

	// Obtener URLs de nodos Raft
	raftNodesStr := getEnv("RAFT_NODES_URLS", "http://localhost:8001,http://localhost:8002,http://localhost:8003")
	var raftNodesURLs []string
	if raftNodesStr != "" {
		raftNodesURLs = strings.Split(raftNodesStr, ",")
		for i, url := range raftNodesURLs {
			raftNodesURLs[i] = strings.TrimSpace(url)
		}
	}

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
		RaftNodesURLs:    raftNodesURLs,
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
