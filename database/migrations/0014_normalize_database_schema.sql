-- Migration: Normalize Database Schema to Eliminate Data Duplication
-- Date: 2026-03-06
-- Purpose: Create proper normalized structure while preserving data
-- 
-- RATIONALE:
-- 1. Current schema has massive data duplication between ipo_list columns and groww_details JSONB
-- 2. Same fields stored in multiple places: symbol, status, dates, prices, etc.
-- 3. pros/cons/strengths/risks come from DIFFERENT sources (Chittorgarh vs Groww) - should be separate
-- 4. This migration normalizes while keeping backward compatibility via views
--
-- STRATEGY:
-- - Phase 1: Add new normalized columns to ipo_list (migrate from groww_details)
-- - Phase 2: Keep old columns for compatibility, create views for app
-- - Phase 3: (Future) Drop redundant columns after full migration

-- +goose Up

-- ============================================================================
-- PHASE 1: Add normalized columns to ipo_list for data that was in groww_details
-- ============================================================================

-- Add missing columns that are currently ONLY in groww_details
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS isin VARCHAR(20);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS is_sme BOOLEAN DEFAULT false;
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS issue_type VARCHAR(10);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS face_value NUMERIC(10,2);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS tick_size INTEGER;
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS lot_size INTEGER;
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS max_bid_qty INTEGER;
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS issue_price NUMERIC(10,2);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS cut_off_price NUMERIC(10,2);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS listing_price NUMERIC(10,2);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS bse_scrip_code VARCHAR(20);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS nse_scrip_code VARCHAR(20);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS registrar_link VARCHAR(500);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS logo_url_groww VARCHAR(500);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS document_url VARCHAR(500);

-- Add columns for subscription rates (currently only in groww_details)
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS subscription_qib NUMERIC(10,2);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS subscription_nii NUMERIC(10,2);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS subscription_retail NUMERIC(10,2);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS subscription_total NUMERIC(10,2);
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS subscription_updated_at TIMESTAMP;

-- Add company info columns (from groww_details.aboutCompany)
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS year_founded INTEGER;
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS about_company TEXT;
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS managing_director VARCHAR(255);

-- Add dates for application timing
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS daily_start_time TIME;
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS daily_end_time TIME;
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS last_bid_place_time TIME;
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS allotment_announced BOOLEAN DEFAULT false;

-- ============================================================================
-- PHASE 2: Create new normalized tables for multi-source data
-- ============================================================================

-- Table to store pros/cons from DIFFERENT sources (Chittorgarh vs Groww)
-- This solves the issue where both sources have pros/cons but DIFFERENT content
CREATE TABLE IF NOT EXISTS ipo_analysis_points (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ipo_id UUID NOT NULL REFERENCES ipo_list(id) ON DELETE CASCADE,
    source VARCHAR(50) NOT NULL, -- 'chittorgarh' or 'groww'
    point_type VARCHAR(20) NOT NULL, -- 'strength' or 'risk'
    content TEXT NOT NULL,
    display_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_ipo_analysis_source_type UNIQUE (ipo_id, source, point_type, display_order)
);

-- Table to store FAQs from DIFFERENT sources
CREATE TABLE IF NOT EXISTS ipo_faqs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ipo_id UUID NOT NULL REFERENCES ipo_list(id) ON DELETE CASCADE,
    source VARCHAR(50) NOT NULL, -- 'chittorgarh' or 'groww'
    question TEXT NOT NULL,
    answer TEXT NOT NULL,
    display_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_ipo_faq_source_order UNIQUE (ipo_id, source, display_order)
);

-- Table to store financials from DIFFERENT sources
CREATE TABLE IF NOT EXISTS ipo_financials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ipo_id UUID NOT NULL REFERENCES ipo_list(id) ON DELETE CASCADE,
    source VARCHAR(50) NOT NULL, -- 'chittorgarh' or 'groww'
    title VARCHAR(100) NOT NULL, -- 'Revenue', 'Profit', etc.
    year VARCHAR(10) NOT NULL, -- '2023', '2024', etc.
    value NUMERIC(15,2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_ipo_financial_source_year UNIQUE (ipo_id, source, title, year)
);

-- Create indexes for new tables
CREATE INDEX IF NOT EXISTS idx_ipo_analysis_ipo_id ON ipo_analysis_points(ipo_id);
CREATE INDEX IF NOT EXISTS idx_ipo_analysis_source ON ipo_analysis_points(source);
CREATE INDEX IF NOT EXISTS idx_ipo_faqs_ipo_id ON ipo_faqs(ipo_id);
CREATE INDEX IF NOT EXISTS idx_ipo_faqs_source ON ipo_faqs(source);
CREATE INDEX IF NOT EXISTS idx_ipo_financials_ipo_id ON ipo_financials(ipo_id);
CREATE INDEX IF NOT EXISTS idx_ipo_financials_source ON ipo_financials(source);

