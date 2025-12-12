package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"redis_supervisor_service/internal/clients"
	"redis_supervisor_service/internal/config"
	"redis_supervisor_service/internal/supervisor"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Println("Starting Redis Supervisor Service...")
	log.Printf("Monitoring Redis nodes: %v", cfg.RedisAddrs)
	log.Printf("DB Service URL: %s", cfg.DBServiceURL)
	log.Printf("Raft Nodes URLs: %v", cfg.RaftNodesURLs)

	// Initialize clients
	redisClient := clients.NewRedisClient()
	dbClient := clients.NewDBClient(cfg.DBServiceURL, cfg.RaftNodesURLs)
	dockerClient, err := clients.NewDockerClient()
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	// Create and run the supervisor
	sup := supervisor.New(cfg, redisClient, dbClient, dockerClient)
	go sup.Run()

	// Wait for a shutdown signal
	waitForShutdown()
	log.Println("Redis Supervisor Service shutting down.")
}

func waitForShutdown() {
	// Set up a channel to listen for OS signals
	sigs := make(chan os.Signal, 1)
	// Notify this channel for SIGINT (Ctrl+C) and SIGTERM (termination)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	// Block until a signal is received
	<-sigs
}