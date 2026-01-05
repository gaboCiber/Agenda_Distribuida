package raft

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RaftNodeInfo representa información sobre un nodo Raft
type RaftNodeInfo struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Leader string `json:"leader"`
}

// LeaderDiscovery maneja el descubrimiento del líder Raft
type LeaderDiscovery struct {
	httpClient *http.Client
	logger     *zap.Logger
	leaderURL  string
	sync.RWMutex
}

// NewLeaderDiscovery crea una nueva instancia de LeaderDiscovery
func NewLeaderDiscovery(logger *zap.Logger) *LeaderDiscovery {
	return &LeaderDiscovery{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		logger:     logger,
	}
}

// GetLeaderURL devuelve la URL del líder actual
func (ld *LeaderDiscovery) GetLeaderURL() string {
	ld.RLock()
	defer ld.RUnlock()
	return ld.leaderURL
}

// FindAndUpdateLeader encuentra y actualiza el líder del clúster Raft
func (ld *LeaderDiscovery) FindAndUpdateLeader(ctx context.Context, raftNodes []string) (string, error) {
	if len(raftNodes) == 0 {
		return "", fmt.Errorf("no se proporcionaron nodos Raft para descubrir líder")
	}

	for _, rawNodeURL := range raftNodes {
		nodeURL := strings.TrimSuffix(strings.TrimSpace(rawNodeURL), "/")
		if nodeURL == "" {
			continue
		}

		if err := ctx.Err(); err != nil {
			return "", err
		}

		nodeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		isLeader, _ := ld.isRaftLeader(nodeCtx, nodeURL)
		cancel()

		// if err != nil {
		// 	ld.logger.Debug("Error verificando líder",
		// 		zap.String("node", nodeURL),
		// 		zap.Error(err))
		// 	lastErr = err
		// 	continue
		// }

		if isLeader {
			ld.Lock()
			ld.leaderURL = nodeURL
			ld.Unlock()

			ld.logger.Info("Líder Raft actualizado",
				zap.String("leader", nodeURL))
			return nodeURL, nil
		}
	}

	return "", fmt.Errorf("no se pudo encontrar un líder Raft entre los nodos: %v", raftNodes)
}

// isRaftLeader verifica si un nodo Raft es el líder actual
func (ld *LeaderDiscovery) isRaftLeader(ctx context.Context, nodeURL string) (bool, error) {
	url := fmt.Sprintf("%s/raft/status", nodeURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("error creando request: %w", err)
	}

	resp, err := ld.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("error ejecutando request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("error leyendo respuesta: %w", err)
	}

	var nodeInfo RaftNodeInfo
	if err := json.Unmarshal(body, &nodeInfo); err != nil {
		return false, fmt.Errorf("error parseando respuesta: %w", err)
	}

	return nodeInfo.State == "Leader", nil
}
