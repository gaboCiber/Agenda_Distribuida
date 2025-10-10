package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/agenda-distribuida/group-service/internal/config"
	"github.com/agenda-distribuida/group-service/internal/database"
	"github.com/agenda-distribuida/group-service/internal/events"
	"github.com/agenda-distribuida/group-service/internal/server"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Initialize database
	db, err := database.NewSQLiteDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer db.Close()

	// Initialize Redis client for pub/sub
	redisClient := events.NewRedisClient(cfg.RedisURL)
	defer redisClient.Close()

	// Create event publisher
	eventPublisher := events.NewPublisher(redisClient)

	// Initialize and start the HTTP server
	srv := server.NewServer(cfg, db, eventPublisher)
	
	// Start the server in a goroutine
	go func() {
		log.Printf("🚀 Starting Group Service on %s", cfg.ServerAddress)
		if err := srv.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("👋 Server exiting")
}
