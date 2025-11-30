package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// DBClient handles communication with the DB service
type DBClient struct {
	baseURL string
}

// NewDBClient creates a new DB client
func NewDBClient(baseURL string) *DBClient {
	return &DBClient{baseURL: baseURL}
}

// configPayload defines the structure for our JSON requests.
type configPayload struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// GetConfig retrieves a specific configuration value by name.
func (c *DBClient) GetConfig(name string) (string, error) {
	url := fmt.Sprintf("%s/config/%s", c.baseURL, name)
	resp, err := http.Get(url)
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
		url = fmt.Sprintf("%s/config/%s", c.baseURL, configName)
	} else {
		// Value does not exist, so we create it
		method = "POST"
		url = fmt.Sprintf("%s/config", c.baseURL)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send %s request to db service: %w", method, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("db service returned non-ok status for %s: %d", method, resp.StatusCode)
	}

	return nil
}