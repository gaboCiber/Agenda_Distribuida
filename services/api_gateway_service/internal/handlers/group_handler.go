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

type CreateGroupEventRequest struct {
	GroupID        string `json:"group_id" binding:"required"`
	EventID        string `json:"event_id" binding:"required"`
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

	// ✅ ENRIQUECER GRUPOS CON NOMBRES DE USUARIO
	enrichedGroups, err := h.enrichGroupsWithUsernames(c.Request.Context(), groups)
	if err != nil {
		h.logger.Error("❌ Failed to enrich groups with usernames",
			zap.Error(err),
			zap.String("user_id", userID))
		// Continuar sin enriquecimiento si falla
		enrichedGroups = groups
	}

	h.logger.Info("✅ Groups processing completed",
		zap.String("user_id", userID),
		zap.Int("groups_count", len(enrichedGroups)))

	// Siempre retornar un array, aunque esté vacío
	c.JSON(http.StatusOK, gin.H{"groups": enrichedGroups})
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

	// ✅ ENRIQUECER MIEMBROS CON NOMBRES DE USUARIO
	enrichedMembers, err := h.enrichMembersWithUsernames(c.Request.Context(), members)
	if err != nil {
		h.logger.Error("❌ Failed to enrich members with usernames",
			zap.Error(err),
			zap.String("group_id", groupID))
		// Continuar sin enriquecimiento si falla
		enrichedMembers = members
	}

	// Always return an array, even if empty
	c.JSON(http.StatusOK, gin.H{"members": enrichedMembers})
}

