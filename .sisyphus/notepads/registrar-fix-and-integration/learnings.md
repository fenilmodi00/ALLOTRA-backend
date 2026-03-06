# Registrar Code Case-Sensitivity Fix - Learnings

## Pattern Discovered
The `extractRegistrarShortCode` function in `jobs/registrar_code_scheduler.go` had a **case-sensitivity bug**. While the function attempted to handle multiple registrar name variants (lines 226-239), it wasn't normalizing the input to lowercase before comparison.

## Root Cause Analysis
The code at line 247 declared `lowerRegistrar := registrar` but never actually converted it to lowercase. This meant:
- Database values like "KFIN" or "KFin Technologies Limited" would not match the hardcoded map keys
- Only exact case matches worked
- Most real-world data with mixed case would fail to map

## Fix Implementation
**Single change in `extractRegistrarShortCode` function:**
1. Added `import "strings"` at top of file
2. Changed line 226 from:
   ```go
   lowerRegistrar := registrar
   ```
   To:
   ```go
   lowerRegistrar := strings.ToLower(registrar)
   ```
3. Normalized all map keys to lowercase for consistent matching
4. Removed the unused loop that iterated through registrarMap

## Test Coverage Strategy
Created comprehensive test suite with 31 test cases covering:
- **Exact matches**: Testing lowercase inputs against lowercase map keys
- **Case variants**: 
  - All uppercase: "KFIN", "KFIN TECHNOLOGIES LIMITED"
  - All lowercase: "kfin", "kfin technologies limited"
  - Mixed case: "KFin", "Kfin Technologies Limited"
  - Random case: "kFiN tEcHnOlOgIeS pVt LtD"
- **All registrars**: KFIN, BIGSHARE, MUFG, BOI, COMPUTERSHARE, NSDL, CDSL
- **Edge cases**: Empty string, unknown registrars

## Test Results
✅ **All 31 tests pass** in 1.236 seconds
- No failures
- No flaky tests
- Comprehensive coverage of all registrar variants

## Key Insight
**Input normalization should happen at function entry**, not within loops. This is cleaner, more efficient, and prevents the subtle bug we fixed here where normalization was declared but never executed.

## Code Quality
- Backward compatible (return values unchanged)
- More efficient (single normalization instead of repeated iterations)
- Fully tested (31 test cases)
- Clear and maintainable

## Files Modified
1. `jobs/registrar_code_scheduler.go` - Fixed function logic
2. `jobs/registrar_code_scheduler_test.go` - Added test suite

## Integration Impact
The fix automatically improves the `scheduleRegistrarCodeJobs()` function (line 126) which calls `extractRegistrarShortCode()` with database values that may have different casing than expected.

## Entry 1: ResultDate Nil Pointer Fix (2026-03-02)

### Problem Identified
- **File**: `jobs/fetch_registrar_code_job.go` line 61
- **Issue**: `ipo.ResultDate.In(istLocation)` panics when `ResultDate` is nil
- **Root Cause**: `ResultDate` is defined as `*time.Time` pointer in models, can be nil for IPOs without result dates
- **Impact**: Job crashes when processing IPOs with null result_date in database

### Solution Implemented
Added defensive nil check BEFORE dereferencing the pointer:
```go
// Check if result_date is set (nil check)
if ipo.ResultDate == nil {
    logger.WithField("ipo_name", payload.IPOName).Info("Skipping IPO: no result_date set")
    return nil
}
```

**Lines**: 60-64 of fetch_registrar_code_job.go

### Key Pattern Discovered
**Never assume pointers are non-nil in Go** - Always check before dereferencing. This is especially critical for fields populated from database queries where nullable columns exist.

### Testing Strategy
1. Unit tests for nil case (happy path safety)
2. Unit tests for valid pointer case (no regression)
3. Unit tests for timezone operations (domain-specific)
4. Unit tests for payload parsing (integration point)

### Files Changed
- `jobs/fetch_registrar_code_job.go`: Added 4-line nil check
- `jobs/fetch_registrar_code_job_test.go`: Created 6 comprehensive unit tests

### Test Results
- All 6 new tests: ✓ PASS
- All 5 existing job tests: ✓ PASS  
- Total: 11/11 tests pass
- No regressions detected

### Lessons for Future Work
1. **Nullable Database Fields**: When loading from database, ALWAYS check pointer types for nil
2. **Logging on Skip**: Use info-level logging when skipping IPOs - helps with debugging
3. **Test Coverage**: Pointer nil cases often overlooked - explicitly test them
4. **Pattern**: `if ptr == nil { return nil }` is idiomatic Go for graceful degradation

### Related Codebase Patterns
- Similar timezone handling exists in other job functions
- Timezone operations consistently use `time.LoadLocation("Asia/Kolkata")`
- Job executors follow consistent error handling pattern
- Logger fields include `ipo_id`, `ipo_name`, `registrar_short_code` for tracing

### Database Schema Notes
From models/ipo.go:26:
```go
ResultDate  *time.Time `json:"result_date" db:"result_date"`
```
- Type: Pointer to time.Time (nullable)
- Can be NULL in database
- Required for scheduling result announcement jobs

### Integration Points Validated
- Payload marshaling/unmarshaling works correctly
- Timezone operations isolated and tested
- No side effects on other job functions
- Existing GMP history tests still pass

### Confidence Level
**HIGH** - Single, focused change with comprehensive test coverage. No architectural changes needed.

