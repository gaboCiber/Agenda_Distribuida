package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agenda-distribuida/group-service/internal/clients"
	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Event type constants
const (
	// Group events
	EventTypeGroupCreate       = "group.create"
	EventTypeGroupGet          = "group.get"
	EventTypeGroupUpdate       = "group.update"
	EventTypeGroupDelete       = "group.delete"
	EventTypeGroupMemberAdd    = "group.member.add"
	EventTypeGroupMemberGet    = "group.member.get"
	EventTypeGroupMemberList   = "group.member.list"
	EventTypeGroupMemberRemove = "group.member.remove"
	EventTypeGroupMemberUpdate = "group.member.update"

	EventTypeUserGroupsList = "user.groups.list"

	// Invitation events
	EventTypeInviteCreate = "group.invite.create"
	EventTypeInviteAccept = "group.invite.accept"
	EventTypeInviteReject = "group.invite.reject"
	EventTypeInviteList   = "group.invite.list"
	EventTypeInviteGet    = "group.invite.get"
	EventTypeInviteCancel = "group.invite.cancel"

	// Group event events
	EventTypeGroupEventCreate       = "group.event.create"
	EventTypeGroupEventGet          = "group.event.get"
	EventTypeGroupEventDelete       = "group.event.delete"
	EventTypeGroupEventList         = "group.event.list"
	EventTypeGroupEventStatusUpdate = "group.event.status.update"
	EventTypeGroupEventStatusGet    = "group.event.status.get"
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
	// Group management events
	case EventTypeGroupCreate:
		return s.handleCreateGroup(ctx, event)
	case EventTypeGroupGet:
		return s.handleGetGroup(ctx, event)
	case EventTypeGroupUpdate:
		return s.handleUpdateGroup(ctx, event)
	case EventTypeGroupDelete:
		return s.handleDeleteGroup(ctx, event)
	// case EventTypeGroupMemberAdd:
	// 	return s.handleAddGroupMember(ctx, event)
	case EventTypeGroupMemberList:
		return s.handleListGroupMembers(ctx, event)
	// case EventTypeGroupMemberGet:
	// 	return s.handleGetGroupMember(ctx, event)
	case EventTypeGroupMemberUpdate:
		return s.handleUpdateGroupMember(ctx, event)
	case EventTypeGroupMemberRemove:
		return s.handleRemoveGroupMember(ctx, event)
	case EventTypeUserGroupsList:
		return s.handleListUserGroups(ctx, event)

	// Invitation events
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

	// Group event events
	case EventTypeGroupEventCreate:
		return s.handleCreateGroupEvent(ctx, event)
	case EventTypeGroupEventGet:
		return s.handleGetGroupEvent(ctx, event)
	case EventTypeGroupEventDelete:
		return s.handleDeleteGroupEvent(ctx, event)
	case EventTypeGroupEventList:
		return s.handleListGroupEvents(ctx, event)
	case EventTypeGroupEventStatusUpdate:
		return s.handleUpdateGroupEventStatus(ctx, event)
	case EventTypeGroupEventStatusGet:
		return s.handleGetGroupEventStatus(ctx, event)

	default:
		return nil, fmt.Errorf("unknown event type: %s", event.Type)
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
	member, err := s.dbClient.AddGroupMember(ctx, req.GroupID, req.UserID.String(), req.Role)
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

// handleGetGroupMember handles getting a specific group member
func (s *EventService) handleGetGroupMember(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	// Parse request data
	var requestData struct {
		GroupID string `json:"group_id" validate:"required"`
		UserID  string `json:"user_id" validate:"required"`
	}

	if err := s.mapToStruct(event.Data, &requestData); err != nil {
		s.logger.Error("Error parsing get group member request", zap.Error(err))
		return nil, fmt.Errorf("invalid request data: %w", err)
	}

	// Validate request data
	if _, err := uuid.Parse(requestData.GroupID); err != nil {
		return nil, fmt.Errorf("invalid group ID format: %w", err)
	}

	if _, err := uuid.Parse(requestData.UserID); err != nil {
		return nil, fmt.Errorf("invalid user ID format: %w", err)
	}

	// Get the group member from the database
	member, err := s.dbClient.GetGroupMember(ctx, requestData.GroupID, requestData.UserID)
	if err != nil {
		s.logger.Error("Error getting group member",
			zap.String("group_id", requestData.GroupID),
			zap.String("user_id", requestData.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get group member: %w", err)
	}

	// Return the group member
	return &models.EventResponse{
		Type: event.Type + ".response",
		Data: map[string]interface{}{
			"member": member,
		},
	}, nil
}

// handleUpdateGroupMember handles group updates
func (s *EventService) handleUpdateGroupMember(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	// Parse the request data
	var req struct {
		GroupID   string `json:"group_id"`
		UserEmail string `json:"email"`
		Role      string `json:"role"`
	}

	if err := mapToStruct(event.Data, &req); err != nil {
		errMsg := fmt.Errorf("invalid request data: %w", err)
		resp := models.NewErrorResponse(event.ID, "group.member.remove.error", errMsg)
		return &resp, nil
	}

	// Parse the UUID
	_, err := uuid.Parse(req.GroupID)
	if err != nil {
		return nil, fmt.Errorf("invalid group ID: %w", err)
	}

	// Update the group
	err3 := s.dbClient.UpdateGroupMember(ctx, req.GroupID, req.UserEmail, req.Role)
	if err3 != nil {
		return nil, fmt.Errorf("error updating group: %w", err)
	}

	// Return success response
	return &models.EventResponse{
		EventID: event.ID,
		Type:    "group.member.updated",
		Success: true,
	}, nil
}

// handleRemoveGroupMember handles removing a member from a group
func (s *EventService) handleRemoveGroupMember(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	// Parse the request data
	var req struct {
		GroupID   string `json:"group_id"`
		UserEmail string `json:"email"`
	}

	if err := mapToStruct(event.Data, &req); err != nil {
		errMsg := fmt.Errorf("invalid request data: %w", err)
		resp := models.NewErrorResponse(event.ID, "group.member.remove.error", errMsg)
		return &resp, nil
	}

	// Remove the member from the group
	err := s.dbClient.RemoveGroupMember(ctx, req.GroupID, req.UserEmail)
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
		req.UserEmail,
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
	_, err = s.dbClient.AddGroupMember(ctx, invitation.GroupID.String(), invitation.UserEmail, "member")
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

// handleCreateGroupEvent handles creating a new group event
func (s *EventService) handleCreateGroupEvent(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing create group event",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract required fields
	groupID, ok := event.Data["group_id"].(string)
	if !ok || groupID == "" {
		return nil, fmt.Errorf("missing or invalid group_id")
	}

	eventID, ok := event.Data["event_id"].(string)
	if !ok || eventID == "" {
		return nil, fmt.Errorf("missing or invalid event_id")
	}

	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("missing or invalid user_id")
	}

	// Check if the user is a member of the group
	isMember, err := s.dbClient.IsGroupMember(ctx, groupID, userID)
	if err != nil {
		return nil, fmt.Errorf("error checking group membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of the group")
	}

	// Convert groupID to uuid.UUID
	groupUUID, err := uuid.Parse(groupID)
	if err != nil {
		return nil, fmt.Errorf("invalid group ID format: %w", err)
	}

	// Check if the group is hierarchical
	group, err := s.dbClient.GetGroup(ctx, groupUUID)
	if err != nil {
		return nil, fmt.Errorf("error getting group: %w", err)
	}

	// In a hierarchical group, only admins can create events
	if group.IsHierarchical {
		userUUID, err := uuid.Parse(userID)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID: %w", err)
		}

		isAdmin, err := s.dbClient.IsGroupAdmin(ctx, groupID, userUUID)
		if err != nil {
			return nil, fmt.Errorf("error checking admin status: %w", err)
		}

		if !isAdmin {
			return nil, fmt.Errorf("unauthorized: only group admins can create events in hierarchical groups")
		}

		// For hierarchical groups, create the event with status 'accepted'
		groupEvent, err := s.dbClient.CreateGroupEvent(ctx, groupID, eventID, userID, "accepted", true)
		if err != nil {
			return nil, fmt.Errorf("error creating group event: %w", err)
		}

		// For hierarchical groups, automatically accept the event for all members
		members, err := s.dbClient.ListGroupMembers(ctx, groupID)
		if err != nil {
			return nil, fmt.Errorf("error getting group members: %w", err)
		}

		// Add and set status for each member
		for _, member := range members {
			_, err := s.dbClient.AddEventStatus(ctx, eventID, groupID, member.UserID.String(), "accepted")
			if err != nil {
				s.logger.Error("Failed to add event status for member",
					zap.String("event_id", eventID),
					zap.String("user_id", member.UserID.String()),
					zap.Error(err))
				// Continue with other members even if one fails
			}
		}

		return &models.EventResponse{
			EventID: event.ID,
			Type:    "group.event.created",
			Success: true,
			Data: map[string]interface{}{
				"group_id":        groupEvent.GroupID,
				"event_id":        groupEvent.EventID,
				"status":          groupEvent.Status,
				"is_hierarchical": true,
			},
		}, nil
	}

	// For non-hierarchical groups, create the event with status 'pending'
	groupEvent, err := s.dbClient.CreateGroupEvent(ctx, groupID, eventID, userID, "pending", false)
	if err != nil {
		return nil, fmt.Errorf("error creating group event: %w", err)
	}

	// For non-hierarchical groups, set to pending the event for all members
	members, err := s.dbClient.ListGroupMembers(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("error getting group members: %w", err)
	}

	// Add and set status for each member
	for _, member := range members {
		if member.UserID.String() == userID {
			_, err = s.dbClient.AddEventStatus(ctx, eventID, groupID, member.UserID.String(), "accepted")
		} else {
			_, err = s.dbClient.AddEventStatus(ctx, eventID, groupID, member.UserID.String(), "pending")
		}

		if err != nil {
			s.logger.Error("Failed to add event status for member",
				zap.String("event_id", eventID),
				zap.String("user_id", member.UserID.String()),
				zap.Error(err))
			// Continue with other members even if one fails
		}
	}

	return &models.EventResponse{
		EventID: event.ID,
		Type:    "group.event.created",
		Success: true,
		Data: map[string]interface{}{
			"group_id":        groupEvent.GroupID,
			"event_id":        groupEvent.EventID,
			"status":          groupEvent.Status,
			"is_hierarchical": false,
		},
	}, nil
}

// handleGetGroupEvent handles retrieving a group event
func (s *EventService) handleGetGroupEvent(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing get group event",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract required fields
	groupID, ok := event.Data["group_id"].(string)
	if !ok || groupID == "" {
		return nil, fmt.Errorf("missing or invalid group_id")
	}

	eventID, ok := event.Data["event_id"].(string)
	if !ok || eventID == "" {
		return nil, fmt.Errorf("missing or invalid event_id")
	}

	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("missing or invalid user_id")
	}

	// Check if the user is a member of the group
	isMember, err := s.dbClient.IsGroupMember(ctx, groupID, userID)
	if err != nil {
		return nil, fmt.Errorf("error checking group membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of the group")
	}

	// Get the group event
	groupEvent, err := s.dbClient.GetGroupEvent(ctx, groupID, eventID)
	if err != nil {
		return nil, fmt.Errorf("error getting group event: %w", err)
	}

	// Get the user's status for this event
	eventStatus, err := s.dbClient.GetEventStatus(ctx, eventID, userID)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, fmt.Errorf("error getting event status: %w", err)
	}

	// For non-hierarchical groups, check if all members have accepted
	var allAccepted bool
	if !groupEvent.IsHierarchical {
		allAccepted, err = s.dbClient.HasAllMembersAccepted(ctx, eventID, groupID)
		if err != nil {
			s.logger.Error("Error checking if all members have accepted",
				zap.String("event_id", eventID),
				zap.String("group_id", groupID),
				zap.Error(err))
		}
	}

	return &models.EventResponse{
		EventID: event.ID,
		Type:    "group.event.retrieved",
		Success: true,
		Data: map[string]interface{}{
			"group_id":        groupEvent.GroupID,
			"event_id":        groupEvent.EventID,
			"added_by":        groupEvent.AddedBy,
			"is_hierarchical": groupEvent.IsHierarchical,
			"status":          groupEvent.Status,
			"created_at":      groupEvent.CreatedAt,
			"user_status":     eventStatus,
			"all_accepted":    allAccepted,
		},
	}, nil
}

