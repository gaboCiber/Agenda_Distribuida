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
	Database struct {
		Path string
	}
	LogLevel string
}

func Load() *Config {
	cfg := &Config{}

	// Server configuration
	cfg.Server.Host = getEnv("SERVER_HOST", "0.0.0.0")
	cfg.Server.Port = getEnv("SERVER_PORT", "8000")
	cfg.Server.ReadTimeout = getEnvAsDuration("SERVER_READ_TIMEOUT", "10s")
	cfg.Server.WriteTimeout = getEnvAsDuration("SERVER_WRITE_TIMEOUT", "10s")
	cfg.Server.IdleTimeout = getEnvAsDuration("SERVER_IDLE_TIMEOUT", "60s")

	// Database configuration
	cfg.Database.Path = getEnv("DB_PATH", "./data/agenda_distribuida.db")

	// Logging
	cfg.LogLevel = getEnv("LOG_LEVEL", "debug")

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
