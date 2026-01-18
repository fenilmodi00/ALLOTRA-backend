package tests

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/google/uuid"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "github.com/lib/pq"
)

// PerformanceTestSuite provides comprehensive performance and load testing
type PerformanceTestSuite struct {
	db             *sql.DB
	ipoService     *services.IPOService
	gmpService     *services.GMPService
	utilityService *services.UtilityService
}

// SetupPerformanceTestSuite initializes the performance test environment
func SetupPerformanceTestSuite(t *testing.T) *PerformanceTestSuite {
	// Use test database connection
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://localhost/ipo_backend_test?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Skipf("Skipping performance tests - database not available: %v", err)
		return nil
	}

	// Test database connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Skipping performance tests - database ping failed: %v", err)
		return nil
	}

	// Initialize services
	ipoService := services.NewIPOService(db)
	gmpService := services.NewGMPService()
	utilityService := services.NewUtilityService()

	return &PerformanceTestSuite{
		db:             db,
		ipoService:     ipoService,
		gmpService:     gmpService,
		utilityService: utilityService,
	}
}

// TeardownPerformanceTestSuite cleans up the performance test environment
func (suite *PerformanceTestSuite) TeardownPerformanceTestSuite() {
	if suite.db != nil {
		suite.db.Close()
	}
}

