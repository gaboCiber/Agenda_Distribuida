package handlers

import (
	"encoding/json"
	"events-service/database"
	"events-service/models"
	"events-service/services"
	"log"
	"time"

	"github.com/google/uuid"
)

type EventsHandler struct {
	RedisService *services.RedisService
	DB           *database.Database
}

func NewEventsHandler(redisService *services.RedisService, db *database.Database) *EventsHandler {
	return &EventsHandler{
		RedisService: redisService,
		DB:           db,
	}
}

func (h *EventsHandler) HandleEventCreation(eventData string) {
	var redisEvent models.RedisEvent
	if err := json.Unmarshal([]byte(eventData), &redisEvent); err != nil {
		log.Printf("❌ Error parsing event: %v", err)
		return
	}

	log.Printf("🔄 Procesando creación de evento: %s", redisEvent.EventID)

	// Extraer datos del payload
	payload := redisEvent.Payload
	title, _ := payload["title"].(string)
	description, _ := payload["description"].(string)
	userID, _ := payload["user_id"].(string)
	correlationID, _ := payload["correlation_id"].(string)

	// Parsear fechas
	startTimeStr, _ := payload["start_time"].(string)
	endTimeStr, _ := payload["end_time"].(string)

	startTime, err1 := time.Parse(time.RFC3339, startTimeStr)
	endTime, err2 := time.Parse(time.RFC3339, endTimeStr)

	if err1 != nil || err2 != nil {
		log.Printf("❌ Error parsing dates: %v, %v", err1, err2)
		h.publishEventResponse(correlationID, false, "Invalid date format", nil)
		return
	}

	// Verificar conflictos
	hasConflict, conflictingEvents, err := h.DB.CheckTimeConflictWithDetails(userID, startTime, endTime)
	if err != nil {
		log.Printf("❌ Error checking time conflict: %v", err)
		h.publishEventResponse(correlationID, false, "Database error", nil)
		return
	}

	if hasConflict {
		log.Printf("⚠️ Conflicto de horario detectado para usuario %s - %d eventos conflictivos", userID, len(conflictingEvents))

		// ✅ CORRECCIÓN: Convertir eventos conflictivos a formato para JSON
		conflictingEventsData := make([]map[string]interface{}, len(conflictingEvents))
		for i, event := range conflictingEvents {
			conflictingEventsData[i] = map[string]interface{}{
				"id":         event.ID,
				"title":      event.Title,
				"start_time": event.StartTime.Format(time.RFC3339),
				"end_time":   event.EndTime.Format(time.RFC3339),
			}
		}

		h.publishEventResponse(correlationID, false, "Time conflict detected", map[string]interface{}{
			"conflicting_events": conflictingEventsData,
		})
		return
	}

	// Crear evento en base de datos
	event := &models.Event{
		ID:          uuid.New().String(),
		Title:       title,
		Description: description,
		StartTime:   startTime,
		EndTime:     endTime,
		UserID:      userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.DB.CreateEvent(event); err != nil {
		log.Printf("❌ Error creating event: %v", err)
		h.publishEventResponse(correlationID, false, "Error creating event", nil)
		return
	}

	log.Printf("✅ Evento creado exitosamente: %s", event.ID)
	h.publishEventResponse(correlationID, true, "Event created successfully", map[string]interface{}{
		"event_id": event.ID,
		"title":    event.Title,
	})
}

// ✅ FUNCIÓN CORREGIDA para publicar respuestas
func (h *EventsHandler) publishEventResponse(correlationID string, success bool, message string, data map[string]interface{}) {
	responseData := map[string]interface{}{
		"success": success,
		"message": message,
	}

	for key, value := range data {
		responseData[key] = value
	}

	responseEvent := models.RedisEvent{
		EventID:   uuid.New().String(),
		Type:      "event_creation_response",
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   "1.0",
		Payload: map[string]interface{}{
			"correlation_id": correlationID,
			"response":       responseData,
		},
	}

	if err := h.RedisService.PublishEvent("events_events_response", responseEvent); err != nil {
		log.Printf("❌ Error publishing response: %v", err)
	} else {
		log.Printf("✅ Respuesta publicada para correlation_id %s: %s", correlationID, message)
	}
}
