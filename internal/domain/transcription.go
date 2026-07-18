package domain

import "time"

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

type Transcription struct {
	ID                  string     `json:"id"`
	UserID              string     `json:"user_id"`
	AudioGCSURL         string     `json:"-"`
	AudioFilename       string     `json:"audio_filename"`
	AudioDurationSecs   *int       `json:"audio_duration_secs,omitempty"`
	OriginalTranscript  *string    `json:"original_transcript,omitempty"`
	CleanedTranscript   *string    `json:"cleaned_transcript,omitempty"`
	Summary             *string    `json:"summary,omitempty"`
	Status              string     `json:"status"`
	ErrorMessage        *string    `json:"-"`
	ProcessingStartedAt *time.Time `json:"-"`
	ProcessingEndedAt   *time.Time `json:"processing_ended_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