func (h *GroupHandler) ListGroupEvents(c *gin.Context) {
	groupID := c.Param("group_id")
	userID := c.Query("user_id")

	if userID == "" {
		h.logger.Warn("⚠️ user_id parameter is missing")
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id parameter is required"})
		return
	}

	h.logger.Info("📋 Getting events for group",
		zap.String("group_id", groupID),
		zap.String("user_id", userID))

	// Create event to request group events from group service
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "group.event.list",
		"data": map[string]interface{}{
			"group_id": groupID,
			"user_id":  userID,
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Requesting group events from group service",
		zap.String("event_id", eventID),
		zap.String("group_id", groupID),
		zap.String("user_id", userID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to get group events",
			zap.Error(err),
			zap.String("group_id", groupID),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve group events: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Get group events failed",
			zap.String("error", response.Error),
			zap.String("group_id", groupID),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve group events: " + response.Error})
		return
	}

	// Extract events from response
	h.logger.Info("📦 Processing group events response",
		zap.String("event_id", eventID),
		zap.Any("response_data", response.Data))

	// The response data should contain the events
	events, ok := response.Data.([]interface{})
	if !ok {
		// Try alternative format
		if data, ok := response.Data.(map[string]interface{}); ok {
			if eventsField, exists := data["events"]; exists {
				if eventsArray, ok := eventsField.([]interface{}); ok {
					events = eventsArray
				}
			}
		}
	}

	h.logger.Info("✅ Group events processing completed",
		zap.String("group_id", groupID),
		zap.String("user_id", userID),
		zap.Int("events_count", len(events)))

	// Always return an array, even if empty
	c.JSON(http.StatusOK, gin.H{"events": events})
}

func (h *GroupHandler) AcceptGroupEvent(c *gin.Context) {
	eventID := c.Param("event_id")

	var req struct {
		EventID string `json:"event_id" binding:"required"`
		GroupID string `json:"group_id" binding:"required"`
		UserID  string `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing accept group event request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := req.UserID

	h.logger.Info("✅ Accepting group event",
		zap.String("event_id", eventID),
		zap.String("group_id", req.GroupID),
		zap.String("user_id", userID))

	// Create event to accept group event
	eventIDUUID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventIDUUID,
		"type": "group.event.status.update",
		"data": map[string]interface{}{
			"event_id": eventID,
			"group_id": req.GroupID,
			"user_id":  userID,
			"status":   "accepted",
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Sending group event accept request",
		zap.String("event_id", eventID),
		zap.String("group_id", req.GroupID),
		zap.String("user_id", userID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to accept group event",
			zap.Error(err),
			zap.String("event_id", eventID),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept group event: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Accept group event failed",
			zap.String("error", response.Error),
			zap.String("event_id", eventID),
			zap.String("user_id", userID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to accept group event: " + response.Error})
		return
	}

	h.logger.Info("✅ Group event accepted successfully",
		zap.String("event_id", eventID),
		zap.String("user_id", userID))

	c.JSON(http.StatusOK, gin.H{
		"message":  "Group event accepted successfully",
		"event_id": eventID,
		"user_id":  userID,
		"status":   "accepted",
	})
}

func (h *GroupHandler) DeclineGroupEvent(c *gin.Context) {
	eventID := c.Param("event_id")

	var req struct {
		EventID string `json:"event_id" binding:"required"`
		GroupID string `json:"group_id" binding:"required"`
		UserID  string `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing decline group event request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := req.UserID

	h.logger.Info("❌ Declining group event",
		zap.String("event_id", eventID),
		zap.String("group_id", req.GroupID),
		zap.String("user_id", userID))

	// Create event to decline group event
	eventIDUUID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventIDUUID,
		"type": "group.event.status.update",
		"data": map[string]interface{}{
			"event_id": eventID,
			"group_id": req.GroupID,
			"user_id":  userID,
			"status":   "declined",
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Sending group event decline request",
		zap.String("event_id", eventID),
		zap.String("group_id", req.GroupID),
		zap.String("user_id", userID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to decline group event",
			zap.Error(err),
			zap.String("event_id", eventID),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decline group event: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Decline group event failed",
			zap.String("error", response.Error),
			zap.String("event_id", eventID),
			zap.String("user_id", userID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decline group event: " + response.Error})
		return
	}

	h.logger.Info("✅ Group event declined successfully",
		zap.String("event_id", eventID),
		zap.String("user_id", userID))

	c.JSON(http.StatusOK, gin.H{
		"message":  "Group event declined successfully",
		"event_id": eventID,
		"user_id":  userID,
		"status":   "declined",
	})
}

// enrichMembersWithUsernames enriquece la lista de miembros con nombres de usuario
func (h *GroupHandler) enrichMembersWithUsernames(ctx context.Context, members []interface{}) ([]interface{}, error) {
	enrichedMembers := make([]interface{}, len(members))

	for i, memberInterface := range members {
		member, ok := memberInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Copiar el miembro original
		enrichedMember := make(map[string]interface{})
		for k, v := range member {
			enrichedMember[k] = v
		}

		// Obtener el nombre del usuario si existe user_id
		if userID, exists := member["user_id"]; exists {
			if userIDStr, ok := userID.(string); ok {
				username, err := h.getUsernameByID(ctx, userIDStr)
				if err != nil {
					h.logger.Warn("Failed to get username for member",
						zap.String("user_id", userIDStr),
						zap.Error(err))
					username = "Usuario desconocido"
				}
				enrichedMember["username"] = username
			}
		}

		enrichedMembers[i] = enrichedMember
	}

	return enrichedMembers, nil
}

// enrichGroupsWithUsernames enriquece la lista de grupos con nombres de usuario
func (h *GroupHandler) enrichGroupsWithUsernames(ctx context.Context, groups []interface{}) ([]interface{}, error) {
	enrichedGroups := make([]interface{}, len(groups))

	for i, groupInterface := range groups {
		group, ok := groupInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Copiar el grupo original
		enrichedGroup := make(map[string]interface{})
		for k, v := range group {
			enrichedGroup[k] = v
		}

		// Obtener el nombre del creador si existe creator_id
		if creatorID, exists := group["creator_id"]; exists {
			if creatorIDStr, ok := creatorID.(string); ok {
				username, err := h.getUsernameByID(ctx, creatorIDStr)
				if err != nil {
					h.logger.Warn("Failed to get username for creator",
						zap.String("creator_id", creatorIDStr),
						zap.Error(err))
					username = "Usuario desconocido"
				}
				enrichedGroup["creator_name"] = username
			}
		}

		enrichedGroups[i] = enrichedGroup
	}

	return enrichedGroups, nil
}

// getUserEmailByID obtiene el email de usuario por ID consultando el servicio de usuarios
func (h *GroupHandler) getUserEmailByID(ctx context.Context, userID string) (string, error) {
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "user.get",
		"data": map[string]interface{}{
			"user_id": userID,
		},
		"metadata": map[string]string{
			"reply_to": "users_events_response",
		},
	}

	// Send event to users_events channel (not groups_events)
	// We need to publish directly to Redis instead of using sendEventAndWaitForResponse
	// which is configured for groups_events
	responseChan := h.responseHandler.WaitForResponse(eventID)

	// Marshal event to JSON
	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to the correct channel: users_events
	if err := h.redis.Publish(ctx, "users_events", eventJSON).Err(); err != nil {
		return "", fmt.Errorf("failed to publish event: %w", err)
	}

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		if !response.Success {
			return "", fmt.Errorf("user service error: %s", response.Error)
		}

		// Extract email from response
		if userData, ok := response.Data.(map[string]interface{}); ok {
			if user, exists := userData["user"]; exists {
				if userMap, ok := user.(map[string]interface{}); ok {
					if email, exists := userMap["email"]; exists {
						if emailStr, ok := email.(string); ok {
							return emailStr, nil
						}
					}
				}
			}
			// Try direct extraction if nested structure doesn't work
			if email, exists := userData["email"]; exists {
				if emailStr, ok := email.(string); ok {
					return emailStr, nil
				}
			}
		}

		return "", fmt.Errorf("email not found in response")

	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("timeout waiting for user email response after 30 seconds")
	}
}

// getUsernameByID obtiene el nombre de usuario por ID consultando el servicio de usuarios
func (h *GroupHandler) getUsernameByID(ctx context.Context, userID string) (string, error) {
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "user.get",
		"data": map[string]interface{}{
			"user_id": userID,
		},
		"metadata": map[string]string{
			"reply_to": "users_events_response",
		},
	}

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(ctx, eventData, "users_events_response")
	if err != nil {
		return "", fmt.Errorf("failed to get user info: %w", err)
	}

	if !response.Success {
		return "", fmt.Errorf("DB service error: %s", response.Error)
	}

	// Extract username from response
	if userData, ok := response.Data.(map[string]interface{}); ok {
		if user, exists := userData["user"]; exists {
			if userMap, ok := user.(map[string]interface{}); ok {
				if username, exists := userMap["username"]; exists {
					if usernameStr, ok := username.(string); ok {
						return usernameStr, nil
					}
				}
			}
		}
		// Try direct extraction if nested structure doesn't work
		if username, exists := userData["username"]; exists {
			if usernameStr, ok := username.(string); ok {
				return usernameStr, nil
			}
		}
	}

	return "", fmt.Errorf("username not found in response")
}

// getUserIDByEmail obtiene el ID de usuario por email consultando el servicio de usuarios
func (h *GroupHandler) getUserIDByEmail(ctx context.Context, email string) (string, error) {
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "user.get.by.email",
		"data": map[string]interface{}{
			"email": email,
		},
		"metadata": map[string]string{
			"reply_to": "users_events_response",
		},
	}

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(ctx, eventData, "users_events_response")
	if err != nil {
		return "", fmt.Errorf("failed to get user info: %w", err)
	}

	if !response.Success {
		return "", fmt.Errorf("DB service error: %s", response.Error)
	}

	// Extract user ID from response
	if userData, ok := response.Data.(map[string]interface{}); ok {
		if userID, exists := userData["id"]; exists {
			if userIDStr, ok := userID.(string); ok {
				return userIDStr, nil
			}
		}
	}

	return "", fmt.Errorf("user ID not found in response")
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

type InviteUserByEmailRequest struct {
	GroupID string `json:"group_id" binding:"required"`
	Email   string `json:"email" binding:"required,email"`
}

func (h *GroupHandler) InviteUserByEmail(c *gin.Context) {
	var req InviteUserByEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing invite user by email request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the current user ID from the context (who is sending the invitation)
	currentUserID := c.Query("user_id")
	if currentUserID == "" {
		h.logger.Warn("⚠️ user_id parameter is missing for invitation")
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id parameter is required"})
		return
	}

	h.logger.Info("📋 Inviting user by email to group",
		zap.String("group_id", req.GroupID),
		zap.String("email", req.Email),
		zap.String("invited_by", currentUserID))

	// Create event to invite user to group using email directly (new format)
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "group.invite.create",
		"data": map[string]interface{}{
			"group_id":   req.GroupID,
			"email":      req.Email,
			"invited_by": currentUserID,
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Sending group invitation event with email",
		zap.String("event_id", eventID),
		zap.String("group_id", req.GroupID),
		zap.String("email", req.Email),
		zap.String("invited_by", currentUserID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to create group invitation",
			zap.Error(err),
			zap.String("group_id", req.GroupID),
			zap.String("email", req.Email))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create invitation: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Group invitation failed",
			zap.String("error", response.Error),
			zap.String("group_id", req.GroupID),
			zap.String("email", req.Email))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create invitation: " + response.Error})
		return
	}

	h.logger.Info("✅ Group invitation created successfully",
		zap.String("group_id", req.GroupID),
		zap.String("email", req.Email),
		zap.String("invited_by", currentUserID))

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Invitation created successfully",
		"group_id":   req.GroupID,
		"email":      req.Email,
		"invited_by": currentUserID,
	})
}

