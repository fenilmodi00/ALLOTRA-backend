# Registrar Fix & End-to-End Allotment Integration

## TL;DR

> **Quick Summary**: Fix the broken Kfin IPO dropdown scraper (returns 0 results because React SPA), rewire the v2 allotment handler to use registrar clients' `CheckAllotment` instead of the legacy form-scraping `AllotmentChecker`, add an admin endpoint for manual registrar code resolution, update the shell script with registrar code steps, and fix minor bugs (case-sensitivity, nil pointer).
>
> **Deliverables**:
> - Working Kfin `GetActiveIPOs` that fetches dropdown data via API (not HTML scraping)
> - `v2_allotment_handler.go` rewired to use `registrar.CheckAllotment(ctx, companyCode, pan)` with `shared.AllotmentResult`
> - Admin endpoint `POST /api/v2/admin/registrar/resolve` for manual trigger
> - Updated `scripts/run-all-scrapers.sh` with Step 3 for registrar code resolution
> - Bug fixes: `extractRegistrarShortCode` case-insensitive, `ResultDate` nil check
> - End-to-end verified: Accord Transformers + PAN → allotment status returned
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES — 3 waves
> **Critical Path**: Task 1 (Kfin fix) → Task 3 (handler rewire) → Task 7 (E2E verification)

---

## Context

### Original Request
Build a fully working IPO Allotment Registrar System: fix the registrar code resolution so `GetActiveIPOs` actually returns data, make the `/api/v2/allotment/check` endpoint use the new registrar clients instead of the legacy form scraper, add manual admin trigger, update shell script with date-based fetching, and verify everything end-to-end.

### Interview Summary
**Key Discussions**:
- The system has TWO allotment checking approaches: LEGACY (`services/allotment_checker.go` using colly form scraper with `FormFields`/`FormHeaders`/`ParserConfig` JSONB) and NEW (registrar-specific API clients like Kfin AWS API Gateway at `kfinAPIBaseURL`)
- The handler at line 105 resolves company code via NEW system but then calls LEGACY `AllotmentChecker.CheckAllotmentStatus` — the resolved code is never used
- Kfin `GetActiveIPOs` tries to extract data from inline scripts/JS bundles but the React SPA loads data at runtime via API
- Kfin `CheckAllotment` method WORKS via API (`GET kfinAPIBaseURL/query?type=pan&ipocode={code}&pan={PAN}`)
- Admin endpoints follow synchronous pattern: check nil → call service → return JSON with timing
- DB is local Docker PostgreSQL; fresh DB requires manual cron trigger

**Research Findings**:
- **Kfin API base URL already exists**: `kfinAPIBaseURL = "https://0uz601ms56.execute-api.ap-south-1.amazonaws.com/prod/api"` (client.go:21)
- **Kfin API likely has a listing endpoint** — common REST pattern; need to discover it (try `/ipolist`, `/ipo`, or similar)
- **MatchCompanyName works fine** if given options — scoring threshold is appropriate (any word match = 100 > 80 threshold)
- **Admin handler pattern**: sync call with nil check, timing measurement, JSON response (see `TriggerDailyUpdate` at admin_handler.go:102-126)
- **RegistrarCodeService already wired** in server.go:86-87, passed to v2AllotmentHandler at line 334
- **`extractRegistrarShortCode`** at registrar_code_scheduler.go:224-256 has case-sensitivity bug — "fallback" at line 247-252 doesn't actually lowercase

### Metis Review
Metis consultation timed out. Self-review applied instead:

**Identified Gaps** (addressed):
- **Kfin API discovery**: We don't know the exact endpoint for listing IPOs. Plan includes discovery step with fallback to direct API probing.
- **Bigshare/MUFG clients**: May have similar scraping issues — plan scopes to Kfin only since that's the registrar for test IPOs.
- **Legacy AllotmentChecker removal**: NOT removing it — keeping as fallback for IPOs with FormFields configured but no registrar client.
- **Error response contract**: Handler currently returns different status codes (503, 502) — plan preserves existing error contract.
- **Concurrent resolution**: Multiple simultaneous `/allotment/check` calls for same unresolved IPO could trigger duplicate resolution. Plan notes this as acceptable (upsert is idempotent).

---

## Work Objectives

### Core Objective
Make the `/api/v2/allotment/check` endpoint work end-to-end by connecting the Kfin API-based registrar client system through the handler, replacing the legacy form scraper path.

### Concrete Deliverables
- `tools/registrars/kfin/client.go`: `GetActiveIPOs` rewritten to use Kfin API (not HTML scraping)
- `handlers/v2_allotment_handler.go`: Lines 104-108 replaced with registrar client `CheckAllotment` call using resolved company code
- `handlers/admin_handler.go` (or new file): Admin handler method for `POST /api/v2/admin/registrar/resolve`
- `internal/app/server.go`: Admin route registration for registrar resolve endpoint
- `jobs/registrar_code_scheduler.go`: `extractRegistrarShortCode` fixed for case-insensitive matching
- `jobs/fetch_registrar_code_job.go`: `ResultDate` nil check added
- `scripts/run-all-scrapers.sh`: Step 3 added for registrar code resolution

### Definition of Done
- [ ] `curl -X POST http://localhost:8080/api/v2/allotment/check -d '{"ipo_id":"<accord-id>","pan":"AQNPN9478L"}'` returns allotment status (not 503/502 error)
- [ ] `curl -X POST -H 'X-Admin-Token: <token>' http://localhost:8080/api/v2/admin/registrar/resolve` returns success with resolved codes
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes

### Must Have
- Kfin `GetActiveIPOs` returns IPO dropdown options from the API
- Handler uses `registrar.CheckAllotment(ctx, companyCode, pan)` instead of legacy `AllotmentChecker`
- Handler maps `shared.AllotmentResult` fields to `V2AllotmentResponse` correctly
- Admin endpoint for manual registrar code resolution (synchronous)
- Case-insensitive registrar short code extraction
- Nil safety for `ResultDate` pointer

