# Project Structure & Organization

## Directory Layout

```
ipo-backend/
├── config/           # Configuration management
├── database/         # Database schema and connection logic
├── handlers/         # HTTP request handlers (controllers)
├── jobs/            # Background job implementations
├── middleware/      # HTTP middleware (currently empty)
├── models/          # Data models and structures
├── services/        # Business logic layer
├── shared/          # Shared utilities and common code
├── tests/           # Test files (unit, integration, property-based)
├── utils/           # Utility functions (currently empty)
└── dev/             # Development utilities
```

## Core Packages

### `/handlers` - HTTP Layer
- **Purpose**: HTTP request/response handling, input validation, response formatting
- **Pattern**: One handler per domain (IPOHandler, GMPHandler, etc.)
- **Naming**: `{domain}_handler.go`
- **Dependencies**: Services layer only, no direct database access

### `/services` - Business Logic Layer
- **Purpose**: Core business logic, data processing, external API integration
- **Pattern**: Service per domain with clear interfaces
- **Naming**: `{domain}_service.go`
- **Key Services**:
  - `ipo_service.go`: IPO data management and lifecycle
  - `gmp_service.go`: Grey Market Premium data handling
  - `cache_service.go`: Caching layer implementation
  - `simplified_ipo_scraper.go`: Web scraping logic

### `/models` - Data Models
- **Purpose**: Data structures, database models, API contracts
- **Pattern**: One model per entity
- **Naming**: `{entity}.go`
- **Key Models**:
  - `ipo.go`: IPO data structure
  - `gmp.go`: Grey Market Premium model
  - `cache.go`: Cache entry model

### `/database` - Data Layer
- **Purpose**: Database connection, schema management, migrations
- **Key Files**:
  - `schema.sql`: Complete database schema with constraints
  - `postgres.go`: Connection management and utilities
  - `README.md`: Database documentation and procedures

### `/jobs` - Background Processing
- **Purpose**: Scheduled tasks, data synchronization, maintenance
- **Pattern**: One job per scheduled task
- **Key Jobs**:
  - `daily_ipo_update.go`: IPO data scraping (every 8 hours)
  - `gmp_update_job.go`: GMP data updates (hourly)
  - `cache_cleanup.go`: Cache maintenance (every 12 hours)
  - `result_check.go`: Result announcement checking (hourly)

### `/shared` - Common Utilities
- **Purpose**: Shared utilities, common configurations, cross-cutting concerns
- **Key Files**:
  - `errors.go`: Error handling utilities
  - `http_client.go`: HTTP client configuration
  - `rate_limiter.go`: Rate limiting implementation
  - `unified_config.go`: Configuration management

### `/config` - Configuration
- **Purpose**: Application configuration, environment management
- **Pattern**: Centralized configuration loading

## Architectural Patterns

### Layered Architecture
```
HTTP Layer (handlers/) 
    ↓
Business Logic (services/)
    ↓
Data Layer (database/)
```

### Dependency Flow
- Handlers depend on Services
- Services depend on Database/External APIs
- Models are shared across all layers
- Shared utilities used by all layers

### Error Handling
- Structured error responses in handlers
- Error logging in services layer
- Database constraint validation
- Graceful degradation for external API failures

### Configuration Management
- Environment-based configuration
- Default values with overrides
- Validation of required settings
- Separate configs for different environments

## File Naming Conventions

### Go Files
- **Services**: `{domain}_service.go`
- **Handlers**: `{domain}_handler.go`
- **Models**: `{entity}.go`
- **Jobs**: `{task}_job.go` or `{frequency}_{task}.go`
- **Tests**: `{file}_test.go`

### Database Files
- **Schema**: `schema.sql` (single source of truth)
- **Documentation**: `README.md`
- **Connection**: `postgres.go`

### Configuration Files
- **Environment**: `.env`, `.env.prod.template`
- **Docker**: `docker-compose.yml`, `docker-compose.prod.yml`
- **Deployment**: `deploy.sh`

## Code Organization Principles

### Single Responsibility
- Each package has a clear, single purpose
- Services handle one domain area
- Handlers focus only on HTTP concerns

### Dependency Injection
- Services injected into handlers
- Database connections passed to services
- Configuration loaded once and passed down

### Interface Segregation
- Small, focused interfaces
- Easy to mock for testing
- Clear contracts between layers

### Data Flow
1. **Request**: HTTP → Handler → Service → Database
2. **Response**: Database → Service → Handler → HTTP
3. **Background**: Job → Service → Database
4. **External**: Service → HTTP Client → External API

## Testing Structure
- **Unit Tests**: `*_test.go` files alongside source
- **Integration Tests**: `/tests/integration_test.go`
- **Property-Based Tests**: `/tests/*_property_test.go`
- **Performance Tests**: `/tests/performance_test.go`