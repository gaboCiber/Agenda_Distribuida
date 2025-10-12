package events

import (
	"encoding/json"
	"log"
	"time"

	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/agenda-distribuida/group-service/internal/service"
)

// EventHandler handles incoming events from Redis
type EventHandler struct {
	groupService service.GroupService
	publisher    *Publisher
}

// NewEventHandler creates a new event handler
func NewEventHandler(groupService service.GroupService, publisher *Publisher) *EventHandler {
	return &EventHandler{
		groupService: groupService,
		publisher:    publisher,
	}
}

// HandleMessage processes an incoming message from Redis
func (h *EventHandler) HandleMessage(channel, payload string) {
	log.Printf("📨 Received message on channel %s: %s", channel, payload)

	// Parse the event
	var event Event
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("❌ Failed to unmarshal event: %v", err)
		return
	}

	log.Printf("🔍 Processing event: Type=%s, ID=%s", event.Type, event.ID)

	// Route the event to the appropriate handler
	switch event.Type {
	// User events
	case "user_deleted":
		h.handleUserDeleted(event.Payload)

	// Event events
	case "event_deleted":
		h.handleEventDeleted(event.Payload)

	// Group events
	case "group_created":
		h.handleGroupCreated(event.Payload)
	case "group_updated":
		h.handleGroupUpdated(event.Payload)
	case "group_deleted":
		h.handleGroupDeleted(event.Payload)

	// Member events
	case "member_added":
		h.handleMemberAdded(event.Payload)
	case "member_removed":
		h.handleMemberRemoved(event.Payload)
	case "member_role_updated":
		h.handleMemberRoleUpdated(event.Payload)

	// Invitation events
	case "invitation_created":
		h.handleInvitationCreated(event.Payload)
	case "invitation_accepted":
		h.handleInvitationAccepted(event.Payload)
	case "invitation_rejected":
		h.handleInvitationRejected(event.Payload)

	// Group-Event relationship events
	case "event_added_to_group":
		h.handleEventAddedToGroup(event.Payload)
	case "event_removed_from_group":
		h.handleEventRemovedFromGroup(event.Payload)

	default:
		log.Printf("⚠️ Unhandled event type: %s", event.Type)
	}
}

// handleUserDeleted handles user_deleted events
func (h *EventHandler) handleUserDeleted(payload interface{}) {
	// Convert payload to map
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for user_deleted event")
		return
	}

	// Get user ID
	userID, ok := data["user_id"].(string)
	if !ok || userID == "" {
		log.Printf("❌ Missing or invalid user_id in user_deleted event")
		return
	}

	// Handle user deletion using the service
	if err := h.groupService.HandleUserDeleted(userID); err != nil {
		log.Printf("❌ Failed to handle user deleted event: %v", err)
		return
	}

	log.Printf("✅ Successfully handled user_deleted event for user %s", userID)
}

// handleEventDeleted handles event_deleted events
func (h *EventHandler) handleEventDeleted(payload interface{}) {
	// Convert payload to map
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for event_deleted event")
		return
	}

	// Get event ID
	eventID, ok := data["event_id"].(string)
	if !ok || eventID == "" {
		log.Printf("❌ Missing or invalid event_id in event_deleted event")
		return
	}

	// Handle event deletion using the service
	if err := h.groupService.HandleEventDeleted(eventID); err != nil {
		log.Printf("❌ Failed to handle event deleted event: %v", err)
		return
	}

	log.Printf("✅ Successfully handled event_deleted event for event %s", eventID)
}

// handleGroupCreated handles group_created events
func (h *EventHandler) handleGroupCreated(payload interface{}) {
	// Convert payload to map
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for group_created event")
		return
	}

	// Extract group data
	groupID, _ := data["group_id"].(string)
	name, _ := data["name"].(string)
	createdBy, _ := data["created_by"].(string)
	responseChannel, _ := data["response_channel"].(string)

	if groupID == "" || name == "" || createdBy == "" {
		return
	}

	// Create the group using the service
	group := &models.Group{
		ID:          groupID,
		Name:        name,
		Description: data["description"].(string),
		CreatedBy:   createdBy,
	}

	// Create the group and get the created group with its database ID
	createdGroup, err := h.groupService.CreateGroup(group)
	if err != nil {
		log.Printf("❌ Failed to create group: %v", err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "group_created_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to create group",
				"error":   err.Error(),
			})
		}
		return
	}

	log.Printf("✅ Created group %s (ID: %s, DB ID: %s) created by %s",
		name, groupID, createdGroup.ID, createdBy)

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "group_created_response", map[string]interface{}{
			"id":     createdGroup.ID, // Include the database ID
			"status": "success",
			"payload": map[string]interface{}{
				"group_id":   groupID,
				"name":       name,
				"created_by": createdBy,
			},
		})
	}
}

