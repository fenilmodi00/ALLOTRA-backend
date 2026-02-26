# Multi-Source GMP Sync Design

**Goal:** Fix Postgres `ON CONFLICT` errors while explicitly supporting multiple data sources tracking GMP for the same IPOs.

**Context:** The `gmp_history_service.go` and `gmp_service.go` perform UPSERTs to `ipo_gmp` using `ON CONFLICT (ipo_id)` and `ON CONFLICT (ipo_name)` respectively. Both are fragile because `ipo_id` uniquely identifies the IPO but lacks a unique constraint in the table, whereas `ipo_name` has a unique constraint but shouldn't be the unique identifier, especially in a multi-source ecosystem.

## Architecture
We migrate from an implicitly single-sourced model (where IPO Name dictates table uniqueness) to an explicitly multi-source model. The combination of `ipo_id` and `data_source` will form the precise unique identifying key for any given state of GMP.

## Database Changes
We will create a new SQL migration script to update the `ipo_gmp` table:
1. `ALTER TABLE ipo_gmp DROP CONSTRAINT ipo_...;` (Need to find the specific name of the constraint for `ipo_name`)
2. `ALTER TABLE ipo_gmp ADD CONSTRAINT uq_ipo_gmp_source UNIQUE (ipo_id, data_source);`

## Application Code Changes
Update `gmp_history_service.go` and `gmp_service.go`:
1. Change `ON CONFLICT (...)` targets to `ON CONFLICT (ipo_id, data_source)`.
2. Ensure the `UPDATE SET` clause includes `ipo_name = EXCLUDED.ipo_name` since it's no longer the pivot anchor and should be kept updated.
