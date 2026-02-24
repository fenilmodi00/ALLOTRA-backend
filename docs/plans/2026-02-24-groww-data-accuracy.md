# Groww Data Accuracy & Status Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ensure Groww-only IPOs (like Flipkart, Zepto) preserve their API-provided status when dates are missing, fix v2 feed status filtering, skip non-IPO section slugs, and eliminate noise from expected CMS 404s.

**Architecture:** 
1. Use Groww's `details.status` as a fallback when calculating status if dates are absent.
2. Update the v2 API feed to correctly filter data records based on the user-provided status query parameter.
3. Treat known section tabs (`/sme`, `/mainboard`) as non-IPO slugs in the discovery scraper.
4. Enhance `fetchJSON` to return strongly typed HTTP errors, and handle CMS 404s cleanly (no retries, no breaker trips).
5. For missing dates (Unknown), we will map string outputs to "TBA" for better UX.

**Tech Stack:** Go 1.21+, Fiber, PostgreSQL (goose), logrus, test-driven-development.

---

### Task 1: Map native Groww status & TBA fallback in Mapper

**Files:**
- Modify: `services/groww_mapper.go:88-100`

**Step 1: Write the failing test**

```go
// Add to services/groww_mapper_test.go
func TestMapGrowwToIPO_StatusAndTBA(t *testing.T) {
	t.Parallel()
	details := &models.GrowwIPODetailsResponse{
		CompanyName: "Test TBA",
		Status:      "UPCOMING",
		StartDate:   "",
		EndDate:     "",
	}
	ipo := &models.GrowwScrapedIPO{
		Slug:    "test-tba",
		Details: details,
	}
	
	mapper := NewGrowwMapper(nil)
	result := mapper.MapGrowwToIPO(ipo)
	
	if result.Status != "UPCOMING" {
		t.Errorf("expected status UPCOMING, got %s", result.Status)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./services -run TestMapGrowwToIPO_StatusAndTBA`
Expected: FAIL.

**Step 3: Write minimal implementation**

In `services/groww_mapper.go` inside `MapGrowwToIPO`:
```go
	// Default status
	ipo.Status = "UNKNOWN"
	
	// Try dates first
	if ipo.OpenDate != nil || ipo.CloseDate != nil || ipo.ListingDate != nil {
		ipo.Status = m.utility.CalculateIPOStatus(ipo.OpenDate, ipo.CloseDate, ipo.ListingDate)
	} else if data.Details != nil && data.Details.Status != "" {
		// Fallback to Groww's native status if dates are missing
		ipo.Status = data.Details.Status
	}
```

**Step 4: Run test to verify it passes**

Run: `go test ./services -run TestMapGrowwToIPO_StatusAndTBA`
Expected: PASS.

**Step 5: Commit**

```bash
git add services/groww_mapper.go services/groww_mapper_test.go
git commit -m "fix(scraper): map native groww status when dates are missing"
```

---

### Task 2: Preserve existing Status in Recalculate if dates are empty

**Files:**
- Modify: `services/ipo_service.go:417-425`
- Modify: `services/utility_service.go:780-811`

**Step 1: Write the failing test**

```go
// Add to services/utility_service_test.go
func TestCalculateIPOStatus_Fallback(t *testing.T) {
	t.Parallel()
	svc := NewUtilityService()
	status := svc.CalculateIPOStatus(nil, nil, nil, "UPCOMING")
	if status != "UPCOMING" {
		t.Errorf("expected UPCOMING, got %s", status)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./services -run TestCalculateIPOStatus_Fallback`
Expected: Compile error or logic failure.

**Step 3: Write minimal implementation**

Update `services/utility_service.go`:
```go
func (s *UtilityService) CalculateIPOStatus(openDate, closeDate, listingDate *time.Time, existingStatus string) string {
	now := time.Now()
	
	if listingDate != nil && now.After(*listingDate) {
		return "LISTED"
	}
	if closeDate != nil && now.After(*closeDate) {
		return models.StatusClosed
	}
	if openDate != nil && now.After(*openDate) {
		return "ACTIVE"
	}
	if openDate != nil && now.Before(*openDate) {
		return models.StatusUpcoming
	}
	
	if existingStatus != "" && existingStatus != "UNKNOWN" {
		return existingStatus
	}
	return "UNKNOWN"
}
```

Update `services/ipo_service.go` lines 417-425 to pass existing status:
```go
func (s *IPOService) recalculateStatus(ipo *models.IPO) {
	ipo.Status = s.UtilityService.CalculateIPOStatus(ipo.OpenDate, ipo.CloseDate, ipo.ListingDate, ipo.Status)
}

func (s *IPOService) recalculateStatusWithGMP(ipo *models.IPOWithGMP) {
	ipo.Status = s.UtilityService.CalculateIPOStatus(ipo.OpenDate, ipo.CloseDate, ipo.ListingDate, ipo.Status)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./services -run TestCalculateIPOStatus`
Expected: PASS.

**Step 5: Commit**

```bash
git add services/utility_service.go services/utility_service_test.go services/ipo_service.go
git commit -m "fix(services): preserve existing status when timeline dates are missing"
```

---

### Task 3: Fix V2 Feed Status Filtering

**Files:**
- Modify: `services/ipo_service.go:1137-1266`
- Modify: `services/ipo_service.go:1275-1306`

**Step 1: Write the failing test**

