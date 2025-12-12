package models

import (
	"time"

	"github.com/google/uuid"
)

// Group represents a user group in the system
type Group struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	Description    *string    `json:"description,omitempty"`
	CreatedBy      uuid.UUID  `json:"created_by"`
	IsHierarchical bool       `json:"is_hierarchical"`
	ParentGroupID  *uuid.UUID `json:"parent_group_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
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
	Name           string     `json:"name"`
	Description    *string    `json:"description,omitempty"`
	IsHierarchical bool       `json:"is_hierarchical"`
	ParentGroupID  *uuid.UUID `json:"parent_group_id,omitempty"`
	CreatorID      uuid.UUID  `json:"creator_id"`
}

// GroupMember represents a member of a group
type GroupMember struct {
	ID          uuid.UUID `json:"id"`
	GroupID     uuid.UUID `json:"group_id"`
	UserID      uuid.UUID `json:"user_id"`
	UserName    string    `json:"user_name"`
	UserEmail   string    `json:"user_email"`
	Role        string    `json:"role"` // "admin" or "member"
	IsInherited bool      `json:"is_inherited"`
	JoinedAt    time.Time `json:"joined_at"`
}

// GroupMemberRequest represents the data needed to add a member to a group
type GroupMemberRequest struct {
	UserID      uuid.UUID `json:"user_id"`
	Role        string    `json:"role"` // "admin" or "member"
	IsInherited bool      `json:"is_inherited"`
}

// GroupInvitation represents an invitation to join a group
type GroupInvitation struct {
	ID          uuid.UUID `json:"id"`
	GroupID     uuid.UUID `json:"group_id"`
	UserID      uuid.UUID `json:"user_id"`
	UserEmail   string    `json:"email"`
	InvitedBy   uuid.UUID `json:"invited_by"`
	Status      string    `json:"status"` // "pending", "accepted", "rejected", "cancelled"
	CreatedAt   time.Time `json:"created_at"`
	RespondedAt time.Time `json:"responded_at,omitempty"`
	Message     string    `json:"message,omitempty"`
}

// GroupInvitationResponse represents an invitation to join a group with additional details
type GroupInvitationResponse struct {
	ID           uuid.UUID  `json:"id"`
	GroupID      uuid.UUID  `json:"group_id"`
	GroupName    string     `json:"group_name"`
	UserID       uuid.UUID  `json:"user_id"`
	UserEmail    string     `json:"email"`
	InvitedBy    uuid.UUID  `json:"invited_by"`
	InviterName  string     `json:"inviter_name"`
	InviterEmail string     `json:"inviter_email"`
	Status       string     `json:"status"` // "pending", "accepted", "rejected", "cancelled"
	CreatedAt    time.Time  `json:"created_at"`
	RespondedAt  *time.Time `json:"responded_at,omitempty"`
	Message      string     `json:"message,omitempty"`
}

// InvitationStatus represents the possible statuses of a group invitation
type InvitationStatus string

const (
	InvitationStatusPending   InvitationStatus = "pending"
	InvitationStatusAccepted  InvitationStatus = "accepted"
	InvitationStatusRejected  InvitationStatus = "rejected"
	InvitationStatusCancelled InvitationStatus = "cancelled"
)

// InvitationRequest represents the data needed to create a new group invitation
type InvitationRequest struct {
	GroupID   uuid.UUID `json:"group_id"`
	UserEmail string    `json:"email"`
	InvitedBy uuid.UUID `json:"invited_by"`
	Message   string    `json:"message,omitempty"`
}

// InvitationResponse represents the data needed to respond to a group invitation
type InvitationResponse struct {
	Status InvitationStatus `json:"status"`
	UserID uuid.UUID        `json:"user_id"`
}
