package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// nullString returns sql.NullString for a string pointer
func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

// CreateGroup creates a new group and adds the creator as an admin
func (d *Database) CreateGroup(group *Group) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	group.ID = uuid.New().String()
	group.CreatedAt = time.Now().UTC()
	group.UpdatedAt = group.CreatedAt

	// Insert the group
	var parentGroupID sql.NullString
	if group.ParentGroupID != nil {
		parentGroupID = sql.NullString{String: *group.ParentGroupID, Valid: true}
	}

	_, err = tx.Exec(
		`INSERT INTO groups (id, name, description, created_by, created_at, updated_at, is_hierarchical, parent_group_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		group.ID,
		group.Name,
		group.Description,
		group.CreatedBy,
		group.CreatedAt,
		group.UpdatedAt,
		group.IsHierarchical,
		parentGroupID, // Use the properly initialized NullString
	)

	if err != nil {
		tx.Rollback()
		return err
	}

	// Add the creator as an admin member
	_, err = tx.Exec(
		`INSERT INTO group_members (id, group_id, user_id, role, joined_at)
		VALUES (?, ?, ?, ?, ?)`,
		uuid.New().String(),
		group.ID,
		group.CreatedBy,
		"admin",
		time.Now().UTC(),
	)

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// GetGroupByID retrieves a group by its ID
func (d *Database) GetGroupByID(id string) (*Group, error) {
	group := &Group{}
	var parentGroupID sql.NullString

	err := d.db.QueryRow(
		`SELECT id, name, description, created_by, is_hierarchical, parent_group_id, created_at, updated_at 
		FROM groups WHERE id = ?`, id,
	).Scan(
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
		return nil, err
	}

	if parentGroupID.Valid {
		group.ParentGroupID = &parentGroupID.String
	}

	return group, nil
}

// UpdateGroup updates an existing group
func (d *Database) UpdateGroup(group *Group) error {
	group.UpdatedAt = time.Now().UTC()

	// Start a transaction
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update the group
	result, err := tx.Exec(
		`UPDATE groups 
		SET name = ?, description = ?, is_hierarchical = ?, parent_group_id = ?, updated_at = ?
		WHERE id = ?`,
		group.Name,
		group.Description,
		group.IsHierarchical,
		nullString(group.ParentGroupID),
		group.UpdatedAt,
		group.ID,
	)

	if err != nil {
		tx.Rollback()
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return err
	}

	if rowsAffected == 0 {
		tx.Rollback()
		return errors.New("group not found")
	}

	// If this is a hierarchical group, ensure the parent exists and is not creating a cycle
	if group.IsHierarchical && group.ParentGroupID != nil {
		// Check for circular reference
		var isCircular bool
		err = tx.QueryRow(`
			WITH RECURSIVE group_hierarchy AS (
				SELECT id, parent_group_id, 1 as level
				FROM groups
				WHERE id = ?
				UNION ALL
				SELECT g.id, g.parent_group_id, h.level + 1
				FROM groups g
				JOIN group_hierarchy h ON g.id = h.parent_group_id
				WHERE h.level < 10  -- Prevent infinite recursion
			)
			SELECT EXISTS (SELECT 1 FROM group_hierarchy WHERE id = ?)
		`, *group.ParentGroupID, group.ID).Scan(&isCircular)

		if err != nil {
			tx.Rollback()
			return err
		}

		if isCircular {
			tx.Rollback()
			return errors.New("circular group reference detected")
		}

		// Verify parent exists
		var parentExists bool
		err = tx.QueryRow(`
			SELECT EXISTS (SELECT 1 FROM groups WHERE id = ?)
		`, *group.ParentGroupID).Scan(&parentExists)

		if err != nil || !parentExists {
			tx.Rollback()
			return errors.New("parent group not found")
		}

		// If making a group hierarchical and it has a parent, ensure parent is also hierarchical
		if group.ParentGroupID != nil {
			var parentIsHierarchical bool
			err = tx.QueryRow(`
				SELECT is_hierarchical FROM groups WHERE id = ?
			`, *group.ParentGroupID).Scan(&parentIsHierarchical)

			if err != nil {
				tx.Rollback()
				return err
			}

			if !parentIsHierarchical {
				tx.Rollback()
				return errors.New("parent group must be hierarchical")
			}
		}
	}

	return tx.Commit()
}

// DeleteGroup deletes a group and all its associated data
func (d *Database) DeleteGroup(id string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	// Delete group events
	_, err = tx.Exec(`DELETE FROM group_events WHERE group_id = ?`, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Delete group invitations
	_, err = tx.Exec(`DELETE FROM group_invitations WHERE group_id = ?`, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Delete group members
	_, err = tx.Exec(`DELETE FROM group_members WHERE group_id = ?`, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Finally, delete the group
	result, err := tx.Exec(`DELETE FROM groups WHERE id = ?`, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return err
	}

	if rowsAffected == 0 {
		tx.Rollback()
		return sql.ErrNoRows
	}

	return tx.Commit()
}

// ListUserGroups returns all groups a user is a member of
func (d *Database) ListUserGroups(userID string) ([]*Group, error) {
	query := `
		SELECT g.id, g.name, g.description, g.created_by, g.created_at, g.updated_at
		FROM groups g
		JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = ?
		ORDER BY g.updated_at DESC
	`

	rows, err := d.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*Group
	for rows.Next() {
		var group Group
		err := rows.Scan(
			&group.ID,
			&group.Name,
			&group.Description,
			&group.CreatedBy,
			&group.CreatedAt,
			&group.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		groups = append(groups, &group)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return groups, nil
}

// AddGroupMember adds a user to a group with the specified role
func (d *Database) AddGroupMember(member *GroupMember) error {
	// Start a transaction
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Check if user is already a direct member (not inherited)
	var existingMemberID string
	err = tx.QueryRow(`
		SELECT id FROM group_members 
		WHERE group_id = ? AND user_id = ? AND is_inherited = ?
	`, member.GroupID, member.UserID, member.IsInherited).Scan(&existingMemberID)

	if err == nil {
		tx.Rollback()
		return errors.New("user is already a member of the group with the same inheritance status")
	} else if err != sql.ErrNoRows {
		tx.Rollback()
		return err
	}

	if member.ID == "" {
		member.ID = uuid.New().String()
	}

	if member.JoinedAt.IsZero() {
		member.JoinedAt = time.Now().UTC()
	}

	// Add the member
	_, err = tx.Exec(`
		INSERT INTO group_members (id, group_id, user_id, role, is_inherited, joined_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		member.ID,
		member.GroupID,
		member.UserID,
		member.Role,
		member.IsInherited,
		member.JoinedAt,
	)

	if err != nil {
		tx.Rollback()
		return err
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
				childMember := &GroupMember{
					ID:          uuid.New().String(),
					GroupID:     childGroupID,
					UserID:      member.UserID,
					Role:        member.Role,
					IsInherited: true,
					JoinedAt:    time.Now().UTC(),
				}

				// Insert the member for this child group
				_, err = tx.Exec(`
					INSERT INTO group_members (id, group_id, user_id, role, is_inherited, joined_at)
					VALUES (?, ?, ?, ?, ?, ?)
				`,
					childMember.ID,
					childMember.GroupID,
					childMember.UserID,
					childMember.Role,
					childMember.IsInherited,
					childMember.JoinedAt,
				)
				if err != nil {
					tx.Rollback()
					return err
				}
			}
			if err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	return tx.Commit()
}

