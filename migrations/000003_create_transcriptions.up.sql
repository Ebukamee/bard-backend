CREATE TYPE transcription_status AS ENUM (
    'pending',
    'processing',
    'completed',
    'failed'
);

CREATE TABLE transcriptions (
    id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    audio_gcs_url         TEXT NOT NULL,
    audio_filename        TEXT NOT NULL,
    audio_duration_secs   INTEGER,
    original_transcript   TEXT,
    cleaned_transcript    TEXT,
    summary               TEXT,
    status                transcription_status NOT NULL DEFAULT 'pending',
    error_message         TEXT,
    processing_started_at TIMESTAMPTZ,
    processing_ended_at   TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transcriptions_user_id ON transcriptions(user_id);
CREATE INDEX idx_transcriptions_status ON transcriptions(status);
CREATE INDEX idx_transcriptions_user_created ON transcriptions(user_id, created_at DESC);
