# GMP History Charting and Scraper Reliability Spec

## 1) Overview and user value

### Feature summary
Add reliable historical GMP scraping and chart-ready API output so retail investors can view GMP price movement trends on the frontend.

### Problem statement
Users currently need trustworthy historical GMP points to understand trend direction. If scraping is unstable or data is inconsistent, chart trust drops and frontend experience degrades.

### Confirmed source
- Primary source for V1: InvestorGain GMP pages
- URL pattern in scope: `https://www.investorgain.com/gmp/{company-code}-ipo/{numeric-id}/`
- Example validated target: `https://www.investorgain.com/gmp/gaudium-ivf-ipo/1621/`
- Numeric ID discovery source: `https://www.investorgain.com/report/live-ipo-gmp/331/all/`

### Target users
- Primary: retail investors consuming IPO GMP movement charts

### Release priority
- High (current sprint)

---

## 2) Functional requirements (EARS format)

### FR-01 Source URL construction
When the system prepares to scrape historical GMP for an IPO, the system shall build the InvestorGain URL using company code and numeric ID in the format `/gmp/{company-code}-ipo/{numeric-id}/`.

### FR-02 Numeric ID discovery
When numeric ID is not already known for an IPO, the system shall discover it from the live GMP listing page by matching company code and IPO name.

### FR-03 Historical table extraction
When the scraper opens a valid IPO GMP history page, the system shall extract all available historical rows for that IPO from the history table.

### FR-04 Field extraction for charting
For each valid historical row, the system shall extract at minimum `record_date`, `gmp_value`, and `ipo_price`.

### FR-05 Data integrity over smoothing
Where source history has missing dates, the system shall preserve gaps and shall not fabricate or forward-fill synthetic points.

### FR-06 Persistence and deduplication
When extracted rows are valid, the system shall upsert rows into `gmp_price_history` keyed to IPO identity and date to prevent duplicates.

### FR-07 Scheduled refresh
When background jobs run on schedule, the system shall refresh GMP history for eligible IPOs and keep chart data freshness within 24 hours in normal operation.

### FR-08 One-time backfill
When the backfill command/job is triggered for rollout, the system shall perform one-time full historical backfill for all active IPOs.

### FR-09 Chart endpoint contract
When a client calls `GET /api/v1/gmp/history/:ipo_id/chart`, the system shall return chart-ready points including `date`, `gmp_value`, and `ipo_price`.

### FR-10 Identifier support
When a client calls a GMP history endpoint, the system shall accept either IPO UUID or stock ID and resolve to canonical IPO UUID.

### FR-11 Metadata for trust
When chart data is returned, the system shall include metadata with source, last updated timestamp, record count, and scrape status.

### FR-12 Fail-soft behavior
If scraping fails during refresh, then the system shall return the latest persisted/cached history where available and surface clear status metadata instead of crashing the endpoint.

### FR-13 Admin operational control
When an authorized admin triggers history update or backfill, the system shall execute the run and expose status/metrics endpoints for monitoring.

### FR-14 HTML change resilience
If source HTML structure changes and selectors partially fail, then the system shall isolate row-level parse errors, continue processing valid rows, and log structured diagnostics.

---

## 3) Non-functional requirements

### Performance
- API p95 for chart endpoint: <= 1 second under normal cached reads.
- Scrape run should be bounded by timeout per IPO scrape attempt.
- Backfill should process IPOs in controlled batches to avoid DB/resource spikes.

### Reliability and resiliency
- Minimum scraper success target: 99% on scheduled runs (measured per job window).
- Mandatory controls: retry with exponential backoff, circuit breaker, rate limiter, dead-letter/retry queue for failed persistence items.
- Service must degrade gracefully when source is unavailable.

### Security
- Admin trigger endpoints must remain token-protected.
- Validate all path/query inputs (UUID/stock ID/date range).
- Sanitize scraped HTML text before parsing/persistence.
- Avoid logging sensitive tokens or credentials.

### Data quality
- Timezone for record date and chart labels: Asia/Kolkata.
- No synthetic data point insertion for missing days.
- Enforce numeric parsing validation and reject malformed values per row.

### Observability
- Emit metrics for scrape success/failure, rows extracted, parse errors, endpoint latency, cache hit rate.
- Alert when success ratio falls below threshold or row count drops abnormally.
- Detect source HTML signature/selector drift and alert.

---

## 4) API plan (endpoint design)

### Keep existing endpoints, enhance payload
- `GET /api/v1/gmp/history/:ipo_id`
- `GET /api/v1/gmp/history/:ipo_id/chart` (primary frontend endpoint)
- `GET /api/v1/gmp/history/:ipo_id/summary`
- `GET /api/v1/gmp/history/health`
- `GET /api/v1/gmp/history/metrics`

### Admin/ops endpoints
- `POST /api/v1/admin/gmp-history/update` (scheduled/manual refresh)
- `GET /api/v1/admin/gmp-history/status`
- `GET /api/v1/admin/gmp-history/metrics`
- New planned: `POST /api/v1/admin/gmp-history/backfill` (one-time full historical run)

### Chart response minimum schema (V1)
```json
{
  "success": true,
  "data": {
    "ipo_info": {
      "ipo_id": "uuid",
      "ipo_name": "string",
      "company_code": "string",
      "timezone": "Asia/Kolkata"
    },
    "chart_data": [
      {
        "date": "YYYY-MM-DD",
        "gmp_value": 25.0,
        "ipo_price": 100.0
      }
    ],
    "metadata": {
      "total_records": 12,
      "data_source": "investorgain.com",
      "last_updated": "RFC3339",
      "scraping_success": true
    }
  }
}
```

