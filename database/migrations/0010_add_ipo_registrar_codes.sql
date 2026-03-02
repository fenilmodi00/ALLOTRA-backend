-- +goose Up
-- Migration: Add IPO Registrar Codes table
-- Purpose: Store registrar short codes and company codes for IPOs to support
--          allotment registrar matching and resolution tracking
CREATE TABLE IF NOT EXISTS ipo_registrar_codes (
    -- Primary identification
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Foreign key to IPO
    ipo_id UUID NOT NULL REFERENCES ipo_list(id) ON DELETE CASCADE,
    
    -- Registrar identification
    registrar_short_code VARCHAR(20) NOT NULL,
    registrar_company_code VARCHAR(100),
    
    -- IPO matching metadata
    ipo_name VARCHAR(255),
    match_score FLOAT DEFAULT 0,
    
    -- Resolution tracking
    is_resolved BOOLEAN DEFAULT false,
    last_attempted_at TIMESTAMP,
    
    -- Audit fields
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Unique constraint to ensure one registrar code per IPO
    CONSTRAINT uq_ipo_registrar_codes_ipo_id_registrar_short_code UNIQUE (ipo_id, registrar_short_code)
);

-- Index for common lookup patterns
CREATE INDEX IF NOT EXISTS idx_ipo_registrar_codes_ipo_id_registrar_short_code 
ON ipo_registrar_codes(ipo_id, registrar_short_code);

-- Index for scheduler queries to find unresolved codes for retry
CREATE INDEX IF NOT EXISTS idx_ipo_registrar_codes_is_resolved_last_attempted_at 
ON ipo_registrar_codes(is_resolved, last_attempted_at);

-- +goose Down
DROP TABLE IF EXISTS ipo_registrar_codes;
