package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/agenda-distribuida/api-gateway-service/internal/clients"
	"github.com/agenda-distribuida/api-gateway-service/internal/loadbalancer"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type EventHandler struct {
	dbClient        *clients.DBClient
	responseHandler *ResponseHandler
	logger          *zap.Logger
	loadBalancer    *loadbalancer.LoadBalancer
}

type CreateEventRequest struct {
	Title       string    `json:"title" binding:"required"`
	Description string    `json:"description"`
	StartTime   time.Time `json:"start_time" binding:"required"`
	EndTime     time.Time `json:"end_time" binding:"required"`
	UserID      string    `json:"user_id" binding:"required"`
	GroupID     *string   `json:"group_id,omitempty"`
	Location    string    `json:"location,omitempty"`
}

func NewEventHandler(dbClient *clients.DBClient, responseHandler *ResponseHandler, logger *zap.Logger, loadBalancer *loadbalancer.LoadBalancer) *EventHandler {
	return &EventHandler{
		dbClient:        dbClient,
		responseHandler: responseHandler,
		logger:          logger,
		loadBalancer:    loadBalancer,
	}
}

func (h *EventHandler) CreateEvent(c *gin.Context) {
	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing create event request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create event for user service
	eventID := uuid.New().String()

	// Create event in the EXACT format that user-service expects
	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "agenda.event.create",
		"data": map[string]interface{}{
			"title":       req.Title,
			"description": req.Description,
			"start_time":  req.StartTime.Format(time.RFC3339), // Format as RFC3339 string
			"end_time":    req.EndTime.Format(time.RFC3339),   // Format as RFC3339 string
			"location":    req.Location,
			"user_id":     req.UserID,
		},
	}

	// If group_id is provided, add it to the event data
	if req.GroupID != nil && *req.GroupID != "" {
		eventData["data"].(map[string]interface{})["group_id"] = *req.GroupID
	}

	h.logger.Info("📤 Enviando evento de creación de evento",
		zap.String("event_id", eventID),
		zap.String("title", req.Title),
		zap.String("user_id", req.UserID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData)
	if err != nil {
		h.logger.Error("❌ Failed to create event",
			zap.Error(err),
			zap.String("event_id", eventID))

		// ✅ MEJOR MANEJO DE ERRORES - Mensajes específicos para el usuario
		errorMsg := "Failed to create event"
		if strings.Contains(err.Error(), "timeout") {
			errorMsg = "Service temporarily unavailable. Please try again."
		} else if strings.Contains(err.Error(), "Time conflict") {
			errorMsg = "There is already an event scheduled during this time. Please choose a different time."
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Event creation failed",
			zap.String("error", response.Error),
			zap.String("event_id", eventID))

		// ✅ MENSAJES ESPECÍFICOS PARA EL USUARIO
		userMessage := "Failed to create event"
		if strings.Contains(response.Error, "Time conflict detected") {
			userMessage = "There is already an event scheduled during this time. Please choose a different time."
		} else if strings.Contains(response.Error, "time conflict") {
			userMessage = "Time conflict: There's already an event in this time slot."
		} else if strings.Contains(response.Error, "invalid time") {
			userMessage = "Invalid time: End time must be after start time."
		} else if strings.Contains(response.Error, "required") {
			userMessage = "Please fill all required fields."
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": userMessage})
		return
	}

	// Extract event data from response
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		h.logger.Error("❌ Invalid response data format",
			zap.Any("response_data", response.Data))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response from user service"})
		return
	}

	eventIDStr, ok := data["id"].(string)
	if !ok {
		h.logger.Error("❌ Event ID not found in response",
			zap.Any("response_data", data))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Event ID not found in response"})
		return
	}

	h.logger.Info("✅ Event created successfully",
		zap.String("event_id", eventIDStr),
		zap.String("title", req.Title))
	c.JSON(http.StatusCreated, gin.H{
		"message":  "Event created successfully",
		"event_id": eventIDStr,
		"title":    req.Title,
	})
}

