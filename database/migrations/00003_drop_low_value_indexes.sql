-- +goose Up
DROP INDEX IF EXISTS idx_ipo_gmp_ipo_name;
DROP INDEX IF EXISTS idx_ipo_gmp_data_source;
DROP INDEX IF EXISTS idx_ipo_update_log_field_name;

-- +goose Down
CREATE INDEX IF NOT EXISTS idx_ipo_gmp_ipo_name ON ipo_gmp(ipo_name);
CREATE INDEX IF NOT EXISTS idx_ipo_gmp_data_source ON ipo_gmp(data_source) WHERE data_source IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ipo_update_log_field_name ON ipo_update_log(field_name);
