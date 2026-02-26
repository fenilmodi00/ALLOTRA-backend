-- +goose Up
ALTER TABLE ipo_list
    ADD COLUMN IF NOT EXISTS groww_details JSONB DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE ipo_list
    DROP COLUMN IF EXISTS groww_details;
