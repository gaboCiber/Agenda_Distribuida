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
	case "group.member.add":
		return s.handleAddGroupMember(ctx, event)
	case "group.member.list":
		return s.handleListGroupMembers(ctx, event)
	case "group.member.remove":
		return s.handleRemoveGroupMember(ctx, event)
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
		ID   string              `json:"id"`
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

// handleAddGroupMember handles adding a member to a group
func (s *EventService) handleAddGroupMember(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	// Parse the request data
	var req struct {
		GroupID string    `json:"group_id"`
		UserID  uuid.UUID `json:"user_id"`
		Role    string    `json:"role,omitempty"`
		AddedBy uuid.UUID `json:"added_by"`
	}

	if err := mapToStruct(event.Data, &req); err != nil {
		errMsg := fmt.Errorf("invalid request data: %w", err)
		resp := models.NewErrorResponse(event.ID, "group.member.add.error", errMsg)
		return &resp, nil
	}

	// Set default role if not provided
	if req.Role == "" {
		req.Role = "member"
	}

	// Convert string groupID to UUID
	groupID, err := uuid.Parse(req.GroupID)
	if err != nil {
		errMsg := fmt.Errorf("invalid group ID format: %w", err)
		resp := models.NewErrorResponse(event.ID, "group.member.add.error", errMsg)
		return &resp, nil
	}

	// Verify the user adding the member is an admin
	isAdmin, err := s.dbClient.IsGroupAdmin(ctx, req.GroupID, req.AddedBy)
	if err != nil {
		errMsg := fmt.Errorf("error checking admin status: %w", err)
		resp := models.NewErrorResponse(event.ID, "group.member.add.error", errMsg)
		return &resp, nil
	}

	if !isAdmin {
		resp := models.NewErrorResponse(event.ID, "group.member.add.unauthorized",
			fmt.Errorf("only group admins can add members"))
		return &resp, nil
	}

	// Check if the group is hierarchical and if we're adding an admin
	if req.Role == "admin" {
		group, err := s.dbClient.GetGroup(ctx, groupID)
		if err != nil {
			errMsg := fmt.Errorf("error getting group: %w", err)
			resp := models.NewErrorResponse(event.ID, "group.member.add.error", errMsg)
			return &resp, nil
		}

		if group.IsHierarchical && group.ParentGroupID != nil {
			// Check if the user is an admin of the parent group
			parentAdmin, err := s.dbClient.IsGroupAdmin(ctx, group.ParentGroupID.String(), req.AddedBy)
			if err != nil {
				errMsg := fmt.Errorf("error checking parent group admin status: %w", err)
				resp := models.NewErrorResponse(event.ID, "group.member.add.error", errMsg)
				return &resp, nil
			}

			if !parentAdmin {
				errMsg := fmt.Errorf("only parent group admins can add subgroup admins")
				resp := models.NewErrorResponse(event.ID, "group.member.add.unauthorized", errMsg)
				return &resp, nil
			}
		}
	}

	// Add the member to the group
	member, err := s.dbClient.AddGroupMember(ctx, req.GroupID, req.UserID.String(), req.Role, req.AddedBy.String())
	if err != nil {
		errMsg := fmt.Errorf("error adding group member: %w", err)
		resp := models.NewErrorResponse(event.ID, "group.member.add.error", errMsg)
		return &resp, nil
	}

	resp := models.NewSuccessResponse(event.ID, "group.member.added", member)
	return &resp, nil
}

// handleListGroupMembers handles listing group members
func (s *EventService) handleListGroupMembers(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	// Parse the request data
	var req struct {
		GroupID string `json:"group_id"`
	}

	if err := mapToStruct(event.Data, &req); err != nil {
		errMsg := fmt.Errorf("invalid request data: %w", err)
		resp := models.NewErrorResponse(event.ID, "group.member.list.error", errMsg)
		return &resp, nil
	}

	// Get the list of members
	members, err := s.dbClient.ListGroupMembers(ctx, req.GroupID)
	if err != nil {
		errMsg := fmt.Errorf("error listing group members: %w", err)
		resp := models.NewErrorResponse(event.ID, "group.member.list.error", errMsg)
		return &resp, nil
	}

	resp := models.NewSuccessResponse(event.ID, "group.member.list", members)
	return &resp, nil
}

// handleRemoveGroupMember handles removing a member from a group
func (s *EventService) handleRemoveGroupMember(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	// Parse the request data
	var req struct {
		GroupID   string    `json:"group_id"`
		UserID    uuid.UUID `json:"user_id"`
		RemovedBy uuid.UUID `json:"removed_by"`
	}

	if err := mapToStruct(event.Data, &req); err != nil {
		errMsg := fmt.Errorf("invalid request data: %w", err)
		resp := models.NewErrorResponse(event.ID, "group.member.remove.error", errMsg)
		return &resp, nil
	}

	// Verify the user removing the member is an admin
	isAdmin, err := s.dbClient.IsGroupAdmin(ctx, req.GroupID, req.RemovedBy)
	if err != nil {
		errMsg := fmt.Errorf("error checking admin status: %w", err)
		resp := models.NewErrorResponse(event.ID, "group.member.remove.error", errMsg)
		return &resp, nil
	}

	if !isAdmin {
		errMsg := fmt.Errorf("only group admins can remove members")
		resp := models.NewErrorResponse(event.ID, "group.member.remove.unauthorized", errMsg)
		return &resp, nil
	}

	// Check if trying to remove the last admin
	if req.UserID == req.RemovedBy {
		members, err := s.dbClient.ListGroupMembers(ctx, req.GroupID)
		if err != nil {
			errMsg := fmt.Errorf("error listing group members: %w", err)
			resp := models.NewErrorResponse(event.ID, "group.member.remove.error", errMsg)
			return &resp, nil
		}

		// Count admins
		adminCount := 0
		for _, m := range members {
			if m.Role == "admin" {
				adminCount++
			}
		}

		if adminCount <= 1 {
			errMsg := fmt.Errorf("cannot remove the last admin of the group")
			resp := models.NewErrorResponse(event.ID, "group.member.remove.error", errMsg)
			return &resp, nil
		}
	}

	// Remove the member from the group
	err = s.dbClient.RemoveGroupMember(ctx, req.GroupID, req.UserID.String(), req.RemovedBy.String())
	if err != nil {
		errMsg := fmt.Errorf("error removing group member: %w", err)
		resp := models.NewErrorResponse(event.ID, "group.member.remove.error", errMsg)
		return &resp, nil
	}

	resp := models.NewSuccessResponse(event.ID, "group.member.removed", nil)
	return &resp, nil
}

// mapToStruct is a helper function to convert map to struct
func mapToStruct(data interface{}, target interface{}) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, target); err != nil {
		return fmt.Errorf("error unmarshaling data: %w", err)
	}

	return nil
}
