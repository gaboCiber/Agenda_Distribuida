package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agenda-distribuida/group-service/internal/clients"
	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type EventService struct {
	dbClient *clients.DBServiceClient
	logger   *zap.Logger
}

// NewEventService creates a new instance of EventService
func NewEventService(dbClient *clients.DBServiceClient, logger *zap.Logger) *EventService {
	return &EventService{
		dbClient: dbClient,
		logger:   logger.Named("event_service"),
	}
}

// ProcessGroupEvent processes group-related events
func (s *EventService) ProcessGroupEvent(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	switch event.Type {
	case "group.create":
		return s.handleCreateGroup(ctx, event)
	case "group.get":
		return s.handleGetGroup(ctx, event)
	case "group.update":
		return s.handleUpdateGroup(ctx, event)
	case "group.delete":
		return s.handleDeleteGroup(ctx, event)
	default:
		return nil, fmt.Errorf("unsupported event type: %s", event.Type)
	}
}

// handleCreateGroup handles group creation
func (s *EventService) handleCreateGroup(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	// Parse the request data
	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return nil, fmt.Errorf("error marshaling event data: %w", err)
	}

	var req models.GroupRequest
	if err := json.Unmarshal(dataBytes, &req); err != nil {
		return nil, fmt.Errorf("error unmarshaling group request: %w", err)
	}

	// Create the group
	group, err := s.dbClient.CreateGroup(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error creating group: %w", err)
	}

	// Return success response
	return &models.EventResponse{
		EventID: event.ID,
		Type:    "group.created",
		Success: true,
		Data:    group,
	}, nil
}

// handleGetGroup handles group retrieval
func (s *EventService) handleGetGroup(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing get group event",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract the group ID from the event data
	idRaw, exists := event.Data["id"]
	if !exists {
		return nil, fmt.Errorf("missing 'id' field in event data")
	}

	// Convert the ID to string
	groupIDStr, ok := idRaw.(string)
	if !ok {
		return nil, fmt.Errorf("invalid group ID format: expected string, got %T", idRaw)
	}

	s.logger.Debug("Parsing group ID",
		zap.String("group_id_str", groupIDStr))

	// Parse the UUID
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid group ID: %w", err)
	}

	s.logger.Debug("Fetching group from database",
		zap.String("group_id", groupID.String()))

	// Get the group
	group, err := s.dbClient.GetGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("error getting group: %w", err)
	}

	s.logger.Debug("Successfully retrieved group",
		zap.Any("group", group))

	// Return success response
	return &models.EventResponse{
		EventID: event.ID,
		Type:    "group.retrieved",
		Success: true,
		Data:    group,
	}, nil
}

// handleUpdateGroup handles group updates
func (s *EventService) handleUpdateGroup(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	// Parse the request data
	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return nil, fmt.Errorf("error marshaling event data: %w", err)
	}

	var data struct {
		ID   string           `json:"id"`
		Data models.GroupRequest `json:"data"`
	}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return nil, fmt.Errorf("error unmarshaling update group request: %w", err)
	}

	// Parse the UUID
	groupID, err := uuid.Parse(data.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid group ID: %w", err)
	}

	// Update the group
	group, err := s.dbClient.UpdateGroup(ctx, groupID, data.Data)
	if err != nil {
		return nil, fmt.Errorf("error updating group: %w", err)
	}

	// Return success response
	return &models.EventResponse{
		EventID: event.ID,
		Type:    "group.updated",
		Success: true,
		Data:    group,
	}, nil
}

// handleDeleteGroup handles group deletion
func (s *EventService) handleDeleteGroup(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	// Parse the group ID from the event data
	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return nil, fmt.Errorf("error marshaling event data: %w", err)
	}

	var data struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return nil, fmt.Errorf("error unmarshaling delete group request: %w", err)
	}

	// Parse the UUID
	groupID, err := uuid.Parse(data.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid group ID: %w", err)
	}

	// Delete the group
	if err := s.dbClient.DeleteGroup(ctx, groupID); err != nil {
		return nil, fmt.Errorf("error deleting group: %w", err)
	}

	// Return success response
	return &models.EventResponse{
		EventID: event.ID,
		Type:    "group.deleted",
		Success: true,
	}, nil
}
