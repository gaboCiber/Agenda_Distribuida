package config

import (
	"os"
	"strings"
)

type Config struct {
	RedisURL      string
	RedisChannel  string
	DBServiceURL  string
	RaftNodesURLs []string
	ServiceName   string
	LogLevel      string
}

func Load() *Config {
	// Parse Raft nodes URLs
	raftNodesStr := getEnv("RAFT_NODES_URLS", "http://localhost:8001,http://localhost:8002,http://localhost:8003")
	raftNodesURLs := []string{}
	if raftNodesStr != "" {
		raftNodesURLs = strings.Split(raftNodesStr, ",")
		for i, url := range raftNodesURLs {
			raftNodesURLs[i] = strings.TrimSpace(url)
		}
	}

	return &Config{
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379"),
		RedisChannel:  getEnv("REDIS_CHANNEL", "groups_events"),
		DBServiceURL:  getEnv("DB_SERVICE_URL", "http://db-service:8000"),
		RaftNodesURLs: raftNodesURLs,
		ServiceName:   getEnv("SERVICE_NAME", "group-service"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
