-- +goose Up
-- Migration: Support multi-source GMP syncing

-- Drop the unique constraint from ipo_name
ALTER TABLE ipo_gmp DROP CONSTRAINT IF EXISTS ipo_gmp_ipo_name_key;

-- Add a composite UNIQUE constraint so ON CONFLICT queries can correctly UPSERT an IPO's data per source
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'uq_ipo_gmp_source'
          AND conrelid = 'ipo_gmp'::regclass
    ) THEN
        ALTER TABLE ipo_gmp ADD CONSTRAINT uq_ipo_gmp_source UNIQUE (ipo_id, data_source);
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE ipo_gmp DROP CONSTRAINT IF EXISTS uq_ipo_gmp_source;