-- ============================================================================
-- PHASE 3: Create Views for backward compatibility
-- ============================================================================

-- View that combines ipo_list with relevant groww_details fields (flattened)
-- This keeps the app working without major code changes
DROP VIEW IF EXISTS v_ipo_list_enriched;
CREATE VIEW v_ipo_list_enriched AS
SELECT 
    il.*,
    -- Flatten groww_details fields (these were duplicated)
    (il.groww_details->>'symbol')::VARCHAR(50) AS groww_symbol,
    (il.groww_details->>'isin')::VARCHAR(20) AS groww_isin,
    (il.groww_details->>'isSme')::BOOLEAN AS groww_is_sme,
    (il.groww_details->>'issueType')::VARCHAR(10) AS groww_issue_type,
    (il.groww_details->>'faceValue')::NUMERIC(10,2) AS groww_face_value,
    (il.groww_details->>'tickSize')::INTEGER AS groww_tick_size,
    (il.groww_details->>'lotSize')::INTEGER AS groww_lot_size,
    (il.groww_details->>'minBidQty')::INTEGER AS groww_min_bid_qty,
    (il.groww_details->>'maxPrice')::NUMERIC(10,2) AS groww_max_price,
    (il.groww_details->>'minPrice')::NUMERIC(10,2) AS groww_min_price,
    (il.groww_details->>'issuePrice')::NUMERIC(10,2) AS groww_issue_price,
    (il.groww_details->>'cutOffPrice')::NUMERIC(10,2) AS groww_cut_off_price,
    (il.groww_details->>'listingPrice')::NUMERIC(10,2) AS groww_listing_price,
    (il.groww_details->>'bseScripCode')::VARCHAR(20) AS groww_bse_scrip_code,
    (il.groww_details->>'nseScripCode')::VARCHAR(20) AS groww_nse_scrip_code,
    (il.groww_details->>'rtaLink')::VARCHAR(500) AS groww_rta_link,
    (il.groww_details->>'logoUrl')::VARCHAR(500) AS groww_logo_url,
    (il.groww_details->>'documentUrl')::VARCHAR(500) AS groww_document_url,
    -- Subscription rates from groww_details->subscriptionRates (prefixed to avoid collision with real columns)
    (il.groww_details->'subscriptionRates'->0->>'subscriptionRate')::NUMERIC(10,2) AS groww_subscription_qib,
    (il.groww_details->'subscriptionRates'->1->>'subscriptionRate')::NUMERIC(10,2) AS groww_subscription_nii,
    (il.groww_details->'subscriptionRates'->2->>'subscriptionRate')::NUMERIC(10,2) AS groww_subscription_retail,
    (il.groww_details->'subscriptionRates'->3->>'subscriptionRate')::NUMERIC(10,2) AS groww_subscription_total,
    -- About company (prefixed to avoid collision with real columns)
    (il.groww_details->'aboutCompany'->>'yearFounded')::INTEGER AS groww_year_founded,
    (il.groww_details->'aboutCompany'->>'aboutCompany')::TEXT AS groww_about_company,
    (il.groww_details->'aboutCompany'->>'managingDirector')::VARCHAR(255) AS groww_managing_director,
    -- Timing (prefixed to avoid collision with real columns)
    (il.groww_details->>'dailyStartTime')::TIME AS groww_daily_start_time,
    (il.groww_details->>'dailyEndTime')::TIME AS groww_daily_end_time,
    (il.groww_details->>'lastBidPlaceTime')::TIME AS groww_last_bid_place_time,
    (il.groww_details->>'isAllotmentAnnounced')::BOOLEAN AS groww_allotment_announced,
    (il.groww_details->>'subscriptionUpdatedAt')::TIMESTAMP AS groww_subscription_updated_at,
    (il.groww_details->>'allotmentDate')::TIMESTAMP AS groww_allotment_date
FROM ipo_list il;

-- View for GMP data with IPO details joined
DROP VIEW IF EXISTS v_ipo_gmp_with_details;
CREATE VIEW v_ipo_gmp_with_details AS
SELECT
    ig.*,
    il.name AS ipo_list_name,
    il.symbol AS ipo_list_symbol,
    il.status AS ipo_list_status,
    il.open_date AS ipo_list_open_date,
    il.close_date AS ipo_list_close_date,
    il.listing_date AS ipo_list_listing_date
