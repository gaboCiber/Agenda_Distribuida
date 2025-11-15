package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/agenda-distribuida/db-service/internal/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// GroupEventRepository defines the interface for group event operations
type GroupEventRepository interface {
	// Transaction support
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)

	// Group Event Management
	AddGroupEvent(ctx context.Context, groupEvent *models.GroupEvent) error
	AddGroupEventWithTx(ctx context.Context, tx *sql.Tx, groupEvent *models.GroupEvent) error
	RemoveGroupEvent(ctx context.Context, groupID, eventID uuid.UUID) error
	GetGroupEvents(ctx context.Context, groupID uuid.UUID) ([]*models.GroupEvent, error)
	GetGroupEvent(ctx context.Context, eventID uuid.UUID) (*models.GroupEvent, error)
	RemoveEventFromAllGroups(ctx context.Context, eventID uuid.UUID) error

	// Event Status Management
	AddEventStatus(ctx context.Context, status *models.GroupEventStatus) error
	AddEventStatusWithTx(ctx context.Context, tx *sql.Tx, status *models.GroupEventStatus) error
	BatchCreateEventStatus(ctx context.Context, tx *sql.Tx, statuses []*models.GroupEventStatus) error
	UpdateEventStatus(ctx context.Context, eventID, userID uuid.UUID, status models.EventStatus) error
	UpdateEventStatuses(ctx context.Context, tx *sql.Tx, statuses []*models.GroupEventStatus) error
	GetEventStatus(ctx context.Context, eventID, userID uuid.UUID) (*models.GroupEventStatus, error)
	GetEventStatuses(ctx context.Context, eventID uuid.UUID) ([]*models.GroupEventStatus, error)
	GetEventStatusesByGroup(ctx context.Context, groupID, eventID uuid.UUID) ([]*models.GroupEventStatus, error)
	GetEventStatusCounts(ctx context.Context, eventID uuid.UUID) (map[models.EventStatus]int, error)
	HasResponded(ctx context.Context, eventID, userID uuid.UUID) (bool, error)
	HasAllMembersAccepted(ctx context.Context, groupID, eventID uuid.UUID) (bool, error)
	DeleteEventStatus(ctx context.Context, tx *sql.Tx, eventID, userID uuid.UUID) error
	DeleteEventStatuses(ctx context.Context, tx *sql.Tx, eventID uuid.UUID) error
	DeleteEventStatusesByGroup(ctx context.Context, tx *sql.Tx, groupID, eventID uuid.UUID) error

	// Invitation Management
	CreateInvitation(ctx context.Context, invitation *models.GroupInvitation) error
	GetInvitationByID(ctx context.Context, id uuid.UUID) (*models.GroupInvitation, error)
	UpdateInvitation(ctx context.Context, id uuid.UUID, status string) error
	GetUserInvitations(ctx context.Context, userID uuid.UUID, status string) ([]*models.GroupInvitation, error)
	DeleteUserInvitations(ctx context.Context, userID uuid.UUID) error
}

type groupEventRepository struct {
	db  *sql.DB
	log zerolog.Logger
}

// BeginTx starts a new transaction
func (r *groupEventRepository) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, opts)
}

// NewGroupEventRepository creates a new instance of GroupEventRepository
func NewGroupEventRepository(db *sql.DB, log zerolog.Logger) GroupEventRepository {
	return &groupEventRepository{
		db:  db,
		log: log,
	}
}

