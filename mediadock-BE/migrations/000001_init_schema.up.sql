CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(120) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);

CREATE TABLE IF NOT EXISTS media_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(64) NOT NULL,
    source_url TEXT NOT NULL,
    authorization_method VARCHAR(64) NOT NULL,
    authorization_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_media_sources_user_id ON media_sources (user_id);
CREATE INDEX IF NOT EXISTS idx_media_sources_provider ON media_sources (provider);

CREATE TABLE IF NOT EXISTS media_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_id UUID NOT NULL REFERENCES media_sources(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    thumbnail_url TEXT,
    duration INTEGER NOT NULL DEFAULT 0 CHECK (duration >= 0),
    original_format VARCHAR(32) NOT NULL,
    file_size BIGINT NOT NULL DEFAULT 0 CHECK (file_size >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_media_items_user_id ON media_items (user_id);
CREATE INDEX IF NOT EXISTS idx_media_items_source_id ON media_items (source_id);

CREATE TABLE IF NOT EXISTS download_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_id UUID NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'expired')),
    progress INTEGER NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    requested_format VARCHAR(32) NOT NULL,
    requested_quality VARCHAR(32) NOT NULL DEFAULT 'original',
    output_path TEXT,
    error_message TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_download_jobs_user_id ON download_jobs (user_id);
CREATE INDEX IF NOT EXISTS idx_download_jobs_status ON download_jobs (status);
CREATE INDEX IF NOT EXISTS idx_download_jobs_expires_at ON download_jobs (expires_at);

CREATE TABLE IF NOT EXISTS download_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES download_jobs(id) ON DELETE CASCADE,
    file_name VARCHAR(255) NOT NULL,
    downloaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_download_history_user_id ON download_history (user_id);
CREATE INDEX IF NOT EXISTS idx_download_history_downloaded_at ON download_history (downloaded_at);

CREATE TABLE IF NOT EXISTS user_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    theme VARCHAR(32) NOT NULL DEFAULT 'system' CHECK (theme IN ('light', 'dark', 'system')),
    default_format VARCHAR(32) NOT NULL DEFAULT 'mp4',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_settings_user_id ON user_settings (user_id);

