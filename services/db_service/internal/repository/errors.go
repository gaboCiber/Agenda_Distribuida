package repository

import "errors"

var (
	// ErrGroupNotFound is returned when a group is not found
	ErrGroupNotFound = errors.New("group not found")
	// ErrUserAlreadyMember is returned when a user is already a member of a group
	ErrUserAlreadyMember = errors.New("user is already a member of this group")
	// ErrMemberNotFound is returned when a member is not found in a group
	ErrMemberNotFound = errors.New("member not found in group")
	// ErrLastAdmin is returned when trying to remove the last admin from a group
	ErrLastAdmin = errors.New("cannot remove the last admin from the group")
)
