# IPO Backend API Documentation

## Overview

The IPO Backend API provides comprehensive IPO (Initial Public Offering) data including static IPO details and dynamic Grey Market Premium (GMP) information. The API serves data to frontend applications with enhanced scraping capabilities from Chittorgarh.com and InvestorGain.com.

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Currently, the API does not require authentication for public endpoints. Admin endpoints will require authentication in future versions.

## Response Format

All API responses follow a consistent format:

```json
{
  "success": true|false,
  "data": <response_data>,
  "error": "<error_message>" // Only present when success is false
}
```

## Endpoints

### Health Check

#### GET /health

Returns the health status of the API server.

**Response:**
```json
{
  "status": "ok",
  "timestamp": 1703123456
}
```

### IPO Endpoints

#### GET /api/v1/ipos

Retrieve all IPOs with optional status filtering.

**Query Parameters:**
- `status` (optional): Filter by IPO status. Default: "all"
  - Values: "all", "upcoming", "live", "closed", "listed"
  - Status is calculated dynamically based on current time and IPO dates:
    - "upcoming": Before open_date
    - "live": Between open_date and close_date  
    - "closed": After close_date (before listing_date)
    - "listed": After listing_date

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "uuid",
      "stock_id": "COMPANY123",
      "name": "Company Name Ltd IPO",
      "company_code": "company-name-ltd",
      "symbol": "COMPANY",
      "registrar": "KFin Technologies",
      "open_date": "2024-01-15T00:00:00Z",
      "close_date": "2024-01-17T00:00:00Z",
      "result_date": "2024-01-20T00:00:00Z",
      "listing_date": "2024-01-22T00:00:00Z",
      "price_band_low": 100.00,
      "price_band_high": 110.00,
      "issue_size": "₹1000 Cr",
      "min_qty": 100,
      "min_amount": 11000,
      "status": "LIVE",
      "subscription_status": "2.5x subscribed",
      "listing_gain": "15.5%",
      "logo_url": "https://example.com/logo.png",
      "description": "Company description",
      "about": "Detailed company information",
      "slug": "company-name-ltd-ipo",
      "strengths": ["Strong market position", "Experienced management"],
      "risks": ["Market volatility", "Regulatory changes"],
      "form_url": "https://registrar.com/form",
      "form_fields": {},
      "form_headers": {},
      "parser_config": {},
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z",
      "created_by": "system"
    }
  ]
}
```

#### GET /api/v1/ipos/active

Retrieve only active (LIVE status) IPOs.

**Response:** Same format as GET /api/v1/ipos but filtered to active IPOs only.

#### GET /api/v1/ipos/active-with-gmp ⭐ NEW

Retrieve active IPOs with Grey Market Premium data joined by company_code.

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "uuid",
      "stock_id": "COMPANY123",
      "name": "Company Name Ltd IPO",
      "company_code": "company-name-ltd",
      "symbol": "COMPANY",
      "registrar": "KFin Technologies",
      "open_date": "2024-01-15T00:00:00Z",
      "close_date": "2024-01-17T00:00:00Z",
      "result_date": "2024-01-20T00:00:00Z",
      "listing_date": "2024-01-22T00:00:00Z",
      "price_band_low": 100.00,
      "price_band_high": 110.00,
      "issue_size": "₹1000 Cr",
      "min_qty": 100,
      "min_amount": 11000,
      "status": "LIVE",
      "subscription_status": "2.5x subscribed",
      "listing_gain": "15.5%",
      "logo_url": "https://example.com/logo.png",
      "description": "Company description",
      "about": "Detailed company information",
      "slug": "company-name-ltd-ipo",
      "strengths": ["Strong market position", "Experienced management"],
      "risks": ["Market volatility", "Regulatory changes"],
      "form_url": "https://registrar.com/form",
      "form_fields": {},
      "form_headers": {},
      "parser_config": {},
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z",
      "created_by": "system",
      "gmp_value": 25.00,
      "gain_percent": 22.73,
      "estimated_listing": 135.00,
      "gmp_last_updated": "2024-01-15T10:30:00Z"
    }
  ]
}
```

**Note:** GMP fields (`gmp_value`, `gain_percent`, `estimated_listing`, `gmp_last_updated`) will be `null` if no GMP data is available for the IPO.

#### GET /api/v1/ipos/:id

Retrieve a specific IPO by ID.

**Path Parameters:**
- `id`: UUID of the IPO

