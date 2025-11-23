package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/agenda-distribuida/api-gateway-service/internal/clients"
)

type GroupHandler struct {
	redis    *redis.Client
	dbClient *clients.DBClient
	logger   *zap.Logger
}

type CreateGroupRequest struct {
	Name           string `json:"name" binding:"required"`
	Description    string `json:"description"`
	UserID         string `json:"user_id" binding:"required"`
	IsHierarchical bool   `json:"is_hierarchical"`
}

func NewGroupHandler(redisClient *redis.Client, dbClient *clients.DBClient, logger *zap.Logger) *GroupHandler {
	return &GroupHandler{
		redis:    redisClient,
		dbClient: dbClient,
		logger:   logger,
	}
}

func (h *GroupHandler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupID := uuid.New()

	event := map[string]interface{}{
		"event":           "group_create_requested",
		"group_id":        groupID.String(),
		"name":            req.Name,
		"description":     req.Description,
		"user_id":         req.UserID,
		"is_hierarchical": req.IsHierarchical,
		"timestamp":       time.Now().Unix(),
		"request_id":      uuid.New().String(),
	}

	// Marshal to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("Failed to marshal group_created", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		return
	}

	if err := h.redis.Publish(c.Request.Context(), "agenda-events", eventJSON).Err(); err != nil {
		h.logger.Error("Failed to publish group_create_requested", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		return
	}

	h.logger.Info("Group creation requested", zap.String("group_id", groupID.String()))
	c.JSON(http.StatusAccepted, gin.H{"message": "Group creation request sent", "group_id": groupID})
}

func (h *GroupHandler) GetGroups(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id parameter is required"})
		return
	}

	groups, err := h.dbClient.GetGroups(userID)
	if err != nil {
		h.logger.Error("Failed to get groups from DB service", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve groups"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"groups": groups})
}
