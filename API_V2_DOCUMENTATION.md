# IPO Backend API V2 Documentation

## Overview

This document describes the V2 API endpoints for the IPO Backend. The V2 API provides a modern, consistent interface with standardized response envelopes, pagination metadata, and nested GMP data.

**Base URL:** `https://api.example.com/api/v2`

**Last Updated:** February 2026

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

---

## IPO Endpoints

### 1. Get IPO Feed

**Endpoint:** `GET /api/v2/ipos/feed`

**Description:** Returns a paginated list of IPOs with GMP data. Replaces the V1 endpoints `/ipos`, `/ipos/active`, and `/ipos/active-with-gmp`.

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

**Description:** Returns detailed information about a specific IPO, including GMP data and Groww-sourced fields. Replaces V1 endpoints `/ipos/:id`, `/ipos/:id/with-gmp`, and `/ipos/:id/gmp`.

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
    "min_investment": 12000.0,
    "issue_size": "₹500 Cr",
    "registrar": "Link Intime",
    "open_date": "2024-12-08T00:00:00Z",
    "close_date": "2024-12-10T00:00:00Z",
    "allotment_date": "2024-12-15T00:00:00Z",
    "listing_date": "2024-12-20T00:00:00Z",
    "description": "Company description",
    "subscription_status": "Oversubscribed 2.5x",
    "financials": [...],
    "categories": [...],
    "faqs": [...],
    "gmp": {
      "value": 25.0,
      "gain_percent": 22.73,
      "estimated_listing": 135.0,
      "subscription_status": "Oversubscribed 2.5x"
    }
  }
}
```

**Error Responses:**
- `400 VALIDATION_ERROR`: Invalid UUID format
- `404 NOT_FOUND`: IPO not found

**cURL:**
```bash
curl -X GET "https://api.example.com/api/v2/ipos/550e8400-e29b-41d4-a716-446655440000" \
  -H "Content-Type: application/json"
```

---

## Allotment Check Endpoints

### 3. Check Allotment Status

**Endpoint:** `POST /api/v2/allotment/check`

**Description:** Check the allotment status of an IPO application using PAN number.

**Request:**
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

**Status Values:**
| Status | Description |
|--------|-------------|
| `ALLOTTED` | Shares have been allotted |
| `NOT_ALLOTTED` | No shares were allotted |
| `PENDING` | Allotment result pending |

**Error Responses:**
- `400 VALIDATION_ERROR`: Invalid request (missing/invalid fields)
- `404 NOT_FOUND`: IPO not found
- `422 VALIDATION_ERROR`: Invalid PAN format
- `502 BAD_GATEWAY`: External service error

**cURL:**
```bash
curl -X POST "https://api.example.com/api/v2/allotment/check" \
  -H "Content-Type: application/json" \
  -d '{"ipo_id": "550e8400-e29b-41d4-a716-446655440000", "pan": "ABCDE1234F"}'