### Must NOT Have (Guardrails)
- Do NOT modify existing `ipo_list.registrar_company_code` column logic
- Do NOT create new `job_dispatch` table — reuse existing
- Do NOT block API if code not resolved — attempt live resolution once (already implemented at lines 96-101)
- Do NOT schedule jobs before `result_date` 1 PM IST
- Do NOT remove the legacy `AllotmentChecker` service — it may still be used by IPOs with `FormFields` configured
- Do NOT remove `AllotmentChecker` from `V2AllotmentHandler` struct — keep as fallback
- Do NOT change the `V2AllotmentResponse` struct shape (would break API contract)
- Do NOT over-abstract — no new interfaces for admin handler, just add method directly
- Do NOT add excessive error logging beyond existing patterns

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (go test)
- **Automated tests**: Tests-after (add tests for new/modified functions)
- **Framework**: `go test`
- **Approach**: Write tests for `extractRegistrarShortCode` fix, handler rewiring (mock registrar client), and Kfin API integration

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **API endpoints**: Use Bash (curl) — Send requests, assert status + response fields
- **Build verification**: Use Bash (`go build ./...`, `go test ./...`)
- **DB verification**: Use Bash (docker exec psql queries)

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — independent fixes + Kfin API discovery):
├── Task 1: Fix Kfin GetActiveIPOs to use API [deep]
├── Task 2: Fix extractRegistrarShortCode case-sensitivity [quick]
├── Task 3: Fix ResultDate nil pointer in fetch_registrar_code_job.go [quick]

Wave 2 (After Wave 1 — depends on working Kfin client):
├── Task 4: Rewire v2_allotment_handler to use registrar CheckAllotment [deep]
├── Task 5: Add admin endpoint for manual registrar code resolution [unspecified-high]
├── Task 6: Update scripts/run-all-scrapers.sh with registrar step [quick]

Wave 3 (After Wave 2 — verification):
├── Task 7: End-to-end integration test [deep]

Wave FINAL (After ALL tasks — independent review, 4 parallel):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
├── Task F4: Scope fidelity check (deep)

Critical Path: Task 1 → Task 4 → Task 7 → F1-F4
Parallel Speedup: ~50% faster than sequential
Max Concurrent: 3 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | 4, 5, 7 | 1 |
| 2 | — | 7 | 1 |
| 3 | — | 7 | 1 |
| 4 | 1 | 7 | 2 |
| 5 | 1 | 6, 7 | 2 |
| 6 | 5 | 7 | 2 |
| 7 | 1, 2, 3, 4, 5, 6 | F1-F4 | 3 |

### Agent Dispatch Summary

- **Wave 1**: **3** — T1 → `deep`, T2 → `quick`, T3 → `quick`
- **Wave 2**: **3** — T4 → `deep`, T5 → `unspecified-high`, T6 → `quick`
- **Wave 3**: **1** — T7 → `deep`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

> Implementation + Test = ONE Task. Never separate.
> EVERY task MUST have: Recommended Agent Profile + Parallelization info + QA Scenarios.
> **A task WITHOUT QA Scenarios is INCOMPLETE. No exceptions.**

### Wave 1 — Independent Fixes + Kfin API Discovery

