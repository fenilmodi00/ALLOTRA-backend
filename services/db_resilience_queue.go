package services

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/sirupsen/logrus"
)

// QueuedHistoryData represents data waiting to be persisted to the database
type QueuedHistoryData struct {
	Collection *models.GMPPriceHistoryCollection
	QueuedAt   time.Time
	Attempts   int
	LastError  error
}

// DBResilienceQueue manages temporary queuing of data during database failures
// Implements Requirement 6.5 - Queue scraped data temporarily during database failures
type DBResilienceQueue struct {
	queue           []*QueuedHistoryData
	mutex           sync.RWMutex
	maxQueueSize    int
	maxRetries      int
	retryInterval   time.Duration
	logger          *logrus.Logger
	db              *sql.DB
	retryConfig     shared.RetryConfig
	circuitBreaker  *shared.CircuitBreaker
	stopChan        chan struct{}
	wg              sync.WaitGroup
	saveWithRetryFn func(collection *models.GMPPriceHistoryCollection) error
}

func roundToTwoDecimals(value float64) float64 {
	return math.Round(value*100) / 100
}

// NewDBResilienceQueue creates a new database resilience queue
func NewDBResilienceQueue(db *sql.DB, logger *logrus.Logger) *DBResilienceQueue {
	if logger == nil {
		logger = logrus.New()
	}

	// Configure retry with exponential backoff
	retryConfig := shared.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 2 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}

	// Configure circuit breaker for database operations
	cbConfig := shared.CircuitBreakerConfig{
		MaxFailures:         5,
		Timeout:             60 * time.Second,
		HalfOpenMaxRequests: 3,
	}

	queue := &DBResilienceQueue{
		queue:          make([]*QueuedHistoryData, 0),
		maxQueueSize:   1000, // Maximum items to queue before dropping
		maxRetries:     3,
		retryInterval:  30 * time.Second,
		logger:         logger,
		db:             db,
		retryConfig:    retryConfig,
		circuitBreaker: shared.NewCircuitBreaker("db-resilience", cbConfig),
		stopChan:       make(chan struct{}),
	}

	queue.saveWithRetryFn = queue.saveWithRetry

	return queue
}

// Start begins the background worker for processing queued data
func (q *DBResilienceQueue) Start() {
	q.wg.Add(1)
	go q.processQueueWorker()
	q.logger.Info("Database resilience queue worker started")
}

// Stop gracefully stops the background worker
func (q *DBResilienceQueue) Stop() {
	close(q.stopChan)
	q.wg.Wait()
	q.logger.Info("Database resilience queue worker stopped")
}

// Enqueue adds data to the queue for later processing
// Implements Requirement 6.5 - Temporary data queuing
func (q *DBResilienceQueue) Enqueue(collection *models.GMPPriceHistoryCollection) error {
	if collection == nil {
		return fmt.Errorf("collection is nil")
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Check queue size limit
	if len(q.queue) >= q.maxQueueSize {
		q.logger.WithFields(logrus.Fields{
			"queue_size":     len(q.queue),
			"max_queue_size": q.maxQueueSize,
			"ipo_id":         collection.IPOID,
		}).Error("Queue is full, dropping data")
		return fmt.Errorf("queue is full (size: %d), cannot enqueue more data", len(q.queue))
	}

	// Add to queue
	queuedData := &QueuedHistoryData{
		Collection: collection,
		QueuedAt:   time.Now(),
		Attempts:   0,
		LastError:  nil,
	}

	q.queue = append(q.queue, queuedData)

	q.logger.WithFields(logrus.Fields{
		"ipo_id":       collection.IPOID,
		"company_code": collection.CompanyCode,
		"entry_count":  len(collection.Entries),
		"queue_size":   len(q.queue),
	}).Info("Data enqueued for later processing")

	return nil
}

// GetQueueSize returns the current size of the queue
func (q *DBResilienceQueue) GetQueueSize() int {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	return len(q.queue)
}

// GetQueueMetrics returns metrics about the queue
func (q *DBResilienceQueue) GetQueueMetrics() map[string]interface{} {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	totalEntries := 0
	oldestQueueTime := time.Time{}
	maxAttempts := 0

	for _, item := range q.queue {
		totalEntries += len(item.Collection.Entries)
		if oldestQueueTime.IsZero() || item.QueuedAt.Before(oldestQueueTime) {
			oldestQueueTime = item.QueuedAt
		}
		if item.Attempts > maxAttempts {
			maxAttempts = item.Attempts
		}
	}

	metrics := map[string]interface{}{
		"queue_size":      len(q.queue),
		"max_queue_size":  q.maxQueueSize,
		"total_entries":   totalEntries,
		"max_attempts":    maxAttempts,
		"circuit_breaker": q.circuitBreaker.GetState().String(),
	}

	if !oldestQueueTime.IsZero() {
		metrics["oldest_item_age"] = time.Since(oldestQueueTime).String()
	}

	return metrics
}

// processQueueWorker is a background worker that processes queued data
func (q *DBResilienceQueue) processQueueWorker() {
	defer q.wg.Done()

	ticker := time.NewTicker(q.retryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.stopChan:
			q.logger.Info("Queue worker received stop signal")
			return

		case <-ticker.C:
			q.processQueue()
		}
	}
}

