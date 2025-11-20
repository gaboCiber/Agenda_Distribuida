package repository

import "errors"

var (
	// Group errors
	// ErrGroupNotFound is returned when a group is not found
	ErrGroupNotFound = errors.New("group not found")
	// ErrUserAlreadyMember is returned when a user is already a member of a group
	ErrUserAlreadyMember = errors.New("user is already a member of this group")
	// ErrMemberNotFound is returned when a member is not found in a group
	ErrMemberNotFound = errors.New("member not found in group")
	// ErrLastAdmin is returned when trying to remove the last admin from a group
	ErrLastAdmin = errors.New("cannot remove the last admin from the group")

	// Event errors
	// ErrEventAlreadyInGroup is returned when an event is already associated with a group
	ErrEventAlreadyInGroup = errors.New("event is already associated with this group")
	// ErrEventNotInGroup is returned when an event is not associated with a group
	ErrEventNotInGroup = errors.New("event is not associated with this group")
	// ErrGroupEventNotFound is returned when a group event is not found
	ErrGroupEventNotFound = errors.New("group event not found")
	// ErrInvalidEventStatus is returned when an invalid event status is provided
	ErrInvalidEventStatus = errors.New("invalid event status")
	// ErrEventStatusNotFound is returned when an event status is not found
	ErrEventStatusNotFound = errors.New("event status not found")

	// Invitation errors
	// ErrInvitationNotFound is returned when an invitation is not found
	ErrInvitationNotFound = errors.New("invitation not found")
)

// Database represents the database connection wrapper
type Database struct {
	// This is a placeholder for the actual database implementation
	// You should replace this with your actual database connection type
}