**Response:** Single IPO object with same structure as GET /api/v1/ipos

#### GET /api/v1/ipos/:id/with-gmp ⭐ NEW

Retrieve a specific IPO with GMP data joined by company_code.

**Path Parameters:**
- `id`: UUID of the IPO

**Response:** Single IPO object with GMP fields (same structure as active-with-gmp endpoint)

#### GET /api/v1/ipos/:ipo_id/form-config

Retrieve form configuration for IPO allotment checking.

**Path Parameters:**
- `ipo_id`: UUID of the IPO

**Response:** IPO object with form configuration details

#### GET /api/v1/ipos/:id/gmp

Retrieve GMP data for a specific IPO.

**Path Parameters:**
- `id`: UUID of the IPO

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "ipo_name": "Company Name Ltd IPO",
    "company_code": "company-name-ltd",
    "ipo_price": 110.00,
    "gmp_value": 25.00,
    "estimated_listing": 135.00,
    "gain_percent": 22.73,
    "sub2": 2.5,
    "kostak": 5.00,
    "listing_date": "2024-01-22T00:00:00Z",
    "last_updated": "2024-01-15T10:30:00Z"
  }
}
```

### Market Endpoints

#### GET /api/v1/market/indices

Retrieve current market indices information.

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "nifty50",
      "name": "NIFTY 50",
      "value": 21453.95,
      "change": 125.30,
      "change_percent": 0.59,
      "is_positive": true
    },
    {
      "id": "sensex",
      "name": "SENSEX",
      "value": 71315.09,
      "change": 418.75,
      "change_percent": 0.59,
      "is_positive": true
    },
    {
      "id": "banknifty",
      "name": "BANK NIFTY",
      "value": 45892.35,
      "change": -89.45,
      "change_percent": -0.19,
      "is_positive": false
    },
    {
      "id": "niftymidcap",
      "name": "NIFTY MIDCAP 100",
      "value": 48765.20,
      "change": 234.80,
      "change_percent": 0.48,
      "is_positive": true
    }
  ]
}
```

### Cache Endpoints

#### POST /api/v1/cache/store

Store IPO allotment result in cache.

**Request Body:**
```json
{
  "ipo_id": "uuid",
  "pan_hash": "hashed_pan",
  "status": "ALLOTTED",
  "shares_allotted": 100,
  "application_number": "APP123456",
  "refund_status": "NOT_APPLICABLE",
  "source": "manual",
  "user_agent": "Mozilla/5.0...",
  "confidence_score": 95,
  "duplicate_count": 0
}
```

**Response:**
```json
{
  "success": true,
  "message": "Result cached successfully"
}
```

#### GET /api/v1/cache/:ipo_id/:pan_hash

Retrieve cached allotment result.

