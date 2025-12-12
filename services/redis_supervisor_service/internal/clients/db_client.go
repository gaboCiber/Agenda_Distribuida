package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// RaftNodeInfo represents information about a Raft node
type RaftNodeInfo struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Leader string `json:"leader"`
}

// DBClient handles communication with the DB service
type DBClient struct {
	baseURL       string
	client        *http.Client
	raftNodesURLs []string
}

// NewDBClient creates a new DB client
func NewDBClient(baseURL string, raftNodesURLs []string) *DBClient {
	return &DBClient{
		baseURL:       strings.TrimSuffix(baseURL, "/"),
		client:        &http.Client{Timeout: 5 * time.Second},
		raftNodesURLs: raftNodesURLs,
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

	payload := configPayload{Name: configName, Value: primaryAddr}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal json payload: %w", err)
	}

	var method, url string
	if existingValue != "" {
		// Value exists, so we update it
		method = "PUT"
		url = fmt.Sprintf("%s/api/v1/configs/%s", c.baseURL, configName)
	} else {
		// Value does not exist, so we create it
		method = "POST"
		url = fmt.Sprintf("%s/api/v1/configs", c.baseURL)
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