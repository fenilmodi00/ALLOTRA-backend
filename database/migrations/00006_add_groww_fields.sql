-- +goose Up
-- Migration: Add Groww Fields
-- Description: Add JSON fields for richer Groww data
-- Date: 2026-02-22

ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS financials JSONB DEFAULT '[]'::jsonb;
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS categories JSONB DEFAULT '[]'::jsonb;
ALTER TABLE ipo_list ADD COLUMN IF NOT EXISTS faqs JSONB DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE ipo_list DROP COLUMN IF EXISTS financials;
ALTER TABLE ipo_list DROP COLUMN IF EXISTS categories;
ALTER TABLE ipo_list DROP COLUMN IF EXISTS faqs;
