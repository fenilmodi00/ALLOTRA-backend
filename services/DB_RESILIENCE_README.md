# Database Resilience and Queuing System

## Overview

The Database Resilience Queue provides automatic handling of database connection failures for the GMP Price History feature. When database operations fail due to connectivity issues, data is temporarily queued and automatically retried when the connection is restored.

## Features

### 1. Automatic Queuing on Connection Failure
- Detects database connection errors automatically
- Queues data temporarily when database is unavailable
- Prevents data loss during temporary outages

### 2. Retry Logic with Exponential Backoff
- Automatically retries failed operations up to 3 times
- Uses exponential backoff (2s, 4s, 8s) with jitter
- Configurable retry parameters

### 3. Circuit Breaker Protection
- Prevents cascading failures during extended outages
- Opens circuit after 5 consecutive failures
- Automatically attempts recovery after 60 seconds
- Allows limited requests in half-open state

### 4. Transaction Management
- All database operations wrapped in transactions
- Automatic rollback on failure
- Ensures data consistency

### 5. Background Processing
- Background worker processes queued data every 30 seconds
- Automatic retry of failed items
- Drops items after maximum retry attempts exceeded

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    GMPHistoryService                         │
│                                                              │
│  SavePriceHistory() ──────────────────────────────────────┐ │
└────────────────────────────────────────────────────────────┼─┘
                                                             │
                                                             ▼
┌─────────────────────────────────────────────────────────────┐
│                  DBResilienceQueue                           │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  SaveWithResilience()                                 │  │
│  │    ├─► Try direct save with retry                     │  │
│  │    ├─► Detect connection errors                       │  │
│  │    └─► Enqueue on failure                             │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Background Worker (every 30s)                        │  │
│  │    ├─► Process queued items                           │  │
│  │    ├─► Retry with exponential backoff                 │  │
│  │    └─► Drop after max retries                         │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Circuit Breaker                                      │  │
│  │    ├─► CLOSED: Normal operation                       │  │
│  │    ├─► OPEN: Block requests (60s timeout)             │  │
│  │    └─► HALF_OPEN: Test recovery                       │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │   PostgreSQL DB   │
                    └──────────────────┘
```

## Usage

### Basic Usage

The resilience queue is automatically integrated into `GMPHistoryService`:

```go
// Create service (resilience queue is automatically initialized)
service := NewGMPHistoryService(db)
defer service.Close() // Important: cleanup background workers

// Save data (automatic resilience handling)
err := service.SavePriceHistory(collection)
if err != nil {
    // Error returned only for non-recoverable failures
    // Connection failures are handled automatically via queuing
    log.Error("Failed to save:", err)
}
```

### Monitoring Queue Status

```go
// Get queue size
size := service.GetResilienceQueueSize()
fmt.Printf("Items in queue: %d\n", size)

// Get detailed metrics
metrics := service.GetResilienceQueueMetrics()
fmt.Printf("Queue size: %v\n", metrics["queue_size"])
fmt.Printf("Total entries: %v\n", metrics["total_entries"])
fmt.Printf("Circuit breaker state: %v\n", metrics["circuit_breaker"])
fmt.Printf("Oldest item age: %v\n", metrics["oldest_item_age"])
```

### Direct Queue Usage (Advanced)

```go
// Create queue directly
queue := NewDBResilienceQueue(db, logger)
queue.Start()
defer queue.Stop()

// Save with resilience
err := queue.SaveWithResilience(collection)

// Manual enqueue (if needed)
err := queue.Enqueue(collection)

// Get metrics
metrics := queue.GetQueueMetrics()
```

## Configuration

### Queue Configuration

```go
type DBResilienceQueue struct {
    maxQueueSize   int           // Default: 1000 items
    maxRetries     int           // Default: 3 attempts
    retryInterval  time.Duration // Default: 30 seconds
}
```

### Retry Configuration

```go
retryConfig := shared.RetryConfig{
    MaxAttempts:  3,                    // Total attempts (including initial)
    InitialDelay: 2 * time.Second,      // First retry after 2s
    MaxDelay:     30 * time.Second,     // Cap at 30s
    Multiplier:   2.0,                  // Double delay each time
    Jitter:       true,                 // Add randomness (±10%)
}
```

### Circuit Breaker Configuration

```go
cbConfig := shared.CircuitBreakerConfig{
    MaxFailures:         5,              // Open after 5 failures
    Timeout:             60 * time.Second, // Try recovery after 60s
    HalfOpenMaxRequests: 3,              // Allow 3 test requests
}
```

## Error Handling

### Connection Errors (Automatically Queued)

The following errors trigger automatic queuing:
- `connection refused`
- `connection reset`
- `connection timeout`
- `no connection`
- `database is closed`
- `driver: bad connection`
- `broken pipe`
- `EOF`
- `i/o timeout`

### Non-Connection Errors (Returned Immediately)

These errors are returned without queuing:
- Constraint violations
- Invalid data
- Permission errors
- Query syntax errors

## Monitoring and Observability

### Queue Metrics

```go
metrics := queue.GetQueueMetrics()
```

Returns:
- `queue_size`: Current number of items in queue
- `max_queue_size`: Maximum queue capacity
- `total_entries`: Total price history entries queued
- `max_attempts`: Highest retry count among queued items
- `circuit_breaker`: Current circuit breaker state
- `oldest_item_age`: Age of oldest queued item

### Circuit Breaker Metrics

```go
cbMetrics := queue.circuitBreaker.GetMetrics()
```

Returns:
- `name`: Circuit breaker identifier
- `state`: Current state (CLOSED/OPEN/HALF_OPEN)
- `failure_count`: Consecutive failures
- `success_count`: Consecutive successes
- `last_failure_time`: Timestamp of last failure
- `last_state_change`: When state last changed

### Logging

All operations are logged with structured fields:

```
INFO  Data enqueued for later processing
      ipo_id=ABC123 company_code=ABC entry_count=10 queue_size=5

