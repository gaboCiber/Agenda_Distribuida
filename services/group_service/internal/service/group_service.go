package service

import (
	"errors"
	"time"

	"github.com/agenda-distribuida/group-service/internal/models"
)

// GroupService defines the interface for group operations
type GroupService interface {
	CreateGroup(group *models.Group) (*models.Group, error)
	GetGroup(groupID string) (*models.Group, error)
	UpdateGroup(group *models.Group) error
	DeleteGroup(groupID, userID string) error
	ListUserGroups(userID string) ([]*models.Group, error)

	// Member operations
	AddGroupMember(member *models.GroupMember) error
	RemoveGroupMember(groupID, userID string) error
	GetGroupMember(groupID, userID string) (*models.GroupMember, error)
	IsGroupMember(groupID, userID string) (bool, error)
	IsGroupAdmin(groupID, userID string) (bool, error)
	GetGroupMembers(groupID string) ([]*models.GroupMember, error)
	GetGroupAdmins(groupID string) ([]*models.GroupMember, error)

	// Invitation operations
	CreateInvitation(invitation *models.GroupInvitation) error
	GetInvitation(invitationID string) (*models.GroupInvitation, error)
	UpdateInvitation(invitation *models.GroupInvitation) error

	// Group-Event operations
	AddGroupEvent(groupEvent *models.GroupEvent) error
	RemoveEventFromGroup(groupID, eventID string) error
	RemoveEventFromAllGroups(eventID string) error

	// Event handlers
	HandleUserDeleted(userID string) error
	HandleEventDeleted(eventID string) error
	RemoveUserFromAllGroups(userID string) error
}

type groupService struct {
	db *models.Database
}

// NewGroupService creates a new group service
func NewGroupService(db *models.Database) GroupService {
	return &groupService{
		db: db,
	}
}

// CreateGroup creates a new group and adds the creator as an admin
func (s *groupService) CreateGroup(group *models.Group) (*models.Group, error) {
	// Set creation time
	group.CreatedAt = time.Now()

	// Create the group in the database (repository handles adding the creator as admin)
	if err := s.db.CreateGroup(group); err != nil {
		return nil, err
	}
	
	// Get the created group to return with the database-generated ID
	return s.db.GetGroupByID(group.ID)
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
	group.UpdatedAt = time.Now()

	return s.db.UpdateGroup(group)
}

// AddGroupMember adds a user to a group
func (s *groupService) AddGroupMember(member *models.GroupMember) error {
	// Set joined at time if not set
	if member.JoinedAt.IsZero() {
		member.JoinedAt = time.Now()
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
	// Remove event from all groups
	return s.RemoveEventFromAllGroups(eventID)
}
