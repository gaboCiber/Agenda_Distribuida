package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/google/uuid"
)

type eventStatusRepository struct {
	db *sql.DB
}

// NewEventStatusRepository creates a new event status repository
func NewEventStatusRepository(db *sql.DB) models.EventStatusRepository {
	return &eventStatusRepository{db: db}
}

// execQuery executes a query within a transaction if one is provided, otherwise uses the database directly
func (r *eventStatusRepository) execQuery(tx *sql.Tx, query string, args ...interface{}) (sql.Result, error) {
	if tx != nil {
		return tx.Exec(query, args...)
	}
	return r.db.Exec(query, args...)
}

// queryRow executes a query that returns at most one row
func (r *eventStatusRepository) queryRow(tx *sql.Tx, query string, args ...interface{}) *sql.Row {
	if tx != nil {
		return tx.QueryRow(query, args...)
	}
	return r.db.QueryRow(query, args...)
}

// query executes a query that returns multiple rows
func (r *eventStatusRepository) query(tx *sql.Tx, query string, args ...interface{}) (*sql.Rows, error) {
	if tx != nil {
		return tx.Query(query, args...)
	}
	return r.db.Query(query, args...)
}

// prepareStmt prepares a statement either in a transaction or on the database
func (r *eventStatusRepository) prepareStmt(tx *sql.Tx, query string) (*sql.Stmt, error) {
	if tx != nil {
		return tx.Prepare(query)
	}
	return r.db.Prepare(query)
}

// CreateEventStatus creates a new event status
func (r *eventStatusRepository) CreateEventStatus(tx *sql.Tx, status *models.GroupEventStatus) error {
	query := `
		INSERT INTO group_event_status 
		(id, group_id, event_id, user_id, status, responded_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC()
	status.CreatedAt = now
	status.UpdatedAt = now

	_, err := r.execQuery(tx, query,
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
		return fmt.Errorf("failed to create event status: %w", err)
	}

	return nil
}

// BatchCreateEventStatus creates multiple event statuses in a single transaction
func (r *eventStatusRepository) BatchCreateEventStatus(tx *sql.Tx, statuses []*models.GroupEventStatus) error {
	if len(statuses) == 0 {
		return nil
	}

	query := `
		INSERT INTO group_event_status 
		(id, group_id, event_id, user_id, status, responded_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	stmt, err := r.prepareStmt(tx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare batch insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()

	for _, status := range statuses {
		status.ID = uuid.New().String()
		status.CreatedAt = now
		status.UpdatedAt = now

		_, err := stmt.Exec(
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
			return fmt.Errorf("failed to create event status for user %s: %w", status.UserID, err)
		}
	}

	return nil
}

// UpdateEventStatus updates an existing event status
func (r *eventStatusRepository) UpdateEventStatus(tx *sql.Tx, status *models.GroupEventStatus) error {
	query := `
		UPDATE group_event_status 
		SET status = ?, 
		    responded_at = CASE WHEN ? != 'pending' THEN COALESCE(responded_at, ?) ELSE NULL END,
		    updated_at = ?
		WHERE event_id = ? AND user_id = ?
	`

	now := time.Now().UTC()
	status.UpdatedAt = now

	var respondedAt *time.Time
	if status.Status != models.EventStatusPending && status.RespondedAt == nil {
		respondedAt = &now
		status.RespondedAt = respondedAt
	} else {
		respondedAt = status.RespondedAt
	}

	_, err := r.execQuery(tx, query,
		status.Status,
		status.Status,
		respondedAt,
		status.UpdatedAt,
		status.EventID,
		status.UserID,
	)

	if err != nil {
		return fmt.Errorf("failed to update event status: %w", err)
	}

	return nil
}

// UpdateEventStatuses updates multiple event statuses in a single transaction
func (r *eventStatusRepository) UpdateEventStatuses(tx *sql.Tx, statuses []*models.GroupEventStatus) error {
	if len(statuses) == 0 {
		return nil
	}

	query := `
		UPDATE group_event_status 
		SET status = ?, 
		    responded_at = CASE WHEN ? != 'pending' THEN COALESCE(responded_at, ?) ELSE NULL END,
		    updated_at = ?
		WHERE event_id = ? AND user_id = ?
	`

	stmt, err := r.prepareStmt(tx, query)
	if err != nil {
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

		_, err := stmt.Exec(
			status.Status,
			status.Status,
			respondedAt,
			status.UpdatedAt,
			status.EventID,
			status.UserID,
		)

		if err != nil {
			return fmt.Errorf("failed to update status for user %s: %w", status.UserID, err)
		}
	}

	return nil
}

// GetEventStatuses gets all statuses for an event
func (r *eventStatusRepository) GetEventStatuses(tx *sql.Tx, eventID string) ([]*models.GroupEventStatus, error) {
	query := `
		SELECT id, group_id, event_id, user_id, status, responded_at, created_at, updated_at
		FROM group_event_status
		WHERE event_id = ?
	`

	rows, err := r.query(tx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to query event statuses: %w", err)
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
			return nil, fmt.Errorf("failed to scan event status: %w", err)
		}
		statuses = append(statuses, &status)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating statuses: %w", err)
	}

	return statuses, nil
}

// GetEventStatusesByGroup gets all statuses for an event in a specific group
func (r *eventStatusRepository) GetEventStatusesByGroup(tx *sql.Tx, groupID, eventID string) ([]*models.GroupEventStatus, error) {
	query := `
		SELECT id, group_id, event_id, user_id, status, responded_at, created_at, updated_at
		FROM group_event_status
		WHERE group_id = ? AND event_id = ?
	`

	rows, err := r.query(tx, query, groupID, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to query event statuses by group: %w", err)
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
			return nil, fmt.Errorf("failed to scan event status: %w", err)
		}
		statuses = append(statuses, &status)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating statuses: %w", err)
	}

	return statuses, nil
}