WARN  Failed to process queued data, will retry
      ipo_id=ABC123 attempts=2 error="connection refused"

INFO  Successfully processed queued data
      ipo_id=ABC123 attempts=3 queued_time=2m30s

ERROR Queue is full, dropping data
      queue_size=1000 max_queue_size=1000 ipo_id=ABC123
```

## Best Practices

### 1. Always Close Service

```go
service := NewGMPHistoryService(db)
defer service.Close() // Stops background workers
```

### 2. Monitor Queue Size

```go
// Alert if queue grows too large
if service.GetResilienceQueueSize() > 100 {
    log.Warn("Queue size exceeding threshold")
}
```

### 3. Check Circuit Breaker State

```go
metrics := service.GetResilienceQueueMetrics()
if metrics["circuit_breaker"] == "OPEN" {
    log.Error("Database circuit breaker is open")
}
```

### 4. Handle Queue Full Errors

```go
err := service.SavePriceHistory(collection)
if err != nil && strings.Contains(err.Error(), "queue is full") {
    // Queue is full - database has been down too long
    // Consider alerting operations team
    alertOps("Database resilience queue is full")
}
```

## Performance Considerations

### Memory Usage

- Each queued item holds full `GMPPriceHistoryCollection` in memory
- Default max queue size: 1000 items
- Typical item size: ~10-50 KB
- Maximum memory usage: ~50 MB (1000 items × 50 KB)

### Processing Overhead

- Background worker runs every 30 seconds
- Processing time: ~100-500ms per item (with retries)
- Minimal impact on main application performance

### Database Load

- Retry logic uses exponential backoff to reduce load
- Circuit breaker prevents overwhelming recovering database
- Transactions ensure atomic operations

## Testing

### Unit Tests

```bash
go test -v ./services -run TestDBResilience
go test -v ./services -run TestEnqueue
go test -v ./services -run TestCircuitBreaker
```

### Integration Tests

```bash
go test -v ./services -run TestResilienceQueueWorkflow
go test -v ./services -run TestGMPHistoryServiceWithResilience
```

### Load Testing

```bash
go test -v ./services -run TestQueueMetricsUnderLoad
```

## Troubleshooting

### Queue Growing Continuously

**Symptom**: Queue size keeps increasing
**Cause**: Database connection not recovering
**Solution**: 
1. Check database connectivity
2. Verify database credentials
3. Check network connectivity
4. Review database logs

### Circuit Breaker Stuck Open

**Symptom**: Circuit breaker state remains OPEN
**Cause**: Database still unavailable after timeout
**Solution**:
1. Verify database is running
2. Check connection pool settings
3. Manually reset circuit breaker if needed

### Items Dropped from Queue

**Symptom**: Log messages about dropping data
**Cause**: Max retries exceeded or queue full
**Solution**:
1. Increase `maxRetries` if transient issues
2. Increase `maxQueueSize` if needed
3. Investigate root cause of database failures

## Requirements Validation

This implementation satisfies **Requirement 6.5**:

✅ **Temporary data queuing**: Data is queued when database connection fails
✅ **Retry logic**: Exponential backoff retry with 3 attempts
✅ **Transaction management**: All operations wrapped in transactions
✅ **Rollback handling**: Automatic rollback on transaction failures
✅ **Circuit breaker**: Prevents cascading failures during outages
✅ **Background processing**: Automatic retry of queued items
✅ **Monitoring**: Comprehensive metrics and logging

## Related Documentation

- [Retry Logic](../shared/retry.go)
- [Circuit Breaker](../shared/circuit_breaker.go)
- [GMP History Service](./gmp_history_service.go)
- [Database Schema](../database/schema.sql)
