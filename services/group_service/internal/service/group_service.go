package service

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/google/uuid"
)

// GroupService defines the interface for group-related operations
type GroupService interface {
	// Group operations
	CreateGroup(group *models.Group) (*models.Group, error)
	GetGroup(groupID string) (*models.Group, error)
	UpdateGroup(group *models.Group) error
	DeleteGroup(groupID, userID string) error
	ListUserGroups(userID string) ([]*models.Group, error)

	// Hierarchy operations
	GetGroupHierarchy(groupID string) ([]*models.Group, error)
	UpdateGroupHierarchy(groupID string, parentGroupID *string, isHierarchical bool) error
	GetInheritedMembers(groupID string) ([]*models.GroupMember, error)
	GetSubGroups(parentGroupID string) ([]*models.Group, error)

	// Member operations
	AddGroupMember(member *models.GroupMember) error
	RemoveGroupMember(groupID, userID string) error
	GetGroupMember(groupID, userID string) (*models.GroupMember, error)
	IsGroupMember(groupID, userID string) (bool, error)
	IsGroupAdmin(groupID, userID string) (bool, error)
	GetGroupMembers(groupID string) ([]*models.GroupMember, error)
	GetGroupAdmins(groupID string) ([]*models.GroupMember, error)
	RemoveUserFromAllGroups(userID string) error

	// Invitation operations
	CreateInvitation(invitation *models.GroupInvitation) error
	GetInvitation(invitationID string) (*models.GroupInvitation, error)
	UpdateInvitation(invitation *models.GroupInvitation) error
	HasPendingInvitation(groupID, userID string) (bool, error)
	GetUserInvitations(userID string, status string) ([]*models.GroupInvitation, error)

	// Event operations
	AddGroupEvent(groupEvent *models.GroupEvent) error
	RemoveEventFromGroup(groupID, eventID string) error
	GetGroupEvents(groupID string) ([]*models.GroupEvent, error)
	RemoveEventFromAllGroups(eventID string) error
	GetGroupEvent(eventID string) (*models.GroupEvent, error)

	// Event Status operations
	UpdateEventStatus(status *models.GroupEventStatus) error
	GetEventStatus(eventID, userID string) (*models.GroupEventStatus, error)
	GetEventStatuses(eventID string) ([]*models.GroupEventStatus, error)
	HasAllMembersAccepted(groupID, eventID string) (bool, error)
	UpdateGroupEventStatus(groupID, eventID, status string) error

	// Event handlers
	HandleUserDeleted(userID string) error
	HandleEventDeleted(eventID string) error

	// Transaction management
	BeginTx() (*sql.Tx, error)
	CommitTx(tx *sql.Tx) error
	RollbackTx(tx *sql.Tx) error
}

// NewGroupService creates a new instance of GroupService
func NewGroupService(db *models.Database, eventStatusRepo models.EventStatusRepository) GroupService {
	return &groupService{
		db:                 db,
		eventStatusRepo:    eventStatusRepo,
		eventStatusTxCache: make(map[*sql.Tx]models.EventStatusRepository),
	}
}

// groupService implements the GroupService interface
type groupService struct {
	db                 *models.Database
	eventStatusRepo    models.EventStatusRepository
	eventStatusTxCache map[*sql.Tx]models.EventStatusRepository
}

// CreateGroup creates a new group and adds the creator as an admin
func (s *groupService) CreateGroup(group *models.Group) (*models.Group, error) {
	// Set default values if not provided
	if group.ID == "" {
		group.ID = uuid.New().String()
	}
	if group.CreatedAt.IsZero() {
		group.CreatedAt = time.Now().UTC()
	}
	group.UpdatedAt = group.CreatedAt

	// Create the group in the database
	err := s.db.CreateGroup(group)
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %v", err)
	}

	// Return the created group
	return s.db.GetGroupByID(group.ID)
}

// GetGroupHierarchy retrieves the complete hierarchy for a group
func (s *groupService) GetGroupHierarchy(groupID string) ([]*models.Group, error) {
	// Get the group
	group, err := s.db.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, errors.New("group not found")
	}

	var hierarchy []*models.Group
	hierarchy = append(hierarchy, group)

	// If the group has a parent, get its hierarchy
	if group.ParentGroupID != nil {
		parentHierarchy, err := s.GetGroupHierarchy(*group.ParentGroupID)
		if err != nil {
			return nil, err
		}
		hierarchy = append(hierarchy, parentHierarchy...)
	}
	return hierarchy, nil
}

