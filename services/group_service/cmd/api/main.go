package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agenda-distribuida/group-service/internal/api/handlers"
	"github.com/agenda-distribuida/group-service/internal/config"
	"github.com/agenda-distribuida/group-service/internal/database"
	"github.com/agenda-distribuida/group-service/internal/events"
	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	db, err := database.Init(database.Config{
		Driver: "sqlite3",
		DSN:    cfg.DatabasePath,
	})
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	// Initialize Redis client for event publishing
	redisClient := events.NewRedisClient(cfg.RedisURL)
	defer redisClient.Close()

	// Initialize event publisher
	eventPublisher := events.NewPublisher(redisClient)
	_ = eventPublisher // Use the publisher if needed

	// Initialize database models
	dbModels := models.NewDatabase(db)

	// Initialize handlers
	groupHandler := handlers.NewGroupHandler(dbModels)
	memberHandler := handlers.NewMemberHandler(dbModels)
	invitationHandler := handlers.NewInvitationHandler(dbModels)

	// Set up router
	router := setupRouter(groupHandler, memberHandler, invitationHandler)

	// Add CORS middleware
	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // In production, replace with your frontend URL
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-User-ID"},
		AllowCredentials: true,
	}).Handler(router)

	// Create HTTP server
	server := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: handler,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("🚀 Server starting on %s", cfg.ServerAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Create a deadline to wait for
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	// Attempt to gracefully shut down the server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

// setupRouter configures the HTTP routes
func setupRouter(
	groupHandler *handlers.GroupHandler,
	memberHandler *handlers.MemberHandler,
	invitationHandler *handlers.InvitationHandler,
) *mux.Router {
	r := mux.NewRouter()

	// Group routes
	groupRouter := r.PathPrefix("/groups").Subrouter()
	groupRouter.HandleFunc("", groupHandler.CreateGroup).Methods("POST")
	groupRouter.HandleFunc("/{id}", groupHandler.GetGroup).Methods("GET")
	groupRouter.HandleFunc("/{id}", groupHandler.UpdateGroup).Methods("PUT")
	groupRouter.HandleFunc("/{id}", groupHandler.DeleteGroup).Methods("DELETE")
	groupRouter.HandleFunc("/user/{userID}", groupHandler.GetUserGroups).Methods("GET")

	// Member routes
	memberRouter := r.PathPrefix("/groups/{groupID}/members").Subrouter()
	memberRouter.HandleFunc("", memberHandler.AddMember).Methods("POST")
	memberRouter.HandleFunc("/{userID}", memberHandler.RemoveMember).Methods("DELETE")
	memberRouter.HandleFunc("", memberHandler.ListMembers).Methods("GET")
	memberRouter.HandleFunc("/admins", memberHandler.GetAdmins).Methods("GET")

	// Invitation routes
	invitationRouter := r.PathPrefix("/invitations").Subrouter()
	invitationRouter.HandleFunc("", invitationHandler.CreateInvitation).Methods("POST")
	invitationRouter.HandleFunc("/{invitationID}", invitationHandler.RespondToInvitation).Methods("POST")
	invitationRouter.HandleFunc("/user/{userID}", invitationHandler.GetUserInvitations).Methods("GET")
	invitationRouter.HandleFunc("/{invitationID}", invitationHandler.GetInvitation).Methods("GET")

	// Add request logging middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
			next.ServeHTTP(w, r)
		})
	})

	// Add panic recovery middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("Recovered from panic: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	})

	return r
}
