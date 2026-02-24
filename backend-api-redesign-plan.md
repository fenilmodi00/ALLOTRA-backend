# IPO App API Redesign — Backend Implementation Plan

## Overview

This document outlines the complete backend changes required to implement the optimized v2 API endpoints. The backend is built in **Go** using **Chi** or **Gin** router, connected to **PostgreSQL**.

---

## Current State

### Existing Database Schema (Assumed)

Based on the frontend types and API responses, the current database schema likely includes:

```sql
-- Main IPO table
CREATE TABLE ipos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stock_id VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(500) NOT NULL,
    company_code VARCHAR(50),
    symbol VARCHAR(20),
    registrar VARCHAR(255),
    open_date DATE,
    close_date DATE,
    result_date DATE,
    listing_date DATE,
    price_band_low DECIMAL(12,2),
    price_band_high DECIMAL(12,2),
    issue_size VARCHAR(100),
    min_qty INTEGER,
    min_amount DECIMAL(15,2),
    status VARCHAR(20) DEFAULT 'UPCOMING',
    subscription_status VARCHAR(50),
    listing_gain VARCHAR(50),
    logo_url TEXT,
    description TEXT,
    about TEXT,
    strengths TEXT[],
    risks TEXT[],
    slug VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    created_by VARCHAR(255)
);

-- GMP data table
CREATE TABLE gmp_data (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stock_id VARCHAR(255) REFERENCES ipos(stock_id),
    gmp_value DECIMAL(12,2),
    gain_percent DECIMAL(8,4),
    estimated_listing DECIMAL(12,2),
    gmp_last_updated TIMESTAMP,
    gmp_subscription_status VARCHAR(50),
    gmp_listing_gain VARCHAR(50),
    gmp_ipo_status VARCHAR(20),
    gmp_data_source VARCHAR(50),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

---

## New API Endpoints (v2)

### Endpoint 1: `GET /api/v2/ipos/feed`

**Replaces**: `/ipos`, `/ipos/active`, `/ipos/active-with-gmp`

**HTTP Method**: `GET`

**Path**: `/api/v2/ipos/feed`

**Query Parameters**:
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `status` | string | No | `all` | Filter: `all`, `live`, `upcoming`, `closed`, `listed` |

**Response**:
```json
{
  "success": true,
  "data": [
    {
      "id": "uuid-123",
      "stock_id": "hexaware-tech",
      "name": "Hexaware Technologies",
      "logo_url": "https://cdn.example.com/logos/hexaware.png",
      "status": "LIVE",
      "category": "mainboard",
      "price_band_low": 674,
      "price_band_high": 708,
      "open_date": "2026-02-12",
      "close_date": "2026-02-14",
      "listing_date": "2026-02-19",
      "gmp": {
        "value": 45,
        "gain_percent": 6.36
      }
    }
  ]
}
```

**SQL Query**:
```sql
SELECT
    i.id,
    i.stock_id,
    i.name,
    i.logo_url,
    i.status,
    i.price_band_low,
    i.price_band_high,
    i.open_date,
    i.close_date,
    i.listing_date,
    i.issue_size,
    g.gmp_value,
    g.gain_percent
FROM ipos i
LEFT JOIN gmp_data g ON g.stock_id = i.stock_id
WHERE ($1 = 'all' OR i.status = $1)
ORDER BY 
    CASE i.status 
        WHEN 'LIVE' THEN 1 
        WHEN 'UPCOMING' THEN 2 
        WHEN 'CLOSED' THEN 3 
        WHEN 'LISTED' THEN 4 
        ELSE 5 
    END,
    i.open_date DESC;
