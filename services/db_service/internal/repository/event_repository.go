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

var (
	ErrEventNotFound = errors.New("event not found")
)

// EventRepository defines the interface for event data access
// with CRUD methods and conflict checking.
type EventRepository interface {
	Create(ctx context.Context, event *models.Event) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Event, error)
	Update(ctx context.Context, id uuid.UUID, updateReq *models.EventRequest) (*models.Event, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.Event, error)
	CheckTimeConflict(ctx context.Context, userID uuid.UUID, startTime, endTime time.Time, excludeEventID *uuid.UUID) (bool, error)
}

type eventRepository struct {
	db  *sql.DB
	log zerolog.Logger
}

// NewEventRepository creates a new event repository
func NewEventRepository(db *sql.DB, log zerolog.Logger) EventRepository {
	return &eventRepository{
		db:  db,
		log: log,
	}
}

// Create inserts a new event into the database
func (r *eventRepository) Create(ctx context.Context, event *models.Event) error {
	query := `
		INSERT INTO events (id, title, description, start_time, end_time, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	now := time.Now()
	event.CreatedAt = now
	event.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, query,
		event.ID,
		event.Title,
		event.Description,
		event.StartTime,
		event.EndTime,
		event.UserID,
		event.CreatedAt,
		event.UpdatedAt,
	)

	if err != nil {
		r.log.Error().Err(err).Str("event_id", event.ID.String()).Msg("Failed to create event")
		return err
	}

	return nil
}

// GetByID retrieves an event by its ID
func (r *eventRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Event, error) {
	query := `
		SELECT id, title, description, start_time, end_time, user_id, created_at, updated_at
		FROM events
		WHERE id = $1
	`

	var event models.Event
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.Title,
		&event.Description,
		&event.StartTime,
		&event.EndTime,
		&event.UserID,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEventNotFound
		}
		r.log.Error().Err(err).Str("event_id", id.String()).Msg("Failed to get event by ID")
		return nil, err
	}

	return &event, nil
}

// Update modifies an existing event
func (r *eventRepository) Update(ctx context.Context, id uuid.UUID, updateReq *models.EventRequest) (*models.Event, error) {
	// Check for time conflicts, excluding the current event
	hasConflict, err := r.CheckTimeConflict(ctx, updateReq.UserID, updateReq.StartTime, updateReq.EndTime, &id)
	if err != nil {
		r.log.Error().
			Err(err).
			Str("event_id", id.String()).
			Str("user_id", updateReq.UserID.String()).
			Msg("Failed to check time conflict during update")
		return nil, fmt.Errorf("failed to check time conflict: %w", err)
	}

	if hasConflict {
		return nil, fmt.Errorf("time conflict detected for the specified time range")
	}

	query := `
		UPDATE events
		SET title = $1, description = $2, start_time = $3, end_time = $4, user_id = $5, updated_at = $6
		WHERE id = $7
		RETURNING id, title, description, start_time, end_time, user_id, created_at, updated_at
	`

	var event models.Event
	now := time.Now()
	err = r.db.QueryRowContext(ctx, query,
		updateReq.Title,
		updateReq.Description,
		updateReq.StartTime,
		updateReq.EndTime,
		updateReq.UserID,
		now,
		id,
	).Scan(
		&event.ID,
		&event.Title,
		&event.Description,
		&event.StartTime,
		&event.EndTime,
		&event.UserID,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err != nil {
		r.log.Error().Err(err).Str("event_id", id.String()).Msg("Failed to update event")
		return nil, fmt.Errorf("failed to update event: %w", err)
	}

	return &event, nil
}

// Delete removes an event from the database
func (r *eventRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM events WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error().Err(err).Str("event_id", id.String()).Msg("Failed to delete event")
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to get rows affected for event delete")
		return err
	}

	if rowsAffected == 0 {
		return ErrEventNotFound
	}

	return nil
}

// ListByUser lists events for a given user with pagination (personal events and accepted group events)
func (r *eventRepository) ListByUser(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.Event, error) {
	query := `
		SELECT DISTINCT e.id, e.title, e.description, e.start_time, e.end_time, e.user_id, e.created_at, e.updated_at
		FROM events e
		LEFT JOIN group_event_status ges ON e.id = ges.event_id AND ges.user_id = $1
		LEFT JOIN group_events ge ON ges.event_id = ge.event_id AND ges.group_id = ge.group_id
		WHERE e.user_id = $1 OR (ges.status = 'accepted' AND ge.status = 'accepted')
		ORDER BY e.start_time DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		r.log.Error().Err(err).Str("user_id", userID.String()).Msg("Failed to list events")
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		var event models.Event
		if err := rows.Scan(
			&event.ID,
			&event.Title,
			&event.Description,
			&event.StartTime,
			&event.EndTime,
			&event.UserID,
			&event.CreatedAt,
			&event.UpdatedAt,
		); err != nil {
			r.log.Error().Err(err).Msg("Failed to scan event")
			return nil, err
		}
		events = append(events, &event)
	}

	return events, nil
}

// CheckTimeConflict checks if there is a time conflict for a user's events
// excluding the event with the specified ID (if provided)
func (r *eventRepository) CheckTimeConflict(ctx context.Context, userID uuid.UUID, startTime, endTime time.Time, excludeEventID *uuid.UUID) (bool, error) {
	// Check for any of these overlap conditions:
	// 1. New event starts during an existing event
	// 2. New event ends during an existing event
	// 3. New event completely contains an existing event
	// 4. New event is completely contained within an existing event
	query := `
		SELECT EXISTS(
			SELECT 1 FROM events
			WHERE user_id = $1
			AND (id != $2)
			AND (
				-- New event starts during an existing event
				(start_time <= $3 AND end_time > $3) OR
				-- New event ends during an existing event
				(start_time < $4 AND end_time >= $4) OR
				-- New event completely contains an existing event
				(start_time >= $3 AND end_time <= $4) OR
				-- New event is completely within an existing event
				(start_time <= $3 AND end_time >= $4)
			)
		)
	`

	var exists bool
	var err error

	// If excludeEventID is nil, we don't need to exclude any event
	if excludeEventID == nil {
		// Use a different query that doesn't check for excludeEventID
		query = `
			SELECT EXISTS(
				SELECT 1 FROM events
				WHERE user_id = $1
				AND (
					-- New event starts during an existing event
					(start_time <= $2 AND end_time > $2) OR
					-- New event ends during an existing event
					(start_time < $3 AND end_time >= $3) OR
					-- New event completely contains an existing event
					(start_time >= $2 AND end_time <= $3) OR
					-- New event is completely within an existing event
					(start_time <= $2 AND end_time >= $3)
				)
			)
		`
		err = r.db.QueryRowContext(ctx, query, userID, startTime, endTime).Scan(&exists)
	} else {
		err = r.db.QueryRowContext(ctx, query, userID, excludeEventID, startTime, endTime).Scan(&exists)
	}

	if err != nil {
		r.log.Error().
			Err(err).
			Str("user_id", userID.String()).
			Str("exclude_event_id", func() string {
				if excludeEventID != nil {
					return excludeEventID.String()
				}
				return "none"
			}()).
			Str("query", query).
			Msg("Failed to check time conflict")
		return false, fmt.Errorf("failed to check time conflict: %w", err)
	}

	return exists, nil
}
