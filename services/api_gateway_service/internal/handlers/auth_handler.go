package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/agenda-distribuida/api-gateway-service/internal/loadbalancer"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AuthHandler struct {
	jwtSecret       string
	jwtExpiry       time.Duration
	responseHandler *ResponseHandler
	logger          *zap.Logger
	loadBalancer    *loadbalancer.LoadBalancer
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

func NewAuthHandler(jwtSecret string, jwtExpiry time.Duration, responseHandler *ResponseHandler, loadBalancer *loadbalancer.LoadBalancer, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		jwtSecret:       jwtSecret,
		jwtExpiry:       jwtExpiry,
		responseHandler: responseHandler,
		loadBalancer:    loadBalancer,
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
	}

	// Send event and wait for response
	h.logger.Info("📤 Enviando evento de registro de usuario",
		zap.String("event_id", eventID),
		zap.String("email", req.Email))

	// Use background context to avoid cancellation issues
	response, err := h.sendEventAndWaitForResponse(context.Background(), eventData)
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
	}

	// DEBUG: Log the event data before sending
	h.logger.Info("📤 Evento creado antes de enviar",
		zap.Any("event_data", eventData),
		zap.String("event_id", eventID))

	// Send event and wait for response
	response, err := h.sendEventAndWaitForResponse(context.Background(), eventData)
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
func (h *AuthHandler) sendEventAndWaitForResponse(ctx context.Context, eventData interface{}) (*UserEventResponse, error) {
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

func (h *AuthHandler) generateJWT(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     time.Now().Add(h.jwtExpiry).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

// GetUserByID obtiene la información de un usuario por su ID
func (h *AuthHandler) GetUserByID(ctx context.Context, userID string) (map[string]interface{}, error) {
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "user.get",
		"data": map[string]interface{}{
			"user_id": userID,
		},
	}

	h.logger.Info("📤 Requesting user info by ID",
		zap.String("event_id", eventID),
		zap.String("user_id", userID))

	// Usar sendEventAndWaitForResponse para enviar el evento y esperar respuesta
	response, err := h.sendEventAndWaitForResponse(ctx, eventData)
	if err != nil {
		h.logger.Error("❌ Failed to get user info by ID",
			zap.Error(err),
			zap.String("user_id", userID))
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	if !response.Success {
		h.logger.Warn("⚠️ Get user info by ID failed",
			zap.String("user_id", userID),
			zap.String("error", response.Error))
		return nil, fmt.Errorf("user service error: %s", response.Error)
	}

	// Convertir la respuesta a map[string]interface{}
	userData, ok := response.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid user data format")
	}

	return userData, nil
}

// GetUserByEmail obtiene la información de un usuario por su correo electrónico
func (h *AuthHandler) GetUserByEmail(ctx context.Context, email string) (map[string]interface{}, error) {
	eventID := uuid.New().String()

	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "user.get.by.email",
		"data": map[string]interface{}{
			"email": email,
		},
	}

	h.logger.Info("📤 Requesting user info by email",
		zap.String("event_id", eventID),
		zap.String("email", email))

	// Usar sendEventAndWaitForResponse para enviar el evento y esperar respuesta
	response, err := h.sendEventAndWaitForResponse(ctx, eventData)
	if err != nil {
		h.logger.Error("❌ Failed to get user info by email",
			zap.Error(err),
			zap.String("email", email))
		return nil, fmt.Errorf("failed to get user info by email: %w", err)
	}

	if !response.Success {
		h.logger.Warn("⚠️ Get user info by email failed",
			zap.String("email", email),
			zap.String("error", response.Error))
		return nil, fmt.Errorf("user service error: %s", response.Error)
	}

	// Convertir la respuesta a map[string]interface{}
	userData, ok := response.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid user data format")
	}

	return userData, nil
}

// DeleteAccount handles account deletion
func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	// Get user_id from query parameter (should be extracted from JWT in production)
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id parameter is required"})
		return
	}

	// Create event for user deletion
	eventID := uuid.New().String()
	eventData := map[string]interface{}{
		"id":   eventID,
		"type": "user.delete",
		"data": map[string]interface{}{
			"user_id": userID,
		},
	}

	h.logger.Info("📤 Sending delete account event",
		zap.String("event_id", eventID),
		zap.String("user_id", userID))

	// Usar sendEventAndWaitForResponse para enviar el evento y esperar respuesta
	response, err := h.sendEventAndWaitForResponse(c.Request.Context(), eventData)
	if err != nil {
		h.logger.Error("❌ Failed to delete account",
			zap.Error(err),
			zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account: " + err.Error()})
		return
	}

	if !response.Success {
		h.logger.Warn("⚠️ Account deletion failed",
			zap.String("user_id", userID),
			zap.String("error", response.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account: " + response.Error})
		return
	}

	h.logger.Info("✅ Account deleted successfully",
		zap.String("user_id", userID),
		zap.Any("response", response.Data))
	c.JSON(http.StatusOK, gin.H{
		"message": "Account deleted successfully",
		"data":    response.Data,
	})
}
