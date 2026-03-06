# IPO Allotment Registrar Company Code System

## TL;DR

> **Quick Summary**: Implement a system to scrape and store registrar-specific company codes (KFin, Bigshare, MUFG) for each IPO, enabling automated PAN-based allotment checks. New DB table + cron job (30-min intervals during allotment period starting 1 PM IST) + upgraded API endpoint.

> **Deliverables**:
> - New `ipo_registrar_codes` table with SQL migration
> - Model, repository, service layers for registrar code management
> - Job executor: fetches company codes via registrar scrapers
> - Job scheduler: triggers jobs during allotment period (1 PM IST onwards)
> - Upgraded `POST /v2/allotment/check` endpoint using resolved codes

> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: Migration → Model/Repo → Job Executor → Scheduler → Handler Update → Integration

---

## Context

### Original Request
User wants to implement IPO allotment checker system where:
1. Create new DB table storing: IPO ID, registrar short code, scraped company code, IPO name
2. New API endpoint (POST) that takes IPO ID + PAN card number and checks allotment via tools/ scraper
3. Cron job specific to each IPO's allotment date (result_date), running from 1 PM IST every 30 minutes until company code is successfully scraped

### Interview Summary

**Key Discussions**:
- Use existing `job_dispatch` table + `JobPoller` infrastructure (not create new job table)
- Match threshold: 80.0 score to mark resolved
- Registrar short codes: "KFIN", "BIGSHARE", "MUFG" (already mapped in registry.go)
- Each registrar has unique UI + API patterns - handled by existing tools/registrars/* clients
- Existing `v2_allotment_handler.go` will be upgraded, not created new

**Research Findings**:
- `tools/registrars/interface.go`: RegistrarClient interface with GetActiveIPOs(), MatchCompanyName(), CheckAllotment()
- `tools/registrars/registry.go`: GetClient() maps short codes to clients
- `services/allotment_checker.go`: Existing allotment checker service (legacy - uses FormFields/ParserConfig)
- `jobs/job_poller.go`: JobPoller polls job_dispatch table, routes job_type → JobExecutor
- `database/schema.sql`: Main schema with ipo_list, ipo_result_cache, etc.
- `internal/app/server.go`: App wiring with job registration pattern
- **Gap Found**: mufg/client.go MatchCompanyName() is a stub returning ("", 0) - needs implementation

### Metis Review

**Identified Gaps** (addressed):
- MUFG MatchCompanyName stub: Added as task in plan
- IST timezone handling: Will use time.LoadLocation("Asia/Kolkata")
- Job payload structure: Need to define JSONB payload for job_dispatch
- Existing ipo_list.registrar_company_code column: Plan uses new table instead of populating that column (cleaner separation of concerns)

---

## Work Objectives

### Core Objective
Enable automated PAN-based IPO allotment checking by systematically scraping and storing registrar-specific company codes before the allotment result check happens.

### Concrete Deliverables
- Database migration: `database/migrations/0010_add_ipo_registrar_codes.sql`
- Model: `models/registrar_code.go` - RegistrarCode struct
- Repository interface: `repositories/interfaces.go` - add RegistrarCodeRepository interface
- Repository implementation: `repositories/registrar_code_repository.go`
- Service: `services/registrar_code_service.go`
- Job Executor: `jobs/fetch_registrar_code_job.go`
- Job Scheduler: `jobs/registrar_code_scheduler.go`
- Handler update: `handlers/v2_allotment_handler.go` - upgrade CheckAllotment method
- App wiring: `internal/app/server.go` - register executor and start scheduler
- MUFG fix: `tools/registrars/mufg/client.go` - implement MatchCompanyName

### Definition of Done
VZ|- [x] Migration runs successfully: `psql < 0010_add_ipo_registrar_codes.sql`
QZ|- [x] IPO with result_date = today gets job dispatched at 1 PM IST
YZ|- [x] Job executor resolves company code for KFin/Bigshare IPOs (score > 80.0)
TS|- [x] API endpoint returns allotment result using stored company code
NZ|- [x] API endpoint attempts live resolution if code not yet resolved
ZY|- [x] API returns 503 if live resolution also fails

### Must Have
- New ipo_registrar_codes table with proper constraints and indexes
- Job executor registered with JobPoller for job_type = "fetch_registrar_company_code"
- Scheduler runs every 30 minutes and dispatches jobs for IPOs in allotment period
- Handler uses resolved code or attempts live resolution
- All logging follows logrus structured logging pattern

### Must NOT Have (Guardrails)
- Do NOT modify existing ipo_list.registrar_company_code column logic
- Do NOT create new job_dispatch table - reuse existing
- Do NOT block API if code not resolved - attempt live resolution once
- Do NOT schedule jobs before result_date 1 PM IST

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go test framework)
- **Automated tests**: NO (not in scope per existing project patterns)
- **Framework**: N/A
- **Agent-Executed QA**: YES (mandatory for all tasks)

### QA Policy
Every task includes agent-executed QA scenarios. Evidence saved to `.sisyphus/evidence/`.

**Verification Tools**:
- Database: `psql` - run migration, query table
- API: `curl` - POST to endpoint, assert response
- Jobs: observe job_dispatch table inserts

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — can start immediately):
├── Task 1: DB Migration - ipo_registrar_codes table
├── Task 2: Model - RegistrarCode struct
├── Task 3: Repository Interface - add to interfaces.go
└── Task 4: Repository Implementation - postgres impl

Wave 2 (Core Logic - depends on Wave 1):
├── Task 5: Service - RegistrarCodeService
├── Task 6: Job Executor - fetch_registrar_code_job.go
├── Task 7: MUFG Fix - implement MatchCompanyName
└── Task 8: Handler Update - upgrade CheckAllotment

Wave 3 (Integration - depends on Wave 2):
├── Task 9: Job Scheduler - registrar_code_scheduler.go
└── Task 10: App Wiring - register in server.go

Wave 4 (Final - after all tasks):
├── Task F1: Integration test - full flow
└── Task F2: Manual verification
```

### Dependency Matrix

- **Tasks 1-4**: — — 5-10
- **Task 5**: 3, 4 — 6, 8, 9
- **Task 6**: 5 — 9, 10
- **Task 7**: — — 5, 8
- **Task 8**: 5, 7 — F1
- **Task 9**: 6, 8 — 10, F1
- **Task 10**: 9 — F1

---

## TODOs

- [x] 1. **DB Migration: ipo_registrar_codes table**

  **What to do**:
  - Create `database/migrations/0010_add_ipo_registrar_codes.sql`
  - Table: `ipo_registrar_codes` with columns:
    - `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
    - `ipo_id UUID NOT NULL` → FK to `ipo_list(id)`
    - `registrar_short_code VARCHAR(20) NOT NULL`
    - `registrar_company_code VARCHAR(100)` — nullable
    - `ipo_name VARCHAR(255)`
    - `match_score FLOAT DEFAULT 0`
    - `is_resolved BOOLEAN DEFAULT false`
    - `last_attempted_at TIMESTAMP`
    - `created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`
    - `updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`
    - UNIQUE constraint: `(ipo_id, registrar_short_code)`
  - Add index on `(ipo_id, registrar_short_code)`
  - Add index on `(is_resolved, result_date)` for scheduler queries

  **References**:
  - `database/schema.sql:9-63` - ipo_list table structure pattern
  - `database/migrations/00009_add_gmp_multisource_unique.sql` - migration file pattern

  **QA Scenarios**:
  ```
  Scenario: Migration applies cleanly
    Tool: Bash (psql)
    Preconditions: Database running, clean schema
    Steps:
      1. psql -f database/migrations/0010_add_ipo_registrar_codes.sql
      2. \d ipo_registrar_codes
    Expected Result: Table created with all columns and constraints
    Evidence: .sisyphus/evidence/task-1-migration-success.txt
  ```

- [x] 2. **Model: RegistrarCode struct**

  **What to do**:
  - Create `models/registrar_code.go`
  - Struct `RegistrarCode` with fields matching DB table
  - Add db tags for all columns
  - Add time.Time for created_at, updated_at, last_attempted_at

  **References**:
  - `models/ipo.go:10-69` - IPO struct pattern with db tags
  - `models/gmp.go` - example of additional model with UUID

  **QA Scenarios**:
  ```
  Scenario: Model compiles without errors
    Tool: Bash
    Preconditions: Go module available
    Steps:
      1. go build -v ./models/
    Expected Result: No build errors
    Evidence: .sisyphus/evidence/task-2-model-build.txt
  ```

- [x] 3. **Repository Interface: add RegistrarCodeRepository**

  **What to do**:
  - Add to `repositories/interfaces.go`:
    ```go
    type RegistrarCodeRepository interface {
        Upsert(ctx context.Context, code *models.RegistrarCode) error
        GetByIPOAndRegistrar(ctx context.Context, ipoID string, registrarShortCode string) (*models.RegistrarCode, error)
        GetUnresolvedByResultDate(ctx context.Context, date time.Time) ([]*models.RegistrarCode, error)
    }
    ```

  **References**:
  - `repositories/interfaces.go:9-12` - GMPRepository interface pattern

  **QA Scenarios**:
  ```
  Scenario: Interface compiles
    Tool: Bash
    Steps:
      1. go build -v ./repositories/
    Expected Result: No build errors
    Evidence: .sisyphus/evidence/task-3-interface-compiles.txt
  ```

- [x] 4. **Repository Implementation: PostgresRegistrarCodeRepository**

  **What to do**:
  - Create `repositories/registrar_code_repository.go`
  - Implement `PostgresRegistrarCodeRepository` struct
  - Implement all 3 methods from interface
  - Use sqlx for queries

  **References**:
  - `repositories/gmp_repository.go` - postgres implementation pattern
  - `database/schema.sql:169-196` - ipo_result_cache table (similar pattern)

  **Must NOT do**:
  - Do NOT add business logic - only data access

  **QA Scenarios**:
  ```
  Scenario: Repository implements interface correctly
    Tool: Bash
    Steps:
      1. go build -v ./repositories/
    Expected Result: No build errors
    Evidence: .sisyphus/evidence/task-4-repo-build.txt
  ```

- [x] 5. **Service: RegistrarCodeService**

  **What to do**:
  - Create `services/registrar_code_service.go`
  - Struct `RegistrarCodeService` with:
    - `db *sql.DB`
    - `repo repositories.RegistrarCodeRepository`
    - registrar registry (tools/registrars)
  - Methods:
    - `ResolveCode(ctx, ipoID, registrarShortCode, ipoName) (*models.RegistrarCode, error)` - calls GetActiveIPOs + MatchCompanyName, upserts if score > 80.0
    - `GetResolvedCode(ctx, ipoID, registrarShortCode) (*models.RegistrarCode, error)`
    - `GetUnresolvedForToday(ctx) ([]*models.RegistrarCode, error)` - for scheduler

  **References**:
  - `services/ipo_service.go` - service layer pattern
  - `services/gmp_service.go` - service wrapping repository
  - `tools/registrars/registry.go:22-43` - GetClient usage

  **QA Scenarios**:
  ```
  Scenario: Service builds successfully
    Tool: Bash
    Steps:
      1. go build -v ./services/
    Expected Result: No build errors
    Evidence: .sisyphus/evidence/task-5-service-build.txt
  ```

- [x] 6. **Job Executor: fetch_registrar_code_job**

  **What to do**:
  - Create `jobs/fetch_registrar_code_job.go`
  - JobExecutor function signature: `func(ctx context.Context, job JobDispatch) error`
  - Payload struct: `{ipo_id, registrar_short_code, ipo_name}`
  - Logic:
    1. Parse payload JSON
    2. Get IPO from DB (need ipoService or direct query for result_date)
    3. Check if result_date is today AND current time >= 13:00 IST - if not, skip but don't fail
    4. Call RegistrarCodeService.ResolveCode()
    5. If resolved (score > 80.0), mark job done
    6. If not resolved, keep as pending for retry

  **References**:
  - `jobs/job_poller.go:26` - JobExecutor function type
  - `jobs/daily_ipo_update.go:40-271` - complex job pattern
  - `internal/app/server.go:429-454` - job executor registration pattern

  **QA Scenarios**:
  ```
  Scenario: Job executor registered and builds
    Tool: Bash
    Steps:
      1. go build -v ./jobs/
    Expected Result: No build errors
    Evidence: .sisyphus/evidence/task-6-executor-build.txt
  ```

- [x] 7. **MUFG Fix: implement MatchCompanyName**

  **What to do**:
  - Update `tools/registrars/mufg/client.go:374-377`
  - Implement MatchCompanyName using same algorithm as kfin/client.go and bigshare/client.go
  - Use the dropdown options + string matching logic
  - Score calculation: exact=10000, contains=5000+len, contained=3000+len, word overlap=matched*100

  **References**:
  - `tools/registrars/kfin/client.go:478-529` - MatchCompanyName implementation
  - `tools/registrars/bigshare/client.go:181-233` - MatchCompanyName implementation

  **QA Scenarios**:
  ```
  Scenario: MUFG MatchCompanyName returns valid score
    Tool: Bash (Go test/repl)
    Preconditions: MUFG client available
    Steps:
      1. Create test with known IPO name
      2. Call MatchCompanyName
      3. Assert returned score > 0 for exact match
    Expected Result: Score > 0 for exact match
    Evidence: .sisyphus/evidence/task-7-mufg-match.txt
  ```

- [x] 8. **Handler Update: upgrade CheckAllotment**

  **What to do**:
  - Update `handlers/v2_allotment_handler.go`
  - In CheckAllotment method:
    1. Get IPO (existing)
    2. Determine registrar_short_code from IPO.Registrar (use registry mapping)
    3. Call RegistrarCodeService.GetResolvedCode()
    4. If is_resolved=true: use stored registrar_company_code
    5. If is_resolved=false: call RegistrarCodeService.ResolveCode() (live attempt)
    6. If still not resolved (live attempt failed too): return 503 with "Company code not yet resolved"
    7. Call AllotmentChecker or direct registrar client CheckAllotment()
    8. Return result

  **References**:
  - `handlers/v2_allotment_handler.go:42-100` - existing CheckAllotment flow
  - `tools/registrars/interface.go:9-13` - RegistrarClient interface

  **Must NOT do**:
  - Do NOT break existing API contract - same request/response format

  **QA Scenarios**:
  ```
  Scenario: API returns allotment result when code is resolved
    Tool: Bash (curl)
    Preconditions: IPO with resolved registrar_company_code exists
    Steps:
      1. curl -X POST http://localhost:8080/api/v2/allotment/check \
         -H "Content-Type: application/json" \
         -d '{"ipo_id": "...", "pan": "ABCDE1234F"}'
    Expected Result: 200 OK with allotment status
    Evidence: .sisyphus/evidence/task-8-api-resolved.txt

  Scenario: API returns 503 when code not resolved
    Tool: Bash (curl)
    Preconditions: IPO with no resolved code
    Steps:
      1. curl -X POST http://localhost:8080/api/v2/allotment/check \
         -H "Content-Type: application/json" \
         -d '{"ipo_id": "...", "pan": "ABCDE1234F"}'
    Expected Result: 503 Service Unavailable
    Evidence: .sisyphus/evidence/task-8-api-503.txt
  ```

- [x] 9. **Job Scheduler: registrar_code_scheduler**

  **What to do**:
  - Create `jobs/registrar_code_scheduler.go`
  - Scheduler struct with:
    - `db *sql.DB`
    - `interval time.Duration` (30 minutes)
    - `stopChan chan struct{}`
  - Start() method: runs goroutine with ticker
  - Logic every tick:
    1. Load Asia/Kolkata location
    2. Get current IST time
    3. If current hour >= 13 (1 PM):
       - Query ipo_list where result_date = today AND registrar != ""
       - For each IPO, check if ipo_registrar_codes has unresolved entry for any registrar
       - Insert job_dispatch row with job_type = "fetch_registrar_company_code"
       - Payload: {ipo_id, registrar_short_code, ipo_name}
    4. If current hour < 13: skip (no jobs needed)
  - Stop() method: graceful shutdown

  **References**:
  - `jobs/job_poller.go:58-86` - poller Start pattern
  - `internal/app/server.go:409-466` - startBackgroundJobs pattern
  - `database/schema.sql` - job_dispatch table structure (need to check if exists)

  **QA Scenarios**:
  ```
  Scenario: Scheduler runs and inserts job_dispatch rows
    Tool: Bash + psql
    Preconditions: IPO with result_date = today exists
    Steps:
      1. Start scheduler (or trigger manually)
      2. Wait for tick (or force immediate tick)
      3. SELECT * FROM job_dispatch WHERE job_type = 'fetch_registrar_company_code';
    Expected Result: Job rows inserted for unresolved IPOs
    Evidence: .sisyphus/evidence/task-9-scheduler-jobs.txt
  ```

- [x] 10. **App Wiring: register in server.go**

  **What to do**:
  - Update `internal/app/server.go`
  - Add to service initialization section:
    ```go
    registrarCodeRepo := repositories.NewPostgresRegistrarCodeRepository(db)
    registrarCodeService := services.NewRegistrarCodeService(db, registrarCodeRepo)
    ```
  - Add to job registration in startBackgroundJobs():
    ```go
    poller.RegisterExecutor("fetch_registrar_code_job", 
      jobs.NewFetchRegistrarCodeJobExecutor(registrarCodeService, ipoService))
    ```
  - Add scheduler startup (if NOT using Supabase Cron):
    ```go
    registrarCodeScheduler := jobs.NewRegistrarCodeScheduler(db, 30*time.Minute)
    registrarCodeScheduler.Start()
    ```
    Or if using Supabase Cron mode, the scheduler just inserts jobs and poller runs them

  **References**:
  - `internal/app/server.go:76-98` - service initialization pattern
  - `internal/app/server.go:429-454` - executor registration pattern
  - `internal/app/server.go:136` - background jobs start call

  **QA Scenarios**:
  ```
  Scenario: Application starts without errors
    Tool: Bash
    Steps:
      1. go build -v ./...
      2. Run app (or check init)
    Expected Result: No build errors, all services initialize
    Evidence: .sisyphus/evidence/task-10-wiring-build.txt
  ```

---

## Final Verification Wave

- [x] F1. **Integration Test: Full Flow**

  Read the plan end-to-end. For each task: verify implementation exists (read file). Test full flow:
  1. IPO with result_date = today exists in DB
  2. Scheduler runs at/after 1 PM IST
  3. job_dispatch gets row inserted
  4. Job executor runs, resolves company code
  5. API call uses resolved code, returns allotment result
  Output: Full flow works end-to-end

- [x] F2. **Manual Verification**

  Execute manual scenarios:
  1. Create test IPO with result_date = today
  2. Call API with PAN - should return 503 initially
  3. Wait for scheduler + job executor to run
  4. Call API again - should return actual allotment result
  Output: Manual test passes

---

## Commit Strategy

- **1**: `feat(registrar-codes): add ipo_registrar_codes table migration` — 0010_add_ipo_registrar_codes.sql
- **2**: `feat(registrar-codes): add model and repository` — models/registrar_code.go, repositories/interfaces.go, repositories/registrar_code_repository.go
- **3**: `feat(registrar-codes): add service layer` — services/registrar_code_service.go
- **4**: `feat(registrar-codes): add job executor and scheduler` — jobs/fetch_registrar_code_job.go, jobs/registrar_code_scheduler.go
- **5**: `fix(mufg): implement MatchCompanyName stub` — tools/registrars/mufg/client.go
- **6**: `feat(registrar-codes): integrate with allotment handler` — handlers/v2_allotment_handler.go, internal/app/server.go

---

## Success Criteria

### Verification Commands
```bash
# Apply migration
psql -f database/migrations/0010_add_ipo_registrar_codes.sql

# Verify table
psql -c "\d ipo_registrar_codes"

# Build
go build -v ./...

# Test API (with resolved code)
curl -X POST http://localhost:8080/api/v2/allotment/check \
  -H "Content-Type: application/json" \
  -d '{"ipo_id": "<uuid>", "pan": "ABCDE1234F"}'
```

### Final Checklist
MW|- [x] All tasks complete with QA evidence
BP|- [x] All files created per plan
BB|- [x] Build passes
SR|- [x] Integration flow works
NH|- [x] No new registrars added (scope maintained)
