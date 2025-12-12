package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/agenda-distribuida/api-gateway-service/internal/clients"
)

type GroupHandler struct {
	redis           *redis.Client
	dbClient        *clients.DBClient
	responseHandler *ResponseHandler
	logger          *zap.Logger
}

type CreateGroupRequest struct {
	Name           string `json:"name" binding:"required"`
	Description    string `json:"description"`
	UserID         string `json:"user_id" binding:"required"`
	IsHierarchical bool   `json:"is_hierarchical"`
}

func NewGroupHandler(redisClient *redis.Client, dbClient *clients.DBClient, responseHandler *ResponseHandler, logger *zap.Logger) *GroupHandler {
	return &GroupHandler{
		redis:           redisClient,
		dbClient:        dbClient,
		responseHandler: responseHandler,
		logger:          logger,
	}
}

func (h *GroupHandler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing create group request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create event for group service
	eventID := uuid.New().String()

	// ✅ CREAR EVENTO CON EL FORMATO EXACTO DEL EJEMPLO
	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "group.create",
		"data": map[string]interface{}{
			"name":            req.Name,
			"description":     req.Description,
			"is_hierarchical": req.IsHierarchical,
			"creator_id":      req.UserID, // ✅ CAMPO CORRECTO: creator_id en lugar de user_id
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response", // ✅ CANAL CORRECTO
		},
	}

	h.logger.Info("📤 Enviando evento de creación de grupo",
		zap.String("event_id", eventID),
		zap.String("name", req.Name),
		zap.String("creator_id", req.UserID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to create group",
			zap.Error(err),
			zap.String("event_id", eventID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Group creation failed",
			zap.String("error", response.Error),
			zap.String("event_id", eventID))
		c.JSON(http.StatusBadRequest, gin.H{"error": response.Error})
		return
	}

	// Extract group data from response
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		h.logger.Error("❌ Invalid response data format",
			zap.Any("response_data", response.Data))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response from group service"})
		return
	}

	groupID, ok := data["id"].(string)
	if !ok {
		h.logger.Error("❌ Group ID not found in response",
			zap.Any("response_data", data))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Group ID not found in response"})
		return
	}

	h.logger.Info("✅ Group created successfully",
		zap.String("group_id", groupID),
		zap.String("name", req.Name))
	c.JSON(http.StatusCreated, gin.H{
		"message":  "Group created successfully",
		"group_id": groupID,
		"name":     req.Name,
	})
}

