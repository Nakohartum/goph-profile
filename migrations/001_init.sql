CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TABLE IF NOT EXISTS avatars (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id VARCHAR(255) NOT NULL,
 file_name VARCHAR(255) NOT NULL, mime_type VARCHAR(100) NOT NULL,
 size_bytes BIGINT NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 10485760),
 s3_key VARCHAR(500) NOT NULL UNIQUE, thumbnail_s3_keys JSONB NOT NULL DEFAULT '{}'::jsonb,
 upload_status VARCHAR(50) NOT NULL DEFAULT 'uploaded', processing_status VARCHAR(50) NOT NULL DEFAULT 'pending',
 processing_message_id UUID, width INTEGER NOT NULL DEFAULT 0, height INTEGER NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_avatars_user_id ON avatars(user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_avatars_status ON avatars(upload_status, processing_status);
