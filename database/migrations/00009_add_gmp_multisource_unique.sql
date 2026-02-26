-- Migration: Support multi-source GMP syncing
BEGIN;

-- Drop the unique constraint from ipo_name
ALTER TABLE ipo_gmp DROP CONSTRAINT IF EXISTS ipo_gmp_ipo_name_key;

-- Add a composite UNIQUE constraint so ON CONFLICT queries can correctly UPSERT an IPO's data per source
ALTER TABLE ipo_gmp ADD CONSTRAINT uq_ipo_gmp_source UNIQUE (ipo_id, data_source);

COMMIT;