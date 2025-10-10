package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// InvitationHandler handles HTTP requests for group invitation operations
type InvitationHandler struct {
	db *models.Database
}

// NewInvitationHandler creates a new InvitationHandler
func NewInvitationHandler(db *models.Database) *InvitationHandler {
	return &InvitationHandler{db: db}
}

// InvitationResponse represents a group invitation in the API response
type InvitationResponse struct {
	ID          string    `json:"id"`
	GroupID     string    `json:"group_id"`
	GroupName   string    `json:"group_name"`
	UserID      string    `json:"user_id"`
	InvitedBy   string    `json:"invited_by"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	RespondedAt time.Time `json:"responded_at,omitempty"`
}

// CreateInvitationRequest represents the request body for creating an invitation
type CreateInvitationRequest struct {
	UserID string `json:"user_id"`
}

// RespondToInvitationRequest represents the request body for responding to an invitation
type RespondToInvitationRequest struct {
	Action string `json:"action"` // "accept" or "reject"
}

// CreateInvitation creates a new group invitation
func (h *InvitationHandler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
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
		RespondWithError(w, http.StatusForbidden, "Only group admins can invite members")
		return
	}

	var req CreateInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if req.UserID == "" {
		RespondWithError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	// Don't allow inviting yourself
	if req.UserID == userID {
		RespondWithError(w, http.StatusBadRequest, "Cannot invite yourself to the group")
		return
	}

	// Get group name for the response
	group, err := h.db.GetGroupByID(groupID)
	if err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusNotFound, "Group not found")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve group")
		}
		return
	}

	// Create the invitation
	invitation := &models.GroupInvitation{
		ID:        uuid.New().String(),
		GroupID:   groupID,
		UserID:    req.UserID,
		InvitedBy: userID,
	}

	if err := h.db.CreateInvitation(invitation); err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create invitation: "+err.Error())
		return
	}

	// Convert to response format
	response := &InvitationResponse{
		ID:        invitation.ID,
		GroupID:   invitation.GroupID,
		GroupName: group.Name,
		UserID:    invitation.UserID,
		InvitedBy: invitation.InvitedBy,
		Status:    invitation.Status,
		CreatedAt: invitation.CreatedAt,
	}

	// In a real application, you would send a notification to the invited user here

	RespondWithJSON(w, http.StatusCreated, response)
}

// RespondToInvitation handles a user's response to a group invitation
func (h *InvitationHandler) RespondToInvitation(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	vars := mux.Vars(r)
	invitationID := vars["invitation_id"]

	// Get the invitation
	invitation, err := h.db.GetInvitationByID(invitationID)
	if err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusNotFound, "Invitation not found")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve invitation")
		}
		return
	}

	// Verify the invitation is for the authenticated user
	if invitation.UserID != userID {
		RespondWithError(w, http.StatusForbidden, "This invitation is not for you")
		return
	}

	// Check if the invitation has already been processed
	if invitation.Status != "pending" {
		RespondWithError(w, http.StatusBadRequest, "This invitation has already been processed")
		return
	}

	var req RespondToInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	// Process the response
	switch req.Action {
	case "accept":
		// Add user to the group
		member := &models.GroupMember{
			ID:       uuid.New().String(),
			GroupID:  invitation.GroupID,
			UserID:   userID,
			Role:     "member",
			JoinedAt: time.Now().UTC(),
		}

		if err := h.db.AddGroupMember(member); err != nil {
			RespondWithError(w, http.StatusInternalServerError, "Failed to add you to the group: "+err.Error())
			return
		}

		// Update invitation status
		if err := h.db.UpdateInvitation(invitation.ID, "accepted"); err != nil {
			// Log the error but don't fail the request since the user was already added to the group
			log.Printf("Failed to update invitation status: %v", err)
		}

		RespondWithJSON(w, http.StatusOK, map[string]string{
			"message": "Successfully joined the group",
		})

	case "reject":
		// Just update the invitation status
		if err := h.db.UpdateInvitation(invitation.ID, "rejected"); err != nil {
			RespondWithError(w, http.StatusInternalServerError, "Failed to update invitation status")
			return
		}

		RespondWithJSON(w, http.StatusOK, map[string]string{
			"message": "Invitation declined",
		})

	default:
		RespondWithError(w, http.StatusBadRequest, "Invalid action. Must be 'accept' or 'reject'")
	}
}

// ListUserInvitations returns all invitations for the authenticated user
func (h *InvitationHandler) ListUserInvitations(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	// Get all invitations for the user
	invitations, err := h.db.GetUserInvitations(userID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve invitations")
		return
	}

	// Convert to response format
	var response []InvitationResponse
	for _, inv := range invitations {
		// Get group name for each invitation
		group, err := h.db.GetGroupByID(inv.GroupID)
		if err != nil {
			continue // Skip invitations with invalid groups
		}

		resp := InvitationResponse{
			ID:          inv.ID,
			GroupID:     inv.GroupID,
			GroupName:   group.Name,
			UserID:      inv.UserID,
			InvitedBy:   inv.InvitedBy,
			Status:      inv.Status,
			CreatedAt:   inv.CreatedAt,
			RespondedAt: inv.RespondedAt,
		}

		response = append(response, resp)
	}

	RespondWithJSON(w, http.StatusOK, response)
}

// GetInvitation retrieves a specific invitation
func (h *InvitationHandler) GetInvitation(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r)
	if userID == "" {
		RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	vars := mux.Vars(r)
	invitationID := vars["invitation_id"]

	// Get the invitation
	invitation, err := h.db.GetInvitationByID(invitationID)
	if err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusNotFound, "Invitation not found")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve invitation")
		}
		return
	}

	// Only the invited user can view their own invitations
	if invitation.UserID != userID {
		RespondWithError(w, http.StatusForbidden, "You are not authorized to view this invitation")
		return
	}

	// Get group name
	group, err := h.db.GetGroupByID(invitation.GroupID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve group information")
		return
	}

	// Convert to response format
	response := InvitationResponse{
		ID:          invitation.ID,
		GroupID:     invitation.GroupID,
		GroupName:   group.Name,
		UserID:      invitation.UserID,
		InvitedBy:   invitation.InvitedBy,
		Status:      invitation.Status,
		CreatedAt:   invitation.CreatedAt,
		RespondedAt: invitation.RespondedAt,
	}

	RespondWithJSON(w, http.StatusOK, response)
}
