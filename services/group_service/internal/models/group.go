package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Group represents a user group in the system
type Group struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GroupMember represents a member of a group
type GroupMember struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"group_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"` // e.g., "admin", "member"
	JoinedAt  time.Time `json:"joined_at"`
}

// GroupEvent represents an event associated with a group
type GroupEvent struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"group_id"`
	EventID   string    `json:"event_id"`
	AddedBy   string    `json:"added_by"`
	AddedAt   time.Time `json:"added_at"`
}

// GroupInvitation represents an invitation to join a group
type GroupInvitation struct {
	ID          string    `json:"id"`
	GroupID     string    `json:"group_id"`
	UserID      string    `json:"user_id"`
	InvitedBy   string    `json:"invited_by"`
	Status      string    `json:"status"` // "pending", "accepted", "rejected"
	CreatedAt   time.Time `json:"created_at"`
	RespondedAt time.Time `json:"responded_at,omitempty"`
}

// Database wraps the database connection and provides group-related operations
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new Database instance
func NewDatabase(db *sql.DB) *Database {
	return &Database{db: db}
}
