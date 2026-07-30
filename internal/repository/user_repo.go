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

// UpdateName sets the user's display name.
func (r *UserRepository) UpdateName(ctx context.Context, id, name string) (*domain.User, error) {
	query := `
		UPDATE users SET name = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, email, name, avatar_url, auth_provider, google_id, created_at, updated_at
	`

	var user domain.User
	err := r.pool.QueryRow(ctx, query, name, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL,
		&user.AuthProvider, &user.GoogleID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update user name: %w", err)
	}

	return &user, nil
}

// UpsertGoogleUser handles Google OAuth login. It accounts for three cases:
//  1. User already logged in with Google before (google_id match) → update name/avatar
//  2. User exists via magic link with the same email → link Google ID to that account
//  3. Brand new user → create a new account
func (r *UserRepository) UpsertGoogleUser(ctx context.Context, googleID, email, name, avatarURL string) (*domain.User, error) {
	// First, check if a user with this email already exists (e.g. from magic link signup)
	existing, err := r.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	if existing != nil && existing.GoogleID == nil {
		// Case 2: User exists via magic link — link their Google ID to the existing account
		query := `
			UPDATE users
			SET google_id = $1, name = $2, avatar_url = $3, auth_provider = 'google', updated_at = NOW()
			WHERE id = $4
			RETURNING id, email, name, avatar_url, auth_provider, google_id, created_at, updated_at
		`
		var user domain.User
		err := r.pool.QueryRow(ctx, query, googleID, name, avatarURL, existing.ID).Scan(
			&user.ID, &user.Email, &user.Name, &user.AvatarURL,
			&user.AuthProvider, &user.GoogleID, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to link google to existing user: %w", err)
		}
		return &user, nil
	}

	// Case 1 & 3: Upsert by google_id (returning user or brand new)
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
	err = r.pool.QueryRow(ctx, query, email, name, avatarURL, googleID).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL,
		&user.AuthProvider, &user.GoogleID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert google user: %w", err)
	}

	return &user, nil
}
