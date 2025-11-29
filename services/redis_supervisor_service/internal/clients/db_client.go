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

// UpdateRedisPrimary tells the DB service about the new Redis primary
func (c *DBClient) UpdateRedisPrimary(primaryAddr string) error {
	// This is a placeholder. The actual endpoint and payload may differ.
	updateURL := fmt.Sprintf("%s/config/redis_primary", c.baseURL)
	
	payload := map[string]string{"address": primaryAddr}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal json payload: %w", err)
	}

	req, err := http.NewRequest("POST", updateURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to db service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("db service returned non-ok status: %d", resp.StatusCode)
	}

	return nil
}
