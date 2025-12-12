package raft_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agenda-distribuida/db-service/internal/consensus"
	"github.com/agenda-distribuida/db-service/internal/models"
	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// RaftGroupEventRepository is a wrapper for the group event repository that interacts with the Raft cluster.
type RaftGroupEventRepository struct {
	baseRepo repository.GroupEventRepository
	raftNode *consensus.RaftNode
	log      *zerolog.Logger
}

// NewRaftGroupEventRepository creates a new instance of RaftGroupEventRepository.
func NewRaftGroupEventRepository(baseRepo repository.GroupEventRepository, raftNode *consensus.RaftNode, log *zerolog.Logger) repository.GroupEventRepository {
	return &RaftGroupEventRepository{
		baseRepo: baseRepo,
		raftNode: raftNode,
		log:      log,
	}
}

// Transaction support - delegates to base repository since transactions are complex for Raft
func (r *RaftGroupEventRepository) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return r.baseRepo.BeginTx(ctx, opts)
}

// Group Event Management

// AddGroupEvent proposes a group event addition command to the Raft cluster.
func (r *RaftGroupEventRepository) AddGroupEvent(ctx context.Context, groupEvent *models.GroupEvent) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(groupEvent)
	if err != nil {
		return fmt.Errorf("error al serializar evento de grupo: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "AddGroupEvent",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// AddGroupEventWithTx proposes a group event addition command to the Raft cluster.
func (r *RaftGroupEventRepository) AddGroupEventWithTx(ctx context.Context, tx *sql.Tx, groupEvent *models.GroupEvent) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(groupEvent)
	if err != nil {
		return fmt.Errorf("error al serializar evento de grupo: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "AddGroupEventWithTx",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// RemoveGroupEvent proposes a group event removal command to the Raft cluster.
func (r *RaftGroupEventRepository) RemoveGroupEvent(ctx context.Context, groupID, eventID uuid.UUID) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	type removeGroupEventPayload struct {
		GroupID uuid.UUID `json:"group_id"`
		EventID uuid.UUID `json:"event_id"`
	}

	payload, err := json.Marshal(removeGroupEventPayload{
		GroupID: groupID,
		EventID: eventID,
	})
	if err != nil {
		return fmt.Errorf("error al serializar payload de eliminación de evento de grupo: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "RemoveGroupEvent",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// GetGroupEvents delegates the read operation to the base repository.
func (r *RaftGroupEventRepository) GetGroupEvents(ctx context.Context, groupID uuid.UUID) ([]*models.GroupEvent, error) {
	return r.baseRepo.GetGroupEvents(ctx, groupID)
}

// GetGroupEvent delegates the read operation to the base repository.
func (r *RaftGroupEventRepository) GetGroupEvent(ctx context.Context, eventID uuid.UUID) (*models.GroupEvent, error) {
	return r.baseRepo.GetGroupEvent(ctx, eventID)
}

// RemoveEventFromAllGroups proposes an event removal from all groups command to the Raft cluster.
func (r *RaftGroupEventRepository) RemoveEventFromAllGroups(ctx context.Context, eventID uuid.UUID) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(eventID)
	if err != nil {
		return fmt.Errorf("error al serializar ID de evento para eliminación de todos los grupos: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "RemoveEventFromAllGroups",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// UpdateGroupEvent proposes a group event update command to the Raft cluster.
func (r *RaftGroupEventRepository) UpdateGroupEvent(ctx context.Context, groupID, eventID uuid.UUID, status models.EventStatus, isHierarchical bool) (*models.GroupEvent, error) {
	if !r.raftNode.IsLeader() {
		return nil, ErrNotLeader
	}

	type updateGroupEventPayload struct {
		GroupID        uuid.UUID          `json:"group_id"`
		EventID        uuid.UUID          `json:"event_id"`
		Status         models.EventStatus `json:"status"`
		IsHierarchical bool               `json:"is_hierarchical"`
	}

	payload, err := json.Marshal(updateGroupEventPayload{
		GroupID:        groupID,
		EventID:        eventID,
		Status:         status,
		IsHierarchical: isHierarchical,
	})
	if err != nil {
		return nil, fmt.Errorf("error al serializar payload de actualización de evento de grupo: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "UpdateGroupEvent",
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

	// After successful application, fetch the updated group event.
	return r.baseRepo.GetGroupEvent(ctx, eventID)
}

// Event Status Management

// AddEventStatus proposes an event status addition command to the Raft cluster.
func (r *RaftGroupEventRepository) AddEventStatus(ctx context.Context, status *models.GroupEventStatus) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("error al serializar estado de evento: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "AddEventStatus",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// AddEventStatusWithTx proposes an event status addition command to the Raft cluster.
func (r *RaftGroupEventRepository) AddEventStatusWithTx(ctx context.Context, tx *sql.Tx, status *models.GroupEventStatus) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("error al serializar estado de evento: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "AddEventStatusWithTx",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// BatchCreateEventStatus proposes a batch event status creation command to the Raft cluster.
func (r *RaftGroupEventRepository) BatchCreateEventStatus(ctx context.Context, tx *sql.Tx, statuses []*models.GroupEventStatus) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	type batchCreatePayload struct {
		Statuses []*models.GroupEventStatus `json:"statuses"`
	}

	payload, err := json.Marshal(batchCreatePayload{
		Statuses: statuses,
	})
	if err != nil {
		return fmt.Errorf("error al serializar payload de creación batch de estados de evento: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "BatchCreateEventStatus",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// UpdateEventStatus proposes an event status update command to the Raft cluster.
func (r *RaftGroupEventRepository) UpdateEventStatus(ctx context.Context, eventID, userID uuid.UUID, status models.EventStatus, updatedAt time.Time) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	type updateEventStatusPayload struct {
		EventID  uuid.UUID          `json:"event_id"`
		UserID   uuid.UUID          `json:"user_id"`
		Status   models.EventStatus `json:"status"`
		UpdatedAt time.Time         `json:"updated_at"`
	}

	payload, err := json.Marshal(updateEventStatusPayload{
		EventID:  eventID,
		UserID:   userID,
		Status:   status,
		UpdatedAt: updatedAt,
	})
	if err != nil {
		return fmt.Errorf("error al serializar payload de actualización de estado de evento: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "UpdateEventStatus",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// UpdateEventStatuses proposes a batch event status update command to the Raft cluster.
func (r *RaftGroupEventRepository) UpdateEventStatuses(ctx context.Context, tx *sql.Tx, statuses []*models.GroupEventStatus) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	type updateEventStatusesPayload struct {
		Statuses []*models.GroupEventStatus `json:"statuses"`
	}

	payload, err := json.Marshal(updateEventStatusesPayload{
		Statuses: statuses,
	})
	if err != nil {
		return fmt.Errorf("error al serializar payload de actualización batch de estados de evento: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "UpdateEventStatuses",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// GetEventStatus delegates the read operation to the base repository.
func (r *RaftGroupEventRepository) GetEventStatus(ctx context.Context, eventID, userID uuid.UUID) (*models.GroupEventStatus, error) {
	return r.baseRepo.GetEventStatus(ctx, eventID, userID)
}

// GetEventStatuses delegates the read operation to the base repository.
func (r *RaftGroupEventRepository) GetEventStatuses(ctx context.Context, eventID uuid.UUID) ([]*models.GroupEventStatus, error) {
	return r.baseRepo.GetEventStatuses(ctx, eventID)
}

// GetEventStatusesByGroup delegates the read operation to the base repository.
func (r *RaftGroupEventRepository) GetEventStatusesByGroup(ctx context.Context, groupID, eventID uuid.UUID) ([]*models.GroupEventStatus, error) {
	return r.baseRepo.GetEventStatusesByGroup(ctx, groupID, eventID)
}

// GetEventStatusCounts delegates the read operation to the base repository.
func (r *RaftGroupEventRepository) GetEventStatusCounts(ctx context.Context, eventID uuid.UUID) (map[models.EventStatus]int, error) {
	return r.baseRepo.GetEventStatusCounts(ctx, eventID)
}

// HasResponded delegates the read operation to the base repository.
func (r *RaftGroupEventRepository) HasResponded(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	return r.baseRepo.HasResponded(ctx, eventID, userID)
}

// HasAllMembersAccepted delegates the read operation to the base repository.
func (r *RaftGroupEventRepository) HasAllMembersAccepted(ctx context.Context, groupID, eventID uuid.UUID) (bool, error) {
	return r.baseRepo.HasAllMembersAccepted(ctx, groupID, eventID)
}

// DeleteEventStatus proposes an event status deletion command to the Raft cluster.
func (r *RaftGroupEventRepository) DeleteEventStatus(ctx context.Context, tx *sql.Tx, eventID, userID uuid.UUID) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	type deleteEventStatusPayload struct {
		EventID uuid.UUID `json:"event_id"`
		UserID  uuid.UUID `json:"user_id"`
	}

	payload, err := json.Marshal(deleteEventStatusPayload{
		EventID: eventID,
		UserID:  userID,
	})
	if err != nil {
		return fmt.Errorf("error al serializar payload de eliminación de estado de evento: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "DeleteEventStatus",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// DeleteEventStatuses proposes an event statuses deletion command to the Raft cluster.
func (r *RaftGroupEventRepository) DeleteEventStatuses(ctx context.Context, tx *sql.Tx, eventID uuid.UUID) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	type deleteEventStatusesPayload struct {
		EventID uuid.UUID `json:"event_id"`
	}

	payload, err := json.Marshal(deleteEventStatusesPayload{
		EventID: eventID,
	})
	if err != nil {
		return fmt.Errorf("error al serializar payload de eliminación de estados de evento: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "DeleteEventStatuses",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// DeleteEventStatusesByGroup proposes an event statuses deletion by group command to the Raft cluster.
func (r *RaftGroupEventRepository) DeleteEventStatusesByGroup(ctx context.Context, tx *sql.Tx, groupID, eventID uuid.UUID) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	type deleteEventStatusesByGroupPayload struct {
		GroupID uuid.UUID `json:"group_id"`
		EventID uuid.UUID `json:"event_id"`
	}

	payload, err := json.Marshal(deleteEventStatusesByGroupPayload{
		GroupID: groupID,
		EventID: eventID,
	})
	if err != nil {
		return fmt.Errorf("error al serializar payload de eliminación de estados de evento por grupo: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "DeleteEventStatusesByGroup",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// Invitation Management

// CreateInvitation proposes an invitation creation command to the Raft cluster.
func (r *RaftGroupEventRepository) CreateInvitation(ctx context.Context, invitation *models.GroupInvitation) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(invitation)
	if err != nil {
		return fmt.Errorf("error al serializar invitación: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "CreateInvitation",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// GetInvitationByID delegates the read operation to the base repository.
func (r *RaftGroupEventRepository) GetInvitationByID(ctx context.Context, id uuid.UUID) (*models.GroupInvitation, error) {
	return r.baseRepo.GetInvitationByID(ctx, id)
}

// UpdateInvitation proposes an invitation update command to the Raft cluster.
func (r *RaftGroupEventRepository) UpdateInvitation(ctx context.Context, id uuid.UUID, status string) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	type updateInvitationPayload struct {
		ID     uuid.UUID `json:"id"`
		Status string    `json:"status"`
	}

	payload, err := json.Marshal(updateInvitationPayload{
		ID:     id,
		Status: status,
	})
	if err != nil {
		return fmt.Errorf("error al serializar payload de actualización de invitación: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "UpdateInvitation",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// GetUserInvitations delegates the read operation to the base repository.
func (r *RaftGroupEventRepository) GetUserInvitations(ctx context.Context, userID uuid.UUID, status string) ([]*models.GroupInvitation, error) {
	return r.baseRepo.GetUserInvitations(ctx, userID, status)
}

// DeleteUserInvitations proposes a user invitations deletion command to the Raft cluster.
func (r *RaftGroupEventRepository) DeleteUserInvitations(ctx context.Context, userID uuid.UUID) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(userID)
	if err != nil {
		return fmt.Errorf("error al serializar ID de usuario para eliminación de invitaciones: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "DeleteUserInvitations",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// DeleteUserInvitation proposes a user invitation deletion command to the Raft cluster.
func (r *RaftGroupEventRepository) DeleteUserInvitation(ctx context.Context, invitationID uuid.UUID) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(invitationID)
	if err != nil {
		return fmt.Errorf("error al serializar ID de invitación para eliminación: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupEventRepository",
		Method:     "DeleteUserInvitation",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// GetUserByEmail delegates the read operation to the base repository.
func (r *RaftGroupEventRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.baseRepo.GetUserByEmail(ctx, email)
}
