package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agenda-distribuida/db-service/internal/models"
	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// GroupHandler handles HTTP requests related to groups
// and interacts with the GroupRepository.
type GroupHandler struct {
	repo repository.GroupRepository
	log  *zerolog.Logger
}

// NewGroupHandler creates a new GroupHandler
func NewGroupHandler(repo repository.GroupRepository, log *zerolog.Logger) *GroupHandler {
	return &GroupHandler{
		repo: repo,
		log:  log,
	}
}

// RegisterRoutes registers all group routes
func (h *GroupHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("", h.CreateGroup).Methods("POST")
	router.HandleFunc("/{id}", h.GetGroup).Methods("GET")
	router.HandleFunc("/{id}", h.UpdateGroup).Methods("PUT")
	router.HandleFunc("/{id}", h.DeleteGroup).Methods("DELETE")
	router.HandleFunc("/users/{userId}", h.ListUserGroups).Methods("GET")
	router.HandleFunc("/{groupId}/members", h.AddGroupMember).Methods("POST")
	router.HandleFunc("/{groupId}/members", h.ListGroupMembers).Methods("GET")
	router.HandleFunc("/{groupId}/members/{userId}", h.RemoveGroupMember).Methods("DELETE")
}

// CreateGroup handles the creation of a new group
func (h *GroupHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req models.GroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate request
	if err := validate.Struct(req); err != nil {
		h.log.Error().Err(err).Msg("Validation failed")
		http.Error(w, `{"status":"error","message":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	// Get creator ID from request
	userID := req.CreatorID

	// Check if parent group exists if provided
	var parentID *uuid.UUID
	if req.ParentGroupID != nil {
		// Verify parent group exists
		_, err := h.repo.GetByID(r.Context(), *req.ParentGroupID)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, `{"status":"error","message":"Parent group not found"}`, http.StatusNotFound)
				return
			}
			h.log.Error().Err(err).Str("parent_group_id", req.ParentGroupID.String()).Msg("Failed to get parent group")
			http.Error(w, `{"status":"error","message":"Failed to verify parent group"}`, http.StatusInternalServerError)
			return
		}
		parentID = req.ParentGroupID
	}

	group := &models.Group{
		ID:             uuid.New(),
		Name:           req.Name,
		Description:    req.Description,
		CreatedBy:      userID, // Use the user ID from context
		IsHierarchical: req.IsHierarchical,
		ParentGroupID:  parentID,
	}

	if err := h.repo.Create(r.Context(), group); err != nil {
		h.log.Error().Err(err).Str("group_id", group.ID.String()).Msg("Failed to create group")
		http.Error(w, `{"status":"error","message":"Failed to create group"}`, http.StatusInternalServerError)
		return
	}

	// Add creator as admin if not already a member
	isMember, err := h.repo.IsMember(r.Context(), group.ID, userID)
	if err != nil {
		h.log.Error().Err(err).
			Str("group_id", group.ID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to check group membership")
		// Continue anyway, as the group was created successfully
	} else if !isMember {
		// Only add as admin if not already a member
		if addErr := h.repo.AddMember(r.Context(), &models.GroupMember{
			ID:          uuid.New(),
			GroupID:     group.ID,
			UserID:      userID,
			Role:        "admin",
			IsInherited: false,
		}); addErr != nil {
			h.log.Error().Err(addErr).
				Str("group_id", group.ID.String()).
				Str("user_id", userID.String()).
				Msg("Failed to add creator as admin")
			// Continue even if adding admin fails, as the group was created successfully
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"group":  group,
	})
}

// GetGroup retrieves a group by ID
func (h *GroupHandler) GetGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid group ID format"}`, http.StatusBadRequest)
		return
	}

	group, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("group_id", id.String()).Msg("Failed to get group")
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, `{"status":"error","message":"Group not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"status":"error","message":"Failed to get group"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"group":  group,
	})
}

// UpdateGroup updates an existing group
func (h *GroupHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid group ID format"}`, http.StatusBadRequest)
		return
	}

	var req models.GroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Get existing group
	existing, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("group_id", id.String()).Msg("Failed to get group for update")
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, `{"status":"error","message":"Group not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"status":"error","message":"Failed to get group"}`, http.StatusInternalServerError)
		}
		return
	}

	// Update fields
	existing.Name = req.Name
	existing.Description = req.Description
	existing.IsHierarchical = req.IsHierarchical

	// Handle parent group update if needed
	if req.ParentGroupID != nil {
		// Verify parent group exists if provided
		_, err := h.repo.GetByID(r.Context(), *req.ParentGroupID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, `{"status":"error","message":"Parent group not found"}`, http.StatusBadRequest)
				return
			}
			http.Error(w, `{"status":"error","message":"Failed to verify parent group"}`, http.StatusInternalServerError)
			return
		}
		existing.ParentGroupID = req.ParentGroupID
	}

	if err := h.repo.Update(r.Context(), existing); err != nil {
		h.log.Error().Err(err).Str("group_id", id.String()).Msg("Failed to update group")
		http.Error(w, `{"status":"error","message":"Failed to update group"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"group":  existing,
	})
}

// DeleteGroup deletes a group by ID
func (h *GroupHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid group ID format"}`, http.StatusBadRequest)
		return
	}

	// Check if group exists
	_, err = h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("group_id", id.String()).Msg("Failed to get group for deletion")
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, `{"status":"error","message":"Group not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"status":"error","message":"Failed to delete group"}`, http.StatusInternalServerError)
		}
		return
	}

	// Delete the group
	if err := h.repo.Delete(r.Context(), id); err != nil {
		h.log.Error().Err(err).Str("group_id", id.String()).Msg("Failed to delete group")
		http.Error(w, `{"status":"error","message":"Failed to delete group"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Group deleted successfully",
	})
}

// ListUserGroups returns all groups a user is a member of
func (h *GroupHandler) ListUserGroups(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := uuid.Parse(vars["userId"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid user ID format"}`, http.StatusBadRequest)
		return
	}

	groups, err := h.repo.ListByUser(r.Context(), userID)
	if err != nil {
		h.log.Error().Err(err).Str("user_id", userID.String()).Msg("Failed to list user groups")
		http.Error(w, `{"status":"error","message":"Failed to list user groups"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"groups": groups,
	})
}

// AddGroupMember adds a user to a group
func (h *GroupHandler) AddGroupMember(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID, err := uuid.Parse(vars["groupId"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid group ID format"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid user ID format"}`, http.StatusBadRequest)
		return
	}

	// Validate role
	if req.Role != "admin" && req.Role != "member" {
		http.Error(w, `{"status":"error","message":"Invalid role. Must be 'admin' or 'member'"}`, http.StatusBadRequest)
		return
	}

	member := &models.GroupMember{
		ID:          uuid.New(),
		GroupID:     groupID,
		UserID:      userID,
		Role:        req.Role,
		IsInherited: false,
	}

	if err := h.repo.AddMember(r.Context(), member); err != nil {
		h.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to add group member")

		if errors.Is(err, repository.ErrGroupNotFound) {
			http.Error(w, `{"status":"error","message":"Group not found"}`, http.StatusNotFound)
		} else if errors.Is(err, repository.ErrUserAlreadyMember) {
			http.Error(w, `{"status":"error","message":"User is already a member of this group"}`, http.StatusConflict)
		} else {
			http.Error(w, `{"status":"error","message":"Failed to add group member"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"member": member,
	})
}

// ListGroupMembers returns all members of a group
func (h *GroupHandler) ListGroupMembers(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID, err := uuid.Parse(vars["groupId"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid group ID format"}`, http.StatusBadRequest)
		return
	}

	members, err := h.repo.GetMembers(r.Context(), groupID)
	if err != nil {
		h.log.Error().Err(err).Str("group_id", groupID.String()).Msg("Failed to list group members")
		if errors.Is(err, repository.ErrGroupNotFound) {
			http.Error(w, `{"status":"error","message":"Group not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"status":"error","message":"Failed to list group members"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"members": members,
	})
}

// RemoveGroupMember removes a user from a group
func (h *GroupHandler) RemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID, err := uuid.Parse(vars["groupId"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid group ID format"}`, http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(vars["userId"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid user ID format"}`, http.StatusBadRequest)
		return
	}

	// Check if the user is trying to remove themselves
	// You might want to add additional authorization checks here

	if err := h.repo.RemoveMember(r.Context(), groupID, userID); err != nil {
		h.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to remove group member")

		if errors.Is(err, repository.ErrGroupNotFound) {
			http.Error(w, `{"status":"error","message":"Group not found"}`, http.StatusNotFound)
		} else if errors.Is(err, repository.ErrMemberNotFound) {
			http.Error(w, `{"status":"error","message":"User is not a member of this group"}`, http.StatusNotFound)
		} else if errors.Is(err, repository.ErrLastAdmin) {
			http.Error(w, `{"status":"error","message":"Cannot remove the last admin from a group"}`, http.StatusBadRequest)
		} else {
			http.Error(w, `{"status":"error","message":"Failed to remove group member"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Member removed successfully",
	})
}
