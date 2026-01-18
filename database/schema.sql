-- Database Schema for IPO Backend System
-- This schema is designed to align with the simplified IPO scraper implementation
-- and supports all data structures extracted by ChittorgarhIPOScrapingService

-- Enable required PostgreSQL extensions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Main IPO table storing all IPO information
CREATE TABLE ipo_list (
    -- Primary identification
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stock_id VARCHAR(100) NOT NULL UNIQUE,
    
    -- Basic Information (from IPOBasicInformation)
    name VARCHAR(255) NOT NULL,
    company_code VARCHAR(50) NOT NULL UNIQUE,
    symbol VARCHAR(50),
    registrar VARCHAR(255) NOT NULL,
    
    -- Date Information (from IPODateInformation)
    open_date TIMESTAMP,
    close_date TIMESTAMP,
    result_date TIMESTAMP,
    listing_date TIMESTAMP,
    
    -- Pricing Information (from IPOPricingInformation)
    price_band_low DECIMAL(10, 2),
    price_band_high DECIMAL(10, 2),
    issue_size VARCHAR(100),
    min_qty INTEGER,
    min_amount INTEGER,
    
    -- Status Information (from IPOStatusInformation)
    status VARCHAR(50) NOT NULL DEFAULT 'Unknown',
    subscription_status VARCHAR(100),
    listing_gain VARCHAR(50),
    
    -- Additional metadata
    logo_url VARCHAR(500),
    description TEXT,
    about TEXT,
    slug VARCHAR(255),
    
    -- Legacy form fields (kept for API compatibility)
    form_url VARCHAR(500),
    form_fields JSONB DEFAULT '{}',
    form_headers JSONB DEFAULT '{}',
    parser_config JSONB DEFAULT '{}',
    
    -- Additional structured data
    strengths JSONB DEFAULT '[]',
    risks JSONB DEFAULT '[]',
    
    -- Audit fields
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(100)
);

-- Add constraints for essential fields
ALTER TABLE ipo_list ADD CONSTRAINT ipo_list_name_not_empty CHECK (name != '');
ALTER TABLE ipo_list ADD CONSTRAINT ipo_list_company_code_not_empty CHECK (company_code != '');
ALTER TABLE ipo_list ADD CONSTRAINT ipo_list_registrar_not_empty CHECK (registrar != '');
ALTER TABLE ipo_list ADD CONSTRAINT ipo_list_status_not_empty CHECK (status != '');

-- Add date validation constraints
ALTER TABLE ipo_list ADD CONSTRAINT ipo_list_date_logic CHECK (
    (open_date IS NULL OR close_date IS NULL OR open_date <= close_date) AND
    (close_date IS NULL OR result_date IS NULL OR close_date <= result_date) AND
    (result_date IS NULL OR listing_date IS NULL OR result_date <= listing_date)
);

-- Add price band validation
ALTER TABLE ipo_list ADD CONSTRAINT ipo_list_price_band_logic CHECK (
    (price_band_low IS NULL OR price_band_high IS NULL OR price_band_low <= price_band_high) AND
    (price_band_low IS NULL OR price_band_low >= 0) AND
    (price_band_high IS NULL OR price_band_high >= 0)
);

-- Add quantity and amount validation
ALTER TABLE ipo_list ADD CONSTRAINT ipo_list_min_qty_positive CHECK (min_qty IS NULL OR min_qty > 0);
ALTER TABLE ipo_list ADD CONSTRAINT ipo_list_min_amount_positive CHECK (min_amount IS NULL OR min_amount > 0);

-- Performance Indexes for optimized query performance
-- These indexes are designed for common query patterns in the IPO backend

-- Primary lookup indexes
CREATE INDEX idx_ipo_stock_id ON ipo_list(stock_id);
CREATE INDEX idx_ipo_company_code ON ipo_list(company_code);
CREATE INDEX idx_ipo_symbol ON ipo_list(symbol) WHERE symbol IS NOT NULL;