```

**Category Computation (Server-Side)**:
```go
func computeCategory(issueSize string) string {
    if issueSize == "" {
        return "mainboard"
    }
    
    // Remove commas and parse
    sizeStr := strings.ReplaceAll(issueSize, ",", "")
    
    var sizeValue float64
    _, err := fmt.Sscanf(sizeStr, "%f", &sizeValue)
    if err != nil {
        return "mainboard"
    }
    
    // Convert to crores if needed (assume rupees if > 100000)
    if sizeValue > 100000 {
        sizeValue = sizeValue / 10000000 // Convert to crores
    }
    
    // SME threshold: < 25 crores
    if sizeValue > 0 && sizeValue < 25 {
        return "sme"
    }
    return "mainboard"
}
```

**Go Handler**:
```go
// internal/handler/ipo_handler.go

type FeedResponse struct {
    ID            string    `json:"id"`
    StockID       string    `json:"stock_id"`
    Name          string    `json:"name"`
    LogoURL       *string   `json:"logo_url"`
    Status        string    `json:"status"`
    Category      string    `json:"category"`
    PriceBandLow  float64   `json:"price_band_low"`
    PriceBandHigh float64   `json:"price_band_high"`
    OpenDate      *string   `json:"open_date"`
    CloseDate     *string   `json:"close_date"`
    ListingDate   *string   `json:"listing_date"`
    GMP           *GMPFeed  `json:"gmp,omitempty"`
}

type GMPFeed struct {
    Value       float64 `json:"value"`
    GainPercent float64 `json:"gain_percent"`
}

func (h *IPOHandler) GetFeed(w http.ResponseWriter, r *http.Request) {
    status := chi.URLParam(r, "status")
    if status == "" {
        status = "all"
    }

    ipos, err := h.ipoService.GetFeed(r.Context(), status)
    if err != nil {
        respondError(w, r, err)
        return
    }

    respondSuccess(w, r, ipos)
}
```

**Caching**:
- Cache TTL: 60 seconds
- Cache key: `ipos:feed:{status}`
- Invalidation: On any IPO status change

---

### Endpoint 2: `GET /api/v2/ipos/:id`

**Replaces**: `/ipos/:id`, `/ipos/:id/with-gmp`, `/ipos/:id/gmp`

**HTTP Method**: `GET`

**Path**: `/api/v2/ipos/{id}`

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "uuid-123",
    "stock_id": "hexaware-tech",
    "name": "Hexaware Technologies",
    "logo_url": "https://cdn.example.com/logos/hexaware.png",
    "status": "LIVE",
    "category": "mainboard",
    "registrar": "Link Intime India Pvt Ltd",
    "price_band_low": 674,
    "price_band_high": 708,
    "min_investment": 14868,
    "lot_size": 21,
    "issue_size": "2508000000.00",
    "subscription_status": "2.5x",
    "open_date": "2026-02-12",
    "close_date": "2026-02-14",
    "allotment_date": "2026-02-17",
    "listing_date": "2026-02-19",
    "description": "Hexaware Technologies is a global IT services company...",
    "gmp": {
      "value": 45,
      "gain_percent": 6.36,
      "subscription_status": "2.5x"
    }
  }
}
```

**SQL Query**:
```sql
SELECT
    i.id,
    i.stock_id,
    i.name,
    i.logo_url,
    i.status,
    i.issue_size,
    i.registrar,
    i.price_band_low,
    i.price_band_high,
    i.min_amount,
    i.min_qty,
    i.subscription_status,
    i.open_date,
    i.close_date,
    i.result_date,
    i.listing_date,
    i.description,
    g.gmp_value,
    g.gain_percent,
    g.gmp_subscription_status
FROM ipos i
LEFT JOIN gmp_data g ON g.stock_id = i.stock_id
WHERE i.id = $1;
```

