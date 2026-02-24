# Groww Coverage & Resilience Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ensure all relevant Groww IPO tab entries (Open/Closed/Upcoming/GMP/Allotment) are discovered and persisted, while making expected Groww 404 behavior non-fatal and transparent in logs/metrics.

**Architecture:** Keep Chittorgarh as broad fallback, but harden Groww discovery + fetch classification. Split Groww "details" and "CMS" failure semantics so CMS 404 does not look like total Groww failure. Expand discovery sources and add verification tests + endpoint-level checks for known slugs.

**Tech Stack:** Go 1.21+, Fiber, net/http, goquery, logrus, goose/postgres, table-driven tests, Playwright verification (manual/CI helper).

---

### Task 1: Add failing tests for Groww slug discovery coverage

**Files:**
- Modify: `services/groww_scraper_service_test.go`
- Reference: `services/groww_scraper_service.go:232-329`

**Step 1: Write failing test for tab URL set (includes GMP)**

```go
func TestDiscoverSlugs_UsesAllIPOTabs(t *testing.T) {
	t.Parallel()
	// Assert discovery hits:
	// /ipo, /ipo/open, /ipo/closed, /ipo/upcoming, /ipo/gmp, /ipo/allotment
}
```

**Step 2: Write failing test for extracting all `/ipo/{slug}` links from upcoming HTML fixture**

```go
func TestDiscoverSlugs_ParsesUpcomingTableLinks(t *testing.T) {
	t.Parallel()
	// feed fixture with links like /ipo/omnitech-engineering-ipo etc
	// assert deduped slugs include expected set
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./services -run DiscoverSlugs -v`  
Expected: FAIL because `/ipo/gmp` is not included and/or extraction misses expected slugs.

**Step 4: Commit failing tests**

```bash
git add services/groww_scraper_service_test.go
git commit -m "test: add failing coverage tests for Groww IPO tab discovery"
```

---

### Task 2: Implement discovery coverage fixes (minimal code)

**Files:**
- Modify: `services/groww_scraper_service.go:238-244`
- Modify: `services/groww_scraper_service.go:286-317`
- Optional create helper: `services/groww_scraper_service.go` (private helper for href normalization)

**Step 1: Add missing tab URL**

Add `https://groww.in/ipo/gmp` to `urlsToScrape`.

**Step 2: Harden href normalization**

Implement helper to normalize:
- absolute URL `https://groww.in/ipo/foo`
- relative `/ipo/foo`
- strip query + hash
- skip non-IPO section pages (`open`, `closed`, `upcoming`, `gmp`, `allotment`)

**Step 3: Keep dedupe stable + deterministic**

Sort final slug list before return so tests are stable.

**Step 4: Run tests**

Run: `go test ./services -run DiscoverSlugs -v`  
Expected: PASS.

**Step 5: Commit**

```bash
git add services/groww_scraper_service.go services/groww_scraper_service_test.go
git commit -m "feat: include Groww GMP tab and harden slug discovery parsing"
```

---

### Task 3: Add failing tests for Groww failure classification (Details vs CMS)

**Files:**
- Modify: `services/groww_scraper_service_test.go`
- Reference: `services/groww_scraper_service.go:85-192`

**Step 1: Add failing test: CMS 404 + Details 200 should be partial success**

```go
func TestScrapeIPO_CMS404_DetailsSuccess_IsNotTotalFailure(t *testing.T) {
	t.Parallel()
	// mock details=200 JSON, cms=404
	// expect result.Details != nil
	// expect result.CMSError != ""
	// expect behavior treated as Groww-details success by caller semantics
}
```

**Step 2: Add failing test: Details 404 should trigger fallback path signal**

```go
func TestScrapeIPO_Details404_RequiresFallback(t *testing.T) {
	t.Parallel()
	// details=404 -> DetailsError non-empty
}
```

**Step 3: Run tests**

Run: `go test ./services -run ScrapeIPO -v`  
Expected: FAIL until error classification is improved.

**Step 4: Commit failing tests**

```bash
git add services/groww_scraper_service_test.go
git commit -m "test: add failing tests for Groww details vs CMS failure semantics"
```

---

### Task 4: Implement non-fatal CMS 404 handling + cleaner logging semantics

**Files:**
- Modify: `services/groww_scraper_service.go:132-192`
- Modify: `services/groww_scraper_service.go:194-225`
- Modify: `jobs/daily_ipo_update.go:115-130`

**Step 1: Introduce typed HTTP error classification**