-- Status and filtering indexes
CREATE INDEX idx_ipo_status ON ipo_list(status);
CREATE INDEX idx_ipo_status_dates ON ipo_list(status, open_date, close_date);

-- Date-based queries (partial indexes for non-null dates)
CREATE INDEX idx_ipo_open_date ON ipo_list(open_date) WHERE open_date IS NOT NULL;
CREATE INDEX idx_ipo_close_date ON ipo_list(close_date) WHERE close_date IS NOT NULL;
CREATE INDEX idx_ipo_result_date ON ipo_list(result_date) WHERE result_date IS NOT NULL;
CREATE INDEX idx_ipo_listing_date ON ipo_list(listing_date) WHERE listing_date IS NOT NULL;

-- Registrar-based queries
CREATE INDEX idx_ipo_registrar ON ipo_list(registrar);

-- Composite index for API queries (status with creation date for pagination)
CREATE INDEX idx_ipo_list_api ON ipo_list(status, created_at DESC);

-- Composite index for date range queries
CREATE INDEX idx_ipo_date_range ON ipo_list(open_date, close_date) WHERE open_date IS NOT NULL AND close_date IS NOT NULL;

-- Index for pricing queries
CREATE INDEX idx_ipo_price_band ON ipo_list(price_band_low, price_band_high) WHERE price_band_low IS NOT NULL AND price_band_high IS NOT NULL;

-- Index for subscription status filtering
CREATE INDEX idx_ipo_subscription_status ON ipo_list(subscription_status) WHERE subscription_status IS NOT NULL;

-- Full-text search index for company names (using GIN for better text search performance)
CREATE INDEX idx_ipo_name_gin ON ipo_list USING gin(to_tsvector('english', name));

-- Index for recent IPOs (commonly queried)
CREATE INDEX idx_ipo_recent ON ipo_list(created_at DESC, status) WHERE created_at >= CURRENT_DATE - INTERVAL '1 year';

-- Supporting Tables

-- IPO Grey Market Premium (GMP) data table
CREATE TABLE ipo_gmp (
    id VARCHAR(100) PRIMARY KEY,
    ipo_name VARCHAR(255) NOT NULL UNIQUE,
    company_code VARCHAR(50) NOT NULL,
    ipo_price DECIMAL(10, 2) NOT NULL,
    gmp_value DECIMAL(10, 2) NOT NULL,
    estimated_listing DECIMAL(10, 2) NOT NULL,
    gain_percent DECIMAL(10, 2) NOT NULL,
    sub2 DECIMAL(10, 2) DEFAULT 0,
    kostak DECIMAL(10, 2) DEFAULT 0,
    listing_date TIMESTAMP,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Enhanced GMP columns for API compatibility
    stock_id VARCHAR(100),
    subscription_status VARCHAR(100),
    listing_gain VARCHAR(50),
    rating INTEGER,
    updated_on VARCHAR(200),
    ipo_status VARCHAR(50),
    data_source VARCHAR(100) DEFAULT 'investorgain.com',
    extraction_metadata JSONB DEFAULT '{}'
);

-- Add constraints for GMP table
ALTER TABLE ipo_gmp ADD CONSTRAINT ipo_gmp_ipo_name_not_empty CHECK (ipo_name != '');
ALTER TABLE ipo_gmp ADD CONSTRAINT ipo_gmp_company_code_not_empty CHECK (company_code != '');
ALTER TABLE ipo_gmp ADD CONSTRAINT ipo_gmp_ipo_price_positive CHECK (ipo_price >= 0);

-- IPO Result Cache table for storing allotment check results
CREATE TABLE ipo_result_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pan_hash VARCHAR(255) NOT NULL,
    ipo_id UUID NOT NULL,
    status VARCHAR(100) NOT NULL,
    shares_allotted INTEGER DEFAULT 0,
    application_number VARCHAR(100),
    refund_status VARCHAR(100),
    source VARCHAR(100),
    user_agent TEXT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    confidence_score INTEGER DEFAULT 0,
    duplicate_count INTEGER DEFAULT 0,
    
    -- Foreign key constraint to ipo_list table
    CONSTRAINT fk_ipo_result_cache_ipo_id FOREIGN KEY (ipo_id) REFERENCES ipo_list(id) ON DELETE CASCADE
);

