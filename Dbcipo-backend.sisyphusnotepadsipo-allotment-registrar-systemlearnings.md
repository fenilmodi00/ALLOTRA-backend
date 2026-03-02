

## [2026-03-02T00:00:00Z] Task 9: Registrar Code Scheduler Implementation

### Summary
Successfully created `jobs/registrar_code_scheduler.go` - a scheduler that runs every 30 minutes starting at 1 PM IST on result_date to insert job_dispatch rows for fetching registrar company codes.

### Implementation Details
- **Struct Fields**: db (*sql.DB), interval (time.Duration), stopChan (chan struct{})
- **Constructor**: NewRegistrarCodeScheduler() - initializes scheduler with DB connection and interval
- **Start()**: Launches goroutine with time.Ticker pattern (matches job_poller.go pattern)
  - Creates ticker with interval
  - Defers ticker.Stop()
  - Select loop on ticker.C and stopChan
  - Calls scheduleRegistrarCodeJobs() on tick
- **Stop()**: Closes stopChan for graceful shutdown
- **Scheduler Logic**:
  1. Load IST timezone (time.LoadLocation("Asia/Kolkata"))
  2. Get current IST time (time.Now().In(istLocation))
  3. Check hour >= 13 (1 PM IST) - skip if before
  4. Query: SELECT id, name, registrar FROM ipo_list WHERE DATE(result_date AT TIME ZONE 'Asia/Kolkata') = DATE(now)
  5. For each IPO:
     - Extract registrar short code using registrarMap (KFIN, BIGSHARE, MUFG, etc.)
     - Check ipo_registrar_codes table for is_resolved = true
     - If not resolved, insert job_dispatch row
- **Job Dispatch Insertion**:
  - Table: job_dispatch
  - Columns: job_type, payload (JSON), status, created_at, updated_at
  - job_type: "fetch_registrar_company_code" (matches executor registration in server.go)
  - payload: FetchRegistrarCodePayload{IPOID, RegistrarShortCode, IPOName}
  - status: "pending"

### Key Patterns Used
- Ticker pattern from job_poller.go (lines 70-85)
- Context with timeout for database operations (10 second timeout)
- Structured logging with logrus.WithFields()
- Error handling: log but continue (non-blocking scheduler)
- Uses FetchRegistrarCodePayload struct from fetch_registrar_code_job.go

### Registrar Mapping
- KFIN: Kfin Technologies Limited / Kfin Technologies Pvt Ltd / KFIN
- BIGSHARE: Bigshare Services / Bigshare Services Pvt Ltd
- MUFG: MUFG Bank Japan Limited / Mufg Bank Japan Limited / MUFG
- BOI: Bank of India
- COMPUTERSHARE: Computershare India Pvt Ltd
- NSDL: Nsdl Database Management Limited
- CDSL: Central Depository Services (India) Limited

### Build Verification
- `go build -v ./jobs/` succeeds with no errors
- All imports present and correct
- Follows Go best practices and project conventions

### Integration Points
- Works with existing job_dispatch table (no schema changes needed)
- Payload compatible with FetchRegistrarCodeJobExecutor in fetch_registrar_code_job.go
- Will be started in server.go startBackgroundJobs() function
- Assumes ipo_registrar_codes table exists (created in Task 1)