- KFin web app currently exposes only /prod/api/query for allotment lookups; IPO dropdown options are embedded in main.*.js as JSON.parse payload.
- For GetActiveIPOs, API endpoint probing should keep browser headers (Origin/Referer) and fall back to JS bundle parsing when listing endpoints are unavailable.
- Added testable seam getActiveIPOsWithClient(ctx, client) to validate API-first + bundle-fallback behavior without real network calls.

## Entry 2: V2 Allotment Handler Registrar Rewire (2026-03-02)

- `handlers/v2_allotment_handler.go` now uses `registrars.GetClient(registrarShortCode)` + `client.CheckAllotment(ctx, companyCode, pan)` as the primary allotment path instead of the legacy `AllotmentChecker.CheckAllotmentStatus` call.
- Added explicit company-code retry logic: if `resolvedCode.RegistrarCompanyCode` is nil/empty after cache lookup, perform one live `RegistrarCodeService.ResolveCode` call and continue only if resolved code is present.
- Preserved status semantics: unresolved code and unsupported registrar return `503 SERVICE_UNAVAILABLE`; registrar upstream failures return `502 BAD_GATEWAY`.
- Mapped registrar result directly into response fields currently exposed by API contract (`status`, `shares_applied`, `shares_allotted`) and propagated `ApplicationNo` into `IPOResultCache.ApplicationNumber` for richer cache payload.
- Verification note: handler package tests pass, but full-repo `go build ./...` is currently blocked by a pre-existing `NewAdminHandler` signature mismatch in `internal/app/server.go`.

## Task 5: Admin Endpoint for Manual Registrar Code Resolution

### Implementation Details
- Added `POST /api/v2/admin/registrar/resolve` endpoint
- Follows exact pattern of existing `TriggerDailyUpdate` handler
- Service field added to `AdminHandler` struct: `RegistrarCodeService *services.RegistrarCodeService`
- Constructor signature extended to accept registrar code service parameter
- Wired in `server.go` by passing existing `registrarCodeService` instance (line 87)
- Route registered in v2Admin group (inherits `adminAuthMiddleware`)

### Key Design Decisions
1. **Nil Pointer Safety**: Added explicit nil check for `code.IPOName` before dereferencing
   - Pattern: `if code.IPOName == nil { logrus.Warn(...); continue }`
   - Prevents panic if IPO name is missing from database
   
2. **V2 Response Wrapper**: Created wrapper method in `V2AdminHandler`
   - Delegates to legacy handler
   - Wraps response in v2 envelope using `shared.NewV2Response()`
   - Service unavailable returns 503 with `shared.NewV2ErrorResponse()`

3. **Error Handling**: Per-code error handling with continue
   - Does not abort entire batch on single failure
   - Counts successful resolutions only
   - Returns total attempted vs resolved count

4. **Synchronous Execution**: As specified, runs synchronously
   - No goroutines, no background processing
   - Returns when complete with duration metric
   - Suitable for manual admin triggers

### Response Format
```json
{
  "success": true,
  "data": {
    "resolved_count": 5,
    "total": 10,
    "duration": "2.3s"
  }
}
```

### Service Dependencies
- `RegistrarCodeService.GetUnresolvedForToday()` - retrieves codes needing resolution
- `RegistrarCodeService.ResolveCode()` - calls external registrar API and upserts
- Both methods use context propagation from HTTP request

### Authentication
- No additional auth beyond v2Admin group middleware
- Uses existing `adminAuthMiddleware` with token validation
- Requires `X-Admin-Token` header or `Authorization: Bearer <token>`

### Build Verification
- `go build ./...` passes cleanly
- No compilation errors
- All type signatures correct


## Task 6: Registrar Code Resolution Script Step

**Completed:** Integrated registrar code resolution into orchestration script

### Implementation Details
- Added Step 3 to `scripts/run-all-scrapers.sh` (lines 40-61)
- Uses `ADMIN_TOKEN` environment variable (consistent with existing pattern)
- Curl call to `/api/v2/admin/registrar/resolve` endpoint
- Uses `X-Admin-Token` header (not Authorization bearer)
- Gracefully skips if ADMIN_TOKEN not set
- Provides clear success/failure/skip feedback

### Key Pattern
```bash
ADMIN_TOKEN=${ADMIN_TOKEN:-""}
if [ -n "$ADMIN_TOKEN" ]; then
  # curl with X-Admin-Token header
  # Parse HTTP response code and body
fi
```

### Testing Evidence
- Syntax check (bash -n): PASS
- Content verification: All requirements met
- Endpoint documentation updated to include /api/v2/admin/registrar/resolve

### Ordering
Step 3 placed logically:
1. After existing scraper steps (Steps 1-2)
2. Before database query steps
3. After step that populates ipo_list (implicit dependency)

## Entry 3: Task 7 E2E Verification Attempt (2026-03-02)

- `go build ./...` and `go test ./...` both pass in this environment.
- Full API/DB integration verification is environment-sensitive: backend startup hard-depends on Postgres at `localhost:5432`.
- `go run .` fails immediately with `failed to connect to database ... connectex: No connection could be made`.
- Docker daemon is unavailable (`//./pipe/dockerDesktopLinuxEngine` not found), so `docker exec ... psql` checks cannot run.
- For future E2E tasks, ensure Docker daemon (or local Postgres) is up before attempting API flow verification.
