ALTER TABLE media_items
    ADD COLUMN IF NOT EXISTS media_type VARCHAR(16) NOT NULL DEFAULT 'video',
    ADD COLUMN IF NOT EXISTS downloadable BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS download_blocked_reason TEXT;

ALTER TABLE conversion_jobs
    ADD COLUMN IF NOT EXISTS thumbnail_path TEXT;

