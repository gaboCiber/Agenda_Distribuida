package config

import (
	"os"
	"strconv"
)

type Config struct {
	RedisHost string
	RedisPort string
	RedisDB   int
	Port      string
	DBPath    string
}

func Load() *Config {
	return &Config{
		RedisHost: getEnv("REDIS_HOST", "agenda-bus-redis"),
		RedisPort: getEnv("REDIS_PORT", "6379"),
		RedisDB:   getEnvAsInt("REDIS_DB", 0),
		Port:      getEnv("PORT", "8002"),
		DBPath:    getEnv("DB_PATH", "/app/events.db"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
