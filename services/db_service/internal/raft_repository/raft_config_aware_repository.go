package raft_repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agenda-distribuida/db-service/internal/consensus"
	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/rs/zerolog"
)

// RaftConfigRepository is a wrapper for the config repository that interacts with the Raft cluster.
type RaftConfigRepository struct {
	baseRepo *repository.ConfigRepository
	raftNode *consensus.RaftNode
	log      *zerolog.Logger
}

// Config interface defines the methods that RaftConfigRepository implements
type Config interface {
	Create(ctx context.Context, config repository.Config) error
	GetByName(ctx context.Context, name string) (repository.Config, error)
	List(ctx context.Context) ([]repository.Config, error)
	Update(ctx context.Context, config repository.Config) error
	Delete(ctx context.Context, name string) error
}

// NewRaftConfigRepository creates a new instance of RaftConfigRepository.
func NewRaftConfigRepository(baseRepo *repository.ConfigRepository, raftNode *consensus.RaftNode, log *zerolog.Logger) Config {
	return &RaftConfigRepository{
		baseRepo: baseRepo,
		raftNode: raftNode,
		log:      log,
	}
}

// Create proposes a config creation command to the Raft cluster.
func (r *RaftConfigRepository) Create(ctx context.Context, config repository.Config) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("error al serializar configuración: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "ConfigRepository",
		Method:     "Create",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// GetByName delegates the read operation to the base repository.
func (r *RaftConfigRepository) GetByName(ctx context.Context, name string) (repository.Config, error) {
	return r.baseRepo.GetByName(ctx, name)
}

// List delegates the read operation to the base repository.
func (r *RaftConfigRepository) List(ctx context.Context) ([]repository.Config, error) {
	return r.baseRepo.List(ctx)
}

// Update proposes a config update command to the Raft cluster.
func (r *RaftConfigRepository) Update(ctx context.Context, config repository.Config) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("error al serializar configuración para actualización: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "ConfigRepository",
		Method:     "Update",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// Delete proposes a config deletion command to the Raft cluster.
func (r *RaftConfigRepository) Delete(ctx context.Context, name string) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(name)
	if err != nil {
		return fmt.Errorf("error al serializar nombre de configuración: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "ConfigRepository",
		Method:     "Delete",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}
