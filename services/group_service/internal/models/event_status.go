package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

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
	ID          string      `json:"id"`
	GroupID     string      `json:"group_id"`
	EventID     string      `json:"event_id"`
	UserID      string      `json:"user_id"`
	Status      EventStatus `json:"status"`
	RespondedAt *time.Time  `json:"responded_at,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// EventStatusRepository defines the interface for event status operations
type EventStatusRepository interface {
	CreateEventStatus(tx *sql.Tx, status *GroupEventStatus) error
	UpdateEventStatus(tx *sql.Tx, status *GroupEventStatus) error
	GetEventStatuses(tx *sql.Tx, eventID string) ([]*GroupEventStatus, error)
	GetEventStatus(tx *sql.Tx, eventID, userID string) (*GroupEventStatus, error)
	DeleteEventStatus(tx *sql.Tx, eventID, userID string) error
	// HasAllMembersAccepted checks if all members of a non-hierarchical group have accepted an event
	HasAllMembersAccepted(tx *sql.Tx, groupID, eventID string) (bool, error)
}

// NewGroupEventStatus creates a new GroupEventStatus with default values
func NewGroupEventStatus(groupID, eventID, userID string, status EventStatus) *GroupEventStatus {
	now := time.Now().UTC()
	return &GroupEventStatus{
		ID:        uuid.New().String(),
		GroupID:   groupID,
		EventID:   eventID,
		UserID:    userID,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsValid checks if the event status is valid
func (s EventStatus) IsValid() bool {
	switch s {
	case EventStatusPending, EventStatusAccepted, EventStatusRejected:
		return true
	default:
		return false
	}
}

// String returns the string representation of the status
func (s EventStatus) String() string {
	return string(s)
}
