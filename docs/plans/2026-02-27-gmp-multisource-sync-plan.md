# Multi-Source GMP Sync Fix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix Postgres `ON CONFLICT` errors while explicitly supporting multiple data sources mapping GMP back to single IPOs.

**Architecture:** Change the uniqueness constraint in the `ipo_gmp` table from just `ipo_name` to a composite key `(ipo_id, data_source)`. This enables UPSERT statements from both `gmp_history_service.go` and `gmp_service.go` to conflict intentionally and correctly across multiple scrape sources without crashing the database.

**Tech Stack:** Go (Golang), PostgreSQL, pg_dump/sql migrations

---

### Task 1: Create Database Migration

**Files:**
- Create: `database/migrations/0o1_add_gmp_multisource_unique.sql` (Note: filename may need adapting to project's exact migration standard)

**Step 1: Write Migration Script**
We need to remove the existing constraint on `ipo_name` and add a new constraint.

```sql
-- Migration: Support multi-source GMP syncing
BEGIN;

-- Drop the unique constraint from ipo_name
ALTER TABLE ipo_gmp DROP CONSTRAINT IF EXISTS ipo_gmp_ipo_name_key;

-- Add a composite UNIQUE constraint so ON CONFLICT queries can correctly UPSERT an IPO's data per source
ALTER TABLE ipo_gmp ADD CONSTRAINT uq_ipo_gmp_source UNIQUE (ipo_id, data_source);

COMMIT;
```

**Step 2: Commit**

```bash
git add database/migrations/0o1_add_gmp_multisource_unique.sql
git commit -m "chore(db): modify ipo_gmp unique constraints for multi-source"
```

### Task 2: Update Schema Definition File

**Files:**
- Modify: `database/schema.sql`

**Step 1: Edit base schema.sql file**
Update the base initialization file so new environments are built correctly.

In `database/schema.sql`:
Around line 134, change `ipo_name VARCHAR(255) NOT NULL UNIQUE,` to `ipo_name VARCHAR(255) NOT NULL,`
Around line 164, add `ALTER TABLE ipo_gmp ADD CONSTRAINT uq_ipo_gmp_source UNIQUE (ipo_id, data_source);`

**Step 2: Commit**

```bash
git add database/schema.sql
git commit -m "chore(db): update schema.sql with multi-source constraint"
```

### Task 3: Fix UPSERT inside gmp_history_service.go

**Files:**
- Modify: `services/gmp_history_service.go`

**Step 1: Write the code changes**
Update the target of the `ON CONFLICT` clause.

In `services/gmp_history_service.go`, change `ON CONFLICT (ipo_id) DO UPDATE SET` (around line 209) to:
```sql
		ON CONFLICT (ipo_id, data_source) DO UPDATE SET
			ipo_name = EXCLUDED.ipo_name,
```
(Keeping the rest of the existing assignments: `gmp_value = EXCLUDED.gmp_value`, etc.)

**Step 2: Run a build check**
```bash
go build ./services/...
```

**Step 3: Commit**
```bash
git add services/gmp_history_service.go
git commit -m "fix(gmp): change ON CONFLICT target in history service"
```

### Task 4: Fix UPSERT inside gmp_service.go

**Files:**
- Modify: `services/gmp_service.go`

**Step 1: Write the code changes**
In `services/gmp_service.go`, change `ON CONFLICT (ipo_name) DO UPDATE SET` (around line 753) to:
```sql
		ON CONFLICT (ipo_id, data_source) DO UPDATE SET
			ipo_name = EXCLUDED.ipo_name,
```
*(Keep the rest of the assignments).* 

**Step 2: Run a build check**
```bash
go build ./services/...
```

**Step 3: Commit**
```bash
git add services/gmp_service.go
git commit -m "fix(gmp): align ON CONFLICT target across all services"
```