```

---

## GMP History Endpoints

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

All admin endpoints require authentication via `X-Admin-Token` header.

### 5. Create IPO (Admin)

**Endpoint:** `POST /api/v2/admin/ipos`

**Headers:**
| Header | Required | Description |
|--------|----------|-------------|
| `X-Admin-Token` | Yes | Admin authentication token |

### 6. Trigger GMP Update (Admin)

**Endpoint:** `POST /api/v2/admin/gmp/update`

**Description:** Manually trigger a GMP data update.

### 7. Get GMP Data (Admin)

**Endpoint:** `GET /api/v2/admin/gmp/data`

**Description:** Get current GMP data from the database.

### 8. Trigger GMP History Update (Admin)

**Endpoint:** `POST /api/v2/admin/gmp-history/update`

**Description:** Manually trigger GMP history backfill.

### 9. Get GMP History Job Status (Admin)

**Endpoint:** `GET /api/v2/admin/gmp-history/status`

### 10. Get GMP History Job Metrics (Admin)

**Endpoint:** `GET /api/v2/admin/gmp-history/metrics`

---

## Response Fields Reference

### IPO Feed Item Fields
| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Unique identifier |
| `stock_id` | string | Stock exchange ID |
| `name` | string | Company name |
| `logo_url` | string | Company logo URL |
| `status` | string | IPO status (LIVE, UPCOMING, CLOSED, LISTED) |
| `category` | string | IPO category (mainboard, sme) |
| `price_band_low` | float | Lower price bound |
| `price_band_high` | float | Upper price bound |
| `open_date` | ISO8601 | IPO opening date |
| `close_date` | ISO8601 | IPO closing date |
| `listing_date` | ISO8601 | Expected listing date |
| `gmp` | object | Grey Market Premium data (see below) |

### GMP Nested Fields
| Field | Type | Description |
|-------|------|-------------|
| `value` | float | GMP value |
| `gain_percent` | float | Expected gain percentage |
| `estimated_listing` | float | Estimated listing price |
| `subscription_status` | string | Subscription status |

### IPO Detail Additional Fields
| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Company description |
| `registrar` | string | IPO registrar |
| `issue_size` | string | Issue size |
| `min_qty` | int | Minimum lot size |
| `min_investment` | float | Minimum investment amount |
| `allotment_date` | ISO8601 | Allotment result date |
| `subscription_status` | string | Subscription status |
| `financials` | JSON | Financial data from Groww |
| `categories` | JSON | IPO categories from Groww |
| `faqs` | JSON | FAQs from Groww |

---

## Pagination

All list endpoints support pagination using `limit` and `offset` query parameters:

- **Default limit:** 50
- **Maximum limit:** 200
- **Offset:** Number of items to skip

The response includes a `meta` object with:
- `total`: Total number of items available
- `limit`: Items per page
- `offset`: Current offset
- `has_next`: Whether more items are available

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

## Versioning

This is V2 of the API. V1 endpoints remain available at `/api/v1/` for backward compatibility but are considered deprecated.

**Deprecation Policy:**
- V1 endpoints will be supported for 6 months after V2 release
- Deprecation headers will be added to V1 responses
- Monitor usage metrics to track migration progress

---

## Testing

### Manual Testing

```bash
# Test feed endpoint
curl http://localhost:8080/api/v2/ipos/feed | jq

# Test feed with status filter
curl "http://localhost:8080/api/v2/ipos/feed?status=live" | jq

# Test IPO detail
curl http://localhost:8080/api/v2/ipos/550e8400-e29b-41d4-a716-446655440000 | jq

# Test allotment check
curl -X POST http://localhost:8080/api/v2/allotment/check \
  -H "Content-Type: application/json" \
  -d '{"ipo_id": "550e8400-e29b-41d4-a716-446655440000", "pan": "ABCDE1234F"}' | jq
```

---

## Migration Guide from V1 to V2

### Changes in V2:

1. **Response Envelope**: V2 uses standardized `{success, data, meta?}` format
2. **Nested GMP**: GMP data is now nested under `gmp` object instead of flat fields
3. **Pagination**: V2 includes `meta` with pagination info
4. **Category**: New `category` field (mainboard/sme)
5. **Groww Fields**: Detail endpoint includes `financials`, `categories`, `faqs`
6. **Allotment**: PAN validation uses regex `^[A-Z]{5}[0-9]{4}[A-Z]$`

### Example Migration:

**V1 Response:**
```json
{
  "success": true,
  "data": {
    "id": "...",
    "gmp_value": 25.0,
    "gain_percent": 22.73
  }
}
```

**V2 Response:**
```json
{
  "success": true,
  "data": {
    "id": "...",
    "gmp": {
      "value": 25.0,
      "gain_percent": 22.73
    }
  }
}
```

---

**Document Version:** 2.0  
**Last Updated:** February 2026
