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
