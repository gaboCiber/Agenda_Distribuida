package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

type DBClient struct {
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

func NewDBClient(baseURL string, logger *zap.Logger) *DBClient {
	// Asegurarse de que la URL base no termine con /
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &DBClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// RaftNodeInfo represents information about a Raft node
type RaftNodeInfo struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Leader string `json:"leader"`
}

// FindAndUpdateLeader busca el líder actualizando el baseURL
func (c *DBClient) FindAndUpdateLeader(ctx context.Context, raftNodes []string) error {
	// Intentar con las URLs de nodos Raft proporcionadas
	for _, nodeURL := range raftNodes {
		// Limpiar URL
		nodeURL = strings.TrimSuffix(strings.TrimSpace(nodeURL), "/")
		
		// Hacer ping al nodo para ver si es líder
		if c.isNodeLeader(ctx, nodeURL) {
			c.baseURL = nodeURL
			c.logger.Info("Líder actualizado", zap.String("new_leader", nodeURL))
			return nil
		}
	}
	
	return fmt.Errorf("no se encontró ningún líder en los nodos: %v", raftNodes)
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

func (c *DBClient) GetEvents(userID string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/events?user_id=%s", c.baseURL, userID)

	resp, err := c.client.Get(url)
	if err != nil {
		c.logger.Error("Failed to get events from DB service", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("DB service returned error", zap.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("DB service error: %d", resp.StatusCode)
	}

	var result struct {
		Events []map[string]interface{} `json:"events"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.logger.Error("Failed to decode events response", zap.Error(err))
		return nil, err
	}

	return result.Events, nil
}

func (c *DBClient) GetGroups(userID string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/groups?user_id=%s", c.baseURL, userID)

	resp, err := c.client.Get(url)
	if err != nil {
		c.logger.Error("Failed to get groups from DB service", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("DB service returned error", zap.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("DB service error: %d", resp.StatusCode)
	}

	var result struct {
		Groups []map[string]interface{} `json:"groups"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.logger.Error("Failed to decode groups response", zap.Error(err))
		return nil, err
	}

	return result.Groups, nil
}

func (c *DBClient) CreateEvent(event map[string]interface{}) error {
	url := fmt.Sprintf("%s/api/v1/events", c.baseURL)

	jsonData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	resp, err := c.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Error("Failed to create event in DB service", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		c.logger.Error("DB service returned error for event creation", zap.Int("status", resp.StatusCode))
		return fmt.Errorf("DB service error: %d", resp.StatusCode)
	}

	return nil
}

// RegisterUser registra un nuevo usuario directamente en la base de datos
func (c *DBClient) RegisterUser(username, email, hashedPassword string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/users", c.baseURL)

	userData := map[string]interface{}{
		"username":        username,
		"email":           email,
		"hashed_password": hashedPassword,
	}

	jsonData, err := json.Marshal(userData)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Error("Failed to register user in DB service", zap.Error(err))
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		c.logger.Error("DB service returned error for user registration", zap.Int("status", resp.StatusCode))
		return "", fmt.Errorf("DB service error: %d", resp.StatusCode)
	}

	var result struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.logger.Error("Failed to decode user registration response", zap.Error(err))
		return "", err
	}

	return result.UserID, nil
}

// LoginUser verifica las credenciales del usuario en la base de datos
func (c *DBClient) LoginUser(email, password string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/users/login", c.baseURL)

	loginData := map[string]string{
		"email":    email,
		"password": password,
	}

	jsonData, err := json.Marshal(loginData)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		c.logger.Error("Failed to login user in DB service", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("DB service returned error for user login", zap.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("Invalid credentials")
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.logger.Error("Failed to decode user login response", zap.Error(err))
		return nil, err
	}

	return result, nil
}
