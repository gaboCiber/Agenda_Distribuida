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

    // Parsear fechas
    startTimeStr, _ := payload["start_time"].(string)
    endTimeStr, _ := payload["end_time"].(string)
    
    startTime, err1 := time.Parse(time.RFC3339, startTimeStr)
    endTime, err2 := time.Parse(time.RFC3339, endTimeStr)
    
    if err1 != nil || err2 != nil {
        log.Printf("❌ Error parsing dates: %v, %v", err1, err2)
        h.publishEventResponse(redisEvent, false, "Invalid date format")
        return
    }

    // Verificar conflicto de horario
    hasConflict, err := h.DB.CheckTimeConflict(userID, startTime, endTime)
    if err != nil {
        log.Printf("❌ Error checking time conflict: %v", err)
        h.publishEventResponse(redisEvent, false, "Database error")
        return
    }

    if hasConflict {
        log.Printf("⚠️ Conflicto de horario detectado para usuario %s", userID)
        h.publishEventResponse(redisEvent, false, "Time conflict detected")
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
        h.publishEventResponse(redisEvent, false, "Error creating event")
        return
    }

    log.Printf("✅ Evento creado exitosamente: %s", event.ID)
    h.publishEventResponse(redisEvent, true, "Event created successfully", map[string]interface{}{
        "event_id": event.ID,
        "title":    event.Title,
    })
}

func (h *EventsHandler) publishEventResponse(originalEvent models.RedisEvent, success bool, message string, data ...map[string]interface{}) {
    responseData := map[string]interface{}{
        "success": success,
        "message": message,
    }

    if len(data) > 0 {
        for key, value := range data[0] {
            responseData[key] = value
        }
    }

    responseEvent := models.RedisEvent{
        EventID:   uuid.New().String(),
        Type:      "event_creation_response",
        Timestamp: time.Now().Format(time.RFC3339),
        Version:   "1.0",
        Payload: map[string]interface{}{
            "correlation_id": originalEvent.EventID,
            "response":       responseData,
        },
    }

    if err := h.RedisService.PublishEvent("events_events_response", responseEvent); err != nil {
        log.Printf("❌ Error publishing response: %v", err)
    }
}