// GetGroupAdmins returns all admin members of a group
func (d *Database) GetGroupAdmins(groupID string) ([]*GroupMember, error) {
	var admins []*GroupMember

	rows, err := d.db.Query(
		`SELECT id, group_id, user_id, role, joined_at 
		FROM group_members 
		WHERE group_id = ? AND role = ?`,
		groupID, "admin",
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var member GroupMember
		err := rows.Scan(
			&member.ID,
			&member.GroupID,
			&member.UserID,
			&member.Role,
			&member.JoinedAt,
		)
		if err != nil {
			return nil, err
		}
		admins = append(admins, &member)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return admins, nil
}

// RemoveGroupMember removes a user from a group
func (d *Database) RemoveGroupMember(groupID, userID string) error {
	// Don't allow removing the last admin
	var adminCount int
	err := d.db.QueryRow(
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
		err = d.db.QueryRow(
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

	result, err := d.db.Exec(
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

// GetGroupMembers returns all members of a group, including inherited members from parent groups
func (d *Database) GetGroupMembers(groupID string) ([]*GroupMember, error) {
	// Get direct members first
	directMembers, err := d.getDirectGroupMembers(groupID)
	if err != nil {
		return nil, err
	}

	// Get the group to check if it's hierarchical and has a parent
	group, err := d.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}

	// If the group is hierarchical and has a parent, get inherited members
	if group.IsHierarchical && group.ParentGroupID != nil {
		// Get all admins from parent group
		parentAdmins, err := d.GetGroupAdmins(*group.ParentGroupID)
		if err != nil {
			return nil, err
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
				inheritedMember := &GroupMember{
					ID:          uuid.New().String(),
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

// getDirectGroupMembers returns only direct members of a group (no inherited members)
func (d *Database) getDirectGroupMembers(groupID string) ([]*GroupMember, error) {
	rows, err := d.db.Query(
		`SELECT id, group_id, user_id, role, is_inherited, joined_at
		FROM group_members 
		WHERE group_id = ?
		ORDER BY joined_at`,
		groupID,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*GroupMember
	for rows.Next() {
		var member GroupMember
		err := rows.Scan(
			&member.ID,
			&member.GroupID,
			&member.UserID,
			&member.Role,
			&member.IsInherited,
			&member.JoinedAt,
		)
		if err != nil {
			return nil, err
		}
		members = append(members, &member)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return members, nil
}

// IsGroupMember checks if a user is a member of a group (directly or inherited)
func (d *Database) IsGroupMember(groupID, userID string) (bool, error) {
	// First check direct membership
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM group_members 
		WHERE group_id = ? AND user_id = ?`,
		groupID, userID,
	).Scan(&count)

	if err != nil {
		return false, err
	}

	if count > 0 {
		return true, nil
	}

	// If not a direct member, check for inherited membership
	var parentGroupID *string
	err = d.db.QueryRow(
		`SELECT parent_group_id FROM groups 
		WHERE id = ? AND is_hierarchical = true`,
		groupID,
	).Scan(&parentGroupID)

	if err != nil {
		if err == sql.ErrNoRows {
			// Group doesn't exist or is not hierarchical
			return false, nil
		}
		return false, err
	}

	// If there's a parent group, check if the user is an admin there
	if parentGroupID != nil {
		// Check if user is an admin in the parent group
		isAdmin, err := d.IsGroupAdmin(*parentGroupID, userID)
		if err != nil {
			return false, err
		}
		return isAdmin, nil
	}

	return false, nil
}

// GetGroupMember retrieves a specific group member by group ID and user ID
func (d *Database) GetGroupMember(groupID, userID string) (*GroupMember, error) {
	// First try to get a direct member
	member := &GroupMember{}
	err := d.db.QueryRow(
		`SELECT id, group_id, user_id, role, is_inherited, joined_at 
		FROM group_members 
		WHERE group_id = ? AND user_id = ?`,
		groupID, userID,
	).Scan(
		&member.ID,
		&member.GroupID,
		&member.UserID,
		&member.Role,
		&member.IsInherited,
		&member.JoinedAt,
	)

	// If found, return the member
	if err == nil {
		return member, nil
	}

	// If not found and not because of a database error, check for inherited membership
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Check if the user is an inherited member (admin in parent group)
	var parentGroupID *string
	err = d.db.QueryRow(
		`SELECT parent_group_id FROM groups 
		WHERE id = ? AND is_hierarchical = true`,
		groupID,
	).Scan(&parentGroupID)

	if err != nil {
		if err == sql.ErrNoRows {
			// Group doesn't exist or is not hierarchical
			return nil, nil
		}
		return nil, err
	}

	// If there's a parent group, check if the user is an admin there
	if parentGroupID != nil {
		// Check if user is an admin in the parent group
		isAdmin, err := d.IsGroupAdmin(*parentGroupID, userID)
		if err != nil {
			return nil, err
		}

		if isAdmin {
			// Return a virtual member representing the inherited membership
			return &GroupMember{
				ID:          uuid.New().String(),
				GroupID:     groupID,
				UserID:      userID,
				Role:        "member", // Inherited members are always regular members
				IsInherited: true,
				JoinedAt:    time.Now().UTC(),
			}, nil
		}
	}

	// User is not a member of this group
	return nil, nil
}

// IsGroupAdmin checks if a user is a direct admin of a group
// Note: This does not check inherited admin status from parent groups
func (d *Database) IsGroupAdmin(groupID, userID string) (bool, error) {
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM group_members 
		WHERE group_id = ? AND user_id = ? AND role = 'admin' AND is_inherited = 0`,
		groupID, userID,
	).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// RemoveParentFromChildren removes the parent reference from all child groups
func (d *Database) RemoveParentFromChildren(parentID string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get all direct children of the parent group
	rows, err := tx.Query(
		`SELECT id FROM groups 
		WHERE parent_group_id = ?`, parentID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error querying child groups: %v", err)
	}
	defer rows.Close()

	var childIDs []string
	for rows.Next() {
		var childID string
		if err := rows.Scan(&childID); err != nil {
			tx.Rollback()
			return fmt.Errorf("error scanning child group ID: %v", err)
		}
		childIDs = append(childIDs, childID)
	}

	// Update each child to remove the parent reference
	for _, childID := range childIDs {
		_, err = tx.Exec(
			`UPDATE groups 
			SET parent_group_id = NULL, 
			updated_at = CURRENT_TIMESTAMP 
			WHERE id = ?`, childID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error updating child group %s: %v", childID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		tx.Rollback()
		return fmt.Errorf("error committing transaction: %v", err)
	}

	return nil
}

// GetSubGroups returns all direct child groups of a parent group
func (d *Database) GetSubGroups(parentGroupID string) ([]*Group, error) {
	var groups []*Group

	rows, err := d.db.Query(
		`SELECT id, name, description, created_by, created_at, updated_at, is_hierarchical, parent_group_id 
		FROM groups 
		WHERE parent_group_id = ?`,
		parentGroupID,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying subgroups: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var group Group
		var parentID sql.NullString

		err := rows.Scan(
			&group.ID,
			&group.Name,
			&group.Description,
			&group.CreatedBy,
			&group.CreatedAt,
			&group.UpdatedAt,
			&group.IsHierarchical,
			&parentID,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning subgroup: %v", err)
		}

		if parentID.Valid {
			group.ParentGroupID = &parentID.String
		}

		groups = append(groups, &group)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating subgroups: %v", err)
	}

	return groups, nil
}

// HasPendingInvitation checks if there's already a pending invitation for a user in a group
func (d *Database) HasPendingInvitation(groupID, userID string) (bool, error) {
	var count int

	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM group_invitations 
		WHERE group_id = ? AND user_id = ? AND status = 'pending'`,
		groupID, userID,
	).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("error checking for pending invitations: %v", err)
	}

	return count > 0, nil
}
