package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"redis_supervisor_service/internal/clients"
	"redis_supervisor_service/internal/config"
	"redis_supervisor_service/internal/election"
	httpHandler "redis_supervisor_service/internal/http"
	"redis_supervisor_service/internal/supervisor"
)

func main() {
	// Create a cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling to cancel the context on interrupt
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		log.Println("Shutdown signal received, cancelling context.")
		cancel()
	}()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Println("Starting Redis Supervisor Service...")
	log.Printf("Monitoring Redis nodes: %v", cfg.RedisAddrs)
	log.Printf("DB Service URL: %s", cfg.DBServiceURL)
	log.Printf("Raft Nodes URLs: %v", cfg.RaftNodesURLs)
	log.Printf("Supervisor ID: %s", cfg.SupervisorID)
	log.Printf("Supervisor Bind Address: %s", cfg.SupervisorBindAddr)
	log.Printf("Supervisor Peers: %v", cfg.SupervisorPeers)

	// Initialize clients
	redisClient := clients.NewRedisClient()
	dbClient := clients.NewDBClient(cfg.DBServiceURL, cfg.RaftNodesURLs)

	// Initialize and start the leader elector
	peersMap := make(map[string]string)
	for _, p := range cfg.SupervisorPeers {
		peersMap[p.ID] = p.Address
	}
	elector, err := election.NewElector(cfg.SupervisorID, cfg.SupervisorBindAddr, peersMap, log.Default())
	if err != nil {
		log.Fatalf("Failed to create elector: %v", err)
	}

	if err := elector.Start(ctx); err != nil {
		log.Fatalf("Failed to start elector: %v", err)
	}
	defer elector.Stop()

	// Create and run the supervisor
	sup := supervisor.New(cfg, redisClient, dbClient, elector)
	go sup.Run(ctx)

	// Setup HTTP server
	mux := httpHandler.SetupRoutes(elector)
	httpPort := cfg.HTTPPort
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: mux,
	}

	// Start HTTP server
	go func() {
		log.Printf("HTTP server starting on port %d", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for context to be cancelled (e.g., by SIGINT)
	<-ctx.Done()

	// Gracefully shutdown HTTP server
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Redis Supervisor Service shutting down gracefully.")
}
