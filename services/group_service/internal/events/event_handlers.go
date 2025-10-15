package events

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/agenda-distribuida/group-service/internal/service"
	"github.com/google/uuid"
)

// GroupEventHandler handles group event operations
type GroupEventHandler struct {
	groupService service.GroupService
	publisher    *Publisher
}

// NewGroupEventHandler creates a new group event handler
func NewGroupEventHandler(groupService service.GroupService, publisher *Publisher) *GroupEventHandler {
	return &GroupEventHandler{
		groupService: groupService,
		publisher:    publisher,
	}
}

// HandleEventAdded handles when an event is added to a group
func (h *GroupEventHandler) HandleEventAdded(payload json.RawMessage) {
	var eventData struct {
		EventID     string `json:"event_id"`
		GroupID     string `json:"group_id"`
		AddedBy     string `json:"added_by"`
		IsRecursive bool   `json:"is_recursive,omitempty"`
	}

	if err := json.Unmarshal(payload, &eventData); err != nil {
		log.Printf("Failed to unmarshal event added data: %v", err)
		return
	}

	// Get the group to check if it's hierarchical
	group, err := h.groupService.GetGroup(eventData.GroupID)
	if err != nil {
		log.Printf("Error getting group: %v", err)
		return
	}

	// Get all members of the group (including sub-groups if hierarchical)
	members, err := h.getGroupMembers(eventData.GroupID, group.IsHierarchical)
	if err != nil {
		log.Printf("Error getting group members: %v", err)
		return
	}

	// Add event status for each member
	now := time.Now().UTC()
	for _, member := range members {
		status := "pending"
		if group.IsHierarchical {
			// Auto-accept for hierarchical groups
			status = "accepted"
		}

		eventStatus := &models.GroupEventStatus{
			ID:          uuid.New().String(),
			GroupID:     eventData.GroupID,  // Asegurar que GroupID esté establecido
			EventID:     eventData.EventID,
			UserID:      member.UserID,
			Status:      models.EventStatus(status),
			RespondedAt: &now,
			CreatedAt:   now,
		}

		// Add event status for each member
		if err := h.groupService.UpdateEventStatus(eventStatus); err != nil {
			log.Printf("Error adding event status for user %s: %v", member.UserID, err)
		}
	}

	// If this is a hierarchical group and recursive is true, add to all sub-groups
	if group.IsHierarchical && eventData.IsRecursive {
		h.propagateEventToSubgroups(eventData.GroupID, eventData.EventID, eventData.AddedBy)
	}
}

// HandleEventStatusUpdate handles when a user updates their status for an event
func (h *GroupEventHandler) HandleEventStatusUpdate(payload json.RawMessage) {
	var statusData struct {
		EventID string `json:"event_id"`
		UserID  string `json:"user_id"`
		Status  string `json:"status"`
	}

	if err := json.Unmarshal(payload, &statusData); err != nil {
		log.Printf("Failed to unmarshal event status update: %v", err)
		return
	}

	// Get the event status to find the group and update it
	eventStatus, err := h.groupService.GetEventStatus(statusData.EventID, statusData.UserID)
	if err != nil {
		log.Printf("Error getting event status: %v", err)
		return
	}

	if eventStatus == nil {
		log.Printf("Event status not found for event %s and user %s", statusData.EventID, statusData.UserID)
		return
	}

	// Get the group to check if it's hierarchical
	group, err := h.groupService.GetGroup(eventStatus.GroupID)
	if err != nil {
		log.Printf("Error getting group: %v", err)
		return
	}

	// For hierarchical groups, status is always accepted and can't be changed
	if group.IsHierarchical {
		log.Printf("⚠️  Cannot change status in hierarchical group %s - status is always accepted", group.ID)
		return
	}

	// Update the status for non-hierarchical groups
	now := time.Now().UTC()
	eventStatus.Status = models.EventStatus(statusData.Status)
	eventStatus.RespondedAt = &now

	// Update the event status in the database
	if err := h.groupService.UpdateEventStatus(eventStatus); err != nil {
		log.Printf("Error updating event status: %v", err)
		return
	}

	// For non-hierarchical groups, check if all members have accepted
	if !group.IsHierarchical {
		allAccepted, err := h.groupService.HasAllMembersAccepted(eventStatus.GroupID, statusData.EventID)
		if err != nil {
			log.Printf("Error checking if all members have accepted: %v", err)
			return
		}

		if allAccepted {
			// Update the group event status to accepted
			if err := h.groupService.UpdateGroupEventStatus(eventStatus.GroupID, statusData.EventID, "accepted"); err != nil {
				log.Printf("Error updating group event status: %v", err)
				return
			}

			// Publish event status updated event
			h.publisher.Publish("event_status_updated", "group_event_accepted", map[string]interface{}{
				"event_id": statusData.EventID,
				"group_id": eventStatus.GroupID,
				"status":   "accepted",
			})
		}
	} else {
		// For hierarchical groups, propagate the status update to sub-groups
		h.propagateEventStatusToSubgroups(group.ID, statusData.EventID, statusData.UserID, statusData.Status)
	}

	// Publish user's status update
	h.publisher.Publish("event_status_updated", "user_status_updated", map[string]interface{}{
		"event_id": statusData.EventID,
		"user_id":  statusData.UserID,
		"group_id": eventStatus.GroupID,
		"status":   statusData.Status,
	})
}