**Go Handler**:
```go
// internal/handler/ipo_handler.go

type DetailResponse struct {
    ID                  string    `json:"id"`
    StockID             string    `json:"stock_id"`
    Name                string    `json:"name"`
    LogoURL             *string   `json:"logo_url"`
    Status              string    `json:"status"`
    Category            string    `json:"category"`
    Registrar           *string   `json:"registrar"`
    PriceBandLow        float64   `json:"price_band_low"`
    PriceBandHigh       float64   `json:"price_band_high"`
    MinInvestment       *float64  `json:"min_investment,omitempty"`
    LotSize             *int      `json:"lot_size,omitempty"`
    IssueSize           *string   `json:"issue_size,omitempty"`
    SubscriptionStatus  *string   `json:"subscription_status,omitempty"`
    OpenDate            *string   `json:"open_date,omitempty"`
    CloseDate           *string   `json:"close_date,omitempty"`
    AllotmentDate       *string   `json:"allotment_date,omitempty"`
    ListingDate         *string   `json:"listing_date,omitempty"`
    Description         *string   `json:"description,omitempty"`
    GMP                 *GMPDetail `json:"gmp,omitempty"`
}

type GMPDetail struct {
    Value                float64  `json:"value"`
    GainPercent          float64  `json:"gain_percent"`
    SubscriptionStatus   *string  `json:"subscription_status,omitempty"`
}

func (h *IPOHandler) GetByID(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    
    ipo, err := h.ipoService.GetByID(r.Context(), id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            respondError(w, r, &APIError{Code: "NOT_FOUND", Message: "IPO not found"})
            return
        }
        respondError(w, r, err)
        return
    }

    respondSuccess(w, r, ipo)
}
```

**Caching**:
- Cache TTL: 30 seconds
- Cache key: `ipos:detail:{id}`
- Invalidation: On IPO update

---

### Endpoint 3: `POST /api/v2/allotment/check`

**Replaces**: `/check`

**HTTP Method**: `POST`

**Path**: `/api/v2/allotment/check`

**Request**:
```json
{
  "ipo_id": "uuid-123",
  "pan": "ABCDE1234F"
}
```

**Validation Rules**:
| Field | Type | Required | Validation |
|-------|------|----------|------------|
| `ipo_id` | string | Yes | Valid UUID |
| `pan` | string | Yes | Regex: `^[A-Z]{5}[0-9]{4}[A-Z]$` |

**Response**:
```json
{
  "success": true,
  "data": {
    "status": "ALLOTTED",
    "shares_applied": 21,
    "shares_allotted": 21,
    "message": "Congratulations! Shares have been allotted."
  }
}
```

**Error Responses**:
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid PAN format",
    "details": {
      "field": "pan",
      "expected": "ABCDE1234F format"
    }
  }
}
```

```json
{
  "success": false,
  "error": {
    "code": "NOT_FOUND",
    "message": "IPO with id 'abc' not found",
    "details": {}
  }
}
```

**Go Handler**:
```go
// internal/handler/allotment_handler.go

type AllotmentRequest struct {
    IpoID string `json:"ipo_id" validate:"required,uuid"`
    PAN   string `json:"pan" validate:"required,pan"`
}

type AllotmentResponse struct {
    Status        string `json:"status"`
    SharesApplied int    `json:"shares_applied"`
    SharesAllotted int   `json:"shares_allotted"`
    Message       string `json:"message"`
}

// PAN validator
var panRegex = regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]$`)

func ValidatePAN(fl validator.FieldLevel) bool {
    return panRegex.MatchString(fl.Field().String())
}

func (h *AllotmentHandler) Check(w http.ResponseWriter, r *http.Request) {
    var req AllotmentRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, r, &APIError{Code: "INVALID_REQUEST", Message: "Invalid JSON"})
        return
    }

    // Validate
    if !panRegex.MatchString(req.PAN) {
        respondError(w, r, &APIError{
            Code:    "VALIDATION_ERROR",
            Message: "Invalid PAN format",
            Details: map[string]string{"field": "pan", "expected": "ABCDE1234F format"},
        })
        return
    }

    result, err := h.allotmentService.Check(r.Context(), req.IpoID, req.PAN)
    if err != nil {
        respondError(w, r, err)
        return
    }

    respondSuccess(w, r, result)
}
```

**Rate Limiting**:
- 10 requests per minute per IP
- Use `httprateLimiter` middleware

---

### Endpoint 4: `GET /api/v2/gmp/history/:stockId/chart`

**Status**: Keep as-is (already lean)

No changes needed.

---

## Project Structure

