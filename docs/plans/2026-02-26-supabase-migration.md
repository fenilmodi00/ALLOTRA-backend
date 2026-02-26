# Supabase Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate IPO Backend from self-hosted Docker PostgreSQL to Supabase (PostgreSQL + Auth + Storage + Edge Functions) while keeping Go scrapers and backend logic unchanged.

**Architecture:** Hybrid approach - Keep Go for scraping/business logic, use Supabase for database, authentication, storage, and lightweight APIs. This leverages Go's strength for heavy scraping while benefiting from Supabase's managed infrastructure.

**Tech Stack:** Go 1.21, PostgreSQL (Supabase), Supabase Auth, Supabase Storage, Supabase Edge Functions (Deno), Docker

---

## Current State Analysis

### Supabase Project Status
- **Project URL:** https://bfirenjerddoyqsihytp.supabase.co
- **Database:** Empty (no tables, no data)
- **Storage:** Empty (no buckets)
- **Extensions:** 80+ available including pg_cron, pg_graphql, vector, postgis, pgmq

### Current Architecture
- **Backend:** Go 1.21 (Docker container)
- **Database:** Self-hosted PostgreSQL 15 (Docker)
- **Port:** 5432 (local), 8080 (API)

### Files to Modify
1. `.env` - DATABASE_URL
2. `config/config.go` - Database config
3. `database/postgres.go` - Connection logic
4. `docker-compose.yml` - Remove local DB (keep app only)
5. `docker-compose.prod.yml` - Update for Supabase

---

## Phase 1: Schema Migration (Supabase Setup)

### Task 1: Apply Database Schema to Supabase

**Files:**
- Modify: `database/schema.sql`

**Step 1: Apply schema migration to Supabase**

Run: Use Supabase MCP apply_migration
```sql
-- Apply the full schema from database/schema.sql
-- This creates tables: ipo_list, ipo_gmp, ipo_result_cache, ipo_update_log, gmp_price_history, gmp_history_job_log
```

**Step 2: Verify tables created**

Run: supabase_list_tables
Expected: 6 tables visible

**Step 3: Commit**

```bash
git add database/schema.sql
git commit -m "chore: prepare schema for Supabase migration"
```

---

### Task 2: Configure Supabase Row Level Security (RLS)

**Files:**
- Create: `database/supabase_rls.sql`

**Step 1: Write RLS policy migration**

```sql
-- Enable RLS on all tables
ALTER TABLE ipo_list ENABLE ROW LEVEL SECURITY;
ALTER TABLE ipo_gmp ENABLE ROW LEVEL SECURITY;
ALTER TABLE ipo_result_cache ENABLE ROW LEVEL SECURITY;
ALTER TABLE ipo_update_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE gmp_price_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE gmp_history_job_log ENABLE ROW LEVEL SECURITY;

-- Public read access for IPO data (read-only)
CREATE POLICY "Public can view IPO list" ON ipo_list FOR SELECT USING (true);
CREATE POLICY "Public can view GMP" ON ipo_gmp FOR SELECT USING (true);
CREATE POLICY "Public can view GMP history" ON gmp_price_history FOR SELECT USING (true);

-- Authenticated users can insert/update their own data
CREATE POLICY "Users can insert result cache" ON ipo_result_cache FOR INSERT WITH CHECK (auth.uid() IS NOT NULL);
CREATE POLICY "Users can update own result cache" ON ipo_result_cache FOR UPDATE USING (auth.uid() IS NOT NULL);

-- Service role can do anything (for Go backend)
CREATE POLICY "Service role full access" ON ipo_list FOR ALL USING (auth.role() = 'service_role');
CREATE POLICY "Service role full access GMP" ON ipo_gmp FOR ALL USING (auth.role() = 'service_role');
```

**Step 2: Apply RLS migration to Supabase**

Run: supabase_apply_migration with name "add_rls_policies"

**Step 3: Commit**

