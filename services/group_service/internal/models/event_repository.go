package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AddGroupEvent adds an event to a group
func (d *Database) AddGroupEvent(groupEvent *GroupEvent) error {
	groupEvent.ID = uuid.New().String()
	groupEvent.AddedAt = time.Now().UTC()

	// Check if the event is already in the group
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM group_events 
		WHERE group_id = ? AND event_id = ?`,
		groupEvent.GroupID, groupEvent.EventID,
	).Scan(&count)

	if err != nil {
		return err
	}

	if count > 0 {
		return errors.New("event already exists in this group")
	}

	// Convert boolean to int for SQLite (0 or 1)
	isHierarchicalInt := 0
	if groupEvent.IsHierarchical {
		isHierarchicalInt = 1
	}

	_, err = d.db.Exec(
		`INSERT INTO group_events (id, group_id, event_id, added_by, added_at, status, is_hierarchical)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		groupEvent.ID,
		groupEvent.GroupID,
		groupEvent.EventID,
		groupEvent.AddedBy,
		groupEvent.AddedAt,
		groupEvent.Status,
		isHierarchicalInt,
	)

	return err
}

// RemoveGroupEvent removes an event from a group
func (d *Database) RemoveGroupEvent(groupID, eventID string) error {
	result, err := d.db.Exec(
		`DELETE FROM group_events 
		WHERE group_id = ? AND event_id = ?`,
		groupID, eventID,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GetGroupEvents returns all events in a group
func (d *Database) GetGroupEvents(groupID string) ([]*GroupEvent, error) {
	rows, err := d.db.Query(
		`SELECT id, group_id, event_id, added_by, added_at, status, is_hierarchical
		FROM group_events 
		WHERE group_id = ?
		ORDER BY added_at DESC`,
		groupID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*GroupEvent
	for rows.Next() {
		var event GroupEvent
		var isHierarchicalInt int

		scanErr := rows.Scan(
			&event.ID,
			&event.GroupID,
			&event.EventID,
			&event.AddedBy,
			&event.AddedAt,
			&event.Status,
			&isHierarchicalInt,
		)

		if scanErr != nil {
			return nil, scanErr
		}

		// Convert int to boolean
		event.IsHierarchical = (isHierarchicalInt == 1)
		events = append(events, &event)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

// CreateInvitation creates a new group invitation
func (d *Database) CreateInvitation(invitation *GroupInvitation) error {
	invitation.ID = uuid.New().String()
	invitation.CreatedAt = time.Now().UTC()
	invitation.Status = "pending"

	// Check if user is already a member
	isMember, err := d.IsGroupMember(invitation.GroupID, invitation.UserID)
	if err != nil {
		return err
	}

	if isMember {
		return errors.New("user is already a member of this group")
	}

	// Check for existing pending invitation
	var count int
	err = d.db.QueryRow(
		`SELECT COUNT(*) FROM group_invitations 
		WHERE group_id = ? AND user_id = ? AND status = 'pending'`,
		invitation.GroupID, invitation.UserID,
	).Scan(&count)

	if err != nil {
		return err
	}

	if count > 0 {
		return errors.New("user already has a pending invitation to this group")
	}

	_, err = d.db.Exec(
		`INSERT INTO group_invitations 
		(id, group_id, user_id, invited_by, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		invitation.ID,
		invitation.GroupID,
		invitation.UserID,
		invitation.InvitedBy,
		invitation.Status,
		invitation.CreatedAt,
	)

	return err
}

// GetInvitationByID retrieves an invitation by its ID
func (d *Database) GetInvitationByID(id string) (*GroupInvitation, error) {
	invitation := &GroupInvitation{}
	var respondedAt sql.NullTime // Usar sql.NullTime para manejar valores NULL

	err := d.db.QueryRow(
		`SELECT id, group_id, user_id, invited_by, status, created_at, responded_at
		FROM group_invitations 
		WHERE id = ?`,
		id,
	).Scan(
		&invitation.ID,
		&invitation.GroupID,
		&invitation.UserID,
		&invitation.InvitedBy,
		&invitation.Status,
		&invitation.CreatedAt,
		&respondedAt, // Escanear a sql.NullTime
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("error retrieving invitation: %v", err)
	}

	// Asignar el valor de respondedAt si no es NULL
	if respondedAt.Valid {
		invitation.RespondedAt = respondedAt.Time
	} else {
		// Si es NULL, asignar el valor cero de time.Time
		invitation.RespondedAt = time.Time{}
	}

	return invitation, nil
}

// UpdateInvitation updates an invitation's status
func (d *Database) UpdateInvitation(id, status string) error {
	invitation, err := d.GetInvitationByID(id)
	if err != nil {
		return err
	}

	if invitation == nil {
		return sql.ErrNoRows
	}

	if invitation.Status != "pending" {
		return errors.New("invitation has already been processed")
	}

	_, err = d.db.Exec(
		`UPDATE group_invitations 
		SET status = ?, responded_at = ? 
		WHERE id = ?`,
		status,
		time.Now().UTC(),
		id,
	)

	return err
}

// GetUserInvitations returns all invitations for a user, optionally filtered by status
// If status is empty, all invitations are returned
func (d *Database) GetUserInvitations(userID, status string) ([]*GroupInvitation, error) {
	if userID == "" {
		return nil, errors.New("user ID is required")
	}

	// Build the query based on whether status is provided
	query := `
		SELECT i.id, i.group_id, i.user_id, i.invited_by, i.status, 
		i.created_at, i.responded_at, g.name as group_name, g.description as group_description
		FROM group_invitations i
		JOIN groups g ON i.group_id = g.id
		WHERE i.user_id = ?
	`

	var args []interface{}
	args = append(args, userID)

	// Add status filter if provided
	if status != "" {
		// Validate status
		validStatuses := map[string]bool{
			"pending":  true,
			"accepted": true,
			"rejected": true,
		}
		if !validStatuses[status] {
			return nil, fmt.Errorf("invalid status: %s. Must be one of: pending, accepted, rejected", status)
		}
		query += " AND i.status = ?"
		args = append(args, status)
	}

	query += " ORDER BY i.created_at DESC"

	rows, err := d.db.Query(query, args...)

	if err != nil {
		return nil, fmt.Errorf("error querying invitations: %v", err)
	}
	defer rows.Close()

	var invitations []*GroupInvitation
	for rows.Next() {
		var invitation GroupInvitation
		var groupName string
		var respondedAt sql.NullTime // Usar sql.NullTime para manejar valores NULL

		var groupDesc string
		err := rows.Scan(
			&invitation.ID,
			&invitation.GroupID,
			&invitation.UserID,
			&invitation.InvitedBy,
			&invitation.Status,
			&invitation.CreatedAt,
			&respondedAt, // Escanear a sql.NullTime
			&groupName,
			&groupDesc, // Agregar group_description
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning invitation row: %v", err)
		}

		// Asignar el valor de respondedAt si no es NULL
		if respondedAt.Valid {
			invitation.RespondedAt = respondedAt.Time
		} else {
			// Si es NULL, asignar el valor cero de time.Time
			invitation.RespondedAt = time.Time{}
		}

		invitations = append(invitations, &invitation)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating invitation rows: %v", err)
	}

	return invitations, nil
}

// DeleteUserInvitations deletes all invitations for a user
func (d *Database) DeleteUserInvitations(userID string) error {
	_, err := d.db.Exec(
		`DELETE FROM group_invitations 
		WHERE user_id = ?`,
		userID,
	)
	return err
}

// RemoveEventFromAllGroups removes an event from all groups
func (d *Database) RemoveEventFromAllGroups(eventID string) error {
	_, err := d.db.Exec(
		`DELETE FROM group_events 
		WHERE event_id = ?`,
		eventID,
	)
	return err
}

// AddGroupEventWithTx adds an event to a group within a transaction
func (d *Database) AddGroupEventWithTx(tx *sql.Tx, groupEvent *GroupEvent) error {
	groupEvent.ID = uuid.New().String()
	groupEvent.AddedAt = time.Now().UTC()

	_, err := tx.Exec(
		`INSERT INTO group_events (id, group_id, event_id, added_by, added_at)
		VALUES (?, ?, ?, ?, ?)`,
		groupEvent.ID,
		groupEvent.GroupID,
		groupEvent.EventID,
		groupEvent.AddedBy,
		groupEvent.AddedAt,
	)

	return err
}

// GetGroupEvent retrieves a group event by ID
func (d *Database) GetGroupEvent(eventID string) (*GroupEvent, error) {
	var event GroupEvent
	var isHierarchicalInt int

	err := d.db.QueryRow(
		`SELECT id, group_id, event_id, added_by, added_at, status, is_hierarchical
		FROM group_events 
		WHERE event_id = ?`,
		eventID,
	).Scan(
		&event.ID,
		&event.GroupID,
		&event.EventID,
		&event.AddedBy,
		&event.AddedAt,
		&event.Status,
		&isHierarchicalInt,
	)

	// Convert int to boolean
	event.IsHierarchical = (isHierarchicalInt == 1)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("event not found")
		}
		return nil, err
	}

	return &event, nil
}

// AddEventStatus adds a new event status record
func (d *Database) AddEventStatus(status *GroupEventStatus) error {
	status.ID = uuid.New().String()
	now := time.Now().UTC()
	status.CreatedAt = now
	status.UpdatedAt = now

	_, err := d.db.Exec(
		`INSERT INTO group_event_status 
		(id, group_id, event_id, user_id, status, responded_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		status.ID,
		status.GroupID,
		status.EventID,
		status.UserID,
		status.Status,
		status.RespondedAt,
		status.CreatedAt,
		status.UpdatedAt,
	)

	return err
}

// AddEventStatusWithTx adds a new event status record within a transaction
func (d *Database) AddEventStatusWithTx(tx *sql.Tx, status *GroupEventStatus) error {
	status.ID = uuid.New().String()
	now := time.Now().UTC()
	status.CreatedAt = now
	status.UpdatedAt = now

	_, err := tx.Exec(
		`INSERT INTO group_event_status 
		(id, group_id, event_id, user_id, status, responded_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		status.ID,
		status.GroupID,
		status.EventID,
		status.UserID,
		status.Status,
		status.RespondedAt,
		status.CreatedAt,
		status.UpdatedAt,
	)

	return err
}

// UpdateEventStatus updates the status of an event for a user
func (d *Database) UpdateEventStatus(eventID, userID string, status EventStatus) error {
	now := time.Now().UTC()
	var respondedAt interface{}
	if status != EventStatusPending {
		respondedAt = now
	}

	result, err := d.db.Exec(
		`UPDATE group_event_status 
		SET status = ?, responded_at = ?, updated_at = ?
		WHERE event_id = ? AND user_id = ?`,
		status, respondedAt, now, eventID, userID,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("event status not found for user %s and event %s", userID, eventID)
	}

	return nil
}

// GetEventStatus retrieves the status of an event for a specific user
func (d *Database) GetEventStatus(eventID, userID string) (*GroupEventStatus, error) {
	var status GroupEventStatus

	err := d.db.QueryRow(
		`SELECT id, group_id, event_id, user_id, status, responded_at, created_at, updated_at
		FROM group_event_status
		WHERE event_id = ? AND user_id = ?`,
		eventID, userID,
	).Scan(
		&status.ID,
		&status.GroupID,
		&status.EventID,
		&status.UserID,
		&status.Status,
		&status.RespondedAt,
		&status.CreatedAt,
		&status.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("event status not found for user %s and event %s", userID, eventID)
		}
		return nil, err
	}

	return &status, nil
}

// GetEventStatuses retrieves all statuses for an event
func (d *Database) GetEventStatuses(eventID string) ([]*GroupEventStatus, error) {
	rows, err := d.db.Query(
		`SELECT id, group_id, event_id, user_id, status, responded_at, created_at, updated_at
		FROM group_event_status
		WHERE event_id = ?`,
		eventID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []*GroupEventStatus
	for rows.Next() {
		var status GroupEventStatus
		if err := rows.Scan(
			&status.ID,
			&status.GroupID,
			&status.EventID,
			&status.UserID,
			&status.Status,
			&status.RespondedAt,
			&status.CreatedAt,
			&status.UpdatedAt,
		); err != nil {
			return nil, err
		}
		statuses = append(statuses, &status)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return statuses, nil
}

// RemoveUserFromAllGroups removes a user from all groups
func (d *Database) RemoveUserFromAllGroups(userID string) error {
	// Start a transaction
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	// Delete all group memberships for the user
	_, err = tx.Exec(`
		DELETE FROM group_members 
		WHERE user_id = ? AND is_inherited = false
	`, userID)

	if err != nil {
		tx.Rollback()
		return err
	}

	// Then delete the user from all other groups
	_, err = tx.Exec(
		`DELETE FROM group_members 
		WHERE user_id = ?`,
		userID,
	)

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