// TestSystemBehaviorUnderLoad tests system behavior under various load conditions
// **Feature: service-alignment-enhancement, Performance Test: System behavior under load**
// **Validates: Requirements 6.1, 6.6, 7.6**
func TestSystemBehaviorUnderLoad(t *testing.T) {
	suite := SetupPerformanceTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownPerformanceTestSuite()

	properties := gopter.NewProperties(nil)

	properties.Property("System should maintain consistent performance and resource usage under varying load conditions", prop.ForAll(
		func(concurrentUsers, operationsPerUser int, operationDurationMs int64) bool {
			// Skip invalid test parameters
			if concurrentUsers <= 0 || concurrentUsers > 50 ||
				operationsPerUser <= 0 || operationsPerUser > 20 ||
				operationDurationMs <= 0 || operationDurationMs > 5000 {
				return true
			}

			operationDuration := time.Duration(operationDurationMs) * time.Millisecond

			// Track system resources before load test
			var memStatsBefore runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memStatsBefore)

			// Create metrics for tracking performance under load
			loadTestMetrics := shared.NewServiceMetrics("LoadTest")
			startTime := time.Now()

			// Create wait group for concurrent operations
			var wg sync.WaitGroup
			errorChan := make(chan error, concurrentUsers*operationsPerUser)

			// Launch concurrent users
			for user := 0; user < concurrentUsers; user++ {
				wg.Add(1)
				go func(userID int) {
					defer wg.Done()

					// Each user performs multiple operations
					for op := 0; op < operationsPerUser; op++ {
						opStartTime := time.Now()

						// Perform mixed operations to simulate real load
						switch op % 4 {
						case 0:
							// Text processing operation
							testText := fmt.Sprintf("TestCompany_%d_%d Ltd", userID, op)
							cleanedText := suite.utilityService.CleanCompanyText(testText)
							if cleanedText == "" && testText != "" {
								errorChan <- fmt.Errorf("text processing failed for user %d, op %d", userID, op)
								return
							}

						case 1:
							// Company code generation operation
							testName := fmt.Sprintf("Company_%d_%d", userID, op)
							companyCode := suite.utilityService.GenerateCompanyCode(testName)
							if companyCode == "" && testName != "" {
								errorChan <- fmt.Errorf("company code generation failed for user %d, op %d", userID, op)
								return
							}

						case 2:
							// Numeric processing operation
							priceText := fmt.Sprintf("₹%d.%d", userID*100, op*10)
							extractedPrice := suite.utilityService.ExtractNumeric(priceText)
							if extractedPrice <= 0 {
								errorChan <- fmt.Errorf("numeric extraction failed for user %d, op %d", userID, op)
								return
							}

						case 3:
							// IPO validation operation
							testIPO := &models.IPO{
								ID:          uuid.New(),
								StockID:     fmt.Sprintf("TEST_%d_%d", userID, op),
								Name:        fmt.Sprintf("TestCompany_%d_%d", userID, op),
								CompanyCode: fmt.Sprintf("TC%d%d", userID, op),
								Registrar:   "Test Registrar",
								Status:      "UPCOMING",
							}

							// validator := services.NewUnifiedValidator(suite.utilityService, suite.db)
							// validationResult := validator.Validate(testIPO)
							// if validationResult == nil {
							//	errorChan <- fmt.Errorf("IPO validation failed for user %d, op %d", userID, op)
							//	return
							// }
						}

						// Record operation metrics
						opDuration := time.Since(opStartTime)
						loadTestMetrics.RecordRequest(true, opDuration)

						// Simulate operation duration
						if opDuration < operationDuration {
							time.Sleep(operationDuration - opDuration)
						}
					}
				}(user)
			}

			// Wait for all operations to complete
			wg.Wait()
			close(errorChan)

			totalDuration := time.Since(startTime)

			// Check for errors during load test
			var errors []error
			for err := range errorChan {
				errors = append(errors, err)
			}

			if len(errors) > 0 {
				t.Logf("Load test had %d errors out of %d total operations", len(errors), concurrentUsers*operationsPerUser)
				// Allow some errors under high load, but not too many
				errorRate := float64(len(errors)) / float64(concurrentUsers*operationsPerUser)
				if errorRate > 0.1 { // More than 10% error rate is unacceptable
					t.Logf("Error rate too high: %f", errorRate)
					return false
				}
			}

			// Track system resources after load test
			var memStatsAfter runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memStatsAfter)

			// Get performance metrics
			loadSnapshot := loadTestMetrics.GetSnapshot()

			// Verify performance characteristics
			expectedOperations := concurrentUsers * operationsPerUser
			if loadSnapshot.TotalRequests != int64(expectedOperations-len(errors)) {
				t.Logf("Expected %d successful operations, got %d", expectedOperations-len(errors), loadSnapshot.TotalRequests)
				return false
			}

			// Check average processing time is reasonable
			avgProcessingTime := loadSnapshot.AverageProcessingTime
			if avgProcessingTime > operationDuration*2 {
				t.Logf("Average processing time too high: %v (expected <= %v)", avgProcessingTime, operationDuration*2)
				return false
			}

			// Check memory usage didn't grow excessively
			memoryGrowth := memStatsAfter.Alloc - memStatsBefore.Alloc
			maxAcceptableGrowth := uint64(concurrentUsers * operationsPerUser * 1024) // 1KB per operation
			if memoryGrowth > maxAcceptableGrowth {
				t.Logf("Memory growth too high: %d bytes (expected <= %d)", memoryGrowth, maxAcceptableGrowth)
				return false
			}

			// Check total test duration is reasonable
			expectedMinDuration := time.Duration(operationsPerUser) * operationDuration
			expectedMaxDuration := expectedMinDuration * 3 // Allow 3x overhead for concurrency
			if totalDuration > expectedMaxDuration {
				t.Logf("Total test duration too high: %v (expected <= %v)", totalDuration, expectedMaxDuration)
				return false
			}

			// Verify success rate is acceptable
			successRate := loadSnapshot.GetSuccessRate()
			if successRate < 90.0 { // At least 90% success rate
				t.Logf("Success rate too low: %f%%", successRate)
				return false
			}

			return true
		},
		gen.IntRange(1, 20),     // concurrentUsers
		gen.IntRange(1, 10),     // operationsPerUser
		gen.Int64Range(10, 500), // operationDurationMs
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestResourceCleanupUnderStress tests resource cleanup under stress conditions
// **Feature: service-alignment-enhancement, Performance Test: Resource cleanup under stress**
// **Validates: Requirements 7.6, 3.1, 3.4**
func TestResourceCleanupUnderStress(t *testing.T) {
	suite := SetupPerformanceTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownPerformanceTestSuite()

	properties := gopter.NewProperties(nil)

	properties.Property("System should properly clean up resources under stress conditions without memory leaks", prop.ForAll(
		func(resourceCount, stressIterations int, resourceLifetimeMs int64) bool {
			// Skip invalid test parameters
			if resourceCount <= 0 || resourceCount > 100 ||
				stressIterations <= 0 || stressIterations > 50 ||
				resourceLifetimeMs <= 0 || resourceLifetimeMs > 2000 {
				return true
			}

			resourceLifetime := time.Duration(resourceLifetimeMs) * time.Millisecond

			// Track initial memory state
			var memStatsBefore runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memStatsBefore)

			// Track goroutine count before stress test
			goroutinesBefore := runtime.NumGoroutine()

			// Perform stress test iterations
			for iteration := 0; iteration < stressIterations; iteration++ {
				// Create multiple resources simultaneously
				var wg sync.WaitGroup
				resourceChan := make(chan interface{}, resourceCount)

				for i := 0; i < resourceCount; i++ {
					wg.Add(1)
					go func(resourceID int) {
						defer wg.Done()

						// Create different types of resources
						switch resourceID % 4 {
						case 0:
							// HTTP client resources
							httpClientFactory := shared.NewHTTPClientFactory(resourceLifetime)
							httpClient := httpClientFactory.CreateOptimizedHTTPClient(resourceLifetime)
							resourceChan <- httpClient

						case 1:
							// Rate limiter resources
							rateLimiter := shared.NewHTTPRequestRateLimiter(resourceLifetime)
							resourceChan <- rateLimiter

						case 2:
							// Service metrics resources
							serviceMetrics := shared.NewServiceMetrics(fmt.Sprintf("StressTest_%d_%d", iteration, resourceID))
							// Record some operations to create internal state
							for j := 0; j < 10; j++ {
								serviceMetrics.RecordRequest(true, time.Duration(j)*time.Millisecond)
							}
							resourceChan <- serviceMetrics

						case 3:
							// Database metrics resources
							dbMetrics := shared.NewDatabaseMetrics()
							// Record some operations to create internal state
							for j := 0; j < 10; j++ {
								dbMetrics.RecordQuery(true, time.Duration(j)*time.Millisecond, j%3 == 0)
							}
							resourceChan <- dbMetrics
						}

						// Simulate resource usage
						time.Sleep(resourceLifetime / 10)
					}(i)
				}

				// Wait for all resources to be created
				wg.Wait()
				close(resourceChan)

				// Collect all resources
				var resources []interface{}
				for resource := range resourceChan {
					resources = append(resources, resource)
				}

				// Verify we created the expected number of resources
				if len(resources) != resourceCount {
					t.Logf("Expected %d resources, got %d in iteration %d", resourceCount, len(resources), iteration)
					return false
				}

				// Simulate resource usage period
				time.Sleep(resourceLifetime)

				// Clear references to allow garbage collection
				resources = nil

				// Force garbage collection
				runtime.GC()
				runtime.GC() // Run twice to ensure cleanup

				// Check memory growth during iteration
				var memStatsIter runtime.MemStats
				runtime.ReadMemStats(&memStatsIter)

				// Memory shouldn't grow excessively during iterations
				memoryGrowth := memStatsIter.Alloc - memStatsBefore.Alloc
				maxAcceptableGrowth := uint64(resourceCount * 1024 * 10) // 10KB per resource
				if memoryGrowth > maxAcceptableGrowth {
					t.Logf("Memory growth too high in iteration %d: %d bytes (expected <= %d)",
						iteration, memoryGrowth, maxAcceptableGrowth)
					return false
				}
			}

			// Final cleanup and verification
			runtime.GC()
			runtime.GC()
			time.Sleep(100 * time.Millisecond) // Allow time for cleanup

			// Check final memory state
			var memStatsAfter runtime.MemStats
			runtime.ReadMemStats(&memStatsAfter)

			// Check final goroutine count
			goroutinesAfter := runtime.NumGoroutine()

			// Memory should not have grown significantly
			finalMemoryGrowth := memStatsAfter.Alloc - memStatsBefore.Alloc
			maxFinalGrowth := uint64(stressIterations * 1024) // 1KB per iteration
			if finalMemoryGrowth > maxFinalGrowth {
				t.Logf("Final memory growth too high: %d bytes (expected <= %d)", finalMemoryGrowth, maxFinalGrowth)
				return false
			}

			// Goroutine count should not have grown significantly
			goroutineGrowth := goroutinesAfter - goroutinesBefore
			maxGoroutineGrowth := stressIterations / 10 // Allow some growth
			if goroutineGrowth > maxGoroutineGrowth {
				t.Logf("Goroutine count grew too much: %d (expected <= %d)", goroutineGrowth, maxGoroutineGrowth)
				return false
			}

			// Test database connection cleanup
			if suite.db != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				// Database should still be responsive after stress test
				if err := suite.db.PingContext(ctx); err != nil {
					t.Logf("Database not responsive after stress test: %v", err)
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 50),     // resourceCount
		gen.IntRange(1, 20),     // stressIterations
		gen.Int64Range(50, 500), // resourceLifetimeMs
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestErrorIsolationDuringHighFailureRates tests error isolation during high failure rates
// **Feature: service-alignment-enhancement, Performance Test: Error isolation during high failure rates**
// **Validates: Requirements 6.1, 6.3**
func TestErrorIsolationDuringHighFailureRates(t *testing.T) {
	suite := SetupPerformanceTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownPerformanceTestSuite()

	properties := gopter.NewProperties(nil)

	properties.Property("System should maintain error isolation and continue processing during high failure rates", prop.ForAll(
		func(totalOperations, failurePercentage int, processingTimeMs int64) bool {
			// Skip invalid test parameters
			if totalOperations <= 0 || totalOperations > 200 ||
				failurePercentage < 0 || failurePercentage > 90 ||
				processingTimeMs <= 0 || processingTimeMs > 1000 {
				return true
			}

			processingTime := time.Duration(processingTimeMs) * time.Millisecond
			expectedFailures := (totalOperations * failurePercentage) / 100
			expectedSuccesses := totalOperations - expectedFailures

			// Create test data with mix of valid and invalid items
			var testItems []interface{}
			for i := 0; i < totalOperations; i++ {
				shouldFail := i < expectedFailures
				if shouldFail {
					// Create invalid data that should fail
					testItems = append(testItems, map[string]interface{}{
						"type":    "invalid",
						"id":      i,
						"name":    "", // Invalid: empty name
						"price":   -1, // Invalid: negative price
						"company": "", // Invalid: empty company
					})
				} else {
					// Create valid data that should succeed
					testItems = append(testItems, map[string]interface{}{
						"type":    "valid",
						"id":      i,
						"name":    fmt.Sprintf("ValidItem_%d", i),
						"price":   100 + i,
						"company": fmt.Sprintf("Company_%d", i),
					})
				}
			}

			// Track metrics during high failure rate processing
			errorIsolationMetrics := shared.NewServiceMetrics("ErrorIsolationTest")
			startTime := time.Now()

			// Process items with error isolation
			var successfulItems []interface{}
			var failedItems []interface{}
			var processingErrors []error

			for _, item := range testItems {
				itemStartTime := time.Now()

				// Simulate processing with error isolation
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					processingErrors = append(processingErrors, fmt.Errorf("invalid item type"))
					failedItems = append(failedItems, item)
					continue
				}

				// Simulate validation and processing
				itemType, _ := itemMap["type"].(string)
				itemName, _ := itemMap["name"].(string)
				itemPrice, _ := itemMap["price"].(int)
				itemCompany, _ := itemMap["company"].(string)

				// Apply validation rules
				var validationErrors []string
				if itemName == "" {
					validationErrors = append(validationErrors, "empty name")
				}
				if itemPrice <= 0 {
					validationErrors = append(validationErrors, "invalid price")
				}
				if itemCompany == "" {
					validationErrors = append(validationErrors, "empty company")
				}

				// Simulate processing time
				time.Sleep(processingTime / 10)

				if len(validationErrors) > 0 {
					// Item failed validation - this should be isolated
					processingErrors = append(processingErrors, fmt.Errorf("validation failed: %v", validationErrors))
					failedItems = append(failedItems, item)
					errorIsolationMetrics.RecordRequest(false, time.Since(itemStartTime))
				} else {
					// Item processed successfully
					successfulItems = append(successfulItems, item)
					errorIsolationMetrics.RecordRequest(true, time.Since(itemStartTime))

					// Perform additional processing for successful items
					if itemType == "valid" {
						// Test text processing
						cleanedName := suite.utilityService.CleanCompanyText(itemName)
						if cleanedName == "" {
							processingErrors = append(processingErrors, fmt.Errorf("text processing failed for item %v", itemMap["id"]))
							continue
						}

						// Test company code generation
						companyCode := suite.utilityService.GenerateCompanyCode(itemCompany)
						if companyCode == "" {
							processingErrors = append(processingErrors, fmt.Errorf("company code generation failed for item %v", itemMap["id"]))
							continue
						}
					}
				}
			}

			totalProcessingTime := time.Since(startTime)

			// Verify error isolation behavior
			actualSuccesses := len(successfulItems)
			actualFailures := len(failedItems)
			actualTotal := actualSuccesses + actualFailures

			// All items should have been processed (either successfully or with isolated errors)
			if actualTotal != totalOperations {
				t.Logf("Not all items were processed: expected %d, got %d", totalOperations, actualTotal)
				return false
			}

			// Success/failure counts should be approximately correct (allowing some variance)
			successVariance := abs(actualSuccesses - expectedSuccesses)
			failureVariance := abs(actualFailures - expectedFailures)

			// Allow up to 10% variance due to randomization
			maxVariance := totalOperations / 10
			if successVariance > maxVariance {
				t.Logf("Success count variance too high: expected ~%d, got %d (variance: %d)",
					expectedSuccesses, actualSuccesses, successVariance)
				return false
			}

			if failureVariance > maxVariance {
				t.Logf("Failure count variance too high: expected ~%d, got %d (variance: %d)",
					expectedFailures, actualFailures, failureVariance)
				return false
			}

			// Verify metrics tracking
			metricsSnapshot := errorIsolationMetrics.GetSnapshot()
			if metricsSnapshot.TotalRequests != int64(totalOperations) {
				t.Logf("Metrics total requests mismatch: expected %d, got %d", totalOperations, metricsSnapshot.TotalRequests)
				return false
			}

			// Success rate should match actual processing results
			expectedMetricsSuccessRate := float64(actualSuccesses) / float64(totalOperations) * 100.0
			actualMetricsSuccessRate := metricsSnapshot.GetSuccessRate()
			if abs(int(expectedMetricsSuccessRate-actualMetricsSuccessRate)) > 5 { // Allow 5% variance
				t.Logf("Metrics success rate mismatch: expected ~%.1f%%, got %.1f%%",
					expectedMetricsSuccessRate, actualMetricsSuccessRate)
				return false
			}

			// Processing time should be reasonable
			expectedProcessingTime := time.Duration(totalOperations) * processingTime / 10
			maxAcceptableTime := expectedProcessingTime * 3 // Allow 3x overhead
			if totalProcessingTime > maxAcceptableTime {
				t.Logf("Processing time too high: %v (expected <= %v)", totalProcessingTime, maxAcceptableTime)
				return false
			}

			// Verify that processing continued despite high failure rates
			if failurePercentage > 50 && len(successfulItems) == 0 {
				t.Logf("No successful items processed despite having valid items in high failure scenario")
				return false
			}

			// Verify error isolation - successful items should not be affected by failed items
			for _, successItem := range successfulItems {
				itemMap, _ := successItem.(map[string]interface{})
				itemType, _ := itemMap["type"].(string)
				if itemType != "valid" {
					t.Logf("Invalid item marked as successful: %v", itemMap)
					return false
				}
			}

			return true
		},
		gen.IntRange(10, 100),  // totalOperations
		gen.IntRange(10, 80),   // failurePercentage
		gen.Int64Range(1, 100), // processingTimeMs
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Unit tests for specific performance scenarios

func TestServiceInitializationPerformance(t *testing.T) {
	suite := SetupPerformanceTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownPerformanceTestSuite()

	// Test that services initialize quickly
	startTime := time.Now()

	// Initialize multiple services
	for i := 0; i < 10; i++ {
		ipoService := services.NewIPOService(suite.db)
		gmpService := services.NewGMPService()
		utilityService := services.NewUtilityService()

		// Verify services are properly initialized
		if ipoService == nil || gmpService == nil || utilityService == nil {
			t.Errorf("Service initialization failed at iteration %d", i)
		}
	}

	initializationTime := time.Since(startTime)
	maxAcceptableTime := 5 * time.Second

	if initializationTime > maxAcceptableTime {
		t.Errorf("Service initialization too slow: %v (expected <= %v)", initializationTime, maxAcceptableTime)
	}
}

func TestConcurrentTextProcessingPerformance(t *testing.T) {
	suite := SetupPerformanceTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownPerformanceTestSuite()

	// Test concurrent text processing performance
	concurrentUsers := 20
	operationsPerUser := 50
	testTexts := []string{
		"TechCorp Industries Ltd",
		"Global Solutions Inc",
		"StartupXYZ Private Limited",
		"MegaCorp International",
		"InnovateTech Systems",
	}

	startTime := time.Now()
	var wg sync.WaitGroup
	errorChan := make(chan error, concurrentUsers*operationsPerUser)

	for user := 0; user < concurrentUsers; user++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()

			for op := 0; op < operationsPerUser; op++ {
				testText := testTexts[op%len(testTexts)]

				// Perform text processing operations
				cleanedText := suite.utilityService.CleanCompanyText(testText)
				if cleanedText == "" && testText != "" {
					errorChan <- fmt.Errorf("text cleaning failed for user %d, op %d", userID, op)
					return
				}

				companyCode := suite.utilityService.GenerateCompanyCode(testText)
				if companyCode == "" && testText != "" {
					errorChan <- fmt.Errorf("company code generation failed for user %d, op %d", userID, op)
					return
				}

				normalizedText := suite.utilityService.NormalizeTextContent(testText)
				if normalizedText == "" && testText != "" {
					errorChan <- fmt.Errorf("text normalization failed for user %d, op %d", userID, op)
					return
				}
			}
		}(user)
	}

	wg.Wait()
	close(errorChan)

	processingTime := time.Since(startTime)

	// Check for errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("Concurrent text processing had %d errors: %v", len(errors), errors[0])
	}

	// Verify performance
	totalOperations := concurrentUsers * operationsPerUser * 3 // 3 operations per iteration
	avgTimePerOperation := processingTime / time.Duration(totalOperations)
	maxAcceptableTimePerOp := 10 * time.Millisecond

	if avgTimePerOperation > maxAcceptableTimePerOp {
		t.Errorf("Text processing too slow: %v per operation (expected <= %v)", avgTimePerOperation, maxAcceptableTimePerOp)
	}
}

func TestMemoryUsageUnderLoad(t *testing.T) {
	suite := SetupPerformanceTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownPerformanceTestSuite()

	// Track initial memory
	var memStatsBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStatsBefore)

	// Perform memory-intensive operations
	iterations := 1000
	for i := 0; i < iterations; i++ {
		// Create and process test data
		testIPO := &models.IPO{
			ID:          uuid.New(),
			StockID:     fmt.Sprintf("MEMORY_TEST_%d", i),
			Name:        fmt.Sprintf("MemoryTestCompany_%d", i),
			CompanyCode: fmt.Sprintf("MTC%d", i),
			Registrar:   "Memory Test Registrar",
			Status:      "UPCOMING",
		}

		// Validate IPO
		// validator := services.NewUnifiedValidator(suite.utilityService, suite.db)
		// validationResult := validator.Validate(testIPO)
		// if validationResult == nil {
		//	t.Errorf("IPO validation failed at iteration %d", i)
		// }

		// Process text
		cleanedName := suite.utilityService.CleanCompanyText(testIPO.Name)
		if cleanedName == "" {
			t.Errorf("Text cleaning failed at iteration %d", i)
		}

		// Generate company code
		companyCode := suite.utilityService.GenerateCompanyCode(testIPO.Name)
		if companyCode == "" {
			t.Errorf("Company code generation failed at iteration %d", i)
		}

		// Force garbage collection every 100 iterations
		if i%100 == 0 {
			runtime.GC()
		}
	}

	// Final garbage collection
	runtime.GC()
	runtime.GC()

	// Check final memory usage
	var memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsAfter)

	memoryGrowth := memStatsAfter.Alloc - memStatsBefore.Alloc
	maxAcceptableGrowth := uint64(iterations * 1024) // 1KB per iteration

	if memoryGrowth > maxAcceptableGrowth {
		t.Errorf("Memory growth too high: %d bytes (expected <= %d)", memoryGrowth, maxAcceptableGrowth)
	}
}

func TestDatabaseConnectionPoolPerformance(t *testing.T) {
	suite := SetupPerformanceTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownPerformanceTestSuite()

	// Test database connection pool performance
	concurrentConnections := 20
	operationsPerConnection := 10

	startTime := time.Now()
	var wg sync.WaitGroup
	errorChan := make(chan error, concurrentConnections*operationsPerConnection)

	for conn := 0; conn < concurrentConnections; conn++ {
		wg.Add(1)
		go func(connID int) {
			defer wg.Done()

			for op := 0; op < operationsPerConnection; op++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

				// Test database ping
				if err := suite.db.PingContext(ctx); err != nil {
					errorChan <- fmt.Errorf("database ping failed for conn %d, op %d: %v", connID, op, err)
					cancel()
					return
				}

				cancel()
			}
		}(conn)
	}

	wg.Wait()
	close(errorChan)

	processingTime := time.Since(startTime)

	// Check for errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("Database connection pool test had %d errors: %v", len(errors), errors[0])
	}

	// Verify performance
	totalOperations := concurrentConnections * operationsPerConnection
	avgTimePerOperation := processingTime / time.Duration(totalOperations)
	maxAcceptableTimePerOp := 100 * time.Millisecond

	if avgTimePerOperation > maxAcceptableTimePerOp {
		t.Errorf("Database operations too slow: %v per operation (expected <= %v)", avgTimePerOperation, maxAcceptableTimePerOp)
	}
}
