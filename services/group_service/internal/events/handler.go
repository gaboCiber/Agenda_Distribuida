package events

import (
	"database/sql"
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
	log.Printf("Received message on %s: %s", channel, payload)

	// Parse the message to get the event type
	var event struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}

	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("Failed to parse message: %v", err)
		return
	}

	log.Printf("Processing event: %s", event.Type)

	// Route the event to the appropriate handler
	switch event.Type {
	case "group_created":
		h.handleGroupCreated(event.Payload)
	case "group_updated":
		h.handleGroupUpdated(event.Payload)
	case "group_deleted":
		h.handleGroupDeleted(event.Payload)
	case "get_group":
		h.handleGetGroup(event.Payload)
	case "list_user_groups":
		h.handleListUserGroups(event.Payload)
	case "user_deleted":
		h.handleUserDeleted(event.Payload)
	case "event_deleted":
		h.handleEventDeleted(event.Payload)
	case "member_added":
		// Parse the payload into a map for the member handlers
		var payloadMap map[string]interface{}
		if err := json.Unmarshal(event.Payload, &payloadMap); err != nil {
			log.Printf("Failed to parse member_added payload: %v", err)
			return
		}
		h.handleMemberAdded(payloadMap)
	case "member_removed":
		// Parse the payload into a map for the member handlers
		var payloadMap map[string]interface{}
		if err := json.Unmarshal(event.Payload, &payloadMap); err != nil {
			log.Printf("Failed to parse member_removed payload: %v", err)
			return
		}
		h.handleMemberRemoved(payloadMap)
	case "member_role_updated":
		// Parse the payload into a map for the member handlers
		var payloadMap map[string]interface{}
		if err := json.Unmarshal(event.Payload, &payloadMap); err != nil {
			log.Printf("Failed to parse member_role_updated payload: %v", err)
			return
		}
		h.handleMemberRoleUpdated(payloadMap)
	case "list_members":
		h.handleListMembers(event.Payload)
	case "get_group_admins":
		h.handleGetGroupAdmins(event.Payload)
	case "invitation_created":
		h.handleInvitationCreated(event.Payload)
	case "invitation_accepted":
		h.handleInvitationAccepted(event.Payload)
	case "invitation_rejected":
		h.handleInvitationRejected(event.Payload)
	case "event_added_to_group":
		h.handleEventAddedToGroup(event.Payload)
	case "event_removed_from_group":
		h.handleEventRemovedFromGroup(event.Payload)
	default:
		log.Printf("⚠️ Unhandled event type: %s", event.Type)
	}
}

// handleGetGroup handles get_group events
func (h *EventHandler) handleGetGroup(payload json.RawMessage) {
	// Parse payload to map
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse get_group payload: %v", err)
		return
	}

	// Extract group data
	groupID, _ := data["group_id"].(string)
	responseChannel, _ := data["response_channel"].(string)

	if groupID == "" {
		log.Printf("❌ Missing group_id in get_group event")
		return
	}

	// Get the group from the service
	group, err := h.groupService.GetGroup(groupID)
	if err != nil {
		log.Printf("❌ Failed to get group %s: %v", groupID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "get_group_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to get group",
				"error":   err.Error(),
			})
		}
		return
	}

	if group == nil {
		log.Printf("❌ Group %s not found", groupID)
		// Publish not found response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "get_group_response", map[string]interface{}{
				"status":  "not_found",
				"message": "Group not found",
			})
		}
		return
	}

	// Get member count
	members, err := h.groupService.GetGroupMembers(groupID)
	if err != nil {
		log.Printf("❌ Failed to get group members for group %s: %v", groupID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "get_group_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to get group members",
				"error":   err.Error(),
			})
		}
		return
	}

	// Prepare the group response
	groupResponse := map[string]interface{}{
		"id":           group.ID,
		"name":         group.Name,
		"description":  group.Description,
		"created_by":   group.CreatedBy,
		"created_at":   group.CreatedAt,
		"updated_at":   group.UpdatedAt,
		"member_count": len(members),
	}

	log.Printf("✅ Retrieved group %s (ID: %s)", group.Name, groupID)

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "get_group_response", map[string]interface{}{
			"status": "success",
			"group":  groupResponse,
		})
	}
}

