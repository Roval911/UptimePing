-- Fix api_keys schema to match auth-service domain/repository

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS secret_hash VARCHAR(255);

ALTER TABLE api_keys
    ALTER COLUMN secret_hash SET NOT NULL;

-- Ensure updated_at exists (created by base migration, but keep idempotent)
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;

