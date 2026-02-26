# IPO Backend API V2 Documentation

## Overview

This document describes the V2 API endpoints for the IPO Backend. The V2 API provides a modern, consistent interface with standardized response envelopes, pagination metadata, and nested GMP data.

**Base URL:** `https://api.example.com` (or `http://localhost:8080` locally)

**Authentication:**
*   Public endpoints: None required.
*   Admin endpoints: Require `X-Admin-Token` header.

---

## Response Envelope

All V2 responses follow a consistent envelope format:

### Success Response
```json
{
  "success": true,
  "data": { ... }
}
```

### Paginated Response
```json
{
  "success": true,
  "data": [ ... ],
  "meta": {
    "total": 100,
    "limit": 20,
    "offset": 0,
    "has_next": true
  }
}
```

### Error Response
```json
{
  "success": false,
  "error": {
    "code": "NOT_FOUND",
    "message": "IPO not found",
    "details": { ... }
  }
}
```

### Error Codes
| Code | HTTP Status | Description |
|------|-------------|-------------|
| `VALIDATION_ERROR` | 400 | Invalid request parameters |
| `NOT_FOUND` | 404 | Resource not found |
| `INTERNAL_ERROR` | 500 | Server error |
| `BAD_GATEWAY` | 502 | External service error |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily unavailable |
| `RATE_LIMITED` | 429 | Too many requests |

**Note:** Some Admin endpoints currently return a legacy format:
```json
{
  "success": true,
  "data": ...
}
```
(without strict envelope structure for data/meta/error code)

---

## Public Endpoints

### 1. Get IPO Feed

**Endpoint:** `GET /api/v2/ipos/feed`

**Description:** Returns a paginated list of IPOs with GMP data.

**Query Parameters:**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `status` | string | No | `all` | Filter by status: `all`, `live`, `upcoming`, `closed`, `listed` |
| `limit` | int | No | 50 | Number of items per page (max 200) |
| `offset` | int | No | 0 | Number of items to skip |

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "stock_id": "wakefit-innovations",
      "name": "Wakefit Innovations Ltd",
      "logo_url": "https://example.com/logo.png",
      "status": "LIVE",
      "category": "mainboard",
      "price_band_low": 100.0,
      "price_band_high": 120.0,
      "open_date": "2024-12-08T00:00:00Z",
      "close_date": "2024-12-10T00:00:00Z",
      "listing_date": "2024-12-20T00:00:00Z",
      "gmp": {
        "value": 25.0,
        "gain_percent": 22.73,
        "estimated_listing": 135.0,
        "subscription_status": "Oversubscribed 2.5x"
      }
    }
  ],
  "meta": {
    "total": 15,
    "limit": 50,
    "offset": 0,
    "has_next": false
  }
}
```

**cURL:**
```bash
curl -X GET "https://api.example.com/api/v2/ipos/feed?status=live&limit=20" \
  -H "Content-Type: application/json"
```

---

### 2. Get IPO Detail

**Endpoint:** `GET /api/v2/ipos/:id`

**Description:** Returns detailed information about a specific IPO, including GMP data and extensive details from external sources (Groww).

**Path Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | UUID | Yes | IPO unique identifier |

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "stock_id": "wakefit-innovations",
    "name": "Wakefit Innovations Ltd",
    "logo_url": "https://example.com/logo.png",
    "status": "LIVE",
    "category": "mainboard",
    "price_band_low": 100.0,
    "price_band_high": 120.0,
    "min_qty": 100,
    "min_amount": 12000,
    "min_investment": 12000.0,
    "issue_size": "₹500 Cr",
    "registrar": "Link Intime",
    "open_date": "2024-12-08T00:00:00Z",
    "close_date": "2024-12-10T00:00:00Z",
    "allotment_date": "2024-12-15T00:00:00Z",
    "listing_date": "2024-12-20T00:00:00Z",
    "description": "Company description",
    "subscription_status": "Oversubscribed 2.5x",
    "strengths": ["Strong brand", "Experienced management"],
    "risks": ["High competition"],
    "financials": [
        {
            "title": "Revenue",
            "yearly": {"2023": 100.0, "2024": 120.0}
        }
    ],
    "categories": [],
    "faqs": [],
    "objectives": [],
    "lead_manager": "Axis Capital",
    "registrar_phone": "+91-22-12345678",
    "registrar_email": "support@linkintime.co.in",
    "company_address": "Bangalore, India",
    "company_phone": "+91-80-12345678",
    "company_email": "investor@wakefit.co",
    "cms_details": {
        "content": "<div>HTML content...</div>",
        "objectives": [...]
    },
    "groww_details": {
        "minPrice": 100.0,
        "maxPrice": 120.0,
        "issueSize": 5000000000,
        "lotSize": 100
    },
    "gmp": {
      "value": 25.0,
      "gain_percent": 22.73,
      "estimated_listing": 135.0,
      "subscription_status": "Oversubscribed 2.5x"
    }
  }
}
```

**cURL:**
```bash
curl -X GET "https://api.example.com/api/v2/ipos/550e8400-e29b-41d4-a716-446655440000" \
  -H "Content-Type: application/json"
```

---

### 3. Check Allotment Status

**Endpoint:** `POST /api/v2/allotment/check`

**Description:** Check the allotment status of an IPO application using PAN number.

**Request Body:**
```json
{
  "ipo_id": "550e8400-e29b-41d4-a716-446655440000",
  "pan": "ABCDE1234F"
}
```

**Request Parameters:**
| Parameter | Type | Required | Validation |
|-----------|------|----------|------------|
| `ipo_id` | string | Yes | Valid UUID |
| `pan` | string | Yes | Format: `^[A-Z]{5}[0-9]{4}[A-Z]$` |

