package services

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/sirupsen/logrus"
)

// TestNewDBResilienceQueue tests queue initialization
func TestNewDBResilienceQueue(t *testing.T) {
	// Create a mock database connection (nil is acceptable for this test)
	var db *sql.DB
	logger := logrus.New()

	queue := NewDBResilienceQueue(db, logger)

	if queue == nil {
		t.Fatal("Expected queue to be created, got nil")
	}

	if queue.maxQueueSize != 1000 {
		t.Errorf("Expected maxQueueSize to be 1000, got %d", queue.maxQueueSize)
	}

	if queue.maxRetries != 3 {
		t.Errorf("Expected maxRetries to be 3, got %d", queue.maxRetries)
	}

	if queue.retryInterval != 30*time.Second {
		t.Errorf("Expected retryInterval to be 30s, got %v", queue.retryInterval)
	}

	if queue.circuitBreaker == nil {
		t.Error("Expected circuit breaker to be initialized")
	}
}

// TestEnqueueSuccess tests successful enqueueing of data
func TestEnqueueSuccess(t *testing.T) {
	var db *sql.DB
	logger := logrus.New()
	queue := NewDBResilienceQueue(db, logger)

	collection := &models.GMPPriceHistoryCollection{
		IPOID:       "test-ipo-id",
		CompanyCode: "TEST",
		Entries: []models.GMPPriceHistoryEntry{
			{
				ID:         "entry-1",
				RecordDate: time.Now(),
				GMPValue:   100.0,
			},
		},
	}

	err := queue.Enqueue(collection)
	if err != nil {
		t.Fatalf("Expected successful enqueue, got error: %v", err)
	}

	queueSize := queue.GetQueueSize()
	if queueSize != 1 {
		t.Errorf("Expected queue size to be 1, got %d", queueSize)
	}
}

// TestEnqueueMultiple tests enqueueing multiple items
func TestEnqueueMultiple(t *testing.T) {
	var db *sql.DB
	logger := logrus.New()
	queue := NewDBResilienceQueue(db, logger)

	// Enqueue 5 items
	for i := 0; i < 5; i++ {
		collection := &models.GMPPriceHistoryCollection{
			IPOID:       "test-ipo-" + string(rune(i)),
			CompanyCode: "TEST",
			Entries: []models.GMPPriceHistoryEntry{
				{
					ID:         "entry-" + string(rune(i)),
					RecordDate: time.Now(),
					GMPValue:   100.0,
				},
			},
		}

		err := queue.Enqueue(collection)
		if err != nil {
			t.Fatalf("Expected successful enqueue for item %d, got error: %v", i, err)
		}
	}

	queueSize := queue.GetQueueSize()
	if queueSize != 5 {
		t.Errorf("Expected queue size to be 5, got %d", queueSize)
	}
}

// TestEnqueueQueueFull tests behavior when queue is full
func TestEnqueueQueueFull(t *testing.T) {
	var db *sql.DB
	logger := logrus.New()
	queue := NewDBResilienceQueue(db, logger)

	// Set a small max queue size for testing
	queue.maxQueueSize = 2

	// Enqueue 2 items (should succeed)
	for i := 0; i < 2; i++ {
		collection := &models.GMPPriceHistoryCollection{
			IPOID:       "test-ipo-" + string(rune(i)),
			CompanyCode: "TEST",
			Entries:     []models.GMPPriceHistoryEntry{{ID: "entry-" + string(rune(i))}},
		}

		err := queue.Enqueue(collection)
		if err != nil {
			t.Fatalf("Expected successful enqueue for item %d, got error: %v", i, err)
		}
	}

	// Try to enqueue a 3rd item (should fail)
	collection := &models.GMPPriceHistoryCollection{
		IPOID:       "test-ipo-overflow",
		CompanyCode: "TEST",
		Entries:     []models.GMPPriceHistoryEntry{{ID: "entry-overflow"}},
	}

	err := queue.Enqueue(collection)
	if err == nil {
		t.Error("Expected error when queue is full, got nil")
	}

	queueSize := queue.GetQueueSize()
	if queueSize != 2 {
		t.Errorf("Expected queue size to remain 2, got %d", queueSize)
	}
}

