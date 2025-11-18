package config

import (
	"os"
)

type Config struct {
	RedisURL     string
	RedisChannel string
	DBServiceURL string
	ServiceName  string
	LogLevel     string
}

func Load() *Config {
	return &Config{
		RedisURL:     getEnv("REDIS_URL", "redis://localhost:6379"),
		RedisChannel: getEnv("REDIS_CHANNEL", "groups_events"),
		DBServiceURL: getEnv("DB_SERVICE_URL", "http://db-service:8000"),
		ServiceName:  getEnv("SERVICE_NAME", "group-service"),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
