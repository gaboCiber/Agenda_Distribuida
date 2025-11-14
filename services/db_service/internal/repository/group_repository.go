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

// GroupRepository defines the interface for group data access
type GroupRepository interface {
	Create(ctx context.Context, group *models.Group) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Group, error)
	Update(ctx context.Context, group *models.Group) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.Group, error)
	AddMember(ctx context.Context, member *models.GroupMember) error
	GetMembers(ctx context.Context, groupID uuid.UUID) ([]*models.GroupMember, error)
	RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error
	IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error)
	IsAdmin(ctx context.Context, groupID, userID uuid.UUID) (bool, error)
}

type groupRepository struct {
	db  *sql.DB
	log zerolog.Logger
}

// NewGroupRepository creates a new group repository
func NewGroupRepository(db *sql.DB, log zerolog.Logger) GroupRepository {
	return &groupRepository{
		db:  db,
		log: log,
	}
}

// Create creates a new group and adds the creator as an admin
func (r *groupRepository) Create(ctx context.Context, group *models.Group) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to begin transaction")
		return err
	}

	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			r.log.Error().Err(err).Msg("Failed to rollback transaction")
		}
	}()

	// Generate new UUID for the group
	group.ID = uuid.New()
	group.CreatedAt = time.Now().UTC()
	group.UpdatedAt = group.CreatedAt

	// Insert the group
	_, err = tx.ExecContext(ctx, `
		INSERT INTO groups (id, name, description, created_by, is_hierarchical, parent_group_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		group.ID,
		group.Name,
		group.Description,
		group.CreatedBy,
		group.IsHierarchical,
		group.ParentGroupID,
		group.CreatedAt,
		group.UpdatedAt,
	)

	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", group.ID.String()).
			Msg("Failed to create group")
		return fmt.Errorf("failed to create group: %w", err)
	}

	// Add creator as admin
	memberID := uuid.New()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO group_members (id, group_id, user_id, role, is_inherited, joined_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		memberID,
		group.ID,
		group.CreatedBy,
		"admin",
		false,
		time.Now().UTC(),
	)

	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", group.ID.String()).
			Str("user_id", group.CreatedBy.String()).
			Msg("Failed to add creator as admin")
		return fmt.Errorf("failed to add creator as admin: %w", err)
	}

	if err := tx.Commit(); err != nil {
		r.log.Error().Err(err).Msg("Failed to commit transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetByID retrieves a group by its ID
func (r *groupRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Group, error) {
	query := `
		SELECT id, name, description, created_by, is_hierarchical, parent_group_id, created_at, updated_at
		FROM groups
		WHERE id = $1
	`

	var group models.Group
	var parentGroupID *uuid.UUID

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&group.ID,
		&group.Name,
		&group.Description,
		&group.CreatedBy,
		&group.IsHierarchical,
		&parentGroupID,
		&group.CreatedAt,
		&group.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.log.Error().
			Err(err).
			Str("group_id", id.String()).
			Msg("Failed to get group by ID")
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	group.ParentGroupID = parentGroupID
	return &group, nil
}

// Update updates an existing group
func (r *groupRepository) Update(ctx context.Context, group *models.Group) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to begin transaction")
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			r.log.Error().Err(err).Msg("Failed to rollback transaction")
		}
	}()

	group.UpdatedAt = time.Now().UTC()

	// Update the group
	result, err := tx.ExecContext(ctx, `
		UPDATE groups
		SET name = $1, 
			description = $2, 
			is_hierarchical = $3, 
			parent_group_id = $4, 
			updated_at = $5
		WHERE id = $6
	`,
		group.Name,
		group.Description,
		group.IsHierarchical,
		group.ParentGroupID,
		group.UpdatedAt,
		group.ID,
	)

	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", group.ID.String()).
			Msg("Failed to update group")
		return fmt.Errorf("failed to update group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", group.ID.String()).
			Msg("Failed to get rows affected")
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	// Check for circular references if this is a hierarchical group with a parent
	if group.IsHierarchical && group.ParentGroupID != nil {
		var isCircular bool
		err = tx.QueryRowContext(ctx, `
			WITH RECURSIVE group_hierarchy AS (
				SELECT id, parent_group_id, 1 as level
				FROM groups
				WHERE id = $1
				UNION ALL
				SELECT g.id, g.parent_group_id, h.level + 1
				FROM groups g
				JOIN group_hierarchy h ON g.id = h.parent_group_id
				WHERE h.level < 10  -- Prevent infinite recursion
			)
			SELECT EXISTS (SELECT 1 FROM group_hierarchy WHERE id = $2)
		`, *group.ParentGroupID, group.ID).Scan(&isCircular)

		if err != nil {
			r.log.Error().
				Err(err).
				Str("group_id", group.ID.String()).
				Str("parent_group_id", group.ParentGroupID.String()).
				Msg("Failed to check for circular references")
			return fmt.Errorf("failed to check for circular references: %w", err)
		}

		if isCircular {
			return fmt.Errorf("circular reference detected in group hierarchy")
		}

		// Ensure parent exists and is hierarchical
		var parent models.Group
		err = tx.QueryRowContext(ctx, `
			SELECT id, is_hierarchical 
			FROM groups 
			WHERE id = $1
		`, *group.ParentGroupID).Scan(&parent.ID, &parent.IsHierarchical)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("parent group %s not found", group.ParentGroupID.String())
			}
			r.log.Error().
				Err(err).
				Str("parent_group_id", group.ParentGroupID.String()).
				Msg("Failed to get parent group")
			return fmt.Errorf("failed to get parent group: %w", err)
		}

		if !parent.IsHierarchical {
			return fmt.Errorf("parent group %s must be hierarchical", group.ParentGroupID.String())
		}
	}

	if err := tx.Commit(); err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", group.ID.String()).
			Msg("Failed to commit transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Delete deletes a group and all its associated data
func (r *groupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to begin transaction")
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			r.log.Error().Err(err).Msg("Failed to rollback transaction")
		}
	}()

	// Delete group events
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_events WHERE group_id = $1`, id); err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", id.String()).
			Msg("Failed to delete group events")
		return fmt.Errorf("failed to delete group events: %w", err)
	}

	// Delete group invitations
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_invitations WHERE group_id = $1`, id); err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", id.String()).
			Msg("Failed to delete group invitations")
		return fmt.Errorf("failed to delete group invitations: %w", err)
	}

	// Delete group members
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = $1`, id); err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", id.String()).
			Msg("Failed to delete group members")
		return fmt.Errorf("failed to delete group members: %w", err)
	}

	// Finally, delete the group
	result, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id = $1`, id)
	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", id.String()).
			Msg("Failed to delete group")
		return fmt.Errorf("failed to delete group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", id.String()).
			Msg("Failed to get rows affected")
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if err := tx.Commit(); err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", id.String()).
			Msg("Failed to commit transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ListByUser returns all groups a user is a member of
func (r *groupRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.Group, error) {
	query := `
		SELECT g.id, g.name, g.description, g.created_by, g.is_hierarchical, g.parent_group_id, g.created_at, g.updated_at
		FROM groups g
		JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = $1
		ORDER BY g.updated_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		r.log.Error().
			Err(err).
			Str("user_id", userID.String()).
			Msg("Failed to query user groups")
		return nil, fmt.Errorf("failed to query user groups: %w", err)
	}
	defer rows.Close()

	var groups []*models.Group
	for rows.Next() {
		var group models.Group
		var parentGroupID *uuid.UUID

		err := rows.Scan(
			&group.ID,
			&group.Name,
			&group.Description,
			&group.CreatedBy,
			&group.IsHierarchical,
			&parentGroupID,
			&group.CreatedAt,
			&group.UpdatedAt,
		)
		if err != nil {
			r.log.Error().
				Err(err).
				Str("user_id", userID.String()).
				Msg("Failed to scan group")
			return nil, fmt.Errorf("failed to scan group: %w", err)
		}

		group.ParentGroupID = parentGroupID
		groups = append(groups, &group)
	}

	if err = rows.Err(); err != nil {
		r.log.Error().
			Err(err).
			Str("user_id", userID.String()).
			Msg("Error iterating over groups")
		return nil, fmt.Errorf("error iterating over groups: %w", err)
	}

	return groups, nil
}