// handleDeleteGroupEvent handles deleting a group event
func (s *EventService) handleDeleteGroupEvent(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing delete group event",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract required fields
	groupID, ok := event.Data["group_id"].(string)
	if !ok || groupID == "" {
		return nil, fmt.Errorf("missing or invalid group_id")
	}

	eventID, ok := event.Data["event_id"].(string)
	if !ok || eventID == "" {
		return nil, fmt.Errorf("missing or invalid event_id")
	}

	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("missing or invalid user_id")
	}

	// Get the group event first to check permissions
	groupEvent, err := s.dbClient.GetGroupEvent(ctx, groupID, eventID)
	if err != nil {
		return nil, fmt.Errorf("error getting group event: %w", err)
	}

	// Only the user who created the event or a group admin can delete it
	if groupEvent.AddedBy != userID {
		// Check if user is a group admin
		userUUID, err := uuid.Parse(userID)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID: %w", err)
		}

		isAdmin, err := s.dbClient.IsGroupAdmin(ctx, groupID, userUUID)
		if err != nil {
			return nil, fmt.Errorf("error checking admin status: %w", err)
		}

		if !isAdmin {
			return nil, fmt.Errorf("unauthorized: only the event creator or group admin can delete the event")
		}
	}

	// Delete the group event
	err = s.dbClient.DeleteGroupEvent(ctx, groupID, eventID)
	if err != nil {
		return nil, fmt.Errorf("error deleting group event: %w", err)
	}

	return &models.EventResponse{
		EventID: event.ID,
		Type:    "group.event.deleted",
		Success: true,
		Data: map[string]interface{}{
			"group_id": groupID,
			"event_id": eventID,
		},
	}, nil
}

