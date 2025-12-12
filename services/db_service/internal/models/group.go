package models

import (
	"time"

	"github.com/google/uuid"
)

// Group represents a user group in the system
type Group struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	Name           string     `json:"name" db:"name"`
	Description    *string    `json:"description,omitempty" db:"description"`
	CreatedBy      uuid.UUID  `json:"created_by" db:"created_by"`
	IsHierarchical bool       `json:"is_hierarchical" db:"is_hierarchical"`
	ParentGroupID  *uuid.UUID `json:"parent_group_id,omitempty" db:"parent_group_id"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

type GroupExtended struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	Name           string     `json:"name" db:"name"`
	Description    *string    `json:"description,omitempty" db:"description"`
	CreatedBy      uuid.UUID  `json:"created_by" db:"created_by"`
	IsHierarchical bool       `json:"is_hierarchical" db:"is_hierarchical"`
	ParentGroupID  *uuid.UUID `json:"parent_group_id,omitempty" db:"parent_group_id"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
	Role           string     `json:"role" db:"role"` // "admin" or "member"
}

// GroupRequest represents the data needed to create or update a group
type GroupRequest struct {
	Name           string     `json:"name" validate:"required"`
	Description    *string    `json:"description,omitempty"`
	IsHierarchical bool       `json:"is_hierarchical"`
	ParentGroupID  *uuid.UUID `json:"parent_group_id,omitempty"`
	CreatorID      uuid.UUID  `json:"creator_id" validate:"required"`
}

// GroupMember represents a member of a group
type GroupMember struct {
	ID          uuid.UUID `json:"id" db:"id"`
	GroupID     uuid.UUID `json:"group_id" db:"group_id"`
	UserID      uuid.UUID `json:"user_id" db:"user_id"`
	UserName    string    `json:"user_name" db:"user_name"`
	UserEmail   string    `json:"user_email" db:"user_email"`
	Role        string    `json:"role" db:"role"` // "admin" or "member"
	IsInherited bool      `json:"is_inherited" db:"is_inherited"`
	JoinedAt    time.Time `json:"joined_at" db:"joined_at"`
}

// GroupMemberRequest represents the data needed to add a member to a group
type GroupMemberRequest struct {
	UserID      uuid.UUID `json:"user_id" validate:"required"`
	Role        string    `json:"role" validate:"required,oneof=admin member"`
	IsInherited bool      `json:"is_inherited"`
}

// GroupEvent represents an event associated with a group
type GroupEvent struct {
	ID             uuid.UUID `json:"id" db:"id"`
	GroupID        uuid.UUID `json:"group_id" db:"group_id"`
	EventID        uuid.UUID `json:"event_id" db:"event_id"`
	AddedBy        uuid.UUID `json:"added_by" db:"added_by"`
	IsHierarchical bool      `json:"is_hierarchical" db:"is_hierarchical"`
	Status         string    `json:"status" db:"status"`
	AddedAt        time.Time `json:"added_at" db:"added_at"`
}

// GroupInvitation represents an invitation to join a group
type GroupInvitation struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	GroupID     uuid.UUID  `json:"group_id" db:"group_id"`
	UserID      uuid.UUID  `json:"user_id" db:"user_id"`
	UserEmail   string     `json:"email" db:"email"`
	InvitedBy   uuid.UUID  `json:"invited_by" db:"invited_by"`
	Status      string     `json:"status" db:"status"` // "pending", "accepted", "rejected"
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	RespondedAt *time.Time `json:"responded_at,omitempty" db:"responded_at"`
}

// GroupInvitationResponse represents an invitation to join a group with additional details
type GroupInvitationResponse struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	GroupID     uuid.UUID  `json:"group_id" db:"group_id"`
	GroupName   string     `json:"group_name" db:"group_name"`
	UserID      uuid.UUID  `json:"user_id" db:"user_id"`
	UserEmail   string     `json:"email" db:"email"`
	InvitedBy   uuid.UUID  `json:"invited_by" db:"invited_by"`
	InviterName string     `json:"inviter_name" db:"inviter_name"`
	InviterEmail string    `json:"inviter_email" db:"inviter_email"`
	Status      string     `json:"status" db:"status"` // "pending", "accepted", "rejected"
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	RespondedAt *time.Time `json:"responded_at,omitempty" db:"responded_at"`
}

// EventStatus represents the status of an event for a specific user
type EventStatus string

// Possible event status values
const (
	EventStatusPending  EventStatus = "pending"
	EventStatusAccepted EventStatus = "accepted"
	EventStatusRejected EventStatus = "rejected"
)

// GroupEventStatus represents the status of an event for a specific user in a group
type GroupEventStatus struct {
	ID          uuid.UUID   `json:"id" db:"id"`
	GroupID     uuid.UUID   `json:"group_id" db:"group_id"`
	EventID     uuid.UUID   `json:"event_id" db:"event_id"`
	UserID      uuid.UUID   `json:"user_id" db:"user_id"`
	Status      EventStatus `json:"status" db:"status"`
	RespondedAt *time.Time  `json:"responded_at,omitempty" db:"responded_at"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
}

// EventStatusRequest represents the data needed to update an event status
type EventStatusRequest struct {
	Status EventStatus `json:"status" validate:"required,oneof=pending accepted rejected"`
}

// NewGroupEventStatus creates a new GroupEventStatus with default values
func NewGroupEventStatus(groupID, eventID, userID uuid.UUID, status EventStatus) *GroupEventStatus {
	now := time.Now().UTC()
	return &GroupEventStatus{
		ID:        uuid.New(),
		GroupID:   groupID,
		EventID:   eventID,
		UserID:    userID,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsValid checks if the event status is valid
func (es EventStatus) IsValid() bool {
	switch es {
	case EventStatusPending, EventStatusAccepted, EventStatusRejected:
		return true
	default:
		return false
	}
}

// String returns the string representation of the status
func (es EventStatus) String() string {
	return string(es)
}