// GetEventStatusCounts returns the count of each status for an event
func (r *eventStatusRepository) GetEventStatusCounts(tx *sql.Tx, eventID string) (map[models.EventStatus]int, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM group_event_status
		WHERE event_id = ?
		GROUP BY status
	`

	rows, err := r.query(tx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to query event status counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[models.EventStatus]int)
	for rows.Next() {
		var statusStr string
		var count int
		if err := rows.Scan(&statusStr, &count); err != nil {
			return nil, fmt.Errorf("failed to scan status count: %w", err)
		}
		status := models.EventStatus(statusStr)
		if status.IsValid() {
			counts[status] = count
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating status counts: %w", err)
	}

	return counts, nil
}

// HasResponded checks if a user has responded to an event
func (r *eventStatusRepository) HasResponded(tx *sql.Tx, eventID, userID string) (bool, error) {
	query := `
		SELECT COUNT(*) > 0
		FROM group_event_status
		WHERE event_id = ? AND user_id = ? AND status != 'pending'
	`

	var hasResponded bool
	err := r.queryRow(tx, query, eventID, userID).Scan(&hasResponded)
	if err != nil {
		return false, fmt.Errorf("failed to check response status: %w", err)
	}

	return hasResponded, nil
}

// GetEventStatus gets the status for a specific user and event
func (r *eventStatusRepository) GetEventStatus(tx *sql.Tx, eventID, userID string) (*models.GroupEventStatus, error) {
	query := `
		SELECT id, group_id, event_id, user_id, status, responded_at, created_at, updated_at
		FROM group_event_status
		WHERE event_id = ? AND user_id = ?
	`

	var status models.GroupEventStatus
	err := r.queryRow(tx, query, eventID, userID).Scan(
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
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get event status: %w", err)
	}

	return &status, nil
}

// DeleteEventStatus deletes an event status for a specific user and event
func (r *eventStatusRepository) DeleteEventStatus(tx *sql.Tx, eventID, userID string) error {
	query := `DELETE FROM group_event_status WHERE event_id = ? AND user_id = ?`
	_, err := r.execQuery(tx, query, eventID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete event status: %w", err)
	}
	return nil
}

// DeleteEventStatuses deletes all statuses for an event
func (r *eventStatusRepository) DeleteEventStatuses(tx *sql.Tx, eventID string) error {
	query := `DELETE FROM group_event_status WHERE event_id = ?`
	_, err := r.execQuery(tx, query, eventID)
	if err != nil {
		return fmt.Errorf("failed to delete event statuses: %w", err)
	}
	return nil
}

// DeleteEventStatusesByGroup deletes all statuses for an event in a specific group
func (r *eventStatusRepository) DeleteEventStatusesByGroup(tx *sql.Tx, groupID, eventID string) error {
	query := `DELETE FROM group_event_status WHERE group_id = ? AND event_id = ?`
	_, err := r.execQuery(tx, query, groupID, eventID)
	if err != nil {
		return fmt.Errorf("failed to delete event statuses by group: %w", err)
	}
	return nil
}

// HasAllMembersAccepted checks if all members of a non-hierarchical group have accepted an event
func (r *eventStatusRepository) HasAllMembersAccepted(tx *sql.Tx, groupID, eventID string) (bool, error) {
	// First, get the total number of members in the group
	memberQuery := `
		SELECT COUNT(*) 
		FROM group_members 
		WHERE group_id = ?
	`

	var totalMembers int
	err := r.queryRow(tx, memberQuery, groupID).Scan(&totalMembers)
	if err != nil {
		return false, fmt.Errorf("failed to count group members: %w", err)
	}

	// If there are no members, return false
	if totalMembers == 0 {
		return false, nil
	}

	// Count how many members have responded to the event
	respondedQuery := `
		SELECT COUNT(*) 
		FROM group_event_status 
		WHERE group_id = ? AND event_id = ? AND status IS NOT NULL
	`

	var respondedCount int
	err = r.queryRow(tx, respondedQuery, groupID, eventID).Scan(&respondedCount)
	if err != nil {
		return false, fmt.Errorf("failed to count responded statuses: %w", err)
	}

	// If not all members have responded, we can't say all have accepted
	if respondedCount < totalMembers {
		return false, nil
	}

	// Count how many members have accepted the event
	statusQuery := `
		SELECT COUNT(*) 
		FROM group_event_status 
		WHERE group_id = ? AND event_id = ? AND status = ?
	`

	var acceptedCount int
	err = r.queryRow(tx, statusQuery, groupID, eventID, models.EventStatusAccepted).Scan(&acceptedCount)
	if err != nil {
		return false, fmt.Errorf("failed to count accepted statuses: %w", err)
	}

	// All members have accepted if the counts match
	return acceptedCount == totalMembers, nil
}