// UpdateGroupHierarchy updates a group's hierarchical relationships
func (s *groupService) UpdateGroupHierarchy(groupID string, parentGroupID *string, isHierarchical bool) error {
	// Get the current group
	group, err := s.db.GetGroupByID(groupID)
	if err != nil {
		return err
	}
	if group == nil {
		return errors.New("group not found")
	}

	// If setting a new parent, verify it's not creating a circular reference
	if parentGroupID != nil && (group.ParentGroupID == nil || *parentGroupID != *group.ParentGroupID) {
		isCircular, err := s.checkCircularReference(*parentGroupID, groupID)
		if err != nil {
			return err
		}
		if isCircular {
			return errors.New("cannot create circular reference in group hierarchy")
		}

		// Verify the parent group exists and is hierarchical
		parentGroup, err := s.db.GetGroupByID(*parentGroupID)
		if err != nil {
			return err
		}
		if parentGroup == nil {
			return errors.New("parent group not found")
		}
		if !parentGroup.IsHierarchical {
			return errors.New("parent group must be hierarchical")
		}
	}

	// Update the group's hierarchical properties
	group.IsHierarchical = isHierarchical
	group.ParentGroupID = parentGroupID

	// Save the updated group
	err = s.db.UpdateGroup(group)
	if err != nil {
		return err
	}

	// If this group is no longer hierarchical, remove all parent references from its children
	if !isHierarchical {
		err = s.db.RemoveParentFromChildren(groupID)
		if err != nil {
			return err
		}
	}

	return nil
}

// checkCircularReference checks if adding a parent would create a circular reference
func (s *groupService) checkCircularReference(parentID, childID string) (bool, error) {
	if parentID == "" {
		return false, nil
	}

	// If we've come full circle
	if parentID == childID {
		return true, nil
	}

	// Get the parent group
	parent, err := s.db.GetGroupByID(parentID)
	if err != nil {
		return false, err
	}
	if parent == nil || parent.ParentGroupID == nil {
		return false, nil
	}

	// Recursively check the parent's parent
	return s.checkCircularReference(*parent.ParentGroupID, childID)
}

// GetInheritedMembers retrieves all inherited members for a group
func (s *groupService) GetInheritedMembers(groupID string) ([]*models.GroupMember, error) {
	// Get the group
	group, err := s.db.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, errors.New("group not found")
	}

	// If the group doesn't have a parent, it can't have inherited members
	if group.ParentGroupID == nil {
		return []*models.GroupMember{}, nil
	}

	// Get all members from the parent group
	parentMembers, err := s.db.GetGroupMembers(*group.ParentGroupID)
	if err != nil {
		return nil, err
	}

	// Filter out non-inherited members
	var inheritedMembers []*models.GroupMember
	for _, member := range parentMembers {
		if member.IsInherited {
			inheritedMembers = append(inheritedMembers, member)
		}
	}

	return inheritedMembers, nil
}

// GetGroup retrieves a group by ID
func (s *groupService) GetGroup(groupID string) (*models.Group, error) {
	return s.db.GetGroupByID(groupID)
}

// ListUserGroups returns all groups that a user is a member of
func (s *groupService) ListUserGroups(userID string) ([]*models.Group, error) {
	return s.db.ListUserGroups(userID)
}

// UpdateGroup updates an existing group
func (s *groupService) UpdateGroup(group *models.Group) error {
	// Set updated at time
	group.UpdatedAt = time.Now().UTC()

	return s.db.UpdateGroup(group)
}

// AddGroupMember adds a user to a group
func (s *groupService) AddGroupMember(member *models.GroupMember) error {
	// Set joined at time if not set
	if member.JoinedAt.IsZero() {
		member.JoinedAt = time.Now().UTC()
	}

	return s.db.AddGroupMember(member)
}

// RemoveGroupMember removes a user from a group
func (s *groupService) RemoveGroupMember(groupID, userID string) error {
	return s.db.RemoveGroupMember(groupID, userID)
}

// GetGroupMember retrieves a specific group member by group ID and user ID
func (s *groupService) GetGroupMember(groupID, userID string) (*models.GroupMember, error) {
	return s.db.GetGroupMember(groupID, userID)
}

// IsGroupMember checks if a user is a member of a group
func (s *groupService) IsGroupMember(groupID, userID string) (bool, error) {
	return s.db.IsGroupMember(groupID, userID)
}

