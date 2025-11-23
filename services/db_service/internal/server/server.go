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
	Server        *http.Server
	log           *zerolog.Logger
	db            *sql.DB
	userAPI       *UserHandler
	eventAPI      *EventHandler
	groupAPI      *GroupHandler
	groupEventAPI *GroupEventHandler
}

func New(addr string, db *sql.DB, log *zerolog.Logger) *Server {
	// Initialize repositories
	userRepo := repository.NewUserRepository(db, *log)
	eventRepo := repository.NewEventRepository(db, *log)
	groupRepo := repository.NewGroupRepository(db, *log)
	groupEventRepo := repository.NewGroupEventRepository(db, *log)

	// Initialize handlers
	userAPI := NewUserHandler(userRepo, log)
	eventAPI := NewEventHandler(eventRepo, log)
	groupAPI := NewGroupHandler(groupRepo, log)
	groupEventAPI := NewGroupEventHandler(groupEventRepo, log)

	s := &Server{
		Server: &http.Server{
			Addr:         addr,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		db:            db,
		log:           log,
		userAPI:       userAPI,
		eventAPI:      eventAPI,
		groupAPI:      groupAPI,
		groupEventAPI: groupEventAPI,
	}

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
	users.HandleFunc("/login", s.userAPI.Login).Methods("POST")

	// Events routes
	events := api.PathPrefix("/events").Subrouter()
	events.HandleFunc("", s.eventAPI.CreateEvent).Methods("POST")
	events.HandleFunc("/{id}", s.eventAPI.GetEvent).Methods("GET")
	events.HandleFunc("/{id}", s.eventAPI.UpdateEvent).Methods("PUT")
	events.HandleFunc("/{id}", s.eventAPI.DeleteEvent).Methods("DELETE")
	events.HandleFunc("/users/{user_id}", s.eventAPI.ListEventsByUser).Methods("GET")

	// Groups routes
	groups := api.PathPrefix("/groups").Subrouter()
	s.groupAPI.RegisterRoutes(groups)

	// Group Event routes
	groupEvents := api.PathPrefix("").Subrouter() // Base path is already /api/v1
	s.groupEventAPI.RegisterRoutes(groupEvents)
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
