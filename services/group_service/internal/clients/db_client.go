package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type DBServiceClient struct {
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

type apiResponse struct {
	Group  *models.Group `json:"group"`
	Status string        `json:"status"`
}

// doRequest es una función auxiliar para realizar peticiones HTTP
func (c *DBServiceClient) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}

	if err != nil {
		c.logger.Error("Error al crear la solicitud", zap.Error(err))
		return nil, fmt.Errorf("error al crear la solicitud: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("Error al enviar la solicitud", zap.Error(err))
		return nil, fmt.Errorf("error al enviar la solicitud: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("Error al leer la respuesta", zap.Error(err))
		return nil, fmt.Errorf("error al leer la respuesta: %w", err)
	}

	if resp.StatusCode >= 400 {
		c.logger.Error("Error en la respuesta del servidor",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(respBody)))
		return nil, fmt.Errorf("error en la respuesta del servidor (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Group-related methods

// CreateGroup creates a new group
func (c *DBServiceClient) CreateGroup(ctx context.Context, req models.GroupRequest) (*models.Group, error) {
	url := fmt.Sprintf("%s/api/v1/groups", c.baseURL)

	c.logger.Debug("Creating new group",
		zap.Any("request", req))

	// Convert request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling group request: %w", err)
	}

	// Make the request
	respBody, err := c.doRequest(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		c.logger.Error("Error in doRequest",
			zap.Error(err),
			zap.String("url", url))
		return nil, fmt.Errorf("error creating group: %w", err)
	}

	c.logger.Debug("Response from DB service",
		zap.ByteString("response", respBody))

	// Define a response structure that matches the API response

	var resp apiResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		c.logger.Error("Failed to parse DB service response",
			zap.Error(err),
			zap.ByteString("response", respBody))
		return nil, fmt.Errorf("error unmarshaling group response: %w", err)
	}

	if resp.Group == nil {
		return nil, fmt.Errorf("group not found in response")
	}

	c.logger.Debug("Successfully created group",
		zap.Any("group", resp.Group))

	return resp.Group, nil
}

// GetGroup retrieves a group by ID
func (c *DBServiceClient) GetGroup(ctx context.Context, id uuid.UUID) (*models.Group, error) {
	url := fmt.Sprintf("%s/api/v1/groups/%s", c.baseURL, id.String())

	c.logger.Debug("Fetching group from DB service",
		zap.String("url", url))

	// Make the request
	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.logger.Error("Error in doRequest",
			zap.Error(err),
			zap.String("url", url))
		return nil, fmt.Errorf("error getting group: %w", err)
	}

	c.logger.Debug("Response from DB service",
		zap.ByteString("response", respBody))

	// Define a response structure that matches the API response

	var resp apiResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		c.logger.Error("Failed to parse DB service response",
			zap.Error(err),
			zap.ByteString("response", respBody))
		return nil, fmt.Errorf("error unmarshaling group response: %w", err)
	}

	if resp.Group == nil {
		return nil, fmt.Errorf("group not found in response")
	}

	c.logger.Debug("Successfully parsed group",
		zap.Any("group", resp.Group))

	return resp.Group, nil
}

// UpdateGroup updates an existing group
func (c *DBServiceClient) UpdateGroup(ctx context.Context, id uuid.UUID, req models.GroupRequest) (*models.Group, error) {
	url := fmt.Sprintf("%s/api/v1/groups/%s", c.baseURL, id.String())

	c.logger.Debug("Updating group",
		zap.String("group_id", id.String()),
		zap.Any("request", req))

	// Convert request to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling group update request: %w", err)
	}

	// Make the request
	respBody, err := c.doRequest(ctx, http.MethodPut, url, reqBody)
	if err != nil {
		c.logger.Error("Error in doRequest",
			zap.Error(err),
			zap.String("url", url))
		return nil, fmt.Errorf("error updating group: %w", err)
	}

	c.logger.Debug("Response from DB service",
		zap.ByteString("response", respBody))

	// Define a response structure that matches the API response

	var resp apiResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		c.logger.Error("Failed to parse DB service response",
			zap.Error(err),
			zap.ByteString("response", respBody))
		return nil, fmt.Errorf("error unmarshaling group response: %w", err)
	}

	if resp.Group == nil {
		return nil, fmt.Errorf("group not found in response")
	}

	c.logger.Debug("Successfully updated group",
		zap.Any("group", resp.Group))

	return resp.Group, nil
}

