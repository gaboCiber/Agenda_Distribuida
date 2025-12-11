package raft_repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agenda-distribuida/db-service/internal/consensus"
	"github.com/agenda-distribuida/db-service/internal/models"
	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// RaftEventRepository is a wrapper for the event repository that interacts with the Raft cluster.
type RaftEventRepository struct {
	baseRepo repository.EventRepository
	raftNode *consensus.RaftNode
	log      *zerolog.Logger
}

// NewRaftEventRepository creates a new instance of RaftEventRepository.
func NewRaftEventRepository(baseRepo repository.EventRepository, raftNode *consensus.RaftNode, log *zerolog.Logger) repository.EventRepository {
	return &RaftEventRepository{
		baseRepo: baseRepo,
		raftNode: raftNode,
		log:      log,
	}
}

// Create proposes an event creation command to the Raft cluster.
func (r *RaftEventRepository) Create(ctx context.Context, event *models.Event) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	// Create payload with leader-generated ID
	type createPayload struct {
		ID          uuid.UUID `json:"id"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		StartTime   time.Time `json:"start_time"`
		EndTime     time.Time `json:"end_time"`
		UserID      uuid.UUID `json:"user_id"`
	}

	payload, err := json.Marshal(createPayload{
		ID:          event.ID,
		Title:       event.Title,
		Description: event.Description,
		StartTime:   event.StartTime,
		EndTime:     event.EndTime,
		UserID:      event.UserID,
	})
	if err != nil {
		return fmt.Errorf("error al serializar evento: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "EventRepository",
		Method:     "Create",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// GetByID delegates the read operation to the base repository.
func (r *RaftEventRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Event, error) {
	return r.baseRepo.GetByID(ctx, id)
}

// Update proposes an event update command to the Raft cluster.
// It waits for the command to be applied and then fetches the updated event.
func (r *RaftEventRepository) Update(ctx context.Context, id uuid.UUID, updateReq *models.EventRequest) (*models.Event, error) {
	if !r.raftNode.IsLeader() {
		return nil, ErrNotLeader
	}

	// We need to pass both the ID and the update request in the payload.
	type updatePayload struct {
		ID        uuid.UUID            `json:"id"`
		UpdateReq *models.EventRequest `json:"update_req"`
	}

	payload, err := json.Marshal(updatePayload{ID: id, UpdateReq: updateReq})
	if err != nil {
		return nil, fmt.Errorf("error al serializar payload de actualización: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "EventRepository",
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

	// After successful application, fetch the updated event.
	return r.baseRepo.GetByID(ctx, id)
}

// Delete proposes an event deletion command to the Raft cluster.
func (r *RaftEventRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(id)
	if err != nil {
		return fmt.Errorf("error al serializar ID de evento: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "EventRepository",
		Method:     "Delete",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// ListByUser delegates the read operation to the base repository.
func (r *RaftEventRepository) ListByUser(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.Event, error) {
	return r.baseRepo.ListByUser(ctx, userID, offset, limit)
}

// CheckTimeConflict delegates the read operation to the base repository.
func (r *RaftEventRepository) CheckTimeConflict(ctx context.Context, userID uuid.UUID, startTime, endTime time.Time, excludeEventID *uuid.UUID) (bool, error) {
	return r.baseRepo.CheckTimeConflict(ctx, userID, startTime, endTime, excludeEventID)
}
