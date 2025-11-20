package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type GroupHandler struct {
	redis  *redis.Client
	logger *zap.Logger
}

type CreateGroupRequest struct {
	Name           string `json:"name" binding:"required"`
	Description    string `json:"description"`
	UserID         string `json:"user_id" binding:"required"`
	IsHierarchical bool   `json:"is_hierarchical"`
}

func NewGroupHandler(redisClient *redis.Client, logger *zap.Logger) *GroupHandler {
	return &GroupHandler{
		redis:  redisClient,
		logger: logger,
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
		"event":           "group_created",
		"group_id":        groupID.String(),
		"name":            req.Name,
		"description":     req.Description,
		"user_id":         req.UserID,
		"is_hierarchical": req.IsHierarchical,
		"timestamp":       time.Now().Unix(),
	}

	// Marshal to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("Failed to marshal group_created", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		return
	}

	if err := h.redis.Publish(c.Request.Context(), "agenda-events", eventJSON).Err(); err != nil {
		h.logger.Error("Failed to publish group_created", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		return
	}

	h.logger.Info("Group created", zap.String("group_id", groupID.String()))
	c.JSON(http.StatusCreated, gin.H{"message": "Group created successfully", "group_id": groupID})
}

func (h *GroupHandler) GetGroups(c *gin.Context) {
	// Placeholder: in real implementation, query DB service
	c.JSON(http.StatusOK, gin.H{"groups": []string{}})
}
