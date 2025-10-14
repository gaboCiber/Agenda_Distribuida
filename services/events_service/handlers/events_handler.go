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

		// Convertir eventos conflictivos a formato para JSON
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

// NUEVO MÉTODO: Manejar eliminación de eventos
func (h *EventsHandler) HandleEventDeletion(eventData string) {
	var redisEvent models.RedisEvent
	if err := json.Unmarshal([]byte(eventData), &redisEvent); err != nil {
		log.Printf("❌ Error parsing deletion event: %v", err)
		return
	}

	log.Printf("🔄 Procesando eliminación de evento: %s", redisEvent.EventID)

	// Extraer datos del payload
	payload := redisEvent.Payload
	eventID, _ := payload["event_id"].(string)
	userID, _ := payload["user_id"].(string)
	correlationID, _ := payload["correlation_id"].(string)

	if eventID == "" || userID == "" {
		log.Printf("❌ Datos incompletos para eliminación: event_id=%s, user_id=%s", eventID, userID)
		h.publishDeletionResponse(correlationID, false, "Missing event_id or user_id", nil)
		return
	}

	// Verificar que el evento existe y pertenece al usuario
	event, err := h.DB.GetEventByID(eventID)
	if err != nil {
		log.Printf("❌ Error verificando evento: %v", err)
		h.publishDeletionResponse(correlationID, false, "Error verifying event", nil)
		return
	}

	if event == nil {
		log.Printf("❌ Evento no encontrado: %s", eventID)
		h.publishDeletionResponse(correlationID, false, "Event not found", nil)
		return
	}

	if event.UserID != userID {
		log.Printf("❌ Usuario %s no es propietario del evento %s", userID, eventID)
		h.publishDeletionResponse(correlationID, false, "Unauthorized - Event does not belong to user", nil)
		return
	}

	// Eliminar evento
	deleted, err := h.DB.DeleteEvent(eventID)
	if err != nil {
		log.Printf("❌ Error eliminando evento: %v", err)
		h.publishDeletionResponse(correlationID, false, "Error deleting event", nil)
		return
	}

	if !deleted {
		log.Printf("❌ No se pudo eliminar el evento: %s", eventID)
		h.publishDeletionResponse(correlationID, false, "Event could not be deleted", nil)
		return
	}

	log.Printf("✅ Evento eliminado exitosamente: %s", eventID)
	h.publishDeletionResponse(correlationID, true, "Event deleted successfully", map[string]interface{}{
		"event_id": eventID,
	})
}

// FUNCIÓN para publicar respuestas de creación
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
		log.Printf("❌ Error publishing creation response: %v", err)
	} else {
		log.Printf("✅ Respuesta de creación publicada para correlation_id %s: %s", correlationID, message)
	}
}

// NUEVA FUNCIÓN: Publicar respuestas de eliminación
func (h *EventsHandler) publishDeletionResponse(correlationID string, success bool, message string, data map[string]interface{}) {
	responseData := map[string]interface{}{
		"success": success,
		"message": message,
	}

	for key, value := range data {
		responseData[key] = value
	}

	responseEvent := models.RedisEvent{
		EventID:   uuid.New().String(),
		Type:      "event_deletion_response",
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   "1.0",
		Payload: map[string]interface{}{
			"correlation_id": correlationID,
			"response":       responseData,
		},
	}

	if err := h.RedisService.PublishEvent("events_events_response", responseEvent); err != nil {
		log.Printf("❌ Error publishing deletion response: %v", err)
	} else {
		log.Printf("✅ Respuesta de eliminación publicada para correlation_id %s: %s", correlationID, message)
	}
}