**Path Parameters:**
- `ipo_id`: UUID of the IPO
- `pan_hash`: Hashed PAN number

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "pan_hash": "hashed_pan",
    "ipo_id": "uuid",
    "status": "ALLOTTED",
    "shares_allotted": 100,
    "application_number": "APP123456",
    "refund_status": "NOT_APPLICABLE",
    "source": "manual",
    "user_agent": "Mozilla/5.0...",
    "timestamp": "2024-01-15T10:30:00Z",
    "expires_at": "2024-01-22T10:30:00Z",
    "confidence_score": 95,
    "duplicate_count": 0
  }
}
```

### Check Endpoint

#### POST /api/v1/check

Check IPO allotment status.

**Request Body:**
```json
{
  "ipo_id": "uuid",
  "pan": "ABCDE1234F"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "pan_hash": "hashed_pan",
    "ipo_id": "uuid",
    "status": "ALLOTTED",
    "shares_allotted": 100,
    "application_number": "",
    "refund_status": "",
    "source": "live_check",
    "user_agent": "",
    "timestamp": "2024-01-15T10:30:00Z",
    "expires_at": "2024-01-22T10:30:00Z",
    "confidence_score": 0,
    "duplicate_count": 0
  }
}
```

### Admin Endpoints

#### POST /api/v1/admin/ipos

Create a new IPO (Admin only - Authentication required in future).

**Request Body:** IPO object with all required fields

**Response:**
```json
{
  "success": true,
  "data": {
    // IPO object with generated ID and timestamps
  }
}
```

### Performance Endpoints ⭐ NEW

#### GET /api/v1/performance/metrics

Get current performance metrics including query performance, cache statistics, and database connection pool stats.

**Response:**
```json
{
  "success": true,
  "data": {
    "get_active_ipos_with_gmp": {
      "duration_ms": 45,
      "count": 12,
      "cached": false
    },
    "get_active_ipos_with_gmp_cached": {
      "duration_ms": 2,
      "count": 12,
      "cached": true
    },
    "cache_stats": {
      "hit_rate": 0.85,
      "total_requests": 1000,
      "cache_hits": 850,
      "cache_misses": 150
    },
    "database_stats": {
      "open_connections": 5,
      "in_use": 2,
      "idle": 3,
      "wait_count": 0,
      "wait_duration_ms": 0,
      "max_idle_closed": 0,
      "max_idle_time_closed": 0,
      "max_lifetime_closed": 0
    },
    "index_stats": [
      {
        "schema": "public",
        "table": "ipo_list",
        "index": "idx_ipo_list_company_code",
        "scans": 1250,
        "tuples_read": 1250,
        "tuples_fetched": 1250
      }
    ]
  }
}
```

#### POST /api/v1/performance/test

Run a comprehensive performance test with load testing and cache performance comparison.

**Response:**
```json
{
  "success": true,
  "data": {
    "load_test": {
      "iterations": 10,
      "total_duration_ms": 450,
      "avg_duration_ms": 45,
      "queries_per_sec": 22.2
    },
    "cache_performance": {
      "uncached_duration_ms": 45,
      "cached_duration_ms": 2,
      "speedup_factor": 22.5
    },
    "query_plans": {
      "active_ipos_with_gmp": [
        "Nested Loop Left Join  (cost=0.28..123.45 rows=10 width=1234)",
        "  ->  Index Scan using idx_ipo_list_status on ipo_list i  (cost=0.14..45.67 rows=10 width=890)",
        "  ->  Index Scan using idx_ipo_gmp_company_code on ipo_gmp g  (cost=0.14..7.78 rows=1 width=344)"
      ]
    }
  }
}
```

#### DELETE /api/v1/performance/cache

Clear all cached data.

**Response:**
```json
{
  "success": true,
  "message": "Cache cleared successfully"
}
```

#### POST /api/v1/performance/cache/warmup

Pre-load frequently accessed data into cache.

**Response:**
```json
{
  "success": true,
  "message": "Cache warmed up successfully",
  "duration_ms": 1250
}
```

## Data Models

### IPO Model

```typescript
interface IPO {
  id: string;                    // UUID
  stock_id: string;              // Stock identifier (unique)
  name: string;                  // IPO name
  company_code: string;          // Normalized company identifier
  symbol?: string;               // Stock symbol
  registrar: string;             // Registrar name
  open_date?: Date;              // IPO opening date
  close_date?: Date;             // IPO closing date
  result_date?: Date;            // Allotment result date
  listing_date?: Date;           // Expected listing date
  price_band_low?: number;       // Lower price band
  price_band_high?: number;      // Upper price band
  issue_size?: string;           // Issue size (e.g., "₹1000 Cr")
  min_qty?: number;              // Minimum application quantity
  min_amount?: number;           // Minimum investment amount
  status: string;                // IPO status (UPCOMING, LIVE, CLOSED, LISTED) - calculated dynamically
  subscription_status?: string;   // Subscription information
  listing_gain?: string;         // Expected/actual listing gain %
  logo_url?: string;             // Company logo URL
  description?: string;          // Company description
  about?: string;                // Detailed company information
  slug?: string;                 // URL-friendly identifier
  strengths: string[];           // Company strengths (JSON array)
  risks: string[];               // Investment risks (JSON array)
  form_url?: string;             // Legacy form URL
  form_fields: object;           // Legacy form fields (JSON)
  form_headers: object;          // Legacy form headers (JSON)
  parser_config: object;         // Legacy parser config (JSON)
  created_at: Date;
  updated_at: Date;
  created_by?: string;
}
```

### IPOWithGMP Model

```typescript
interface IPOWithGMP extends IPO {
  gmp_value?: number;            // Grey market premium value
  gain_percent?: number;         // Expected gain percentage
  estimated_listing?: number;    // Estimated listing price
  gmp_last_updated?: Date;       // Last GMP update timestamp
}
```

### GMP Model

```typescript
interface GMP {
  id: string;
  ipo_name: string;
  company_code: string;
  ipo_price: number;
  gmp_value: number;
  estimated_listing: number;
  gain_percent: number;
  sub2?: number;                 // Subscription data
  kostak?: number;               // Kostak rate
  listing_date?: Date;
  last_updated: Date;
}
```

### Market Index Model

```typescript
interface MarketIndex {
  id: string;                    // Index identifier (e.g., "nifty50", "sensex")
  name: string;                  // Display name (e.g., "NIFTY 50", "SENSEX")
  value: number;                 // Current index value
  change: number;                // Absolute change
  change_percent: number;        // Percentage change
  is_positive: boolean;          // Whether change is positive
}
```

### Cache Model

```typescript
interface IPOResultCache {
  id: string;                    // UUID
  pan_hash: string;              // Hashed PAN number
  ipo_id: string;                // IPO UUID
  status: string;                // Allotment status
  shares_allotted: number;       // Number of shares allotted
  application_number: string;    // Application number
  refund_status: string;         // Refund status
  source: string;                // Data source
  user_agent: string;            // User agent string
  timestamp: Date;               // Cache timestamp
  expires_at: Date;              // Cache expiration
  confidence_score: number;      // Confidence score (0-100)
  duplicate_count: number;       // Number of duplicates found
}
```

## Error Codes

| Status Code | Description |
|-------------|-------------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request - Invalid parameters |
| 404 | Not Found - Resource not found |
| 500 | Internal Server Error |
| 502 | Bad Gateway - External service error |

## Rate Limiting

Currently, no rate limiting is implemented. Consider implementing rate limiting for production use.

## Data Sources

- **IPO Data**: Scraped from Chittorgarh.com (updated every 8 hours)
- **GMP Data**: Scraped from InvestorGain.com (updated hourly)
- **Market Data**: Mock data (real-time integration planned)

## Data Freshness

- **IPO Details**: Updated every 8 hours via background job
- **GMP Data**: Updated hourly via background job
- **Cache**: Results cached with configurable TTL, automatic cleanup every 12 hours
- **Performance**: Cache warmup on startup, metrics tracking enabled

## Performance Features

- **Caching Layer**: Intelligent caching with hit rate tracking
- **Connection Pooling**: Optimized database connections
- **Query Optimization**: Indexed queries with execution plan analysis
- **Background Jobs**: Automated data updates and cache management
- **Performance Monitoring**: Real-time metrics and load testing endpoints

## Background Jobs

- **Daily IPO Update**: Runs every 8 hours, scrapes latest IPO data
- **GMP Update**: Runs hourly, updates Grey Market Premium data
- **Result Check**: Runs hourly, checks for result announcements
- **Cache Cleanup**: Runs every 12 hours, removes expired cache entries

## Changelog

### Version 3.0 (Service Alignment Enhancement)

**New Features:**
- Enhanced service architecture with standardized patterns
- Improved error handling and logging consistency
- Optimized HTTP clients with connection pooling
- Advanced text processing and data normalization
- Comprehensive performance monitoring endpoints

**Performance Endpoints:**
- `GET /api/v1/performance/metrics` - Real-time performance metrics
- `POST /api/v1/performance/test` - Comprehensive performance testing
- `DELETE /api/v1/performance/cache` - Cache management
- `POST /api/v1/performance/cache/warmup` - Cache pre-loading

**Enhanced Features:**
- Standardized configuration management across all services
- Error isolation to prevent cascading failures
- Structured logging with consistent field names
- Resource management with proper cleanup
- Database query optimization with execution plan analysis

### Version 2.0 (Enhanced IPO Scraping)

**New Endpoints:**
- `GET /api/v1/ipos/active-with-gmp` - Active IPOs with GMP data
- `GET /api/v1/ipos/:id/with-gmp` - Single IPO with GMP data

**Enhanced Features:**
- Complete IPO field population from Chittorgarh scraping
- GMP data integration from InvestorGain
- Improved data normalization and matching
- Separate storage for static IPO data and dynamic GMP data
- Enhanced error handling and logging
- **Dynamic status calculation**: IPO status now calculated in real-time based on current date and IPO timeline
  - "UPCOMING": Before open_date
  - "LIVE": Between open_date and close_date
  - "CLOSED": After close_date (before listing_date)  
  - "LISTED": After listing_date

**Database Changes:**
- Added indexes on `company_code` for both `ipo_list` and `ipo_gmp` tables
- Enhanced GMP table structure with additional fields
- Improved data normalization for cross-table matching
- Connection pooling and query optimization

## Support

For API support or questions, please contact the development team.