func (h *EventHandler) GetEvents(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		h.logger.Warn("⚠️ user_id parameter is missing")
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id parameter is required"})
		return
	}

	h.logger.Info("📋 Getting events for user", zap.String("user_id", userID))

	// Create event to request events from user service - USANDO EL TIPO CORRECTO
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "agenda.event.list", // ✅ TIPO CORRECTO: agenda.event.list
		"data": map[string]interface{}{
			"user_id": userID,
			"offset":  0,  // ✅ Incluir paginación
			"limit":   50, // ✅ Límite por defecto
		},
	}

	h.logger.Info("📤 Requesting events list from user service",
		zap.String("event_id", eventID),
		zap.String("user_id", userID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData)
	if err != nil {
		h.logger.Error("❌ Failed to get events",
			zap.Error(err),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve events: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Get events failed",
			zap.String("error", response.Error),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve events: " + response.Error})
		return
	}

	// Extract events from response
	h.logger.Info("📦 Procesando respuesta de eventos",
		zap.String("event_id", eventID),
		zap.Any("response_data", response.Data))

	// El formato de respuesta puede variar, manejemos diferentes casos
	var events []interface{}

	switch data := response.Data.(type) {
	case []interface{}:
		// Caso 1: La respuesta es directamente un array de eventos
		events = data
		h.logger.Info("✅ Formato de respuesta: array directo de eventos")

	case map[string]interface{}:
		// Caso 2: La respuesta es un objeto que contiene eventos
		if eventsField, exists := data["events"]; exists {
			if eventsArray, ok := eventsField.([]interface{}); ok {
				events = eventsArray
				h.logger.Info("✅ Formato de respuesta: objeto con campo 'events'")
			} else {
				h.logger.Warn("⚠️ Campo 'events' no es un array",
					zap.Any("events_field", eventsField))
			}
		} else {
			h.logger.Warn("⚠️ No se encontró campo 'events' en la respuesta",
				zap.Any("response_data", data))
		}

	default:
		h.logger.Warn("⚠️ Formato de respuesta inesperado",
			zap.Any("response_data", response.Data))
	}

	h.logger.Info("✅ Events processing completed",
		zap.String("user_id", userID),
		zap.Int("events_count", len(events)))

	// Siempre retornar un array, aunque esté vacío
	c.JSON(http.StatusOK, gin.H{"events": events})
}

// sendEventAndWaitForResponse publishes an event and waits for a response using the response handler
func (h *EventHandler) sendEventAndWaitForResponse(ctx context.Context, eventData interface{}) (*UserEventResponse, error) {
	// Extract event ID from eventData
	eventMap, ok := eventData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("eventData must be a map")
	}

	eventID, ok := eventMap["id"].(string)
	if !ok {
		return nil, fmt.Errorf("eventData must contain an 'id' field")
	}

	// Get the user channel from load balancer
	userChannel := h.loadBalancer.SelectUserNode()
	// Calculate corresponding response channel (user_events_1 -> users_events_response_1)
	nodeNumber := strings.TrimPrefix(userChannel, "user_events_")
	replyChannel := "users_events_response_" + nodeNumber

	// Update the reply_to in metadata
	if metadata, ok := eventMap["metadata"].(map[string]interface{}); ok {
		metadata["reply_to"] = replyChannel
	} else {
		eventMap["metadata"] = map[string]interface{}{
			"reply_to": replyChannel,
		}
	}

	// Create a response channel for this specific event
	h.logger.Info("⏳ Esperando respuesta para evento",
		zap.String("event_id", eventID),
		zap.String("publish_channel", userChannel),
		zap.String("reply_channel", replyChannel))

	responseChan := h.responseHandler.WaitForResponse(eventID)

	// Marshal event to JSON
	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	// DEBUG: Log exactly what is being sent
	h.logger.Info("📤 JSON que se enviará a Redis",
		zap.String("event_json", string(eventJSON)),
		zap.Any("event_data", eventData))

	// Publish event to user service channel
	redisClient := h.responseHandler.GetRedisClient()
	if err := redisClient.Publish(ctx, userChannel, eventJSON).Err(); err != nil {
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	h.logger.Info("✅ Evento ENVIADO al user_service",
		zap.String("event_id", eventID),
		zap.String("channel", userChannel))

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		h.logger.Info("✅✅✅ Respuesta RECIBIDA del user_service",
			zap.String("event_id", eventID),
			zap.Bool("success", response.Success),
			zap.String("error", response.Error),
			zap.Any("data", response.Data))

		if !response.Success {
			return nil, fmt.Errorf("user service error: %s", response.Error)
		}

		return response, nil

	case <-time.After(30 * time.Second): // Increased timeout for debugging
		h.logger.Error("❌❌❌ TIMEOUT esperando respuesta del user_service",
			zap.String("event_id", eventID),
			zap.String("channel", replyChannel))
		return nil, fmt.Errorf("timeout waiting for response after 30 seconds")
	}
}

