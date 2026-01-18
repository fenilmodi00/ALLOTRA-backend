package shared

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ErrorCategory represents different types of errors that can occur
type ErrorCategory string

const (
	ErrorCategoryConfiguration  ErrorCategory = "configuration"
	ErrorCategoryNetwork        ErrorCategory = "network"
	ErrorCategoryDatabase       ErrorCategory = "database"
	ErrorCategoryValidation     ErrorCategory = "validation"
	ErrorCategoryProcessing     ErrorCategory = "processing"
	ErrorCategoryResource       ErrorCategory = "resource"
	ErrorCategoryTimeout        ErrorCategory = "timeout"
	ErrorCategoryAuthentication ErrorCategory = "authentication"
	ErrorCategoryAuthorization  ErrorCategory = "authorization"
)

// ServiceError represents a standardized error with additional context
type ServiceError struct {
	Category    ErrorCategory `json:"category"`
	Code        string        `json:"code"`
	Message     string        `json:"message"`
	Details     interface{}   `json:"details,omitempty"`
	Timestamp   time.Time     `json:"timestamp"`
	ServiceName string        `json:"service_name"`
	Operation   string        `json:"operation"`
	Retryable   bool          `json:"retryable"`
	Cause       error         `json:"-"` // Original error, not serialized
}

