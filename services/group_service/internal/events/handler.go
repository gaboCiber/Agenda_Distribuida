package events

import (
	"encoding/json"
	"log"

	"github.com/agenda-distribuida/group-service/internal/models"
)

// EventHandler handles incoming events from Redis
type EventHandler struct {
	db *models.Database
}

// NewEventHandler creates a new event handler
func NewEventHandler(db *models.Database) *EventHandler {
	return &EventHandler{
		db: db,
	}
}

// HandleMessage processes an incoming message from Redis
func (h *EventHandler) HandleMessage(channel, payload string) {
	log.Printf("📨 Received message on channel %s: %s", channel, payload)

	// Parse the event
	var event Event
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("❌ Failed to unmarshal event: %v", err)
		return
	}

	// Route the event to the appropriate handler
	switch event.Type {
	case "user_deleted":
		h.handleUserDeleted(event.Payload)
	case "event_deleted":
		h.handleEventDeleted(event.Payload)
	default:
		log.Printf("⚠️ Unhandled event type: %s", event.Type)
	}
}

// handleUserDeleted handles user_deleted events
func (h *EventHandler) handleUserDeleted(payload interface{}) {
	// Convert payload to map
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for user_deleted event")
		return
	}

	// Get user ID
	userID, ok := data["user_id"].(string)
	if !ok || userID == "" {
		log.Printf("❌ Missing or invalid user_id in user_deleted event")
		return
	}

	// Remove user from all groups
	if err := h.db.RemoveUserFromAllGroups(userID); err != nil {
		log.Printf("❌ Failed to remove user %s from all groups: %v", userID, err)
		return
	}

	// Delete all invitations for the user
	if err := h.db.DeleteUserInvitations(userID); err != nil {
		log.Printf("❌ Failed to delete invitations for user %s: %v", userID, err)
		return
	}

	log.Printf("✅ Removed user %s from all groups and deleted invitations", userID)
}

// handleEventDeleted handles event_deleted events
func (h *EventHandler) handleEventDeleted(payload interface{}) {
	// Convert payload to map
	data, ok := payload.(map[string]interface{})
	if !ok {
		log.Printf("❌ Invalid payload format for event_deleted event")
		return
	}

	// Get event ID
	eventID, ok := data["event_id"].(string)
	if !ok || eventID == "" {
		log.Printf("❌ Missing or invalid event_id in event_deleted event")
		return
	}

	// Remove event from all groups
	if err := h.db.RemoveEventFromAllGroups(eventID); err != nil {
		log.Printf("❌ Failed to remove event %s from all groups: %v", eventID, err)
		return
	}

	log.Printf("✅ Removed event %s from all groups", eventID)
}

// StartListening starts listening for events on the specified channels
func (h *EventHandler) StartListening(redisClient *RedisClient, channels ...string) {
	for _, channel := range channels {
		go func(ch string) {
			if err := redisClient.Subscribe(ch, func(payload string) {
				h.HandleMessage(ch, payload)
			}); err != nil {
				log.Printf("❌ Failed to subscribe to channel %s: %v", ch, err)
			}
		}(channel)
	}
}
