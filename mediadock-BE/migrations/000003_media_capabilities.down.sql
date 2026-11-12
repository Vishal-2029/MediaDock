ALTER TABLE conversion_jobs
    DROP COLUMN IF EXISTS thumbnail_path;

ALTER TABLE media_items
    DROP COLUMN IF EXISTS download_blocked_reason,
    DROP COLUMN IF EXISTS downloadable,
    DROP COLUMN IF EXISTS media_type;

