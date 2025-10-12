package events

import (
	"encoding/json"
	"log"
	"time"

	"github.com/agenda-distribuida/group-service/internal/models"
)

// handleMemberAdded handles member_added events
func (h *EventHandler) handleMemberAdded(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for member_added event")
		return
	}

	// Skip if this is a system-generated event (to prevent loops)
	if source, ok := data["source"].(string); ok && source == "group-service" {
		log.Printf("ℹ️  Ignoring system-generated member_added event")
		return
	}

	// Extract member data
	groupID, _ := data["group_id"].(string)
	userID, _ := data["user_id"].(string)
	role, _ := data["role"].(string)
	responseChannel, _ := data["response_channel"].(string)

	if groupID == "" || userID == "" {
		log.Printf("❌ Missing required fields in member_added event")
		return
	}

	// Set default role if not provided
	if role == "" {
		role = "member"
	}

	// Create the group member
	member := &models.GroupMember{
		ID:       userID, // Usamos el userID como ID del miembro por simplicidad
		GroupID:  groupID,
		UserID:   userID,
		Role:     role,
		JoinedAt: time.Now().UTC(),
	}

	// Add the member to the group
	if err := h.groupService.AddGroupMember(member); err != nil {
		log.Printf("❌ Failed to add member %s to group %s: %v", userID, groupID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "member_added_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to add member to group",
				"error":   err.Error(),
			})
		}
		return
	}

	log.Printf("✅ Successfully added member %s to group %s with role %s", userID, groupID, role)

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "member_added_response", map[string]interface{}{
			"status": "success",
			"payload": map[string]interface{}{
				"group_id": groupID,
				"user_id":  userID,
				"role":     role,
			},
		})
	}

	// Publish member_added event with source marker
	h.publisher.Publish("groups", "member_added", map[string]interface{}{
		"group_id": groupID,
		"user_id":  userID,
		"role":     role,
		"source":   "group-service", // Mark as system-generated
	})
}

// handleMemberRemoved handles member_removed events
func (h *EventHandler) handleMemberRemoved(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for member_removed event")
		return
	}

	// Skip if this is a system-generated event (to prevent loops)
	if source, ok := data["source"].(string); ok && source == "group-service" {
		log.Printf("ℹ️  Ignoring system-generated member_removed event")
		return
	}

	// Extract member data
	groupID, _ := data["group_id"].(string)
	userID, _ := data["user_id"].(string)
	removedBy, _ := data["removed_by"].(string)
	responseChannel, _ := data["response_channel"].(string)

	if groupID == "" || userID == "" {
		log.Printf("❌ Missing required fields in member_removed event")
		return
	}

	// Get member before removing to include in the response
	member, err := h.groupService.GetGroupMember(groupID, userID)
	if err != nil {
		log.Printf("❌ Failed to get member %s from group %s: %v", userID, groupID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "member_removed_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to get member",
				"error":   err.Error(),
			})
		}
		return
	}

	if member == nil {
		log.Printf("❌ Member %s not found in group %s", userID, groupID)
		// Publish not found response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "member_removed_response", map[string]interface{}{
				"status":  "not_found",
				"message": "Member not found in group",
			})
		}
		return
	}

	// Remove the member from the group
	if err := h.groupService.RemoveGroupMember(groupID, userID); err != nil {
		log.Printf("❌ Failed to remove member %s from group %s: %v", userID, groupID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "member_removed_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to remove member from group",
				"error":   err.Error(),
			})
		}
		return
	}

	log.Printf("✅ Successfully removed member %s from group %s", userID, groupID)

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "member_removed_response", map[string]interface{}{
			"status": "success",
			"payload": map[string]interface{}{
				"group_id":   groupID,
				"user_id":    userID,
				"removed_by": removedBy,
				"removed_at": time.Now().Format(time.RFC3339),
			},
		})
	}

	// Publish member_removed event with source marker
	h.publisher.Publish("groups", "member_removed", map[string]interface{}{
		"group_id":   groupID,
		"user_id":    userID,
		"removed_by": removedBy,
		"removed_at": time.Now().Format(time.RFC3339),
		"source":     "group-service", // Mark as system-generated
	})
}

// handleListMembers handles list_members events
func (h *EventHandler) handleListMembers(payload json.RawMessage) {
	// Parse payload to map
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse list_members payload: %v", err)
		return
	}

	// Extract group data
	groupID, _ := data["group_id"].(string)
	responseChannel, _ := data["response_channel"].(string)

	if groupID == "" {
		log.Printf("❌ Missing group_id in list_members event")
		return
	}

	// Get the members from the service
	members, err := h.groupService.GetGroupMembers(groupID)
	if err != nil {
		log.Printf("❌ Failed to get members for group %s: %v", groupID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "list_members_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to retrieve group members",
				"error":   err.Error(),
			})
		}
		return
	}

	log.Printf("✅ Retrieved %d members for group %s", len(members), groupID)

	// Convert members to response format
	membersResponse := make([]map[string]interface{}, 0, len(members))
	for _, m := range members {
		membersResponse = append(membersResponse, map[string]interface{}{
			"user_id":   m.UserID,
			"role":      m.Role,
			"joined_at": m.JoinedAt.Format(time.RFC3339),
		})
	}

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "list_members_response", map[string]interface{}{
			"status": "success",
			"payload": map[string]interface{}{
				"group_id": groupID,
				"members":  membersResponse,
				"count":    len(membersResponse),
			},
		})
	}
}

