# Supabase Features Implementation Plan

> **For Claude:** Use superpowers:executing-plans to implement recommended features.

**Goal:** Leverage Supabase features to reduce infrastructure complexity while keeping Go scrapers efficient.

**Architecture:** Hybrid - Keep Go for heavy scraping, use Supabase for managed infrastructure (DB, Auth, Jobs, Storage).

**Tech Stack:** Go 1.21, Supabase (PostgreSQL + pg_cron + Auth + Storage + Edge Functions), Docker

---

## Recommendation Matrix

| Feature | Use Case | Keep in Go? | Effort | Priority |
|---------|----------|-------------|--------|----------|
| **Database** | All data storage | ❌ | Done | ✅ |
| **pg_cron** | Job scheduling | ❌ Yes | Low | 🔴 HIGH |
| **Edge Functions** | Lightweight APIs | ❌ Yes | Medium | 🟡 MEDIUM |
| **Auth** | User management | ❌ Yes | Medium | 🟡 MEDIUM |
| **Storage** | IPO logos | ❌ Yes | Low | 🟢 LOW |
| **Realtime** | Live GMP updates | N/A | High | ⚪ FUTURE |

---

## Recommended Implementation Order

### Phase 1: Job Scheduling with pg_cron (HIGHEST PRIORITY)

**Why:** Replace cron/Docker scheduling with database-native scheduling.

**Your current setup:**
- `jobs/daily_ipo_update.go` - runs in Docker container
- `jobs/gmp_history_update_job.go` - runs on schedule

**Supabase solution:**
```sql
-- Schedule daily IPO update at 6 AM IST
SELECT cron.schedule('daily-ipo-update', '0 0 6 * *', 
  'INSERT INTO gmp_history_job_log (job_start_time, execution_status) VALUES (NOW(), ''pending'')'
);
```

**What stays in Go:** The actual scraping logic
**What moves to Supabase:** The trigger/schedule

---

### Phase 2: Auth for User Data (MEDIUM PRIORITY)

**Why:** Secure user-specific allotment checks.

**Your current:** Anyone can check any PAN number
**Supabase:** Users must login, own their allotment checks

**Implementation:**
1. Enable Supabase Auth
2. Add `user_id` to `ipo_result_cache` table
3. Update handlers to verify JWT

---

### Phase 3: Edge Functions for Public APIs (MEDIUM PRIORITY)

**Why:** Reduce Go server load for read-only endpoints.

**Candidates for Edge Functions:**
- `GET /health` - already planned
- `GET /api/v1/ipo/list` - public IPO list
- `GET /api/v1/gmp` - public GMP data

**What stays in Go:** 
- All write operations (scraper results)
- Complex business logic
- Auth-protected endpoints

---

### Phase 4: Storage for IPO Assets (LOW PRIORITY)

**Why:** Already have Go service ready.

**Status:** `services/storage.go` created ✅

---

## Detailed Task List

### Task 1: Enable pg_cron & Create Job Schedule

**Files:**
- Modify: Supabase (via MCP)

**Step 1: Enable pg_cron extension**

```sql
CREATE EXTENSION IF NOT EXISTS pg_cron;
GRANT USAGE ON SCHEMA cron TO postgres;
```

**Step 2: Create job trigger table**

```sql
CREATE TABLE scheduled_job_triggers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_name VARCHAR(100) NOT NULL UNIQUE,
    schedule VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    last_run TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Step 3: Insert job schedules**

```sql
INSERT INTO scheduled_job_triggers (job_name, schedule) VALUES
    ('daily-ipo-update', '0 0 6 * *'),      -- 6 AM daily
    ('gmp-history-update', '0 */4 * * *'), -- Every 4 hours
    ('cache-cleanup', '0 2 * * *');         -- 2 AM daily
```

**Step 4: Commit**

```bash
git add database/supabase_pg_cron.sql
git commit -m "feat: add pg_cron job scheduling"
```

---

### Task 2: Setup Supabase Auth

**Files:**
- Modify: `services/supabase_auth.go` (already exists)
- Modify: `.env`

**Step 1: Get auth keys from Supabase dashboard**

- SUPABASE_URL: https://bfirenjerddoyqsihytp.supabase.co
- Add anon key to .env

**Step 2: Update ipo_result_cache for user ownership**

```sql
ALTER TABLE ipo_result_cache ADD COLUMN user_id UUID REFERENCES auth.users(id);
CREATE INDEX idx_ipo_result_cache_user_id ON ipo_result_cache(user_id);
```

**Step 3: Commit**

```bash
git commit -m "feat: add user_id to ipo_result_cache for auth"
```

---

### Task 3: Deploy Public Edge Functions

**Files:**
- Create: `supabase/functions/ipo-list/index.ts`
- Create: `supabase/functions/gmp/index.ts`

**Step 1: Write Edge Function for IPO list**

```typescript
Deno.serve(async (req) => {
  const supabaseUrl = Deno.env.get('SUPABASE_URL')!
  const supabaseKey = Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')!
  
  const res = await fetch(`${supabaseUrl}/rest/v1/ipo_list?select=*&limit=20`, {
    headers: {
      'apikey': supabaseKey,
      'Authorization': `Bearer ${supabaseKey}`
    }
  })
  
  const data = await res.json()
  return new Response(JSON.stringify({ success: true, data }), {
    headers: { 'Content-Type': 'application/json' }
  })
})
```

**Step 2: Deploy**

Run: supabase_deploy_edge_function

**Step 3: Commit**

```bash
git add supabase/functions/
git commit -m "feat: add public IPO list edge function"
```

---

## Summary

| Task | What Changes | Files Modified |
|------|-------------|---------------|
| Task 1 | pg_cron for scheduling | `database/supabase_pg_cron.sql` |
| Task 2 | Auth integration | `.env`, schema update |
| Task 3 | Edge Functions | `supabase/functions/*` |

---

## Rollback Plan

If any feature doesn't work:
1. Disable pg_cron: `SELECT cron.unschedule('job-name')`
2. Revert Edge Function: Delete from Supabase dashboard
3. Auth: Remove user_id column

---

## Plan saved to `docs/plans/2026-02-27-supabase-features-plan.md`

Execute with: **superpowers:executing-plans**
