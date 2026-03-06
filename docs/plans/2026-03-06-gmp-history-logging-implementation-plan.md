# GMP History Scraper - Cleaner Logging Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make GMP history scraper logs clearer by categorizing failures and adding single summary logs per IPO instead of verbose multi-line logs.

**Architecture:** Define failure type constants, update scraper to return typed errors, update service to log clear summaries, add batch summary at end.

**Tech Stack:** Go, logrus, existing GMP scraper service

---

## Task 1: Define Failure Type Constants

**Files:**
- Modify: `services/gmp_history_scraper.go`

**Step 1: Add failure type constants at top of file**

Find where other constants are defined (around line 1-50), add:

```go
// GMPHistoryFailureType categorizes why GMP scraping failed
type GMPHistoryFailureType string

const (
    FailureTypeNOIGID     GMPHistoryFailureType = "NO_IG_ID"     // Could not find InvestorGain ID
    FailureTypeNOGMPData GMPHistoryFailureType = "NO_GMP_DATA"  // Found ID but no GMP data yet
    FailureTypeParseError GMPHistoryFailureType = "PARSE_ERROR"  // HTML parsing failed
    FailureTypeNetworkErr GMPHistoryFailureType = "NETWORK_ERROR" // HTTP request failed
)
```

**Step 2: Commit**

```bash
git add services/gmp_history_scraper.go
git commit -m "feat: add GMP failure type constants"
```

---

## Task 2: Create Typed Error Wrapper

**Files:**
- Modify: `services/gmp_history_scraper.go`

**Step 1: Add typed error struct**

Add after failure type constants (around line 50):

```go
// GMPHistoryError wraps errors with failure type for clearer logging
type GMPHistoryError struct {
    FailureType GMPHistoryFailureType
    Message     string
    Err         error
}

func (e *GMPHistoryError) Error() string {
    return fmt.Sprintf("[%s] %s: %v", e.FailureType, e.Message, e.Err)
}

func (e *GMPHistoryError) Unwrap() error {
    return e.Err
}

// NewGMPHistoryError creates a typed error
func NewGMPHistoryError(failureType GMPHistoryFailureType, message string, err error) *GMPHistoryError {
    return &GMPHistoryFailureType{
        FailureType: failureType,
        Message:     message,
        Err:         err,
    }
}
```

**Step 2: Fix typo in constructor**

Note: Fix `GMPHistoryFailureType` → `GMPHistoryError` in the return type.

**Step 3: Commit**

```bash
git add services/gmp_history_scraper.go
git commit -m "feat: add GMPHistoryError typed error wrapper"
```

---

## Task 3: Update findInvestorGainNumericID to Return Typed Errors

**Files:**
- Modify: `services/gmp_history_scraper.go:1050-1115`

**Step 1: Find the function and update return statements**

Replace error returns with typed errors:

```go
// Line ~1067: API URL list lookup failed
return "", NewGMPHistoryError(
    FailureTypeNOIGID,
    fmt.Sprintf("IPO %s (%s): not found in InvestorGain API URL list", ipoName, companyCode),
    err,
)

// Line ~1112: Listing page lookup failed  
return "", NewGMPHistoryError(
    FailureTypeNOIGID,
    fmt.Sprintf("IPO %s (%s): not found on InvestorGain listing page", ipoName, companyCode),
    err,
)
```

**Step 2: Commit**

```bash
git add services/gmp_history_scraper.go
git commit -m "feat: return typed errors from findInvestorGainNumericID"
```

---

## Task 4: Update ScrapeWithAPI to Return Typed Error for No GMP Data

**Files:**
- Modify: `services/gmp_history_scraper.go:180-190`

**Step 1: Find and update the "No GMP data available" log**

Replace:
```go
s.logger.WithError(err).Info("No GMP data available from API for IPO")
return nil, fmt.Errorf("failed to fetch GMP data from API: %w", err)
```

With:
```go
// Return typed error - this is expected for some upcoming IPOs
return nil, NewGMPHistoryError(
    FailureTypeNOGMPData,
    fmt.Sprintf("IPO %s: InvestorGain ID found but no GMP data available yet", companyCode),
    err,
)
```

**Step 2: Commit**

```bash
git add services/gmp_history_scraper.go
git commit -m "feat: return typed error for no GMP data case"
```

---

## Task 5: Update gmp_history_service.go to Log Clear Summary

**Files:**
- Modify: `services/gmp_history_service.go:875-900`

**Step 1: Update the error logging block**

Replace the current verbose error logging:

```go
if err != nil {
    // Determine failure type and log appropriate message
    var gmphErr *GMPHistoryError
    failureType := "UNKNOWN"
    message := err.Error()
    
    if errors.As(err, &gmphErr) {
        failureType = string(gmphErr.FailureType)
        message = gmphErr.Message
    }
    
    // Categorize based on failure type
    logLevel := logrus.WarnLevel
    if failureType == "PARSE_ERROR" || failureType == "NETWORK_ERROR" {
        logLevel = logrus.ErrorLevel
    }
    
    s.logger.WithFields(logrus.Fields{
        "ipo_id":        ipo.ID,
        "company_code":  ipo.CompanyCode,
        "stock_id":      ipo.StockID,
        "failure_type":  failureType,
        "is_upcoming":   ipo.Status == "UPCOMING",
    }).Logf(logLevel, "[%s] %s", failureType, message)
    
    // ... rest of error handling
}
```

**Step 2: Commit**

```bash
git add services/gmp_history_service.go
git commit -m "feat: log clear summary with failure type for each IPO"
```

---

## Task 6: Add Batch Summary at End of Job

**Files:**
- Modify: `services/gmp_history_service.go:950-970`

**Step 1: Find where results are returned and add summary**

Add before returning results:

```go
// Log batch summary
s.logger.WithFields(logrus.Fields{
    "total_ipos":      results.TotalProcessed,
    "success_count":   results.SuccessCount,
    "failure_count":   results.FailureCount,
    "records_added":   results.TotalRecordsAdded,
    "skipped_no_ig":   len(results.FailedIPOs), // These are expected for upcoming
}).Info("GMP History Backfill Complete")
```

**Step 2: Commit**

```bash
git add services/gmp_history_service.go
git commit -m "feat: add batch summary to GMP history job"
```

---

## Task 7: Run and Verify

**Step 1: Build the project**

```bash
go build ./...
```

**Step 2: Run the GMP history job**

```bash
go run cmd/main.go 2>&1 | head -100
```

**Step 3: Verify logs look cleaner**

Expected output should show:
```
[NO_IG_ID] IPO "Cult Fit" (cult-fit) - not found on InvestorGain listing. Stock ID: GROWW-cult-fit-ipo. Skipping.
[NO_GMP_DATA] IPO "Kent RO Systems" (kent-ro-systems) - InvestorGain ID found but no GMP data available yet.
```

And at end:
```
GMP History Backfill Complete: total=61 success=33 failure=28
```

**Step 4: Commit**

```bash
git commit -m "chore: verify GMP logging changes work correctly"
```

---

## Plan Complete

Two execution options:

1. **Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

2. **Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach?