In scraper, wrap non-200 with typed metadata:
- status code
- endpoint kind (`details` or `cms`)
- slug

**Step 2: For CMS endpoint only**
- Do not retry on HTTP 404.
- Do not count CMS 404 toward breaker failures (or bypass breaker for CMS request).
- Return `CMSError` but keep run healthy if Details succeeded.

**Step 3: For Details endpoint**
- Keep retry/circuit breaker behavior.
- 404 means slug absent in details API -> fallback to Chittorgarh.

**Step 4: Fix job-level log wording**

In `daily_ipo_update.go`:
- If `Details != nil` and `CMSError != ""`, log:
  - `"Groww details fetched; CMS missing (non-fatal)"`
- If details fails, log fallback clearly.
- Avoid `"Scraped rich data success..."` when CMS missing.

**Step 5: Run tests**

Run:
- `go test ./services -run "ScrapeIPO|FetchIPODetails|FetchCMSContent" -v`
- `go test ./jobs -run DailyIPOUpdateJob -v`

Expected: PASS.

**Step 6: Commit**

```bash
git add services/groww_scraper_service.go jobs/daily_ipo_update.go services/groww_scraper_service_test.go
git commit -m "fix: classify Groww CMS 404 as non-fatal and improve job logging semantics"
```

---

### Task 5: Add endpoint/database verification for known slugs

**Files:**
- Modify: `tests/integration_test.go` (or existing integration suite file)
- Optional create: `tests/groww_ingestion_integration_test.go`

**Step 1: Add table-driven test for required slugs**

Use slugs:
- `omnitech-engineering-ipo`
- `pngs-reva-ipo`
- `shree-ram-twistex-ipo`

Validate after job run:
- record exists in `ipo_list`
- `stock_id`, `name`, `slug` non-empty
- logo url not empty if provided by Groww details

**Step 2: Add API assertion**

Hit `GET /api/v1/ipos` (or paginated endpoint) and assert these slugs/company codes appear.

**Step 3: Run tests**

Run:
- `go test ./tests -run GrowwIngestion -v`
- `go test ./tests -run API -v`

Expected: PASS.

**Step 4: Commit**

```bash
git add tests/groww_ingestion_integration_test.go tests/integration_test.go
git commit -m "test: verify required Groww slugs persist in DB and API"
```

---

### Task 6: GMP parser noise hardening (separate but visible log issue)

**Files:**
- Modify: `services/gmp_history_service.go` (date normalization path)
- Modify: related test file in `services/*gmp*test.go`

**Step 1: Add failing tests for date tokens like `"21-09-2023 Listing"`**

```go
func TestParseHistoryDate_StripsListingSuffix(t *testing.T) { ... }
```

**Step 2: Minimal fix**
- sanitize trailing non-date tokens (`Listing`, etc.) before parsing.

**Step 3: Run tests**
- `go test ./services -run GMPHistory -v`

**Step 4: Commit**
```bash
git add services/gmp_history_service.go services/gmp_history_service_test.go
git commit -m "fix: parse investor gain history dates with trailing tokens"
```

---

### Task 7: Verification, lint, race, docs

**Files:**
- Modify: `README.md` (Groww discovery/fallback behavior section)
- Modify: `docs/` relevant API or scraper docs

**Step 1: Run formatting + lint**
- `gofmt -w services/groww_scraper_service.go jobs/daily_ipo_update.go services/groww_scraper_service_test.go`
- `golangci-lint run`

Expected: clean.

**Step 2: Run race detector on affected tests**
- `go test -race ./services ./jobs ./tests -run "Groww|DailyIPO|GMPHistory" -v`

Expected: PASS.

**Step 3: Full smoke**
- Start app locally.
- Confirm logs show:
  - discovery includes GMP tab
  - CMS 404 treated as non-fatal (clear wording)
  - no misleading "rich data success" when CMS failed.

**Step 4: Commit docs + final changes**
```bash
git add README.md docs/
git commit -m "docs: document Groww tab discovery and partial-failure behavior"
```

---

## Manual verification checklist (with your exact links)

1. Open:
   - `https://groww.in/ipo/omnitech-engineering-ipo`
   - `https://groww.in/ipo/pngs-reva-ipo`
   - `https://groww.in/ipo/shree-ram-twistex-ipo`
2. Verify backend has these records:
   - DB query by slug/company_code
   - API response includes them
3. Confirm logs:
   - If CMS 404 only: not treated as total Groww failure
   - fallback only when details API fails
4. Confirm upcoming tab slugs from `https://groww.in/ipo/upcoming` are represented in DB after job run.
