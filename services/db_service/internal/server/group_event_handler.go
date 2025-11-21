package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/agenda-distribuida/db-service/internal/models"
	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// GroupEventHandler handles HTTP requests related to group events
// and interacts with the GroupEventRepository.
type GroupEventHandler struct {
	repo repository.GroupEventRepository
	log  *zerolog.Logger
}

// NewGroupEventHandler creates a new GroupEventHandler
func NewGroupEventHandler(repo repository.GroupEventRepository, log *zerolog.Logger) *GroupEventHandler {
	return &GroupEventHandler{
		repo: repo,
		log:  log,
	}
}

// RegisterRoutes registers all group event routes
func (h *GroupEventHandler) RegisterRoutes(router *mux.Router) {
	// Group Event Management
	router.HandleFunc("/groups/{groupId}/events", h.AddGroupEvent).Methods("POST")
	router.HandleFunc("/groups/{groupId}/events", h.GetGroupEvents).Methods("GET")
	router.HandleFunc("/groups/{groupId}/events/{eventId}", h.RemoveGroupEvent).Methods("DELETE")
	router.HandleFunc("/groups/{groupId}/events/{eventId}", h.UpdateGroupEvent).Methods("PUT")

	// Event Status Management
	router.HandleFunc("/events/{eventId}/status", h.AddEventStatus).Methods("POST")
	router.HandleFunc("/events/{eventId}/status", h.UpdateEventStatus).Methods("PUT")
	router.HandleFunc("/events/{eventId}/status/{userId}", h.GetEventStatus).Methods("GET")
	router.HandleFunc("/events/{eventId}/statuses", h.GetEventStatuses).Methods("GET")
	router.HandleFunc("/events/{eventId}/statuses/count", h.GetEventStatusCounts).Methods("GET")
	router.HandleFunc("/events/{eventId}/statuses/group/{groupId}", h.GetEventStatusesByGroup).Methods("GET")
	router.HandleFunc("/events/{eventId}/responded/{userId}", h.HasResponded).Methods("GET")
	router.HandleFunc("/events/{eventId}/all-accepted/{groupId}", h.HasAllMembersAccepted).Methods("GET")
	router.HandleFunc("/events/{eventId}/status/{userId}", h.DeleteEventStatus).Methods("DELETE")
	router.HandleFunc("/events/{eventId}/statuses", h.DeleteEventStatuses).Methods("DELETE")
	router.HandleFunc("/events/{eventId}/statuses/group/{groupId}", h.DeleteEventStatusesByGroup).Methods("DELETE")

	// Invitation Management
	router.HandleFunc("/invitations", h.CreateInvitation).Methods("POST")
	router.HandleFunc("/invitations/{id}", h.GetInvitation).Methods("GET")
	router.HandleFunc("/invitations/{id}", h.DeleteInvitation).Methods("DELETE")
	router.HandleFunc("/invitations/{id}", h.RespondToInvitation).Methods("PUT")
	router.HandleFunc("/users/{userId}/invitations", h.GetUserInvitations).Methods("GET")
}