// DeleteGroup deletes a group by ID
func (c *DBServiceClient) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	url := fmt.Sprintf("%s/api/v1/groups/%s", c.baseURL, id.String())

	// Make the request
	_, err := c.doRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("error deleting group: %w", err)
	}

	return nil
}

func NewDBServiceClient(baseURL string, logger *zap.Logger) *DBServiceClient {
	// Asegurarse de que la URL base no termine con /
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &DBServiceClient{
		baseURL: baseURL,
		client:  &http.Client{},
		logger:  logger,
	}
}

// Group Members related methods

// AddGroupMember adds a user to a group
func (c *DBServiceClient) AddGroupMember(ctx context.Context, groupID, userID, role, addedBy string) (*models.GroupMember, error) {
	url := fmt.Sprintf("%s/api/v1/groups/%s/members", c.baseURL, groupID)

	// The database service expects userId (camelCase) not user_id (snake_case)
	request := map[string]interface{}{
		"userId": userID, // Changed from user_id to userId
		"role":   role,
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling add member request: %w", err)
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		c.logger.Error("Error adding group member",
			zap.Error(err),
			zap.String("group_id", groupID),
			zap.String("user_id", userID))
		return nil, fmt.Errorf("error adding group member: %w", err)
	}

	// The response is a JSON object with a 'member' field containing the group member data
	var response struct {
		Status string             `json:"status"`
		Member models.GroupMember `json:"member"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		c.logger.Error("Error decoding add member response",
			zap.Error(err),
			zap.String("response", string(respBody)))
		return nil, fmt.Errorf("error decoding add member response: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("unexpected status in response: %s", response.Status)
	}

	return &response.Member, nil
}

// ListGroupMembers returns all members of a group
func (c *DBServiceClient) ListGroupMembers(ctx context.Context, groupID string) ([]models.GroupMember, error) {
	url := fmt.Sprintf("%s/api/v1/groups/%s/members", c.baseURL, groupID)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.logger.Error("Error listing group members",
			zap.Error(err),
			zap.String("group_id", groupID))
		return nil, fmt.Errorf("error listing group members: %w", err)
	}

	// The response is a JSON object with a 'members' field containing the array
	var response struct {
		Status  string               `json:"status"`
		Members []models.GroupMember `json:"members"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("error decoding group members response: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("unexpected status in response: %s", response.Status)
	}

	return response.Members, nil
}

// RemoveGroupMember removes a user from a group
func (c *DBServiceClient) RemoveGroupMember(ctx context.Context, groupID, userID, removedBy string) error {
	url := fmt.Sprintf("%s/api/v1/groups/%s/members/%s", c.baseURL, groupID, userID)

	// Add removed_by as a query parameter
	url = fmt.Sprintf("%s?removed_by=%s", url, removedBy)

	_, err := c.doRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		c.logger.Error("Error removing group member",
			zap.Error(err),
			zap.String("group_id", groupID),
			zap.String("user_id", userID))
		return fmt.Errorf("error removing group member: %w", err)
	}

	return nil
}

// IsGroupAdmin checks if a user is an admin of a group
func (c *DBServiceClient) IsGroupAdmin(ctx context.Context, groupID string, userID uuid.UUID) (bool, error) {
	members, err := c.ListGroupMembers(ctx, groupID)
	if err != nil {
		return false, fmt.Errorf("error checking group admin status: %w", err)
	}

	for _, member := range members {
		if member.UserID == userID && member.Role == "admin" {
			return true, nil
		}
	}

	return false, nil
}
