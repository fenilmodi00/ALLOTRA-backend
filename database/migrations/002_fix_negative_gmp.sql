-- Migration: Fix Negative GMP Values
-- Description: Allows negative GMP values in gmp_price_history table
-- Date: 2026-02-21
-- Reason: Real IPO data can have negative GMP (e.g., Fractal Analytics IPO had GMP = -28)

-- Drop the existing check constraint that blocks negative GMP
ALTER TABLE gmp_price_history DROP CONSTRAINT IF EXISTS chk_gmp_history_prices_positive;

-- Add new check constraint that only validates non-negative IPO price
-- GMP can now be negative (grey market discount)
ALTER TABLE gmp_price_history ADD CONSTRAINT chk_gmp_history_ipo_price_positive CHECK (ipo_price >= 0);

-- Note: estimated_listing can also be negative if GMP is very negative
-- We'll allow it but validate in application code