// getGroupMembers gets all members of a group, including sub-groups if hierarchical
func (h *GroupEventHandler) getGroupMembers(groupID string, includeSubgroups bool) ([]*models.GroupMember, error) {
	var members []*models.GroupMember

	// Get direct members of the group
	directMembers, err := h.groupService.GetGroupMembers(groupID)
	if err != nil {
		return nil, err
	}
	members = append(members, directMembers...)

	// If hierarchical, get members from all sub-groups
	if includeSubgroups {
		subgroups, err := h.groupService.GetSubGroups(groupID)
		if err != nil {
			return nil, err
		}

		for _, subgroup := range subgroups {
			subgroupMembers, err := h.getGroupMembers(subgroup.ID, true)
			if err != nil {
				return nil, err
			}
			members = append(members, subgroupMembers...)
		}
	}

	// Remove duplicates
	uniqueMembers := make(map[string]*models.GroupMember)
	for _, member := range members {
		if member != nil {
			uniqueMembers[member.UserID] = member
		}
	}

	result := make([]*models.GroupMember, 0, len(uniqueMembers))
	for _, member := range uniqueMembers {
		result = append(result, member)
	}

	return result, nil
}

// HandleEventCreated handles when a new event is created
func (h *GroupEventHandler) HandleEventCreated(payload json.RawMessage) {
	var eventData struct {
		EventID     string    `json:"event_id"`
		GroupID     string    `json:"group_id"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		StartTime   time.Time `json:"start_time"`
		EndTime     time.Time `json:"end_time"`
		CreatedBy   string    `json:"created_by"`
		IsHierarchical bool    `json:"is_hierarchical,omitempty"`
	}

	if err := json.Unmarshal(payload, &eventData); err != nil {
		log.Printf("Failed to unmarshal event created data: %v", err)
		return
	}

	// Get the group to check if it's hierarchical
	group, err := h.groupService.GetGroup(eventData.GroupID)
	if err != nil {
		log.Printf("Error getting group: %v", err)
		return
	}

	// Get all members of the group (including sub-groups if hierarchical)
	members, err := h.getGroupMembers(eventData.GroupID, group.IsHierarchical)
	if err != nil {
		log.Printf("Error getting group members: %v", err)
		return
	}

	// Create the group event
	groupEvent := &models.GroupEvent{
		ID:             uuid.New().String(),
		EventID:        eventData.EventID,
		GroupID:        eventData.GroupID,
		AddedBy:        eventData.CreatedBy,
		IsHierarchical: group.IsHierarchical,
		Status:         "pending", // Default status
		AddedAt:        time.Now().UTC(),
	}

	// For hierarchical groups, set status to accepted immediately
	if group.IsHierarchical {
		groupEvent.Status = "accepted"
	}

	// Add the event to the group
	if err := h.groupService.AddGroupEvent(groupEvent); err != nil {
		log.Printf("Failed to add event to group: %v", err)
		return
	}

	log.Printf("✅ Added event %s to group %s", eventData.EventID, eventData.GroupID)

	// Add event status for each member
	now := time.Now().UTC()
	for _, member := range members {
		status := models.EventStatusPending
		respondedAt := (*time.Time)(nil)
		
		if group.IsHierarchical {
			// Auto-accept for hierarchical groups
			status = models.EventStatusAccepted
			tempNow := now
			respondedAt = &tempNow
		}

		eventStatus := &models.GroupEventStatus{
			ID:          uuid.New().String(),
			EventID:     eventData.EventID,
			GroupID:     eventData.GroupID,
			UserID:      member.UserID,
			Status:      status,
			RespondedAt: respondedAt,
			CreatedAt:   now,
		}

		// Add event status for each member
		if err := h.groupService.UpdateEventStatus(eventStatus); err != nil {
			log.Printf("Error adding event status for user %s: %v", member.UserID, err)
		} else {
			log.Printf("✅ Added event status for user %s: %s", member.UserID, status)
		}
	}

	// If hierarchical, propagate to subgroups
	if group.IsHierarchical {
		h.propagateEventToSubgroups(eventData.GroupID, eventData.EventID, eventData.CreatedBy)
	}
}

// propagateEventToSubgroups adds an event to all sub-groups of a group
func (h *GroupEventHandler) propagateEventToSubgroups(parentGroupID, eventID, addedBy string) {
	subgroups, err := h.groupService.GetSubGroups(parentGroupID)
	if err != nil {
		log.Printf("Error getting subgroups: %v", err)
		return
	}

	for _, subgroup := range subgroups {
		// Add event to subgroup
		groupEvent := &models.GroupEvent{
			ID:      uuid.New().String(),
			GroupID: subgroup.ID,
			EventID: eventID,
			AddedBy: addedBy,
			AddedAt: time.Now().UTC(),
		}

		if err := h.groupService.AddGroupEvent(groupEvent); err != nil {
			log.Printf("Error adding event to subgroup %s: %v", subgroup.ID, err)
		}

		// Recursively add to sub-subgroups
		h.propagateEventToSubgroups(subgroup.ID, eventID, addedBy)
	}
}

// propagateEventStatusToSubgroups propagates an event status update to all sub-groups
func (h *GroupEventHandler) propagateEventStatusToSubgroups(parentGroupID, eventID, userID, status string) {
	subgroups, err := h.groupService.GetSubGroups(parentGroupID)
	if err != nil {
		log.Printf("Error getting subgroups: %v", err)
		return
	}

	for _, subgroup := range subgroups {
		// Update status for this subgroup
		eventStatus := &models.GroupEventStatus{
			EventID: eventID,
			UserID:  userID,
			Status:  models.EventStatus(status),
		}

		if err := h.groupService.UpdateEventStatus(eventStatus); err != nil {
			log.Printf("Error updating event status in subgroup %s: %v", subgroup.ID, err)
		}

		// Recursively update sub-subgroups
		h.propagateEventStatusToSubgroups(subgroup.ID, eventID, userID, status)
	}
}

// HandleGetEventStatus handles get_event_status events
func (h *GroupEventHandler) HandleGetEventStatus(payload json.RawMessage) {
	var data struct {
		EventID         string `json:"event_id"`
		UserID          string `json:"user_id"`
		ResponseChannel string `json:"response_channel"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("❌ Failed to parse get_event_status payload: %v", err)
		return
	}

	// Get the event status for the user
	statuses, err := h.groupService.GetEventStatuses(data.EventID)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get event status: %v", err)
		log.Printf("❌ %s", errMsg)
		h.publisher.Publish(data.ResponseChannel, "event_status_response", map[string]interface{}{
			"status":  "error",
			"message": errMsg,
		})
		return
	}

	// Find the status for this specific user
	var userStatus *models.GroupEventStatus
	for _, status := range statuses {
		if status.UserID == data.UserID {
			userStatus = status
			break
		}
	}

	if userStatus == nil {
		h.publisher.Publish(data.ResponseChannel, "event_status_response", map[string]interface{}{
			"status":  "not_found",
			"message": "No status found for this user and event",
		})
		return
	}

	// Return the status
	h.publisher.Publish(data.ResponseChannel, "event_status_response", map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"event_id":     userStatus.EventID,
			"user_id":      userStatus.UserID,
			"status":       userStatus.Status,
			"responded_at": userStatus.RespondedAt,
			"created_at":   userStatus.CreatedAt,
		},
	})
}
