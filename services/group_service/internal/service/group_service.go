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

	// Event operations
	AddGroupEvent(groupEvent *models.GroupEvent) error
	RemoveEventFromGroup(groupID, eventID string) error
	GetGroupEvents(groupID string) ([]*models.GroupEvent, error)
	RemoveEventFromAllGroups(eventID string) error

	// Event handlers
	HandleUserDeleted(userID string) error
	HandleEventDeleted(eventID string) error

	// Transaction management
	BeginTx() (*sql.Tx, error)
	CommitTx(tx *sql.Tx) error
	RollbackTx(tx *sql.Tx) error
}

// NewGroupService creates a new instance of GroupService
func NewGroupService(db *models.Database) GroupService {
	return &groupService{
		db: db,
	}
}

// groupService implements the GroupService interface
type groupService struct {
	db *models.Database
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

	// Add the creator as an admin if not already added
	if group.CreatedBy != "" {
		member := &models.GroupMember{
			ID:       uuid.New().String(),
			GroupID:  group.ID,
			UserID:   group.CreatedBy,
			Role:     "admin",
			JoinedAt: time.Now().UTC(),
		}

		if err := s.db.AddGroupMember(member); err != nil {
			// Log the error but don't fail the group creation
			// The group was created successfully, we just couldn't add the admin
			// This should be monitored and fixed as it's a critical part of group creation
			log.Printf("Warning: Failed to add creator as admin to group %s: %v", group.ID, err)
		}
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
	// Remove the event from all groups
	return s.RemoveEventFromAllGroups(eventID)
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
	if tx != nil {
		if err := tx.Rollback(); err != nil {
			return fmt.Errorf("failed to rollback transaction: %v", err)
		}
	}
	return nil
}
