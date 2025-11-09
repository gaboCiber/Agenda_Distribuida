package server

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

var validate = validator.New()

type Server struct {
	Server  *http.Server
	log     *zerolog.Logger
	db      *sql.DB
	userAPI *UserHandler
}

func New(addr string, db *sql.DB, log *zerolog.Logger) *Server {
	// Initialize repository and handlers
	userRepo := repository.NewUserRepository(db, *log)
	userAPI := NewUserHandler(userRepo, log)

	s := &Server{
		Server: &http.Server{
			Addr:         addr,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		db:      db,
		log:     log,
		userAPI: userAPI,
	}

	// Setup routes
	r := mux.NewRouter()
	s.setupRoutes(r)
	s.Server.Handler = r

	return s
}

func (s *Server) setupRoutes(r *mux.Router) {
	// Use the logging middleware for all routes
	r.Use(s.loggingMiddleware)

	// Health check endpoint
	r.HandleFunc("/health", s.healthCheck).Methods("GET")

	// API v1 routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Users routes
	users := api.PathPrefix("/users").Subrouter()
	users.HandleFunc("", s.userAPI.CreateUser).Methods("POST")
	users.HandleFunc("/{id}", s.userAPI.GetUser).Methods("GET")
	users.HandleFunc("/{id}", s.userAPI.UpdateUser).Methods("PUT")
	users.HandleFunc("/{id}", s.userAPI.DeleteUser).Methods("DELETE")

	// Groups routes
	groups := api.PathPrefix("/groups").Subrouter()
	groups.HandleFunc("", s.createGroup).Methods("POST")
	groups.HandleFunc("/{id}", s.getGroup).Methods("GET")
	groups.HandleFunc("/{id}", s.updateGroup).Methods("PUT")
	groups.HandleFunc("/{id}", s.deleteGroup).Methods("DELETE")

	// Group members routes
	groups.HandleFunc("/{id}/members", s.getGroupMembers).Methods("GET")
	groups.HandleFunc("/{id}/members", s.addGroupMember).Methods("POST")
	groups.HandleFunc("/{id}/members/{user_id}", s.removeGroupMember).Methods("DELETE")

	// Events routes
	events := api.PathPrefix("/events").Subrouter()
	events.HandleFunc("", s.createEvent).Methods("POST")
	events.HandleFunc("/{id}", s.getEvent).Methods("GET")
	events.HandleFunc("/{id}", s.updateEvent).Methods("PUT")
	events.HandleFunc("/{id}", s.deleteEvent).Methods("DELETE")

	// Group events routes
	groups.HandleFunc("/{id}/events", s.getGroupEvents).Methods("GET")
	groups.HandleFunc("/{id}/events", s.addGroupEvent).Methods("POST")
	groups.HandleFunc("/{id}/events/{event_id}", s.removeGroupEvent).Methods("DELETE")

	// Event status routes
	events.HandleFunc("/{id}/status", s.updateEventStatus).Methods("POST")
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.log.Info().Str("address", s.Server.Addr).Msg("Starting server")
	return s.Server.ListenAndServe()
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	s.log.Info().Msg("Shutting down server")
	return s.Server.Shutdown(ctx)
}

// loggingMiddleware logs all incoming requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer to capture the status code
		rw := &responseWriter{w, http.StatusOK}

		// Process the request
		next.ServeHTTP(rw, r)

		// Log the request
		duration := time.Since(start)
		s.log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", rw.status).
			Str("duration", duration.String()).
			Msg("Request processed")
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// healthCheck handles the health check endpoint
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if database is initialized
	if s.db == nil {
		s.log.Error().Msg("Database is not initialized")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unhealthy","error":"database not initialized"}`))
		return
	}

	// Check database connection with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		s.log.Error().Err(err).Msg("Database health check failed")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unhealthy","error":"database connection failed"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// notImplemented is a helper function to return 501 Not Implemented
func (s *Server) notImplemented(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"status": "error", "message": "Not implemented"}`))
}

// Group handlers
func (s *Server) createGroup(w http.ResponseWriter, r *http.Request)       { s.notImplemented(w) }
func (s *Server) getGroup(w http.ResponseWriter, r *http.Request)          { s.notImplemented(w) }
func (s *Server) updateGroup(w http.ResponseWriter, r *http.Request)       { s.notImplemented(w) }
func (s *Server) deleteGroup(w http.ResponseWriter, r *http.Request)       { s.notImplemented(w) }
func (s *Server) getGroupMembers(w http.ResponseWriter, r *http.Request)   { s.notImplemented(w) }
func (s *Server) addGroupMember(w http.ResponseWriter, r *http.Request)    { s.notImplemented(w) }
func (s *Server) removeGroupMember(w http.ResponseWriter, r *http.Request) { s.notImplemented(w) }

// Event handlers
func (s *Server) createEvent(w http.ResponseWriter, r *http.Request)       { s.notImplemented(w) }
func (s *Server) getEvent(w http.ResponseWriter, r *http.Request)          { s.notImplemented(w) }
func (s *Server) updateEvent(w http.ResponseWriter, r *http.Request)       { s.notImplemented(w) }
func (s *Server) deleteEvent(w http.ResponseWriter, r *http.Request)       { s.notImplemented(w) }
func (s *Server) getGroupEvents(w http.ResponseWriter, r *http.Request)    { s.notImplemented(w) }
func (s *Server) addGroupEvent(w http.ResponseWriter, r *http.Request)     { s.notImplemented(w) }
func (s *Server) removeGroupEvent(w http.ResponseWriter, r *http.Request)  { s.notImplemented(w) }
func (s *Server) updateEventStatus(w http.ResponseWriter, r *http.Request) { s.notImplemented(w) }
