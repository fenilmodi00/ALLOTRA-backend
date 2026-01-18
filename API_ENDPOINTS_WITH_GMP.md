# API Endpoints with GMP Integration

## Overview

This document describes the new API endpoints that provide IPO data with integrated Grey Market Premium (GMP) information. These endpoints join data from the `ipos` and `ipo_gmp` tables using the `company_code` field.

**Last Updated:** December 7, 2024

---

## New Endpoints

### 1. Get Active IPOs with GMP Data

**Endpoint:** `GET /api/v1/ipos/active-with-gmp`

**Description:** Returns all active IPOs (status = 'LIVE' or 'RESULT_OUT') with GMP data joined by company_code. If no GMP data exists for an IPO, the GMP fields will be null.

**Request:**
```bash
curl http://localhost:8080/api/v1/ipos/active-with-gmp
```

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Wakefit Innovations Ltd",
      "company_code": "wakefit-innovations",
      "description": "Leading sleep solutions company",
      "price_band_low": 100.0,
      "price_band_high": 120.0,
      "issue_size": "₹500 Cr",
      "open_date": "2024-12-08T00:00:00Z",
      "close_date": "2024-12-10T00:00:00Z",
      "result_date": "2024-12-15T00:00:00Z",
      "listing_date": "2024-12-20T00:00:00Z",
      "registrar": "Link Intime",
      "stock_id": "WAKEFIT",
      "symbol": "WAKEFIT",
      "slug": "wakefit-innovations",
      "listing_gain": null,
      "min_qty": 100,
      "min_amount": 12000,
      "logo_url": "https://example.com/logo.png",
      "about": "Company description",
      "strengths": ["Strong brand", "Growing market"],
      "risks": ["Competition", "Market volatility"],
      "status": "LIVE",
      "subscription_status": "Oversubscribed 2.5x",
      "form_url": "https://linkintime.co.in/ipo/wakefit",
      "form_fields": {},
      "form_headers": {},
      "parser_config": {},
      "created_at": "2024-12-01T00:00:00Z",
      "updated_at": "2024-12-06T00:00:00Z",
      "created_by": "admin",
      "gmp_value": 25.0,
      "gain_percent": 22.73,
      "estimated_listing": 135.0,
      "gmp_last_updated": "2024-12-06T10:00:00Z"
    },
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "name": "Another IPO Ltd",
      "company_code": "another-ipo",
      "description": "Another company",
      "price_band_low": 200.0,
      "price_band_high": 250.0,
      "issue_size": "₹1000 Cr",
      "open_date": "2024-12-10T00:00:00Z",
      "close_date": "2024-12-12T00:00:00Z",
      "result_date": null,
      "listing_date": null,
      "registrar": "KFin Technologies",
      "stock_id": "ANOTHER",
      "symbol": "ANOTHER",
      "slug": "another-ipo",
      "listing_gain": null,
      "min_qty": 50,
      "min_amount": 12500,
      "logo_url": null,
      "about": null,
      "strengths": null,
      "risks": null,
      "status": "LIVE",
      "subscription_status": null,
      "form_url": "",
      "form_fields": {},
      "form_headers": {},
      "parser_config": {},
      "created_at": "2024-12-05T00:00:00Z",
      "updated_at": "2024-12-06T00:00:00Z",
      "created_by": "admin",
      "gmp_value": null,
      "gain_percent": null,
      "estimated_listing": null,
      "gmp_last_updated": null
    }
  ]
}
```

**GMP Fields:**
- `gmp_value` (float64, nullable): Grey market premium value
- `gain_percent` (float64, nullable): Expected gain percentage
- `estimated_listing` (float64, nullable): Estimated listing price
- `gmp_last_updated` (timestamp, nullable): Last time GMP data was updated

**Notes:**
- GMP fields will be `null` if no matching GMP data exists for the IPO
- GMP data is updated hourly by the GMP Update Job
- The join is performed using the `company_code` field

---

### 2. Get Single IPO with GMP Data

**Endpoint:** `GET /api/v1/ipos/:id/with-gmp`

**Description:** Returns a single IPO by ID with GMP data joined by company_code. If no GMP data exists for the IPO, the GMP fields will be null.

**Request:**
```bash
curl http://localhost:8080/api/v1/ipos/550e8400-e29b-41d4-a716-446655440000/with-gmp
```

**Response (with GMP data):**
```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Wakefit Innovations Ltd",
    "company_code": "wakefit-innovations",
    "description": "Leading sleep solutions company",
    "price_band_low": 100.0,
    "price_band_high": 120.0,
    "issue_size": "₹500 Cr",
    "open_date": "2024-12-08T00:00:00Z",
    "close_date": "2024-12-10T00:00:00Z",
    "result_date": "2024-12-15T00:00:00Z",
    "listing_date": "2024-12-20T00:00:00Z",
    "registrar": "Link Intime",
    "stock_id": "WAKEFIT",
    "symbol": "WAKEFIT",
    "slug": "wakefit-innovations",
    "listing_gain": null,
    "min_qty": 100,
    "min_amount": 12000,
    "logo_url": "https://example.com/logo.png",
    "about": "Company description",
    "strengths": ["Strong brand", "Growing market"],
    "risks": ["Competition", "Market volatility"],
    "status": "LIVE",
    "subscription_status": "Oversubscribed 2.5x",
    "form_url": "https://linkintime.co.in/ipo/wakefit",
    "form_fields": {},
    "form_headers": {},
    "parser_config": {},
    "created_at": "2024-12-01T00:00:00Z",
    "updated_at": "2024-12-06T00:00:00Z",
    "created_by": "admin",
    "gmp_value": 25.0,
    "gain_percent": 22.73,
    "estimated_listing": 135.0,
    "gmp_last_updated": "2024-12-06T10:00:00Z"
  }
}
```

**Response (without GMP data):**
```json
{
  "success": true,
  "data": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "name": "Another IPO Ltd",
    "company_code": "another-ipo",
    "description": "Another company",
    "price_band_low": 200.0,
    "price_band_high": 250.0,
    "issue_size": "₹1000 Cr",
    "open_date": "2024-12-10T00:00:00Z",
    "close_date": "2024-12-12T00:00:00Z",
    "result_date": null,
    "listing_date": null,
    "registrar": "KFin Technologies",
    "stock_id": "ANOTHER",
    "symbol": "ANOTHER",
    "slug": "another-ipo",
    "listing_gain": null,
    "min_qty": 50,
    "min_amount": 12500,
    "logo_url": null,
    "about": null,
    "strengths": null,
    "risks": null,
    "status": "LIVE",
    "subscription_status": null,
    "form_url": "",
    "form_fields": {},
    "form_headers": {},
    "parser_config": {},
    "created_at": "2024-12-05T00:00:00Z",
    "updated_at": "2024-12-06T00:00:00Z",
    "created_by": "admin",
    "gmp_value": null,
    "gain_percent": null,
    "estimated_listing": null,
    "gmp_last_updated": null
  }
}
```

**Response (IPO not found):**
```json
{
  "success": false,
  "error": "IPO not found"
}
```

**Status Codes:**
- `200 OK`: IPO found (with or without GMP data)
- `404 Not Found`: IPO does not exist
- `500 Internal Server Error`: Database or server error

---

## Backward Compatibility

The existing endpoints remain unchanged and continue to work:

- `GET /api/v1/ipos/active` - Returns active IPOs without GMP data
- `GET /api/v1/ipos/:id` - Returns single IPO without GMP data
- `GET /api/v1/ipos/:id/gmp` - Returns only GMP data for an IPO

**Migration Strategy:**

Frontend applications can gradually migrate to the new endpoints:

1. **Phase 1:** Use new endpoints alongside existing ones
2. **Phase 2:** Update UI to display GMP data
3. **Phase 3:** Deprecate separate GMP endpoint calls

---

## Database Schema

### Tables Involved

**ipos table:**
- Primary key: `id` (UUID)
- Unique key: `company_code` (normalized slug)
- Contains static IPO details

**ipo_gmp table:**
- Primary key: `id` or `ipo_name`
- Foreign key concept: `company_code` (matches ipos.company_code)
- Contains dynamic GMP data

### Join Query

```sql
SELECT 
    i.*,
    g.gmp_value,
    g.gain_percent,
    g.estimated_listing,
    g.last_updated as gmp_last_updated