```bash
git add database/supabase_rls.sql
git commit -m "chore: add Supabase RLS policies"
```

---

## Phase 2: Go Backend Configuration

### Task 3: Update Database Connection for Supabase

**Files:**
- Modify: `.env`
- Modify: `config/config.go:66-82`
- Modify: `database/postgres.go:18-58`

**Step 1: Update .env file**

Modify `.env`:
```
# Old (Docker)
# DATABASE_URL=postgres://user:password@localhost:5432/ipo_db

# New (Supabase)
DATABASE_URL=postgres://postgres:RsCcdtSEFyGdJMG0@db.bfirenjerddoyqsihytp.supabase.co:5432/postgres
```

**Step 2: Test connection to Supabase**

Run: Test locally with Go app connecting to new DATABASE_URL

**Step 3: Commit**

```bash
git add .env
git commit -m "chore: update DATABASE_URL to Supabase"
```

---

### Task 4: Add Supabase Auth Client to Go

**Files:**
- Create: `services/supabase_auth.go`
- Modify: `go.mod`

**Step 1: Add Supabase Go client dependency**

Run:
```bash
go get github.com/supabase-community/supabase-go
```

**Step 2: Write Supabase auth service**

```go
package services

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/supabase-community/supabase-go"
)

type SupabaseAuth struct {
	client *supabase.Client
}

func NewSupabaseAuth() (*SupabaseAuth, error) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_KEY") // Service role key for backend

	if supabaseURL == "" || supabaseKey == "" {
		return nil, fmt.Errorf("SUPABASE_URL and SUPABASE_SERVICE_KEY required")
	}

	client := supabase.NewClient(supabaseURL, supabaseKey)
	return &SupabaseAuth{client: client}, nil
}

func (s *SupabaseAuth) VerifyToken(ctx context.Context, token string) (*User, error) {
	user, err := s.client.Auth.GetUser(token)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}
	return &User{
		ID:    user.ID,
		Email: user.Email,
	}, nil
}

type User struct {
	ID    string
	Email string
}
```

**Step 3: Commit**

```bash
git add services/supabase_auth.go go.mod go.sum
git commit -m "feat: add Supabase auth client for Go"
```

---

## Phase 3: Storage Migration

### Task 5: Create Supabase Storage Bucket

**Files:**
- Modify: Supabase (via MCP)

**Step 1: Create storage bucket via Supabase dashboard or API**

Create bucket: `ipo-logos` for storing IPO logos

**Step 2: Add storage config to Go**

Create `services/storage.go`:
```go
package services

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/supabase-community/supabase-go"
)

type StorageService struct {
	client   *supabase.Client
	bucket   string
}

func NewStorageService() (*StorageService, error) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_KEY")

	client := supabase.NewClient(supabaseURL, supabaseKey)
	return &StorageService{client: client, bucket: "ipo-logos"}, nil
}

func (s *StorageService) UploadLogo(ctx context.Context, ipoID string, data []byte, contentType string) (string, error) {
	path := fmt.Sprintf("logos/%s", ipoID)
	
	resp, err := s.client.Storage.From(s.bucket).Upload(path, data, supabase.StorageUploadOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}
	
	return resp, nil
}

func (s *StorageService) GetLogoURL(ipoID string) string {
	return fmt.Sprintf("%s/storage/v1/object/public/%s/logos/%s", 
		os.Getenv("SUPABASE_URL"), s.bucket, ipoID)
}
```

**Step 3: Commit**

```bash
git add services/storage.go
git commit -m "feat: add Supabase storage service"
```

---

## Phase 4: Edge Functions (Optional Lightweight APIs)

### Task 6: Create Health Check Edge Function

**Files:**
- Create: `supabase/functions/health/index.ts`

**Step 1: Write Edge Function**

