package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agenda-distribuida/db-service/internal/models"
	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// EventHandler handles HTTP requests related to events
// and interacts with the EventRepository.
type EventHandler struct {
	repo repository.EventRepository
	log  *zerolog.Logger
}

// NewEventHandler creates a new EventHandler
func NewEventHandler(repo repository.EventRepository, log *zerolog.Logger) *EventHandler {
	return &EventHandler{
		repo: repo,
		log:  log,
	}
}

// CreateEvent handles the creation of a new event
func (h *EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req models.EventRequest
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

	event := &models.Event{
		ID:          uuid.New(),
		Title:       req.Title,
		Description: req.Description,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		UserID:      req.UserID,
	}

	// Check for time conflicts
	// Check for time conflicts, excludeEventID is nil for new events
	hasConflict, err := h.repo.CheckTimeConflict(r.Context(), req.UserID, req.StartTime, req.EndTime, nil)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to check time conflict")
		http.Error(w, `{"status":"error","message":"Internal server error"}`, http.StatusInternalServerError)
		return
	}

	if hasConflict {
		http.Error(w, `{"status":"error","message":"Time conflict detected"}`, http.StatusConflict)
		return
	}

	if err := h.repo.Create(r.Context(), event); err != nil {
		h.log.Error().Err(err).Str("event_id", event.ID.String()).Msg("Failed to create event")
		http.Error(w, `{"status":"error","message":"Failed to create event"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"event":  event,
	})
}

// GetEvent retrieves an event by ID
func (h *EventHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid event ID format"}`, http.StatusBadRequest)
		return
	}

	event, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("event_id", id.String()).Msg("Failed to get event")
		if errors.Is(err, repository.ErrEventNotFound) {
			http.Error(w, `{"status":"error","message":"Event not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"status":"error","message":"Failed to get event"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"event":  event,
	})
}

// UpdateEvent updates an existing event
func (h *EventHandler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid event ID format"}`, http.StatusBadRequest)
		return
	}

	var req models.EventRequest
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

	// Check for time conflicts
	hasConflict, err := h.repo.CheckTimeConflict(r.Context(), req.UserID, req.StartTime, req.EndTime, &id)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to check time conflict")
		http.Error(w, `{"status":"error","message":"Internal server error"}`, http.StatusInternalServerError)
		return
	}

	if hasConflict {
		http.Error(w, `{"status":"error","message":"Time conflict detected"}`, http.StatusConflict)
		return
	}

	event, err := h.repo.Update(r.Context(), id, &req)
	if err != nil {
		h.log.Error().Err(err).Str("event_id", id.String()).Msg("Failed to update event")
		http.Error(w, `{"status":"error","message":"Failed to update event"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"event":  event,
	})
}

// DeleteEvent deletes an event by ID
func (h *EventHandler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid event ID format"}`, http.StatusBadRequest)
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		h.log.Error().Err(err).Str("event_id", id.String()).Msg("Failed to delete event")
		if errors.Is(err, repository.ErrEventNotFound) {
			http.Error(w, `{"status":"error","message":"Event not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"status":"error","message":"Failed to delete event"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}