```
backend/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── handler/
│   │   ├── ipo_handler.go       # Feed + Detail endpoints
│   │   ├── allotment_handler.go # Allotment check
│   │   └── gmp_handler.go      # GMP history
│   ├── service/
│   │   ├── ipo_service.go      # Business logic
│   │   └── allotment_service.go
│   ├── repository/
│   │   ├── ipo_repository.go    # Database queries
│   │   └── db.go              # sqlc generated
│   ├── model/
│   │   ├── feed.go            # Feed response structs
│   │   ├── detail.go          # Detail response structs
│   │   └── allotment.go       # Allotment response structs
│   ├── middleware/
│   │   ├── rate_limiter.go    # Rate limiting
│   │   ├── cors.go            # CORS
│   │   └── logger.go          # Request logging
│   └── cache/
│       └── cache.go           # In-memory cache
├── migrations/
│   └── 001_initial.sql
├── go.mod
├── go.sum
├── .env
└── Dockerfile
```

---

## Implementation Tasks

### Phase 1: Database

#### Task 1.1: Add Category Column (Optional)
```sql
ALTER TABLE ipos ADD COLUMN category VARCHAR(20) DEFAULT 'mainboard';
-- Or compute on-the-fly in queries (recommended)
```

### Phase 2: Project Setup

#### Task 2.1: Initialize Go Module
```bash
go mod init github.com/yourorg/ipo-api
go get github.com/go-chi/chi/v5
go get github.com/jackc/pgx/v5
go get github.com/joho/godotenv
go get github.com/go-playground/validator/v10
go get github.com/redis/go-redis/v9  # Optional, if using Redis
```

#### Task 2.2: Set Up sqlc for Type-Safe Queries
```bash
go install github.com/sqlc-dev/sqlc@latest
```

Create `sqlc.yaml`:
```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/repository/queries"
    gen:
      go:
        package: "db"
        out: "internal/repository/db"
        sql_package: "pgx/v5"
```

### Phase 3: Implement Handlers

#### Task 3.1: Create Response Models
Create files in `internal/model/`:
- `feed.go` — 12 fields
- `detail.go` — 19 fields
- `allotment.go` — 4 fields

#### Task 3.2: Create Handlers
Create files in `internal/handler/`:
- `ipo_handler.go` — Feed + Detail
- `allotment_handler.go` — Allotment check

#### Task 3.3: Create Services
Create files in `internal/service/`:
- `ipo_service.go` — Category computation, cache logic
- `allotment_service.go` — PAN validation, lookup

### Phase 4: Middleware

#### Task 4.1: Rate Limiting
```go
// internal/middleware/rate_limiter.go
import (
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/rate"
)

func NewRateLimiter() func(h http.Handler) http.Handler {
    return rate.NewLimiter(
        rate.Limit(60),    // 60 requests
        60,                // per 60 seconds
    ).Middleware
}
```

#### Task 4.2: CORS
```go
// internal/middleware/cors.go
func CORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

### Phase 5: Caching

#### Task 5.1: In-Memory Cache
```go
// internal/cache/cache.go
type Cache struct {
    mu    sync.RWMutex
    items map[string]cacheItem
}

type cacheItem struct {
    value      interface{}
    expiration time.Time
}

func (c *Cache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    item, found := c.items[key]
    if !found || time.Now().After(item.expiration) {
        return nil, false
    }
    return item.value, true
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.items[key] = cacheItem{
        value:      value,
        expiration: time.Now().Add(ttl),
    }
}
```

### Phase 6: Router Setup

#### Task 6.1: Main Router
```go
// cmd/server/main.go

func main() {
    r := chi.NewRouter()
    
    // Middleware
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.CORS)
    r.Use(rateLimiter)
    
    // Routes
    r.Route("/api/v2", func(r chi.Router) {
        r.Get("/ipos/feed", ipoHandler.GetFeed)
        r.Get("/ipos/{id}", ipoHandler.GetByID)
        r.Post("/allotment/check", allotmentHandler.Check)
        r.Get("/gmp/history/{stockId}/chart", gmpHandler.GetHistory)
    })
    
    http.ListenAndServe(":8080", r)
}
```

### Phase 7: Dockerize

#### Task 7.1: Dockerfile
```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o server ./cmd/server