---

## 5) Acceptance criteria (Given/When/Then)

### AC-01 Correct source scraping
Given an IPO with known InvestorGain numeric ID,
When the scraper builds and fetches the history URL,
Then it must pull historical rows from the matching InvestorGain page and persist valid points.

### AC-02 Numeric ID discovery
Given an IPO without stored numeric ID,
When refresh is executed,
Then the system must discover numeric ID from live GMP listing and continue history scraping.

### AC-03 Chart endpoint contract
Given an IPO with stored history,
When `GET /api/v1/gmp/history/:ipo_id/chart` is called,
Then response must include `date`, `gmp_value`, `ipo_price`, source metadata, and record count.

### AC-04 Identifier compatibility
Given either valid IPO UUID or stock ID,
When chart/history endpoints are called,
Then both identifier types resolve and return the same underlying IPO data.

### AC-05 Gap handling
Given source history with missing dates,
When chart data is returned,
Then missing dates remain absent/null in series and no synthetic points are inserted.

### AC-06 Daily freshness
Given scheduler is healthy,
When users request chart data,
Then last successful refresh is within 24 hours under normal operation.

### AC-07 Reliability controls
Given transient network/source failures,
When scrape jobs run,
Then retry, circuit breaker, and rate limiting are applied, and failure details are logged.

### AC-08 Performance target
Given production-like load and warm cache,
When chart endpoint is invoked,
Then p95 response latency is <= 1 second.

### AC-09 Backfill coverage
Given one-time backfill is triggered,
When backfill completes,
Then all active IPOs have attempted historical fetch and completion status per IPO is auditable.

### AC-10 Graceful degradation
Given current scrape fails but prior history exists,
When frontend requests chart data,
Then API still returns latest persisted data with metadata indicating stale/scrape-failed status.

---

## 6) Error handling table

| Scenario | Detection | API/Job behavior | HTTP/Status | Retry policy | Logged fields |
|---|---|---|---|---|---|
| Invalid IPO identifier | UUID/stock-id validation fails | Reject request | 400 | No retry | endpoint, identifier, validation_error |
| IPO not found | DB lookup returns none | Return not found | 404 | No retry | endpoint, identifier, lookup_result |
| InvestorGain unreachable | network timeout/connection error | Use retry; if exhausted mark scrape failed | 503 for live scrape ops; cached read allowed | Exponential backoff | ipo_id, url, attempt, timeout |
| Circuit breaker open | breaker state open | Short-circuit scrape call | 503 (service unavailable) | No immediate retry | circuit_name, state, ipo_id |
| Rate limit hit (internal control) | limiter threshold reached | Delay/schedule next attempt | Job warning | Retry at next slot | ipo_id, limiter_state |
| HTML structure changed | selector miss / parse error spike | Continue partial row parsing; flag degraded | 200 with degraded metadata or job failure threshold | Retry next run | selector, parse_errors, sample_html_hash |
| DB write failure | upsert error | Queue for retry / dead-letter if exhausted | Job failure signal | Queue retry policy | ipo_id, row_date, db_error |
| No history rows found | zero valid rows extracted | Return empty chart with metadata | 200 (empty data) | No retry in request path | ipo_id, url, extracted_rows |

---

## 7) Implementation TODO checklist

### A) Scraper reliability
- [ ] Verify and harden InvestorGain selectors for history table and numeric ID lookup.
- [ ] Add/confirm HTML signature checks and selector drift detection.
- [ ] Ensure retry, rate limiter, and circuit breaker policies are configured for production.
- [ ] Add dead-letter path for rows/collections failing persistence after max retries.

### B) Data model and persistence
- [ ] Confirm upsert uniqueness strategy for `(ipo_id, record_date)` in `gmp_price_history`.
- [ ] Enforce required parsed fields for chart minimum (`record_date`, `gmp_value`, `ipo_price`).
- [ ] Normalize record date handling to Asia/Kolkata for chart contract consistency.

### C) API contract and endpoints
- [ ] Enhance existing `/api/v1/gmp/history/:ipo_id/chart` response to guarantee required point fields.
- [ ] Add explicit metadata flags for `scraping_success`, `is_stale`, and `last_successful_scrape`.
- [ ] Add `POST /api/v1/admin/gmp-history/backfill` endpoint for one-time full backfill.

### D) Jobs and operations
- [ ] Configure daily history refresh schedule and runbook.
- [ ] Implement one-time full backfill job for all active IPOs with resumable progress.
- [ ] Add admin status metrics for backfill progress, failures, and retries.

### E) Testing
- [ ] Add contract tests for chart endpoint schema (UUID and stock ID).
- [ ] Add integration tests: scrape -> save -> read chart endpoint.
- [ ] Add resiliency tests for network failure, circuit open, and HTML drift.
- [ ] Add performance test to validate chart endpoint p95 <= 1s.

### F) Observability and alerts
- [ ] Publish metrics for success ratio, parse errors, latency, and cache hit rate.
- [ ] Add alerts for scrape success < 99% and sudden row-count anomalies.
- [ ] Add dashboard panels for history job health and endpoint latency.

---

## 8) Out of scope (V1)
- Multi-source aggregation beyond InvestorGain.
- Synthetic interpolation/forward-fill for missing historical dates.
- Near real-time (<1h) refresh cadence.
