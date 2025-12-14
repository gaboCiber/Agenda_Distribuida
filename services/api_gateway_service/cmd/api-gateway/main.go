package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/agenda-distribuida/api-gateway-service/internal/clients"
	"github.com/agenda-distribuida/api-gateway-service/internal/config"
	"github.com/agenda-distribuida/api-gateway-service/internal/handlers"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	logger := initLogger(cfg.LogLevel)
	defer logger.Sync()

	// Connect to Redis
	redisOpts, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		logger.Fatal("Error parsing Redis URL", zap.Error(err))
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	// Ping Redis
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	logger.Info("Connected to Redis", zap.String("url", cfg.Redis.URL))

	// Get Redis info for debugging
	// redisInfo, err := redisClient.Info(context.Background()).Result()
	// if err != nil {
	// 	logger.Error("Failed to get Redis info", zap.Error(err))
	// } else {
	// 	logger.Info("Redis info", zap.String("version", extractRedisVersion(redisInfo)))
	// }

	// Initialize DB client
	// logger.Info("🔧 Initializing DB client...")
	dbClient := clients.NewDBClient(cfg.DBService.URL, logger)
	// logger.Info("✅ DB client initialized")

	// Set Gin mode
	if cfg.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin router
	r := gin.New()

	// Middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// Initialize response handler for async responses
	// logger.Info("🔧 Initializing ResponseHandler...")
	responseHandler := handlers.NewResponseHandler(logger)
	// logger.Info("✅ ResponseHandler initialized")

	// Start global response listener with proper error handling
	// logger.Info("🚀 About to start global response listener goroutine...")

	go func() {
		// Add panic recovery
		defer func() {
			if r := recover(); r != nil {
				logger.Error("💥 PANIC in ResponseHandler goroutine",
					zap.Any("recover", r),
					zap.String("stack", string(debug.Stack())))
			}
		}()

		// logger.Info("🔄 Inside goroutine - Creating Redis subscription...")

		// Subscribe to channels
		pubsub := redisClient.Subscribe(context.Background(), "users_events_response", "events_response", "groups_events_response", "group_events_response")
		defer func() {
			pubsub.Close()
			// logger.Info("🔴 Redis subscription closed")
		}()

		// Verify subscription
		_, err := pubsub.Receive(context.Background())
		if err != nil {
			logger.Error("❌ Failed to subscribe to Redis channels", zap.Error(err))
			return
		}

		ch := pubsub.Channel()

		// logger.Info("✅✅✅ STARTED GLOBAL RESPONSE LISTENER",
		// 	zap.Strings("channels", []string{"users_events_response", "events_response", "groups_events_response"}))

		// Test that we can receive messages
		go func() {
			time.Sleep(2 * time.Second)
			// logger.Info("🧪 Sending test message to verify pub/sub...")
			testMsg := map[string]interface{}{
				"event_id": "test-event-123",
				"type":     "test",
				"success":  true,
				"data":     map[string]string{"id": "test-user-id"},
			}
			testJSON, _ := json.Marshal(testMsg)
			if err := redisClient.Publish(context.Background(), "users_events_response", string(testJSON)).Err(); err != nil {
				logger.Error("❌ TEST: Error publishing test message", zap.Error(err))
			} else {
				// logger.Info("✅ TEST: Test message published successfully")
			}
		}()

		for msg := range ch {
			// logger.Info("📨📨📨 RESPONSE LISTENER: Received message",
			// 	zap.String("channel", msg.Channel),
			// 	zap.String("payload", msg.Payload),
			// 	zap.Int("payload_length", len(msg.Payload)))

			responseHandler.HandleResponse(msg.Channel, msg.Payload)
		}

		// logger.Warn("❌ Response listener stopped - channel closed")
	}()

	// Wait for ResponseHandler to initialize
	// logger.Info("⏳ Waiting for ResponseHandler to initialize...")
	time.Sleep(3 * time.Second)
	// logger.Info("✅ ResponseHandler should be ready now")

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(redisClient, cfg.JWT.Secret, cfg.JWT.Expiration, responseHandler, logger)
	eventHandler := handlers.NewEventHandler(redisClient, dbClient, responseHandler, logger)
	groupHandler := handlers.NewGroupHandler(redisClient, dbClient, responseHandler, logger)

	// API routes
	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.DELETE("/account", authHandler.DeleteAccount)
		}

		// Protected routes (would need JWT middleware)
		events := api.Group("/events")
		{
			events.POST("", eventHandler.CreateEvent)
			events.GET("", eventHandler.GetEvents)
			events.DELETE("/:id", eventHandler.DeleteEvent)
		}

		groups := api.Group("/groups")
		{
			groups.POST("", groupHandler.CreateGroup)
			groups.GET("", groupHandler.GetGroups)
			groups.GET("/members", groupHandler.GetGroupMembers)
			groups.POST("/invite", groupHandler.InviteUserByEmail)
			groups.GET("/invitations", groupHandler.GetGroupInvitations)
			groups.POST("/invitations/:invitation_id/accept", groupHandler.AcceptGroupInvitation)
			groups.POST("/invitations/:invitation_id/reject", groupHandler.RejectGroupInvitation)
			groups.POST("/:group_id/leave", groupHandler.LeaveGroup)
			groups.POST("/events", groupHandler.CreateGroupEvent)
			groups.GET("/:group_id/events", groupHandler.ListGroupEvents)
			groups.POST("/events/:event_id/accept", groupHandler.AcceptGroupEvent)
			groups.POST("/events/:event_id/decline", groupHandler.DeclineGroupEvent)
			groups.PUT("/:group_id", groupHandler.UpdateGroup)
			groups.DELETE("/:group_id", groupHandler.DeleteGroup)
			groups.PUT("/:group_id/members/:email/role", groupHandler.UpdateMemberRole)
		}
	}

	// Serve static files for UI
	r.Static("/static", "./web/static")
	r.GET("/", func(c *gin.Context) {
		c.File("./web/templates/index.html")
	})

	// Start server
	srv := &http.Server{
		Addr:         cfg.Server.Host + ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		logger.Info("Starting API Gateway server", zap.String("address", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func initLogger(level string) *zap.Logger {
	var logLevel zapcore.Level
	switch level {
	case "debug":
		logLevel = zap.DebugLevel
	case "info":
		logLevel = zap.InfoLevel
	case "warn":
		logLevel = zap.WarnLevel
	case "error":
		logLevel = zapcore.ErrorLevel
	default:
		logLevel = zap.InfoLevel
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(logLevel),
		Development:      false,
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	return logger
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func extractRedisVersion(info string) string {
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "redis_version:") {
			return strings.TrimSpace(strings.Split(line, ":")[1])
		}
	}
	return "unknown"
}
