-- +goose Up
-- Migration: Add registrar columns to ipo_list table
-- Purpose: Add registrar_id and registrar_company_code columns to ipo_list
--          to support direct IPO-registrar relationship without join tables

-- Add registrar_company_code column
ALTER TABLE ipo_list
ADD COLUMN IF NOT EXISTS registrar_company_code VARCHAR(100);

-- Add registrar_id column (for future use with registrar reference table)
ALTER TABLE ipo_list
ADD COLUMN IF NOT EXISTS registrar_id VARCHAR(100);

-- Add is_fetched flag for tracking fetch status
ALTER TABLE ipo_list
ADD COLUMN IF NOT EXISTS is_fetched BOOLEAN DEFAULT false;

-- Create index for common registrar lookups
CREATE INDEX IF NOT EXISTS idx_ipo_list_registrar_company_code 
ON ipo_list(registrar_company_code) WHERE registrar_company_code IS NOT NULL;

-- Create index for registrar_id lookups
CREATE INDEX IF NOT EXISTS idx_ipo_list_registrar_id 
ON ipo_list(registrar_id) WHERE registrar_id IS NOT NULL;

-- Add comment to document the columns
COMMENT ON COLUMN ipo_list.registrar_company_code IS 'Company code for the registrar handling this IPO allotment';
COMMENT ON COLUMN ipo_list.registrar_id IS 'Identifier for the registrar (for future reference table support)';
COMMENT ON COLUMN ipo_list.is_fetched IS 'Flag indicating if registrar data has been fetched for this IPO';

-- +goose Down
DROP INDEX IF EXISTS idx_ipo_list_registrar_id;
DROP INDEX IF EXISTS idx_ipo_list_registrar_company_code;
ALTER TABLE ipo_list DROP COLUMN IF EXISTS is_fetched;
ALTER TABLE ipo_list DROP COLUMN IF EXISTS registrar_id;
ALTER TABLE ipo_list DROP COLUMN IF EXISTS registrar_company_code;
