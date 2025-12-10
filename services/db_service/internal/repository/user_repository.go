package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/agenda-distribuida/db-service/internal/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

var (
	// ErrUserNotFound is returned when a user is not found
	ErrUserNotFound = errors.New("user not found")
	// ErrEmailAlreadyExists is returned when a user with the same email already exists
	ErrEmailAlreadyExists = errors.New("email already exists")
)

// UserRepository defines the interface for user data access
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, id uuid.UUID, updateReq *models.UpdateUserRequest) (*models.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, offset, limit int) ([]*models.User, error)
}

type userRepository struct {
	db  *sql.DB
	log zerolog.Logger
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sql.DB, log zerolog.Logger) UserRepository {
	return &userRepository{
		db:  db,
		log: log,
	}
}

// Create creates a new user in the database
func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, username, email, hashed_password, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	_, err := r.db.ExecContext(
		ctx,
		query,
		user.ID,
		user.Username,
		user.Email,
		user.HashedPassword,
		user.IsActive,
		user.CreatedAt,
		user.UpdatedAt,
	)

	if err != nil {
		r.log.Error().Err(err).Str("email", user.Email).Msg("Failed to create user")
		return err
	}

	return nil
}

// GetByID retrieves a user by their ID
func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, username, email, hashed_password, is_active, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.HashedPassword,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		r.log.Error().Err(err).Str("user_id", id.String()).Msg("Failed to get user by ID")
		return nil, err
	}

	return &user, nil
}

// GetByEmail retrieves a user by their email
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, username, email, hashed_password, is_active, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var user models.User
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.HashedPassword,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		r.log.Error().Err(err).Str("email", email).Msg("Failed to get user by email")
		return nil, err
	}

	return &user, nil
}

// Update updates a user's information
func (r *userRepository) Update(ctx context.Context, id uuid.UUID, updateReq *models.UpdateUserRequest) (*models.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error().Err(err).Str("user_id", id.String()).Msg("Failed to begin transaction")
		return nil, err
	}

	// Use a closure to handle transaction rollback in case of errors
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Get the current user data
	user, err := func() (*models.User, error) {
		var user models.User
		query := `
			SELECT id, username, email, hashed_password, is_active, created_at, updated_at
			FROM users
			WHERE id = $1
		`

		err := tx.QueryRowContext(ctx, query, id).Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.HashedPassword,
			&user.IsActive,
			&user.CreatedAt,
			&user.UpdatedAt,
		)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrUserNotFound
			}
			r.log.Error().Err(err).Str("user_id", id.String()).Msg("Failed to get user by ID")
			return nil, err
		}
		return &user, nil
	}()

	if err != nil {
		return nil, err
	}

	// Update fields if they are provided in the request
	if updateReq.Username != nil {
		user.Username = *updateReq.Username
	}

	if updateReq.Email != nil && *updateReq.Email != user.Email {
		// Check if email already exists
		exists, err := r.emailExists(ctx, tx, *updateReq.Email)
		if err != nil {
			r.log.Error().Err(err).Str("email", *updateReq.Email).Msg("Failed to check email existence")
			return nil, err
		}
		if exists {
			return nil, ErrEmailAlreadyExists
		}
		user.Email = *updateReq.Email
	}

	if updateReq.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*updateReq.Password), bcrypt.DefaultCost)
		if err != nil {
			r.log.Error().Err(err).Str("user_id", id.String()).Msg("Failed to hash password")
			return nil, err
		}
		user.HashedPassword = string(hashedPassword)
	}

	if updateReq.IsActive != nil {
		user.IsActive = *updateReq.IsActive
	}

	user.UpdatedAt = time.Now()

	// Update the user in the database
	query := `
		UPDATE users
		SET username = $1, email = $2, hashed_password = $3, is_active = $4, updated_at = $5
		WHERE id = $6
	`

	_, err = tx.ExecContext(
		ctx,
		query,
		user.Username,
		user.Email,
		user.HashedPassword,
		user.IsActive,
		user.UpdatedAt,
		id,
	)

	if err != nil {
		r.log.Error().Err(err).Str("user_id", id.String()).Msg("Failed to update user")
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		r.log.Error().Err(err).Str("user_id", id.String()).Msg("Failed to commit transaction")
		return nil, err
	}

	return user, nil
}

// Delete removes a user from the database
func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		r.log.Error().Err(err).Str("user_id", id.String()).Msg("Failed to delete user")
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// List retrieves a paginated list of users
func (r *userRepository) List(ctx context.Context, offset, limit int) ([]*models.User, error) {
	query := `
		SELECT id, username, email, is_active, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to list users")
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.IsActive,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, &user)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// emailExists checks if a user with the given email already exists
func (r *userRepository) emailExists(ctx context.Context, tx *sql.Tx, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	var exists bool
	err := tx.QueryRowContext(ctx, query, email).Scan(&exists)
	if err != nil {
		r.log.Error().Err(err).Str("email", email).Msg("Failed to check if email exists")
		return false, err
	}
	return exists, nil
}
