package domain

import "time"

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// Transcription types — determines how the backend processes the audio
const (
	TypeTranscription = "transcription" // Normal transcription (recorded or uploaded audio) — filler removal + summary
	TypePractice      = "practice"      // Speech practice — tracks specific filler words the user wants to eliminate
	TypeDiary         = "diary"         // Daily diary entry — transcribed and summarized as a journal entry
)

type Transcription struct {
	ID                  string     `json:"id"`
	UserID              string     `json:"user_id"`
	AudioGCSURL         string     `json:"-"`
	AudioFilename       string     `json:"audio_filename"`
	AudioDurationSecs   *int       `json:"audio_duration_secs,omitempty"`
	Type                string     `json:"type"`
	Label               *string    `json:"label,omitempty"` // Frontend display label, e.g. "recording", "song", "lecture", "interview"
	OriginalTranscript  *string    `json:"original_transcript,omitempty"`
	CleanedTranscript   *string    `json:"cleaned_transcript,omitempty"`
	Summary             *string    `json:"summary,omitempty"`
	FillerWords         *string    `json:"filler_words,omitempty"`       // User-specified filler words (comma-separated) for practice mode
	FillerWordCounts    *string    `json:"filler_word_counts,omitempty"` // JSON object: {"um": 5, "like": 3} — returned by AI
	DiaryDate           *string    `json:"diary_date,omitempty"`         // YYYY-MM-DD for diary entries
	Status              string     `json:"status"`
	ErrorMessage        *string    `json:"-"`
	ProcessingStartedAt *time.Time `json:"-"`
	ProcessingEndedAt   *time.Time `json:"processing_ended_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
