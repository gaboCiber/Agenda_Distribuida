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
	Database struct {
		Path string
	}
	Raft struct {
		ID      string
		DataDir string
		Peers   map[string]string
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

	// Raft configuration
	cfg.Raft.ID = getEnv("RAFT_ID", "node1")
	cfg.Raft.DataDir = getEnv("RAFT_DATA_DIR", "./data/raft")
	cfg.Raft.Peers = parsePeers(getEnv("RAFT_PEERS", "node1=127.0.0.1:9001,node2=127.0.0.1:9002,node3=127.0.0.1:9003"))

	// Logging
	cfg.LogLevel = getEnv("LOG_LEVEL", "debug")

	return cfg
}

func parsePeers(raw string) map[string]string {
	peers := make(map[string]string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return peers
	}
	parts := strings.Split(raw, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		id := strings.TrimSpace(kv[0])
		addr := strings.TrimSpace(kv[1])
		if id != "" && addr != "" {
			peers[id] = addr
		}
	}
	return peers
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
