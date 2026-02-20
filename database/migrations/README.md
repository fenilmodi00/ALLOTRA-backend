# Database Migrations

This directory contains database migration scripts for the IPO backend system.

## Migration Files

### 001_add_gmp_price_history.sql
**Purpose**: Adds GMP price history tracking tables  
**Requirements**: 2.1, 2.4, 2.5  
**Tables Created**:
- `gmp_price_history` - Stores historical GMP price data over time
- `gmp_history_job_log` - Tracks job execution for monitoring

**Features**:
- Foreign key constraints to ensure referential integrity
- Unique constraints to prevent duplicate entries
- Check constraints for data validation
- Optimized indexes for query performance
- Automatic timestamp updates via triggers
- Comprehensive table and column comments

## Applying Migrations

### Using the Shell Script (Recommended)
```bash
# Make the script executable (first time only)
chmod +x database/apply_gmp_history_migration.sh

# Run the migration
./database/apply_gmp_history_migration.sh
```

### Manual Application
```bash
# Ensure database container is running
docker-compose up -d db

# Apply the migration
docker exec -i ipo_db psql -U user -d ipo_db < database/migrations/001_add_gmp_price_history.sql
```

### Verification
```bash
# Check if tables were created
docker exec -i ipo_db psql -U user -d ipo_db -c "\dt gmp_*"

# View table structure
docker exec -i ipo_db psql -U user -d ipo_db -c "\d gmp_price_history"
docker exec -i ipo_db psql -U user -d ipo_db -c "\d gmp_history_job_log"

# Check constraints
docker exec -i ipo_db psql -U user -d ipo_db -c "SELECT constraint_name, constraint_type FROM information_schema.table_constraints WHERE table_name = 'gmp_price_history';"
```

## Schema Validation

The system includes automated schema validation for GMP history tables:

```go
import "github.com/fenilmodi00/ipo-backend/database"

// Validate GMP history schema
err := database.ValidateGMPHistorySchema()
if err != nil {
    log.Fatalf("Schema validation failed: %v", err)
}
```

## Rollback

To rollback the migration:

```sql
-- Drop tables (this will cascade delete all data)
DROP TABLE IF EXISTS gmp_price_history CASCADE;
DROP TABLE IF EXISTS gmp_history_job_log CASCADE;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_gmp_history_updated_at() CASCADE;
```

## Migration Best Practices

1. **Always backup** before applying migrations in production
2. **Test migrations** on a development database first
3. **Use transactions** for complex migrations (already included in script)
4. **Verify data integrity** after migration
5. **Monitor performance** after adding new indexes

## Troubleshooting

### Error: "relation already exists"
The migration uses `CREATE TABLE IF NOT EXISTS`, so this should not occur. If it does, the tables already exist and the migration can be safely skipped.

### Error: "constraint already exists"
The migration checks for existing constraints before adding them. This error indicates a partial migration state. Verify the schema manually.

### Error: "foreign key constraint violation"
Ensure the `ipo_list` table exists and has the required structure before applying this migration.

## Data Model Documentation

### gmp_price_history Table

Stores historical GMP price data for IPOs over time.

**Key Columns**:
- `ipo_id` - Links to ipo_list table
- `company_code` - For cross-referencing with IPO data
- `record_date` - Date when GMP price was recorded
- `gmp_value` - Grey Market Premium value
- `estimated_listing` - Calculated as IPO price + GMP
- `listing_percent` - Percentage gain/loss from IPO price

**Constraints**:
- Unique constraint on (ipo_id, record_date) prevents duplicates
- Check constraint ensures estimated_listing = ipo_price + gmp_value
- Foreign key ensures referential integrity with ipo_list

### gmp_history_job_log Table

Tracks execution of GMP history scraping jobs.

**Key Columns**:
- `job_start_time` - When job started
- `job_end_time` - When job completed (NULL if running)
- `ipos_processed` - Number of IPOs processed
- `successful_scrapes` - Number of successful scrapes
- `failed_scrapes` - Number of failed scrapes
- `execution_status` - Status: running, completed, failed, partial

**Use Cases**:
- Monitor job execution health
- Debug scraping failures
- Track performance metrics
- Audit job history
