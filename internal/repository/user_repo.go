package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bard/bard-backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// GetByID finds a user by their UUID
func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, email, name, avatar_url, auth_provider, google_id, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user domain.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.AuthProvider,
		&user.GoogleID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	return &user, nil
}

// GetByEmail finds a user by their email address
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, name, avatar_url, auth_provider, google_id, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var user domain.User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.AuthProvider,
		&user.GoogleID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}

// Create inserts a new user and returns the created record
func (r *UserRepository) Create(ctx context.Context, email, name, authProvider string, googleID *string) (*domain.User, error) {
	query := `
		INSERT INTO users (email, name, auth_provider, google_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, name, avatar_url, auth_provider, google_id, created_at, updated_at
	`

	var user domain.User
	err := r.pool.QueryRow(ctx, query, email, name, authProvider, googleID).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.AuthProvider,
		&user.GoogleID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

// UpsertGoogleUser creates a user if they don't exist, or updates their info if they do.
// This is called every time someone logs in with Google.
func (r *UserRepository) UpsertGoogleUser(ctx context.Context, googleID, email, name, avatarURL string) (*domain.User, error) {
	query := `
		INSERT INTO users (email, name, avatar_url, auth_provider, google_id)
		VALUES ($1, $2, $3, 'google', $4)
		ON CONFLICT (google_id) DO UPDATE
		SET name = EXCLUDED.name,
		    avatar_url = EXCLUDED.avatar_url,
		    updated_at = NOW()
		RETURNING id, email, name, avatar_url, auth_provider, google_id, created_at, updated_at
	`

	var user domain.User
	err := r.pool.QueryRow(ctx, query, email, name, avatarURL, googleID).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.AuthProvider,
		&user.GoogleID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert google user: %w", err)
	}

	return &user, nil
}