// handleListGroupEvents handles listing all events for a group
func (s *EventService) handleListGroupEvents(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing list group events",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract required fields
	groupID, ok := event.Data["group_id"].(string)
	if !ok || groupID == "" {
		return nil, fmt.Errorf("missing or invalid group_id")
	}

	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("missing or invalid user_id")
	}

	// Check if the user is a member of the group
	isMember, err := s.dbClient.IsGroupMember(ctx, groupID, userID)
	if err != nil {
		return nil, fmt.Errorf("error checking group membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of the group")
	}

	// Get all events for the group
	groupEvents, err := s.dbClient.ListGroupEvents(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("error listing group events: %w", err)
	}

	// For each event, get the user's status
	var eventsWithStatus []map[string]interface{}
	for _, ge := range groupEvents {
		eventData := map[string]interface{}{
			"id":              ge.ID,
			"group_id":        ge.GroupID,
			"event_id":        ge.EventID,
			"added_by":        ge.AddedBy,
			"is_hierarchical": ge.IsHierarchical,
			"status":          ge.Status,
			"created_at":      ge.CreatedAt,
		}

		// Get the user's status for this event
		eventStatus, err := s.dbClient.GetEventStatus(ctx, ge.EventID, userID)
		if err == nil && eventStatus != nil {
			eventData["user_status"] = eventStatus.Status
		}

		eventsWithStatus = append(eventsWithStatus, eventData)
	}

	return &models.EventResponse{
		EventID: event.ID,
		Type:    "group.event.listed",
		Success: true,
		Data: map[string]interface{}{
			"group_id": groupID,
			"events":   eventsWithStatus,
		},
	}, nil
}

