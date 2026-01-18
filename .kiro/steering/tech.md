# Technology Stack & Build System

## Core Technologies

### Backend Framework
- **Go 1.24.3**: Primary programming language
- **Fiber v2**: High-performance HTTP web framework
- **PostgreSQL 15**: Primary database with JSONB support
- **Docker**: Containerization and deployment

### Key Libraries
- **Database**: `lib/pq` (PostgreSQL driver), `database/sql` (standard library)
- **Web Scraping**: `gocolly/colly`, `PuerkitoBio/goquery`, `chromedp/chromedp`
- **HTTP Client**: `gofiber/fiber/v2` with custom rate limiting
- **Logging**: `sirupsen/logrus` with structured logging
- **Configuration**: `joho/godotenv` for environment management
- **Testing**: `leanovate/gopter` for property-based testing
- **UUID**: `google/uuid` for unique identifiers

### Architecture Patterns
- **Service Layer Pattern**: Business logic in services package
- **Handler Pattern**: HTTP handlers in handlers package
- **Repository Pattern**: Data access through service layer
- **Background Jobs**: Scheduled tasks with ticker-based execution
- **Caching Layer**: In-memory and database-backed caching

## Build System

### Development Commands
```bash
# Start development environment
docker-compose up -d db
go run main.go

# Hot reload development (with air)
go install github.com/cosmtrek/air@latest
air

# Run tests
go test ./...

# Property-based tests
go test -v ./tests/
```

### Build Commands
```bash
# Build binary
go build -o ipo-backend-enhanced .

# Build with optimizations
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Docker build
docker build -t ipo-backend .

# Multi-stage build (production)
docker build --target production -t ipo-backend:prod .
```

### Database Management
```bash
# Start database
docker-compose up -d db

# Apply schema
docker exec -i ipo_db psql -U user -d ipo_db < database/schema.sql

# Database validation
docker exec -i ipo_db psql -U user -d ipo_db -c "SELECT validate_schema();"

# Maintenance
docker exec -i ipo_db psql -U user -d ipo_db -c "SELECT quick_maintenance();"
```

### Deployment
```bash
# Staging deployment
./deploy.sh staging

# Production deployment
./deploy.sh production --version v2.0.1

# Rollback
./deploy.sh rollback

# View logs
./deploy.sh logs --tail 100

# Create backup
./deploy.sh backup
```

### Testing Commands
```bash
# Unit tests
go test ./services/... -v

# Integration tests
go test ./tests/... -v

# Property-based tests
go test ./tests/ -run TestProperty -v

# Performance tests
go test ./tests/ -run TestPerformance -v

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Environment Configuration

### Required Environment Variables
```bash
# Database
DATABASE_URL=postgres://user:password@localhost:5432/ipo_db

# Server
SERVER_PORT=8080
LOG_LEVEL=info

# Cache
CACHE_TTL_HOURS=24
CACHE_MAX_SIZE=1000

# Scraping
SCRAPING_RATE_LIMIT=2s
HTTP_TIMEOUT=30s
```

### Development Setup
1. Install Go 1.24.3+
2. Install Docker and Docker Compose
3. Copy `.env.prod.template` to `.env`
4. Run `docker-compose up -d db`
5. Apply database schema
6. Run `go run main.go`

## Performance Considerations
- Connection pooling for database connections
- Rate limiting for external API calls
- Caching layer with TTL management
- Background job scheduling for data updates
- Query optimization with proper indexing