// AddGroupEvent adds an event to a group
func (r *groupEventRepository) AddGroupEvent(ctx context.Context, groupEvent *models.GroupEvent) error {
	if groupEvent.ID == uuid.Nil {
		groupEvent.ID = uuid.New()
	}
	if groupEvent.AddedAt.IsZero() {
		groupEvent.AddedAt = time.Now().UTC()
	}

	// Check if the event is already in the group
	var count int
	err := r.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM group_events WHERE group_id = $1 AND event_id = $2`,
		groupEvent.GroupID,
		groupEvent.EventID,
	).Scan(&count)

	if err != nil {
		r.log.Error().Err(err).
			Str("group_id", groupEvent.GroupID.String()).
			Str("event_id", groupEvent.EventID.String()).
			Msg("Failed to check if event exists in group")
		return fmt.Errorf("failed to check if event exists in group: %w", err)
	}

	if count > 0 {
		return ErrEventAlreadyInGroup
	}

	// Convert boolean to int for SQLite (0 or 1)
	isHierarchicalInt := 0
	if groupEvent.IsHierarchical {
		isHierarchicalInt = 1
	}

	_, err = r.db.ExecContext(
		ctx,
		`INSERT INTO group_events (id, group_id, event_id, added_by, added_at, status, is_hierarchical)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		groupEvent.ID,
		groupEvent.GroupID,
		groupEvent.EventID,
		groupEvent.AddedBy,
		groupEvent.AddedAt,
		groupEvent.Status,
		isHierarchicalInt,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("group_id", groupEvent.GroupID.String()).
			Str("event_id", groupEvent.EventID.String()).
			Msg("Failed to add event to group")
		return fmt.Errorf("failed to add event to group: %w", err)
	}

	return nil
}

// AddGroupEventWithTx adds an event to a group within a transaction
func (r *groupEventRepository) AddGroupEventWithTx(ctx context.Context, tx *sql.Tx, groupEvent *models.GroupEvent) error {
	if groupEvent.ID == uuid.Nil {
		groupEvent.ID = uuid.New()
	}
	if groupEvent.AddedAt.IsZero() {
		groupEvent.AddedAt = time.Now().UTC()
	}

	// Convert boolean to int for SQLite (0 or 1)
	isHierarchicalInt := 0
	if groupEvent.IsHierarchical {
		isHierarchicalInt = 1
	}

	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO group_events (id, group_id, event_id, added_by, added_at, status, is_hierarchical)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		groupEvent.ID,
		groupEvent.GroupID,
		groupEvent.EventID,
		groupEvent.AddedBy,
		groupEvent.AddedAt,
		groupEvent.Status,
		isHierarchicalInt,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("group_id", groupEvent.GroupID.String()).
			Str("event_id", groupEvent.EventID.String()).
			Msg("Failed to add event to group in transaction")
		return fmt.Errorf("failed to add event to group in transaction: %w", err)
	}

	return nil
}

// RemoveGroupEvent removes an event from a group
func (r *groupEventRepository) RemoveGroupEvent(ctx context.Context, groupID, eventID uuid.UUID) error {
	result, err := r.db.ExecContext(
		ctx,
		`DELETE FROM group_events WHERE group_id = $1 AND event_id = $2`,
		groupID,
		eventID,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Str("event_id", eventID.String()).
			Msg("Failed to remove event from group")
		return fmt.Errorf("failed to remove event from group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Str("event_id", eventID.String()).
			Msg("Failed to get rows affected when removing event from group")
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrEventNotInGroup
	}

	return nil
}

// GetGroupEvents returns all events in a group
func (r *groupEventRepository) GetGroupEvents(ctx context.Context, groupID uuid.UUID) ([]*models.GroupEvent, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, group_id, event_id, added_by, added_at, status, is_hierarchical
		FROM group_events WHERE group_id = $1`,
		groupID,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Msg("Failed to get group events")
		return nil, fmt.Errorf("failed to get group events: %w", err)
	}
	defer rows.Close()

	var events []*models.GroupEvent
	for rows.Next() {
		var event models.GroupEvent

		err := rows.Scan(
			&event.ID,
			&event.GroupID,
			&event.EventID,
			&event.AddedBy,
			&event.AddedAt,
			&event.Status,
			&event.IsHierarchical,
		)

		if err != nil {
			r.log.Error().Err(err).
				Str("group_id", groupID.String()).
				Msg("Failed to scan group event row")
			return nil, fmt.Errorf("failed to scan group event row: %w", err)
		}

		events = append(events, &event)
	}

	if err = rows.Err(); err != nil {
		r.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Msg("Error iterating over group events")
		return nil, fmt.Errorf("error iterating over group events: %w", err)
	}

	return events, nil
}

// GetGroupEvent retrieves a group event by ID
func (r *groupEventRepository) GetGroupEvent(ctx context.Context, eventID uuid.UUID) (*models.GroupEvent, error) {
	var event models.GroupEvent

	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, group_id, event_id, added_by, added_at, status, is_hierarchical
		FROM group_events WHERE event_id = $1`,
		eventID,
	).Scan(
		&event.ID,
		&event.GroupID,
		&event.EventID,
		&event.AddedBy,
		&event.AddedAt,
		&event.Status,
		&event.IsHierarchical,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrGroupEventNotFound
		}

		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Msg("Failed to get group event")
		return nil, fmt.Errorf("failed to get group event: %w", err)
	}

	return &event, nil
}