// handleGroupUpdated handles group_updated events
func (h *EventHandler) handleGroupUpdated(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for group_updated event")
		return
	}

	groupID, _ := data["group_id"].(string)
	name, _ := data["name"].(string)
	description, _ := data["description"].(string)

	if groupID == "" {
		log.Printf("❌ Missing group_id in group_updated event")
		return
	}

	group, err := h.groupService.GetGroup(groupID)
	if err != nil {
		log.Printf("❌ Failed to get group %s: %v", groupID, err)
		return
	}

	// Update fields if provided
	if name != "" {
		group.Name = name
	}
	if description != "" {
		group.Description = description
	}

	if err := h.groupService.UpdateGroup(group); err != nil {
		log.Printf("❌ Failed to update group %s: %v", groupID, err)
		return
	}

	log.Printf("✅ Updated group %s (ID: %s)", name, groupID)
}

// handleGroupDeleted handles group_deleted events
func (h *EventHandler) handleGroupDeleted(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for group_deleted event")
		return
	}

	groupID, _ := data["group_id"].(string)
	userID, _ := data["user_id"].(string)

	if groupID == "" || userID == "" {
		log.Printf("❌ Missing required fields in group_deleted event")
		return
	}

	if err := h.groupService.DeleteGroup(groupID, userID); err != nil {
		log.Printf("❌ Failed to delete group %s: %v", groupID, err)
		return
	}

	log.Printf("✅ Deleted group %s", groupID)
}

// handleMemberAdded handles member_added events
func (h *EventHandler) handleMemberAdded(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for member_added event")
		return
	}

	groupID, _ := data["group_id"].(string)
	userID, _ := data["user_id"].(string)
	role, _ := data["role"].(string)

	if groupID == "" || userID == "" {
		log.Printf("❌ Missing required fields in member_added event")
		return
	}

	// Default role to 'member' if not provided
	if role == "" {
		role = "member"
	}

	member := &models.GroupMember{
		GroupID:  groupID,
		UserID:   userID,
		Role:     role,
		JoinedAt: time.Now(),
	}

	if err := h.groupService.AddGroupMember(member); err != nil {
		log.Printf("❌ Failed to add member %s to group %s: %v", userID, groupID, err)
		return
	}

	log.Printf("✅ Added member %s to group %s with role %s", userID, groupID, role)
}

// handleMemberRemoved handles member_removed events
func (h *EventHandler) handleMemberRemoved(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for member_removed event")
		return
	}

	groupID, _ := data["group_id"].(string)
	userID, _ := data["user_id"].(string)

	if groupID == "" || userID == "" {
		log.Printf("❌ Missing required fields in member_removed event")
		return
	}

	if err := h.groupService.RemoveGroupMember(groupID, userID); err != nil {
		log.Printf("❌ Failed to remove member %s from group %s: %v", userID, groupID, err)
		return
	}

	log.Printf("✅ Removed member %s from group %s", userID, groupID)
}

// handleMemberRoleUpdated handles member_role_updated events
func (h *EventHandler) handleMemberRoleUpdated(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for member_role_updated event")
		return
	}

	groupID, _ := data["group_id"].(string)
	userID, _ := data["user_id"].(string)
	role, _ := data["role"].(string)

	if groupID == "" || userID == "" || role == "" {
		log.Printf("❌ Missing required fields in member_role_updated event")
		return
	}

	// First, get the existing member
	isMember, err := h.groupService.IsGroupMember(groupID, userID)
	if err != nil || !isMember {
		log.Printf("❌ User %s is not a member of group %s", userID, groupID)
		return
	}

	// Remove and re-add with new role
	if err := h.groupService.RemoveGroupMember(groupID, userID); err != nil {
		log.Printf("❌ Failed to update member role: %v", err)
		return
	}

	member := &models.GroupMember{
		GroupID:  groupID,
		UserID:   userID,
		Role:     role,
		JoinedAt: time.Now(),
	}

	if err := h.groupService.AddGroupMember(member); err != nil {
		log.Printf("❌ Failed to update member role: %v", err)
		return
	}

	log.Printf("✅ Updated role of member %s in group %s to %s", userID, groupID, role)
}

// handleInvitationCreated handles invitation_created events
func (h *EventHandler) handleInvitationCreated(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for invitation_created event")
		return
	}

	invitationID, _ := data["invitation_id"].(string)
	groupID, _ := data["group_id"].(string)
	userID, _ := data["user_id"].(string)

	if invitationID == "" || groupID == "" || userID == "" {
		log.Printf("❌ Missing required fields in invitation_created event")
		return
	}

	invitation := &models.GroupInvitation{
		ID:      invitationID,
		GroupID: groupID,
		UserID:  userID,
	}

	if err := h.groupService.CreateInvitation(invitation); err != nil {
		log.Printf("❌ Failed to create invitation: %v", err)
		return
	}

	log.Printf("✅ Created invitation %s for user %s to group %s",
		invitationID, userID, groupID)
}

