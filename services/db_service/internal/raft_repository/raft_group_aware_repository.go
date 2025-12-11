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

// RaftGroupRepository is a wrapper for the group repository that interacts with the Raft cluster.
type RaftGroupRepository struct {
	baseRepo repository.GroupRepository
	raftNode *consensus.RaftNode
	log      *zerolog.Logger
}

// NewRaftGroupRepository creates a new instance of RaftGroupRepository.
func NewRaftGroupRepository(baseRepo repository.GroupRepository, raftNode *consensus.RaftNode, log *zerolog.Logger) repository.GroupRepository {
	return &RaftGroupRepository{
		baseRepo: baseRepo,
		raftNode: raftNode,
		log:      log,
	}
}

// Create proposes a group creation command to the Raft cluster.
func (r *RaftGroupRepository) Create(ctx context.Context, group *models.Group) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	// Create payload with leader-generated ID
	type createPayload struct {
		ID             uuid.UUID  `json:"id"`
		Name           string     `json:"name"`
		Description    string     `json:"description"`
		CreatedBy      uuid.UUID  `json:"created_by"`
		IsHierarchical bool       `json:"is_hierarchical"`
		ParentGroupID  *uuid.UUID `json:"parent_group_id"`
	}

	payload, err := json.Marshal(createPayload{
		ID:             group.ID,
		Name:           group.Name,
		Description:    *group.Description,
		CreatedBy:      group.CreatedBy,
		IsHierarchical: group.IsHierarchical,
		ParentGroupID:  group.ParentGroupID,
	})
	if err != nil {
		return fmt.Errorf("error al serializar grupo: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupRepository",
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
func (r *RaftGroupRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Group, error) {
	return r.baseRepo.GetByID(ctx, id)
}

// Update proposes a group update command to the Raft cluster.
func (r *RaftGroupRepository) Update(ctx context.Context, group *models.Group) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("error al serializar grupo para actualización: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupRepository",
		Method:     "Update",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// Delete proposes a group deletion command to the Raft cluster.
func (r *RaftGroupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	payload, err := json.Marshal(id)
	if err != nil {
		return fmt.Errorf("error al serializar ID de grupo: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupRepository",
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
func (r *RaftGroupRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.GroupExtended, error) {
	return r.baseRepo.ListByUser(ctx, userID)
}

// AddMember proposes a member addition command to the Raft cluster.
func (r *RaftGroupRepository) AddMember(ctx context.Context, member *models.GroupMember) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	// Create payload with leader-generated ID
	type addMemberPayload struct {
		ID          uuid.UUID `json:"id"`
		GroupID     uuid.UUID `json:"group_id"`
		UserID      uuid.UUID `json:"user_id"`
		Role        string    `json:"role"`
		IsInherited bool      `json:"is_inherited"`
		JoinedAt    time.Time `json:"joined_at"`
	}

	payload, err := json.Marshal(addMemberPayload{
		ID:          member.ID,
		GroupID:     member.GroupID,
		UserID:      member.UserID,
		Role:        member.Role,
		IsInherited: member.IsInherited,
		JoinedAt:    member.JoinedAt,
	})
	if err != nil {
		return fmt.Errorf("error al serializar miembro de grupo: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupRepository",
		Method:     "AddMember",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// GetMembers delegates the read operation to the base repository.
func (r *RaftGroupRepository) GetMembers(ctx context.Context, groupID uuid.UUID) ([]*models.GroupMember, error) {
	return r.baseRepo.GetMembers(ctx, groupID)
}

// AddMemberBasic adds a member to a group using the basic method (no hierarchical logic)
func (r *RaftGroupRepository) AddMemberBasic(ctx context.Context, member *models.GroupMember) error {
	// For Raft, we still need to propose this command to maintain consistency
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	// Use the same payload structure as AddMember for consistency
	type addMemberPayload struct {
		ID          uuid.UUID `json:"id"`
		GroupID     uuid.UUID `json:"group_id"`
		UserID      uuid.UUID `json:"user_id"`
		Role        string    `json:"role"`
		IsInherited bool      `json:"is_inherited"`
		JoinedAt    time.Time `json:"joined_at"`
	}

	payload, err := json.Marshal(addMemberPayload{
		ID:          member.ID,
		GroupID:     member.GroupID,
		UserID:      member.UserID,
		Role:        member.Role,
		IsInherited: member.IsInherited,
		JoinedAt:    member.JoinedAt,
	})
	if err != nil {
		return fmt.Errorf("error al serializar miembro de grupo: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupRepository",
		Method:     "AddMemberBasic",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// UpdateGroupMember proposes a member update command to the Raft cluster.
func (r *RaftGroupRepository) UpdateGroupMember(ctx context.Context, groupID, userID uuid.UUID, role string) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	// We need to pass both groupID, userID, and role in the payload.
	type updateMemberPayload struct {
		GroupID uuid.UUID `json:"group_id"`
		UserID  uuid.UUID `json:"user_id"`
		Role    string    `json:"role"`
	}

	payload, err := json.Marshal(updateMemberPayload{
		GroupID: groupID,
		UserID:  userID,
		Role:    role,
	})
	if err != nil {
		return fmt.Errorf("error al serializar payload de actualización de miembro: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupRepository",
		Method:     "UpdateGroupMember",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// RemoveMember proposes a member removal command to the Raft cluster.
func (r *RaftGroupRepository) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	if !r.raftNode.IsLeader() {
		return ErrNotLeader
	}

	// We need to pass both groupID and userID in the payload.
	type removeMemberPayload struct {
		GroupID uuid.UUID `json:"group_id"`
		UserID  uuid.UUID `json:"user_id"`
	}

	payload, err := json.Marshal(removeMemberPayload{
		GroupID: groupID,
		UserID:  userID,
	})
	if err != nil {
		return fmt.Errorf("error al serializar payload de eliminación de miembro: %w", err)
	}

	cmd := consensus.DBCommand{
		Repository: "GroupRepository",
		Method:     "RemoveMember",
		Payload:    payload,
	}

	applyCh, err := r.raftNode.Propose(cmd)
	if err != nil {
		return err
	}

	return <-applyCh
}

// IsMember delegates the read operation to the base repository.
func (r *RaftGroupRepository) IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	return r.baseRepo.IsMember(ctx, groupID, userID)
}

// IsAdmin delegates the read operation to the base repository.
func (r *RaftGroupRepository) IsAdmin(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	return r.baseRepo.IsAdmin(ctx, groupID, userID)
}

// GetGroupMember delegates the read operation to the base repository.
func (r *RaftGroupRepository) GetGroupMember(ctx context.Context, groupID, userID uuid.UUID) (*models.GroupMember, error) {
	return r.baseRepo.GetGroupMember(ctx, groupID, userID)
}

// GetChildGroups delegates to read operation to the base repository.
func (r *RaftGroupRepository) GetChildGroups(ctx context.Context, parentGroupID uuid.UUID) ([]uuid.UUID, error) {
	return r.baseRepo.GetChildGroups(ctx, parentGroupID)
}