// AddMember adds a user to a group with the specified role
func (r *groupRepository) AddMember(ctx context.Context, member *models.GroupMember) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to begin transaction")
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			r.log.Error().Err(err).Msg("Failed to rollback transaction")
		}
	}()

	// Check if user is already a direct member (not inherited)
	var existingMemberID uuid.UUID
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM group_members 
		WHERE group_id = $1 AND user_id = $2 AND is_inherited = $3
	`, member.GroupID, member.UserID, member.IsInherited).Scan(&existingMemberID)

	if err == nil {
		r.log.Debug().
			Str("group_id", member.GroupID.String()).
			Str("user_id", member.UserID.String()).
			Bool("is_inherited", member.IsInherited).
			Msg("User is already a member of the group with the same inheritance status")
		return fmt.Errorf("user %s is already a member of group %s with the same inheritance status",
			member.UserID, member.GroupID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		r.log.Error().
			Err(err).
			Str("group_id", member.GroupID.String()).
			Str("user_id", member.UserID.String()).
			Msg("Failed to check for existing group membership")
		return fmt.Errorf("failed to check for existing group membership: %w", err)
	}

	// Generate new UUID for the member if not provided
	if member.ID == uuid.Nil {
		member.ID = uuid.New()
	}

	// Set joined at time if not provided
	if member.JoinedAt.IsZero() {
		member.JoinedAt = time.Now().UTC()
	}

	// Add the member
	_, err = tx.ExecContext(ctx, `
		INSERT INTO group_members (id, group_id, user_id, role, is_inherited, joined_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		member.ID,
		member.GroupID,
		member.UserID,
		member.Role,
		member.IsInherited,
		member.JoinedAt,
	)

	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", member.GroupID.String()).
			Str("user_id", member.UserID.String()).
			Msg("Failed to add group member")
		return fmt.Errorf("failed to add group member: %w", err)
	}

	// If this is a hierarchical group and not an inherited member,
	// add the member to all child groups as inherited
	if !member.IsInherited {
		// First, check if the group is hierarchical
		var isHierarchical bool
		err = tx.QueryRow(`
			SELECT is_hierarchical FROM groups WHERE id = ?
		`, member.GroupID).Scan(&isHierarchical)

		if err != nil {
			tx.Rollback()
			return err
		}

		if isHierarchical {
			// Add the member to all child groups as inherited
			// Get all child groups
			rows, err := tx.Query(`
				SELECT id FROM groups 
				WHERE parent_group_id = ?
			`, member.GroupID)
			if err != nil {
				tx.Rollback()
				return err
			}
			defer rows.Close()

			// For each child group, add the member with a new UUID
			for rows.Next() {
				var childGroupID string
				if err := rows.Scan(&childGroupID); err != nil {
					tx.Rollback()
					return err
				}

				// Create a new member for the child group
				childGroupUUID, err := uuid.Parse(childGroupID)
				if err != nil {
					tx.Rollback()
					return fmt.Errorf("invalid child group ID: %v", err)
				}

				childMember := &models.GroupMember{
					ID:          uuid.New(),
					GroupID:     childGroupUUID,
					UserID:      member.UserID,
					Role:        member.Role,
					IsInherited: true,
					JoinedAt:    time.Now().UTC(),
				}

				// Insert the member for this child group
				if _, err = tx.Exec(`
					INSERT INTO group_members (id, group_id, user_id, role, is_inherited, joined_at)
					VALUES (?, ?, ?, ?, ?, ?)
				`,
					childMember.ID,
					childMember.GroupID,
					childMember.UserID,
					childMember.Role,
					childMember.IsInherited,
					childMember.JoinedAt,
				); err != nil {
					tx.Rollback()
					return fmt.Errorf("failed to add member to child group: %w", err)
				}
			}

			// Check for errors that might have occurred during iteration
			if err = rows.Err(); err != nil {
				tx.Rollback()
				return fmt.Errorf("error iterating child groups: %w", err)
			}
		}
	}

	return tx.Commit()
}