// processQueue attempts to process all queued data
func (q *DBResilienceQueue) processQueue() {
	q.mutex.Lock()
	if len(q.queue) == 0 {
		q.mutex.Unlock()
		return
	}

	// Get a copy of the queue to process and clear the shared queue.
	queueCopy := make([]*QueuedHistoryData, len(q.queue))
	copy(queueCopy, q.queue)
	q.queue = nil
	q.mutex.Unlock()

	q.logger.WithField("queue_size", len(queueCopy)).Info("Processing queued data")

	successCount := 0
	failureCount := 0
	var remainingQueue []*QueuedHistoryData

	for _, item := range queueCopy {
		// Check if max retries exceeded
		if item.Attempts >= q.maxRetries {
			q.logger.WithFields(logrus.Fields{
				"ipo_id":      item.Collection.IPOID,
				"attempts":    item.Attempts,
				"max_retries": q.maxRetries,
				"queued_at":   item.QueuedAt,
				"last_error":  item.LastError,
			}).Error("Max retries exceeded, dropping queued data")
			failureCount++
			continue
		}

		// Attempt to save with retry logic
		err := error(nil)
		if q.saveWithRetryFn != nil {
			err = q.saveWithRetryFn(item.Collection)
		} else {
			err = q.saveWithRetry(item.Collection)
		}
		item.Attempts++

		if err != nil {
			item.LastError = err
			remainingQueue = append(remainingQueue, item)
			failureCount++

			q.logger.WithFields(logrus.Fields{
				"ipo_id":   item.Collection.IPOID,
				"attempts": item.Attempts,
				"error":    err.Error(),
			}).Warn("Failed to process queued data, will retry")
		} else {
			successCount++
			q.logger.WithFields(logrus.Fields{
				"ipo_id":      item.Collection.IPOID,
				"attempts":    item.Attempts,
				"queued_time": time.Since(item.QueuedAt).String(),
			}).Info("Successfully processed queued data")
		}
	}

	// Merge any newly enqueued items that arrived while processing.
	q.mutex.Lock()
	if len(remainingQueue) > 0 {
		q.queue = append(remainingQueue, q.queue...)
	}
	q.mutex.Unlock()

	q.logger.WithFields(logrus.Fields{
		"processed":       len(queueCopy),
		"success_count":   successCount,
		"failure_count":   failureCount,
		"remaining_queue": len(remainingQueue),
	}).Info("Queue processing completed")
}

// saveWithRetry attempts to save data with retry logic and circuit breaker protection
// Implements Requirement 6.5 - Retry storage operations
func (q *DBResilienceQueue) saveWithRetry(collection *models.GMPPriceHistoryCollection) error {
	// Use circuit breaker to protect database
	err := q.circuitBreaker.Execute(func() error {
		// Use retry logic for the actual save operation
		return shared.RetryWithExponentialBackoff(func() error {
			return q.saveToDatabase(collection)
		}, q.retryConfig, q.logger)
	})

	if err != nil {
		if err == shared.ErrCircuitOpen {
			q.logger.WithField("ipo_id", collection.IPOID).Warn("Circuit breaker is open, skipping save attempt")
		}
		return err
	}

	return nil
}

