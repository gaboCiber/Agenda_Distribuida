package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/gorilla/mux"
)

// MemberHandler handles HTTP requests for group member operations
type MemberHandler struct {
	db *models.Database
}

// NewMemberHandler creates a new MemberHandler
func NewMemberHandler(db *models.Database) *MemberHandler {
	return &MemberHandler{db: db}
}

// MemberResponse represents a group member in the API response
type MemberResponse struct {
	ID       string    `json:"id"`
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// AddMemberRequest represents the request body for adding a member
type AddMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role,omitempty"`
}

// AddMember adds a user to a group
func (h *MemberHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	vars := mux.Vars(r)
	groupID := vars["id"]

	// Check if the requesting user is an admin of the group
	isAdmin, err := h.db.IsGroupAdmin(groupID, userID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to verify permissions")
		return
	}

	if !isAdmin {
		RespondWithError(w, http.StatusForbidden, "Only group admins can add members")
		return
	}

	var req AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if req.UserID == "" {
		RespondWithError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	// Set default role if not provided
	if req.Role == "" {
		req.Role = "member"
	}

	// Check if the group exists
	if _, err := h.db.GetGroupByID(groupID); err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusNotFound, "Group not found")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve group")
		}
		return
	}

	member := &models.GroupMember{
		ID:       uuid.New().String(),
		GroupID:  groupID,
		UserID:   req.UserID,
		Role:     req.Role,
		JoinedAt: time.Now().UTC(),
	}

	if err := h.db.AddGroupMember(member); err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to add member: "+err.Error())
		return
	}

	RespondWithJSON(w, http.StatusCreated, toMemberResponse(member))
}

// RemoveMember removes a user from a group
func (h *MemberHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	vars := mux.Vars(r)
	groupID := vars["id"]
	memberID := vars["member_id"]

	// Check if the requesting user is an admin or the member themselves
	isAdmin, err := h.db.IsGroupAdmin(groupID, userID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to verify permissions")
		return
	}

	if !isAdmin && userID != memberID {
		RespondWithError(w, http.StatusForbidden, "You can only remove yourself from the group")
		return
	}

	// If user is trying to remove themselves, allow it regardless of role
	// If user is an admin and trying to remove someone else, check if they're not removing another admin
	if isAdmin && userID != memberID {
		// Check if the target user is an admin
		targetIsAdmin, err := h.db.IsGroupAdmin(groupID, memberID)
		if err != nil {
			RespondWithError(w, http.StatusInternalServerError, "Failed to verify member role")
			return
		}

		// If target is an admin, make sure there are other admins
		if targetIsAdmin {
			admins, err := h.db.GetGroupAdmins(groupID)
			if err != nil {
				RespondWithError(w, http.StatusInternalServerError, "Failed to verify admin count")
				return
			}

			if len(admins) <= 1 {
				RespondWithError(w, http.StatusBadRequest, "Cannot remove the last admin from a group")
				return
			}
		}
	}

	if err := h.db.RemoveGroupMember(groupID, memberID); err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusNotFound, "Member not found in group")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Failed to remove member: "+err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListMembers returns all members of a group
func (h *MemberHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	vars := mux.Vars(r)
	groupID := vars["id"]

	// Check if the user is a member of the group
	isMember, err := h.db.IsGroupMember(groupID, userID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to verify group membership")
		return
	}

	if !isMember {
		RespondWithError(w, http.StatusForbidden, "You are not a member of this group")
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
	members, err := h.db.GetGroupMembers(groupID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve group members")
		return
	}

	// Convert to response format
	var response []MemberResponse
	for _, member := range members {
		response = append(response, *toMemberResponse(member))
	}

	// In a real implementation, you would return paginated results
	RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"members": response,
		"page":    page,
		"total":   len(response),
	})
}

// GetGroupAdmins returns all admin members of a group
func (h *MemberHandler) GetGroupAdmins(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	vars := mux.Vars(r)
	groupID := vars["id"]

	// Check if the user is a member of the group
	isMember, err := h.db.IsGroupMember(groupID, userID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to verify group membership")
		return
	}

	if !isMember {
		RespondWithError(w, http.StatusForbidden, "You are not a member of this group")
		return
	}

	// Get all admins
	admins, err := h.db.GetGroupAdmins(groupID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve group admins")
		return
	}

	// Convert to response format
	var response []MemberResponse
	for _, admin := range admins {
		response = append(response, *toMemberResponse(admin))
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// toMemberResponse converts a database GroupMember to an API response
func toMemberResponse(member *models.GroupMember) *MemberResponse {
	return &MemberResponse{
		ID:       member.ID,
		UserID:   member.UserID,
		Role:     member.Role,
		JoinedAt: member.JoinedAt,
	}
}
