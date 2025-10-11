package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/agenda-distribuida/group-service/internal/events"
	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/gorilla/mux"
)

// EventHandler handles event-related HTTP requests
type EventHandler struct {
	db             *models.Database
	eventPublisher *events.Publisher
}

// NewEventHandler creates a new EventHandler
func NewEventHandler(db *models.Database, publisher *events.Publisher) *EventHandler {
	return &EventHandler{
		db:             db,
		eventPublisher: publisher,
	}
}

// AddEventToGroup handles adding an event to a group
func (h *EventHandler) AddEventToGroup(w http.ResponseWriter, r *http.Request) {
	// Get group ID from URL
	vars := mux.Vars(r)
	groupID := vars["groupID"]
	if groupID == "" {
		RespondWithError(w, http.StatusBadRequest, "Group ID is required")
		return
	}

	// Parse request body
	var req struct {
		EventID string `json:"event_id"`
		AddedBy string `json:"added_by"`
	}

	if err := parseJSONBody(r, &req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Add event to group
	event := &models.GroupEvent{
		GroupID: groupID,
		EventID: req.EventID,
		AddedBy: req.AddedBy,
	}

	if err := h.db.AddGroupEvent(event); err != nil {
		log.Printf("Error adding event to group: %v", err)
		RespondWithError(w, http.StatusInternalServerError, "Failed to add event to group")
		return
	}

	// Publish event
	if h.eventPublisher != nil {
		if err := h.eventPublisher.PublishGroupEventAdded(groupID, req.EventID, req.AddedBy); err != nil {
			log.Printf("Failed to publish group event added: %v", err)
		}
	}

	RespondWithJSON(w, http.StatusCreated, event)
}

// RemoveEventFromGroup handles removing an event from a group
func (h *EventHandler) RemoveEventFromGroup(w http.ResponseWriter, r *http.Request) {
	// Get group ID and event ID from URL
	vars := mux.Vars(r)
	groupID := vars["groupID"]
	eventID := vars["eventID"]

	if groupID == "" || eventID == "" {
		RespondWithError(w, http.StatusBadRequest, "Group ID and Event ID are required")
		return
	}

	// Remove event from group
	if err := h.db.RemoveGroupEvent(groupID, eventID); err != nil {
		log.Printf("Error removing event from group: %v", err)
		RespondWithError(w, http.StatusInternalServerError, "Failed to remove event from group")
		return
	}

	// Publish event
	if h.eventPublisher != nil {
		// Get user ID from context or request
		userID := "system" // Default value if not available
		if ctxUserID := r.Context().Value("user_id"); ctxUserID != nil {
			if id, ok := ctxUserID.(string); ok {
				userID = id
			}
		}

		if err := h.eventPublisher.PublishGroupEventRemoved(groupID, eventID, userID); err != nil {
			log.Printf("Failed to publish group event removed: %v", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListGroupEvents lists all events in a group
func (h *EventHandler) ListGroupEvents(w http.ResponseWriter, r *http.Request) {
	// Get group ID from URL
	vars := mux.Vars(r)
	groupID := vars["groupID"]
	if groupID == "" {
		RespondWithError(w, http.StatusBadRequest, "Group ID is required")
		return
	}

	// Get events for group
	events, err := h.db.GetGroupEvents(groupID)
	if err != nil {
		log.Printf("Error getting group events: %v", err)
		RespondWithError(w, http.StatusInternalServerError, "Failed to get group events")
		return
	}

	RespondWithJSON(w, http.StatusOK, events)
}

// parseJSONBody is a helper function to parse JSON request body
func parseJSONBody(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	return nil
}

// // RespondWithError sends an error response with the given status code and message
// func RespondWithError(w http.ResponseWriter, code int, message string) {
// 	RespondWithJSON(w, code, map[string]string{"error": message})
// }

// // RespondWithJSON sends a JSON response with the given status code and data
// func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
// 	response, err := json.Marshal(payload)
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		w.Write([]byte("Error marshaling response"))
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(code)
// 	w.Write(response)
// }
