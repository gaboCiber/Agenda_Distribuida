package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agenda-distribuida/db-service/internal/config"
	"github.com/agenda-distribuida/db-service/internal/database"
	"github.com/agenda-distribuida/db-service/internal/server"
	"github.com/rs/zerolog"
)

func main() {
	// Initialize logger with console writer for better formatting in containers
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}
	logger := zerolog.New(output).With().
		Timestamp().
		Logger()

	// Set the global logger
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.DefaultContextLogger = &logger

	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.New(cfg.Database.Path)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()

	// Create and start server
	srv := server.New(
		cfg.Server.Host+":"+cfg.Server.Port,
		db.DB(),
		&logger,
	)

	// Channel to listen for errors from server
	errChan := make(chan error, 1)
	go func() {
		logger.Info().Str("address", srv.Server.Addr).Msg("Starting server")
		errChan <- srv.Start()
	}()

	// Channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Wait for an error or interrupt signal
	select {
	case err := <-errChan:
		if err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server error")
		}
	case sig := <-quit:
		logger.Info().Str("signal", sig.String()).Msg("Shutting down server...")
	}

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Stop(ctx); err != nil {
		logger.Error().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("Server stopped")
}