// handleListUserGroups handles list_user_groups events
func (h *EventHandler) handleListUserGroups(payload json.RawMessage) {
	// Parse payload to map
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse list_user_groups payload: %v", err)
		return
	}

	// Extract user data
	userID, _ := data["user_id"].(string)
	responseChannel, _ := data["response_channel"].(string)

	if userID == "" {
		log.Printf("❌ Missing user_id in list_user_groups event")
		return
	}

	// Get user's groups from the service
	groups, err := h.groupService.ListUserGroups(userID)
	if err != nil {
		log.Printf("❌ Failed to list groups for user %s: %v", userID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "list_user_groups_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to list user groups",
				"error":   err.Error(),
			})
		}
		return
	}

	// Prepare the groups response
	groupsResponse := make([]map[string]interface{}, 0, len(groups))
	for _, group := range groups {
		// Get member count for each group
		members, err := h.groupService.GetGroupMembers(group.ID)
		memberCount := 0
		if err == nil {
			memberCount = len(members)
		}

		groupData := map[string]interface{}{
			"id":           group.ID,
			"name":         group.Name,
			"description":  group.Description,
			"created_by":   group.CreatedBy,
			"created_at":   group.CreatedAt,
			"updated_at":   group.UpdatedAt,
			"member_count": memberCount,
		}
		groupsResponse = append(groupsResponse, groupData)
	}

	log.Printf("✅ Retrieved %d groups for user %s", len(groups), userID)

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "list_user_groups_response", map[string]interface{}{
			"status": "success",
			"groups": groupsResponse,
		})
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
func (h *EventHandler) handleGroupCreated(payload json.RawMessage) {
	// Parse payload to map
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse group_created payload: %v", err)
		return
	}

	// Skip if this is a system-generated event (to prevent loops)
	if source, ok := data["source"].(string); ok && source == "group-service" {
		log.Printf("ℹ️  Ignoring system-generated group_created event")
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
func (h *EventHandler) handleGroupUpdated(payload json.RawMessage) {
	// Parse payload to map
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse group_updated payload: %v", err)
		return
	}

	// Skip if this is a system-generated event (to prevent loops)
	if source, ok := data["source"].(string); ok && source == "group-service" {
		log.Printf("ℹ️  Ignoring system-generated group_updated event")
		return
	}

	// Extract group data
	groupID, _ := data["group_id"].(string)
	name, _ := data["name"].(string)
	description, _ := data["description"].(string)
	updatedBy, _ := data["updated_by"].(string)
	responseChannel, _ := data["response_channel"].(string)

	if groupID == "" || name == "" || updatedBy == "" {
		log.Printf("❌ Missing required fields in group_updated event")
		return
	}

	// Get the existing group
	existingGroup, err := h.groupService.GetGroup(groupID)
	if err != nil {
		log.Printf("❌ Failed to get group %s: %v", groupID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "group_updated_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to get group",
				"error":   err.Error(),
			})
		}
		return
	}

	if existingGroup == nil {
		log.Printf("❌ Group %s not found", groupID)
		// Publish not found response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "group_updated_response", map[string]interface{}{
				"status":  "not_found",
				"message": "Group not found",
			})
		}
		return
	}

	// Update group fields
	existingGroup.Name = name
	existingGroup.Description = description
	existingGroup.UpdatedAt = time.Now()

	// Update the group in the database
	if err := h.groupService.UpdateGroup(existingGroup); err != nil {
		log.Printf("❌ Failed to update group %s: %v", groupID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "group_updated_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to update group",
				"error":   err.Error(),
			})
		}
		return
	}

	log.Printf("✅ Successfully updated group %s (ID: %s) updated by %s",
		name, groupID, updatedBy)

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "group_updated_response", map[string]interface{}{
			"status": "success",
			"payload": map[string]interface{}{
				"group_id":    groupID,
				"name":        name,
				"description": description,
				"updated_by":  updatedBy,
				"updated_at":  existingGroup.UpdatedAt,
			},
		})
	}

	// Publish group updated event with source marker
	h.publisher.Publish("groups", "group_updated", map[string]interface{}{
		"group_id":    groupID,
		"name":        name,
		"description": description,
		"updated_by":  updatedBy,
		"updated_at":  existingGroup.UpdatedAt,
		"source":      "group-service", // Mark as system-generated
	})
}

