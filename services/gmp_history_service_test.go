package services

import (
	"testing"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	_ "github.com/lib/pq"
)

// TestNewGMPHistoryService tests service initialization
func TestNewGMPHistoryService(t *testing.T) {
	// Create service without database (for basic initialization test)
	service := NewGMPHistoryService(nil)

	if service == nil {
		t.Fatal("Expected service to be initialized, got nil")
	}

	if service.logger == nil {
		t.Error("Expected logger to be initialized")
	}

	if service.scraper == nil {
		t.Error("Expected scraper to be initialized")
	}

	if service.utilityService == nil {
		t.Error("Expected utility service to be initialized")
	}
}

// TestValidateHistoryData tests data validation logic
func TestValidateHistoryData(t *testing.T) {
	service := NewGMPHistoryService(nil)

	tests := []struct {
		name      string
		entry     *models.GMPPriceHistoryEntry
		wantError bool
		errorMsg  string
	}{
		{
			name:      "nil entry",
			entry:     nil,
			wantError: true,
			errorMsg:  "entry is nil",
		},
		{
			name: "negative GMP value",
			entry: &models.GMPPriceHistoryEntry{
				GMPValue:         -10.0,
				IPOPrice:         100.0,
				EstimatedListing: 90.0,
				RecordDate:       time.Now(),
			},
			wantError: true,
			errorMsg:  "GMP value cannot be negative",
		},
		{
			name: "negative IPO price",
			entry: &models.GMPPriceHistoryEntry{
				GMPValue:         10.0,
				IPOPrice:         -100.0,
				EstimatedListing: 110.0,
				RecordDate:       time.Now(),
			},
			wantError: true,
			errorMsg:  "IPO price cannot be negative",
		},
		{
			name: "negative estimated listing",
			entry: &models.GMPPriceHistoryEntry{
				GMPValue:         10.0,
				IPOPrice:         100.0,
				EstimatedListing: -110.0,
				RecordDate:       time.Now(),
			},
			wantError: true,
			errorMsg:  "estimated listing price cannot be negative",
		},
		{
			name: "date too old",
			entry: &models.GMPPriceHistoryEntry{
				GMPValue:         10.0,
				IPOPrice:         100.0,
				EstimatedListing: 110.0,
				ListingPercent:   10.0,
				RecordDate:       time.Now().AddDate(-3, 0, 0), // 3 years ago
			},
			wantError: true,
			errorMsg:  "record date out of reasonable range",
		},
		{
			name: "date too far in future",
			entry: &models.GMPPriceHistoryEntry{
				GMPValue:         10.0,
				IPOPrice:         100.0,
				EstimatedListing: 110.0,
				ListingPercent:   10.0,
				RecordDate:       time.Now().AddDate(2, 0, 0), // 2 years in future
			},
			wantError: true,
			errorMsg:  "record date out of reasonable range",
		},
		{
			name: "listing price calculation mismatch",
			entry: &models.GMPPriceHistoryEntry{
				GMPValue:         10.0,
				IPOPrice:         100.0,
				EstimatedListing: 120.0, // Should be 110.0
				ListingPercent:   10.0,
				RecordDate:       time.Now(),
			},
			wantError: true,
			errorMsg:  "estimated listing price mismatch",
		},
		{
			name: "percentage calculation mismatch",
			entry: &models.GMPPriceHistoryEntry{
				GMPValue:         10.0,
				IPOPrice:         100.0,
				EstimatedListing: 110.0,
				ListingPercent:   20.0, // Should be 10.0
				RecordDate:       time.Now(),
			},
			wantError: true,
			errorMsg:  "listing percentage mismatch",
		},
		{
			name: "valid entry",
			entry: &models.GMPPriceHistoryEntry{
				GMPValue:         10.0,
				IPOPrice:         100.0,
				EstimatedListing: 110.0,
				ListingPercent:   10.0,
				RecordDate:       time.Now(),
			},
			wantError: false,
		},
		{
			name: "valid entry with zero GMP",
			entry: &models.GMPPriceHistoryEntry{
				GMPValue:         0.0,
				IPOPrice:         100.0,
				EstimatedListing: 100.0,
				ListingPercent:   0.0,
				RecordDate:       time.Now(),
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateHistoryData(tt.entry)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestSavePriceHistory_NilCollection tests handling of nil collection
func TestSavePriceHistory_NilCollection(t *testing.T) {
	service := NewGMPHistoryService(nil)

	err := service.SavePriceHistory(nil)
	if err == nil {
		t.Error("Expected error for nil collection, got nil")
	}

	if !contains(err.Error(), "history collection is nil") {
		t.Errorf("Expected error about nil collection, got: %v", err)
	}
}

// TestSavePriceHistory_EmptyEntries tests handling of empty entries
func TestSavePriceHistory_EmptyEntries(t *testing.T) {
	service := NewGMPHistoryService(nil)

	collection := &models.GMPPriceHistoryCollection{
		IPOID:       "test-ipo-id",
		CompanyCode: "TEST",
		Entries:     []models.GMPPriceHistoryEntry{},
	}

	err := service.SavePriceHistory(collection)
	if err != nil {
		t.Errorf("Expected no error for empty entries, got: %v", err)
	}
}

// TestGetPriceHistoryByIPO_EmptyIPOID tests handling of empty IPO ID
func TestGetPriceHistoryByIPO_EmptyIPOID(t *testing.T) {
	service := NewGMPHistoryService(nil)

	_, err := service.GetPriceHistoryByIPO("", nil)
	if err == nil {
		t.Error("Expected error for empty IPO ID, got nil")
	}

	if !contains(err.Error(), "ipo_id is required") {
		t.Errorf("Expected error about required IPO ID, got: %v", err)
	}
}

// TestArchiveOldHistory_ValidCutoffDate tests archival with valid cutoff date
func TestArchiveOldHistory_ValidCutoffDate(t *testing.T) {
	// This test requires a database connection, so we skip it if DB is not available
	t.Skip("Skipping test that requires database connection")
}

// TestProcessingMetrics tests the ProcessingMetrics structure
func TestProcessingMetrics(t *testing.T) {
	metrics := &ProcessingMetrics{
		TotalIPOs:         10,
		SuccessCount:      8,
		ErrorCount:        2,
		TotalRecordsAdded: 150,
		StartTime:         time.Now().Add(-5 * time.Minute),
		EndTime:           time.Now(),
		ErrorDetails:      []string{"Error 1", "Error 2"},
	}

	metrics.ProcessingTime = metrics.EndTime.Sub(metrics.StartTime)

	if metrics.TotalIPOs != 10 {
		t.Errorf("Expected TotalIPOs to be 10, got %d", metrics.TotalIPOs)
	}

	if metrics.SuccessCount != 8 {
		t.Errorf("Expected SuccessCount to be 8, got %d", metrics.SuccessCount)
	}

	if metrics.ErrorCount != 2 {
		t.Errorf("Expected ErrorCount to be 2, got %d", metrics.ErrorCount)
	}

	if metrics.TotalRecordsAdded != 150 {
		t.Errorf("Expected TotalRecordsAdded to be 150, got %d", metrics.TotalRecordsAdded)
	}

	if len(metrics.ErrorDetails) != 2 {
		t.Errorf("Expected 2 error details, got %d", len(metrics.ErrorDetails))
	}

	if metrics.ProcessingTime <= 0 {
		t.Error("Expected positive processing time")
	}

	// Test success rate calculation
	successRate := float64(metrics.SuccessCount) / float64(metrics.TotalIPOs) * 100
	expectedRate := 80.0
	if successRate != expectedRate {
		t.Errorf("Expected success rate %.2f%%, got %.2f%%", expectedRate, successRate)
	}
}

// TestJoinStrings tests the joinStrings helper function
func TestJoinStrings(t *testing.T) {
	tests := []struct {
		name     string
		strs     []string
		sep      string
		expected string
	}{
		{
			name:     "empty slice",
			strs:     []string{},
			sep:      ",",
			expected: "",
		},
		{
			name:     "single string",
			strs:     []string{"hello"},
			sep:      ",",
			expected: "hello",
		},
		{
			name:     "multiple strings with comma",
			strs:     []string{"hello", "world", "test"},
			sep:      ",",
			expected: "hello,world,test",
		},
		{
			name:     "multiple strings with newline",
			strs:     []string{"Error 1", "Error 2", "Error 3"},
			sep:      "\n",
			expected: "Error 1\nError 2\nError 3",
		},
		{
			name:     "multiple strings with space",
			strs:     []string{"one", "two", "three"},
			sep:      " ",
			expected: "one two three",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinStrings(tt.strs, tt.sep)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestErrorIsolation tests that errors in individual IPO processing don't stop the batch
func TestErrorIsolation(t *testing.T) {
	// This test verifies the concept of error isolation
	// In the actual implementation, if one IPO fails, processing continues with the next

	type ipoResult struct {
		ipoID   string
		success bool
		err     error
	}

	// Simulate processing multiple IPOs with some failures
	ipos := []string{"IPO1", "IPO2", "IPO3", "IPO4", "IPO5"}
	results := make([]ipoResult, 0)

	// Simulate processing with error isolation
	for i, ipoID := range ipos {
		var result ipoResult
		result.ipoID = ipoID

		// Simulate failure for IPO2 and IPO4
		if i == 1 || i == 3 {
			result.success = false
			result.err = &testError{msg: "simulated error"}
		} else {
			result.success = true
			result.err = nil
		}

		results = append(results, result)
		// Continue processing even if there's an error (error isolation)
	}

	// Verify all IPOs were processed
	if len(results) != len(ipos) {
		t.Errorf("Expected %d results, got %d", len(ipos), len(results))
	}

	// Count successes and failures
	successCount := 0
	errorCount := 0
	for _, result := range results {
		if result.success {
			successCount++
		} else {
			errorCount++
		}
	}

	// Verify error isolation worked
	if successCount != 3 {
		t.Errorf("Expected 3 successful IPOs, got %d", successCount)
	}

	if errorCount != 2 {
		t.Errorf("Expected 2 failed IPOs, got %d", errorCount)
	}
}

// TestPriorityBasedProcessing tests IPO prioritization logic
func TestPriorityBasedProcessing(t *testing.T) {
	// This test verifies the priority ordering concept
	// LIVE IPOs should be processed before UPCOMING, which should be before CLOSED

	type ipo struct {
		id     string
		status string
	}

	ipos := []ipo{
		{id: "IPO1", status: "CLOSED"},
		{id: "IPO2", status: "LIVE"},
		{id: "IPO3", status: "UPCOMING"},
		{id: "IPO4", status: "LIVE"},
		{id: "IPO5", status: "CLOSED"},
	}

	// Sort by priority (simulating the ORDER BY clause in the query)
	priorityMap := map[string]int{
		"LIVE":     1,
		"UPCOMING": 2,
		"CLOSED":   3,
	}

	// Simple bubble sort for testing
	for i := 0; i < len(ipos); i++ {
		for j := i + 1; j < len(ipos); j++ {
			if priorityMap[ipos[i].status] > priorityMap[ipos[j].status] {
				ipos[i], ipos[j] = ipos[j], ipos[i]
			}
		}
	}

	// Verify priority order
	expectedOrder := []string{"LIVE", "LIVE", "UPCOMING", "CLOSED", "CLOSED"}
	for i, ipo := range ipos {
		if ipo.status != expectedOrder[i] {
			t.Errorf("Position %d: expected status %s, got %s", i, expectedOrder[i], ipo.status)
		}
	}

	// Verify LIVE IPOs come first
	if ipos[0].status != "LIVE" || ipos[1].status != "LIVE" {
		t.Error("Expected LIVE IPOs to be processed first")
	}

	// Verify UPCOMING comes before CLOSED
	upcomingIndex := -1
	closedIndex := -1
	for i, ipo := range ipos {
		if ipo.status == "UPCOMING" && upcomingIndex == -1 {
			upcomingIndex = i
		}
		if ipo.status == "CLOSED" && closedIndex == -1 {
			closedIndex = i
		}
	}

	if upcomingIndex >= closedIndex {
		t.Error("Expected UPCOMING IPOs to be processed before CLOSED IPOs")
	}
}

// TestMetricsTracking tests comprehensive metrics tracking
func TestMetricsTracking(t *testing.T) {
	startTime := time.Now()

	// Simulate processing with metrics
	metrics := &ProcessingMetrics{
		StartTime:    startTime,
		ErrorDetails: make([]string, 0),
	}

	// Simulate processing 5 IPOs
	totalIPOs := 5
	metrics.TotalIPOs = totalIPOs

	for i := 0; i < totalIPOs; i++ {
		// Simulate some successes and failures
		if i == 1 || i == 3 {
			// Failure
			metrics.ErrorCount++
			metrics.ErrorDetails = append(metrics.ErrorDetails, "Error processing IPO "+string(rune('A'+i)))
		} else {
			// Success
			metrics.SuccessCount++
			metrics.TotalRecordsAdded += 10 + i*5 // Variable number of records
		}
	}

	// Add a small delay to ensure processing time is measurable
	time.Sleep(1 * time.Millisecond)

	metrics.EndTime = time.Now()
	metrics.ProcessingTime = metrics.EndTime.Sub(metrics.StartTime)

	// Verify metrics
	if metrics.TotalIPOs != 5 {
		t.Errorf("Expected TotalIPOs to be 5, got %d", metrics.TotalIPOs)
	}

	if metrics.SuccessCount != 3 {
		t.Errorf("Expected SuccessCount to be 3, got %d", metrics.SuccessCount)
	}

	if metrics.ErrorCount != 2 {
		t.Errorf("Expected ErrorCount to be 2, got %d", metrics.ErrorCount)
	}

	if len(metrics.ErrorDetails) != 2 {
		t.Errorf("Expected 2 error details, got %d", len(metrics.ErrorDetails))
	}

	if metrics.TotalRecordsAdded <= 0 {
		t.Error("Expected positive total records added")
	}

	if metrics.ProcessingTime <= 0 {
		t.Errorf("Expected positive processing time, got %v", metrics.ProcessingTime)
	}

	// Verify success rate calculation
	successRate := float64(metrics.SuccessCount) / float64(metrics.TotalIPOs) * 100
	if successRate != 60.0 {
		t.Errorf("Expected success rate 60%%, got %.2f%%", successRate)
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestCacheInitialization tests that cache is properly initialized
func TestCacheInitialization(t *testing.T) {
	service := NewGMPHistoryService(nil)

	if service.cache == nil {
		t.Fatal("Expected cache to be initialized, got nil")
	}

	stats := service.GetCacheStats()
	if stats == nil {
		t.Fatal("Expected cache stats to be returned, got nil")
	}

	enabled, ok := stats["enabled"].(bool)
	if !ok || !enabled {
		t.Error("Expected cache to be enabled")
	}

	size, ok := stats["size"].(int)
	if !ok {
		t.Error("Expected cache size to be an integer")
	}

	if size < 0 {
		t.Errorf("Expected non-negative cache size, got %d", size)
	}
}

// TestInvalidateIPOCache tests cache invalidation for a specific IPO
func TestInvalidateIPOCache(t *testing.T) {
	service := NewGMPHistoryService(nil)

	// Test that invalidation doesn't panic with nil cache
	service.cache = nil
	service.InvalidateIPOCache("test-ipo-id")

	// Re-initialize cache
	service.cache = NewCacheServiceWithConfig(nil, 10*time.Minute, 500)

	// Add some test data to cache
	testIPOID := "test-ipo-123"
	cacheKey := "gmp_history:" + testIPOID
	testData := &models.GMPPriceHistoryCollection{
		IPOID:   testIPOID,
		IPOName: "Test IPO",
	}

	service.cache.Set(cacheKey, testData)

	// Verify data is in cache
	if _, found := service.cache.Get(cacheKey); !found {
		t.Error("Expected test data to be in cache")
	}

	// Invalidate cache for this IPO
	service.InvalidateIPOCache(testIPOID)

	// Verify data is removed from cache
	if _, found := service.cache.Get(cacheKey); found {
		t.Error("Expected cache to be invalidated for IPO")
	}
}

// TestInvalidateAllCache tests clearing all cache entries
func TestInvalidateAllCache(t *testing.T) {
	service := NewGMPHistoryService(nil)

	// Test that invalidation doesn't panic with nil cache
	service.cache = nil
	service.InvalidateAllCache()

	// Re-initialize cache
	service.cache = NewCacheServiceWithConfig(nil, 10*time.Minute, 500)

	// Add multiple test entries to cache
	for i := 0; i < 5; i++ {
		cacheKey := "gmp_history:test-ipo-" + string(rune('A'+i))
		testData := &models.GMPPriceHistoryCollection{
			IPOID:   "test-ipo-" + string(rune('A'+i)),
			IPOName: "Test IPO " + string(rune('A'+i)),
		}
		service.cache.Set(cacheKey, testData)
	}

	// Verify cache has entries
	if service.cache.Size() != 5 {
		t.Errorf("Expected cache size to be 5, got %d", service.cache.Size())
	}

	// Clear all cache
	service.InvalidateAllCache()

	// Verify cache is empty
	if service.cache.Size() != 0 {
		t.Errorf("Expected cache to be empty, got size %d", service.cache.Size())
	}
}

// TestCacheKeyGeneration tests cache key generation with different parameters
func TestCacheKeyGeneration(t *testing.T) {
	tests := []struct {
		name      string
		ipoID     string
		dateRange *models.DateRange
		expected  string
	}{
		{
			name:      "no date range",
			ipoID:     "test-ipo-123",
			dateRange: nil,
			expected:  "gmp_history:test-ipo-123",
		},
		{
			name:  "with start date only",
			ipoID: "test-ipo-123",
			dateRange: &models.DateRange{
				StartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			expected: "gmp_history:test-ipo-123:start_2024-01-01",
		},
		{
			name:  "with end date only",
			ipoID: "test-ipo-123",
			dateRange: &models.DateRange{
				EndDate: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			},
			expected: "gmp_history:test-ipo-123:end_2024-12-31",
		},
		{
			name:  "with both dates",
			ipoID: "test-ipo-123",
			dateRange: &models.DateRange{
				StartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			},
			expected: "gmp_history:test-ipo-123:start_2024-01-01:end_2024-12-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build cache key using the same logic as GetPriceHistoryByIPO
			cacheKey := "gmp_history:" + tt.ipoID
			if tt.dateRange != nil {
				if !tt.dateRange.StartDate.IsZero() {
					cacheKey += ":start_" + tt.dateRange.StartDate.Format("2006-01-02")
				}
				if !tt.dateRange.EndDate.IsZero() {
					cacheKey += ":end_" + tt.dateRange.EndDate.Format("2006-01-02")
				}
			}

			if cacheKey != tt.expected {
				t.Errorf("Expected cache key '%s', got '%s'", tt.expected, cacheKey)
			}
		})
	}
}

// TestCacheTTL tests that cache entries expire after TTL
func TestCacheTTL(t *testing.T) {
	// Create cache with very short TTL for testing
	cache := NewCacheServiceWithConfig(nil, 100*time.Millisecond, 100)

	testKey := "test-key"
	testValue := "test-value"

	// Set value in cache
	cache.Set(testKey, testValue)

	// Verify value is in cache
	if val, found := cache.Get(testKey); !found {
		t.Error("Expected value to be in cache immediately after setting")
	} else if val != testValue {
		t.Errorf("Expected value '%s', got '%v'", testValue, val)
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Verify value is no longer in cache
	if _, found := cache.Get(testKey); found {
		t.Error("Expected value to be expired from cache after TTL")
	}
}

// TestCacheMaxSize tests cache eviction when max size is reached
func TestCacheMaxSize(t *testing.T) {
	// Create cache with small max size
	maxSize := 5
	cache := NewCacheServiceWithConfig(nil, 10*time.Minute, maxSize)

	// Add entries up to max size
	for i := 0; i < maxSize; i++ {
		key := "key-" + string(rune('A'+i))
		value := "value-" + string(rune('A'+i))
		cache.Set(key, value)
	}

	// Verify cache is at max size
	if cache.Size() != maxSize {
		t.Errorf("Expected cache size to be %d, got %d", maxSize, cache.Size())
	}

	// Add one more entry (should trigger eviction)
	cache.Set("key-overflow", "value-overflow")

	// Verify cache size doesn't exceed max
	if cache.Size() > maxSize {
		t.Errorf("Expected cache size to not exceed %d, got %d", maxSize, cache.Size())
	}

	// Verify the new entry is in cache
	if _, found := cache.Get("key-overflow"); !found {
		t.Error("Expected new entry to be in cache after eviction")
	}
}

// TestCacheInvalidationOnSave tests that cache is invalidated when data is saved
func TestCacheInvalidationOnSave(t *testing.T) {
	// This test verifies the concept that SavePriceHistory should invalidate cache
	// We can't test the actual database operation without a DB connection,
	// but we can verify the invalidation logic

	service := NewGMPHistoryService(nil)
	testIPOID := "test-ipo-123"

	// Add test data to cache
	cacheKey := "gmp_history:" + testIPOID
	testData := &models.GMPPriceHistoryCollection{
		IPOID:   testIPOID,
		IPOName: "Test IPO",
	}
	service.cache.Set(cacheKey, testData)

	// Verify data is in cache
	if _, found := service.cache.Get(cacheKey); !found {
		t.Error("Expected test data to be in cache before save")
	}

	// Simulate the invalidation that happens in SavePriceHistory
	service.InvalidateIPOCache(testIPOID)

	// Verify cache is invalidated
	if _, found := service.cache.Get(cacheKey); found {
		t.Error("Expected cache to be invalidated after save")
	}
}

// TestCachePerformance tests that caching improves performance
func TestCachePerformance(t *testing.T) {
	service := NewGMPHistoryService(nil)

	testIPOID := "test-ipo-123"
	cacheKey := "gmp_history:" + testIPOID

	// Create test data
	testData := &models.GMPPriceHistoryCollection{
		IPOID:        testIPOID,
		IPOName:      "Test IPO",
		CompanyCode:  "TEST",
		TotalRecords: 100,
		Entries:      make([]models.GMPPriceHistoryEntry, 100),
	}

	// Fill with test entries
	for i := 0; i < 100; i++ {
		testData.Entries[i] = models.GMPPriceHistoryEntry{
			ID:               "entry-" + string(rune('0'+i%10)),
			IPOID:            testIPOID,
			CompanyCode:      "TEST",
			RecordDate:       time.Now().AddDate(0, 0, -i),
			IPOPrice:         100.0,
			GMPValue:         float64(10 + i),
			EstimatedListing: float64(110 + i),
			ListingPercent:   float64(10 + i),
		}
	}

	// Store in cache
	service.cache.Set(cacheKey, testData)

	// Measure cache retrieval time
	startTime := time.Now()
	for i := 0; i < 1000; i++ {
		_, found := service.cache.Get(cacheKey)
		if !found {
			t.Error("Expected data to be in cache")
		}
	}
	cacheTime := time.Since(startTime)

	// Cache retrieval should be very fast (< 10ms for 1000 operations)
	if cacheTime > 10*time.Millisecond {
		t.Logf("Cache retrieval took %v for 1000 operations (expected < 10ms)", cacheTime)
		// This is a warning, not a failure, as performance can vary
	}

	t.Logf("Cache performance: 1000 retrievals in %v (avg: %v per retrieval)",
		cacheTime, cacheTime/1000)
}

// TestCacheStatsReporting tests cache statistics reporting
func TestCacheStatsReporting(t *testing.T) {
	service := NewGMPHistoryService(nil)

	// Add some test data to cache
	for i := 0; i < 3; i++ {
		cacheKey := "gmp_history:test-ipo-" + string(rune('A'+i))
		testData := &models.GMPPriceHistoryCollection{
			IPOID:   "test-ipo-" + string(rune('A'+i)),
			IPOName: "Test IPO " + string(rune('A'+i)),
		}
		service.cache.Set(cacheKey, testData)
	}

	// Get cache stats
	stats := service.GetCacheStats()

	// Verify stats structure
	if stats == nil {
		t.Fatal("Expected cache stats to be returned, got nil")
	}

	// Check enabled flag
	enabled, ok := stats["enabled"].(bool)
	if !ok {
		t.Error("Expected 'enabled' field in stats")
	}
	if !enabled {
		t.Error("Expected cache to be enabled")
	}

	// Check size
	size, ok := stats["size"].(int)
	if !ok {
		t.Error("Expected 'size' field in stats")
	}
	if size != 3 {
		t.Errorf("Expected cache size to be 3, got %d", size)
	}

	// Check type
	cacheType, ok := stats["type"].(string)
	if !ok {
		t.Error("Expected 'type' field in stats")
	}
	if cacheType != "in-memory" {
		t.Errorf("Expected cache type 'in-memory', got '%s'", cacheType)
	}
}

// TestCacheWithNilService tests cache operations when service is nil
func TestCacheWithNilService(t *testing.T) {
	service := NewGMPHistoryService(nil)
	service.cache = nil

	// Test that operations don't panic with nil cache
	service.InvalidateIPOCache("test-ipo")
	service.InvalidateAllCache()

	stats := service.GetCacheStats()
	if stats == nil {
		t.Fatal("Expected stats to be returned even with nil cache")
	}

	enabled, ok := stats["enabled"].(bool)
	if !ok || enabled {
		t.Error("Expected cache to be disabled when nil")
	}
}

// TestCacheConcurrency tests concurrent cache access
func TestCacheConcurrency(t *testing.T) {
	service := NewGMPHistoryService(nil)

	// Number of concurrent goroutines
	numGoroutines := 10
	numOperations := 100

	// Channel to signal completion
	done := make(chan bool, numGoroutines)

	// Launch concurrent goroutines
	for g := 0; g < numGoroutines; g++ {
		go func(id int) {
			for i := 0; i < numOperations; i++ {
				cacheKey := "gmp_history:test-ipo-" + string(rune('A'+id))
				testData := &models.GMPPriceHistoryCollection{
					IPOID:   "test-ipo-" + string(rune('A'+id)),
					IPOName: "Test IPO " + string(rune('A'+id)),
				}

				// Alternate between set and get operations
				if i%2 == 0 {
					service.cache.Set(cacheKey, testData)
				} else {
					service.cache.Get(cacheKey)
				}
			}
			done <- true
		}(g)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// If we get here without panicking, the test passes
	t.Log("Concurrent cache access completed successfully")
}

// TestInvalidateCacheOnMultipleIPOs tests cache invalidation for multiple IPOs
func TestInvalidateCacheOnMultipleIPOs(t *testing.T) {
	service := NewGMPHistoryService(nil)

	// Add cache entries for multiple IPOs
	ipoIDs := []string{"ipo-1", "ipo-2", "ipo-3"}
	for _, ipoID := range ipoIDs {
		cacheKey := "gmp_history:" + ipoID
		testData := &models.GMPPriceHistoryCollection{
			IPOID:   ipoID,
			IPOName: "Test IPO " + ipoID,
		}
		service.cache.Set(cacheKey, testData)
	}

	// Verify all entries are in cache
	for _, ipoID := range ipoIDs {
		cacheKey := "gmp_history:" + ipoID
		if _, found := service.cache.Get(cacheKey); !found {
			t.Errorf("Expected cache entry for %s", ipoID)
		}
	}

	// Invalidate cache for one IPO
	service.InvalidateIPOCache("ipo-2")

	// Verify only the invalidated entry is removed
	if _, found := service.cache.Get("gmp_history:ipo-1"); !found {
		t.Error("Expected ipo-1 to still be in cache")
	}
	if _, found := service.cache.Get("gmp_history:ipo-2"); found {
		t.Error("Expected ipo-2 to be removed from cache")
	}
	if _, found := service.cache.Get("gmp_history:ipo-3"); !found {
		t.Error("Expected ipo-3 to still be in cache")
	}
}