// RemoveEventFromAllGroups removes an event from all groups
func (r *groupEventRepository) RemoveEventFromAllGroups(ctx context.Context, eventID uuid.UUID) error {
	_, err := r.db.ExecContext(
		ctx,
		`DELETE FROM group_events WHERE event_id = $1`,
		eventID,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Msg("Failed to remove event from all groups")
		return fmt.Errorf("failed to remove event from all groups: %w", err)
	}

	return nil
}

// AddEventStatus adds a new event status record
func (r *groupEventRepository) AddEventStatus(ctx context.Context, status *models.GroupEventStatus) error {
	if status.ID == uuid.Nil {
		status.ID = uuid.New()
	}

	now := time.Now().UTC()
	if status.CreatedAt.IsZero() {
		status.CreatedAt = now
	}
	status.UpdatedAt = now

	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO group_event_status (id, group_id, event_id, user_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		status.ID,
		status.GroupID,
		status.EventID,
		status.UserID,
		status.Status,
		status.CreatedAt,
		status.UpdatedAt,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("event_id", status.EventID.String()).
			Str("user_id", status.UserID.String()).
			Msg("Failed to add event status")
		return fmt.Errorf("failed to add event status: %w", err)
	}

	return nil
}

// AddEventStatusWithTx adds a new event status record within a transaction
func (r *groupEventRepository) AddEventStatusWithTx(ctx context.Context, tx *sql.Tx, status *models.GroupEventStatus) error {
	if status.ID == uuid.Nil {
		status.ID = uuid.New()
	}

	now := time.Now().UTC()
	if status.CreatedAt.IsZero() {
		status.CreatedAt = now
	}
	status.UpdatedAt = now

	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO group_event_status (id, group_id, event_id, user_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		status.ID,
		status.GroupID,
		status.EventID,
		status.UserID,
		status.Status,
		status.CreatedAt,
		status.UpdatedAt,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("event_id", status.EventID.String()).
			Str("user_id", status.UserID.String()).
			Msg("Failed to add event status in transaction")
		return fmt.Errorf("failed to add event status in transaction: %w", err)
	}

	return nil
}

// UpdateEventStatus updates the status of an event for a user
func (r *groupEventRepository) UpdateEventStatus(ctx context.Context, eventID, userID uuid.UUID, status models.EventStatus) error {
	if !status.IsValid() {
		return ErrInvalidEventStatus
	}

	now := time.Now().UTC()

	result, err := r.db.ExecContext(
		ctx,
		`UPDATE group_event_status 
		SET status = $1, updated_at = $2, responded_at = $3
		WHERE event_id = $4 AND user_id = $5`,
		status,
		now,
		now,
		eventID,
		userID,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Str("user_id", userID.String()).
			Str("status", string(status)).
			Msg("Failed to update event status")
		return fmt.Errorf("failed to update event status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to get rows affected when updating event status")
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrEventStatusNotFound
	}

	return nil
}

