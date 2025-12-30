package raft_repository

import (
	"context"
	"encoding/json"

	"github.com/agenda-distribuida/db-service/internal/consensus"
	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/rs/zerolog"
)

type RaftServiceRegistryRepository struct {
	repo     repository.ServiceRegistryRepository
	raftNode *consensus.RaftNode
	logger   *zerolog.Logger
}

func NewRaftServiceRegistryRepository(
	repo repository.ServiceRegistryRepository,
	raftNode *consensus.RaftNode,
	logger *zerolog.Logger,
) *RaftServiceRegistryRepository {
	return &RaftServiceRegistryRepository{
		repo:     repo,
		raftNode: raftNode,
		logger:   logger,
	}
}

func (r *RaftServiceRegistryRepository) Register(ctx context.Context, serviceName, address string) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(map[string]string{
		"service_name": serviceName,
		"address":      address,
	})
	if err != nil {
		return err
	}

	cmd := consensus.DBCommand{
		Repository: "ServiceRegistryRepository",
		Method:     "Register",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

func (r *RaftServiceRegistryRepository) Deregister(ctx context.Context, serviceName string) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(serviceName)
	if err != nil {
		return err
	}

	cmd := consensus.DBCommand{
		Repository: "ServiceRegistryRepository",
		Method:     "Deregister",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

func (r *RaftServiceRegistryRepository) GetService(ctx context.Context, serviceName string) (string, error) {
	// Read operations can go to any node
	return r.repo.GetService(ctx, serviceName)
}

func (r *RaftServiceRegistryRepository) ListServices(ctx context.Context) (map[string]string, error) {
	// Read operations can go to any node
	return r.repo.ListServices(ctx)
}

func (r *RaftServiceRegistryRepository) UpdateLastSeen(ctx context.Context, serviceName string) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(serviceName)
	if err != nil {
		return err
	}

	cmd := consensus.DBCommand{
		Repository: "ServiceRegistryRepository",
		Method:     "UpdateLastSeen",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}
