package config

import (
	"os"
	"strconv"
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
	LogLevel string
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