// GetEventStatus retrieves the status of an event for a specific user
func (r *groupEventRepository) GetEventStatus(ctx context.Context, eventID, userID uuid.UUID) (*models.GroupEventStatus, error) {
	var status models.GroupEventStatus

	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, group_id, event_id, user_id, status, responded_at, created_at, updated_at
		FROM group_event_status 
		WHERE event_id = $1 AND user_id = $2`,
		eventID,
		userID,
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
			return nil, ErrEventStatusNotFound
		}

		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to get event status")
		return nil, fmt.Errorf("failed to get event status: %w", err)
	}

	return &status, nil
}

// BatchCreateEventStatus creates multiple event statuses in a single transaction
func (r *groupEventRepository) BatchCreateEventStatus(ctx context.Context, tx *sql.Tx, statuses []*models.GroupEventStatus) error {
	if len(statuses) == 0 {
		return nil
	}

	query := `
		INSERT INTO group_event_status 
		(id, group_id, event_id, user_id, status, responded_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	var stmt *sql.Stmt
	var err error

	if tx != nil {
		stmt, err = tx.PrepareContext(ctx, query)
	} else {
		stmt, err = r.db.PrepareContext(ctx, query)
	}

	if err != nil {
		r.log.Error().Err(err).Msg("Failed to prepare batch insert")
		return fmt.Errorf("failed to prepare batch insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()

	for _, status := range statuses {
		if status.ID == uuid.Nil {
			status.ID = uuid.New()
		}
		status.CreatedAt = now
		status.UpdatedAt = now

		_, err := stmt.ExecContext(
			ctx,
			status.ID,
			status.GroupID,
			status.EventID,
			status.UserID,
			status.Status,
			status.RespondedAt,
			status.CreatedAt,
			status.UpdatedAt,
		)

		if err != nil {
			r.log.Error().Err(err).
				Str("event_id", status.EventID.String()).
				Str("user_id", status.UserID.String()).
				Msg("Failed to create event status in batch")
			return fmt.Errorf("failed to create event status for user %s: %w", status.UserID, err)
		}
	}

	return nil
}

// UpdateEventStatuses updates multiple event statuses in a single transaction
func (r *groupEventRepository) UpdateEventStatuses(ctx context.Context, tx *sql.Tx, statuses []*models.GroupEventStatus) error {
	if len(statuses) == 0 {
		return nil
	}

	query := `
		UPDATE group_event_status 
		SET status = $1, 
		    responded_at = CASE WHEN $2 != 'pending' THEN COALESCE($3, $4) ELSE NULL END,
		    updated_at = $5
		WHERE event_id = $6 AND user_id = $7
	`

	var stmt *sql.Stmt
	var err error

	if tx != nil {
		stmt, err = tx.PrepareContext(ctx, query)
	} else {
		stmt, err = r.db.PrepareContext(ctx, query)
	}

	if err != nil {
		r.log.Error().Err(err).Msg("Failed to prepare batch update")
		return fmt.Errorf("failed to prepare batch update: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()

	for _, status := range statuses {
		status.UpdatedAt = now
		var respondedAt *time.Time

		if status.Status != models.EventStatusPending && status.RespondedAt == nil {
			respondedAt = &now
			status.RespondedAt = respondedAt
		} else {
			respondedAt = status.RespondedAt
		}

		_, err := stmt.ExecContext(
			ctx,
			status.Status,
			status.Status,
			status.RespondedAt,
			now,
			now,
			status.EventID,
			status.UserID,
		)

		if err != nil {
			r.log.Error().Err(err).
				Str("event_id", status.EventID.String()).
				Str("user_id", status.UserID.String()).
				Msg("Failed to update event status in batch")
			return fmt.Errorf("failed to update event status for user %s: %w", status.UserID, err)
		}
	}

	return nil
}

// GetEventStatuses retrieves all statuses for an event
func (r *groupEventRepository) GetEventStatuses(ctx context.Context, eventID uuid.UUID) ([]*models.GroupEventStatus, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, group_id, event_id, user_id, status, responded_at, created_at, updated_at
		FROM group_event_status 
		WHERE event_id = $1`,
		eventID,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Msg("Failed to get event statuses")
		return nil, fmt.Errorf("failed to get event statuses: %w", err)
	}
	defer rows.Close()

	var statuses []*models.GroupEventStatus
	for rows.Next() {
		var status models.GroupEventStatus

		err := rows.Scan(
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
			r.log.Error().Err(err).
				Str("event_id", eventID.String()).
				Msg("Failed to scan event status row")
			return nil, fmt.Errorf("failed to scan event status row: %w", err)
		}

		statuses = append(statuses, &status)
	}

	if err = rows.Err(); err != nil {
		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Msg("Error iterating over event statuses")
		return nil, fmt.Errorf("error iterating over event statuses: %w", err)
	}

	return statuses, nil
}

// CreateInvitation creates a new group invitation
func (r *groupEventRepository) CreateInvitation(ctx context.Context, invitation *models.GroupInvitation) error {
	if invitation.ID == uuid.Nil {
		invitation.ID = uuid.New()
	}

	if invitation.CreatedAt.IsZero() {
		invitation.CreatedAt = time.Now().UTC()
	}

	if invitation.Status == "" {
		invitation.Status = string(models.EventStatusPending)
	}

	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO group_invitations (id, group_id, user_id, invited_by, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		invitation.ID,
		invitation.GroupID,
		invitation.UserID,
		invitation.InvitedBy,
		invitation.Status,
		invitation.CreatedAt,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("group_id", invitation.GroupID.String()).
			Str("user_id", invitation.UserID.String()).
			Msg("Failed to create group invitation")
		return fmt.Errorf("failed to create group invitation: %w", err)
	}

	return nil
}

// GetInvitationByID retrieves an invitation by its ID
func (r *groupEventRepository) GetInvitationByID(ctx context.Context, id uuid.UUID) (*models.GroupInvitation, error) {
	var invitation models.GroupInvitation

	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, group_id, user_id, invited_by, status, created_at, responded_at
		FROM group_invitations 
		WHERE id = $1`,
		id,
	).Scan(
		&invitation.ID,
		&invitation.GroupID,
		&invitation.UserID,
		&invitation.InvitedBy,
		&invitation.Status,
		&invitation.CreatedAt,
		&invitation.RespondedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvitationNotFound
		}

		r.log.Error().Err(err).
			Str("invitation_id", id.String()).
			Msg("Failed to get group invitation")
		return nil, fmt.Errorf("failed to get group invitation: %w", err)
	}

	return &invitation, nil
}

// UpdateInvitation updates an invitation's status
func (r *groupEventRepository) UpdateInvitation(ctx context.Context, id uuid.UUID, status string) error {
	var respondedAt interface{}
	if status != string(models.EventStatusPending) {
		respondedAt = time.Now().UTC()
	}

	result, err := r.db.ExecContext(
		ctx,
		`UPDATE group_invitations 
		SET status = $1, responded_at = $2
		WHERE id = $3`,
		status,
		respondedAt,
		id,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("invitation_id", id.String()).
			Str("status", status).
			Msg("Failed to update group invitation")
		return fmt.Errorf("failed to update group invitation: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.log.Error().Err(err).
			Str("invitation_id", id.String()).
			Msg("Failed to get rows affected when updating group invitation")
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrInvitationNotFound
	}

	return nil
}

// GetUserInvitations returns all invitations for a user, optionally filtered by status
func (r *groupEventRepository) GetUserInvitations(ctx context.Context, userID uuid.UUID, status string) ([]*models.GroupInvitation, error) {
	var rows *sql.Rows
	var err error

	if status == "" {
		rows, err = r.db.QueryContext(
			ctx,
			`SELECT id, group_id, user_id, invited_by, status, created_at, responded_at
			FROM group_invitations 
			WHERE user_id = $1`,
			userID,
		)
	} else {
		rows, err = r.db.QueryContext(
			ctx,
			`SELECT id, group_id, user_id, invited_by, status, created_at, responded_at
			FROM group_invitations 
			WHERE user_id = $1 AND status = $2`,
			userID,
			status,
		)
	}

	if err != nil {
		r.log.Error().Err(err).
			Str("user_id", userID.String()).
			Str("status", status).
			Msg("Failed to get user invitations")
		return nil, fmt.Errorf("failed to get user invitations: %w", err)
	}
	defer rows.Close()

	var invitations []*models.GroupInvitation
	for rows.Next() {
		var invitation models.GroupInvitation

		err := rows.Scan(
			&invitation.ID,
			&invitation.GroupID,
			&invitation.UserID,
			&invitation.InvitedBy,
			&invitation.Status,
			&invitation.CreatedAt,
			&invitation.RespondedAt,
		)

		if err != nil {
			r.log.Error().Err(err).
				Str("user_id", userID.String()).
				Msg("Failed to scan user invitation row")
			return nil, fmt.Errorf("failed to scan user invitation row: %w", err)
		}

		invitations = append(invitations, &invitation)
	}

	if err = rows.Err(); err != nil {
		r.log.Error().Err(err).
			Str("user_id", userID.String()).
			Msg("Error iterating over user invitations")
		return nil, fmt.Errorf("error iterating over user invitations: %w", err)
	}

	return invitations, nil
}

// GetEventStatusesByGroup gets all statuses for an event in a specific group
func (r *groupEventRepository) GetEventStatusesByGroup(ctx context.Context, groupID, eventID uuid.UUID) ([]*models.GroupEventStatus, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, group_id, event_id, user_id, status, responded_at, created_at, updated_at
		FROM group_event_status 
		WHERE group_id = $1 AND event_id = $2`,
		groupID,
		eventID,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Str("event_id", eventID.String()).
			Msg("Failed to get event statuses by group")
		return nil, fmt.Errorf("failed to get event statuses by group: %w", err)
	}
	defer rows.Close()

	var statuses []*models.GroupEventStatus
	for rows.Next() {
		var status models.GroupEventStatus
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
			r.log.Error().Err(err).
				Str("group_id", groupID.String()).
				Str("event_id", eventID.String()).
				Msg("Failed to scan event status row")
			return nil, fmt.Errorf("failed to scan event status row: %w", err)
		}
		statuses = append(statuses, &status)
	}

	if err = rows.Err(); err != nil {
		r.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Str("event_id", eventID.String()).
			Msg("Error iterating over event status rows")
		return nil, fmt.Errorf("error iterating over event status rows: %w", err)
	}

	return statuses, nil
}

// GetEventStatusCounts returns the count of each status for an event
func (r *groupEventRepository) GetEventStatusCounts(ctx context.Context, eventID uuid.UUID) (map[models.EventStatus]int, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT status, COUNT(*) 
		FROM group_event_status 
		WHERE event_id = $1 
		GROUP BY status`,
		eventID,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Msg("Failed to get event status counts")
		return nil, fmt.Errorf("failed to get event status counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[models.EventStatus]int)
	for rows.Next() {
		var status models.EventStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			r.log.Error().Err(err).
				Str("event_id", eventID.String()).
				Msg("Failed to scan event status count row")
			return nil, fmt.Errorf("failed to scan event status count row: %w", err)
		}
		counts[status] = count
	}

	if err = rows.Err(); err != nil {
		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Msg("Error iterating over event status count rows")
		return nil, fmt.Errorf("error iterating over event status count rows: %w", err)
	}

	return counts, nil
}

// HasResponded checks if a user has responded to an event
func (r *groupEventRepository) HasResponded(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) 
		FROM group_event_status 
		WHERE event_id = $1 AND user_id = $2 AND status != 'pending'`,
		eventID,
		userID,
	).Scan(&count)

	if err != nil {
		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to check if user has responded to event")
		return false, fmt.Errorf("failed to check if user has responded to event: %w", err)
	}

	return count > 0, nil
}

// HasAllMembersAccepted checks if all members of a non-hierarchical group have accepted an event
func (r *groupEventRepository) HasAllMembersAccepted(ctx context.Context, groupID, eventID uuid.UUID) (bool, error) {
	// First, check if the event is hierarchical
	var isHierarchical bool
	err := r.db.QueryRowContext(
		ctx,
		`SELECT is_hierarchical FROM group_events WHERE event_id = $1`,
		eventID,
	).Scan(&isHierarchical)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Msg("Failed to check if event is hierarchical")
		return false, fmt.Errorf("failed to check if event is hierarchical: %w", err)
	}

	// If the event is hierarchical, we don't need to check all members
	if isHierarchical {
		return true, nil
	}

	// For non-hierarchical events, check if all group members have accepted
	var totalMembers, acceptedMembers int

	// Get total number of members in the group
	err = r.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) 
		FROM group_members 
		WHERE group_id = $1`,
		groupID,
	).Scan(&totalMembers)

	if err != nil {
		r.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Msg("Failed to get total group members count")
		return false, fmt.Errorf("failed to get total group members count: %w", err)
	}

	// If there are no members, return false
	if totalMembers == 0 {
		return false, nil
	}

	// Get number of members who have accepted the event
	err = r.db.QueryRowContext(
		ctx,
		`SELECT COUNT(DISTINCT gs.user_id) 
		FROM group_event_status gs
		JOIN group_members gm ON gs.user_id = gm.user_id AND gs.group_id = gm.group_id
		WHERE gs.event_id = $1 AND gs.group_id = $2 AND gs.status = 'accepted'`,
		eventID,
		groupID,
	).Scan(&acceptedMembers)

	if err != nil {
		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Str("group_id", groupID.String()).
			Msg("Failed to get accepted members count")
		return false, fmt.Errorf("failed to get accepted members count: %w", err)
	}

	return acceptedMembers == totalMembers, nil
}

// DeleteEventStatus deletes an event status for a specific user and event
func (r *groupEventRepository) DeleteEventStatus(ctx context.Context, tx *sql.Tx, eventID, userID uuid.UUID) error {
	query := `DELETE FROM group_event_status WHERE event_id = $1 AND user_id = $2`
	var err error

	if tx != nil {
		_, err = tx.ExecContext(ctx, query, eventID, userID)
	} else {
		_, err = r.db.ExecContext(ctx, query, eventID, userID)
	}

	if err != nil {
		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to delete event status")
		return fmt.Errorf("failed to delete event status: %w", err)
	}

	return nil
}

// DeleteEventStatuses deletes all statuses for an event
func (r *groupEventRepository) DeleteEventStatuses(ctx context.Context, tx *sql.Tx, eventID uuid.UUID) error {
	query := `DELETE FROM group_event_status WHERE event_id = $1`
	var err error

	if tx != nil {
		_, err = tx.ExecContext(ctx, query, eventID)
	} else {
		_, err = r.db.ExecContext(ctx, query, eventID)
	}

	if err != nil {
		r.log.Error().Err(err).
			Str("event_id", eventID.String()).
			Msg("Failed to delete event statuses")
		return fmt.Errorf("failed to delete event statuses: %w", err)
	}

	return nil
}

// DeleteEventStatusesByGroup deletes all statuses for an event in a specific group
func (r *groupEventRepository) DeleteEventStatusesByGroup(ctx context.Context, tx *sql.Tx, groupID, eventID uuid.UUID) error {
	query := `DELETE FROM group_event_status WHERE group_id = $1 AND event_id = $2`
	var err error

	if tx != nil {
		_, err = tx.ExecContext(ctx, query, groupID, eventID)
	} else {
		_, err = r.db.ExecContext(ctx, query, groupID, eventID)
	}

	if err != nil {
		r.log.Error().Err(err).
			Str("group_id", groupID.String()).
			Str("event_id", eventID.String()).
			Msg("Failed to delete event statuses by group")
		return fmt.Errorf("failed to delete event statuses by group: %w", err)
	}

	return nil
}

// DeleteUserInvitations deletes all invitations for a user
func (r *groupEventRepository) DeleteUserInvitations(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(
		ctx,
		`DELETE FROM group_invitations WHERE user_id = $1`,
		userID,
	)

	if err != nil {
		r.log.Error().Err(err).
			Str("user_id", userID.String()).
			Msg("Failed to delete user invitations")
		return fmt.Errorf("failed to delete user invitations: %w", err)
	}

	return nil
}
