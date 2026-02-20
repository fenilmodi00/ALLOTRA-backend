-- +goose Up
-- Baseline marker migration.
--
-- The existing application already applies database/schema.sql at startup for
-- bootstrap schema creation. This migration is a version anchor for goose so
-- subsequent incremental migrations can be tracked consistently.

-- +goose Down
-- No-op baseline rollback.
