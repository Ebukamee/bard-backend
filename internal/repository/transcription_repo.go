package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bard/bard-backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrTranscriptionNotFound = errors.New("transcription not found")

type TranscriptionRepository struct {
	pool *pgxpool.Pool
}

func NewTranscriptionRepository(pool *pgxpool.Pool) *TranscriptionRepository {
	return &TranscriptionRepository{pool: pool}
}

// Create inserts a new transcription record (status = "pending")
func (r *TranscriptionRepository) Create(ctx context.Context, userID, audioGCSURL, audioFilename string) (*domain.Transcription, error) {
	query := `
		INSERT INTO transcriptions (user_id, audio_gcs_url, audio_filename, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, audio_gcs_url, audio_filename, audio_duration_secs,
		          original_transcript, cleaned_transcript, summary,
		          status, error_message, processing_started_at, processing_ended_at,
		          created_at, updated_at
	`

	var t domain.Transcription
	err := r.pool.QueryRow(ctx, query, userID, audioGCSURL, audioFilename, domain.StatusPending).Scan(
		&t.ID, &t.UserID, &t.AudioGCSURL, &t.AudioFilename, &t.AudioDurationSecs,
		&t.OriginalTranscript, &t.CleanedTranscript, &t.Summary,
		&t.Status, &t.ErrorMessage, &t.ProcessingStartedAt, &t.ProcessingEndedAt,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcription: %w", err)
	}

	return &t, nil
}

// GetByID returns a single transcription (only if it belongs to the given user)
func (r *TranscriptionRepository) GetByID(ctx context.Context, id, userID string) (*domain.Transcription, error) {
	query := `
		SELECT id, user_id, audio_gcs_url, audio_filename, audio_duration_secs,
		       original_transcript, cleaned_transcript, summary,
		       status, error_message, processing_started_at, processing_ended_at,
		       created_at, updated_at
		FROM transcriptions
		WHERE id = $1 AND user_id = $2
	`

	var t domain.Transcription
	err := r.pool.QueryRow(ctx, query, id, userID).Scan(
		&t.ID, &t.UserID, &t.AudioGCSURL, &t.AudioFilename, &t.AudioDurationSecs,
		&t.OriginalTranscript, &t.CleanedTranscript, &t.Summary,
		&t.Status, &t.ErrorMessage, &t.ProcessingStartedAt, &t.ProcessingEndedAt,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTranscriptionNotFound
		}
		return nil, fmt.Errorf("failed to get transcription: %w", err)
	}

	return &t, nil
}

// ListByUser returns all transcriptions for a user, newest first
func (r *TranscriptionRepository) ListByUser(ctx context.Context, userID string, limit, offset int) ([]domain.Transcription, error) {
	query := `
		SELECT id, user_id, audio_gcs_url, audio_filename, audio_duration_secs,
		       original_transcript, cleaned_transcript, summary,
		       status, error_message, processing_started_at, processing_ended_at,
		       created_at, updated_at
		FROM transcriptions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list transcriptions: %w", err)
	}
	defer rows.Close()

	var transcriptions []domain.Transcription
	for rows.Next() {
		var t domain.Transcription
		err := rows.Scan(
			&t.ID, &t.UserID, &t.AudioGCSURL, &t.AudioFilename, &t.AudioDurationSecs,
			&t.OriginalTranscript, &t.CleanedTranscript, &t.Summary,
			&t.Status, &t.ErrorMessage, &t.ProcessingStartedAt, &t.ProcessingEndedAt,
			&t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transcription: %w", err)
		}
		transcriptions = append(transcriptions, t)
	}

	return transcriptions, nil
}

// UpdateStatus changes the status of a transcription (used by the processing pipeline)
func (r *TranscriptionRepository) UpdateStatus(ctx context.Context, id, status string, errorMsg *string) error {
	query := `
		UPDATE transcriptions
		SET status = $1, error_message = $2, updated_at = NOW()
		WHERE id = $3
	`

	_, err := r.pool.Exec(ctx, query, status, errorMsg, id)
	if err != nil {
		return fmt.Errorf("failed to update transcription status: %w", err)
	}

	return nil
}

// UpdateResult saves the processing results (transcripts + summary)
func (r *TranscriptionRepository) UpdateResult(ctx context.Context, id string, durationSecs int, original, cleaned, summary string) error {
	query := `
		UPDATE transcriptions
		SET audio_duration_secs = $1,
		    original_transcript = $2,
		    cleaned_transcript = $3,
		    summary = $4,
		    status = $5,
		    processing_ended_at = NOW(),
		    updated_at = NOW()
		WHERE id = $6
	`

	_, err := r.pool.Exec(ctx, query, durationSecs, original, cleaned, summary, domain.StatusCompleted, id)
	if err != nil {
		return fmt.Errorf("failed to update transcription result: %w", err)
	}

	return nil
}

// Delete removes a transcription (only if it belongs to the given user)
func (r *TranscriptionRepository) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM transcriptions WHERE id = $1 AND user_id = $2`

	result, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete transcription: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrTranscriptionNotFound
	}

	return nil
}
