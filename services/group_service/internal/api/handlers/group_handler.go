package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/gorilla/mux"
)

// GroupHandler handles HTTP requests for group operations
type GroupHandler struct {
	db *models.Database
}

// NewGroupHandler creates a new GroupHandler
func NewGroupHandler(db *models.Database) *GroupHandler {
	return &GroupHandler{db: db}
}

// CreateGroupRequest represents the request body for creating a group
type CreateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// GroupResponse represents a group in the API response
type GroupResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	MemberCount int       `json:"member_count"`
}

// CreateGroup handles the creation of a new group
func (h *GroupHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if req.Name == "" {
		RespondWithError(w, http.StatusBadRequest, "Group name is required")
		return
	}

	group := &models.Group{
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   userID,
	}

	if err := h.db.CreateGroup(group); err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create group: "+err.Error())
		return
	}

	RespondWithJSON(w, http.StatusCreated, toGroupResponse(group, 1)) // 1 member (the creator)
}

// GetGroup retrieves a group by ID
func (h *GroupHandler) GetGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID := vars["id"]

	group, err := h.db.GetGroupByID(groupID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve group")
		return
	}

	if group == nil {
		RespondWithError(w, http.StatusNotFound, "Group not found")
		return
	}

	// Get member count
	members, err := h.db.GetGroupMembers(groupID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve group members")
		return
	}

	RespondWithJSON(w, http.StatusOK, toGroupResponse(group, len(members)))
}

// UpdateGroup updates an existing group
func (h *GroupHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	vars := mux.Vars(r)
	groupID := vars["id"]

	// Check if user is an admin of the group
	isAdmin, err := h.db.IsGroupAdmin(groupID, userID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to verify permissions")
		return
	}

	if !isAdmin {
		RespondWithError(w, http.StatusForbidden, "Only group admins can update the group")
		return
	}

	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if req.Name == "" {
		RespondWithError(w, http.StatusBadRequest, "Group name is required")
		return
	}

	group, err := h.db.GetGroupByID(groupID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve group")
		return
	}

	if group == nil {
		RespondWithError(w, http.StatusNotFound, "Group not found")
		return
	}

	// Update group fields
	group.Name = req.Name
	group.Description = req.Description

	if err := h.db.UpdateGroup(group); err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to update group: "+err.Error())
		return
	}

	// Get updated member count
	members, err := h.db.GetGroupMembers(groupID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve group members")
		return
	}

	RespondWithJSON(w, http.StatusOK, toGroupResponse(group, len(members)))
}

// DeleteGroup deletes a group
func (h *GroupHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	vars := mux.Vars(r)
	groupID := vars["id"]

	// Check if user is an admin of the group
	isAdmin, err := h.db.IsGroupAdmin(groupID, userID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to verify permissions")
		return
	}

	if !isAdmin {
		RespondWithError(w, http.StatusForbidden, "Only group admins can delete the group")
		return
	}

	if err := h.db.DeleteGroup(groupID); err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusNotFound, "Group not found")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Failed to delete group: "+err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListUserGroups returns all groups the authenticated user is a member of
func (h *GroupHandler) ListUserGroups(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Parse pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	switch {
	case pageSize > 100:
		pageSize = 100
	case pageSize <= 0:
		pageSize = 20
	}

	// In a real implementation, you would use pagination in the database query
	groups, err := h.db.ListUserGroups(userID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve groups")
		return
	}

	// Convert to response format
	var response []GroupResponse
	for _, group := range groups {
		// Get member count for each group
		members, err := h.db.GetGroupMembers(group.ID)
		if err != nil {
			continue // Skip groups with errors
		}
		response = append(response, *toGroupResponse(group, len(members)))
	}

	// In a real implementation, you would return paginated results
	RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"groups": response,
		"page":   page,
		"total":  len(response),
	})
}

// toGroupResponse converts a database Group to an API response
func toGroupResponse(group *models.Group, memberCount int) *GroupResponse {
	return &GroupResponse{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		CreatedBy:   group.CreatedBy,
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
		MemberCount: memberCount,
	}
}
