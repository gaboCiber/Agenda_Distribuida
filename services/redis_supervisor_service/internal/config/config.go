package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the supervisor
type Config struct {
	RedisAddrs      []string
	DBServiceURL    string
	RaftNodesURLs   []string
	PingInterval    time.Duration
	FailureThreshold int
	SupervisorID    string
	SupervisorBindAddr string
	SupervisorPeers []PeerConfig
	HTTPPort        int
}

// PeerConfig represents another redis-supervisor instance available for leader election.
type PeerConfig struct {
	ID      string
	Address string
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

	supervisorID := strings.TrimSpace(getEnv("SUPERVISOR_ID", ""))
	if supervisorID == "" {
		return nil, fmt.Errorf("SUPERVISOR_ID is required")
	}

	supervisorBindAddr := strings.TrimSpace(getEnv("SUPERVISOR_BIND_ADDR", ":6000"))
	if supervisorBindAddr == "" {
		return nil, fmt.Errorf("SUPERVISOR_BIND_ADDR must not be empty")
	}

	peersStr := strings.TrimSpace(getEnv("SUPERVISOR_PEERS", ""))
	supervisorPeers, err := parseSupervisorPeers(peersStr)
	if err != nil {
		return nil, err
	}

	httpPort, err := strconv.Atoi(getEnv("HTTP_PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP_PORT: %w", err)
	}

	return &Config{
		RedisAddrs:       redisAddrs,
		DBServiceURL:     dbServiceURL,
		RaftNodesURLs:    raftNodesURLs,
		PingInterval:     time.Duration(pingIntervalSeconds) * time.Second,
		FailureThreshold: failureThreshold,
		SupervisorID:     supervisorID,
		SupervisorBindAddr: supervisorBindAddr,
		SupervisorPeers:  supervisorPeers,
		HTTPPort:         httpPort,
	}, nil
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func parseSupervisorPeers(peers string) ([]PeerConfig, error) {
	if peers == "" {
		return []PeerConfig{}, nil
	}

	entries := strings.Split(peers, ",")
	result := make([]PeerConfig, 0, len(entries))

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.Split(entry, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid peer format: %s (expected id=address)", entry)
		}

		id := strings.TrimSpace(parts[0])
		addr := strings.TrimSpace(parts[1])

		if id == "" || addr == "" {
			return nil, fmt.Errorf("invalid peer format: %s (id and address must not be empty)", entry)
		}

		result = append(result, PeerConfig{ID: id, Address: addr})
	}

	return result, nil
}