type UpdateGroupRequest struct {
	GroupID     string `json:"group_id" binding:"required"`
	Name        string `json:"name"`
	Description string `json:"description"`
	UserID      string `json:"user_id" binding:"required"`
}

func (h *GroupHandler) UpdateGroup(c *gin.Context) {
	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing update group request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("📋 Updating group",
		zap.String("group_id", req.GroupID),
		zap.String("user_id", req.UserID))

	// Create event to update group
	eventID := uuid.New().String()

	// Build update data - only include fields that are provided
	updateData := make(map[string]interface{})
	if req.Name != "" {
		updateData["name"] = req.Name
	}
	if req.Description != "" {
		updateData["description"] = req.Description
	}

	// If no update data provided, return error
	if len(updateData) == 0 {
		h.logger.Warn("⚠️ No update data provided for group update")
		c.JSON(http.StatusBadRequest, gin.H{"error": "No update data provided"})
		return
	}

	// Build the update data with creator_id for group service compatibility
	updateDataWithCreator := make(map[string]interface{})
	for k, v := range updateData {
		updateDataWithCreator[k] = v
	}
	// Add creator_id if not present (required by group service)
	if _, exists := updateDataWithCreator["creator_id"]; !exists {
		updateDataWithCreator["creator_id"] = req.UserID
	}

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "group.update",
		"data": map[string]interface{}{
			"id":   req.GroupID,
			"data": updateDataWithCreator,
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Sending group update event",
		zap.String("event_id", eventID),
		zap.String("group_id", req.GroupID),
		zap.Any("update_data", updateData))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to update group",
			zap.Error(err),
			zap.String("group_id", req.GroupID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Group update failed",
			zap.String("error", response.Error),
			zap.String("group_id", req.GroupID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update group: " + response.Error})
		return
	}

	h.logger.Info("✅ Group updated successfully",
		zap.String("group_id", req.GroupID))

	c.JSON(http.StatusOK, gin.H{
		"message":  "Group updated successfully",
		"group_id": req.GroupID,
	})
}

type DeleteGroupRequest struct {
	GroupID string `json:"group_id" binding:"required"`
	UserID  string `json:"user_id" binding:"required"`
}

func (h *GroupHandler) DeleteGroup(c *gin.Context) {
	var req DeleteGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing delete group request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("🗑️ Deleting group",
		zap.String("group_id", req.GroupID),
		zap.String("user_id", req.UserID))

	// Create event to delete group
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "group.delete",
		"data": map[string]interface{}{
			"id": req.GroupID,
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Sending group delete event",
		zap.String("event_id", eventID),
		zap.String("group_id", req.GroupID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to delete group",
			zap.Error(err),
			zap.String("group_id", req.GroupID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Group deletion failed",
			zap.String("error", response.Error),
			zap.String("group_id", req.GroupID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to delete group: " + response.Error})
		return
	}

	h.logger.Info("✅ Group deleted successfully",
		zap.String("group_id", req.GroupID))

	c.JSON(http.StatusOK, gin.H{
		"message":  "Group deleted successfully",
		"group_id": req.GroupID,
	})
}

type UpdateMemberRoleRequest struct {
	GroupID string `json:"group_id" binding:"required"`
	Email   string `json:"email" binding:"required,email"`
	Role    string `json:"role" binding:"required,oneof=admin member"`
	UserID  string `json:"user_id" binding:"required"`
}

func (h *GroupHandler) UpdateMemberRole(c *gin.Context) {
	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing update member role request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("👤 Updating member role in group",
		zap.String("group_id", req.GroupID),
		zap.String("email", req.Email),
		zap.String("role", req.Role),
		zap.String("updated_by", req.UserID))

	// Create event to update member role
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "group.member.update",
		"data": map[string]interface{}{
			"group_id": req.GroupID,
			"email":    req.Email,
			"role":     req.Role,
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Sending member role update event",
		zap.String("event_id", eventID),
		zap.String("group_id", req.GroupID),
		zap.String("email", req.Email),
		zap.String("role", req.Role))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to update member role",
			zap.Error(err),
			zap.String("group_id", req.GroupID),
			zap.String("email", req.Email))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update member role: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Member role update failed",
			zap.String("error", response.Error),
			zap.String("group_id", req.GroupID),
			zap.String("email", req.Email))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to update member role: " + response.Error})
		return
	}

	h.logger.Info("✅ Member role updated successfully",
		zap.String("group_id", req.GroupID),
		zap.String("email", req.Email),
		zap.String("role", req.Role))

	c.JSON(http.StatusOK, gin.H{
		"message":  "Member role updated successfully",
		"group_id": req.GroupID,
		"email":    req.Email,
		"role":     req.Role,
	})
}

type AcceptGroupInvitationRequest struct {
	InvitationID string `json:"invitation_id" binding:"required"`
	GroupID      string `json:"group_id" binding:"required"`
	UserID       string `json:"user_id" binding:"required"`
}

func (h *GroupHandler) AcceptGroupInvitation(c *gin.Context) {
	var req AcceptGroupInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing accept group invitation request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("✅ Accepting group invitation",
		zap.String("invitation_id", req.InvitationID),
		zap.String("group_id", req.GroupID),
		zap.String("user_id", req.UserID))

	// Create event to accept group invitation
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "group.invite.accept",
		"data": map[string]interface{}{
			"invitation_id": req.InvitationID,
			"group_id":      req.GroupID,
			"user_id":       req.UserID,
			"status":        "accepted",
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Sending group invitation acceptance event",
		zap.String("event_id", eventID),
		zap.String("invitation_id", req.InvitationID),
		zap.String("group_id", req.GroupID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to accept group invitation",
			zap.Error(err),
			zap.String("invitation_id", req.InvitationID),
			zap.String("group_id", req.GroupID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept invitation: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Group invitation acceptance failed",
			zap.String("error", response.Error),
			zap.String("invitation_id", req.InvitationID),
			zap.String("group_id", req.GroupID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to accept invitation: " + response.Error})
		return
	}

	h.logger.Info("✅ Group invitation accepted successfully",
		zap.String("invitation_id", req.InvitationID),
		zap.String("group_id", req.GroupID))

	c.JSON(http.StatusOK, gin.H{
		"message":       "Group invitation accepted successfully",
		"invitation_id": req.InvitationID,
		"group_id":      req.GroupID,
		"status":        "accepted",
	})
}

type RejectGroupInvitationRequest struct {
	InvitationID string `json:"invitation_id" binding:"required"`
	GroupID      string `json:"group_id" binding:"required"`
	UserID       string `json:"user_id" binding:"required"`
}

func (h *GroupHandler) RejectGroupInvitation(c *gin.Context) {
	var req RejectGroupInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing reject group invitation request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("❌ Rejecting group invitation",
		zap.String("invitation_id", req.InvitationID),
		zap.String("group_id", req.GroupID),
		zap.String("user_id", req.UserID))

	// Create event to reject group invitation
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "group.invite.reject",
		"data": map[string]interface{}{
			"invitation_id": req.InvitationID,
			"group_id":      req.GroupID,
			"user_id":       req.UserID,
			"status":        "rejected",
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Sending group invitation rejection event",
		zap.String("event_id", eventID),
		zap.String("invitation_id", req.InvitationID),
		zap.String("group_id", req.GroupID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to reject group invitation",
			zap.Error(err),
			zap.String("invitation_id", req.InvitationID),
			zap.String("group_id", req.GroupID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject invitation: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Group invitation rejection failed",
			zap.String("error", response.Error),
			zap.String("invitation_id", req.InvitationID),
			zap.String("group_id", req.GroupID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to reject invitation: " + response.Error})
		return
	}

	h.logger.Info("✅ Group invitation rejected successfully",
		zap.String("invitation_id", req.InvitationID),
		zap.String("group_id", req.GroupID))

	c.JSON(http.StatusOK, gin.H{
		"message":       "Group invitation rejected successfully",
		"invitation_id": req.InvitationID,
		"group_id":      req.GroupID,
		"status":        "rejected",
	})
}

func (h *GroupHandler) GetGroupInvitations(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		h.logger.Warn("⚠️ user_id parameter is missing")
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id parameter is required"})
		return
	}

	h.logger.Info("📋 Getting group invitations for user", zap.String("user_id", userID))

	// Create event to request group invitations from group service
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "group.invite.list",
		"data": map[string]interface{}{
			"user_id": userID,
			"status":  "pending",
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Requesting group invitations from group service",
		zap.String("event_id", eventID),
		zap.String("user_id", userID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to get group invitations",
			zap.Error(err),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve group invitations: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Get group invitations failed",
			zap.String("error", response.Error),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve group invitations: " + response.Error})
		return
	}

	// Extract invitations from response
	h.logger.Info("📦 Processing group invitations response",
		zap.String("event_id", eventID),
		zap.Any("response_data", response.Data))

	// The response data should contain the invitations
	var invitations []interface{}

	switch data := response.Data.(type) {
	case []interface{}:
		// Case 1: The response is directly an array of invitations
		invitations = data
		h.logger.Info("✅ Formato de respuesta: array directo de invitaciones")

	case map[string]interface{}:
		// Case 2: The response is an object that contains invitations
		if invitationsField, exists := data["invitations"]; exists {
			if invitationsArray, ok := invitationsField.([]interface{}); ok {
				invitations = invitationsArray
				h.logger.Info("✅ Formato de respuesta: objeto con campo 'invitations'")
			}
		}
	}

	h.logger.Info("✅ Group invitations processing completed",
		zap.String("user_id", userID),
		zap.Int("invitations_count", len(invitations)))

	// Always return an array, even if empty
	c.JSON(http.StatusOK, gin.H{"invitations": invitations})
}

type LeaveGroupRequest struct {
	GroupID string `json:"group_id" binding:"required"`
	UserID  string `json:"user_id" binding:"required"`
}

func (h *GroupHandler) CreateGroupEvent(c *gin.Context) {
	var req CreateGroupEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing create group event request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log the parsed request for debugging
	h.logger.Info("📥 CreateGroupEvent parsed request",
		zap.String("group_id", req.GroupID),
		zap.String("event_id", req.EventID),
		zap.String("user_id", req.UserID),
		zap.Bool("is_hierarchical", req.IsHierarchical))

	h.logger.Info("📅 Creating group event",
		zap.String("group_id", req.GroupID),
		zap.String("event_id", req.EventID),
		zap.Bool("is_hierarchical", req.IsHierarchical))

	// Create the group event
	groupEventID := uuid.New().String()

	groupEventData := map[string]interface{}{
		"id":   groupEventID,
		"type": "group.event.create",
		"data": map[string]interface{}{
			"group_id":        req.GroupID,
			"event_id":        req.EventID,
			"is_hierarchical": req.IsHierarchical,
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	// Add the user_id field
	groupEventData["data"].(map[string]interface{})["user_id"] = req.UserID

	h.logger.Info("📤 Creating group event",
		zap.String("group_event_id", groupEventID),
		zap.String("group_id", req.GroupID),
		zap.String("event_id", req.EventID))

	// Send group event creation request
	groupResponse, err := h.sendEventAndWaitForResponse(c.Request.Context(), groupEventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to create group event",
			zap.Error(err),
			zap.String("group_id", req.GroupID),
			zap.String("event_id", req.EventID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group event: " + err.Error()})
		return
	}

	if !groupResponse.Success {
		h.logger.Warn("⚠️ Group event creation failed",
			zap.String("error", groupResponse.Error),
			zap.String("group_id", req.GroupID),
			zap.String("event_id", req.EventID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to create group event: " + groupResponse.Error})
		return
	}

	h.logger.Info("✅ Group event created successfully",
		zap.String("group_id", req.GroupID),
		zap.String("event_id", req.EventID))

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Group event created successfully",
		"group_id": req.GroupID,
		"event_id": req.EventID,
	})
}

func (h *GroupHandler) LeaveGroup(c *gin.Context) {
	var req LeaveGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("❌ Error parsing leave group request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("🚪 User leaving group",
		zap.String("group_id", req.GroupID),
		zap.String("user_id", req.UserID))

	// First, get the user's email from the user service
	userEmail, err := h.getUserEmailByID(c.Request.Context(), req.UserID)
	if err != nil {
		h.logger.Error("❌ Failed to get user email for leave group operation",
			zap.Error(err),
			zap.String("user_id", req.UserID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user information: " + err.Error()})
		return
	}

	h.logger.Info("✅ Retrieved user email for leave group operation",
		zap.String("user_id", req.UserID),
		zap.String("email", userEmail))

	// Create event to remove user from group
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "group.member.remove",
		"data": map[string]interface{}{
			"group_id": req.GroupID,
			"email":    userEmail, // Use the actual email
		},
		"metadata": map[string]string{
			"reply_to": "group_events_response",
		},
	}

	h.logger.Info("📤 Sending group leave event",
		zap.String("event_id", eventID),
		zap.String("group_id", req.GroupID),
		zap.String("email", userEmail))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData, "group_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to leave group",
			zap.Error(err),
			zap.String("group_id", req.GroupID),
			zap.String("email", userEmail))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave group: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Leave group failed",
			zap.String("error", response.Error),
			zap.String("group_id", req.GroupID),
			zap.String("email", userEmail))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to leave group: " + response.Error})
		return
	}

	h.logger.Info("✅ User left group successfully",
		zap.String("group_id", req.GroupID),
		zap.String("email", userEmail))

	c.JSON(http.StatusOK, gin.H{
		"message":  "Left group successfully",
		"group_id": req.GroupID,
		"user_id":  req.UserID,
	})
}
