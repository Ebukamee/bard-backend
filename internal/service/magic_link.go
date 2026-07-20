package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MagicLinkExpiry = 15 * time.Minute
	TokenByteLength = 32 // 32 bytes = 64 hex characters
)

var (
	ErrMagicLinkInvalid = errors.New("magic link is invalid or expired")
	ErrMagicLinkUsed    = errors.New("magic link has already been used")
)

// MagicLinkRecord represents a row from the magic_links table
type MagicLinkRecord struct {
	ID        string
	UserID    *string // nil if user doesn't exist yet
	Email     string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// MagicLinkService handles creating and verifying magic links
type MagicLinkService struct {
	pool *pgxpool.Pool
}

func NewMagicLinkService(pool *pgxpool.Pool) *MagicLinkService {
	return &MagicLinkService{pool: pool}
}

// CreateToken generates a random token for the given email, stores its hash in the DB,
// and returns the RAW token (to be sent in the email link).
// The raw token is NEVER stored — only its SHA-256 hash lives in the database.
func (s *MagicLinkService) CreateToken(ctx context.Context, email string, userID *string) (string, error) {
	// Step 1: Generate 32 cryptographically random bytes
	rawBytes := make([]byte, TokenByteLength)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	// Step 2: Convert to a hex string (human-readable, safe for URLs)
	rawToken := hex.EncodeToString(rawBytes)

	// Step 3: Hash the token with SHA-256 before storing
	tokenHash := hashToken(rawToken)

	// Step 4: Store the hash in the database
	query := `
		INSERT INTO magic_links (user_id, email, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`
	expiresAt := time.Now().Add(MagicLinkExpiry)

	_, err := s.pool.Exec(ctx, query, userID, email, tokenHash, expiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to store magic link: %w", err)
	}

	// Return the raw token — this goes in the email link
	return rawToken, nil
}

// VerifyToken checks if a magic link token is valid.
// It finds the matching hash, checks expiry and usage, then marks it as used.
// Returns the email associated with the magic link.
func (s *MagicLinkService) VerifyToken(ctx context.Context, rawToken string) (string, error) {
	// Step 1: Hash the incoming token so we can look it up
	tokenHash := hashToken(rawToken)

	// Step 2: Find the magic link record
	query := `
		SELECT id, user_id, email, token_hash, expires_at, used_at, created_at
		FROM magic_links
		WHERE token_hash = $1
	`

	var record MagicLinkRecord
	err := s.pool.QueryRow(ctx, query, tokenHash).Scan(
		&record.ID,
		&record.UserID,
		&record.Email,
		&record.TokenHash,
		&record.ExpiresAt,
		&record.UsedAt,
		&record.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrMagicLinkInvalid
		}
		return "", fmt.Errorf("failed to query magic link: %w", err)
	}

	// Step 3: Check if already used
	if record.UsedAt != nil {
		return "", ErrMagicLinkUsed
	}

	// Step 4: Check if expired
	if time.Now().After(record.ExpiresAt) {
		return "", ErrMagicLinkInvalid
	}

	// Step 5: Mark as used (prevents replay attacks)
	markQuery := `UPDATE magic_links SET used_at = NOW() WHERE id = $1`
	_, err = s.pool.Exec(ctx, markQuery, record.ID)
	if err != nil {
		return "", fmt.Errorf("failed to mark magic link as used: %w", err)
	}

	return record.Email, nil
}

// hashToken creates a SHA-256 hash of the raw token and returns it as a hex string.
func hashToken(rawToken string) string {
	hash := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(hash[:])
}
