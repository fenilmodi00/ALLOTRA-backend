# GMP Price History Archival Implementation

## Overview

This document describes the enhanced archival logic implementation for the GMP price history feature, completing Task 8.1 of the gmp-price-history spec.

## Features Implemented

### 1. Age-Based Archival
- **Method**: `ArchiveOldHistory(cutoffDate time.Time) (*ArchivalReport, error)`
- **Purpose**: Archives price history entries older than a specified cutoff date (typically 2 years)
- **Process**:
  1. Counts records to be archived
  2. Creates archive table if it doesn't exist
  3. Copies old records to `gmp_price_history_archive` table
  4. Deletes archived records from main table
  5. Logs operation to `gmp_archival_log` tracking table
  6. Returns detailed archival report

### 2. Volume-Based Archival
- **Method**: `ArchiveByVolume(volumeThreshold int) (*ArchivalReport, error)`
- **Purpose**: Archives oldest 20% of records when total volume exceeds threshold
- **Process**:
  1. Checks current total record count
  2. If below threshold, returns without archival
  3. If above threshold, calculates cutoff date for oldest 20% of records
  4. Archives records up to cutoff date
  5. Logs operation and returns report

### 3. Archival Status Tracking
- **Table**: `gmp_archival_log`
- **Fields**:
  - `archival_id`: Unique identifier for each archival operation
  - `start_time`, `end_time`: Operation timestamps
  - `trigger_type`: "age-based" or "volume-based"
  - `cutoff_date`: Date threshold used for archival
  - `records_archived`: Number of records moved to archive
  - `ipos_affected`: Number of unique IPOs affected
  - `archival_status`: "success", "partial", or "failed"
  - `error_message`: Error details if operation failed
  - `processing_time_ms`: Duration of operation in milliseconds

### 4. Archival Reporting
- **Type**: `ArchivalReport` struct
- **Contains**:
  - Operation metadata (ID, timestamps, trigger type)
  - Results (records archived, IPOs affected)
  - Status and error information
  - Processing time metrics

### 5. Helper Methods

#### CheckArchivalNeeded
```go
CheckArchivalNeeded(volumeThreshold int, ageThresholdYears int) (bool, string, error)
```
- Checks if archival is needed based on volume or age thresholds
- Returns boolean indicating need, reason string, and error
- Useful for monitoring and automated triggers

#### GetArchivalStatistics
```go
GetArchivalStatistics() (map[string]interface{}, error)
```
- Returns comprehensive statistics about archived and active data
- Includes:
  - Active and archived record counts
  - Oldest and newest active record dates
  - Total archival operations count
  - Last archival operation details

#### GetArchivalHistory
```go
GetArchivalHistory(limit int) ([]ArchivalReport, error)
```
- Retrieves history of archival operations
- Returns list of archival reports ordered by most recent first
- Useful for auditing and monitoring

## Database Schema

### Archive Table
```sql
CREATE TABLE IF NOT EXISTS gmp_price_history_archive (
    LIKE gmp_price_history INCLUDING ALL
)
```
- Identical structure to main table
- Stores archived historical data
- Maintains all constraints and indexes

### Tracking Table
```sql
CREATE TABLE IF NOT EXISTS gmp_archival_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    archival_id VARCHAR(100) UNIQUE NOT NULL,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    trigger_type VARCHAR(50) NOT NULL,
    cutoff_date DATE,
    records_archived INTEGER DEFAULT 0,
    ipos_affected INTEGER DEFAULT 0,
    archival_status VARCHAR(50) DEFAULT 'running',
    error_message TEXT,
    processing_time_ms INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
```

## Usage Examples

### Age-Based Archival (2 Years)
```go
service := NewGMPHistoryService(db)
cutoffDate := time.Now().AddDate(-2, 0, 0)
report, err := service.ArchiveOldHistory(cutoffDate)
if err != nil {
    log.Fatalf("Archival failed: %v", err)
}
log.Printf("Archived %d records affecting %d IPOs", 
    report.RecordsArchived, report.IPOsAffected)
```

### Volume-Based Archival
```go
service := NewGMPHistoryService(db)
volumeThreshold := 100000 // Archive when exceeding 100k records
report, err := service.ArchiveByVolume(volumeThreshold)
if err != nil {
    log.Fatalf("Archival failed: %v", err)
}
log.Printf("Status: %s, Records archived: %d", 
    report.ArchivalStatus, report.RecordsArchived)
```

### Check if Archival Needed
```go
service := NewGMPHistoryService(db)
needed, reason, err := service.CheckArchivalNeeded(100000, 2)
if err != nil {
    log.Fatalf("Check failed: %v", err)
}
if needed {
    log.Printf("Archival needed: %s", reason)
    // Trigger archival process
}
```

### Get Archival Statistics
```go
service := NewGMPHistoryService(db)
stats, err := service.GetArchivalStatistics()
if err != nil {
    log.Fatalf("Failed to get statistics: %v", err)
}
log.Printf("Active records: %v", stats["active_records"])
log.Printf("Archived records: %v", stats["archived_records"])
log.Printf("Oldest active date: %v", stats["oldest_active_date"])
```

## Testing

### Test Coverage
- **TestArchiveOldHistory_AgeBasedArchival**: Tests age-based archival with 2-year cutoff
- **TestArchiveByVolume_VolumeBasedArchival**: Tests volume-based archival when threshold exceeded
- **TestArchiveByVolume_BelowThreshold**: Tests that no archival occurs when below threshold
- **TestCheckArchivalNeeded**: Tests archival need detection logic
- **TestGetArchivalStatistics**: Tests statistics retrieval
- **TestGetArchivalHistory**: Tests archival history retrieval
- **TestArchiveOldHistory_Integration**: Integration test with real database

All tests pass successfully with proper cleanup.

## Requirements Satisfied

### Requirement 2.5: Data Lifecycle Management
✅ Implemented automatic archival for entries older than 2 years
✅ Archive table maintains referential integrity
✅ Transactional archival ensures data consistency

### Requirement 7.5: Volume-Based Archival Triggers
✅ Implemented volume threshold checking
✅ Automatic archival of oldest 20% when threshold exceeded
✅ Maintains query performance by keeping active table size manageable

### Additional Features
✅ Comprehensive archival status tracking
✅ Detailed reporting with metrics
✅ Error handling and partial success tracking
✅ Audit trail for all archival operations
✅ Helper methods for monitoring and automation

## Performance Considerations

1. **Transactional Safety**: All archival operations use database transactions
2. **Batch Processing**: Archives records in single operations for efficiency
3. **Index Preservation**: Archive table includes all indexes from main table
4. **Minimal Downtime**: Archival can run while system is operational
5. **Monitoring**: Statistics and history methods enable performance tracking

## Future Enhancements

1. **Scheduled Archival Job**: Add background job to run archival automatically
2. **Compression**: Implement compression for archived data
3. **Cold Storage**: Move very old archives to separate database or storage
4. **Restore Functionality**: Add method to restore archived data if needed
5. **Configurable Thresholds**: Make age and volume thresholds configurable via environment variables

## Maintenance

### Regular Monitoring
- Check archival statistics weekly
- Review archival history for failures
- Monitor active table size growth

### Recommended Schedule
- **Age-based archival**: Run monthly to archive data older than 2 years
- **Volume-based archival**: Run when active records exceed 100,000
- **Statistics review**: Weekly monitoring of data distribution

### Troubleshooting
- Check `gmp_archival_log` table for failed operations
- Review error messages in archival reports
- Verify archive table exists and has correct schema
- Ensure sufficient database storage for archive table