// AddGroupEvent handles adding an event to a group
func (h *GroupEventHandler) AddGroupEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID, err := uuid.Parse(vars["groupId"])
	if err != nil {
		h.log.Error().Err(err).Str("group_id", vars["groupId"]).Msg("Invalid group ID")
		http.Error(w, `{"status":"error","message":"Invalid group ID"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		EventID        uuid.UUID `json:"event_id"`
		AddedBy        uuid.UUID `json:"added_by"`
		IsHierarchical bool      `json:"is_hierarchical"`
		Status         string    `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	groupEvent := &models.GroupEvent{
		GroupID:        groupID,
		EventID:        req.EventID,
		AddedBy:        req.AddedBy,
		IsHierarchical: req.IsHierarchical,
		Status:         req.Status,
	}

	if err := h.repo.AddGroupEvent(r.Context(), groupEvent); err != nil {
		h.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Str("event_id", req.EventID.String()).
			Msg("Failed to add event to group")

		if err == repository.ErrEventAlreadyInGroup {
			http.Error(w, `{"status":"error","message":"Event already exists in group"}`, http.StatusConflict)
			return
		}

		http.Error(w, `{"status":"error","message":"Failed to add event to group"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Event added to group successfully",
		"data":    groupEvent,
	})
}

// GetGroupEvents retrieves all events in a group
func (h *GroupEventHandler) GetGroupEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID, err := uuid.Parse(vars["groupId"])
	if err != nil {
		h.log.Error().Err(err).Str("group_id", vars["groupId"]).Msg("Invalid group ID")
		http.Error(w, `{"status":"error","message":"Invalid group ID"}`, http.StatusBadRequest)
		return
	}

	events, err := h.repo.GetGroupEvents(r.Context(), groupID)
	if err != nil {
		h.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Msg("Failed to get group events")
		http.Error(w, `{"status":"error","message":"Failed to get group events"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   events,
	})
}

// UpdateGroupEvent updates a group event's status and hierarchical flag
func (h *GroupEventHandler) UpdateGroupEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID, err := uuid.Parse(vars["groupId"])
	if err != nil {
		h.log.Error().Err(err).Msg("Invalid group ID")
		http.Error(w, "Invalid group ID", http.StatusBadRequest)
		return
	}

	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Msg("Invalid event ID")
		http.Error(w, "Invalid event ID", http.StatusBadRequest)
		return
	}

	var requestBody struct {
		Status         models.EventStatus `json:"status"`
		IsHierarchical bool               `json:"is_hierarchical"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	updatedStatus, err := h.repo.UpdateGroupEvent(r.Context(), groupID, eventID, requestBody.Status, requestBody.IsHierarchical)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to update group event")
		if err.Error() == "group event not found" {
			http.Error(w, "Group event not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to update group event", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   updatedStatus,
	})
}

// RemoveGroupEvent removes an event from a group
func (h *GroupEventHandler) RemoveGroupEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID, err := uuid.Parse(vars["groupId"])
	if err != nil {
		h.log.Error().Err(err).Str("group_id", vars["groupId"]).Msg("Invalid group ID")
		http.Error(w, `{"status":"error","message":"Invalid group ID"}`, http.StatusBadRequest)
		return
	}

	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Str("event_id", vars["eventId"]).Msg("Invalid event ID")
		http.Error(w, `{"status":"error","message":"Invalid event ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.repo.RemoveGroupEvent(r.Context(), groupID, eventID); err != nil {
		h.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Str("event_id", eventID.String()).
			Msg("Failed to remove event from group")

		if err == repository.ErrEventNotInGroup {
			http.Error(w, `{"status":"error","message":"Event not found in group"}`, http.StatusNotFound)
			return
		}

		http.Error(w, `{"status":"error","message":"Failed to remove event from group"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Event removed from group successfully",
	})
}

// AddEventStatus adds a status for an event
func (h *GroupEventHandler) AddEventStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Str("event_id", vars["eventId"]).Msg("Invalid event ID")
		http.Error(w, `{"status":"error","message":"Invalid event ID"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		GroupID uuid.UUID          `json:"group_id"`
		UserID  uuid.UUID          `json:"user_id"`
		Status  models.EventStatus `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	status := &models.GroupEventStatus{
		EventID: eventID,
		GroupID: req.GroupID,
		UserID:  req.UserID,
		Status:  req.Status,
	}

	if err := h.repo.AddEventStatus(r.Context(), status); err != nil {
		h.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Str("user_id", req.UserID.String()).
			Msg("Failed to add event status")

		http.Error(w, `{"status":"error","message":"Failed to add event status"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Event status added successfully",
		"data":    status,
	})
}

// UpdateEventStatus updates the status of an event for a user
func (h *GroupEventHandler) UpdateEventStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Str("event_id", vars["eventId"]).Msg("Invalid event ID")
		http.Error(w, `{"status":"error","message":"Invalid event ID"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		UserID uuid.UUID          `json:"user_id"`
		Status models.EventStatus `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := h.repo.UpdateEventStatus(r.Context(), eventID, req.UserID, req.Status); err != nil {
		h.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Str("user_id", req.UserID.String()).
			Msg("Failed to update event status")

		switch err {
		case repository.ErrEventStatusNotFound:
			http.Error(w, `{"status":"error","message":"Event status not found"}`, http.StatusNotFound)
		case repository.ErrInvalidEventStatus:
			http.Error(w, `{"status":"error","message":"Invalid event status"}`, http.StatusBadRequest)
		default:
			http.Error(w, `{"status":"error","message":"Failed to update event status"}`, http.StatusInternalServerError)
		}
		return
	}

	status, err := h.repo.GetEventStatus(r.Context(), eventID, req.UserID)
	if err != nil {
		h.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Str("user_id", req.UserID.String()).
			Msg("Failed to get updated event status")
		http.Error(w, `{"status":"error","message":"Failed to get updated event status"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   status,
	})
}

// GetEventStatus retrieves the status of an event for a specific user
func (h *GroupEventHandler) GetEventStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Str("event_id", vars["eventId"]).Msg("Invalid event ID")
		http.Error(w, `{"status":"error","message":"Invalid event ID"}`, http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(vars["userId"])
	if err != nil {
		h.log.Error().Err(err).Str("user_id", vars["userId"]).Msg("Invalid user ID")
		http.Error(w, `{"status":"error","message":"Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	status, err := h.repo.GetEventStatus(r.Context(), eventID, userID)
	if err != nil {
		h.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to get event status")

		if err == repository.ErrEventStatusNotFound {
			http.Error(w, `{"status":"error","message":"Event status not found"}`, http.StatusNotFound)
			return
		}

		http.Error(w, `{"status":"error","message":"Failed to get event status"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   status,
	})
}

// GetEventStatuses retrieves all statuses for an event
func (h *GroupEventHandler) GetEventStatuses(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Str("event_id", vars["eventId"]).Msg("Invalid event ID")
		http.Error(w, `{"status":"error","message":"Invalid event ID"}`, http.StatusBadRequest)
		return
	}

	statuses, err := h.repo.GetEventStatuses(r.Context(), eventID)
	if err != nil {
		h.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Msg("Failed to get event statuses")
		http.Error(w, `{"status":"error","message":"Failed to get event statuses"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   statuses,
	})
}

// GetEventStatusesByGroup retrieves all statuses for an event in a specific group
func (h *GroupEventHandler) GetEventStatusesByGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID, err := uuid.Parse(vars["groupId"])
	if err != nil {
		h.log.Error().Err(err).Str("group_id", vars["groupId"]).Msg("Invalid group ID")
		http.Error(w, `{"status":"error","message":"Invalid group ID"}`, http.StatusBadRequest)
		return
	}

	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Str("event_id", vars["eventId"]).Msg("Invalid event ID")
		http.Error(w, `{"status":"error","message":"Invalid event ID"}`, http.StatusBadRequest)
		return
	}

	statuses, err := h.repo.GetEventStatusesByGroup(r.Context(), groupID, eventID)
	if err != nil {
		h.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Str("event_id", eventID.String()).
			Msg("Failed to get event statuses by group")
		http.Error(w, `{"status":"error","message":"Failed to get event statuses by group"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   statuses,
	})
}

// GetEventStatusCounts retrieves the count of each status for an event
func (h *GroupEventHandler) GetEventStatusCounts(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Str("event_id", vars["eventId"]).Msg("Invalid event ID")
		http.Error(w, `{"status":"error","message":"Invalid event ID"}`, http.StatusBadRequest)
		return
	}

	counts, err := h.repo.GetEventStatusCounts(r.Context(), eventID)
	if err != nil {
		h.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Msg("Failed to get event status counts")
		http.Error(w, `{"status":"error","message":"Failed to get event status counts"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   counts,
	})
}

// HasResponded checks if a user has responded to an event
func (h *GroupEventHandler) HasResponded(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Str("event_id", vars["eventId"]).Msg("Invalid event ID")
		http.Error(w, `{"status":"error","message":"Invalid event ID"}`, http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(vars["userId"])
	if err != nil {
		h.log.Error().Err(err).Str("user_id", vars["userId"]).Msg("Invalid user ID")
		http.Error(w, `{"status":"error","message":"Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	hasResponded, err := h.repo.HasResponded(r.Context(), eventID, userID)
	if err != nil {
		h.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to check if user has responded to event")
		http.Error(w, `{"status":"error","message":"Failed to check if user has responded to event"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data": map[string]bool{
			"has_responded": hasResponded,
		},
	})
}

// HasAllMembersAccepted checks if all members of a non-hierarchical group have accepted an event
func (h *GroupEventHandler) HasAllMembersAccepted(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID, err := uuid.Parse(vars["groupId"])
	if err != nil {
		h.log.Error().Err(err).Str("group_id", vars["groupId"]).Msg("Invalid group ID")
		http.Error(w, `{"status":"error","message":"Invalid group ID"}`, http.StatusBadRequest)
		return
	}

	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Str("event_id", vars["eventId"]).Msg("Invalid event ID")
		http.Error(w, `{"status":"error","message":"Invalid event ID"}`, http.StatusBadRequest)
		return
	}

	allAccepted, err := h.repo.HasAllMembersAccepted(r.Context(), groupID, eventID)
	if err != nil {
		h.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Str("event_id", eventID.String()).
			Msg("Failed to check if all members have accepted the event")
		http.Error(w, `{"status":"error","message":"Failed to check if all members have accepted the event"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data": map[string]bool{
			"all_accepted": allAccepted,
		},
	})
}

// DeleteEventStatus deletes an event status for a specific user and event
func (h *GroupEventHandler) DeleteEventStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Str("event_id", vars["eventId"]).Msg("Invalid event ID")
		http.Error(w, `{"status":"error","message":"Invalid event ID"}`, http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(vars["userId"])
	if err != nil {
		h.log.Error().Err(err).Str("user_id", vars["userId"]).Msg("Invalid user ID")
		http.Error(w, `{"status":"error","message":"Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	tx, err := h.repo.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	}).BeginTx(r.Context(), nil)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to begin transaction")
		http.Error(w, `{"status":"error","message":"Failed to begin transaction"}`, http.StatusInternalServerError)
		return
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = h.repo.DeleteEventStatus(r.Context(), tx, eventID, userID)
	if err != nil {
		h.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to delete event status")
		http.Error(w, `{"status":"error","message":"Failed to delete event status"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteEventStatuses deletes all statuses for an event
func (h *GroupEventHandler) DeleteEventStatuses(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Str("event_id", vars["eventId"]).Msg("Invalid event ID")
		http.Error(w, `{"status":"error","message":"Invalid event ID"}`, http.StatusBadRequest)
		return
	}

	tx, err := h.repo.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	}).BeginTx(r.Context(), nil)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to begin transaction")
		http.Error(w, `{"status":"error","message":"Failed to begin transaction"}`, http.StatusInternalServerError)
		return
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = h.repo.DeleteEventStatuses(r.Context(), tx, eventID)
	if err != nil {
		h.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Msg("Failed to delete event statuses")
		http.Error(w, `{"status":"error","message":"Failed to delete event statuses"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteEventStatusesByGroup deletes all statuses for an event in a specific group
func (h *GroupEventHandler) DeleteEventStatusesByGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	groupID, err := uuid.Parse(vars["groupId"])
	if err != nil {
		h.log.Error().Err(err).Str("group_id", vars["groupId"]).Msg("Invalid group ID")
		http.Error(w, `{"status":"error","message":"Invalid group ID"}`, http.StatusBadRequest)
		return
	}

	eventID, err := uuid.Parse(vars["eventId"])
	if err != nil {
		h.log.Error().Err(err).Str("event_id", vars["eventId"]).Msg("Invalid event ID")
		http.Error(w, `{"status":"error","message":"Invalid event ID"}`, http.StatusBadRequest)
		return
	}

	tx, err := h.repo.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	}).BeginTx(r.Context(), nil)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to begin transaction")
		http.Error(w, `{"status":"error","message":"Failed to begin transaction"}`, http.StatusInternalServerError)
		return
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = h.repo.DeleteEventStatusesByGroup(r.Context(), tx, groupID, eventID)
	if err != nil {
		h.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Str("event_id", eventID.String()).
			Msg("Failed to delete event statuses by group")
		http.Error(w, `{"status":"error","message":"Failed to delete event statuses by group"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// CreateInvitation creates a new group invitation
func (h *GroupEventHandler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GroupID   uuid.UUID `json:"group_id"`
		UserID    uuid.UUID `json:"user_id"`
		InvitedBy uuid.UUID `json:"invited_by"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	invitation := &models.GroupInvitation{
		GroupID:   req.GroupID,
		UserID:    req.UserID,
		InvitedBy: req.InvitedBy,
	}

	if err := h.repo.CreateInvitation(r.Context(), invitation); err != nil {
		h.log.Error().Err(err).
			Str("group_id", req.GroupID.String()).
			Str("user_id", req.UserID.String()).
			Msg("Failed to create invitation")

		http.Error(w, `{"status":"error","message":"Failed to create invitation"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Invitation created successfully",
		"data":    invitation,
	})
}

// GetInvitation retrieves an invitation by ID
func (h *GroupEventHandler) GetInvitation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	invitationID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.log.Error().Err(err).Str("invitation_id", vars["id"]).Msg("Invalid invitation ID")
		http.Error(w, `{"status":"error","message":"Invalid invitation ID"}`, http.StatusBadRequest)
		return
	}

	invitation, err := h.repo.GetInvitationByID(r.Context(), invitationID)
	if err != nil {
		h.log.Error().Err(err).
			Str("invitation_id", invitationID.String()).
			Msg("Failed to get invitation")

		if err == repository.ErrInvitationNotFound {
			http.Error(w, `{"status":"error","message":"Invitation not found"}`, http.StatusNotFound)
			return
		}

		http.Error(w, `{"status":"error","message":"Failed to get invitation"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   invitation,
	})
}

// RespondToInvitation handles a user's response to a group invitation
func (h *GroupEventHandler) RespondToInvitation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	invitationID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.log.Error().Err(err).Str("invitation_id", vars["id"]).Msg("Invalid invitation ID")
		http.Error(w, `{"status":"error","message":"Invalid invitation ID"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := h.repo.UpdateInvitation(r.Context(), invitationID, req.Status); err != nil {
		h.log.Error().Err(err).
			Str("invitation_id", invitationID.String()).
			Str("status", req.Status).
			Msg("Failed to update invitation status")

		if err == repository.ErrInvitationNotFound {
			http.Error(w, `{"status":"error","message":"Invitation not found"}`, http.StatusNotFound)
			return
		}

		http.Error(w, `{"status":"error","message":"Failed to update invitation status"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Invitation updated successfully",
	})
}

// DeleteInvitation deletes a specific invitation by ID
func (h *GroupEventHandler) DeleteInvitation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	invitationID, err := uuid.Parse(vars["id"])
	if err != nil {
		h.log.Error().Err(err).Str("invitation_id", vars["id"]).Msg("Invalid invitation ID")
		http.Error(w, "Invalid invitation ID", http.StatusBadRequest)
		return
	}

	// Delete the invitation
	err = h.repo.DeleteUserInvitation(r.Context(), invitationID)
	if err != nil {
		if err == sql.ErrNoRows {
			h.log.Warn().Str("invitation_id", invitationID.String()).Msg("Invitation not found")
			http.Error(w, "Invitation not found", http.StatusNotFound)
			return
		}

		h.log.Error().Err(err).Str("invitation_id", invitationID.String()).Msg("Failed to delete invitation")
		http.Error(w, "Failed to delete invitation", http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Invitation deleted successfully",
	})
}

// GetUserInvitations retrieves all invitations for a user
func (h *GroupEventHandler) GetUserInvitations(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := uuid.Parse(vars["userId"])
	if err != nil {
		h.log.Error().Err(err).Str("user_id", vars["userId"]).Msg("Invalid user ID")
		http.Error(w, `{"status":"error","message":"Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	// Get status filter from query parameter
	status := r.URL.Query().Get("status")

	invitations, err := h.repo.GetUserInvitations(r.Context(), userID, status)
	if err != nil {
		h.log.Error().Err(err).
			Str("user_id", userID.String()).
			Msg("Failed to get user invitations")
		http.Error(w, `{"status":"error","message":"Failed to get user invitations"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   invitations,
	})
}