func (h *GroupHandler) GetGroups(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		h.logger.Warn("⚠️ user_id parameter is missing")
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id parameter is required"})
		return
	}

	h.logger.Info("📋 Getting groups for user", zap.String("user_id", userID))

	// Create event to request groups from group service
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "user.groups.list",
		"data": map[string]interface{}{
			"user_id": userID,
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Requesting groups from group service",
		zap.String("event_id", eventID),
		zap.String("user_id", userID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to get groups",
			zap.Error(err),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve groups: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Get groups failed",
			zap.String("error", response.Error),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve groups: " + response.Error})
		return
	}

	// Extract groups from response
	h.logger.Info("📦 Procesando respuesta de grupos",
		zap.String("event_id", eventID),
		zap.Any("response_data", response.Data))

	// El formato de respuesta puede variar, manejemos diferentes casos
	var groups []interface{}

	switch data := response.Data.(type) {
	case []interface{}:
		// Caso 1: La respuesta es directamente un array de grupos
		groups = data
		h.logger.Info("✅ Formato de respuesta: array directo de grupos")

	case map[string]interface{}:
		// Caso 2: La respuesta es un objeto que contiene grupos
		if groupsField, exists := data["groups"]; exists {
			if groupsArray, ok := groupsField.([]interface{}); ok {
				groups = groupsArray
				h.logger.Info("✅ Formato de respuesta: objeto con campo 'groups'")
			} else {
				h.logger.Warn("⚠️ Campo 'groups' no es un array",
					zap.Any("groups_field", groupsField))
			}
		} else {
			h.logger.Warn("⚠️ No se encontró campo 'groups' en la respuesta",
				zap.Any("response_data", data))
		}

	default:
		h.logger.Warn("⚠️ Formato de respuesta inesperado",
			zap.Any("response_data", response.Data))
	}

	h.logger.Info("✅ Groups processing completed",
		zap.String("user_id", userID),
		zap.Int("groups_count", len(groups)))

	// Siempre retornar un array, aunque esté vacío
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

func (h *GroupHandler) GetGroupMembers(c *gin.Context) {
	groupID := c.Query("group_id")
	if groupID == "" {
		// Intentar con "id" como fallback por si acaso
		groupID = c.Query("id")
	}

	if groupID == "" {
		h.logger.Warn("⚠️ group_id parameter is missing",
			zap.String("query_params", c.Request.URL.RawQuery),
			zap.Any("all_params", c.Request.URL.Query()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_id parameter is required"})
		return
	}

	h.logger.Info("📋 Getting members for group",
		zap.String("group_id", groupID),
		zap.String("query_params", c.Request.URL.RawQuery))

	// Create event to request group members from group service
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "group.member.list",
		"data": map[string]interface{}{
			"group_id": groupID,
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Requesting group members from group service",
		zap.String("event_id", eventID),
		zap.String("group_id", groupID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to get group members",
			zap.Error(err),
			zap.String("group_id", groupID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve group members: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Get group members failed",
			zap.String("error", response.Error),
			zap.String("group_id", groupID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve group members: " + response.Error})
		return
	}

	// Extract members from response
	h.logger.Info("📦 Processing group members response",
		zap.String("event_id", eventID),
		zap.Any("response_data", response.Data))

	// The response data should contain the members
	members, ok := response.Data.([]interface{})
	if !ok {
		// Try alternative format
		if data, ok := response.Data.(map[string]interface{}); ok {
			if membersField, exists := data["members"]; exists {
				if membersArray, ok := membersField.([]interface{}); ok {
					members = membersArray
				}
			}
		}
	}

	h.logger.Info("✅ Group members processing completed",
		zap.String("group_id", groupID),
		zap.Int("members_count", len(members)))

	// Always return an array, even if empty
	c.JSON(http.StatusOK, gin.H{"members": members})
}

// sendEventAndWaitForResponse publishes an event and waits for a response using the response handler
func (h *GroupHandler) sendEventAndWaitForResponse(ctx context.Context, eventData interface{}, replyChannel string) (*UserEventResponse, error) {
	// Extract event ID from eventData
	eventMap, ok := eventData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("eventData must be a map")
	}

	eventID, ok := eventMap["id"].(string)
	if !ok {
		return nil, fmt.Errorf("eventData must contain an 'id' field")
	}

	// Create a response channel for this specific event
	h.logger.Info("⏳ Esperando respuesta para evento",
		zap.String("event_id", eventID),
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

	// ✅ PUBLICAR EN EL CANAL CORRECTO: groups_events
	if err := h.redis.Publish(ctx, "groups_events", eventJSON).Err(); err != nil {
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	h.logger.Info("✅ Evento ENVIADO al group_service",
		zap.String("event_id", eventID),
		zap.String("channel", "groups_events"))

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		h.logger.Info("✅✅✅ Respuesta RECIBIDA del group_service",
			zap.String("event_id", eventID),
			zap.Bool("success", response.Success),
			zap.String("error", response.Error),
			zap.Any("data", response.Data))

		if !response.Success {
			return nil, fmt.Errorf("group service error: %s", response.Error)
		}

		return response, nil

	case <-time.After(30 * time.Second): // Increased timeout for debugging
		h.logger.Error("❌❌❌ TIMEOUT esperando respuesta del group_service",
			zap.String("event_id", eventID),
			zap.String("channel", replyChannel))
		return nil, fmt.Errorf("timeout waiting for response after 30 seconds")
	}
}
