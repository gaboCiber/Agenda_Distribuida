package service

import (
	"database/sql"
	"errors"

	"github.com/agenda-distribuida/group-service/internal/models"
)

// EventService defines the interface for event-related operations
type EventService interface {
	// Event operations
	AddGroupEvent(event *models.GroupEvent) error
	RemoveEventFromGroup(groupID, eventID string) error
	GetGroupEvents(groupID string) ([]*models.GroupEvent, error)
	UpdateEventStatus(status *models.GroupEventStatus) error
	GetEventStatuses(eventID string) ([]*models.GroupEventStatus, error)
}

type eventService struct {
	db *models.Database
}

// NewEventService creates a new instance of EventService
func NewEventService(db *models.Database) EventService {
	return &eventService{
		db: db,
	}
}

// AddGroupEvent adds an event to a group
func (s *eventService) AddGroupEvent(event *models.GroupEvent) error {
	// Check if the group exists
	_, err := s.db.GetGroupByID(event.GroupID)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("group not found")
		}
		return err
	}

	// Add the event to the group
	return s.db.AddGroupEvent(event)
}

// RemoveEventFromGroup removes an event from a group
func (s *eventService) RemoveEventFromGroup(groupID, eventID string) error {
	// Verify the group exists
	_, err := s.db.GetGroupByID(groupID)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("group not found")
		}
		return err
	}

	// Remove the event from the group
	return s.db.RemoveGroupEvent(groupID, eventID)
}

// GetGroupEvents returns all events for a specific group
func (s *eventService) GetGroupEvents(groupID string) ([]*models.GroupEvent, error) {
	// Verify the group exists
	_, err := s.db.GetGroupByID(groupID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("group not found")
		}
		return nil, err
	}

	// Get all events for the group
	return s.db.GetGroupEvents(groupID)
}

// UpdateEventStatus updates the status of an event for a user
func (s *eventService) UpdateEventStatus(status *models.GroupEventStatus) error {
	// Check if the event exists in any group
	_, err := s.db.GetGroupEvent(status.EventID)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("event not found in any group")
		}
		return err
	}

	// Update the event status
	return s.db.UpdateEventStatus(status.EventID, status.UserID, status.Status)
}

// GetEventStatuses returns all statuses for an event
func (s *eventService) GetEventStatuses(eventID string) ([]*models.GroupEventStatus, error) {
	// Check if the event exists in any group
	_, err := s.db.GetGroupEvent(eventID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("event not found in any group")
		}
		return nil, err
	}

	// Get all statuses for the event
	return s.db.GetEventStatuses(eventID)
}