// Error implements the error interface
func (e *ServiceError) Error() string {
	return fmt.Sprintf("[%s:%s] %s", e.Category, e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *ServiceError) Unwrap() error {
	return e.Cause
}

// NewServiceError creates a new service error
func NewServiceError(category ErrorCategory, code, message, serviceName, operation string, retryable bool, cause error) *ServiceError {
	return &ServiceError{
		Category:    category,
		Code:        code,
		Message:     message,
		Timestamp:   time.Now(),
		ServiceName: serviceName,
		Operation:   operation,
		Retryable:   retryable,
		Cause:       cause,
	}
}

// WithDetails adds additional details to the error
func (e *ServiceError) WithDetails(details interface{}) *ServiceError {
	e.Details = details
	return e
}

// IsRetryable returns whether the error is retryable
func (e *ServiceError) IsRetryable() bool {
	return e.Retryable
}

// GetCategory returns the error category
func (e *ServiceError) GetCategory() ErrorCategory {
	return e.Category
}

// LogError logs the error with structured fields
func (e *ServiceError) LogError() {
	logrus.WithFields(logrus.Fields{
		"error_category":   e.Category,
		"error_code":       e.Code,
		"error_message":    e.Message,
		"service_name":     e.ServiceName,
		"operation":        e.Operation,
		"retryable":        e.Retryable,
		"timestamp":        e.Timestamp,
		"details":          e.Details,
		"underlying_error": e.Cause,
	}).Error("Service error occurred")
}

// ErrorIsolationHandler handles error isolation to prevent cascading failures
type ErrorIsolationHandler struct {
	maxFailureRate      float64
	serviceName         string
	circuitBreakerOpen  bool
	failureCount        int64
	successCount        int64
	lastResetTime       time.Time
	fallbackStrategy    FallbackStrategy
	halfOpenAttempts    int
	maxHalfOpenAttempts int
}

// FallbackStrategy defines how to handle operations when circuit breaker is open
type FallbackStrategy interface {
	Execute(operation string, originalFunc func() (interface{}, error)) (interface{}, error)
}

// DefaultFallbackStrategy provides a simple fallback that returns cached data or default values
type DefaultFallbackStrategy struct {
	serviceName string
}

// Execute implements the FallbackStrategy interface
func (f *DefaultFallbackStrategy) Execute(operation string, originalFunc func() (interface{}, error)) (interface{}, error) {
	logrus.WithFields(logrus.Fields{
		"service_name": f.serviceName,
		"operation":    operation,
		"component":    "DefaultFallbackStrategy",
	}).Warn("Executing fallback strategy due to circuit breaker")

	// Return a default error indicating service is temporarily unavailable
	return nil, NewServiceError(
		ErrorCategoryResource,
		"SERVICE_UNAVAILABLE",
		fmt.Sprintf("Service %s is temporarily unavailable for operation %s", f.serviceName, operation),
		f.serviceName,
		operation,
		true, // This is retryable
		nil,
	)
}

// CachedFallbackStrategy provides fallback using cached data
type CachedFallbackStrategy struct {
	serviceName string
	cache       map[string]interface{}
}

// Execute implements the FallbackStrategy interface with cached data
func (f *CachedFallbackStrategy) Execute(operation string, originalFunc func() (interface{}, error)) (interface{}, error) {
	logrus.WithFields(logrus.Fields{
		"service_name": f.serviceName,
		"operation":    operation,
		"component":    "CachedFallbackStrategy",
	}).Info("Using cached data as fallback")

	if cachedData, exists := f.cache[operation]; exists {
		return cachedData, nil
	}

	// If no cached data available, return error
	return nil, NewServiceError(
		ErrorCategoryResource,
		"NO_CACHED_DATA",
		fmt.Sprintf("No cached data available for operation %s", operation),
		f.serviceName,
		operation,
		true,
		nil,
	)
}

// NewErrorIsolationHandler creates a new error isolation handler
func NewErrorIsolationHandler(serviceName string, maxFailureRate float64) *ErrorIsolationHandler {
	return &ErrorIsolationHandler{
		maxFailureRate:      maxFailureRate,
		serviceName:         serviceName,
		lastResetTime:       time.Now(),
		fallbackStrategy:    &DefaultFallbackStrategy{serviceName: serviceName},
		maxHalfOpenAttempts: 3,
	}
}

// NewErrorIsolationHandlerWithoutCircuitBreaker creates a new error isolation handler with circuit breaker disabled
func NewErrorIsolationHandlerWithoutCircuitBreaker(serviceName string) *ErrorIsolationHandler {
	return &ErrorIsolationHandler{
		maxFailureRate:      -1, // Negative value disables circuit breaker
		serviceName:         serviceName,
		lastResetTime:       time.Now(),
		fallbackStrategy:    &DefaultFallbackStrategy{serviceName: serviceName},
		maxHalfOpenAttempts: 3,
	}
}

// NewErrorIsolationHandlerWithFallback creates a new error isolation handler with custom fallback
func NewErrorIsolationHandlerWithFallback(serviceName string, maxFailureRate float64, fallback FallbackStrategy) *ErrorIsolationHandler {
	return &ErrorIsolationHandler{
		maxFailureRate:      maxFailureRate,
		serviceName:         serviceName,
		lastResetTime:       time.Now(),
		fallbackStrategy:    fallback,
		maxHalfOpenAttempts: 3,
	}
}

// RecordSuccess records a successful operation
func (h *ErrorIsolationHandler) RecordSuccess() {
	h.successCount++

	// Skip circuit breaker logic if disabled (negative maxFailureRate)
	if h.maxFailureRate < 0 {
		return
	}

	// If circuit breaker is in half-open state, check if we can close it
	if h.circuitBreakerOpen {
		h.halfOpenAttempts++

		// Close circuit breaker if we have enough successful operations in half-open state
		if h.halfOpenAttempts >= h.maxHalfOpenAttempts {
			h.circuitBreakerOpen = false
			h.failureCount = 0
			h.successCount = 0
			h.halfOpenAttempts = 0
			h.lastResetTime = time.Now()

			logrus.WithFields(logrus.Fields{
				"service_name": h.serviceName,
				"component":    "ErrorIsolationHandler",
			}).Info("Circuit breaker closed after successful half-open attempts")
		}
	}
}

// RecordFailure records a failed operation
func (h *ErrorIsolationHandler) RecordFailure() {
	h.failureCount++

	// Skip circuit breaker logic if disabled (negative maxFailureRate)
	if h.maxFailureRate < 0 {
		return
	}

	// If in half-open state and we get a failure, go back to open
	if h.circuitBreakerOpen && h.halfOpenAttempts > 0 {
		h.halfOpenAttempts = 0
		logrus.WithFields(logrus.Fields{
			"service_name": h.serviceName,
			"component":    "ErrorIsolationHandler",
		}).Warn("Circuit breaker returned to open state after failure in half-open")
		return
	}

	totalOperations := h.failureCount + h.successCount
	if totalOperations >= 10 { // Minimum sample size
		currentFailureRate := float64(h.failureCount) / float64(totalOperations)

		if currentFailureRate > h.maxFailureRate && !h.circuitBreakerOpen {
			h.circuitBreakerOpen = true
			h.halfOpenAttempts = 0

			logrus.WithFields(logrus.Fields{
				"service_name":     h.serviceName,
				"component":        "ErrorIsolationHandler",
				"failure_rate":     currentFailureRate,
				"max_failure_rate": h.maxFailureRate,
				"failure_count":    h.failureCount,
				"success_count":    h.successCount,
			}).Warn("Circuit breaker opened due to high failure rate")
		}
	}
}

// IsCircuitBreakerOpen returns whether the circuit breaker is open
func (h *ErrorIsolationHandler) IsCircuitBreakerOpen() bool {
	// If circuit breaker is disabled (negative maxFailureRate), always return false
	if h.maxFailureRate < 0 {
		return false
	}

	// If circuit breaker is not open, return false
	if !h.circuitBreakerOpen {
		return false
	}

	// Check if enough time has passed to try half-open state
	timeSinceLastReset := time.Since(h.lastResetTime)
	if timeSinceLastReset > 30*time.Second && h.halfOpenAttempts == 0 {
		logrus.WithFields(logrus.Fields{
			"service_name": h.serviceName,
			"component":    "ErrorIsolationHandler",
		}).Info("Circuit breaker entering half-open state")
		// Allow one attempt by returning false, but don't change circuitBreakerOpen yet
		return false
	}

	return h.circuitBreakerOpen
}

// ExecuteWithCircuitBreaker executes an operation with circuit breaker protection
func (h *ErrorIsolationHandler) ExecuteWithCircuitBreaker(operation string, fn func() (interface{}, error)) (interface{}, error) {
	// Check if circuit breaker is open
	if h.IsCircuitBreakerOpen() {
		logrus.WithFields(logrus.Fields{
			"service_name": h.serviceName,
			"operation":    operation,
			"component":    "ErrorIsolationHandler",
		}).Warn("Circuit breaker is open, using fallback strategy")

		return h.fallbackStrategy.Execute(operation, fn)
	}

	// Execute the operation
	result, err := fn()

	if err != nil {
		h.RecordFailure()
		return result, err
	}

	h.RecordSuccess()
	return result, nil
}

// ProcessBatchWithIsolation processes a batch of items with error isolation
func (h *ErrorIsolationHandler) ProcessBatchWithIsolation(
	items []interface{},
	processor func(interface{}) (interface{}, error),
) BatchProcessingResult {
	startTime := time.Now()
	var successfulItems []interface{}
	var failedItems []FailedItem
	var sampleErrors []error

	for _, item := range items {
		result, err := h.ExecuteWithCircuitBreaker(
			fmt.Sprintf("process_item_%T", item),
			func() (interface{}, error) {
				return processor(item)
			},
		)

		if err != nil {
			failedItem := FailedItem{
				OriginalData: item,
				Error:        err,
				RetryCount:   0,
				FailureTime:  time.Now(),
			}
			failedItems = append(failedItems, failedItem)

			// Collect sample errors for summary (limit to prevent memory issues)
			if len(sampleErrors) < 10 {
				sampleErrors = append(sampleErrors, err)
			}
		} else {
			successfulItems = append(successfulItems, result)
		}
	}

	processingTime := time.Since(startTime)
	totalProcessed := len(items)
	successCount := len(successfulItems)
	successRate := float64(successCount) / float64(totalProcessed)

	errorSummary := ""
	if len(failedItems) > 0 {
		errorSummary = BuildBatchProcessingErrorSummary(successCount, len(failedItems), sampleErrors)
	}

	return BatchProcessingResult{
		SuccessfulItems: successfulItems,
		FailedItems:     failedItems,
		TotalProcessed:  totalProcessed,
		SuccessRate:     successRate,
		ProcessingTime:  processingTime,
		ErrorSummary:    errorSummary,
	}
}

// GetFailureRate returns the current failure rate
func (h *ErrorIsolationHandler) GetFailureRate() float64 {
	totalOperations := h.failureCount + h.successCount
	if totalOperations == 0 {
		return 0.0
	}

	return float64(h.failureCount) / float64(totalOperations)
}

// BatchProcessingResult represents the result of batch processing with error isolation
type BatchProcessingResult struct {
	SuccessfulItems []interface{} `json:"successful_items"`
	FailedItems     []FailedItem  `json:"failed_items"`
	TotalProcessed  int           `json:"total_processed"`
	SuccessRate     float64       `json:"success_rate"`
	ProcessingTime  time.Duration `json:"processing_time"`
	ErrorSummary    string        `json:"error_summary,omitempty"`
}

// FailedItem represents an item that failed processing
type FailedItem struct {
	OriginalData interface{} `json:"original_data"`
	Error        error       `json:"error"`
	RetryCount   int         `json:"retry_count"`
	FailureTime  time.Time   `json:"failure_time"`
}

// BuildBatchProcessingErrorSummary creates a comprehensive error summary for batch processing results
func BuildBatchProcessingErrorSummary(successCount, totalErrorCount int, sampleErrors []error) string {
	var summaryBuilder strings.Builder
	summaryBuilder.WriteString(fmt.Sprintf("batch processing completed with %d successes and %d failures", successCount, totalErrorCount))

	// Include sample errors for debugging (limited to prevent memory issues)
	sampleSize := len(sampleErrors)
	if sampleSize > 3 {
		sampleSize = 3
	}

	for i := 0; i < sampleSize; i++ {
		summaryBuilder.WriteString(fmt.Sprintf("; %s", sampleErrors[i].Error()))
	}

	if totalErrorCount > len(sampleErrors) {
		summaryBuilder.WriteString(fmt.Sprintf("; and %d additional errors", totalErrorCount-len(sampleErrors)))
	}

	return summaryBuilder.String()
}

// WrapError wraps an existing error with service error context
func WrapError(err error, category ErrorCategory, code, serviceName, operation string, retryable bool) *ServiceError {
	if err == nil {
		return nil
	}

	// If it's already a ServiceError, just update the context
	if serviceErr, ok := err.(*ServiceError); ok {
		serviceErr.ServiceName = serviceName
		serviceErr.Operation = operation
		return serviceErr
	}

	return NewServiceError(category, code, err.Error(), serviceName, operation, retryable, err)
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if serviceErr, ok := err.(*ServiceError); ok {
		return serviceErr.IsRetryable()
	}

	// Default heuristics for standard errors
	errorMsg := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"timeout", "connection refused", "connection reset",
		"temporary failure", "service unavailable", "too many requests",
		"network", "dns", "socket",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errorMsg, pattern) {
			return true
		}
	}

	return false
}
