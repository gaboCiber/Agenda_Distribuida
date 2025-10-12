package models

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

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
	_, err = tx.Exec(
		`INSERT INTO groups (id, name, description, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		group.ID,
		group.Name,
		group.Description,
		group.CreatedBy,
		group.CreatedAt,
		group.UpdatedAt,
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
	err := d.db.QueryRow(
		`SELECT id, name, description, created_by, created_at, updated_at 
		FROM groups WHERE id = ?`, id,
	).Scan(
		&group.ID,
		&group.Name,
		&group.Description,
		&group.CreatedBy,
		&group.CreatedAt,
		&group.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return group, nil
}

// UpdateGroup updates an existing group
func (d *Database) UpdateGroup(group *Group) error {
	group.UpdatedAt = time.Now().UTC()

	result, err := d.db.Exec(
		`UPDATE groups 
		SET name = ?, description = ?, updated_at = ? 
		WHERE id = ?`,
		group.Name,
		group.Description,
		group.UpdatedAt,
		group.ID,
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

// AddGroupMember adds a user to a group
func (d *Database) AddGroupMember(member *GroupMember) error {
	// Check if user is already a member
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM group_members 
		WHERE group_id = ? AND user_id = ?`,
		member.GroupID, member.UserID,
	).Scan(&count)

	if err != nil {
		return err
	}

	if count > 0 {
		return errors.New("user is already a member of this group")
	}

	_, err = d.db.Exec(
		`INSERT INTO group_members (id, group_id, user_id, role, joined_at)
		VALUES (?, ?, ?, ?, ?)`,
		member.ID,
		member.GroupID,
		member.UserID,
		member.Role,
		member.JoinedAt,
	)

	return err
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

// GetGroupMembers returns all members of a group
func (d *Database) GetGroupMembers(groupID string) ([]*GroupMember, error) {
	rows, err := d.db.Query(
		`SELECT id, group_id, user_id, role, joined_at
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

// IsGroupMember checks if a user is a member of a group
func (d *Database) IsGroupMember(groupID, userID string) (bool, error) {
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM group_members 
		WHERE group_id = ? AND user_id = ?`,
		groupID, userID,
	).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetGroupMember retrieves a specific group member by group ID and user ID
func (d *Database) GetGroupMember(groupID, userID string) (*GroupMember, error) {
	member := &GroupMember{}
	err := d.db.QueryRow(
		`SELECT id, group_id, user_id, role, joined_at 
		FROM group_members 
		WHERE group_id = ? AND user_id = ?`,
		groupID, userID,
	).Scan(
		&member.ID,
		&member.GroupID,
		&member.UserID,
		&member.Role,
		&member.JoinedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return member, nil
}

// IsGroupAdmin checks if a user is an admin of a group
func (d *Database) IsGroupAdmin(groupID, userID string) (bool, error) {
	var count int
	err := d.db.QueryRow(
		`SELECT COUNT(*) FROM group_members 
		WHERE group_id = ? AND user_id = ? AND role = 'admin'`,
		groupID, userID,
	).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}