// TestGetQueueMetrics tests queue metrics retrieval
func TestGetQueueMetrics(t *testing.T) {
	var db *sql.DB
	logger := logrus.New()
	queue := NewDBResilienceQueue(db, logger)

	// Enqueue some items
	for i := 0; i < 3; i++ {
		collection := &models.GMPPriceHistoryCollection{
			IPOID:       "test-ipo-" + string(rune(i)),
			CompanyCode: "TEST",
			Entries: []models.GMPPriceHistoryEntry{
				{ID: "entry-1"},
				{ID: "entry-2"},
			},
		}

		err := queue.Enqueue(collection)
		if err != nil {
			t.Fatalf("Failed to enqueue item %d: %v", i, err)
		}
	}

	metrics := queue.GetQueueMetrics()

	if metrics["queue_size"] != 3 {
		t.Errorf("Expected queue_size to be 3, got %v", metrics["queue_size"])
	}

	if metrics["max_queue_size"] != 1000 {
		t.Errorf("Expected max_queue_size to be 1000, got %v", metrics["max_queue_size"])
	}

	if metrics["total_entries"] != 6 {
		t.Errorf("Expected total_entries to be 6 (3 collections * 2 entries), got %v", metrics["total_entries"])
	}

	if metrics["circuit_breaker"] == nil {
		t.Error("Expected circuit_breaker state in metrics")
	}

	if metrics["oldest_item_age"] == nil {
		t.Error("Expected oldest_item_age in metrics when queue has items")
	}
}

// TestIsDatabaseConnectionError tests connection error detection
func TestIsDatabaseConnectionError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      errors.New("connection reset by peer"),
			expected: true,
		},
		{
			name:     "connection timeout",
			err:      errors.New("connection timeout"),
			expected: true,
		},
		{
			name:     "database is closed",
			err:      errors.New("sql: database is closed"),
			expected: true,
		},
		{
			name:     "bad connection",
			err:      errors.New("driver: bad connection"),
			expected: true,
		},
		{
			name:     "broken pipe",
			err:      errors.New("write: broken pipe"),
			expected: true,
		},
		{
			name:     "EOF error",
			err:      errors.New("EOF"),
			expected: true,
		},
		{
			name:     "i/o timeout",
			err:      errors.New("i/o timeout"),
			expected: true,
		},
		{
			name:     "case insensitive - Connection Refused",
			err:      errors.New("Connection Refused"),
			expected: true,
		},
		{
			name:     "non-connection error",
			err:      errors.New("invalid syntax"),
			expected: false,
		},
		{
			name:     "constraint violation",
			err:      errors.New("unique constraint violation"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isDatabaseConnectionError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected %v for error '%v', got %v", tc.expected, tc.err, result)
			}
		})
	}
}

// TestQueueStartStop tests starting and stopping the queue worker
func TestQueueStartStop(t *testing.T) {
	var db *sql.DB
	logger := logrus.New()
	queue := NewDBResilienceQueue(db, logger)

	// Start the worker
	queue.Start()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the worker
	queue.Stop()

	// Verify it stopped gracefully (no panic)
	t.Log("Queue worker started and stopped successfully")
}

// TestEnqueueNilCollection tests handling of nil collection
func TestEnqueueNilCollection(t *testing.T) {
	var db *sql.DB
	logger := logrus.New()
	queue := NewDBResilienceQueue(db, logger)

	// Attempt to enqueue nil - should return an error
	err := queue.Enqueue(nil)
	if err == nil {
		t.Error("Expected error when enqueueing nil collection, got nil")
	}

	// Queue size should remain 0
	if queue.GetQueueSize() != 0 {
		t.Errorf("Expected queue size to be 0 after nil enqueue, got %d", queue.GetQueueSize())
	}
}

