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

type EventHandler struct {
	redis  *redis.Client
	logger *zap.Logger
}

type CreateEventRequest struct {
	Title       string    `json:"title" binding:"required"`
	Description string    `json:"description"`
	StartTime   time.Time `json:"start_time" binding:"required"`
	EndTime     time.Time `json:"end_time" binding:"required"`
	UserID      string    `json:"user_id" binding:"required"`
	GroupID     *string   `json:"group_id,omitempty"`
}

func NewEventHandler(redisClient *redis.Client, logger *zap.Logger) *EventHandler {
	return &EventHandler{
		redis:  redisClient,
		logger: logger,
	}
}

func (h *EventHandler) CreateEvent(c *gin.Context) {
	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	eventID := uuid.New()

	event := map[string]interface{}{
		"event":       "event_created",
		"event_id":    eventID.String(),
		"title":       req.Title,
		"description": req.Description,
		"start_time":  req.StartTime.Unix(),
		"end_time":    req.EndTime.Unix(),
		"user_id":     req.UserID,
		"group_id":    req.GroupID,
		"timestamp":   time.Now().Unix(),
	}

	// Marshal to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("Failed to marshal event_created", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create event"})
		return
	}

	if err := h.redis.Publish(c.Request.Context(), "agenda-events", eventJSON).Err(); err != nil {
		h.logger.Error("Failed to publish event_created", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create event"})
		return
	}

	h.logger.Info("Event created", zap.String("event_id", eventID.String()))
	c.JSON(http.StatusCreated, gin.H{"message": "Event created successfully", "event_id": eventID})
}

func (h *EventHandler) GetEvents(c *gin.Context) {
	// Placeholder: in real implementation, query DB service
	c.JSON(http.StatusOK, gin.H{"events": []string{}})
}