FROM ipo_list i
LEFT JOIN ipo_gmp g ON i.company_code = g.company_code
WHERE i.status = 'LIVE' OR i.status = 'RESULT_OUT';
```

---

## Performance Considerations

### Indexes

Ensure the following indexes exist for optimal performance:

```sql
-- Index on ipos.company_code for fast lookups
CREATE INDEX IF NOT EXISTS idx_ipos_company_code ON ipo_list(company_code);

-- Index on ipo_gmp.company_code for fast joins
CREATE INDEX IF NOT EXISTS idx_ipo_gmp_company_code ON ipo_gmp(company_code);

-- Index on ipos.status for filtering
CREATE INDEX IF NOT EXISTS idx_ipos_status ON ipo_list(status);
```

### Caching

Consider implementing caching for frequently accessed data:

1. **Cache active IPOs with GMP** for 5-10 minutes
2. **Invalidate cache** when GMP job runs (every hour)
3. **Use Redis** or in-memory cache for best performance

---

## Testing

### Manual Testing

```bash
# Test active IPOs with GMP
curl http://localhost:8080/api/v1/ipos/active-with-gmp | jq

# Test single IPO with GMP
curl http://localhost:8080/api/v1/ipos/550e8400-e29b-41d4-a716-446655440000/with-gmp | jq