- [ ] 1. Fix Kfin `GetActiveIPOs` to use API instead of HTML scraping

  **What to do**:
  - Discover the Kfin API listing endpoint. The base URL is already defined: `kfinAPIBaseURL = "https://0uz601ms56.execute-api.ap-south-1.amazonaws.com/prod/api"` (client.go:21). Try common patterns:
    - `GET {kfinAPIBaseURL}/ipolist` or `GET {kfinAPIBaseURL}/ipo` or `GET {kfinAPIBaseURL}/list`
    - Try with same headers as `CheckAllotment` uses (lines 85-110): `Origin: https://ipostatus.kfintech.com`, `Referer: https://ipostatus.kfintech.com/`
    - Inspect network traffic at `https://ipostatus.kfintech.com` using Playwright to discover the exact listing endpoint (open page, watch XHR requests)
  - Rewrite `GetActiveIPOs()` (lines 419-475) to call the discovered API endpoint
  - Parse response into `[]shared.DropdownOption` (ID=company code, Name=company name)
  - Remove all colly/HTML scraping logic from `GetActiveIPOs` (the `fetchIPODropdownOptions` helper at lines 310-370 and `extractIPOListFromHTML` at lines 372-417)
  - Keep `kfinWebBaseURL` constant (line 20) — other methods may still use it
  - If API discovery fails: implement a hardcoded fallback using the web page's select element via an HTTP GET + regex on the React hydration data or JS bundle, documenting the fragility
  - Add/update tests for the new `GetActiveIPOs` implementation

  **Must NOT do**:
  - Do NOT modify `CheckAllotment` method — it already works via API
  - Do NOT modify `MatchCompanyName` — it works correctly when given options
  - Do NOT remove `kfinWebBaseURL` constant
  - Do NOT add new external dependencies (use existing `net/http` or the project's HTTP client patterns)

  **Recommended Agent Profile**:
  > Select category + skills based on task domain.
  - **Category**: `deep`
    - Reason: Requires API discovery (network inspection), reverse engineering, and multi-step implementation with fallback strategy
  - **Skills**: [`playwright`]
    - `playwright`: Needed to open `https://ipostatus.kfintech.com` and inspect network XHR requests to discover the API listing endpoint
  - **Skills Evaluated but Omitted**:
    - `api-designer`: This is API consumption, not design

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Tasks 4, 5, 7
  - **Blocked By**: None (can start immediately)

  **References** (CRITICAL - Be Exhaustive):

  **Pattern References** (existing code to follow):
  - `tools/registrars/kfin/client.go:85-110` — `CheckAllotment` method shows the working API call pattern: HTTP GET to `kfinAPIBaseURL`, with specific headers (`Origin`, `Referer`), JSON response parsing. Use this EXACT same HTTP pattern for the listing endpoint.
  - `tools/registrars/kfin/client.go:19-24` — Constants: `kfinAPIBaseURL` (AWS API Gateway) and `kfinWebBaseURL` (React SPA). The API base is what you need.
  - `tools/registrars/kfin/client.go:419-475` — Current broken `GetActiveIPOs` (returns empty). This is what you're REPLACING.
  - `tools/registrars/kfin/client.go:310-370` — `fetchIPODropdownOptions` helper (colly-based HTML scraping). REMOVE this.
  - `tools/registrars/kfin/client.go:372-417` — `extractIPOListFromHTML` (regex on script tags). REMOVE this.

  **API/Type References** (contracts to implement against):
  - `shared/registrar_types.go:8-11` — `DropdownOption{ID string, Name string}` — this is the return type for `GetActiveIPOs`. `ID` = company code (e.g., "ACTR"), `Name` = company name.
  - `tools/registrars/interface.go:7` — `GetActiveIPOs(ctx context.Context) ([]shared.DropdownOption, error)` — interface contract to satisfy.

  **External References** (libraries and frameworks):
  - Kfin API base: `https://0uz601ms56.execute-api.ap-south-1.amazonaws.com/prod/api` — try appending `/ipolist`, `/ipo`, `/list`, `/query?type=list`
  - Kfin web SPA: `https://ipostatus.kfintech.com` — open this in Playwright to discover XHR calls

  **WHY Each Reference Matters**:
  - `CheckAllotment` (lines 85-110): Copy this HTTP call pattern exactly — same headers, same error handling, same JSON parsing approach. The API Gateway likely requires the same Origin/Referer headers.
  - `DropdownOption` type: Your return value MUST be `[]shared.DropdownOption` — ID is the company code string used by `CheckAllotment`, Name is the display name used by `MatchCompanyName`.
  - Current broken implementation (lines 310-475): Understand what it tried to do (scrape HTML select elements) so you know what to replace and remove.

  **Acceptance Criteria**:
  - [ ] `GetActiveIPOs()` returns non-empty `[]shared.DropdownOption` with at least 1 entry
  - [ ] Each `DropdownOption.ID` is a valid company code usable by `CheckAllotment`
  - [ ] Old HTML scraping functions removed (`fetchIPODropdownOptions`, `extractIPOListFromHTML`)
  - [ ] `go build ./...` passes
  - [ ] `go test ./tools/registrars/kfin/...` passes

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Kfin API returns active IPO dropdown options (happy path)
    Tool: Bash (curl)
    Preconditions: Network access to AWS API Gateway
    Steps:
      1. Run: go build ./... (must pass)
      2. Write a small Go test or main that calls kfin.NewClient().GetActiveIPOs(context.Background())
      3. Assert: returned slice length > 0
      4. Assert: first element has non-empty ID and Name
      5. Pick one returned ID, call CheckAllotment(ctx, id, "AQNPN9478L") to verify ID is usable
    Expected Result: GetActiveIPOs returns ≥1 options; at least one ID works with CheckAllotment
    Failure Indicators: Empty slice, HTTP 403/404, timeout, JSON parse error
    Evidence: .sisyphus/evidence/task-1-kfin-api-listing.txt

  Scenario: GetActiveIPOs handles API unavailability gracefully (error path)
    Tool: Bash (go test)
    Preconditions: None
    Steps:
      1. Write test that mocks HTTP client to return 500
      2. Call GetActiveIPOs()
      3. Assert: error returned, no panic
    Expected Result: Returns error with descriptive message, does not panic
    Failure Indicators: Panic, nil error on failure
    Evidence: .sisyphus/evidence/task-1-kfin-api-error.txt
  ```

  **Evidence to Capture:**
  - [ ] task-1-kfin-api-listing.txt — curl output or Go test output showing returned IPO options
  - [ ] task-1-kfin-api-error.txt — test output for error handling

  **Commit**: YES (standalone)
  - Message: `fix(registrar): rewrite Kfin GetActiveIPOs to use API instead of HTML scraping`
  - Files: `tools/registrars/kfin/client.go`, test files
  - Pre-commit: `go build ./... && go test ./tools/registrars/kfin/...`

- [ ] 2. Fix `extractRegistrarShortCode` case-sensitivity bug

  **What to do**:
  - In `jobs/registrar_code_scheduler.go` at function `extractRegistrarShortCode` (lines 224-256):
    - Add `strings.ToLower()` to the input `registrar` parameter at the top of the function
    - Ensure all map keys in `registrarMap` (line 227-240) are lowercase
    - The fallback `strings.Contains` checks at lines 247-252 should also use lowercased input
  - Add unit test for `extractRegistrarShortCode` covering: exact match, mixed-case input, "KFin Technologies" variant, unknown registrar

  **Must NOT do**:
  - Do NOT change the return values (short code strings like "KFIN", "LINKINTIME") — only fix input normalization
  - Do NOT modify the registrar map entries — only add `strings.ToLower` to the input

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single function fix — add `strings.ToLower` and a test. Minimal scope.
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - None — straightforward Go fix

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: Task 7 (E2E verification)
  - **Blocked By**: None (can start immediately)

  **References** (CRITICAL - Be Exhaustive):

  **Pattern References**:
  - `jobs/registrar_code_scheduler.go:224-256` — The ENTIRE function to fix. Line 226 is the `registrarMap` definition. Line 247-252 is the fallback `strings.Contains` block. Both need lowercased input.
  - `jobs/registrar_code_scheduler.go:1-10` — Check existing imports for `strings` package (may already be imported).

  **API/Type References**:
  - Return values are string constants: `"KFIN"`, `"LINKINTIME"`, `"BIGSHARE"`, `"MUFG"`, `"SKYLINE"`, `"MAASHITLA"`, `"PURVA"`, `"CAMEO"`, `"INTEGRATED"` — these must NOT change.

  **Test References**:
  - Look for existing test file: `jobs/registrar_code_scheduler_test.go` — if it exists, add test cases there. If not, create it.

  **WHY Each Reference Matters**:
  - Lines 224-256: This is the ONLY code to modify. The fix is surgical: `registrar = strings.ToLower(registrar)` at line 225, then ensure map keys are lowercase.

  **Acceptance Criteria**:
  - [ ] `extractRegistrarShortCode("KFin Technologies Limited")` returns `"KFIN"`
  - [ ] `extractRegistrarShortCode("KFIN TECHNOLOGIES LIMITED")` returns `"KFIN"`
  - [ ] `extractRegistrarShortCode("Link Intime India Private Limited")` returns `"LINKINTIME"`
  - [ ] `extractRegistrarShortCode("unknown registrar")` returns `""`
  - [ ] `go test ./jobs/...` passes

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Case-insensitive registrar matching (happy path)
    Tool: Bash (go test)
    Preconditions: Test file exists
    Steps:
      1. Run: go test ./jobs/... -run TestExtractRegistrarShortCode -v
      2. Assert: all 4 test cases pass (exact match, uppercase, mixed case, unknown)
    Expected Result: PASS for all variants
    Failure Indicators: FAIL on any case variant
    Evidence: .sisyphus/evidence/task-2-case-insensitive.txt

  Scenario: Existing scheduler tests still pass (regression)
    Tool: Bash (go test)
    Preconditions: None
    Steps:
      1. Run: go test ./jobs/... -v
      2. Assert: all tests pass, no regressions
    Expected Result: PASS (0 failures)
    Failure Indicators: Any FAIL
    Evidence: .sisyphus/evidence/task-2-regression.txt
  ```

  **Evidence to Capture:**
  - [ ] task-2-case-insensitive.txt — test output showing all case variants pass
  - [ ] task-2-regression.txt — full `go test ./jobs/...` output

  **Commit**: YES (groups with Task 3)
  - Message: `fix(scheduler): case-insensitive extractRegistrarShortCode + ResultDate nil check`
  - Files: `jobs/registrar_code_scheduler.go`, `jobs/registrar_code_scheduler_test.go`, `jobs/fetch_registrar_code_job.go`
  - Pre-commit: `go test ./jobs/...`

- [ ] 3. Fix `ResultDate` nil pointer in `fetch_registrar_code_job.go`

  **What to do**:
  - In `jobs/fetch_registrar_code_job.go` at line 61: `scheduledTime := ipo.ResultDate.In(istLocation)`
    - Add nil check before this line: `if ipo.ResultDate == nil { log.Printf("Skipping IPO %s: no result_date set", ipo.IPOName); continue }`
    - `ResultDate` is `*time.Time` (pointer) in `models/ipo.go:68` — can be nil for IPOs without a result date
  - Add unit test for the nil ResultDate scenario

  **Must NOT do**:
  - Do NOT change the `ResultDate` type in the model
  - Do NOT set a default date — skip the IPO if nil

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single nil check addition — 2 lines of code + test
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: Task 7 (E2E verification)
  - **Blocked By**: None (can start immediately)

  **References** (CRITICAL - Be Exhaustive):

  **Pattern References**:
  - `jobs/fetch_registrar_code_job.go:55-65` — The loop body where `ipo.ResultDate.In(istLocation)` is called at line 61. This is where the nil check must be inserted.
  - `jobs/fetch_registrar_code_job.go:45-50` — The function signature and loop start. Understand the loop context: iterating over IPOs from `GetUnresolvedByResultDate`.

  **API/Type References**:
  - `models/ipo.go:68` — `ResultDate *time.Time` — pointer type, confirms nil is possible.
  - `repositories/interfaces.go` — `GetUnresolvedByResultDate` may return IPOs with nil result_date depending on query logic.

  **WHY Each Reference Matters**:
  - Line 61 is the crash site. The fix is a nil guard 1 line above it.
  - `models/ipo.go:68` confirms the field IS a pointer (not `time.Time` value type), so nil is a valid state.

  **Acceptance Criteria**:
  - [ ] No panic when `ipo.ResultDate` is nil — IPO is skipped with log message
  - [ ] Normal flow works when `ipo.ResultDate` is set
  - [ ] `go build ./...` passes
  - [ ] `go test ./jobs/...` passes

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Job handles nil ResultDate gracefully (error path)
    Tool: Bash (go test)
    Preconditions: Test exists for fetch_registrar_code_job
    Steps:
      1. Write test with mock IPO having ResultDate = nil
      2. Call the job function
      3. Assert: no panic, IPO skipped, log message emitted
    Expected Result: Job continues processing other IPOs, logs skip message
    Failure Indicators: Panic, nil pointer dereference
    Evidence: .sisyphus/evidence/task-3-nil-resultdate.txt
  ```

  **Evidence to Capture:**
  - [ ] task-3-nil-resultdate.txt — test output confirming nil safety

  **Commit**: YES (groups with Task 2)
  - Message: `fix(scheduler): case-insensitive extractRegistrarShortCode + ResultDate nil check`
  - Files: `jobs/fetch_registrar_code_job.go`, test files
  - Pre-commit: `go test ./jobs/...`

### Wave 2 — Core Rewiring (depends on Wave 1)

- [ ] 4. Rewire `v2_allotment_handler.go` to use registrar `CheckAllotment` instead of legacy scraper

  **What to do**:
  - In `handlers/v2_allotment_handler.go`, replace lines 104-108 (the legacy `AllotmentChecker.CheckAllotmentStatus` call):
    ```go
    // REMOVE (line 105):
    status, sharesAllotted, err := h.AllotmentChecker.CheckAllotmentStatus(c.Context(), ipo, req.PAN)
    
    // REPLACE WITH:
    client := registrars.GetClient(registrarShortCode)
    if client == nil {
        return c.Status(503).JSON(V2Response{Success: false, Error: "Unsupported registrar: " + registrarShortCode})
    }
    allotmentResult, err := client.CheckAllotment(c.Context(), *resolvedCode.RegistrarCompanyCode, req.PAN)
    ```
  - Map `shared.AllotmentResult` fields to the existing `V2AllotmentResponse`:
    - `allotmentResult.Status` → `response.Status` (string: "ALLOTTED", "NOT_ALLOTTED", etc.)
    - `allotmentResult.SharesApplied` → `response.SharesApplied`
    - `allotmentResult.SharesAllotted` → `response.SharesAllotted`
    - `allotmentResult.ApplicationNo` → `response.ApplicationNumber`
    - `allotmentResult.Name` → `response.ApplicantName`
    - `allotmentResult.Category` → `response.Category`
  - Handle the case where `resolvedCode.RegistrarCompanyCode` is nil (code wasn't resolved): attempt live resolution once using `RegistrarCodeService.ResolveCode`, then retry. This logic may already exist at lines 96-101 — verify and connect.
  - Preserve existing error handling patterns (503 for service unavailable, 502 for upstream failure)
  - Add import for `tools/registrars` package if not already imported
  - Keep `AllotmentChecker` in the handler struct as fallback — but the primary path should now use registrar clients

  **Must NOT do**:
  - Do NOT change `V2AllotmentResponse` struct shape (would break API contract for frontend)
  - Do NOT remove `AllotmentChecker` from handler struct or constructor
  - Do NOT modify the `/api/v2/allotment/check` route path
  - Do NOT change request validation logic (lines 70-80)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires understanding two allotment systems, careful field mapping between `shared.AllotmentResult` and `V2AllotmentResponse`, and preserving error contracts. Integration-heavy.
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `api-designer`: Not designing new API, just rewiring internal handler logic

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 5, 6 in Wave 2)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 7 (E2E verification)
  - **Blocked By**: Task 1 (needs working `GetActiveIPOs` for live resolution fallback)

  **References** (CRITICAL - Be Exhaustive):

  **Pattern References**:
  - `handlers/v2_allotment_handler.go:70-120` — The ENTIRE `CheckAllotment` handler method. Lines 70-80 = request validation. Lines 82-92 = IPO lookup + registrar detection. Lines 94-102 = code resolution (NEW system, KEEP). Line 105 = legacy checker call (REPLACE). Lines 107-120 = response building (MODIFY to use AllotmentResult).
  - `handlers/v2_allotment_handler.go:20-35` — `V2AllotmentHandler` struct and constructor. Check what fields are available: `AllotmentChecker`, `RegistrarCodeService`, `IPOService`.
  - `handlers/v2_allotment_handler.go:40-55` — `V2AllotmentResponse` struct definition. Map AllotmentResult fields to these exact field names.

  **API/Type References**:
  - `shared/registrar_types.go:13-20` — `AllotmentResult{Status, ApplicationNo, Name, SharesApplied, SharesAllotted, Category}` — the NEW return type from registrar clients.
  - `shared/registrar_types.go:22-25` — Status constants: `StatusAllotted = "ALLOTTED"`, `StatusNotAllotted = "NOT_ALLOTTED"` — use these.
  - `tools/registrars/registry.go:20-30` — `GetClient(shortCode string) RegistrarClient` — returns nil if registrar unknown. Check for nil!
  - `tools/registrars/interface.go:9` — `CheckAllotment(ctx, companyCode, pan string) (*AllotmentResult, error)` — the method to call.
  - `models/registrar_code.go:15-20` — `RegistrarCode.RegistrarCompanyCode *string` — pointer, check for nil.

  **WHY Each Reference Matters**:
  - Lines 94-102 (resolution logic): This is the NEW code that resolves company codes. It WORKS. Don't touch it — just connect its output (`resolvedCode`) to the new `CheckAllotment` call.
  - Line 105 (legacy call): This is the ONE LINE to replace. Everything else around it stays.
  - `V2AllotmentResponse` struct: Your field mapping MUST match these exact names — the frontend depends on them.
  - `GetClient` returns nil: You MUST handle this case or the app will panic on unknown registrars.

  **Acceptance Criteria**:
  - [ ] Handler calls `registrar.CheckAllotment(ctx, companyCode, pan)` instead of `AllotmentChecker.CheckAllotmentStatus`
  - [ ] `V2AllotmentResponse` correctly populated from `shared.AllotmentResult` fields
  - [ ] Unknown registrar returns 503 with descriptive error
  - [ ] Nil company code triggers live resolution attempt
  - [ ] `go build ./...` passes
  - [ ] Existing handler tests pass (if any)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Allotment check returns data via registrar client (happy path)
    Tool: Bash (curl)
    Preconditions: Server running on port 8080, registrar code resolved for Accord Transformers
    Steps:
      1. First resolve the code: curl -X POST -H 'X-Admin-Token: <token>' http://localhost:8080/api/v2/admin/registrar/resolve
      2. Then check allotment: curl -s -X POST -H 'Content-Type: application/json' http://localhost:8080/api/v2/allotment/check -d '{"ipo_id":"f6e09b37-5128-4ba1-b220-93818096f366","pan":"AQNPN9478L"}'
      3. Assert: HTTP 200
      4. Assert: response JSON has `data.status` field ("ALLOTTED" or "NOT_ALLOTTED")
      5. Assert: response JSON has `data.shares_allotted` field (number)
    Expected Result: 200 OK with allotment status data
    Failure Indicators: 503/502 error, empty data, legacy checker being called
    Evidence: .sisyphus/evidence/task-4-allotment-check.json

  Scenario: Allotment check with unresolved code triggers live resolution (edge case)
    Tool: Bash (curl + psql)
    Preconditions: Server running, DB accessible
    Steps:
      1. Delete resolved code: docker exec ipo-backend-db-1 psql -U postgres -d ipo_db -c "UPDATE ipo_registrar_codes SET registrar_company_code = NULL, is_resolved = false WHERE ipo_id = 'f6e09b37-5128-4ba1-b220-93818096f366'"
      2. Call allotment check: curl -s -X POST -H 'Content-Type: application/json' http://localhost:8080/api/v2/allotment/check -d '{"ipo_id":"f6e09b37-5128-4ba1-b220-93818096f366","pan":"AQNPN9478L"}'
      3. Assert: handler attempts live resolution (check logs or response)
      4. Assert: response is either success (resolved live) or descriptive error (not 500/panic)
    Expected Result: Live resolution attempt, either succeeds or returns 503 with message
    Failure Indicators: 500 error, panic, nil pointer
    Evidence: .sisyphus/evidence/task-4-live-resolution.json

  Scenario: Unknown registrar returns descriptive error (error path)
    Tool: Bash (curl + psql)
    Preconditions: Server running, DB accessible
    Steps:
      1. Insert fake IPO with unknown registrar: docker exec ipo-backend-db-1 psql -U postgres -d ipo_db -c "UPDATE ipo_list SET registrar = 'Unknown Corp' WHERE id = 'f6e09b37-5128-4ba1-b220-93818096f366'"
      2. Call allotment check: curl -s -X POST -H 'Content-Type: application/json' http://localhost:8080/api/v2/allotment/check -d '{"ipo_id":"f6e09b37-5128-4ba1-b220-93818096f366","pan":"AQNPN9478L"}'
      3. Assert: HTTP 503 or similar, JSON error message mentioning "unsupported registrar"
      4. CLEANUP: Restore original registrar value
    Expected Result: 503 with "Unsupported registrar" error
    Failure Indicators: 500, panic, empty response
    Evidence: .sisyphus/evidence/task-4-unknown-registrar.json
  ```

  **Evidence to Capture:**
  - [ ] task-4-allotment-check.json — successful allotment check response
  - [ ] task-4-live-resolution.json — live resolution attempt response
  - [ ] task-4-unknown-registrar.json — error response for unknown registrar

  **Commit**: YES (standalone)
  - Message: `feat(allotment): rewire handler to use registrar CheckAllotment instead of legacy scraper`
  - Files: `handlers/v2_allotment_handler.go`
  - Pre-commit: `go build ./... && go test ./handlers/...`

- [ ] 5. Add admin endpoint `POST /api/v2/admin/registrar/resolve` for manual code resolution

  **What to do**:
  - Add `RegistrarCodeService` field to `AdminHandler` struct in `handlers/admin_handler.go`
  - Update `NewAdminHandler` constructor to accept and store `*services.RegistrarCodeService`
  - Add method `TriggerRegistrarResolve(c *fiber.Ctx) error` following the `TriggerDailyUpdate` pattern:
    ```go
    func (h *AdminHandler) TriggerRegistrarResolve(c *fiber.Ctx) error {
        if h.RegistrarCodeService == nil {
            return c.Status(503).JSON(fiber.Map{"error": "registrar code service not available"})
        }
        start := time.Now()
        codes, err := h.RegistrarCodeService.GetUnresolvedForToday(c.Context())
        if err != nil {
            return c.Status(500).JSON(fiber.Map{"error": err.Error()})
        }
        resolved := 0
        for _, code := range codes {
            _, err := h.RegistrarCodeService.ResolveCode(c.Context(), code.IPOID, code.RegistrarShortCode, *code.IPOName)
            if err == nil { resolved++ }
        }
        return c.JSON(fiber.Map{"success": true, "resolved_count": resolved, "total": len(codes), "duration": time.Since(start).String()})
    }
    ```
  - Wire in `internal/app/server.go`:
    - Pass `registrarCodeService` to `NewAdminHandler` (it's already created at line 87)
    - Register route: `v2Admin.Post("/registrar/resolve", v2AdminHandler.TriggerRegistrarResolve)` in the v2Admin group (lines 346-353)
  - Add nil check for `code.IPOName` before dereferencing (it's `*string`)

  **Must NOT do**:
  - Do NOT add authentication beyond what the admin group already has (`adminAuthMiddleware`)
  - Do NOT make it async/background — synchronous call, return when done
  - Do NOT create a new handler file — add method to existing `admin_handler.go`

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-file change (handler + server wiring) but follows clear established pattern. More than `quick` but not `deep`.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 4, 6 in Wave 2)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 6 (shell script calls this endpoint), Task 7 (E2E verification)
  - **Blocked By**: Task 1 (admin endpoint calls `ResolveCode` which calls `GetActiveIPOs`)

  **References** (CRITICAL - Be Exhaustive):

  **Pattern References**:
  - `handlers/admin_handler.go:102-126` — `TriggerDailyUpdate` method — COPY THIS PATTERN EXACTLY. Same structure: nil check, timing, loop, JSON response.
  - `handlers/admin_handler.go:20-35` — `AdminHandler` struct and `NewAdminHandler` constructor. Add `RegistrarCodeService` field here.
  - `internal/app/server.go:86-87` — `registrarCodeService` is ALREADY created. Just pass it to `NewAdminHandler`.
  - `internal/app/server.go:108-115` — `NewAdminHandler` call site. Add the new parameter here.
  - `internal/app/server.go:346-353` — v2Admin route group with `adminAuthMiddleware`. Register the new route here.

  **API/Type References**:
  - `services/registrar_code_service.go:135-149` — `GetUnresolvedForToday(ctx) ([]models.RegistrarCode, error)` — returns codes needing resolution.
  - `services/registrar_code_service.go:31-80` — `ResolveCode(ctx, ipoID, shortCode, ipoName)` — the resolution function to call per code.
  - `models/registrar_code.go:10-30` — `RegistrarCode` struct. `IPOName` is `*string` (pointer, needs nil check). `IPOID` is `uuid.UUID`.

  **WHY Each Reference Matters**:
  - `TriggerDailyUpdate` (lines 102-126): This is your blueprint. Same structure, same error handling, same response format. Don't innovate — copy.
  - `server.go:86-87`: The service already exists. You just need to pass it through. Don't create a new instance.
  - v2Admin group (lines 346-353): This group already has auth middleware. Your route automatically gets protected.

  **Acceptance Criteria**:
  - [ ] `POST /api/v2/admin/registrar/resolve` with valid admin token returns `{"success":true, "resolved_count":N, ...}`
  - [ ] Without admin token returns 401/403
  - [ ] When `RegistrarCodeService` is nil, returns 503
  - [ ] `go build ./...` passes
  - [ ] `go test ./...` passes

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Admin triggers registrar code resolution (happy path)
    Tool: Bash (curl)
    Preconditions: Server running on port 8080, ADMIN_TOKEN set in .env
    Steps:
      1. Read admin token from .env: grep ADMIN_TOKEN .env
      2. Call endpoint: curl -s -X POST -H 'X-Admin-Token: <token>' http://localhost:8080/api/v2/admin/registrar/resolve
      3. Assert: HTTP 200
      4. Assert: JSON has "success":true
      5. Assert: JSON has "resolved_count" field (number >= 0)
      6. Assert: JSON has "duration" field
    Expected Result: 200 with success response and resolution count
    Failure Indicators: 500, 404, missing fields
    Evidence: .sisyphus/evidence/task-5-admin-resolve.json

  Scenario: Admin endpoint requires auth (security)
    Tool: Bash (curl)
    Preconditions: Server running
    Steps:
      1. Call without token: curl -s -X POST http://localhost:8080/api/v2/admin/registrar/resolve
      2. Assert: HTTP 401 or 403
      3. Call with wrong token: curl -s -X POST -H 'X-Admin-Token: wrong' http://localhost:8080/api/v2/admin/registrar/resolve
      4. Assert: HTTP 401 or 403
    Expected Result: Both requests rejected
    Failure Indicators: 200 without auth, 500 error
    Evidence: .sisyphus/evidence/task-5-admin-auth.json

  Scenario: Verify codes actually resolved in DB (integration)
    Tool: Bash (psql)
    Preconditions: Admin resolve endpoint called successfully
    Steps:
      1. Query DB: docker exec ipo-backend-db-1 psql -U postgres -d ipo_db -c "SELECT ipo_name, registrar_company_code, is_resolved, match_score FROM ipo_registrar_codes"
      2. Assert: at least one row has is_resolved = true and registrar_company_code IS NOT NULL
    Expected Result: Resolved codes visible in DB
    Failure Indicators: All rows still is_resolved = false
    Evidence: .sisyphus/evidence/task-5-db-verification.txt
  ```

  **Evidence to Capture:**
  - [ ] task-5-admin-resolve.json — successful resolution response
  - [ ] task-5-admin-auth.json — auth rejection responses
  - [ ] task-5-db-verification.txt — psql output showing resolved codes

  **Commit**: YES (standalone)
  - Message: `feat(admin): add registrar code resolution admin endpoint`
  - Files: `handlers/admin_handler.go`, `internal/app/server.go`
  - Pre-commit: `go build ./... && go test ./...`

- [ ] 6. Update `scripts/run-all-scrapers.sh` with registrar code resolution step

  **What to do**:
  - Add Step 3 to `scripts/run-all-scrapers.sh` after the existing scraper steps:
    ```bash
    echo ""
    echo "========================================"
    echo " Step 3: Registrar Code Resolution"
    echo "========================================"
    echo ""
    
    # Only resolve if admin endpoint is available
    ADMIN_TOKEN=${ADMIN_TOKEN:-""}
    if [ -n "$ADMIN_TOKEN" ]; then
      echo "Triggering registrar code resolution..."
      RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
        -H "X-Admin-Token: $ADMIN_TOKEN" \
        "${API_BASE_URL:-http://localhost:8080}/api/v2/admin/registrar/resolve")
      HTTP_CODE=$(echo "$RESPONSE" | tail -1)
      BODY=$(echo "$RESPONSE" | head -1)
      if [ "$HTTP_CODE" = "200" ]; then
        echo "✓ Registrar codes resolved: $BODY"
      else
        echo "✗ Registrar code resolution failed (HTTP $HTTP_CODE): $BODY"
      fi
    else
      echo "⚠ Skipping registrar code resolution (ADMIN_TOKEN not set)"
    fi
    ```
  - Ensure the script uses the `ADMIN_TOKEN` environment variable (already used in other steps or set from .env)
  - Place this AFTER scraper steps (which populate `ipo_list`) but BEFORE any allotment-dependent steps

  **Must NOT do**:
  - Do NOT add date-based logic to the shell script itself — the admin endpoint handles scheduling internally via `GetUnresolvedForToday`
  - Do NOT add complex bash logic — keep it simple curl call like existing steps
  - Do NOT modify existing Step 1 and Step 2 in the script

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Adding ~20 lines of bash to an existing script. Trivial.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 5 for the endpoint to exist)
  - **Parallel Group**: Wave 2 (sequential after Task 5)
  - **Blocks**: Task 7 (E2E verification)
  - **Blocked By**: Task 5 (admin endpoint must exist first)

  **References** (CRITICAL - Be Exhaustive):

  **Pattern References**:
  - `scripts/run-all-scrapers.sh:1-46` — The ENTIRE existing script. Look at the banner/echo pattern for Step 1 and Step 2 — match this style exactly for Step 3.
  - `scripts/run-all-scrapers.sh:1-5` — Shebang and env var setup. Check if `ADMIN_TOKEN` is already referenced.

  **External References**:
  - Admin endpoint contract: `POST /api/v2/admin/registrar/resolve` with `X-Admin-Token` header → `{"success":true, "resolved_count":N}`

  **WHY Each Reference Matters**:
  - Existing script pattern (lines 1-46): Your addition must look identical in style — same echo format, same error handling, same variable naming.

  **Acceptance Criteria**:
  - [ ] `scripts/run-all-scrapers.sh` has Step 3 for registrar code resolution
  - [ ] Step 3 uses `ADMIN_TOKEN` env var and skips if not set
  - [ ] Step 3 calls `POST /api/v2/admin/registrar/resolve` with correct header
  - [ ] Script is still valid bash (`bash -n scripts/run-all-scrapers.sh` passes)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Script syntax is valid (smoke test)
    Tool: Bash
    Preconditions: None
    Steps:
      1. Run: bash -n scripts/run-all-scrapers.sh
      2. Assert: exit code 0 (no syntax errors)
    Expected Result: Clean syntax check
    Failure Indicators: Non-zero exit code, syntax error messages
    Evidence: .sisyphus/evidence/task-6-syntax-check.txt

  Scenario: Step 3 section exists in script (content check)
    Tool: Bash (grep)
    Preconditions: None
    Steps:
      1. Run: grep -c 'registrar' scripts/run-all-scrapers.sh (case-insensitive)
      2. Assert: count > 0
      3. Run: grep 'admin/registrar/resolve' scripts/run-all-scrapers.sh
      4. Assert: endpoint URL found
    Expected Result: Script contains registrar resolution step
    Failure Indicators: grep returns 0 matches
    Evidence: .sisyphus/evidence/task-6-content-check.txt
  ```

  **Evidence to Capture:**
  - [ ] task-6-syntax-check.txt — bash -n output
  - [ ] task-6-content-check.txt — grep output showing Step 3 content

  **Commit**: YES (standalone)
  - Message: `chore(scripts): add registrar code resolution step to run-all-scrapers.sh`
  - Files: `scripts/run-all-scrapers.sh`
  - Pre-commit: `bash -n scripts/run-all-scrapers.sh`

### Wave 3 — End-to-End Verification

- [ ] 7. End-to-end integration test: full allotment check flow

  **What to do**:
  - Verify the COMPLETE flow works: IPO in DB → registrar code resolved → allotment checked via registrar API → response returned to client
  - Steps to execute:
    1. Ensure server is running (`go run . &` or verify port 8080)
    2. Ensure DB is running with IPO data (Accord Transformers should be in `ipo_list`)
    3. Trigger registrar code resolution via admin endpoint: `POST /api/v2/admin/registrar/resolve`
    4. Verify code was resolved: query `ipo_registrar_codes` table for `is_resolved = true`
    5. Call allotment check: `POST /api/v2/allotment/check` with Accord Transformers IPO ID + PAN `AQNPN9478L`
    6. Verify response has allotment status data (not error)
    7. Test error scenarios: invalid PAN format, non-existent IPO ID, IPO without registrar
  - Run full test suite: `go test ./...`
  - Run build: `go build ./...`
  - Capture all evidence

  **Must NOT do**:
  - Do NOT modify any source code in this task — verification only
  - Do NOT add test data to DB manually — use existing data from scraper runs
  - Do NOT skip error scenario testing

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Multi-step verification requiring DB queries, API calls, log analysis, and evidence collection across the entire system.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (sequential, after all implementation)
  - **Blocks**: F1-F4 (Final verification wave)
  - **Blocked By**: Tasks 1, 2, 3, 4, 5, 6 (ALL implementation tasks)

  **References** (CRITICAL - Be Exhaustive):

  **Pattern References**:
  - `.sisyphus/plans/registrar-fix-and-integration.md` — This plan file. Read "Definition of Done" and "Success Criteria" sections for exact commands and expected outputs.

  **API/Type References**:
  - Admin endpoint: `POST /api/v2/admin/registrar/resolve` with `X-Admin-Token` header
  - Allotment check: `POST /api/v2/allotment/check` with `{"ipo_id":"...","pan":"..."}`
  - Expected response shape: `{"data":{"status":"...","shares_allotted":...,"shares_applied":...,"application_number":"...","applicant_name":"...","category":"..."}}`

  **Test Data**:
  - IPO: Accord Transformer & Switchgear Ltd. — ID: `f6e09b37-5128-4ba1-b220-93818096f366`
  - Registrar: KFIN
  - PAN: `AQNPN9478L`
  - Admin token: read from `.env` (`ADMIN_TOKEN` variable)

  **WHY Each Reference Matters**:
  - Plan's "Definition of Done": These are the EXACT curl commands that must work. Copy them verbatim.
  - Test data: These specific values were confirmed during investigation. Accord Transformers is in the DB, KFIN is the registrar.

  **Acceptance Criteria**:
  - [ ] `go build ./...` → passes
  - [ ] `go test ./...` → all pass
  - [ ] Admin resolve endpoint returns `{"success":true, "resolved_count":>=1}`
  - [ ] `ipo_registrar_codes` shows `is_resolved = true` for Accord Transformers
  - [ ] Allotment check returns HTTP 200 with `data.status` field
  - [ ] Invalid PAN returns descriptive error (not 500)
  - [ ] Non-existent IPO ID returns 404

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full end-to-end allotment check flow (integration)
    Tool: Bash (curl + psql)
    Preconditions: Server running, DB running, IPO data loaded
    Steps:
      1. Check server health: curl -s http://localhost:8080/api/v2/ipos/feed | head -c 200
      2. Read admin token: grep ADMIN_TOKEN .env | cut -d'=' -f2
      3. Resolve codes: curl -s -X POST -H 'X-Admin-Token: <token>' http://localhost:8080/api/v2/admin/registrar/resolve
      4. Assert resolve response: "success":true, "resolved_count" >= 1
      5. Verify DB: docker exec ipo-backend-db-1 psql -U postgres -d ipo_db -c "SELECT ipo_name, registrar_company_code, is_resolved FROM ipo_registrar_codes WHERE ipo_id = 'f6e09b37-5128-4ba1-b220-93818096f366'"
      6. Assert: is_resolved = true, registrar_company_code IS NOT NULL
      7. Check allotment: curl -s -X POST -H 'Content-Type: application/json' http://localhost:8080/api/v2/allotment/check -d '{"ipo_id":"f6e09b37-5128-4ba1-b220-93818096f366","pan":"AQNPN9478L"}'
      8. Assert: HTTP 200, response has data.status, data.shares_allotted fields
    Expected Result: Complete flow works — code resolved, allotment data returned
    Failure Indicators: Any step returns error, 503, 502, or empty data
    Evidence: .sisyphus/evidence/task-7-e2e-flow.json

  Scenario: Invalid PAN format returns error (negative)
    Tool: Bash (curl)
    Preconditions: Server running
    Steps:
      1. Call: curl -s -X POST -H 'Content-Type: application/json' http://localhost:8080/api/v2/allotment/check -d '{"ipo_id":"f6e09b37-5128-4ba1-b220-93818096f366","pan":"invalid"}'
      2. Assert: HTTP 400 or descriptive error
    Expected Result: Validation error with message about PAN format
    Failure Indicators: 200 with empty data, 500 error, panic
    Evidence: .sisyphus/evidence/task-7-invalid-pan.json

  Scenario: Non-existent IPO returns 404 (negative)
    Tool: Bash (curl)
    Preconditions: Server running
    Steps:
      1. Call: curl -s -X POST -H 'Content-Type: application/json' http://localhost:8080/api/v2/allotment/check -d '{"ipo_id":"00000000-0000-0000-0000-000000000000","pan":"AQNPN9478L"}'
      2. Assert: HTTP 404 with "IPO not found" message
    Expected Result: 404 with descriptive error
    Failure Indicators: 200, 500, panic
    Evidence: .sisyphus/evidence/task-7-missing-ipo.json

  Scenario: Build and test suite pass (regression)
    Tool: Bash
    Preconditions: All implementation complete
    Steps:
      1. Run: go build ./...
      2. Assert: exit code 0
      3. Run: go test ./...
      4. Assert: all tests pass
    Expected Result: PASS for both
    Failure Indicators: Build errors, test failures
    Evidence: .sisyphus/evidence/task-7-build-tests.txt
  ```

  **Evidence to Capture:**
  - [ ] task-7-e2e-flow.json — complete flow: resolve + check + DB verification
  - [ ] task-7-invalid-pan.json — validation error response
  - [ ] task-7-missing-ipo.json — 404 response
  - [ ] task-7-build-tests.txt — go build + go test output

  **Commit**: YES (standalone)
  - Message: `test(e2e): verify end-to-end allotment check flow`
  - Files: test files + evidence
  - Pre-commit: `go build ./... && go test ./...`

---


## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Rejection → fix → re-run.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in `.sisyphus/evidence/`. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...` + `go vet ./...` + `go test ./...`. Review all changed files for: type assertion without ok check, empty error handling, `fmt.Println` in production, commented-out code, unused imports. Check for AI slop: excessive comments, over-abstraction, generic variable names.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration: resolve registrar code via admin → check allotment via API. Test edge cases: unknown registrar, invalid PAN, unresolved code. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (`git diff`). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance: no modifications to `ipo_list.registrar_company_code`, no new `job_dispatch` table, no removal of legacy `AllotmentChecker`. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Wave 1**: `fix(registrar): rewrite Kfin GetActiveIPOs to use API instead of HTML scraping` — `tools/registrars/kfin/client.go`
- **Wave 1**: `fix(scheduler): case-insensitive extractRegistrarShortCode + ResultDate nil check` — `jobs/registrar_code_scheduler.go`, `jobs/fetch_registrar_code_job.go`
- **Wave 2**: `feat(allotment): rewire handler to use registrar CheckAllotment instead of legacy scraper` — `handlers/v2_allotment_handler.go`
- **Wave 2**: `feat(admin): add registrar code resolution admin endpoint` — `handlers/admin_handler.go`, `internal/app/server.go`
- **Wave 2**: `chore(scripts): add registrar code resolution step to run-all-scrapers.sh` — `scripts/run-all-scrapers.sh`
- **Wave 3**: `test(e2e): verify end-to-end allotment check flow` — test files + evidence

---

## Success Criteria

### Verification Commands
```bash
go build ./...           # Expected: no errors
go test ./...            # Expected: all tests pass
curl -X POST http://localhost:8080/api/v2/allotment/check \
  -H 'Content-Type: application/json' \
  -d '{"ipo_id":"f6e09b37-5128-4ba1-b220-93818096f366","pan":"AQNPN9478L"}'
# Expected: 200 OK with {"data":{"status":"ALLOTTED"|"NOT_ALLOTTED","shares_applied":...,"shares_allotted":...,"message":"..."}}

curl -X POST http://localhost:8080/api/v2/admin/registrar/resolve \
  -H 'X-Admin-Token: <ADMIN_TOKEN>'
# Expected: 200 OK with {"success":true,"resolved_count":N,"duration":"..."}
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass
- [ ] Server starts without errors
- [ ] Allotment check returns real data for Accord Transformers