FROM ipo_gmp ig
LEFT JOIN ipo_list il ON ig.ipo_id = il.id;

-- ============================================================================
-- PHASE 4: Data Migration Functions (Optional - for future use)
-- ============================================================================

-- Function to migrate groww_details to normalized columns
-- Run this manually if you want to move data from JSONB to columns
CREATE OR REPLACE FUNCTION migrate_groww_details_to_columns()
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE ipo_list SET
        isin = (groww_details->>'isin')::VARCHAR(20),
        is_sme = (groww_details->>'isSme')::BOOLEAN,
        issue_type = (groww_details->>'issueType')::VARCHAR(10),
        face_value = (groww_details->>'faceValue')::NUMERIC(10,2),
        tick_size = (groww_details->>'tickSize')::INTEGER,
        lot_size = (groww_details->>'lotSize')::INTEGER,
        max_bid_qty = (groww_details->>'minBidQty')::INTEGER,
        issue_price = (groww_details->>'issuePrice')::NUMERIC(10,2),
        cut_off_price = (groww_details->>'cutOffPrice')::NUMERIC(10,2),
        listing_price = (groww_details->'listing'->>'listingPrice')::NUMERIC(10,2),
        bse_scrip_code = (groww_details->'listing'->>'bseScripCode')::VARCHAR(20),
        nse_scrip_code = (groww_details->'listing'->>'nseScripCode')::VARCHAR(20),
        registrar_link = (groww_details->>'rtaLink')::VARCHAR(500),
        logo_url_groww = (groww_details->>'logoUrl')::VARCHAR(500),
        document_url = (groww_details->>'documentUrl')::VARCHAR(500),
        year_founded = (groww_details->'aboutCompany'->>'yearFounded')::INTEGER,
        about_company = (groww_details->'aboutCompany'->>'aboutCompany')::TEXT,
        managing_director = (groww_details->'aboutCompany'->>'managingDirector')::VARCHAR(255),
        daily_start_time = (groww_details->>'dailyStartTime')::TIME,
        daily_end_time = (groww_details->>'dailyEndTime')::TIME,
        last_bid_place_time = (groww_details->>'lastBidPlaceTime')::TIME,
        allotment_announced = (groww_details->>'isAllotmentAnnounced')::BOOLEAN,
        subscription_updated_at = (groww_details->>'subscriptionUpdatedAt')::TIMESTAMP
    WHERE groww_details IS NOT NULL AND groww_details::text != '{}';
    
    -- Update subscription rates from the array
    UPDATE ipo_list SET
        subscription_qib = (groww_details->'subscriptionRates'->0->>'subscriptionRate')::NUMERIC(10,2),
        subscription_nii = (groww_details->'subscriptionRates'->1->>'subscriptionRate')::NUMERIC(10,2),
        subscription_retail = (groww_details->'subscriptionRates'->2->>'subscriptionRate')::NUMERIC(10,2),
        subscription_total = (groww_details->'subscriptionRates'->3->>'subscriptionRate')::NUMERIC(10,2)
    WHERE groww_details->'subscriptionRates' IS NOT NULL;
END;
$$;

-- ============================================================================
-- PHASE 5: RLS for new tables
-- ============================================================================

ALTER TABLE ipo_analysis_points ENABLE ROW LEVEL SECURITY;
ALTER TABLE ipo_faqs ENABLE ROW LEVEL SECURITY;
ALTER TABLE ipo_financials ENABLE ROW LEVEL SECURITY;

-- Public read access
CREATE POLICY "Public can view analysis points" ON ipo_analysis_points FOR SELECT USING (true);
CREATE POLICY "Public can view FAQs" ON ipo_faqs FOR SELECT USING (true);
CREATE POLICY "Public can view financials" ON ipo_financials FOR SELECT USING (true);

-- Service role full access
CREATE POLICY "Service role full access analysis" ON ipo_analysis_points FOR ALL USING ((SELECT auth.role() = 'service_role'));
CREATE POLICY "Service role full access faqs" ON ipo_faqs FOR ALL USING ((SELECT auth.role() = 'service_role'));
CREATE POLICY "Service role full access financials" ON ipo_financials FOR ALL USING ((SELECT auth.role() = 'service_role'));

-- Analyze tables
ANALYZE ipo_list;
ANALYZE ipo_analysis_points;
ANALYZE ipo_faqs;
ANALYZE ipo_financials;

-- +goose Down
-- No rollback needed (schema already applied directly to Supabase via MCP)