-- Add constraints for result cache table
ALTER TABLE ipo_result_cache ADD CONSTRAINT ipo_result_cache_pan_hash_not_empty CHECK (pan_hash != '');
ALTER TABLE ipo_result_cache ADD CONSTRAINT ipo_result_cache_status_not_empty CHECK (status != '');
ALTER TABLE ipo_result_cache ADD CONSTRAINT ipo_result_cache_shares_allotted_non_negative CHECK (shares_allotted >= 0);
ALTER TABLE ipo_result_cache ADD CONSTRAINT ipo_result_cache_confidence_score_range CHECK (confidence_score >= 0 AND confidence_score <= 100);
ALTER TABLE ipo_result_cache ADD CONSTRAINT ipo_result_cache_duplicate_count_non_negative CHECK (duplicate_count >= 0);
ALTER TABLE ipo_result_cache ADD CONSTRAINT ipo_result_cache_expires_after_timestamp CHECK (expires_at > timestamp);

-- IPO Update Log table for audit trail
CREATE TABLE ipo_update_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ipo_id UUID NOT NULL,
    field_name VARCHAR(100) NOT NULL,
    old_value TEXT,
    new_value TEXT,
    source VARCHAR(100),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Foreign key constraint to ipo_list table
    CONSTRAINT fk_ipo_update_log_ipo_id FOREIGN KEY (ipo_id) REFERENCES ipo_list(id) ON DELETE CASCADE
);

-- Add constraints for update log table
ALTER TABLE ipo_update_log ADD CONSTRAINT ipo_update_log_field_name_not_empty CHECK (field_name != '');

-- Indexes for supporting tables

-- GMP table indexes
CREATE INDEX idx_ipo_gmp_company_code ON ipo_gmp(company_code);
CREATE INDEX idx_ipo_gmp_ipo_name ON ipo_gmp(ipo_name);
CREATE INDEX idx_ipo_gmp_last_updated ON ipo_gmp(last_updated DESC);
CREATE INDEX idx_ipo_gmp_listing_date ON ipo_gmp(listing_date) WHERE listing_date IS NOT NULL;
CREATE INDEX idx_ipo_gmp_stock_id ON ipo_gmp(stock_id) WHERE stock_id IS NOT NULL;
CREATE INDEX idx_ipo_gmp_ipo_status ON ipo_gmp(ipo_status) WHERE ipo_status IS NOT NULL;
CREATE INDEX idx_ipo_gmp_data_source ON ipo_gmp(data_source) WHERE data_source IS NOT NULL;

-- Result cache table indexes
CREATE INDEX idx_ipo_result_cache_pan_hash ON ipo_result_cache(pan_hash);
CREATE INDEX idx_ipo_result_cache_ipo_id ON ipo_result_cache(ipo_id);
CREATE INDEX idx_ipo_result_cache_expires_at ON ipo_result_cache(expires_at);
CREATE INDEX idx_ipo_result_cache_timestamp ON ipo_result_cache(timestamp DESC);
CREATE UNIQUE INDEX idx_ipo_result_cache_unique_check ON ipo_result_cache(pan_hash, ipo_id, application_number) WHERE application_number IS NOT NULL;
CREATE UNIQUE INDEX idx_ipo_result_cache_pan_ipo ON ipo_result_cache(pan_hash, ipo_id);

-- Update log table indexes
CREATE INDEX idx_ipo_update_log_ipo_id ON ipo_update_log(ipo_id);
CREATE INDEX idx_ipo_update_log_timestamp ON ipo_update_log(timestamp DESC);
CREATE INDEX idx_ipo_update_log_field_name ON ipo_update_log(field_name);
CREATE INDEX idx_ipo_update_log_source ON ipo_update_log(source) WHERE source IS NOT NULL;