func (h *EventHandler) DeleteEvent(c *gin.Context) {
	eventID := c.Param("id")
	if eventID == "" {
		h.logger.Warn("⚠️ Event ID is missing in URL")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event ID is required"})
		return
	}

	userID := c.Query("user_id")
	if userID == "" {
		h.logger.Warn("⚠️ user_id parameter is missing")
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id parameter is required"})
		return
	}

	h.logger.Info("🗑️ Deleting event",
		zap.String("event_id", eventID),
		zap.String("user_id", userID))

	// Create event to request event deletion from user service
	deleteEventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   deleteEventID,
		"type": "agenda.event.delete",
		"data": map[string]interface{}{
			"event_id": eventID,
			"user_id":  userID,
		},
	}

	h.logger.Info("📤 Requesting event deletion from user service",
		zap.String("delete_event_id", deleteEventID),
		zap.String("target_event_id", eventID),
		zap.String("user_id", userID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData)
	if err != nil {
		h.logger.Error("❌ Failed to delete event",
			zap.Error(err),
			zap.String("event_id", eventID),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete event: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Event deletion failed",
			zap.String("error", response.Error),
			zap.String("event_id", eventID),
			zap.String("user_id", userID))

		// User-friendly error messages
		userMessage := "Failed to delete event"
		if strings.Contains(response.Error, "not found") {
			userMessage = "Event not found or already deleted"
		} else if strings.Contains(response.Error, "permission") {
			userMessage = "You don't have permission to delete this event"
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": userMessage})
		return
	}

	h.logger.Info("✅ Event deleted successfully",
		zap.String("event_id", eventID),
		zap.String("user_id", userID))
	c.JSON(http.StatusOK, gin.H{
		"message":  "Event deleted successfully",
		"event_id": eventID,
	})
}

// UpdateEvent handles event update requests
func (h *EventHandler) UpdateEvent(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing update event request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("📝 Raw update request received", zap.Any("full_request", req))

	// Extract data from the nested structure
	data, ok := req["data"].(map[string]interface{})
	if !ok {
		h.logger.Error("❌ Missing data field in update request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "data field is required"})
		return
	}

	// Extract required fields from data
	var eventID string
	switch v := data["event_id"].(type) {
	case string:
		eventID = v
	case nil:
		h.logger.Error("❌ event_id is nil in update request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "event_id is required"})
		return
	default:
		h.logger.Error("❌ event_id is not a string in update request", zap.Any("event_id_type", fmt.Sprintf("%T", v)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "event_id must be a string"})
		return
	}

	if eventID == "" {
		h.logger.Error("❌ Empty event_id in update request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "event_id is required"})
		return
	}

	userID, ok := data["user_id"].(string)
	if !ok || userID == "" {
		h.logger.Error("❌ Missing user_id in update request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	h.logger.Info("📝 Updating event",
		zap.String("event_id", eventID),
		zap.String("user_id", userID))

	// Create update event data
	eventData := map[string]interface{}{
		"id":   uuid.New().String(),
		"type": "agenda.event.update",
		"data": map[string]interface{}{
			"user_id":  userID,
			"event_id": eventID,
		},
		"metadata": map[string]string{
			"reply_to": "users_events_response",
		},
	}

	// Add optional fields if present (from data, not req)
	if title, ok := data["title"].(string); ok && title != "" {
		eventData["data"].(map[string]interface{})["title"] = title
	}
	if description, ok := data["description"].(string); ok {
		eventData["data"].(map[string]interface{})["description"] = description
	}
	if location, ok := data["location"].(string); ok {
		eventData["data"].(map[string]interface{})["location"] = location
	}
	if startTime, ok := data["start_time"].(string); ok && startTime != "" {
		eventData["data"].(map[string]interface{})["start_time"] = startTime
	}
	if endTime, ok := data["end_time"].(string); ok && endTime != "" {
		eventData["data"].(map[string]interface{})["end_time"] = endTime
	}

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData)
	if err != nil {
		h.logger.Error("❌ Failed to update event",
			zap.Error(err),
			zap.String("event_id", eventID),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update event: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Event update failed",
			zap.String("error", response.Error),
			zap.String("event_id", eventID),
			zap.String("user_id", userID))

		// User-friendly error messages
		userMessage := "Failed to update event"
		if strings.Contains(response.Error, "not found") {
			userMessage = "Event not found"
		} else if strings.Contains(response.Error, "permission") {
			userMessage = "You don't have permission to update this event"
		}

		c.JSON(http.StatusBadRequest, gin.H{"error": userMessage})
		return
	}

	h.logger.Info("✅ Event updated successfully",
		zap.String("event_id", eventID),
		zap.String("user_id", userID))

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Event updated successfully",
		"event_id": eventID,
		"data":     response.Data,
	})
}