// IsGroupAdmin checks if a user is an admin of a group
func (s *groupService) IsGroupAdmin(groupID, userID string) (bool, error) {
	return s.db.IsGroupAdmin(groupID, userID)
}

// GetGroupMembers returns all members of a group
func (s *groupService) GetGroupMembers(groupID string) ([]*models.GroupMember, error) {
	return s.db.GetGroupMembers(groupID)
}

// GetGroupAdmins returns all admin members of a group
func (s *groupService) GetGroupAdmins(groupID string) ([]*models.GroupMember, error) {
	return s.db.GetGroupAdmins(groupID)
}

// DeleteGroup deletes a group if the user is an admin
func (s *groupService) DeleteGroup(groupID, userID string) error {
	// Verify the user is an admin of the group
	isAdmin, err := s.IsGroupAdmin(groupID, userID)
	if err != nil {
		return err
	}

	if !isAdmin {
		return errors.New("user is not authorized to delete this group")
	}

	return s.db.DeleteGroup(groupID)
}

// CreateInvitation creates a new group invitation
func (s *groupService) CreateInvitation(invitation *models.GroupInvitation) error {
	return s.db.CreateInvitation(invitation)
}

// GetInvitation retrieves an invitation by ID
func (s *groupService) GetInvitation(invitationID string) (*models.GroupInvitation, error) {
	return s.db.GetInvitationByID(invitationID)
}

// UpdateInvitation updates an existing invitation's status
func (s *groupService) UpdateInvitation(invitation *models.GroupInvitation) error {
	return s.db.UpdateInvitation(invitation.ID, invitation.Status)
}

// AddGroupEvent adds an event to a group
func (s *groupService) AddGroupEvent(groupEvent *models.GroupEvent) error {
	return s.db.AddGroupEvent(groupEvent)
}

// RemoveEventFromGroup removes an event from a specific group
func (s *groupService) RemoveEventFromGroup(groupID, eventID string) error {
	return s.db.RemoveGroupEvent(groupID, eventID)
}

// GetGroupEvents returns all events for a specific group
func (s *groupService) GetGroupEvents(groupID string) ([]*models.GroupEvent, error) {
	return s.db.GetGroupEvents(groupID)
}

// RemoveEventFromAllGroups removes an event from all groups
func (s *groupService) RemoveEventFromAllGroups(eventID string) error {
	return s.db.RemoveEventFromAllGroups(eventID)
}

// RemoveUserFromAllGroups removes a user from all groups
func (s *groupService) RemoveUserFromAllGroups(userID string) error {
	// Get all groups the user is a member of
	groups, err := s.db.ListUserGroups(userID)
	if err != nil {
		return err
	}

	// Remove user from each group
	for _, group := range groups {
		if err := s.db.RemoveGroupMember(group.ID, userID); err != nil {
			return err
		}
	}

	return nil
}

// HandleUserDeleted handles the user_deleted event
func (s *groupService) HandleUserDeleted(userID string) error {
	// Remove user from all groups
	return s.RemoveUserFromAllGroups(userID)
}

// HandleEventDeleted handles the event_deleted event
func (s *groupService) HandleEventDeleted(eventID string) error {
	return s.db.RemoveEventFromAllGroups(eventID)
}

// GetSubGroups returns all direct child groups of a parent group
func (s *groupService) GetSubGroups(parentGroupID string) ([]*models.Group, error) {
	return s.db.GetSubGroups(parentGroupID)
}

// GetGroupEvent retrieves a group event by event ID
func (s *groupService) GetGroupEvent(eventID string) (*models.GroupEvent, error) {
	return s.db.GetGroupEvent(eventID)
}

// GetEventStatuses retrieves all statuses for an event
func (s *groupService) GetEventStatuses(eventID string) ([]*models.GroupEventStatus, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	statuses, err := s.eventStatusRepo.GetEventStatuses(tx, eventID)
	if err != nil {
		return nil, err
	}

	return statuses, nil
}

// UpdateEventStatus updates the status of an event for a user
func (s *groupService) UpdateEventStatus(status *models.GroupEventStatus) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Check if status already exists
	existing, err := s.eventStatusRepo.GetEventStatus(tx, status.EventID, status.UserID)
	if err != nil {
		return err
	}

	if existing == nil {
		// Create new status
		if status.ID == "" {
			status.ID = uuid.New().String()
		}
		return s.eventStatusRepo.CreateEventStatus(tx, status)
	}

	// Update existing status
	existing.Status = status.Status
	existing.RespondedAt = status.RespondedAt
	existing.UpdatedAt = time.Now().UTC()

	return s.eventStatusRepo.UpdateEventStatus(tx, existing)
}

