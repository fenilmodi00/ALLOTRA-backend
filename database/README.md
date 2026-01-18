# Database Management

This folder contains the core database files for the IPO backend system.

## Core Files

### Essential Files (DO NOT DELETE)
- `schema.sql` - Complete database schema with all tables, indexes, and constraints
- `postgres.go` - Database connection and validation logic
- `unified_batch_processor.go` - Batch processing framework for data operations
- `ipo_batch_operation.go` - IPO-specific batch operations
- `unified_gmp_batch_operation.go` - GMP-specific batch operations

### Documentation
- `README.md` - This documentation file

## Database Setup

### Fresh Installation
```bash
# Start database
docker-compose up -d db

# Apply schema (includes management functions)
docker exec -i ipo_db psql -U user -d ipo_db < database/schema.sql
```

### Schema Validation
```sql
-- Check if schema is properly set up
SELECT validate_schema();

-- Get database statistics
SELECT * FROM get_database_stats();
```

### Maintenance
```sql
-- Quick maintenance (clean cache, update stats)
SELECT quick_maintenance();

-- Clean expired cache entries only
SELECT cleanup_expired_cache();
```

## Tables Overview

- `ipo_list` - Main IPO data (53 records currently)
- `ipo_gmp` - Grey Market Premium data (linked to IPO data)
- `ipo_result_cache` - Allotment check results cache
- `ipo_update_log` - Audit trail for data changes

## Important Notes

1. **Never create temporary migration files** - Update `schema.sql` directly
2. **Always test schema changes** on a copy first
3. **Use the batch processors** for data operations to ensure consistency
4. **Run maintenance regularly** to keep performance optimal

## Troubleshooting

### API Error: "column does not exist"
- Run `SELECT validate_schema();` to check for missing columns
- If validation fails, re-apply the schema

### Performance Issues
- Run `SELECT quick_maintenance();` to update statistics
- Check slow queries with the monitoring functions

### Data Inconsistency
- Use the unified batch processors instead of direct SQL
- Check the audit log in `ipo_update_log` table