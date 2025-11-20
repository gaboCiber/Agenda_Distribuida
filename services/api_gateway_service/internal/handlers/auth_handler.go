package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AuthHandler struct {
	redis     *redis.Client
	jwtSecret string
	jwtExpiry time.Duration
	logger    *zap.Logger
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

func NewAuthHandler(redisClient *redis.Client, jwtSecret string, jwtExpiry time.Duration, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		redis:     redisClient,
		jwtSecret: jwtSecret,
		jwtExpiry: jwtExpiry,
		logger:    logger,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate user ID
	userID := uuid.New()

	// Publish to Redis (simulate publishing user_created event)
	event := map[string]interface{}{
		"event":     "user_created",
		"user_id":   userID.String(),
		"username":  req.Username,
		"email":     req.Email,
		"timestamp": time.Now().Unix(),
	}

	// Marshal to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("Failed to marshal user_created event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	if err := h.redis.Publish(c.Request.Context(), "agenda-events", eventJSON).Err(); err != nil {
		h.logger.Error("Failed to publish user_created event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	h.logger.Info("User registered", zap.String("user_id", userID.String()))
	c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully", "user_id": userID})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// For demo, assume login is successful and return a JWT
	// In real implementation, verify credentials via DB service
	userID := uuid.New() // Mock user ID

	token, err := h.generateJWT(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	response := LoginResponse{
		Token:  token,
		UserID: userID,
	}

	h.logger.Info("User logged in", zap.String("user_id", userID.String()))
	c.JSON(http.StatusOK, response)
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