# Run stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/server /server
COPY --from=builder /app/.env /.env
EXPOSE 8080
CMD ["/server"]
```

---

## Remove Old Endpoints

After frontend migration is complete, remove these old endpoints:

| Method | Old Path | Action |
|--------|----------|--------|
| GET | `/ipos` | Remove |
| GET | `/ipos/active` | Remove |
| GET | `/ipos/active-with-gmp` | Remove |
| GET | `/ipos/{id}` | Remove |
| GET | `/ipos/{id}/with-gmp` | Remove |
| GET | `/ipos/{id}/gmp` | Remove |
| POST | `/check` | Remove |
| GET | `/market/stocks/most-traded` | Remove (dead code) |
| GET | `/market/stocks/gainers` | Remove (dead code) |
| GET | `/market/stocks/losers` | Remove (dead code) |
| GET | `/market/stocks/{symbol}` | Remove (dead code) |
| GET | `/market/stocks/search` | Remove (dead code) |
| GET | `/performance/metrics` | Remove (internal only) |
| POST | `/performance/cache/warmup` | Keep (internal) |

---

## Configuration

### Environment Variables
```env
# Server
PORT=8080
ENV=development

# Database
DATABASE_URL=postgres://user:password@localhost:5432/ipo_db?sslmode=disable

# Cache
CACHE_TTL_FEED=60s
CACHE_TTL_DETAIL=30s

# Rate Limiting
RATE_LIMIT_IPO_FEED=60
RATE_LIMIT_DETAIL=120
RATE_LIMIT_ALLOTMENT=10
```

---

## Testing

### Manual Testing Checklist
- [ ] `GET /api/v2/ipos/feed` returns correct fields
- [ ] `GET /api/v2/ipos/feed?status=live` filters correctly
- [ ] `GET /api/v2/ipos/{id}` includes GMP data
- [ ] `POST /api/v2/allotment/check` validates PAN format
- [ ] Invalid PAN returns 422 error
- [ ] Rate limiting returns 429 after threshold
- [ ] Cache reduces database load

### Load Testing (Optional)
```bash
# Install hey for load testing
go install github.com/rakyll/hey@latest

# Test feed endpoint
hey -n 1000 -c 10 "http://localhost:8080/api/v2/ipos/feed"
```

---

## Deployment

### Recommended: Render.com (Free Tier)

1. Create new Web Service
2. Connect GitHub repository
3. Build command: `go build -o server ./cmd/server`
4. Start command: `./server`
5. Add environment variables
6. Deploy

### Recommended: Fly.io (Better Free DB)

1. Install `flyctl`
2. `fly launch` — creates `fly.toml`
3. `fly postgres create` — free 3GB PostgreSQL
4. `fly deploy`

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Payload size (feed, 50 IPOs) | < 15KB (from ~40KB) |
| Payload size (detail) | < 8KB (from ~12KB) |
| Response time (p95) | < 200ms |
| Database queries per request | 1 (JOIN, no N+1) |
| Cache hit rate | > 80% for feed |

---

## Rollout Timeline

| Week | Task |
|------|------|
| Week 1 | Set up Go project, handlers, repository layer |
| Week 2 | Implement v2 endpoints with caching |
| Week 3 | Add rate limiting, error handling, testing |
| Week 4 | Deploy to staging, frontend integration |
| Week 5 | Production deploy, monitor |
| Week 6+ | Remove v1 endpoints after frontend rollout |

---

## Appendix: Response Envelope Standardization

All responses follow this envelope:

```go
type APIResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   *APIError   `json:"error,omitempty"`
}

type APIError struct {
    Code    string      `json:"code"`
    Message string      `json:"message"`
    Details interface{} `json:"details,omitempty"`
}
```

---

*Generated: February 2026*
*Part of: IPO App API Redesign Project*
