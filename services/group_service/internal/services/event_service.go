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

// Event type constants
const (
	EventTypeGroupCreate       = "group.create"
	EventTypeGroupGet          = "group.get"
	EventTypeGroupUpdate       = "group.update"
	EventTypeGroupDelete       = "group.delete"
	EventTypeGroupMemberAdd    = "group.member.add"
	EventTypeGroupMemberList   = "group.member.list"
	EventTypeGroupMemberRemove = "group.member.remove"
	EventTypeUserGroupsList    = "user.groups.list"
	EventTypeInviteCreate      = "group.invite.create"
	EventTypeInviteAccept      = "group.invite.accept"
	EventTypeInviteReject      = "group.invite.reject"
	EventTypeInviteList        = "group.invite.list"
	EventTypeInviteGet         = "group.invite.get"
	EventTypeInviteCancel      = "group.invite.cancel"
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
	case EventTypeGroupCreate:
		return s.handleCreateGroup(ctx, event)
	case EventTypeGroupGet:
		return s.handleGetGroup(ctx, event)
	case EventTypeGroupUpdate:
		return s.handleUpdateGroup(ctx, event)
	case EventTypeGroupDelete:
		return s.handleDeleteGroup(ctx, event)
	case EventTypeGroupMemberAdd:
		return s.handleAddGroupMember(ctx, event)
	case EventTypeGroupMemberList:
		return s.handleListGroupMembers(ctx, event)
	case EventTypeGroupMemberRemove:
		return s.handleRemoveGroupMember(ctx, event)
	case EventTypeUserGroupsList:
		return s.handleListUserGroups(ctx, event)
	case EventTypeInviteCreate:
		return s.handleCreateInvitation(ctx, event)
	case EventTypeInviteAccept:
		return s.handleAcceptInvitation(ctx, event)
	case EventTypeInviteReject:
		return s.handleRejectInvitation(ctx, event)
	case EventTypeInviteList:
		return s.handleListInvitations(ctx, event)
	case EventTypeInviteGet:
		return s.handleGetInvitation(ctx, event)
	case EventTypeInviteCancel:
		return s.handleCancelInvitation(ctx, event)
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

// handleListUserGroups handles listing all groups that a user is a member of
func (s *EventService) handleListUserGroups(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	// Parse the request data
	var req struct {
		UserID string `json:"user_id"`
	}

	if err := s.mapToStruct(event.Data, &req); err != nil {
		errMsg := fmt.Errorf("invalid request data: %w", err)
		resp := models.NewErrorResponse(event.ID, "user.groups.list.error", errMsg)
		return &resp, nil
	}

	// Get the list of groups for the user
	groups, err := s.dbClient.ListUserGroups(ctx, req.UserID)
	if err != nil {
		errMsg := fmt.Errorf("error listing user groups: %w", err)
		resp := models.NewErrorResponse(event.ID, "user.groups.list.error", errMsg)
		return &resp, nil
	}

	resp := models.NewSuccessResponse(event.ID, "user.groups.list", groups)
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

func (s *EventService) mapToStruct(data interface{}, target interface{}) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, target); err != nil {
		return fmt.Errorf("error unmarshaling data: %w", err)
	}

	return nil
}

// handleCreateInvitation handles the creation of a new group invitation
func (s *EventService) handleCreateInvitation(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing create invitation event",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract invitation data
	var req models.InvitationRequest
	if err := s.mapToStruct(event.Data, &req); err != nil {
		return nil, fmt.Errorf("invalid invitation data: %w", err)
	}

	// Check if the inviter is an admin of the group
	isAdmin, err := s.dbClient.IsGroupAdmin(ctx, req.GroupID.String(), req.InvitedBy)
	if err != nil {
		return nil, fmt.Errorf("error checking admin status: %w", err)
	}

	if !isAdmin {
		return nil, fmt.Errorf("only group admins can send invitations")
	}

	// Create the invitation
	invitation, err := s.dbClient.CreateInvitation(
		ctx,
		req.GroupID.String(),
		req.UserID.String(),
		req.InvitedBy.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating invitation: %w", err)
	}

	return &models.EventResponse{
		EventID: event.ID,
		Type:    "invitation.created",
		Success: true,
		Data:    invitation,
	}, nil
}

// handleAcceptInvitation handles accepting a group invitation
func (s *EventService) handleAcceptInvitation(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing accept invitation event",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract invitation ID and user ID
	invitationID, ok := event.Data["invitation_id"].(string)
	if !ok || invitationID == "" {
		return nil, fmt.Errorf("missing or invalid invitation_id")
	}

	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("missing or invalid user_id")
	}

	// Get the invitation
	invitation, err := s.dbClient.GetInvitation(ctx, invitationID)
	if err != nil {
		return nil, fmt.Errorf("error getting invitation: %w", err)
	}

	if invitation == nil {
		return nil, fmt.Errorf("invitation not found")
	}

	// Check if the user is the one who was invited
	if invitation.UserID.String() != userID {
		return nil, fmt.Errorf("unauthorized to accept this invitation")
	}

	// Update the invitation status to accepted
	if err := s.dbClient.RespondToInvitation(ctx, invitationID, "accepted"); err != nil {
		return nil, fmt.Errorf("error accepting invitation: %w", err)
	}

	// Add user to the group as a member
	_, err = s.dbClient.AddGroupMember(ctx, invitation.GroupID.String(), userID, "member", userID)
	if err != nil {
		return nil, fmt.Errorf("error adding user to group: %w", err)
	}

	return &models.EventResponse{
		EventID: event.ID,
		Type:    "invitation.accepted",
		Success: true,
		Data:    map[string]string{"status": "accepted"},
	}, nil
}