// GetEventStatus retrieves the status of an event for a specific user
func (s *groupService) GetEventStatus(eventID, userID string) (*models.GroupEventStatus, error) {
	// Get event status from the repository
	return s.eventStatusRepo.GetEventStatus(nil, eventID, userID)
}

// HasAllMembersAccepted checks if all members of a non-hierarchical group have accepted an event
func (s *groupService) HasAllMembersAccepted(groupID, eventID string) (bool, error) {
	// Get the group to check if it's hierarchical
	group, err := s.GetGroup(groupID)
	if err != nil {
		return false, fmt.Errorf("error getting group: %w", err)
	}

	// For hierarchical groups, we consider the event as accepted for all members
	if group.IsHierarchical {
		// Update the group_events status to 'accepted' for hierarchical groups
		err := s.UpdateGroupEventStatus(groupID, eventID, "accepted")
		if err != nil {
			log.Printf("Error updating group event status for hierarchical group: %v", err)
			return false, fmt.Errorf("error updating group event status: %w", err)
		}
		return true, nil
	}

	// For non-hierarchical groups, check if all members have accepted
	allAccepted, err := s.eventStatusRepo.HasAllMembersAccepted(nil, groupID, eventID)
	if err != nil {
		return false, fmt.Errorf("error checking member acceptances: %w", err)
	}

	// If all members have accepted, update the group_events status
	if allAccepted {
		err := s.UpdateGroupEventStatus(groupID, eventID, "accepted")
		if err != nil {
			log.Printf("Error updating group event status after all members accepted: %v", err)
			return false, fmt.Errorf("error updating group event status: %w", err)
		}
	}

	return allAccepted, nil
}

// UpdateGroupEventStatus updates the status of a group event
func (s *groupService) UpdateGroupEventStatus(groupID, eventID, status string) error {
	// Validate status
	if status != "accepted" && status != "rejected" && status != "pending" {
		return fmt.Errorf("invalid status: %s", status)
	}

	// Start a transaction
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Check if the updated_at column exists in the group_events table
	var columnExists bool
	err = tx.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('group_events') 
		WHERE name = 'updated_at'`).Scan(&columnExists)

	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error checking for updated_at column: %w", err)
	}

	// Build the query based on whether updated_at exists
	var query string
	var args []interface{}

	if columnExists {
		query = `
			UPDATE group_events 
			SET status = ?, 
			    updated_at = ?
			WHERE group_id = ? AND event_id = ?
		`
		args = []interface{}{status, time.Now().UTC(), groupID, eventID}
	} else {
		query = `
			UPDATE group_events 
			SET status = ?
			WHERE group_id = ? AND event_id = ?
		`
		args = []interface{}{status, groupID, eventID}
	}

	// Execute the update
	result, err := tx.Exec(query, args...)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error updating group event status: %w", err)
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("no group event found with group_id %s and event_id %s", groupID, eventID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("✅ Updated group event status for group %s and event %s to %s", groupID, eventID, status)
	return nil
}

// BeginTx starts a new database transaction
func (s *groupService) BeginTx() (*sql.Tx, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}
	return tx, nil
}

// CommitTx commits a transaction
func (s *groupService) CommitTx(tx *sql.Tx) error {
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}
	return nil
}

// RollbackTx rolls back a transaction
func (s *groupService) RollbackTx(tx *sql.Tx) error {
	if tx == nil {
		return errors.New("transaction is nil")
	}

	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("error rolling back transaction: %v", err)
	}
	return nil
}

// HasPendingInvitation checks if there's already a pending invitation for a user in a group
func (s *groupService) HasPendingInvitation(groupID, userID string) (bool, error) {
	return s.db.HasPendingInvitation(groupID, userID)
}

// GetUserInvitations retrieves all invitations for a user, optionally filtered by status
// If status is empty, all invitations are returned
func (s *groupService) GetUserInvitations(userID string, status string) ([]*models.GroupInvitation, error) {
	if userID == "" {
		return nil, errors.New("user ID is required")
	}

	// Validate status if provided
	if status != "" {
		validStatuses := map[string]bool{
			"pending":  true,
			"accepted": true,
			"rejected": true,
		}
		if !validStatuses[status] {
			return nil, fmt.Errorf("invalid status: %s. Must be one of: pending, accepted, rejected", status)
		}
	}

	return s.db.GetUserInvitations(userID, status)
}
