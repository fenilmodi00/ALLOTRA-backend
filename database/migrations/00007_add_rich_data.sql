-- +goose Up
ALTER TABLE ipo_list
    ADD COLUMN IF NOT EXISTS rich_data JSONB DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE ipo_list
    DROP COLUMN IF EXISTS rich_data;