// handleUpdateGroupEventStatus handles updating a user's status for a group event
func (s *EventService) handleUpdateGroupEventStatus(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing update group event status",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract required fields
	eventID, ok := event.Data["event_id"].(string)
	if !ok || eventID == "" {
		return nil, fmt.Errorf("missing or invalid event_id")
	}

	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("missing or invalid user_id")
	}

	status, ok := event.Data["status"].(string)
	if !ok || status == "" {
		return nil, fmt.Errorf("missing or invalid status")
	}

	// Validate status
	validStatuses := map[string]bool{
		"accepted": true,
		"declined": true,
		"pending":  true,
	}

	if !validStatuses[status] {
		return nil, fmt.Errorf("invalid status: %s. Must be one of: accepted, declined, pending", status)
	}

	// Get the group event to check permissions
	groupID, ok := event.Data["group_id"].(string)
	if !ok || groupID == "" {
		return nil, fmt.Errorf("missing or invalid group_id")
	}

	// Check if the user is a member of the group
	isMember, err := s.dbClient.IsGroupMember(ctx, groupID, userID)
	if err != nil {
		return nil, fmt.Errorf("error checking group membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of the group")
	}

	// Get the group to check if it's hierarchical

	// Convert groupID to uuid.UUID
	groupUUID, err := uuid.Parse(groupID)
	if err != nil {
		return nil, fmt.Errorf("invalid group ID format: %w", err)
	}

	// Check if the group is hierarchical
	group, err := s.dbClient.GetGroup(ctx, groupUUID)
	if err != nil {
		return nil, fmt.Errorf("error getting group: %w", err)
	}

	// In a hierarchical group, only admins can update event status for other users
	if group.IsHierarchical && status != "accepted" {
		// Only allow admins to decline events in hierarchical groups
		userUUID, err := uuid.Parse(userID)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID: %w", err)
		}

		isAdmin, err := s.dbClient.IsGroupAdmin(ctx, groupID, userUUID)
		if err != nil {
			return nil, fmt.Errorf("error checking admin status: %w", err)
		}

		if !isAdmin {
			return nil, fmt.Errorf("unauthorized: only group admins can decline events in hierarchical groups")
		}
	}

	// Update the event status
	eventStatus, err := s.dbClient.UpdateEventStatus(ctx, eventID, userID, status)
	if err != nil {
		return nil, fmt.Errorf("error updating event status: %w", err)
	}

	// For non-hierarchical groups, check if all members have accepted
	var allAccepted bool
	if !group.IsHierarchical && status == "accepted" {
		allAccepted, err = s.dbClient.HasAllMembersAccepted(ctx, eventID, groupID)
		if err != nil {
			s.logger.Error("Error checking if all members have accepted",
				zap.String("event_id", eventID),
				zap.String("group_id", groupID),
				zap.Error(err))
		}

		// If all members have accepted, update the event status to 'accepted'
		if allAccepted {
			// First, get the current event to preserve its hierarchical status
			groupEvent, err := s.dbClient.GetGroupEvent(ctx, groupID, eventID)
			if err != nil {
				s.logger.Error("Error getting group event details",
					zap.String("event_id", eventID),
					zap.String("group_id", groupID),
					zap.Error(err))
				return nil, fmt.Errorf("error getting group event details: %w", err)
			}

			// Update the event status while preserving the existing hierarchical status
			_, err = s.dbClient.UpdateGroupEvent(ctx, groupID, eventID, "accepted", groupEvent.IsHierarchical)
			if err != nil {
				s.logger.Error("Error updating group event status to accepted",
					zap.String("event_id", eventID),
					zap.String("group_id", groupID),
					zap.Error(err))
			}
		}
	}

	return &models.EventResponse{
		EventID: event.ID,
		Type:    "group.event.status.updated",
		Success: true,
		Data: map[string]interface{}{
			"event_id":     eventID,
			"user_id":      userID,
			"status":       eventStatus.Status,
			"updated_at":   eventStatus.UpdatedAt,
			"all_accepted": allAccepted,
		},
	}, nil
}