// RemoveGroupMember removes a user from a group
func (r *groupRepository) RemoveGroupMember(ctx context.Context, groupID, userID uuid.UUID) error {
	// Don't allow removing the last admin
	var adminCount int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM group_members 
		WHERE group_id = ? AND role = 'admin'`,
		groupID,
	).Scan(&adminCount)

	if err != nil {
		return err
	}

	if adminCount <= 1 {
		// Check if the user being removed is an admin
		var isAdmin bool
		err = r.db.QueryRow(
			`SELECT role = 'admin' FROM group_members 
			WHERE group_id = ? AND user_id = ?`,
			groupID, userID,
		).Scan(&isAdmin)

		if err != nil {
			return err
		}

		if isAdmin {
			return errors.New("cannot remove the last admin from a group")
		}
	}

	result, err := r.db.Exec(
		`DELETE FROM group_members 
		WHERE group_id = ? AND user_id = ?`,
		groupID, userID,
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

// GetMembers returns all members of a group, including inherited members from parent groups
func (r *groupRepository) GetMembers(ctx context.Context, groupID uuid.UUID) ([]*models.GroupMember, error) {
	// Get direct members first
	directMembers, err := r.getDirectMembers(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get direct members: %w", err)
	}

	// Get the group to check if it's hierarchical and has a parent
	group, err := r.GetByID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	// If the group is hierarchical and has a parent, get inherited members
	if group.IsHierarchical && group.ParentGroupID != nil {
		// Get all admins from parent group
		parentAdmins, err := r.getAdmins(ctx, *group.ParentGroupID)
		if err != nil {
			r.log.Error().
				Err(err).
				Str("group_id", groupID.String()).
				Str("parent_group_id", group.ParentGroupID.String()).
				Msg("Failed to get parent group admins")
			return nil, fmt.Errorf("failed to get parent group admins: %w", err)
		}

		// Add parent admins as inherited members if not already in direct members
		for _, admin := range parentAdmins {
			isDuplicate := false
			for _, member := range directMembers {
				if member.UserID == admin.UserID {
					isDuplicate = true
					break
				}
			}

			if !isDuplicate {
				inheritedMember := &models.GroupMember{
					ID:          uuid.New(),
					GroupID:     groupID,
					UserID:      admin.UserID,
					Role:        "member", // Inherited members are always regular members
					IsInherited: true,
					JoinedAt:    admin.JoinedAt,
				}
				directMembers = append(directMembers, inheritedMember)
			}
		}
	}

	return directMembers, nil
}

// getDirectMembers returns only direct members of a group (no inherited members)
func (r *groupRepository) getDirectMembers(ctx context.Context, groupID uuid.UUID) ([]*models.GroupMember, error) {
	query := `
		SELECT id, group_id, user_id, role, is_inherited, joined_at
		FROM group_members 
		WHERE group_id = $1
		ORDER BY joined_at
	`

	rows, err := r.db.QueryContext(ctx, query, groupID)
	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", groupID.String()).
			Msg("Failed to query direct group members")
		return nil, fmt.Errorf("failed to query direct group members: %w", err)
	}
	defer rows.Close()

	var members []*models.GroupMember
	for rows.Next() {
		var member models.GroupMember
		err := rows.Scan(
			&member.ID,
			&member.GroupID,
			&member.UserID,
			&member.Role,
			&member.IsInherited,
			&member.JoinedAt,
		)
		if err != nil {
			r.log.Error().
				Err(err).
				Str("group_id", groupID.String()).
				Msg("Failed to scan group member")
			return nil, fmt.Errorf("failed to scan group member: %w", err)
		}
		members = append(members, &member)
	}

	if err = rows.Err(); err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", groupID.String()).
			Msg("Error iterating over group members")
		return nil, fmt.Errorf("error iterating over group members: %w", err)
	}

	return members, nil
}

// getAdmins returns all admin members of a group
func (r *groupRepository) getAdmins(ctx context.Context, groupID uuid.UUID) ([]*models.GroupMember, error) {
	query := `
		SELECT id, group_id, user_id, role, is_inherited, joined_at
		FROM group_members 
		WHERE group_id = $1 AND role = 'admin'
	`

	rows, err := r.db.QueryContext(ctx, query, groupID)
	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", groupID.String()).
			Msg("Failed to query group admins")
		return nil, fmt.Errorf("failed to query group admins: %w", err)
	}
	defer rows.Close()

	var admins []*models.GroupMember
	for rows.Next() {
		var admin models.GroupMember
		err := rows.Scan(
			&admin.ID,
			&admin.GroupID,
			&admin.UserID,
			&admin.Role,
			&admin.IsInherited,
			&admin.JoinedAt,
		)
		if err != nil {
			r.log.Error().
				Err(err).
				Str("group_id", groupID.String()).
				Msg("Failed to scan group admin")
			return nil, fmt.Errorf("failed to scan group admin: %w", err)
		}
		admins = append(admins, &admin)
	}

	if err = rows.Err(); err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", groupID.String()).
			Msg("Error iterating over group admins")
		return nil, fmt.Errorf("error iterating over group admins: %w", err)
	}

	return admins, nil
}

// RemoveMember removes a user from a group
func (r *groupRepository) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to begin transaction")
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			r.log.Error().Err(err).Msg("Failed to rollback transaction")
		}
	}()

	// Don't allow removing the last admin
	var adminCount int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM group_members 
		WHERE group_id = $1 AND role = 'admin' AND is_inherited = false
	`, groupID).Scan(&adminCount)

	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", groupID.String()).
			Msg("Failed to count group admins")
		return fmt.Errorf("failed to count group admins: %w", err)
	}

	if adminCount <= 1 {
		// Check if the user being removed is an admin
		var isAdmin bool
		err = tx.QueryRowContext(ctx, `
			SELECT role = 'admin' 
			FROM group_members 
			WHERE group_id = $1 AND user_id = $2 AND is_inherited = false
		`, groupID, userID).Scan(&isAdmin)

		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			r.log.Error().
				Err(err).
				Str("group_id", groupID.String()).
				Str("user_id", userID.String()).
				Msg("Failed to check if user is admin")
			return fmt.Errorf("failed to check if user is admin: %w", err)
		}

		if isAdmin {
			return fmt.Errorf("cannot remove the last admin from a group")
		}
	}

	// Remove the member
	result, err := tx.ExecContext(ctx, `
		DELETE FROM group_members 
		WHERE group_id = $1 AND user_id = $2 AND is_inherited = false
	`, groupID, userID)

	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", groupID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to remove group member")
		return fmt.Errorf("failed to remove group member: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", groupID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to get rows affected")
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if err := tx.Commit(); err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", groupID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to commit transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// IsMember checks if a user is a member of a group (directly or inherited)
func (r *groupRepository) IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	// Check direct membership first
	var isMember bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 
			FROM group_members 
			WHERE group_id = $1 AND user_id = $2
		)
	`, groupID, userID).Scan(&isMember)

	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", groupID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to check group membership")
		return false, fmt.Errorf("failed to check group membership: %w", err)
	}

	if isMember {
		return true, nil
	}

	// If not a direct member, check for inherited membership
	var parentGroupID *uuid.UUID
	err = r.db.QueryRowContext(ctx, `
		SELECT parent_group_id 
		FROM groups 
		WHERE id = $1 AND is_hierarchical = true
	`, groupID).Scan(&parentGroupID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Group doesn't exist or is not hierarchical
			return false, nil
		}
		r.log.Error().
			Err(err).
			Str("group_id", groupID.String()).
			Msg("Failed to get parent group ID")
		return false, fmt.Errorf("failed to get parent group ID: %w", err)
	}

	// If there's a parent group, check if the user is an admin there
	if parentGroupID != nil {
		return r.IsAdmin(ctx, *parentGroupID, userID)
	}

	return false, nil
}

// IsAdmin checks if a user is an admin of a group
func (r *groupRepository) IsAdmin(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	var isAdmin bool

	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 
			FROM group_members 
			WHERE group_id = $1 AND user_id = $2 AND role = 'admin' AND is_inherited = false
		)
	`, groupID, userID).Scan(&isAdmin)

	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", groupID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to check admin status")
		return false, fmt.Errorf("failed to check admin status: %w", err)
	}

	// If not a direct admin, check for inherited admin status from parent group
	if !isAdmin {
		var parentGroupID *uuid.UUID
		err = r.db.QueryRowContext(ctx, `
			SELECT parent_group_id 
			FROM groups 
			WHERE id = $1 AND is_hierarchical = true
		`, groupID).Scan(&parentGroupID)

		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			r.log.Error().
				Err(err).
				Str("group_id", groupID.String()).
				Msg("Failed to get parent group ID for admin check")
			return false, fmt.Errorf("failed to get parent group ID: %w", err)
		}

		if parentGroupID != nil {
			return r.IsAdmin(ctx, *parentGroupID, userID)
		}
	}

	return isAdmin, nil
}