# Test IPO without GMP data (should return null GMP fields)
curl http://localhost:8080/api/v1/ipos/660e8400-e29b-41d4-a716-446655440001/with-gmp | jq

# Test non-existent IPO (should return 404)
curl http://localhost:8080/api/v1/ipos/00000000-0000-0000-0000-000000000000/with-gmp | jq
```

### Integration Tests

See `tests/api_integration_test.go` for automated tests covering:
- Active IPOs with GMP data
- Single IPO with GMP data
- IPOs without GMP data (null fields)
- Non-existent IPOs (404 response)

---

## Frontend Integration

### TypeScript Types

```typescript
interface IPOWithGMP {
  // All IPO fields
  id: string
  name: string
  company_code: string
  description: string
  price_band_low: number
  price_band_high: number
  issue_size: string
  open_date: string
  close_date: string
  result_date: string | null
  listing_date: string | null
  registrar: string
  stock_id: string
  symbol: string
  slug: string
  listing_gain: number | null
  min_qty: number
  min_amount: number
  logo_url: string | null
  about: string | null
  strengths: any | null
  risks: any | null
  status: string
  subscription_status: string | null
  form_url: string
  form_fields: any
  form_headers: any
  parser_config: any
  created_at: string
  updated_at: string
  created_by: string
  
  // GMP fields (nullable)
  gmp_value: number | null
  gain_percent: number | null
  estimated_listing: number | null
  gmp_last_updated: string | null
}
```

### API Service

```typescript
export const ipoService = {
  // Get active IPOs with GMP data
  getActiveIPOsWithGMP: async () => {
    const response = await api.get('/ipos/active-with-gmp')
    return response.data
  },

  // Get single IPO with GMP data
  getIPOWithGMP: async (id: string) => {
    const response = await api.get(`/ipos/${id}/with-gmp`)
    return response.data
  },
}
```

---

## Requirements Validation

This implementation satisfies **Requirement 11.3** from the requirements document:

> **11.3** WHEN both tables contain data for the same IPO THEN the system SHALL use company_code as the linking key

The API routes successfully:
- ✅ Join `ipos` and `ipo_gmp` tables using `company_code`
- ✅ Return combined data to the frontend
- ✅ Handle cases where GMP data is unavailable (null fields)
- ✅ Maintain backward compatibility with existing endpoints
- ✅ Provide clear API documentation

---

**Document Version:** 1.0  
**Last Updated:** December 7, 2024