```typescript
import "jsr:@supabase/functions-js/edge-runtime.d.ts"

Deno.serve(async (req) => {
  const headers = { 'Content-Type': 'application/json' }
  
  try {
    // Simple health check - could connect to DB if needed
    return new Response(JSON.stringify({ 
      status: 'ok', 
      timestamp: new Date().toISOString(),
      service: 'ipo-backend-health'
    }), { headers })
  } catch (error) {
    return new Response(JSON.stringify({ 
      status: 'error', 
      error: error.message 
    }), { status: 500, headers })
  }
})
```

**Step 2: Deploy to Supabase**

Run: supabase_deploy_edge_function
- name: "health"
- verify_jwt: false

**Step 3: Commit**

```bash
git add supabase/functions/health/index.ts
git commit -m "feat: add health check edge function"
```

---

## Phase 5: Docker Configuration Update

### Task 7: Update Docker Compose for Supabase

**Files:**
- Modify: `docker-compose.yml`
- Modify: `docker-compose.prod.yml`

**Step 1: Update docker-compose.yml (development)**

```yaml
version: '3.8'
services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
      target: development
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://postgres:RsCcdtSEFyGdJMG0@db.bfirenjerddoyqsihytp.supabase.co:5432/postgres
      - SUPABASE_URL=https://bfirenjerddoyqsihytp.supabase.co
      - SUPABASE_SERVICE_KEY=${SUPABASE_SERVICE_KEY}
    volumes:
      - .:/app
```

**Step 2: Update docker-compose.prod.yml**

```yaml
version: '3.8'
services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
      target: production
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - SUPABASE_URL=${SUPABASE_URL}
      - SUPABASE_SERVICE_KEY=${SUPABASE_SERVICE_KEY}
    restart: unless-stopped
```

**Step 3: Commit**

```bash
git add docker-compose.yml docker-compose.prod.yml
git commit -m "chore: update Docker config for Supabase"
```

---

## Phase 6: Data Migration (if existing data)

### Task 8: Migrate Data from Local DB to Supabase

**Files:**
- Create: `scripts/migrate_data.sh`

**Step 1: Export data from local PostgreSQL**

```bash
# Export data as SQL insert statements
pg_dump -U user -d ipo_db --data-only > data.sql
```

**Step 2: Import to Supabase**

```bash
# Using psql with Supabase connection
psql "postgres://postgres:RsCcdtSEFyGdJMG0@db.bfirenjerddoyqsihytp.supabase.co:5432/postgres" < data.sql
```

**Step 3: Verify row counts**

Run: Check row counts match between old and new database

**Step 4: Commit**

```bash
git add scripts/migrate_data.sh
git commit -m "chore: add data migration script"
```

---

## Phase 7: Testing & Verification

### Task 9: Integration Testing

**Files:**
- Modify: Existing test files

**Step 1: Run existing tests**

```bash
go test ./... -v
```

**Step 2: Verify API endpoints**

Test:
- GET /health
- GET /api/v1/ipo/list
- GET /api/v1/gmp/{id}

**Step 3: Commit**

```bash
git commit -m "test: verify integration with Supabase"
```

---

## Summary of Changes

| File | Change |
|------|--------|
| `.env` | Update DATABASE_URL to Supabase |
| `config/config.go` | Add SUPABASE_URL, SUPABASE_SERVICE_KEY |
| `database/postgres.go` | Works with Supabase connection string |
| `docker-compose.yml` | Remove local db service |
| `docker-compose.prod.yml` | Update for Supabase env vars |
| `services/supabase_auth.go` | NEW - Auth verification |
| `services/storage.go` | NEW - File storage |
| `supabase/functions/health/` | NEW - Edge function |
| `database/supabase_rls.sql` | NEW - Security policies |

---

## Rollback Plan

If migration fails:
1. Revert `.env` DATABASE_URL to local PostgreSQL
2. Restart Docker containers
3. Local database remains intact

---

## Plan complete saved to `docs/plans/2026-02-26-supabase-migration.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

**Which approach?**
