package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/agenda-distribuida/db-service/internal/consensus"
	"github.com/agenda-distribuida/db-service/internal/raft_repository"
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
	configAPI     *ConfigHandler
	raftNode      *consensus.RaftNode // Add RaftNode to the server structure
}

func New(addr string, db *sql.DB, log *zerolog.Logger, raftNode *consensus.RaftNode) *Server {
	// Initialize repositories

	// Wrap the user repository with the Raft-aware repository
	sqlUserRepo := repository.NewUserRepository(db, *log)
	raftUserRepo := raft_repository.NewRaftUserRepository(sqlUserRepo, raftNode, log)

	// Wrap the event repository with the Raft-aware repository
	eventRepo := repository.NewEventRepository(db, *log)
	raftEventRepo := raft_repository.NewRaftEventRepository(eventRepo, raftNode, log)

	// Wrap the group repository with the Raft-aware repository
	groupRepo := repository.NewGroupRepository(db, *log)
	raftGroupRepo := raft_repository.NewRaftGroupRepository(groupRepo, raftNode, log)

	// Wrap the group event repository with the Raft-aware repository
	groupEventRepo := repository.NewGroupEventRepository(db, *log)
	raftGroupEventRepo := raft_repository.NewRaftGroupEventRepository(groupEventRepo, raftNode, log)

	// Wrap the config repository with the Raft-aware repository
	configRepo := repository.NewConfigRepository(db)
	raftConfigRepo := raft_repository.NewRaftConfigRepository(configRepo, raftNode, log)

	// Initialize handlers
	userAPI := NewUserHandler(raftUserRepo, log)
	eventAPI := NewEventHandler(raftEventRepo, log)
	groupAPI := NewGroupHandler(raftGroupRepo, log)
	groupEventAPI := NewGroupEventHandler(raftGroupEventRepo, log)
	configHandler := NewConfigHandler(raftConfigRepo)

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
		configAPI:     configHandler,
		raftNode:      raftNode,
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

	// Config routes
	s.configAPI.RegisterRoutes(api)

	// Raft introspection routes
	if s.raftNode != nil {
		raftHandler := newRaftInfoHandler(s.raftNode, s.log)
		r.HandleFunc("/raft/status", raftHandler.Status).Methods("GET")
		r.HandleFunc("/raft/leader", raftHandler.Leader).Methods("GET")
	} else {
		s.log.Warn().Msg("RaftNode is not initialized, skipping Raft introspection routes")
	}
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

// --- Raft Info Handler ---

type raftInfoHandler struct {
	raft *consensus.RaftNode
	log  *zerolog.Logger
}

func newRaftInfoHandler(raft *consensus.RaftNode, log *zerolog.Logger) *raftInfoHandler {
	return &raftInfoHandler{raft: raft, log: log}
}

func (h *raftInfoHandler) Status(w http.ResponseWriter, r *http.Request) {
	status := h.raft.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		h.log.Error().Err(err).Msg("Failed to encode raft status")
		http.Error(w, "Failed to encode raft status", http.StatusInternalServerError)
	}
}

func (h *raftInfoHandler) Leader(w http.ResponseWriter, r *http.Request) {
	leaderID := h.raft.GetLeaderID()
	leaderAddress := h.raft.GetLeaderAddress()

	response := map[string]string{
		"leader_id":      leaderID,
		"leader_address": leaderAddress,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error().Err(err).Msg("Failed to encode raft leader information")
		http.Error(w, "Failed to encode raft leader information", http.StatusInternalServerError)
	}
}
