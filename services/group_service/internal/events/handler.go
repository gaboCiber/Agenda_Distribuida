package events

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	case "list_group_events":
		h.handleListGroupEvents(event.Payload)
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
func (h *EventHandler) handleInvitationCreated(payload json.RawMessage) {
	// Parse the payload
	var data struct {
		InvitationID   string `json:"invitation_id"`
		GroupID        string `json:"group_id"`
		UserID         string `json:"user_id"`
		InvitedBy      string `json:"invited_by"`
		ResponseChannel string `json:"response_channel"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse invitation_created payload: %v", err)
		return
	}

	// Extract data from payload
	invitationID := data.InvitationID
	groupID := data.GroupID
	userID := data.UserID
	invitedBy := data.InvitedBy
	responseChannel := data.ResponseChannel

	// Validate required fields
	if invitationID == "" || groupID == "" || userID == "" || invitedBy == "" {
		errMsg := "❌ Missing required fields in invitation_created event"
		log.Println(errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_created_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	// Create the invitation
	invitation := &models.GroupInvitation{
		ID:        invitationID,
		GroupID:   groupID,
		UserID:    userID,
		InvitedBy: invitedBy,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	// Save the invitation
	if err := h.groupService.CreateInvitation(invitation); err != nil {
		errMsg := fmt.Sprintf("Failed to create invitation: %v", err)
		log.Printf("❌ %s", errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_created_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	log.Printf("✅ Created invitation %s for user %s to group %s", invitationID, userID, groupID)

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "invitation_created_response", map[string]interface{}{
			"status": "success",
			"invitation": map[string]interface{}{
				"id":         invitation.ID,
				"group_id":   invitation.GroupID,
				"user_id":    invitation.UserID,
				"invited_by": invitation.InvitedBy,
				"status":     invitation.Status,
				"created_at": invitation.CreatedAt,
			},
		})
	}

	// Publish notification event
	h.publisher.Publish("notifications", "invitation_created", map[string]interface{}{
		"invitation_id": invitation.ID,
		"group_id":      invitation.GroupID,
		"user_id":       invitation.UserID,
		"invited_by":    invitation.InvitedBy,
	})
}

// handleInvitationAccepted handles invitation_accepted events
func (h *EventHandler) handleInvitationAccepted(payload json.RawMessage) {
	// Parse the payload
	var data struct {
		InvitationID   string `json:"invitation_id"`
		UserID         string `json:"user_id"`
		ResponseChannel string `json:"response_channel"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse invitation_accepted payload: %v", err)
		return
	}

	// Extract data from payload
	invitationID := data.InvitationID
	userID := data.UserID
	responseChannel := data.ResponseChannel

	// Validate required fields
	if invitationID == "" || userID == "" {
		errMsg := "❌ Missing required fields in invitation_accepted event"
		log.Println(errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_accepted_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	// Get the invitation
	invitation, err := h.groupService.GetInvitation(invitationID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get invitation %s: %v", invitationID, err)
		log.Printf("❌ %s", errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_accepted_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	// Only the invited user can accept the invitation
	if invitation.UserID != userID {
		errMsg := fmt.Sprintf("User %s is not authorized to accept invitation %s", userID, invitationID)
		log.Printf("❌ %s", errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_accepted_response", map[string]interface{}{
				"status":  "forbidden",
				"message": errMsg,
			})
		}
		return
	}

	// Check if invitation is already processed
	if invitation.Status != "pending" {
		errMsg := fmt.Sprintf("Invitation %s is already %s", invitationID, invitation.Status)
		log.Printf("❌ %s", errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_accepted_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	// Start a transaction to ensure data consistency
	tx, err := h.groupService.BeginTx()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to start transaction: %v", err)
		log.Printf("❌ %s", errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_accepted_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	// Update invitation status to accepted
	invitation.Status = "accepted"
	invitation.RespondedAt = time.Now()
	if err := h.groupService.UpdateInvitation(invitation); err != nil {
		tx.Rollback()
		errMsg := fmt.Sprintf("Failed to update invitation status: %v", err)
		log.Printf("❌ %s", errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_accepted_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
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
		tx.Rollback()
		errMsg := fmt.Sprintf("Failed to add user %s to group %s: %v", userID, invitation.GroupID, err)
		log.Printf("❌ %s", errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_accepted_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		errMsg := fmt.Sprintf("Failed to commit transaction: %v", err)
		log.Printf("❌ %s", errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_accepted_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	log.Printf("✅ User %s accepted invitation to group %s", userID, invitation.GroupID)

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "invitation_accepted_response", map[string]interface{}{
			"status": "success",
			"member": map[string]interface{}{
				"group_id":  member.GroupID,
				"user_id":   member.UserID,
				"role":      member.Role,
				"joined_at": member.JoinedAt,
			},
		})
	}

	// Publish notification event
	h.publisher.Publish("notifications", "invitation_accepted", map[string]interface{}{
		"invitation_id": invitation.ID,
		"group_id":      invitation.GroupID,
		"user_id":       invitation.UserID,
	})
}

// handleInvitationRejected handles invitation_rejected events
func (h *EventHandler) handleInvitationRejected(payload json.RawMessage) {
	// Parse the payload
	var data struct {
		InvitationID   string `json:"invitation_id"`
		UserID         string `json:"user_id"`
		ResponseChannel string `json:"response_channel"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse invitation_rejected payload: %v", err)
		return
	}

	// Extract data from payload
	invitationID := data.InvitationID
	userID := data.UserID
	responseChannel := data.ResponseChannel

	// Validate required fields
	if invitationID == "" || userID == "" {
		errMsg := "❌ Missing required fields in invitation_rejected event"
		log.Println(errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_rejected_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	// Get the invitation
	invitation, err := h.groupService.GetInvitation(invitationID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get invitation %s: %v", invitationID, err)
		log.Printf("❌ %s", errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_rejected_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	// Only the invited user can reject the invitation
	if invitation.UserID != userID {
		errMsg := fmt.Sprintf("User %s is not authorized to reject invitation %s", userID, invitationID)
		log.Printf("❌ %s", errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_rejected_response", map[string]interface{}{
				"status":  "forbidden",
				"message": errMsg,
			})
		}
		return
	}

	// Check if invitation is already processed
	if invitation.Status != "pending" {
		errMsg := fmt.Sprintf("Invitation %s is already %s", invitationID, invitation.Status)
		log.Printf("❌ %s", errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_rejected_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	// Update invitation status to rejected
	invitation.Status = "rejected"
	invitation.RespondedAt = time.Now()
	if err := h.groupService.UpdateInvitation(invitation); err != nil {
		errMsg := fmt.Sprintf("Failed to update invitation status: %v", err)
		log.Printf("❌ %s", errMsg)
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "invitation_rejected_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	log.Printf("✅ User %s rejected invitation to group %s", userID, invitation.GroupID)

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "invitation_rejected_response", map[string]interface{}{
			"status": "success",
			"invitation": map[string]interface{}{
				"id":           invitation.ID,
				"group_id":     invitation.GroupID,
				"user_id":      invitation.UserID,
				"status":       invitation.Status,
				"responded_at": invitation.RespondedAt,
			},
		})
	}

	// Publish notification event
	h.publisher.Publish("notifications", "invitation_rejected", map[string]interface{}{
		"invitation_id": invitation.ID,
		"group_id":      invitation.GroupID,
		"user_id":       invitation.UserID,
	})
}

// handleEventAddedToGroup handles event_added_to_group events
func (h *EventHandler) handleEventAddedToGroup(payload json.RawMessage) {
	// Parse the payload
	var data struct {
		GroupID string `json:"group_id"`
		EventID string `json:"event_id"`
		AddedBy string `json:"added_by"`
		ResponseChannel string `json:"response_channel,omitempty"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse event_added_to_group payload: %v", err)
		return
	}

	if data.GroupID == "" || data.EventID == "" || data.AddedBy == "" {
		errMsg := "❌ Missing required fields in event_added_to_group event"
		log.Println(errMsg)
		if data.ResponseChannel != "" {
			h.publisher.Publish(data.ResponseChannel, "event_added_to_group_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	// Check if the user has permission to add events to the group
	isMember, err := h.groupService.IsGroupMember(data.GroupID, data.AddedBy)
	if err != nil || !isMember {
		errMsg := fmt.Sprintf("User %s is not a member of group %s", data.AddedBy, data.GroupID)
		log.Printf("❌ %s", errMsg)
		if data.ResponseChannel != "" {
			h.publisher.Publish(data.ResponseChannel, "event_added_to_group_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	groupEvent := &models.GroupEvent{
		GroupID: data.GroupID,
		EventID: data.EventID,
		AddedBy: data.AddedBy,
		AddedAt: time.Now(),
	}

	if err := h.groupService.AddGroupEvent(groupEvent); err != nil {
		errMsg := fmt.Sprintf("Failed to add event %s to group %s: %v", data.EventID, data.GroupID, err)
		log.Printf("❌ %s", errMsg)
		if data.ResponseChannel != "" {
			h.publisher.Publish(data.ResponseChannel, "event_added_to_group_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	log.Printf("✅ Added event %s to group %s", data.EventID, data.GroupID)

	// Publish success response if response channel is provided
	if data.ResponseChannel != "" {
		h.publisher.Publish(data.ResponseChannel, "event_added_to_group_response", map[string]interface{}{
			"status": "success",
			"event": map[string]interface{}{
				"group_id": groupEvent.GroupID,
				"event_id":  groupEvent.EventID,
				"added_by":  groupEvent.AddedBy,
				"added_at":  groupEvent.AddedAt,
			},
		})
	}

	// Publish notification event
	h.publisher.Publish("notifications", "event_added_to_group", map[string]interface{}{
		"group_id": groupEvent.GroupID,
		"event_id":  groupEvent.EventID,
		"added_by":  groupEvent.AddedBy,
	})
}

// handleEventRemovedFromGroup handles event_removed_from_group events
func (h *EventHandler) handleEventRemovedFromGroup(payload json.RawMessage) {
	// Parse the payload
	var data struct {
		GroupID    string `json:"group_id"`
		EventID    string `json:"event_id"`
		RemovedBy  string `json:"removed_by"`
		ResponseChannel string `json:"response_channel,omitempty"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse event_removed_from_group payload: %v", err)
		return
	}

	if data.GroupID == "" || data.EventID == "" || data.RemovedBy == "" {
		errMsg := "❌ Missing required fields in event_removed_from_group event"
		log.Println(errMsg)
		if data.ResponseChannel != "" {
			h.publisher.Publish(data.ResponseChannel, "event_removed_from_group_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	// Check if the user has permission to remove events from the group
	isMember, err := h.groupService.IsGroupMember(data.GroupID, data.RemovedBy)
	if err != nil || !isMember {
		errMsg := fmt.Sprintf("User %s is not a member of group %s", data.RemovedBy, data.GroupID)
		log.Printf("❌ %s", errMsg)
		if data.ResponseChannel != "" {
			h.publisher.Publish(data.ResponseChannel, "event_removed_from_group_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	if err := h.groupService.RemoveEventFromGroup(data.GroupID, data.EventID); err != nil {
		errMsg := fmt.Sprintf("Failed to remove event %s from group %s: %v", data.EventID, data.GroupID, err)
		log.Printf("❌ %s", errMsg)
		if data.ResponseChannel != "" {
			h.publisher.Publish(data.ResponseChannel, "event_removed_from_group_response", map[string]interface{}{
				"status":  "error",
				"message": errMsg,
			})
		}
		return
	}

	log.Printf("✅ Removed event %s from group %s", data.EventID, data.GroupID)

	// Publish success response if response channel is provided
	if data.ResponseChannel != "" {
		h.publisher.Publish(data.ResponseChannel, "event_removed_from_group_response", map[string]interface{}{
			"status": "success",
			"event": map[string]interface{}{
				"group_id": data.GroupID,
				"event_id":  data.EventID,
			},
		})
	}

	// Publish notification event
	h.publisher.Publish("notifications", "event_removed_from_group", map[string]interface{}{
		"group_id": data.GroupID,
		"event_id":  data.EventID,
		"removed_by": data.RemovedBy,
	})
}

// handleListGroupEvents handles list_group_events events
func (h *EventHandler) handleListGroupEvents(payload json.RawMessage) {
	// Parse the payload
	var data struct {
		GroupID         string `json:"group_id"`
		UserID          string `json:"user_id"`
		ResponseChannel string `json:"response_channel"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse list_group_events payload: %v", err)
		return
	}

	if data.GroupID == "" || data.UserID == "" || data.ResponseChannel == "" {
		log.Printf("❌ Missing required fields in list_group_events event")
		return
	}

	// Check if the user is a member of the group
	isMember, err := h.groupService.IsGroupMember(data.GroupID, data.UserID)
	if err != nil || !isMember {
		errMsg := fmt.Sprintf("User %s is not a member of group %s", data.UserID, data.GroupID)
		log.Printf("❌ %s", errMsg)
		h.publisher.Publish(data.ResponseChannel, "list_group_events_response", map[string]interface{}{
			"status":  "error",
			"message": errMsg,
		})
		return
	}

	// Get the group events
	events, err := h.groupService.GetGroupEvents(data.GroupID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get events for group %s: %v", data.GroupID, err)
		log.Printf("❌ %s", errMsg)
		h.publisher.Publish(data.ResponseChannel, "list_group_events_response", map[string]interface{}{
			"status":  "error",
			"message": errMsg,
		})
		return
	}

	// Convert events to a slice of maps for JSON serialization
	eventList := make([]map[string]interface{}, len(events))
	for i, event := range events {
		eventList[i] = map[string]interface{}{
			"event_id": event.EventID,
			"group_id": event.GroupID,
			"added_by": event.AddedBy,
			"added_at": event.AddedAt,
		}
	}

	// Publish the response
	h.publisher.Publish(data.ResponseChannel, "list_group_events_response", map[string]interface{}{
		"status": "success",
		"events": eventList,
	})

	log.Printf("✅ Listed %d events for group %s", len(events), data.GroupID)
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