// GetSubGroups returns all direct child groups of a parent group
func (r *groupRepository) GetSubGroups(ctx context.Context, parentGroupID uuid.UUID) ([]*models.Group, error) {
	var groups []*models.Group

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, created_by, created_at, updated_at, is_hierarchical, parent_group_id 
		FROM groups 
		WHERE parent_group_id = $1`,
		parentGroupID,
	)
	if err != nil {
		r.log.Error().
			Err(err).
			Str("parent_group_id", parentGroupID.String()).
			Msg("Failed to query subgroups")
		return nil, fmt.Errorf("failed to query subgroups: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var group models.Group
		var parentID uuid.NullUUID // Using NullUUID to properly handle NULL values from database

		err := rows.Scan(
			&group.ID,
			&group.Name,
			&group.Description,
			&group.CreatedBy,
			&group.CreatedAt,
			&group.UpdatedAt,
			&group.IsHierarchical,
			&parentID, // This will properly handle NULL values from the database
		)
		if err != nil {
			r.log.Error().
				Err(err).
				Str("parent_group_id", parentGroupID.String()).
				Msg("Failed to scan subgroup")
			return nil, fmt.Errorf("failed to scan subgroup: %w", err)
		}

		if parentID.Valid {
			group.ParentGroupID = &parentID.UUID // Only assign if the UUID is valid
		}

		groups = append(groups, &group)
	}

	if err = rows.Err(); err != nil {
		r.log.Error().
			Err(err).
			Str("parent_group_id", parentGroupID.String()).
			Msg("Error iterating subgroups")
		return nil, fmt.Errorf("error iterating subgroups: %w", err)
	}

	return groups, nil
}

// HasPendingInvitation checks if there's already a pending invitation for a user in a group
func (r *groupRepository) HasPendingInvitation(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	var count int

	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM group_invitations 
		WHERE group_id = $1 AND user_id = $2 AND status = 'pending'`,
		groupID, userID,
	).Scan(&count)

	if err != nil {
		r.log.Error().
			Err(err).
			Str("group_id", groupID.String()).
			Str("user_id", userID.String()).
			Msg("Failed to check for pending invitations")
		return false, fmt.Errorf("failed to check for pending invitations: %w", err)
	}

	return count > 0, nil
}
