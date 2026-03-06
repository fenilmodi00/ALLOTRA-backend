# GMP History Scraper - Cleaner Logging Design

**Date:** 2026-03-06  
**Status:** Approved

## Problem

The GMP history scraper logs are confusing and noisy:

1. **Deep error wrapping** - Long chains like `failed to build URL: failed to find InvestorGain numeric ID: no matching IPO found on InvestorGain listing page`
2. **Mixed log levels** - INFO, WARNING, ERROR used inconsistently
3. **Repetitive patterns** - Same search attempts logged 4+ times per IPO
4. **No categorization** - Can't quickly tell WHY it failed

This makes debugging and monitoring difficult, especially for the 28 upcoming IPOs that legitimately don't have InvestorGain data yet.

## Design

### 1. Failure Type Categories

Define clear failure types with short codes:

| Code | Meaning | Log Level |
|------|---------|-----------|
| `NO_IG_ID` | Could not find InvestorGain ID on listing page | WARNING |
| `NO_GMP_DATA` | Found ID but no GMP data available (no trading yet) | INFO |
| `PARSE_ERROR` | HTML parsing failed | ERROR |
| `NETWORK_ERROR` | HTTP request failed | ERROR |

### 2. Single Summary Log Per IPO

Replace multiple verbose logs with one clear summary:

```go
// BEFORE:
Searching for InvestorGain numeric ID
API URL list lookup failed, falling back to listing page scraping
Loaded InvestorGain listing page (227KB)
Searching for exact company code match
No exact company code match found
Failed to find InvestorGain numeric ID, cannot build URL
Failed to scrape IPO price history, continuing with next IPO

// AFTER:
[NO_IG_ID] IPO "Cult Fit" (cult-fit) - not found on InvestorGain listing. Stock ID: GROWW-cult-fit-ipo. Skipping.
```

### 3. Structured Fields

Add structured fields for filtering and dashboards:

```go
logger.WithFields(logrus.Fields{
    "ipo_id":        ipo.ID,
    "company_code":   ipo.CompanyCode,
    "stock_id":      ipo.StockID,
    "failure_type":  "NO_IG_ID",    // for grep/dashboards
    "is_upcoming":   true,          // helpful context
}).Warn("GMP data not available - IPO not yet on InvestorGain")
```

### 4. Batch Summary

Add end-of-job summary:

```
GMP History Backfill Complete:
  Total: 61 IPOs
  Success: 33 (saved X records)
  Skipped (No IG ID): 28
  Errors: 0
```

### 5. Implementation Locations

1. **gmp_history_scraper.go** - Update scraper to return typed errors
2. **gmp_history_service.go** - Update main loop to log single summary per IPO

## Expected Outcome

- Logs are self-documenting with failure type codes
- Easy to filter: `grep "NO_IG_ID" logs.txt`
- Clear distinction between "expected failures" (upcoming IPOs) vs "real errors"
- Faster debugging when actual issues occur
