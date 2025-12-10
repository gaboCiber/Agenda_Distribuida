package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/agenda-distribuida/db-service/internal/consensus"
	"github.com/agenda-distribuida/db-service/internal/models"
	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

var ErrNotLeader = errors.New("not the leader, please redirect")

// RaftAwareUserRepository es un wrapper para los reposoties que interactúa con el clúster Raft.
type RaftAwareUserRepository struct {
	baseRepo  repository.UserRepository
	raftNode  *consensus.RaftNode
	log       *zerolog.Logger
	leaderURL string // URL del líder actual, para redirecciones.
}

// NewRaftAwareUserRepository crea una nueva instancia de RaftAwareUserRepository.
func NewRaftAwareUserRepository(baseRepo repository.UserRepository, raftNode *consensus.RaftNode, log *zerolog.Logger) repository.UserRepository {
	return &RaftAwareUserRepository{
		baseRepo:  baseRepo,
		raftNode:  raftNode,
		log:       log,
		leaderURL: "", // Se actualizará dinámicamente.
	}
}

// Create proposes a user creation command to the Raft cluster.
func (r *RaftAwareUserRepository) Create(ctx context.Context, user *models.User) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(user)
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
// It waits for the command to be applied and then fetches the updated user.
func (r *RaftAwareUserRepository) Update(ctx context.Context, id uuid.UUID, updateReq *models.UpdateUserRequest) (*models.User, error) {
	if !r.raftNode.IsLeader() {
		return nil, ErrNotLeader
	}

	// We need to pass both the ID and the update request in the payload.
	type updatePayload struct {
		ID        uuid.UUID                 `json:"id"`
		UpdateReq *models.UpdateUserRequest `json:"update_req"`
	}

	payload, err := json.Marshal(updatePayload{ID: id, UpdateReq: updateReq})
	if err != nil {
		return nil, fmt.Errorf("error al serializar payload de actualización: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "UserRepository",
		Method:     "Update",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return nil, err
	}

	// Wait for the command to be applied.
	if applyErr := <-applyCh; applyErr != nil {
		return nil, applyErr
	}

	// After successful application, fetch the updated user.
	return r.baseRepo.GetByID(ctx, id)
}

// Delete proposes a user deletion command to the Raft cluster.
func (r *RaftAwareUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
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
func (r *RaftAwareUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return r.baseRepo.GetByID(ctx, id)
}

// GetByEmail delegates the read operation to the base repository.
func (r *RaftAwareUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.baseRepo.GetByEmail(ctx, email)
}

// List delegates the read operation to the base repository.
func (r *RaftAwareUserRepository) List(ctx context.Context, offset, limit int) ([]*models.User, error) {
	return r.baseRepo.List(ctx, offset, limit)
}

// IsLeader checks if the current node is the Raft leader.
func (r *RaftAwareUserRepository) IsLeader() bool {
	return r.raftNode.IsLeader()
}

// GetLeaderID returns the ID of the current leader.
func (r *RaftAwareUserRepository) GetLeaderID() string {
	return r.raftNode.GetLeaderID()
}

// GetLeaderAddress returns the network address of the current leader.
func (r *RaftAwareUserRepository) GetLeaderAddress() string {
	return r.raftNode.GetLeaderAddress()
}
