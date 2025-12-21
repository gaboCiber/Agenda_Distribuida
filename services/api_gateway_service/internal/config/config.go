package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server struct {
		Host         string
		Port         string
		ReadTimeout  time.Duration
		WriteTimeout time.Duration
		IdleTimeout  time.Duration
	}
	Redis struct {
		URL string
	}
	JWT struct {
		Secret     string
		Expiration time.Duration
	}
	DBService struct {
		URL string
	}
	// RaftNodesURLs contiene las URLs de todos los nodos del cluster Raft
	RaftNodesURLs []string
	// UserNodesURLs contiene las URLs de todos los nodos de user service
	UserNodesURLs []string
	// GroupNodesURLs contiene las URLs de todos los nodos de group service
	GroupNodesURLs []string
	LogLevel       string
}

func Load() *Config {
	cfg := &Config{}

	// Server configuration
	cfg.Server.Host = getEnv("SERVER_HOST", "0.0.0.0")
	cfg.Server.Port = getEnv("SERVER_PORT", "8080")
	cfg.Server.ReadTimeout = getEnvAsDuration("SERVER_READ_TIMEOUT", "10s")
	cfg.Server.WriteTimeout = getEnvAsDuration("SERVER_WRITE_TIMEOUT", "10s")
	cfg.Server.IdleTimeout = getEnvAsDuration("SERVER_IDLE_TIMEOUT", "60s")

	// Redis configuration
	cfg.Redis.URL = getEnv("REDIS_URL", "redis://localhost:6379")

	// JWT configuration
	cfg.JWT.Secret = getEnv("JWT_SECRET", "your-secret-key")
	cfg.JWT.Expiration = getEnvAsDuration("JWT_EXPIRATION", "24h")

	// DB Service configuration
	cfg.DBService.URL = getEnv("DB_SERVICE_URL", "http://agenda-db-service:8000")

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
	cfg.RaftNodesURLs = raftNodesURLs

	// Obtener URLs de nodos User Service
	userNodesStr := getEnv("USER_NODES_URLS", "agenda-user-service-1:8007,agenda-user-service-2:8007,agenda-user-service-3:8007")
	var userNodesURLs []string
	if userNodesStr != "" {
		userNodesURLs = strings.Split(userNodesStr, ",")
		// Limpiar espacios en blanco
		for i, url := range userNodesURLs {
			userNodesURLs[i] = strings.TrimSpace(url)
		}
	}
	cfg.UserNodesURLs = userNodesURLs

	// Obtener URLs de nodos Group Service
	groupNodesStr := getEnv("GROUP_NODES_URLS", "agenda-group-service-1:8008,agenda-group-service-2:8008,agenda-group-service-3:8008")
	var groupNodesURLs []string
	if groupNodesStr != "" {
		groupNodesURLs = strings.Split(groupNodesStr, ",")
		// Limpiar espacios en blanco
		for i, url := range groupNodesURLs {
			groupNodesURLs[i] = strings.TrimSpace(url)
		}
	}
	cfg.GroupNodesURLs = groupNodesURLs

	// Logging
	cfg.LogLevel = getEnv("LOG_LEVEL", "info")

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsDuration(key, defaultValue string) time.Duration {
	val := getEnv(key, defaultValue)
	duration, err := time.ParseDuration(val)
	if err != nil {
		return time.Duration(0)
	}
	return duration
}

func getEnvAsInt(key string, defaultValue int) int {
	val := getEnv(key, strconv.Itoa(defaultValue))
	intVal, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return intVal
}