// handleInvitationAccepted handles invitation_accepted events
func (h *EventHandler) handleInvitationAccepted(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for invitation_accepted event")
		return
	}

	invitationID, _ := data["invitation_id"].(string)
	userID, _ := data["user_id"].(string)

	if invitationID == "" || userID == "" {
		log.Printf("❌ Missing required fields in invitation_accepted event")
		return
	}

	// Get the invitation
	invitation, err := h.groupService.GetInvitation(invitationID)
	if err != nil {
		log.Printf("❌ Failed to get invitation %s: %v", invitationID, err)
		return
	}

	// Only the invited user can accept the invitation
	if invitation.UserID != userID {
		log.Printf("❌ User %s is not authorized to accept invitation %s", userID, invitationID)
		return
	}

	// Add user to the group as a member with default 'member' role
	member := &models.GroupMember{
		GroupID:  invitation.GroupID,
		UserID:   userID,
		Role:     "member", // Default role for new members
		JoinedAt: time.Now(),
	}

	if err := h.groupService.AddGroupMember(member); err != nil {
		log.Printf("❌ Failed to add user %s to group %s: %v", userID, invitation.GroupID, err)
		return
	}

	log.Printf("✅ User %s accepted invitation to group %s", userID, invitation.GroupID)
}

// handleInvitationRejected handles invitation_rejected events
func (h *EventHandler) handleInvitationRejected(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for invitation_rejected event")
		return
	}

	invitationID, _ := data["invitation_id"].(string)
	userID, _ := data["user_id"].(string)

	if invitationID == "" || userID == "" {
		log.Printf("❌ Missing required fields in invitation_rejected event")
		return
	}

	// Get the invitation
	invitation, err := h.groupService.GetInvitation(invitationID)
	if err != nil {
		log.Printf("❌ Failed to get invitation %s: %v", invitationID, err)
		return
	}

	// Only the invited user can reject the invitation
	if invitation.UserID != userID {
		log.Printf("❌ User %s is not authorized to reject invitation %s", userID, invitationID)
		return
	}

	log.Printf("✅ User %s rejected invitation to group %s", userID, invitation.GroupID)
}

// handleEventAddedToGroup handles event_added_to_group events
func (h *EventHandler) handleEventAddedToGroup(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for event_added_to_group event")
		return
	}

	groupID, _ := data["group_id"].(string)
	eventID, _ := data["event_id"].(string)
	addedBy, _ := data["added_by"].(string)

	if groupID == "" || eventID == "" || addedBy == "" {
		log.Printf("❌ Missing required fields in event_added_to_group event")
		return
	}

	// Check if the user has permission to add events to the group
	isMember, err := h.groupService.IsGroupMember(groupID, addedBy)
	if err != nil || !isMember {
		log.Printf("❌ User %s is not a member of group %s", addedBy, groupID)
		return
	}

	groupEvent := &models.GroupEvent{
		GroupID: groupID,
		EventID: eventID,
		AddedBy: addedBy,
		AddedAt: time.Now(),
	}

	if err := h.groupService.AddGroupEvent(groupEvent); err != nil {
		log.Printf("❌ Failed to add event %s to group %s: %v", eventID, groupID, err)
		return
	}

	log.Printf("✅ Added event %s to group %s", eventID, groupID)
}

// handleEventRemovedFromGroup handles event_removed_from_group events
func (h *EventHandler) handleEventRemovedFromGroup(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for event_removed_from_group event")
		return
	}

	groupID, _ := data["group_id"].(string)
	eventID, _ := data["event_id"].(string)
	removedBy, _ := data["removed_by"].(string)

	if groupID == "" || eventID == "" || removedBy == "" {
		log.Printf("❌ Missing required fields in event_removed_from_group event")
		return
	}

	// Check if the user has permission to remove events from the group
	isMember, err := h.groupService.IsGroupMember(groupID, removedBy)
	if err != nil || !isMember {
		log.Printf("❌ User %s is not a member of group %s", removedBy, groupID)
		return
	}

	if err := h.groupService.RemoveEventFromGroup(groupID, eventID); err != nil {
		log.Printf("❌ Failed to remove event %s from group %s: %v", eventID, groupID, err)
		return
	}

	log.Printf("✅ Removed event %s from group %s", eventID, groupID)
}

// StartListening starts listening for events on the specified channels
func (h *EventHandler) StartListening(redisClient *RedisClient, channels ...string) {
	for _, channel := range channels {
		go func(ch string) {
			if err := redisClient.Subscribe(ch, func(payload string) {
				h.HandleMessage(ch, payload)
			}); err != nil {
				log.Printf("❌ Failed to subscribe to channel %s: %v", ch, err)
			}
		}(channel)
	}
}