// handleGetGroupEventStatus handles getting a user's status for a group event
func (s *EventService) handleGetGroupEventStatus(ctx context.Context, event models.Event) (*models.EventResponse, error) {
	s.logger.Debug("Processing get group event status",
		zap.String("event_id", event.ID),
		zap.Any("event_data", event.Data))

	// Extract required fields
	eventID, ok := event.Data["event_id"].(string)
	if !ok || eventID == "" {
		return nil, fmt.Errorf("missing or invalid event_id")
	}

	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return nil, fmt.Errorf("missing or invalid user_id")
	}

	// Get the event status
	eventStatus, err := s.dbClient.GetEventStatus(ctx, eventID, userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return &models.EventResponse{
				EventID: event.ID,
				Type:    "group.event.status.retrieved",
				Success: true,
				Data: map[string]interface{}{
					"event_id": eventID,
					"user_id":  userID,
					"status":   "pending", // Default status if not found
				},
			}, nil
		}
		return nil, fmt.Errorf("error getting event status: %w", err)
	}

	return &models.EventResponse{
		EventID: event.ID,
		Type:    "group.event.status.retrieved",
		Success: true,
		Data: map[string]interface{}{
			"event_id":   eventID,
			"user_id":    userID,
			"status":     eventStatus.Status,
			"updated_at": eventStatus.UpdatedAt,
		},
	}, nil
}