```go
// Add to handlers/v2_ipo_handler_test.go
// Check that querying ?status=UPCOMING actually limits the returned results
```

**Step 2: Run test to verify it fails**

Run: `go test ./handlers -run TestV2IPOFeedFilter`
Expected: FAIL.

**Step 3: Write minimal implementation**

Update `GetActiveIPOsWithGMPPaginatedWithCount` in `services/ipo_service.go`:
Pass `statusFilter` down to `GetActiveIPOsWithGMPPaginated`.

```go
// Change signature to accept statusFilter
func (s *IPOService) GetActiveIPOsWithGMPPaginated(ctx context.Context, statusFilter string, limit, offset int) ([]models.IPOWithGMP, error) {
    // ...
    baseWhere := "1=1"
    args := []interface{}{limit, offset}
    
    if statusFilter != "" && statusFilter != "all" {
        baseWhere = "i.status = $3"
        args = append(args, statusFilter)
    }
    
    query := fmt.Sprintf(`
        SELECT ...
        FROM ipo_list i
        LEFT JOIN LATERAL ...
        WHERE %s
        ORDER BY ...
        LIMIT $1 OFFSET $2
    `, baseWhere, ...)
    // ...
}
```

Update caller in `GetActiveIPOsWithGMPPaginatedWithCount`:
```go
	// Get paginated results
	ipos, err := s.GetActiveIPOsWithGMPPaginated(ctx, statusFilter, limit, offset)
```

**Step 4: Run test to verify it passes**

Run: `go test ./services -run TestGetActiveIPOsWithGMP`
Expected: PASS.

**Step 5: Commit**

```bash
git add services/ipo_service.go
git commit -m "fix(db): apply status filter in paginated IPO query"
```

---

### Task 4: Exclude section slugs in Groww Discovery

**Files:**
- Modify: `services/groww_scraper_service.go:286-317`

**Step 1: Write the failing test**

```go
// Add to services/groww_scraper_service_test.go
func TestDiscoverSlugs_SkipsSections(t *testing.T) {
	// mock discovery extracting "/ipo/sme" and "/ipo/mainboard"
	// assert they are not in the returned slugs
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./services -run TestDiscoverSlugs_SkipsSections`
Expected: FAIL.

**Step 3: Write minimal implementation**

Update `DiscoverSlugs` in `services/groww_scraper_service.go`:
```go
		// Skip known section tabs
		skipSlugs := map[string]bool{
			"open": true, "closed": true, "upcoming": true, 
			"gmp": true, "allotment": true,
			"sme": true, "mainboard": true,
		}
		if skipSlugs[slug] {
			continue
		}
```

**Step 4: Run test to verify it passes**

Run: `go test ./services -run TestDiscoverSlugs`
Expected: PASS.

**Step 5: Commit**

```bash
git add services/groww_scraper_service.go services/groww_scraper_service_test.go
git commit -m "fix(scraper): exclude sme and mainboard section paths from discovery"
```

---

### Task 5: Handle CMS 404 cleanly (No Retry, No Circuit Trip)

**Files:**
- Modify: `shared/http_client.go` (create HTTPError type)
- Modify: `services/groww_scraper_service.go:132-225`

**Step 1: Write the failing test**

```go
// Add to services/groww_scraper_service_test.go
// Mock CMS returning 404, verify it only attempts ONCE and does not error out the whole struct
```

**Step 2: Run test to verify it fails**

Run: `go test ./services -run TestFetchCMSContent_404`

**Step 3: Write minimal implementation**

In `shared/http_client.go`:
```go
// HTTPError represents an HTTP status error
type HTTPError struct {
	StatusCode int
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("GET %s returned HTTP %d", e.URL, e.StatusCode)
}
```

In `services/groww_scraper_service.go` `FetchCMSContent`:
```go
		if resp.StatusCode == 404 {
			// Do not retry 404s for CMS
			return &shared.HTTPError{StatusCode: 404, URL: url}
		}
```

In `ScrapeIPO`:
```go
	err = shared.RetryWithExponentialBackoff(func() error {
		cmsData, err = s.FetchCMSContent(ctx, slug)
		var httpErr *shared.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == 404 {
			// Accept 404 as terminal, but non-fatal
			return nil 
		}
		return err
	}, shared.DefaultRetryConfig(), s.logger)
```

**Step 4: Run test to verify it passes**

Run: `go test ./services -run TestFetchCMSContent`

**Step 5: Commit**

```bash
git add shared/http_client.go services/groww_scraper_service.go
git commit -m "fix(scraper): treat CMS 404 as non-retryable benign error"
```

---

### Task 6: Date TBA Mapping for v2 API

**Files:**
- Modify: `handlers/v2_ipo_handler.go:42-48`

**Step 1: Write the failing test**

```go
// Add to handlers/v2_ipo_handler_test.go
// Verify if OpenDate is nil, it returns "TBA" in JSON instead of null.
```

**Step 2: Write minimal implementation**

Update `handlers/v2_ipo_handler.go`:
```go
func formatDatePtr(t *time.Time) *string {
	if t == nil {
		tba := "TBA"
		return &tba
	}
	d := t.Format(time.RFC3339)
	return &d
}
```

**Step 3: Run test to verify it passes**

Run: `go test ./handlers -run TestV2IPOHandler`
Expected: PASS.

**Step 4: Commit**

```bash
git add handlers/v2_ipo_handler.go
git commit -m "feat(api): map missing dates to TBA in v2 responses"
```
