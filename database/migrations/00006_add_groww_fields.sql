-- Add new JSON fields for richer Groww data
ALTER TABLE ipo_list ADD COLUMN financials JSONB DEFAULT '[]'::jsonb;
ALTER TABLE ipo_list ADD COLUMN categories JSONB DEFAULT '[]'::jsonb;
ALTER TABLE ipo_list ADD COLUMN faqs JSONB DEFAULT '[]'::jsonb;