// saveToDatabase performs the actual database save operation with transaction management
// Implements Requirement 6.5 - Transaction management and rollback handling
func (q *DBResilienceQueue) saveToDatabase(collection *models.GMPPriceHistoryCollection) error {
	if collection == nil || len(collection.Entries) == 0 {
		return nil
	}

	// Start transaction for atomic operation
	ctx := context.Background()
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure rollback on error
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				q.logger.WithError(rbErr).Error("Failed to rollback transaction")
			}
		}
	}()

	// Upsert query with ON CONFLICT clause
	upsertQuery := `
		INSERT INTO gmp_price_history (
			id, ipo_id, company_code, record_date, ipo_price, gmp_value,
			estimated_listing, listing_percent, estimated_profit,
			subscription_status, sub2_sauda, last_updated, data_source,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
		ON CONFLICT (ipo_id, record_date)
		DO UPDATE SET
			ipo_price = EXCLUDED.ipo_price,
			gmp_value = EXCLUDED.gmp_value,
			estimated_listing = EXCLUDED.estimated_listing,
			listing_percent = EXCLUDED.listing_percent,
			estimated_profit = EXCLUDED.estimated_profit,
			subscription_status = EXCLUDED.subscription_status,
			sub2_sauda = EXCLUDED.sub2_sauda,
			last_updated = EXCLUDED.last_updated,
			data_source = EXCLUDED.data_source,
			updated_at = EXCLUDED.updated_at
	`

	stmt, err := tx.PrepareContext(ctx, upsertQuery)
	if err != nil {
		return fmt.Errorf("failed to prepare upsert statement: %w", err)
	}
	defer stmt.Close()

	successCount := 0
	for _, entry := range collection.Entries {
		// Set timestamps
		now := time.Now()
		if entry.CreatedAt.IsZero() {
			entry.CreatedAt = now
		}
		entry.UpdatedAt = now

		// Normalize derived values before persistence so DB constraints remain valid.
		entry.IPOPrice = roundToTwoDecimals(entry.IPOPrice)
		entry.GMPValue = roundToTwoDecimals(entry.GMPValue)
		entry.EstimatedListing = roundToTwoDecimals(entry.IPOPrice + entry.GMPValue)
		if entry.IPOPrice > 0 {
			entry.ListingPercent = roundToTwoDecimals((entry.GMPValue / entry.IPOPrice) * 100)
		} else {
			entry.ListingPercent = 0
		}
		entry.EstimatedProfit = roundToTwoDecimals(entry.EstimatedProfit)
		entry.Sub2Sauda = roundToTwoDecimals(entry.Sub2Sauda)

		// Set IPO ID and company code from collection
		entry.IPOID = collection.IPOID
		entry.CompanyCode = collection.CompanyCode

		// Execute upsert
		_, err = stmt.ExecContext(ctx,
			entry.ID,
			entry.IPOID,
			entry.CompanyCode,
			entry.RecordDate,
			entry.IPOPrice,
			entry.GMPValue,
			entry.EstimatedListing,
			entry.ListingPercent,
			entry.EstimatedProfit,
			entry.SubscriptionStatus,
			entry.Sub2Sauda,
			entry.LastUpdated,
			entry.DataSource,
			entry.CreatedAt,
			entry.UpdatedAt,
		)

		if err != nil {
			return fmt.Errorf("failed to upsert entry for date %s: %w", entry.RecordDate.Format("2006-01-02"), err)
		}

		successCount++
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	q.logger.WithFields(logrus.Fields{
		"ipo_id":        collection.IPOID,
		"success_count": successCount,
	}).Debug("Database save completed successfully")

	return nil
}

// SaveWithResilience attempts to save data to the database with automatic queuing on failure
// This is the main entry point for resilient database operations
// Implements Requirement 6.5 - Queue data temporarily on database connection failures
func (q *DBResilienceQueue) SaveWithResilience(collection *models.GMPPriceHistoryCollection) error {
	// First, try to save directly with retry logic
	err := q.saveWithRetry(collection)

	if err != nil {
		// If save fails, check if it's a database connection issue
		if isDatabaseConnectionError(err) {
			q.logger.WithFields(logrus.Fields{
				"ipo_id": collection.IPOID,
				"error":  err.Error(),
			}).Warn("Database connection failure detected, enqueueing data")

			// Queue the data for later processing
			if enqueueErr := q.Enqueue(collection); enqueueErr != nil {
				// If we can't even queue it, return both errors
				return fmt.Errorf("failed to save and failed to enqueue: save error: %w, enqueue error: %v", err, enqueueErr)
			}

			// Data is queued, return nil to indicate graceful handling
			return nil
		}

		// For non-connection errors, return the error
		return err
	}

	return nil
}

// isDatabaseConnectionError checks if an error is related to database connectivity
func isDatabaseConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common database connection error patterns
	errorStr := strings.ToLower(err.Error())
	connectionErrors := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"no connection",
		"database is closed",
		"driver: bad connection",
		"broken pipe",
		"eof",
		"i/o timeout",
	}

	for _, connErr := range connectionErrors {
		if strings.Contains(errorStr, connErr) {
			return true
		}
	}

	return false
}