// handleGroupDeleted handles group_deleted events
func (h *EventHandler) handleGroupDeleted(payload json.RawMessage) {
	// Parse payload to map
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse group_deleted payload: %v", err)
		return
	}

	// Skip if this is a system-generated event (to prevent loops)
	if source, ok := data["source"].(string); ok && source == "group-service" {
		log.Printf("ℹ️  Ignoring system-generated group_deleted event")
		return
	}

	// Extract group data
	groupID, _ := data["group_id"].(string)
	deletedBy, _ := data["deleted_by"].(string)
	responseChannel, _ := data["response_channel"].(string)

	if groupID == "" || deletedBy == "" {
		log.Printf("❌ Missing required fields in group_deleted event")
		return
	}

	// Get the group before deleting it (for the response)
	group, err := h.groupService.GetGroup(groupID)
	if err != nil {
		log.Printf("❌ Failed to get group %s: %v", groupID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "group_deleted_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to get group",
				"error":   err.Error(),
			})
		}
		return
	}

	if group == nil {
		log.Printf("❌ Group %s not found", groupID)
		// Publish not found response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "group_deleted_response", map[string]interface{}{
				"status":  "not_found",
				"message": "Group not found",
			})
		}
		return
	}

	// Delete the group
	if err := h.groupService.DeleteGroup(groupID, deletedBy); err != nil {
		log.Printf("❌ Failed to delete group %s: %v", groupID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			status := "error"
			message := "Failed to delete group"
			if err == sql.ErrNoRows {
				status = "not_found"
				message = "Group not found"
			}
			h.publisher.Publish(responseChannel, "group_deleted_response", map[string]interface{}{
				"status":  status,
				"message": message,
				"error":   err.Error(),
			})
		}
		return
	}

	log.Printf("✅ Successfully deleted group %s (ID: %s) by user %s",
		group.Name, groupID, deletedBy)

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "group_deleted_response", map[string]interface{}{
			"status": "success",
			"payload": map[string]interface{}{
				"group_id":   groupID,
				"name":       group.Name,
				"deleted_by": deletedBy,
				"deleted_at": time.Now(),
			},
		})
	}

	// Publish group deleted event with source marker
	h.publisher.Publish("groups", "group_deleted", map[string]interface{}{
		"group_id":   groupID,
		"name":       group.Name,
		"deleted_by": deletedBy,
		"deleted_at": time.Now(),
		"source":     "group-service", // Mark as system-generated
	})
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
	// Create a channel to receive messages
	msgChan := make(chan string)

	// Subscribe to each channel in a separate goroutine
	for _, channel := range channels {
		go func(ch string) {
			err := redisClient.Subscribe(ch, func(payload string) {
				msgChan <- payload
			})
			if err != nil {
				log.Printf("❌ Failed to subscribe to channel %s: %v", ch, err)
			}
		}(channel)
	}

	log.Printf("🚀 Started listening for events on channels: %v", channels)

	// Handle incoming messages
	for payload := range msgChan {
		// Parse the message to get the event type
		var msg struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}

		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			log.Printf("❌ Failed to parse message: %v", err)
			continue
		}

		// Use HandleMessage to process the message (this will handle all event types)
		go h.HandleMessage("groups", payload)
	}
}
