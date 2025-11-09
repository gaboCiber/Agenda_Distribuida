package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/agenda-distribuida/db-service/internal/models"
	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// UserHandler handles HTTP requests related to users
type UserHandler struct {
	repo repository.UserRepository
	log  *zerolog.Logger
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(repo repository.UserRepository, log *zerolog.Logger) *UserHandler {
	return &UserHandler{
		repo: repo,
		log:  log,
	}
}

// CreateUser handles the creation of a new user
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate request
	if err := validate.Struct(req); err != nil {
		h.log.Error().Err(err).Msg("Validation failed")
		http.Error(w, `{"status":"error","message":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	// Create hashed password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.log.Error().Err(err).Msg("Failed to hash password")
		http.Error(w, `{"status":"error","message":"Failed to process password"}`, http.StatusInternalServerError)
		return
	}

	// Create user
	user := &models.User{
		ID:             uuid.New(),
		Username:       req.Username,
		Email:          req.Email,
		HashedPassword: string(hashedPassword), // Guardamos el hash, no la contraseña en texto plano
		IsActive:       true,
	}

	if err := h.repo.Create(r.Context(), user); err != nil {
		h.log.Error().Err(err).Str("email", user.Email).Msg("Failed to create user")
		if errors.Is(err, repository.ErrEmailAlreadyExists) {
			http.Error(w, `{"status":"error","message":"Email already exists"}`, http.StatusConflict)
		} else {
			http.Error(w, `{"status":"error","message":"Failed to create user"}`, http.StatusInternalServerError)
		}
		return
	}

	// Return the created user
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"user":   user,
	})
}

// GetUser retrieves a user by ID
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid user ID format"}`, http.StatusBadRequest)
		return
	}

	user, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("user_id", id.String()).Msg("Failed to get user")
		if errors.Is(err, repository.ErrUserNotFound) {
			http.Error(w, `{"status":"error","message":"User not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"status":"error","message":"Failed to get user"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"user":   user,
	})
}

// UpdateUser updates a user's information
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid user ID format"}`, http.StatusBadRequest)
		return
	}

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, `{"status":"error","message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate request
	if err := validate.Struct(req); err != nil {
		h.log.Error().Err(err).Msg("Validation failed")
		http.Error(w, `{"status":"error","message":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	user, err := h.repo.Update(r.Context(), id, &req)
	if err != nil {
		h.log.Error().Err(err).Str("user_id", id.String()).Msg("Failed to update user")
		switch {
		case errors.Is(err, repository.ErrUserNotFound):
			http.Error(w, `{"status":"error","message":"User not found"}`, http.StatusNotFound)
		case errors.Is(err, repository.ErrEmailAlreadyExists):
			http.Error(w, `{"status":"error","message":"Email already in use"}`, http.StatusConflict)
		default:
			http.Error(w, `{"status":"error","message":"Failed to update user"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"user":   user,
	})
}

// DeleteUser deletes a user by ID
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, `{"status":"error","message":"Invalid user ID format"}`, http.StatusBadRequest)
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		h.log.Error().Err(err).Str("user_id", id.String()).Msg("Failed to delete user")
		if errors.Is(err, repository.ErrUserNotFound) {
			http.Error(w, `{"status":"error","message":"User not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"status":"error","message":"Failed to delete user"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// Login handles user authentication
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error().Err(err).Msg("Failed to decode login request")
		http.Error(w, `{"status":"error","message":"Invalid request"}`, http.StatusBadRequest)
		return
	}

	// Validar la solicitud
	if err := validate.Struct(req); err != nil {
		h.log.Error().Err(err).Msg("Login validation failed")
		http.Error(w, `{"status":"error","message":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	// Obtener el usuario por email
	user, err := h.repo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		h.log.Error().Err(err).Str("email", req.Email).Msg("User not found")
		http.Error(w, `{"status":"error","message":"Invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	// Verificar la contraseña
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.Password)); err != nil {
		h.log.Error().Err(err).Str("email", req.Email).Msg("Invalid password")
		http.Error(w, `{"status":"error","message":"Invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	// Si todo está bien, devolver el ID del usuario
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.LoginResponse{
		Status: "success",
		UserID: user.ID,
	})
}
