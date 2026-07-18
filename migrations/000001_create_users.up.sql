CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE auth_provider AS ENUM ('google', 'magic_link');

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL DEFAULT '',
    avatar_url    TEXT,
    auth_provider auth_provider NOT NULL,
    google_id     TEXT UNIQUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_google_id ON users(google_id) WHERE google_id IS NOT NULL;