// TestQueueMetricsEmptyQueue tests metrics when queue is empty
func TestQueueMetricsEmptyQueue(t *testing.T) {
	var db *sql.DB
	logger := logrus.New()
	queue := NewDBResilienceQueue(db, logger)

	metrics := queue.GetQueueMetrics()

	if metrics["queue_size"] != 0 {
		t.Errorf("Expected queue_size to be 0, got %v", metrics["queue_size"])
	}

	if metrics["total_entries"] != 0 {
		t.Errorf("Expected total_entries to be 0, got %v", metrics["total_entries"])
	}

	// oldest_item_age should not be present when queue is empty
	if _, exists := metrics["oldest_item_age"]; exists {
		t.Error("Expected oldest_item_age to not be present when queue is empty")
	}
}

// TestCircuitBreakerIntegration tests that circuit breaker is properly integrated
func TestCircuitBreakerIntegration(t *testing.T) {
	var db *sql.DB
	logger := logrus.New()
	queue := NewDBResilienceQueue(db, logger)

	if queue.circuitBreaker == nil {
		t.Fatal("Expected circuit breaker to be initialized")
	}

	// Check initial state
	state := queue.circuitBreaker.GetState()
	if state.String() != "CLOSED" {
		t.Errorf("Expected initial circuit breaker state to be CLOSED, got %s", state.String())
	}

	// Verify circuit breaker metrics are included in queue metrics
	metrics := queue.GetQueueMetrics()
	if metrics["circuit_breaker"] == nil {
		t.Error("Expected circuit_breaker in queue metrics")
	}
}

// TestRetryConfigDefaults tests that retry configuration has correct defaults
func TestRetryConfigDefaults(t *testing.T) {
	var db *sql.DB
	logger := logrus.New()
	queue := NewDBResilienceQueue(db, logger)

	if queue.retryConfig.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts to be 3, got %d", queue.retryConfig.MaxAttempts)
	}

	if queue.retryConfig.InitialDelay != 2*time.Second {
		t.Errorf("Expected InitialDelay to be 2s, got %v", queue.retryConfig.InitialDelay)
	}

	if queue.retryConfig.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay to be 30s, got %v", queue.retryConfig.MaxDelay)
	}

	if queue.retryConfig.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier to be 2.0, got %f", queue.retryConfig.Multiplier)
	}

	if !queue.retryConfig.Jitter {
		t.Error("Expected Jitter to be enabled")
	}
}

func TestProcessQueuePreservesNewEnqueue(t *testing.T) {
	var db *sql.DB
	logger := logrus.New()
	queue := NewDBResilienceQueue(db, logger)

	first := &models.GMPPriceHistoryCollection{
		IPOID:       "test-ipo-1",
		CompanyCode: "TEST",
		Entries: []models.GMPPriceHistoryEntry{
			{
				ID:         "entry-1",
				RecordDate: time.Now(),
				GMPValue:   100,
			},
		},
	}

	if err := queue.Enqueue(first); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	second := &models.GMPPriceHistoryCollection{
		IPOID:       "test-ipo-2",
		CompanyCode: "TEST",
		Entries: []models.GMPPriceHistoryEntry{
			{
				ID:         "entry-2",
				RecordDate: time.Now(),
				GMPValue:   200,
			},
		},
	}

	ready := make(chan struct{})

	queue.saveWithRetryFn = func(collection *models.GMPPriceHistoryCollection) error {
		if collection != nil && collection.IPOID == first.IPOID {
			close(ready)
			if err := queue.Enqueue(second); err != nil {
				return err
			}
		}
		return fmt.Errorf("simulated failure")
	}

	queue.processQueue()

	select {
	case <-ready:
	default:
		t.Fatal("expected saveWithRetry to be called for first item")
	}

	if queue.GetQueueSize() != 2 {
		t.Fatalf("expected queue size 2 after processing, got %d", queue.GetQueueSize())
	}
}
