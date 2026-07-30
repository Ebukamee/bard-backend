package db

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AutoMigrate runs all migrations to set up the database schema.
// It's idempotent — safe to run on every startup.
func AutoMigrate(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []string{
		// 000001: users
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,
		`DO $$ BEGIN
			CREATE TYPE auth_provider AS ENUM ('google', 'magic_link');
		EXCEPTION WHEN duplicate_object THEN NULL;
		END $$`,
		`CREATE TABLE IF NOT EXISTS users (
			id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			email         TEXT NOT NULL UNIQUE,
			name          TEXT NOT NULL DEFAULT '',
			avatar_url    TEXT,
			auth_provider auth_provider NOT NULL,
			google_id     TEXT UNIQUE,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id) WHERE google_id IS NOT NULL`,

		// 000002: magic_links
		`CREATE TABLE IF NOT EXISTS magic_links (
			id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
			email      TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at    TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_magic_links_token_hash ON magic_links(token_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_magic_links_email ON magic_links(email)`,

		// 000003: transcriptions
		`DO $$ BEGIN
			CREATE TYPE transcription_status AS ENUM ('pending', 'processing', 'completed', 'failed');
		EXCEPTION WHEN duplicate_object THEN NULL;
		END $$`,
		`CREATE TABLE IF NOT EXISTS transcriptions (
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
		)`,
		`CREATE INDEX IF NOT EXISTS idx_transcriptions_user_id ON transcriptions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_transcriptions_status ON transcriptions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_transcriptions_user_created ON transcriptions(user_id, created_at DESC)`,

		// 000004: transcription types + filler word tracking + diary
		`DO $$ BEGIN
			CREATE TYPE transcription_type AS ENUM ('transcription', 'practice', 'diary');
		EXCEPTION WHEN duplicate_object THEN NULL;
		END $$`,
		`ALTER TABLE transcriptions ADD COLUMN IF NOT EXISTS type transcription_type NOT NULL DEFAULT 'transcription'`,
		`ALTER TABLE transcriptions ADD COLUMN IF NOT EXISTS filler_words TEXT`,
		`ALTER TABLE transcriptions ADD COLUMN IF NOT EXISTS filler_word_counts JSONB`,
		`ALTER TABLE transcriptions ADD COLUMN IF NOT EXISTS diary_date DATE`,
		`CREATE INDEX IF NOT EXISTS idx_transcriptions_type ON transcriptions(type)`,
		`CREATE INDEX IF NOT EXISTS idx_transcriptions_diary_date ON transcriptions(user_id, diary_date) WHERE diary_date IS NOT NULL`,

		// 000005: label for transcriptions
		`ALTER TABLE transcriptions ADD COLUMN IF NOT EXISTS label TEXT`,
	}

	for i, sql := range migrations {
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("migration step %d failed: %w", i+1, err)
		}
	}

	log.Println("Database migrations completed")
	return nil
}
