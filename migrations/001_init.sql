CREATE EXTENSION IF NOT EXISTS pgcrypto;
DO $$ BEGIN
 CREATE TYPE upload_status AS ENUM ('uploading', 'uploaded', 'failed');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
DO $$ BEGIN
 CREATE TYPE processing_status AS ENUM ('pending', 'processing', 'completed', 'failed');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
CREATE TABLE IF NOT EXISTS avatars (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id VARCHAR(255) NOT NULL,
 file_name VARCHAR(255) NOT NULL, mime_type VARCHAR(100) NOT NULL,
 size_bytes BIGINT NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 10485760),
 s3_key VARCHAR(500) NOT NULL UNIQUE, thumbnail_s3_keys JSONB NOT NULL DEFAULT '{}'::jsonb,
 upload_status upload_status NOT NULL DEFAULT 'uploaded', processing_status processing_status NOT NULL DEFAULT 'pending',
 processing_message_id UUID, width INTEGER NOT NULL DEFAULT 0, height INTEGER NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_avatars_user_id ON avatars(user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_avatars_status ON avatars(upload_status, processing_status);
