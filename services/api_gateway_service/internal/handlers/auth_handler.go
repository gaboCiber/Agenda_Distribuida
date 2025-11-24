package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AuthHandler struct {
	redis           *redis.Client
	jwtSecret       string
	jwtExpiry       time.Duration
	responseHandler *ResponseHandler
	logger          *zap.Logger
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token  string    `json:"token"`
	UserID uuid.UUID `json:"user_id"`
}

func NewAuthHandler(redisClient *redis.Client, jwtSecret string, jwtExpiry time.Duration, responseHandler *ResponseHandler, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		redis:           redisClient,
		jwtSecret:       jwtSecret,
		jwtExpiry:       jwtExpiry,
		responseHandler: responseHandler,
		logger:          logger,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create event for user service
	eventID := uuid.New().String()

	// Create event as map to avoid any struct marshaling issues
	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "user.create",
		"data": map[string]interface{}{
			"username": req.Username,
			"email":    req.Email,
			"password": req.Password, // user_service will hash it
		},
		"metadata": map[string]string{
			"reply_to": "users_events_response",
		},
	}

	// Send event and wait for response
	h.logger.Info("📤 Enviando evento de registro de usuario",
		zap.String("event_id", eventID),
		zap.String("email", req.Email))

	// Use background context to avoid cancellation issues
	response, err := h.sendEventAndWaitForResponse(context.Background(), eventData, "users_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to register user",
			zap.Error(err),
			zap.String("error_type", "timeout_or_connection"))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ User registration failed",
			zap.String("error", response.Error))
		c.JSON(http.StatusBadRequest, gin.H{"error": response.Error})
		return
	}

	// Extract user ID from response
	h.logger.Info("📦 Procesando respuesta exitosa",
		zap.String("event_id", eventID),
		zap.Any("response_data", response.Data))

	data, ok := response.Data.(map[string]interface{})
	if !ok {
		h.logger.Error("❌ Invalid response data format",
			zap.Any("response_data", response.Data))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response from user service"})
		return
	}

	userID, ok := data["id"].(string) // user_service returns "id", not "user_id"
	if !ok {
		h.logger.Error("❌ User ID not found in response",
			zap.Any("response_data", data),
			zap.Any("full_response", response))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User ID not found in response"})
		return
	}

	h.logger.Info("✅ User registered successfully",
		zap.String("user_id", userID),
		zap.String("email", req.Email))
	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user_id": userID,
		"email":   req.Email,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create event for user service
	eventID := uuid.New().String()

	// Create event as map to avoid any struct marshaling issues
	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "user.login",
		"data": map[string]interface{}{
			"email":    req.Email,
			"password": req.Password, // Plain text - user service will hash and compare
		},
		"metadata": map[string]string{
			"reply_to": "users_events_response",
		},
	}

	// DEBUG: Log the event data before sending
	h.logger.Info("📤 Evento creado antes de enviar",
		zap.Any("event_data", eventData),
		zap.String("event_id", eventID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(context.Background(), eventData, "users_events_response")
	if err != nil {
		h.logger.Error("❌ Failed to login user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process login: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Login failed",
			zap.String("email", req.Email),
			zap.String("error", response.Error))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Extract user data from response
	data, ok := response.Data.(map[string]interface{})
	if !ok {
		h.logger.Error("❌ Invalid response data format")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response from user service"})
		return
	}

	userIDStr, ok := data["id"].(string) // user_service returns "id", not "user_id"
	if !ok {
		h.logger.Error("❌ User ID not found in response", zap.Any("response_data", data))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response from user service"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error("❌ Invalid user ID format",
			zap.String("userID", userIDStr),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Generate JWT token
	token, err := h.generateJWT(userID)
	if err != nil {
		h.logger.Error("❌ Failed to generate JWT token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	result := LoginResponse{
		Token:  token,
		UserID: userID,
	}

	h.logger.Info("✅ User logged in successfully",
		zap.String("user_id", userID.String()),
		zap.String("email", req.Email))
	c.JSON(http.StatusOK, result)
}

// sendEventAndWaitForResponse publishes an event and waits for a response using the response handler
func (h *AuthHandler) sendEventAndWaitForResponse(ctx context.Context, eventData interface{}, replyChannel string) (*UserEventResponse, error) {
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

	// Publish event to user service channel
	if err := h.redis.Publish(ctx, "users_events", eventJSON).Err(); err != nil {
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	h.logger.Info("✅ Evento ENVIADO al user_service",
		zap.String("event_id", eventID),
		zap.String("channel", "users_events"))

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

func (h *AuthHandler) generateJWT(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     time.Now().Add(h.jwtExpiry).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	// Get user_id from query parameter (should be extracted from JWT in production)
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id parameter is required"})
		return
	}

	h.logger.Info("🗑️ Deleting user account", zap.String("user_id", userID))

	// Create event for user deletion
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "user.delete",
		"data": map[string]interface{}{
			"user_id": userID,
		},
		"metadata": map[string]string{
			"reply_to": "users_events_response",
		},
	}

	// Marshal event to JSON
	eventJSON, err := json.Marshal(eventData)
	if err != nil {
		h.logger.Error("Failed to marshal user.delete event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account"})
		return
	}

	// Publish to Redis
	if err := h.redis.Publish(c.Request.Context(), "users_events", eventJSON).Err(); err != nil {
		h.logger.Error("Failed to publish user.delete event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account"})
		return
	}

	h.logger.Info("✅ Account deletion requested", zap.String("user_id", userID))
	c.JSON(http.StatusOK, gin.H{"message": "Account deletion requested successfully"})
}
