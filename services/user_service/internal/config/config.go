package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	RedisURL     string
	RedisChannel string
	DBServiceURL string
	// RaftNodesURLs contiene las URLs de todos los nodos del cluster Raft
	RaftNodesURLs []string
	ServiceName   string
	LogLevel      string
}

func Load() *Config {
	// Obtener URLs de nodos Raft (pueden venir separadas por comas)
	raftNodesStr := getEnv("RAFT_NODES_URLS", "http://localhost:8001,http://localhost:8002,http://localhost:8003")
	var raftNodesURLs []string
	if raftNodesStr != "" {
		raftNodesURLs = strings.Split(raftNodesStr, ",")
		// Limpiar espacios en blanco
		for i, url := range raftNodesURLs {
			raftNodesURLs[i] = strings.TrimSpace(url)
		}
	}

	return &Config{
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379"),
		RedisChannel:   getEnv("REDIS_CHANNEL", "users_events"),
		DBServiceURL:   getEnv("DB_SERVICE_URL", "http://db-service:8000"),
		RaftNodesURLs:  raftNodesURLs,
		ServiceName:   getEnv("SERVICE_NAME", "user-service"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