**Response:**
```json
{
  "success": true,
  "data": {
    "status": "ALLOTTED",
    "shares_applied": 100,
    "shares_allotted": 100,
    "message": "Congratulations! Shares have been allotted."
  }
}
```

**Status Values:** `ALLOTTED`, `NOT_ALLOTTED`, `PENDING`

**cURL:**
```bash
curl -X POST "https://api.example.com/api/v2/allotment/check" \
  -H "Content-Type: application/json" \
  -d '{"ipo_id": "550e8400-e29b-41d4-a716-446655440000", "pan": "ABCDE1234F"}'
```

---

### 4. Get GMP Chart Data

**Endpoint:** `GET /api/v2/gmp/history/:ipo_id/chart`

**Description:** Returns GMP price history chart data for an IPO.

**Path Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `ipo_id` | string | Yes | IPO identifier (UUID or stock_id) |

**Response:**
```json
{
  "success": true,
  "data": {
    "labels": ["2024-12-01", "2024-12-02", "2024-12-03"],
    "datasets": [
      {
        "label": "GMP",
        "data": [20, 22, 25]
      }
    ]
  }
}
```

**cURL:**
```bash
curl -X GET "https://api.example.com/api/v2/gmp/history/550e8400-e29b-41d4-a716-446655440000/chart" \
  -H "Content-Type: application/json"
```

---

## Admin Endpoints

**Authentication:** Requires `X-Admin-Token` header.

### 5. Create IPO

**Endpoint:** `POST /api/v2/admin/ipos`

**Format:** Legacy Response

**Request Body:**
```json
{
  "stock_id": "example-ipo",
  "name": "Example IPO",
  "open_date": "2024-01-01T00:00:00Z",
  "close_date": "2024-01-03T00:00:00Z",
  ...
}
```

**Response:**
```json
{
  "success": true,
  "data": { ... }
}
```

### 6. Trigger GMP Update

**Endpoint:** `POST /api/v2/admin/gmp/update`

**Description:** Manually trigger a GMP data update job.

**Response:**
```json
{
  "success": true,
  "data": {
    "message": "GMP update job completed",
    "duration": "1.234s"
  }
}
```

### 7. Get GMP Data

**Endpoint:** `GET /api/v2/admin/gmp/data`

**Format:** Legacy Response

**Description:** Get recent GMP data from the database.

**Response:**
```json
{
  "success": true,
  "data": [ ... ],
  "count": 20
}
```

### 8. Trigger GMP History Update

**Endpoint:** `POST /api/v2/admin/gmp-history/update`

**Description:** Manually trigger GMP history backfill job.

**Response:**
```json
{
  "success": true,
  "data": {
    "message": "GMP history update job completed",
    "duration": "5m 30s"
  }
}
```

### 9. Get GMP History Job Status

**Endpoint:** `GET /api/v2/admin/gmp-history/status`

**Description:** Get current status of the GMP history job.

**Response:**
```json
{
  "success": true,
  "data": {
    "is_running": false,
    "last_run": "2024-02-01T10:00:00Z",
    ...
  }
}
```

### 10. Get GMP History Job Metrics

**Endpoint:** `GET /api/v2/admin/gmp-history/metrics`

**Description:** Get detailed metrics from the last GMP history job run.

**Response:**
```json
{
  "success": true,
  "data": {
    "job_start_time": "2024-02-01T10:00:00Z",
    "duration": "5m",
    "total_ipos": 50,
    "successful_ipos": 48,
    "failed_ipos": 2,
    "error_summary": { ... }
  }
}
```

---

## Internal / Utility Endpoints

These endpoints are primarily for testing and performance monitoring. They currently reside under `/api/v1` but are available for system maintenance.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/performance/metrics` | GET | Get cache and system performance metrics |
| `/api/v1/performance/test` | POST | Run a performance test (heavy load simulation) |
| `/api/v1/performance/cache` | DELETE | Clear the application cache |
| `/api/v1/performance/cache/warmup` | POST | Manually trigger cache warmup |

---

## Rate Limiting

Public endpoints are rate limited to ensure fair usage:

| Endpoint | Limit |
|----------|-------|
| `/ipos/feed` | 60 requests/minute |
| `/ipos/:id` | 120 requests/minute |
| `/allotment/check` | 10 requests/minute |

When rate limited, the API returns:
```json
{
  "success": false,
  "error": {
    "code": "RATE_LIMITED",
    "message": "Too many requests"
  }
}
```

---

## Data Models Reference

### V2IPOFeedItem
| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Unique identifier |
| `stock_id` | string | Stock exchange ID / Slug |
| `name` | string | Company name |
| `logo_url` | string | Company logo URL |
| `status` | string | IPO status (LIVE, UPCOMING, CLOSED, LISTED) |
| `category` | string | IPO category (mainboard, sme) |
| `price_band_low` | float | Lower price bound |
| `price_band_high` | float | Upper price bound |
| `open_date` | string | IPO opening date (ISO8601) |
| `close_date` | string | IPO closing date (ISO8601) |
| `listing_date` | string | Expected listing date (ISO8601) |
| `gmp` | object | Nested GMP data |

### V2GMPNested
| Field | Type | Description |
|-------|------|-------------|
| `value` | float | GMP value |
| `gain_percent` | float | Expected gain percentage |
| `estimated_listing` | float | Estimated listing price |
| `subscription_status` | string | Subscription status text |

### GrowwIPODetailsResponse (Nested in Detail)
Key fields include:
*   `minPrice`, `maxPrice`, `issueSize`, `lotSize`
*   `startDate`, `endDate`, `listingDate`
*   `subscriptionRates`: Array of category subscription rates
*   `financials`: Revenue, Profit, Assets data
*   `aboutCompany`: Company profile text
*   `pros`, `cons`: Arrays of strings