// handleRejectInvitation handles rejecting a group invitation
func (s *EventService) handleRejectInvitation(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing reject invitation event",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract invitation ID and user ID
	invitationID, ok := event.Data["invitation_id"].(string)
	if !ok || invitationID == "" {
		return nil, fmt.Errorf("missing or invalid invitation_id")
	}

	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("missing or invalid user_id")
	}

	// Get the invitation
	invitation, err := s.dbClient.GetInvitation(ctx, invitationID)
	if err != nil {
		return nil, fmt.Errorf("error getting invitation: %w", err)
	}

	if invitation == nil {
		return nil, fmt.Errorf("invitation not found")
	}

	// Check if the user is the one who was invited
	if invitation.UserID.String() != userID {
		return nil, fmt.Errorf("unauthorized to reject this invitation")
	}

	// Update the invitation status to rejected
	if err := s.dbClient.RespondToInvitation(ctx, invitationID, "rejected"); err != nil {
		return nil, fmt.Errorf("error rejecting invitation: %w", err)
	}

	return &models.EventResponse{
		EventID: event.ID,
		Type:    "invitation.rejected",
		Success: true,
		Data:    map[string]string{"status": "rejected"},
	}, nil
}

// handleListInvitations handles listing invitations for a user
func (s *EventService) handleListInvitations(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing list invitations event",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract user ID and optional status filter
	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("missing or invalid user_id")
	}

	status, _ := event.Data["status"].(string) // Optional status filter

	// Get the invitations
	invitations, err := s.dbClient.ListUserInvitations(ctx, userID, status)
	if err != nil {
		return nil, fmt.Errorf("error listing invitations: %w", err)
	}

	return &models.EventResponse{
		EventID: event.ID,
		Type:    "invitations.listed",
		Success: true,
		Data:    map[string]interface{}{"invitations": invitations},
	}, nil
}

// handleCancelInvitation handles canceling a group invitation
// handleGetInvitation handles getting a specific invitation by ID
func (s *EventService) handleGetInvitation(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing get invitation event",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract invitation ID
	invitationID, ok := event.Data["invitation_id"].(string)
	if !ok || invitationID == "" {
		return nil, fmt.Errorf("missing or invalid invitation_id")
	}

	// Get the invitation
	invitation, err := s.dbClient.GetInvitation(ctx, invitationID)
	if err != nil {
		return nil, fmt.Errorf("error getting invitation: %w", err)
	}

	if invitation == nil {
		return nil, fmt.Errorf("invitation not found")
	}

	return &models.EventResponse{
		EventID: event.ID,
		Type:    "invitation.retrieved",
		Success: true,
		Data:    invitation,
	}, nil
}

// handleCancelInvitation handles canceling a group invitation
// Only the user who created the invitation or a group admin can cancel it
// handleCancelInvitation handles canceling a group invitation
// Only the user who created the invitation or a group admin can cancel it
func (s *EventService) handleCancelInvitation(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing cancel invitation event",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract invitation ID and user ID
	invitationID, ok := event.Data["invitation_id"].(string)
	if !ok || invitationID == "" {
		return nil, fmt.Errorf("missing or invalid invitation_id")
	}

	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("missing or invalid user_id")
	}

	// Get the invitation to verify ownership
	invitation, err := s.dbClient.GetInvitation(ctx, invitationID)
	if err != nil {
		return nil, fmt.Errorf("error getting invitation: %w", err)
	}

	if invitation == nil {
		return nil, fmt.Errorf("invitation not found")
	}

	// Convert string userID to uuid.UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Check if the user is the one who created the invitation or a group admin
	isAdmin, err := s.dbClient.IsGroupAdmin(ctx, invitation.GroupID.String(), userUUID)
	if err != nil {
		return nil, fmt.Errorf("error checking admin status: %w", err)
	}

	// User must be either the inviter or a group admin to cancel
	if invitation.InvitedBy.String() != userID && !isAdmin {
		return nil, fmt.Errorf("unauthorized: only the inviter or group admin can cancel the invitation")
	}

	// Delete the invitation
	err = s.dbClient.DeleteInvitation(ctx, invitationID)
	if err != nil {
		return nil, fmt.Errorf("error canceling invitation: %w", err)
	}

	// Publish notification that the invitation was canceled
	// This would be handled by a separate notification service in a real application

	return &models.EventResponse{
		EventID: event.ID,
		Type:    "invitation.canceled",
		Success: true,
		Data: map[string]interface{}{
			"invitation_id": invitationID,
			"canceled_by":   userID,
		},
	}, nil
}
