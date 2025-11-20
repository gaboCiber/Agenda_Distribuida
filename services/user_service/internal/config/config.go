package config

import (
	"os"
	"strconv"
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
		RedisChannel: getEnv("REDIS_CHANNEL", "users_events"),
		DBServiceURL: getEnv("DB_SERVICE_URL", "http://db-service:8000"),
		ServiceName:  getEnv("SERVICE_NAME", "user-service"),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
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
