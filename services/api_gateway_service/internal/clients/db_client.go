package clients

import (
	"bytes"
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

// ServiceInfo represents a registered service in the registry
type ServiceInfo struct {
	ServiceName string `json:"service_name"`
	Address     string `json:"address"`
}

type DBClient struct {
	baseURL       string
	serviceName   string
	serviceAddr   string
	client        *http.Client
	logger        *zap.Logger
	heartbeatStop chan struct{}
	heartbeatWG   sync.WaitGroup
	raftNodes     []string // List of all Raft node URLs
}

func NewDBClient(serviceName, serviceAddr, baseURL string, logger *zap.Logger) *DBClient {
	// Asegurarse de que la URL base no termine con /
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &DBClient{
		baseURL:     baseURL,
		serviceName: serviceName,
		serviceAddr: serviceAddr,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger:        logger,
		heartbeatStop: make(chan struct{}),
	}
}

// RaftNodeInfo represents information about a Raft node
type RaftNodeInfo struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Leader string `json:"leader"`
}

// SetRaftNodes sets the list of Raft node URLs for leader discovery
func (c *DBClient) SetRaftNodes(raftNodes []string) {
	c.raftNodes = raftNodes
}

// FindAndUpdateLeader busca el líder actualizando el baseURL
func (c *DBClient) FindAndUpdateLeader(ctx context.Context) error {
	if len(c.raftNodes) == 0 {
		return fmt.Errorf("no Raft nodes configured for leader discovery")
	}

	// Intentar con las URLs de nodos Raft configuradas
	for _, nodeURL := range c.raftNodes {
		// Limpiar URL
		nodeURL = strings.TrimSuffix(strings.TrimSpace(nodeURL), "/")

		// Hacer ping al nodo para ver si es líder
		if c.isNodeLeader(ctx, nodeURL) {
			c.baseURL = nodeURL
			c.logger.Info("Líder actualizado", zap.String("new_leader", nodeURL))
			return nil
		}
	}

	return fmt.Errorf("no se encontró ningún líder en los nodos: %v", c.raftNodes)
}

// isNodeLeader verifica si un nodo específico es el líder
func (c *DBClient) isNodeLeader(ctx context.Context, nodeURL string) bool {
	url := fmt.Sprintf("%s/raft/status", nodeURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var nodeInfo RaftNodeInfo
	if err := json.Unmarshal(body, &nodeInfo); err != nil {
		return false
	}

	return nodeInfo.State == "Leader"
}

// RegisterService registers this service with the registry
func (c *DBClient) RegisterService(metadata string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ensure we're talking to the leader
	if err := c.FindAndUpdateLeader(ctx); err != nil {
		return fmt.Errorf("failed to find leader: %w", err)
	}

	// Prepare the request body
	reqBody := struct {
		Address  string `json:"address"`
		Metadata string `json:"metadata,omitempty"`
	}{
		Address:  c.serviceAddr,
		Metadata: metadata,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Send the request
	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/v1/services/%s", c.baseURL, c.serviceName),
		bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to register service, status: %d", resp.StatusCode)
	}

	// Start heartbeats in the background
	c.startHeartbeat()

	return nil
}

// DeregisterService removes this service from the registry
func (c *DBClient) DeregisterService() error {
	if c.heartbeatStop != nil {
		close(c.heartbeatStop)
		c.heartbeatWG.Wait()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("%s/api/v1/services/%s", c.baseURL, c.serviceName), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to deregister service, status: %d", resp.StatusCode)
	}

	return nil
}

// startHeartbeat starts a goroutine to send periodic heartbeats
func (c *DBClient) startHeartbeat() {
	if c.heartbeatStop == nil {
		c.heartbeatStop = make(chan struct{})
	}

	c.heartbeatWG.Add(1)
	go func() {
		defer c.heartbeatWG.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Update the leader before each heartbeat
				if err := c.FindAndUpdateLeader(context.Background()); err != nil {
					c.logger.Error("Failed to update Raft leader", zap.Error(err))
					continue
				}

				// Send the heartbeat
				if err := c.sendHeartbeat(); err != nil {
					c.logger.Error("Failed to send heartbeat, attempting to re-register", zap.Error(err))

					// Try to re-register the service
					if err := c.RegisterService(""); err != nil {
						c.logger.Error("Failed to re-register service", zap.Error(err))
					}
				} else {
					c.logger.Debug("Successfully sent heartbeat",
						zap.String("service", c.serviceName),
						zap.String("leader", c.baseURL))
				}

			case <-c.heartbeatStop:
				return
			}
		}
	}()
}

// sendHeartbeat sends a single heartbeat to the service registry
func (c *DBClient) sendHeartbeat() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	heartbeatURL := fmt.Sprintf("%s/api/v1/services/%s/heartbeat", c.baseURL, c.serviceName)
	req, err := http.NewRequestWithContext(ctx, "POST", heartbeatURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create heartbeat request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed with status: %d", resp.StatusCode)
	}

	return nil
}

// DiscoverService finds a service by name
func (c *DBClient) DiscoverService(serviceName string) (*ServiceInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ensure we're talking to the leader
	if err := c.FindAndUpdateLeader(ctx); err != nil {
		return nil, fmt.Errorf("failed to find leader: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/v1/services/%s", c.baseURL, serviceName), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to discover service, status: %d", resp.StatusCode)
	}

	// The response is a map of service name to address
	var serviceMap map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&serviceMap); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Find our service in the map
	address, exists := serviceMap[serviceName]
	if !exists {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	return &ServiceInfo{
		ServiceName: serviceName,
		Address:     address,
	}, nil
}

// ListServices returns all registered services
func (c *DBClient) ListServices() ([]ServiceInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ensure we're talking to the leader
	if err := c.FindAndUpdateLeader(ctx); err != nil {
		return nil, fmt.Errorf("failed to find leader: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/v1/services", c.baseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list services, status: %d", resp.StatusCode)
	}

	// The response is a map of service name to address
	var serviceMap map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&serviceMap); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert the map to a slice of ServiceInfo
	services := make([]ServiceInfo, 0, len(serviceMap))
	for name, addr := range serviceMap {
		services = append(services, ServiceInfo{
			ServiceName: name,
			Address:     addr,
		})
	}

	return services, nil
}

// GetRedisPrimary obtiene la dirección del Redis primary desde el DB service
func (c *DBClient) GetRedisPrimary(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/api/v1/configs/redis_primary", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error getting redis primary config: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var config struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	if err := json.Unmarshal(body, &config); err != nil {
		return "", err
	}

	return config.Value, nil
}
