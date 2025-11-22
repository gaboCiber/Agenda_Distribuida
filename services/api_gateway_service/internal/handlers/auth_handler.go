package handlers

import (
	"net/http"
	"time"

	"github.com/agenda-distribuida/api-gateway-service/internal/clients"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	redis     *redis.Client
	dbClient  *clients.DBClient
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

func NewAuthHandler(redisClient *redis.Client, dbClient *clients.DBClient, jwtSecret string, jwtExpiry time.Duration, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		redis:     redisClient,
		dbClient:  dbClient,
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

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("Failed to hash password", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Register user directly in database via db_service
	userID, err := h.dbClient.RegisterUser(req.Username, req.Email, string(hashedPassword))
	if err != nil {
		h.logger.Error("Failed to register user in database", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	h.logger.Info("User registered successfully", zap.String("user_id", userID), zap.String("email", req.Email))
	c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully", "user_id": userID})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify credentials directly in database via db_service
	userData, err := h.dbClient.LoginUser(req.Email, req.Password)
	if err != nil {
		h.logger.Warn("Login failed for user", zap.String("email", req.Email), zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Extract user ID from response
	userIDStr, ok := userData["user_id"].(string)
	if !ok {
		h.logger.Error("Invalid user data format", zap.Any("userData", userData))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user data"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error("Invalid user ID format", zap.String("userID", userIDStr), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Generate JWT token
	token, err := h.generateJWT(userID)
	if err != nil {
		h.logger.Error("Failed to generate JWT token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	response := LoginResponse{
		Token:  token,
		UserID: userID,
	}

	h.logger.Info("User logged in successfully", zap.String("user_id", userID.String()), zap.String("email", req.Email))
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
