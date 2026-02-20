# GMP History Charting Implementation Plan

## Overview
Add historical GMP (Grey Market Premium) price movement charts to the IPO dashboard by:
1. Building InvestorGain API client with typed DTOs
2. Updating scraper to parse from API response
3. Fixing negative GMP validation
4. Enhancing chart endpoint contract
5. Adding backfill endpoint

## Pre-requisites
- [x] Worktree created at `.worktrees/gmp-history-charting`
- [x] Dependencies downloaded

---

## Task 1: Build InvestorGain API Client with Typed DTOs

### Steps
1. Create `services/investorgain_api_client.go`
2. Define DTO structs for:
   - `IPOUrlListResponse` - maps company names to IDs
   - `IPOGMPResponse` - contains `ipoGmpTable` HTML and `ipoGmpData` JSON
   - `IPOGmpDataPoint` - individual GMP data entry
3. Implement HTTP client with base URL `https://webnodejs.investorgain.com/cloud/new/ipo/`
4. Add methods:
   - `GetIPOUrlList()` - fetch company name to ID mapping
   - `GetIPOGMPData(ipoID string)` - fetch GMP data for specific IPO

### Verification
- `go build ./services/...` compiles without errors

### Time estimate
3 minutes

---

## Task 2: Update Scraper to Parse from API Response

### Steps
1. Read current `services/gmp_history_scraper.go`
2. Update scraper to:
   - Call `investorgain_api_client.GetIPOGMPData()` instead of HTML scraping
   - Parse `ipoGmpData` JSON field instead of HTML table
   - Extract: record_date, gmp, ipo_price, listing_date
3. Keep existing error handling and logging

### Verification
- `go build ./services/...` compiles without errors

### Time estimate
3 minutes

---

## Task 3: Fix Negative GMP Validation

### Steps
1. Read `database/migrations/001_add_gmp_price_history.sql`
2. Remove or adjust CHECK constraint that blocks negative GMP values
3. Update any validation in `services/gmp_history_service.go` that rejects negative GMP

### Verification
- Migration can be re-run or new migration created
- Go code compiles

### Time estimate
2 minutes

---

## Task 4: Enhance Chart Endpoint Contract

### Steps
1. Read `models/gmp.go`
2. Add `IpoPrice` field to `ChartPoint` struct
3. Read `services/gmp_history_service.go`
4. Update `GetChartData()` to include `ipo_price` per data point
5. Read `handlers/gmp_history_handler.go`
6. Verify response includes new field

### Verification
- `go build ./...` compiles
- API response structure includes ipo_price

### Time estimate
3 minutes

---

## Task 5: Add Backfill Endpoint

### Steps
1. Read `handlers/gmp_history_handler.go`
2. Add new handler `HandleBackfillGMPHistory`
3. Register new route `/api/v1/gmp/backfill` in `internal/app/server.go`
4. Handler should:
   - Accept POST request
   - Fetch all IPO IDs
   - Call scraper for each IPO
   - Return progress/status

### Verification
- `go build ./...` compiles
- Route registered correctly

### Time estimate
4 minutes

---

## Task 6: Final Verification

### Steps
1. Run `go build ./...`
2. Run existing tests if any
3. Verify all files modified correctly

### Time estimate
2 minutes

---

## Completion Criteria
- All 6 tasks completed
- Code compiles without errors
- Changes follow existing code patterns
