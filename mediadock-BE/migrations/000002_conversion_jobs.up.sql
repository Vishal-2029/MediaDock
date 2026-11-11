CREATE TABLE IF NOT EXISTS conversion_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_filename VARCHAR(255) NOT NULL,
    source_format VARCHAR(32) NOT NULL,
    target_format VARCHAR(32) NOT NULL,
    target_resolution VARCHAR(32),
    compress BOOLEAN NOT NULL DEFAULT FALSE,
    extract_audio BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(32) NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'expired')),
    progress INTEGER NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    input_path TEXT NOT NULL,
    output_path TEXT,
    file_size BIGINT NOT NULL DEFAULT 0 CHECK (file_size >= 0),
    duration INTEGER NOT NULL DEFAULT 0 CHECK (duration >= 0),
    error_message TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversion_jobs_user_id ON conversion_jobs (user_id);
CREATE INDEX IF NOT EXISTS idx_conversion_jobs_status ON conversion_jobs (status);
CREATE INDEX IF NOT EXISTS idx_conversion_jobs_expires_at ON conversion_jobs (expires_at);

