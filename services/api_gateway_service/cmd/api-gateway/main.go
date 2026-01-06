package main

import (
	"context"
	"fmt"
	"log"
	"net"
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
	"github.com/agenda-distribuida/api-gateway-service/internal/loadbalancer"
	"github.com/agenda-distribuida/api-gateway-service/internal/services"
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

	// Ping Redis (no fatal si falla, el ResponseHandler se reconectará automáticamente)
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("Failed to connect to Redis at startup, service will continue and reconnect automatically", zap.Error(err))
	}
	logger.Info("Redis connection attempted", zap.String("url", cfg.Redis.URL))

	// Get the host and port for this service
	host, err := GetContainerIP()
	if err != nil {
		logger.Error("Failed to get container IP", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("Container IP", zap.String("ip", host))

	serviceAddr := fmt.Sprintf("%s:%s", host, cfg.Server.Port)

	// Initialize DB client with registry support
	dbClient := clients.NewDBClient(cfg.Server.Name, serviceAddr, cfg.DBService.URL, logger)
	// Set Raft nodes for leader discovery
	dbClient.SetRaftNodes(cfg.RaftNodesURLs)

	// Register this service
	// if err := dbClient.RegisterService(""); err != nil {
	// 	logger.Error("Failed to register service", zap.Error(err))
	// } else {
	// 	logger.Info("Successfully registered service with registry")

	// 	// List all registered services for debugging
	// 	if services, err := dbClient.ListServices(); err != nil {
	// 		logger.Error("Failed to list services", zap.Error(err))
	// 	} else {
	// 		logger.Info("Discovered services in registry", zap.Int("count", len(services)))
	// 		for _, svc := range services {
	// 			logger.Info("Service",
	// 				zap.String("name", svc.ServiceName),
	// 				zap.String("address", svc.Address))
	// 		}
	// 	}

	// 	// Deregister on shutdown
	// 	defer func() {
	// 		if err := dbClient.DeregisterService(); err != nil {
	// 			logger.Error("Failed to deregister service", zap.Error(err))
	// 		} else {
	// 			logger.Info("Successfully deregistered service")
	// 		}
	// 	}()
	// }

	// Initialize EventService
	eventService := services.NewEventService(dbClient, logger)

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
	responseHandler := handlers.NewResponseHandler(redisClient, eventService, cfg.RaftNodesURLs, cfg.Redis.URL, logger)

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start global response listener
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("PANIC in ResponseHandler goroutine",
					zap.Any("recover", r),
					zap.String("stack", string(debug.Stack())))
			}
		}()
		if err := responseHandler.StartResponseListener(ctx); err != nil {
			logger.Error("Response listener stopped with error", zap.Error(err))
		}
	}()

	// Initialize load balancer with node monitoring
	loadBalancer := loadbalancer.NewLoadBalancer(cfg.UserNodesURLs, cfg.GroupNodesURLs, logger)
	logger.Info("Load balancer con monitoreo de nodos activos",
		zap.Strings("nodos_usuario", cfg.UserNodesURLs),
		zap.Strings("nodos_grupo", cfg.GroupNodesURLs))

	// Registrar manejador para apagado limpio
	go func() {
		<-ctx.Done()
		logger.Info("Deteniendo balanceador de carga...")
		loadBalancer.Stop()
	}()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg.JWT.Secret, cfg.JWT.Expiration, responseHandler, loadBalancer, logger)
	eventHandler := handlers.NewEventHandler(dbClient, responseHandler, logger, loadBalancer)
	groupHandler := handlers.NewGroupHandler(dbClient, responseHandler, logger, loadBalancer, authHandler)

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
			events.POST("/update", eventHandler.UpdateEvent)
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

	// Cancel the context to stop the response listener
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

// GetContainerIP obtiene la IP del contenedor actual
func GetContainerIP() (string, error) {
	// Primero intentamos con el nombre del host (funciona en Docker)
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		// Intentar resolver el hostname a una IP
		addrs, err := net.LookupIP(hostname)
		if err == nil {
			for _, addr := range addrs {
				if ipv4 := addr.To4(); ipv4 != nil {
					return ipv4.String(), nil
				}
			}
		}
	}

	// Si lo anterior falla, intentamos con las interfaces de red
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("error obteniendo interfaces de red: %v", err)
	}

	for _, iface := range ifaces {
		// Ignorar interfaces apagadas o loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Verificar que sea una dirección IPv4 válida
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			// Verificar que no sea una dirección link-local
			if !ip.IsLinkLocalUnicast() {
				return ip.String(), nil
			}
		}
	}

	// Último recurso: usar la variable de entorno HOSTNAME (común en Docker)
	if hostIP := os.Getenv("HOSTNAME"); hostIP != "" {
		return hostIP, nil
	}

	return "", fmt.Errorf("no se pudo determinar la IP del contenedor")
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
		// Log todas las peticiones para depuración
		fmt.Printf("[HTTP-DEBUG] %s %s from %s\n", c.Request.Method, c.Request.URL.Path, c.Request.RemoteAddr)

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
