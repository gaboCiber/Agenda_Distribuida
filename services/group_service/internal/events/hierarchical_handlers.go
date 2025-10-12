package events

import (
	"encoding/json"
	"log"
)

// handleHierarchicalGroupUpdated handles updates to hierarchical groups
func (h *EventHandler) handleHierarchicalGroupUpdated(payload json.RawMessage) {
	var updateData struct {
		GroupID         string  `json:"group_id"`
		ParentGroupID   *string `json:"parent_group_id,omitempty"`
		IsHierarchical  bool    `json:"is_hierarchical"`
		UpdatedBy       string  `json:"updated_by"`
	}

	if err := json.Unmarshal(payload, &updateData); err != nil {
		log.Printf("Failed to unmarshal hierarchical group update: %v", err)
		return
	}

	// Publish event for parent group updates
	h.publisher.Publish("group_events", "hierarchical_group_updated", map[string]interface{}{
		"group_id":         updateData.GroupID,
		"parent_group_id":  updateData.ParentGroupID,
		"is_hierarchical": updateData.IsHierarchical,
		"updated_by":       updateData.UpdatedBy,
	})
}

// handleParentGroupUpdated handles updates to parent groups that affect their children
func (h *EventHandler) handleParentGroupUpdated(payload json.RawMessage) {
	var updateData struct {
		GroupID        string `json:"group_id"`
		Field          string `json:"field"`
		Value          string `json:"value"`
	}

	if err := json.Unmarshal(payload, &updateData); err != nil {
		log.Printf("Failed to unmarshal parent group update: %v", err)
		return
	}

	// Publish event to update all child groups
	h.publisher.Publish("group_events", "parent_group_updated", map[string]interface{}{
		"group_id": updateData.GroupID,
		"field":    updateData.Field,
		"value":    updateData.Value,
	})
}
// handleMemberInheritance handles member inheritance in hierarchical groups
func (h *EventHandler) handleMemberInheritance(payload json.RawMessage) {
	var inheritanceData struct {
		GroupID     string `json:"group_id"`
		UserID      string `json:"user_id"`
		InheritFrom string `json:"inherit_from"`
		Role        string `json:"role"`
	}

	if err := json.Unmarshal(payload, &inheritanceData); err != nil {
		log.Printf("Failed to unmarshal member inheritance data: %v", err)
		return
	}

	// Publish event for member inheritance
	h.publisher.Publish("group_events", "member_inheritance_updated", map[string]interface{}{
		"group_id":    inheritanceData.GroupID,
		"user_id":     inheritanceData.UserID,
		"inherit_from": inheritanceData.InheritFrom,
		"role":        inheritanceData.Role,
	})
}