// handleMemberRoleUpdated handles member_role_updated events
func (h *EventHandler) handleMemberRoleUpdated(payload interface{}) {
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for member_role_updated event")
		return
	}

	// Skip if this is a system-generated event (to prevent loops)
	if source, ok := data["source"].(string); ok && source == "group-service" {
		log.Printf("ℹ️  Ignoring system-generated member_role_updated event")
		return
	}

	// Extract member data
	groupID, _ := data["group_id"].(string)
	userID, _ := data["user_id"].(string)
	newRole, _ := data["role"].(string)
	updatedBy, _ := data["updated_by"].(string)
	responseChannel, _ := data["response_channel"].(string)

	if groupID == "" || userID == "" || newRole == "" {
		log.Printf("❌ Missing required fields in member_role_updated event")
		return
	}

	// Get the existing member
	member, err := h.groupService.GetGroupMember(groupID, userID)
	if err != nil {
		log.Printf("❌ Failed to get member %s from group %s: %v", userID, groupID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "member_role_updated_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to get member",
				"error":   err.Error(),
			})
		}
		return
	}

	if member == nil {
		log.Printf("❌ Member %s not found in group %s", userID, groupID)
		// Publish not found response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "member_role_updated_response", map[string]interface{}{
				"status":  "not_found",
				"message": "Member not found in group",
			})
		}
		return
	}

	// Update the member's role
	member.Role = newRole

	// In a real implementation, you would update the member in the database
	// For now, we'll remove and re-add the member with the new role
	if err := h.groupService.RemoveGroupMember(groupID, userID); err != nil {
		log.Printf("❌ Failed to update member role: %v", err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "member_role_updated_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to update member role",
				"error":   err.Error(),
			})
		}
		return
	}

	if err := h.groupService.AddGroupMember(member); err != nil {
		log.Printf("❌ Failed to update member role: %v", err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "member_role_updated_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to update member role",
				"error":   err.Error(),
			})
		}
		return
	}

	log.Printf("✅ Updated role of member %s in group %s to %s", userID, groupID, newRole)

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "member_role_updated_response", map[string]interface{}{
			"status": "success",
			"payload": map[string]interface{}{
				"group_id":   groupID,
				"user_id":    userID,
				"role":       newRole,
				"updated_by": updatedBy,
				"updated_at": time.Now().Format(time.RFC3339),
			},
		})
	}

	// Publish member_role_updated event with source marker
	h.publisher.Publish("groups", "member_role_updated", map[string]interface{}{
		"group_id":   groupID,
		"user_id":    userID,
		"role":       newRole,
		"updated_by": updatedBy,
		"updated_at": time.Now().Format(time.RFC3339),
		"source":     "group-service", // Mark as system-generated
	})
}

// handleGetGroupAdmins handles get_group_admins events
func (h *EventHandler) handleGetGroupAdmins(payload json.RawMessage) {
	// Parse payload to map
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse get_group_admins payload: %v", err)
		return
	}

	// Extract group data
	groupID, _ := data["group_id"].(string)
	responseChannel, _ := data["response_channel"].(string)

	if groupID == "" {
		log.Printf("❌ Missing group_id in get_group_admins event")
		return
	}

	// Get the admins from the service
	admins, err := h.groupService.GetGroupAdmins(groupID)
	if err != nil {
		log.Printf("❌ Failed to get admins for group %s: %v", groupID, err)
		// Publish error response if response channel is provided
		if responseChannel != "" {
			h.publisher.Publish(responseChannel, "get_group_admins_response", map[string]interface{}{
				"status":  "error",
				"message": "Failed to retrieve group admins",
				"error":   err.Error(),
			})
		}
		return
	}

	log.Printf("✅ Retrieved %d admins for group %s", len(admins), groupID)

	// Convert admins to response format
	adminsResponse := make([]map[string]interface{}, 0, len(admins))
	for _, a := range admins {
		adminsResponse = append(adminsResponse, map[string]interface{}{
			"user_id":   a.UserID,
			"joined_at": a.JoinedAt.Format(time.RFC3339),
		})
	}

	// Publish success response if response channel is provided
	if responseChannel != "" {
		h.publisher.Publish(responseChannel, "get_group_admins_response", map[string]interface{}{
			"status": "success",
			"payload": map[string]interface{}{
				"group_id": groupID,
				"admins":   adminsResponse,
				"count":    len(adminsResponse),
			},
		})
	}
}
