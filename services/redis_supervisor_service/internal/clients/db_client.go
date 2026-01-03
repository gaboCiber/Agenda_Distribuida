package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RaftNodeInfo represents information about a Raft node
type RaftNodeInfo struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Leader string `json:"leader"`
}

// ServiceInfo represents a registered service in the registry
type ServiceInfo struct {
	ServiceName string `json:"service_name"`
	Address     string `json:"address"`
}

// DBClient handles communication with the DB service and registry
type DBClient struct {
	baseURL       string
	client        *http.Client
	raftNodesURLs []string
	serviceName   string
	serviceURL    string
	heartbeatStop chan struct{}
	heartbeatWG   sync.WaitGroup
}

// NewDBClient creates a new DB client with registry support
func NewDBClient(baseURL string, raftNodesURLs []string, serviceName, serviceURL string) *DBClient {
	return &DBClient{
		baseURL:       strings.TrimSuffix(baseURL, "/"),
		client:        &http.Client{Timeout: 5 * time.Second},
		raftNodesURLs: raftNodesURLs,
		serviceName:   serviceName,
		serviceURL:    strings.TrimSuffix(serviceURL, "/"),
		heartbeatStop: make(chan struct{}),
	}
}

// FindAndUpdateLeader busca el líder actualizando el baseURL
func (c *DBClient) FindAndUpdateLeader(ctx context.Context) error {
	// Intentar con las URLs de nodos Raft proporcionadas
	for _, nodeURL := range c.raftNodesURLs {
		// Limpiar URL
		nodeURL = strings.TrimSuffix(strings.TrimSpace(nodeURL), "/")

		// Hacer ping al nodo para ver si es líder
		if c.isNodeLeader(ctx, nodeURL) {
			c.baseURL = nodeURL
			log.Printf("Líder Raft actualizado: %s", nodeURL)
			return nil
		}
	}

	return fmt.Errorf("no se encontró ningún líder en los nodos: %v", c.raftNodesURLs)
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

// configPayload defines the structure for our JSON requests.
type configPayload struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// GetConfig retrieves a specific configuration value by name.
func (c *DBClient) GetConfig(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Primero encontrar al líder actual
	if err := c.FindAndUpdateLeader(ctx); err != nil {
		return "", fmt.Errorf("failed to find Raft leader: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/configs/%s", c.baseURL, name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create GET request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send GET request to db service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil // Not found is not an error, it just means no value is set
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("db service returned non-ok status for GET: %d", resp.StatusCode)
	}

	var payload configPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("failed to decode response body: %w", err)
	}

	return payload.Value, nil
}

// RegisterService registers the current service with the registry
func (c *DBClient) RegisterService(metadata string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First find the current leader
	if err := c.FindAndUpdateLeader(ctx); err != nil {
		return fmt.Errorf("failed to find Raft leader: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/services/%s", c.baseURL, c.serviceName)
	service := map[string]string{
		"address": c.serviceURL,
	}

	jsonData, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal service data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create registration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to register service, status: %d, response: %s", resp.StatusCode, string(body))
	}

	// Start heartbeat in the background
	c.startHeartbeat()

	return nil
}

// DeregisterService removes the service from the registry
func (c *DBClient) DeregisterService() error {
	// Stop the heartbeat
	if c.heartbeatStop != nil {
		close(c.heartbeatStop)
		c.heartbeatWG.Wait()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/services/%s", c.baseURL, c.serviceName)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create deregistration request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to deregister service: %w", err)
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
					log.Printf("Failed to update Raft leader: %v", err)
					continue
				}

				// Send the heartbeat
				if err := c.sendHeartbeat(); err != nil {
					log.Printf("Failed to send heartbeat, attempting to re-register: %v", err)
					
					// Try to re-register the service
					if err := c.RegisterService(""); err != nil {
						log.Printf("Failed to re-register service: %v", err)
					}
				} else {
					log.Printf("Successfully sent heartbeat to %s", c.baseURL)
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

// DiscoverService finds a registered service by name
func (c *DBClient) DiscoverService(serviceName string) (*ServiceInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/services/%s", c.baseURL, serviceName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to discover service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("service not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to discover service, status: %d", resp.StatusCode)
	}

	var result struct {
		ServiceName string `json:"service_name"`
		Address     string `json:"address"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode service info: %w", err)
	}

	service := &ServiceInfo{
		ServiceName: result.ServiceName,
		Address:     result.Address,
	}

	return service, nil
}

// ListServices returns all registered services
func (c *DBClient) ListServices() ([]ServiceInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First find the current leader
	if err := c.FindAndUpdateLeader(ctx); err != nil {
		return nil, fmt.Errorf("failed to find Raft leader: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/services", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list services request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list services, status: %d", resp.StatusCode)
	}

	// The response is a map of service names to addresses
	var servicesMap map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&servicesMap); err != nil {
		return nil, fmt.Errorf("failed to decode services list: %w", err)
	}

	// Convert the map to a slice of ServiceInfo
	var services []ServiceInfo
	for name, addr := range servicesMap {
		services = append(services, ServiceInfo{
			ServiceName: name,
			Address:     addr,
		})
	}

	return services, nil
}

// SetRedisPrimary sets the Redis primary address in the central configuration.
// It performs an "upsert" logic: it tries to get the value first, then updates it (PUT)
// if it exists, or creates it (POST) if it doesn't.
func (c *DBClient) SetRedisPrimary(primaryAddr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Primero encontrar al líder actual
	if err := c.FindAndUpdateLeader(ctx); err != nil {
		return fmt.Errorf("failed to find Raft leader: %w", err)
	}

	configName := "redis_primary"

	// Check if the config already exists
	existingValue, err := c.GetConfig(configName)
	if err != nil {
		return fmt.Errorf("failed to check for existing config: %w", err)
	}

	var method, url string
	var jsonPayload []byte

	if existingValue != "" {
		// Value exists, so we update it with PUT (only value in payload)
		method = "PUT"
		url = fmt.Sprintf("%s/api/v1/configs/%s", c.baseURL, configName)
		payload := map[string]string{"value": primaryAddr}
		jsonPayload, err = json.Marshal(payload)
	} else {
		// Value does not exist, so we create it with POST (full payload)
		method = "POST"
		url = fmt.Sprintf("%s/api/v1/configs", c.baseURL)
		payload := configPayload{Name: configName, Value: primaryAddr}
		jsonPayload, err = json.Marshal(payload)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal json payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send %s request to db service: %w", method, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("db service returned non-ok status for %s: %d", method, resp.StatusCode)
	}

	return nil
}
