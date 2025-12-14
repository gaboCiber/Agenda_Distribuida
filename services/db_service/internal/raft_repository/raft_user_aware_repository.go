package raft_repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agenda-distribuida/db-service/internal/consensus"
	"github.com/agenda-distribuida/db-service/internal/models"
	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

var ErrNotLeader = errors.New("not the leader, please redirect")

// RaftUserRepository is a wrapper for the user repository that interacts with the Raft cluster.
type RaftUserRepository struct {
	baseRepo  repository.UserRepository
	raftNode  *consensus.RaftNode
	log       *zerolog.Logger
	leaderURL string // URL del líder actual, para redirecciones.
}

// NewRaftUserRepository creates a new instance of RaftUserRepository.
func NewRaftUserRepository(baseRepo repository.UserRepository, raftNode *consensus.RaftNode, log *zerolog.Logger) repository.UserRepository {
	return &RaftUserRepository{
		baseRepo:  baseRepo,
		raftNode:  raftNode,
		log:       log,
		leaderURL: "", // Se actualizará dinámicamente.
	}
}

// Create proposes a user creation command to the Raft cluster.
func (r *RaftUserRepository) Create(ctx context.Context, user *models.User) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	// Explicitly include hashed password since json:"-" on models.User omits it
	type createPayload struct {
		ID             uuid.UUID `json:"id"`
		Username       string    `json:"username"`
		Email          string    `json:"email"`
		HashedPassword string    `json:"hashed_password"`
		IsActive       bool      `json:"is_active"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
	}

	payload, err := json.Marshal(createPayload{
		ID:             user.ID,
		Username:       user.Username,
		Email:          user.Email,
		HashedPassword: user.HashedPassword,
		IsActive:       user.IsActive,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("error al serializar usuario: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "UserRepository",
		Method:     "Create",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	// Wait for the command to be applied.
	return <-applyCh
}

// Update proposes a user update command to the Raft cluster.
func (r *RaftUserRepository) Update(ctx context.Context, id uuid.UUID, user *models.User) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	// We need to pass both the ID and the user object in the payload.
	type updatePayload struct {
		ID   uuid.UUID    `json:"id"`
		User *models.User `json:"user"`
	}

	payload, err := json.Marshal(updatePayload{ID: id, User: user})
	if err != nil {
		return fmt.Errorf("error al serializar payload de actualización: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "UserRepository",
		Method:     "Update",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	// Wait for the command to be applied.
	return <-applyCh
}

// Delete proposes a user deletion command to the Raft cluster.
func (r *RaftUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(id)
	if err != nil {
		return fmt.Errorf("error al serializar ID de usuario: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "UserRepository",
		Method:     "Delete",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// GetByID delegates the read operation to the base repository.
func (r *RaftUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return r.baseRepo.GetByID(ctx, id)
}

// GetByEmail delegates the read operation to the base repository.
func (r *RaftUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.baseRepo.GetByEmail(ctx, email)
}

// List delegates the read operation to the base repository.
func (r *RaftUserRepository) List(ctx context.Context, offset, limit int) ([]*models.User, error) {
	return r.baseRepo.List(ctx, offset, limit)
}
