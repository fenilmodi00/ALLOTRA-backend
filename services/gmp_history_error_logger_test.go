package services

import (
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestGMPHistoryErrorLogger_LogError(t *testing.T) {
	logger := logrus.New()
	errorLogger := NewGMPHistoryErrorLogger(logger)

	// Test logging different error categories
	testCases := []struct {
		name      string
		category  ErrorCategory
		severity  ErrorSeverity
		component string
		operation string
		err       error
		context   map[string]interface{}
	}{
		{
			name:      "Network Error",
			category:  ErrorCategoryNetwork,
			severity:  ErrorSeverityHigh,
			component: "GMPHistoryScraper",
			operation: "ScrapeHistoryFromURL",
			err:       errors.New("connection timeout"),
			context: map[string]interface{}{
				"url":     "https://example.com",
				"timeout": "30s",
			},
		},
		{
			name:      "Database Error",
			category:  ErrorCategoryDatabase,
			severity:  ErrorSeverityCritical,
			component: "GMPHistoryService",
			operation: "SavePriceHistory",
			err:       errors.New("connection refused"),
			context: map[string]interface{}{
				"ipo_id": "test-123",
			},
		},
		{
			name:      "Validation Error",
			category:  ErrorCategoryValidation,
			severity:  ErrorSeverityMedium,
			component: "GMPHistoryService",
			operation: "ValidateHistoryData",
			err:       errors.New("invalid GMP value"),
			context: map[string]interface{}{
				"gmp_value": -10.5,
			},
		},
		{
			name:      "Parsing Error",
			category:  ErrorCategoryParsing,
			severity:  ErrorSeverityMedium,
			component: "GMPHistoryScraper",
			operation: "ExtractHistoryTable",
			err:       errors.New("malformed HTML"),
			context: map[string]interface{}{
				"row_index": 5,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errorLogger.LogError(tc.category, tc.severity, tc.component, tc.operation, tc.err, tc.context)

			// Verify metrics were updated
			metrics := errorLogger.GetMetrics()
			if metrics.TotalErrors == 0 {
				t.Error("Expected TotalErrors to be incremented")
			}

			// Verify category counter
			if metrics.ErrorsByCategory[tc.category] == 0 {
				t.Errorf("Expected category %s counter to be incremented", tc.category)
			}

			// Verify severity counter
			if metrics.ErrorsBySeverity[tc.severity] == 0 {
				t.Errorf("Expected severity %s counter to be incremented", tc.severity)
			}
		})
	}
}

func TestGMPHistoryErrorLogger_HelperMethods(t *testing.T) {
	logger := logrus.New()
	errorLogger := NewGMPHistoryErrorLogger(logger)

	t.Run("LogNetworkError", func(t *testing.T) {
		errorLogger.LogNetworkError("TestComponent", "TestOperation", errors.New("network error"), nil)
		metrics := errorLogger.GetMetrics()
		if metrics.NetworkErrors != 1 {
			t.Errorf("Expected NetworkErrors to be 1, got %d", metrics.NetworkErrors)
		}
	})

	t.Run("LogDatabaseError", func(t *testing.T) {
		errorLogger.LogDatabaseError("TestComponent", "TestOperation", errors.New("database error"), nil)
		metrics := errorLogger.GetMetrics()
		if metrics.DatabaseErrors != 1 {
			t.Errorf("Expected DatabaseErrors to be 1, got %d", metrics.DatabaseErrors)
		}
	})

	t.Run("LogValidationError", func(t *testing.T) {
		errorLogger.LogValidationError("TestComponent", "TestOperation", errors.New("validation error"), nil)
		metrics := errorLogger.GetMetrics()
		if metrics.ValidationErrors != 1 {
			t.Errorf("Expected ValidationErrors to be 1, got %d", metrics.ValidationErrors)
		}
	})

	t.Run("LogParsingError", func(t *testing.T) {
		errorLogger.LogParsingError("TestComponent", "TestOperation", errors.New("parsing error"), nil)
		metrics := errorLogger.GetMetrics()
		if metrics.ParsingErrors != 1 {
			t.Errorf("Expected ParsingErrors to be 1, got %d", metrics.ParsingErrors)
		}
	})

	t.Run("LogScrapingError", func(t *testing.T) {
		errorLogger.LogScrapingError("TestComponent", "TestOperation", errors.New("scraping error"), nil)
		metrics := errorLogger.GetMetrics()
		if metrics.ScrapingErrors != 1 {
			t.Errorf("Expected ScrapingErrors to be 1, got %d", metrics.ScrapingErrors)
		}
	})
}

func TestGMPHistoryErrorLogger_GetMetricsSummary(t *testing.T) {
	logger := logrus.New()
	errorLogger := NewGMPHistoryErrorLogger(logger)

	// Log some errors
	errorLogger.LogNetworkError("Test", "Op1", errors.New("error1"), nil)
	errorLogger.LogDatabaseError("Test", "Op2", errors.New("error2"), nil)
	errorLogger.LogValidationError("Test", "Op3", errors.New("error3"), nil)

	summary := errorLogger.GetMetricsSummary()

	// Verify summary contains expected keys
	expectedKeys := []string{
		"total_errors",
		"network_errors",
		"database_errors",
		"validation_errors",
		"parsing_errors",
		"scraping_errors",
		"business_errors",
		"system_errors",
		"external_errors",
		"critical_errors",
		"high_errors",
		"medium_errors",
		"low_errors",
		"last_error_time",
		"errors_by_category",
		"errors_by_severity",
	}

	for _, key := range expectedKeys {
		if _, exists := summary[key]; !exists {
			t.Errorf("Expected summary to contain key %s", key)
		}
	}

	// Verify counts
	if summary["total_errors"].(int) != 3 {
		t.Errorf("Expected total_errors to be 3, got %v", summary["total_errors"])
	}
}

func TestGMPHistoryErrorLogger_RecentErrors(t *testing.T) {
	logger := logrus.New()
	errorLogger := NewGMPHistoryErrorLogger(logger)

	// Log multiple errors
	for i := 0; i < 5; i++ {
		errorLogger.LogNetworkError("Test", "Operation", errors.New("test error"), map[string]interface{}{
			"index": i,
		})
	}

	// Get recent errors
	recentErrors := errorLogger.GetRecentErrors(3)
	if len(recentErrors) != 3 {
		t.Errorf("Expected 3 recent errors, got %d", len(recentErrors))
	}

	// Verify they are the most recent ones (indices 2, 3, 4)
	for i, record := range recentErrors {
		expectedIndex := i + 2
		if record.ContextData["index"].(int) != expectedIndex {
			t.Errorf("Expected index %d, got %d", expectedIndex, record.ContextData["index"])
		}
	}
}

func TestGMPHistoryErrorLogger_FilterByCategory(t *testing.T) {
	logger := logrus.New()
	errorLogger := NewGMPHistoryErrorLogger(logger)

	// Log errors of different categories
	errorLogger.LogNetworkError("Test", "Op1", errors.New("network1"), nil)
	errorLogger.LogNetworkError("Test", "Op2", errors.New("network2"), nil)
	errorLogger.LogDatabaseError("Test", "Op3", errors.New("database1"), nil)
	errorLogger.LogValidationError("Test", "Op4", errors.New("validation1"), nil)

	// Filter by network category
	networkErrors := errorLogger.GetErrorsByCategory(ErrorCategoryNetwork)
	if len(networkErrors) != 2 {
		t.Errorf("Expected 2 network errors, got %d", len(networkErrors))
	}

	// Filter by database category
	databaseErrors := errorLogger.GetErrorsByCategory(ErrorCategoryDatabase)
	if len(databaseErrors) != 1 {
		t.Errorf("Expected 1 database error, got %d", len(databaseErrors))
	}
}

func TestGMPHistoryErrorLogger_FilterBySeverity(t *testing.T) {
	logger := logrus.New()
	errorLogger := NewGMPHistoryErrorLogger(logger)

	// Log errors of different severities
	errorLogger.LogError(ErrorCategoryNetwork, ErrorSeverityCritical, "Test", "Op1", errors.New("critical1"), nil)
	errorLogger.LogError(ErrorCategoryNetwork, ErrorSeverityCritical, "Test", "Op2", errors.New("critical2"), nil)
	errorLogger.LogError(ErrorCategoryDatabase, ErrorSeverityHigh, "Test", "Op3", errors.New("high1"), nil)
	errorLogger.LogError(ErrorCategoryValidation, ErrorSeverityMedium, "Test", "Op4", errors.New("medium1"), nil)

	// Filter by critical severity
	criticalErrors := errorLogger.GetErrorsBySeverity(ErrorSeverityCritical)
	if len(criticalErrors) != 2 {
		t.Errorf("Expected 2 critical errors, got %d", len(criticalErrors))
	}

	// Filter by high severity
	highErrors := errorLogger.GetErrorsBySeverity(ErrorSeverityHigh)
	if len(highErrors) != 1 {
		t.Errorf("Expected 1 high error, got %d", len(highErrors))
	}
}

func TestGMPHistoryErrorLogger_ShouldAlert(t *testing.T) {
	logger := logrus.New()

	t.Run("Alert on critical errors", func(t *testing.T) {
		errorLogger := NewGMPHistoryErrorLogger(logger)
		errorLogger.LogError(ErrorCategoryDatabase, ErrorSeverityCritical, "Test", "Op", errors.New("critical"), nil)

		shouldAlert, reason := errorLogger.ShouldAlert()
		if !shouldAlert {
			t.Error("Expected alert for critical error")
		}
		if reason == "" {
			t.Error("Expected alert reason to be provided")
		}
	})

	t.Run("Alert on high database error count", func(t *testing.T) {
		errorLogger := NewGMPHistoryErrorLogger(logger)
		for i := 0; i < 6; i++ {
			errorLogger.LogDatabaseError("Test", "Op", errors.New("db error"), nil)
		}

		shouldAlert, reason := errorLogger.ShouldAlert()
		if !shouldAlert {
			t.Error("Expected alert for high database error count")
		}
		if reason == "" {
			t.Error("Expected alert reason to be provided")
		}
	})

	t.Run("No alert for low error count", func(t *testing.T) {
		errorLogger := NewGMPHistoryErrorLogger(logger)
		errorLogger.LogValidationError("Test", "Op", errors.New("validation"), nil)

		shouldAlert, _ := errorLogger.ShouldAlert()
		if shouldAlert {
			t.Error("Did not expect alert for single validation error")
		}
	})
}

func TestGMPHistoryErrorLogger_ResetMetrics(t *testing.T) {
	logger := logrus.New()
	errorLogger := NewGMPHistoryErrorLogger(logger)

	// Log some errors
	errorLogger.LogNetworkError("Test", "Op1", errors.New("error1"), nil)
	errorLogger.LogDatabaseError("Test", "Op2", errors.New("error2"), nil)

	// Verify errors were logged
	metrics := errorLogger.GetMetrics()
	if metrics.TotalErrors != 2 {
		t.Errorf("Expected 2 total errors before reset, got %d", metrics.TotalErrors)
	}

	// Reset metrics
	errorLogger.ResetMetrics()

	// Verify metrics were reset
	metrics = errorLogger.GetMetrics()
	if metrics.TotalErrors != 0 {
		t.Errorf("Expected 0 total errors after reset, got %d", metrics.TotalErrors)
	}
	if metrics.NetworkErrors != 0 {
		t.Errorf("Expected 0 network errors after reset, got %d", metrics.NetworkErrors)
	}
	if metrics.DatabaseErrors != 0 {
		t.Errorf("Expected 0 database errors after reset, got %d", metrics.DatabaseErrors)
	}
}

func TestGMPHistoryErrorLogger_LogInfoAndSuccess(t *testing.T) {
	logger := logrus.New()
	errorLogger := NewGMPHistoryErrorLogger(logger)

	// These should not increment error counters
	errorLogger.LogInfo("Test", "Op1", "Info message", map[string]interface{}{"key": "value"})
	errorLogger.LogSuccess("Test", "Op2", map[string]interface{}{"records": 10})

	metrics := errorLogger.GetMetrics()
	if metrics.TotalErrors != 0 {
		t.Errorf("Expected 0 total errors for info/success logs, got %d", metrics.TotalErrors)
	}
}
