package services

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// ErrorCategory represents the category of an error for classification
type ErrorCategory string

const (
	// ErrorCategoryNetwork represents network-related errors (timeouts, connection failures)
	ErrorCategoryNetwork ErrorCategory = "network"
	// ErrorCategoryDatabase represents database-related errors (query failures, connection issues)
	ErrorCategoryDatabase ErrorCategory = "database"
	// ErrorCategoryValidation represents data validation errors
	ErrorCategoryValidation ErrorCategory = "validation"
	// ErrorCategoryParsing represents HTML/data parsing errors
	ErrorCategoryParsing ErrorCategory = "parsing"
	// ErrorCategoryScraping represents web scraping errors
	ErrorCategoryScraping ErrorCategory = "scraping"
	// ErrorCategoryBusiness represents business logic errors
	ErrorCategoryBusiness ErrorCategory = "business_logic"
	// ErrorCategorySystem represents system-level errors
	ErrorCategorySystem ErrorCategory = "system"
	// ErrorCategoryExternal represents external service errors
	ErrorCategoryExternal ErrorCategory = "external_service"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	// ErrorSeverityCritical represents critical errors that require immediate attention
	ErrorSeverityCritical ErrorSeverity = "critical"
	// ErrorSeverityHigh represents high-priority errors that should be addressed soon
	ErrorSeverityHigh ErrorSeverity = "high"
	// ErrorSeverityMedium represents medium-priority errors
	ErrorSeverityMedium ErrorSeverity = "medium"
	// ErrorSeverityLow represents low-priority errors or warnings
	ErrorSeverityLow ErrorSeverity = "low"
)

// GMPHistoryErrorLogger provides structured error logging for GMP history operations
// Implements Requirement 8.5 - Consistent error logging using existing logrus framework
type GMPHistoryErrorLogger struct {
	logger  *logrus.Logger
	metrics *ErrorMetrics
}

// ErrorMetrics tracks error counts by category and severity
type ErrorMetrics struct {
	TotalErrors      int
	NetworkErrors    int
	DatabaseErrors   int
	ValidationErrors int
	ParsingErrors    int
	ScrapingErrors   int
	BusinessErrors   int
	SystemErrors     int
	ExternalErrors   int
	CriticalErrors   int
	HighErrors       int
	MediumErrors     int
	LowErrors        int
	LastErrorTime    time.Time
	ErrorsByCategory map[ErrorCategory]int
	ErrorsBySeverity map[ErrorSeverity]int
	RecentErrors     []ErrorRecord
	MaxRecentErrors  int
}

// ErrorRecord represents a single error occurrence for tracking
type ErrorRecord struct {
	Timestamp   time.Time
	Category    ErrorCategory
	Severity    ErrorSeverity
	Component   string
	Operation   string
	Message     string
	ContextData map[string]interface{}
}

// NewGMPHistoryErrorLogger creates a new error logger instance
func NewGMPHistoryErrorLogger(logger *logrus.Logger) *GMPHistoryErrorLogger {
	return &GMPHistoryErrorLogger{
		logger: logger,
		metrics: &ErrorMetrics{
			ErrorsByCategory: make(map[ErrorCategory]int),
			ErrorsBySeverity: make(map[ErrorSeverity]int),
			RecentErrors:     make([]ErrorRecord, 0),
			MaxRecentErrors:  100, // Keep last 100 errors
		},
	}
}

// LogError logs an error with category, severity, and context
func (l *GMPHistoryErrorLogger) LogError(
	category ErrorCategory,
	severity ErrorSeverity,
	component string,
	operation string,
	err error,
	context map[string]interface{},
) {
	// Update metrics
	l.updateMetrics(category, severity)

	// Build log fields
	fields := logrus.Fields{
		"error_category": string(category),
		"error_severity": string(severity),
		"component":      component,
		"operation":      operation,
		"error":          err.Error(),
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	// Add context fields
	for key, value := range context {
		fields[key] = value
	}

	// Record error for tracking
	l.recordError(category, severity, component, operation, err.Error(), context)

	// Log at appropriate level based on severity
	switch severity {
	case ErrorSeverityCritical:
		l.logger.WithFields(fields).Error("Critical error occurred")
	case ErrorSeverityHigh:
		l.logger.WithFields(fields).Error("High-priority error occurred")
	case ErrorSeverityMedium:
		l.logger.WithFields(fields).Warn("Medium-priority error occurred")
	case ErrorSeverityLow:
		l.logger.WithFields(fields).Warn("Low-priority error occurred")
	default:
		l.logger.WithFields(fields).Error("Error occurred")
	}
}

// LogNetworkError logs a network-related error
func (l *GMPHistoryErrorLogger) LogNetworkError(component, operation string, err error, context map[string]interface{}) {
	l.LogError(ErrorCategoryNetwork, ErrorSeverityHigh, component, operation, err, context)
}

// LogDatabaseError logs a database-related error
func (l *GMPHistoryErrorLogger) LogDatabaseError(component, operation string, err error, context map[string]interface{}) {
	l.LogError(ErrorCategoryDatabase, ErrorSeverityCritical, component, operation, err, context)
}

// LogValidationError logs a validation error
func (l *GMPHistoryErrorLogger) LogValidationError(component, operation string, err error, context map[string]interface{}) {
	l.LogError(ErrorCategoryValidation, ErrorSeverityMedium, component, operation, err, context)
}

// LogParsingError logs a parsing error
func (l *GMPHistoryErrorLogger) LogParsingError(component, operation string, err error, context map[string]interface{}) {
	l.LogError(ErrorCategoryParsing, ErrorSeverityMedium, component, operation, err, context)
}

// LogScrapingError logs a scraping error
func (l *GMPHistoryErrorLogger) LogScrapingError(component, operation string, err error, context map[string]interface{}) {
	l.LogError(ErrorCategoryScraping, ErrorSeverityHigh, component, operation, err, context)
}

// LogBusinessError logs a business logic error
func (l *GMPHistoryErrorLogger) LogBusinessError(component, operation string, err error, context map[string]interface{}) {
	l.LogError(ErrorCategoryBusiness, ErrorSeverityMedium, component, operation, err, context)
}

// LogExternalServiceError logs an external service error
func (l *GMPHistoryErrorLogger) LogExternalServiceError(component, operation string, err error, context map[string]interface{}) {
	l.LogError(ErrorCategoryExternal, ErrorSeverityHigh, component, operation, err, context)
}

// LogInfo logs an informational message with context
func (l *GMPHistoryErrorLogger) LogInfo(component, operation, message string, context map[string]interface{}) {
	fields := logrus.Fields{
		"component": component,
		"operation": operation,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	for key, value := range context {
		fields[key] = value
	}

	l.logger.WithFields(fields).Info(message)
}

// LogSuccess logs a successful operation with metrics
func (l *GMPHistoryErrorLogger) LogSuccess(component, operation string, context map[string]interface{}) {
	fields := logrus.Fields{
		"component": component,
		"operation": operation,
		"status":    "success",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	for key, value := range context {
		fields[key] = value
	}

	l.logger.WithFields(fields).Info("Operation completed successfully")
}

// LogWarning logs a warning message
func (l *GMPHistoryErrorLogger) LogWarning(component, operation, message string, context map[string]interface{}) {
	fields := logrus.Fields{
		"component": component,
		"operation": operation,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	for key, value := range context {
		fields[key] = value
	}

	l.logger.WithFields(fields).Warn(message)
}

// updateMetrics updates error metrics counters
func (l *GMPHistoryErrorLogger) updateMetrics(category ErrorCategory, severity ErrorSeverity) {
	l.metrics.TotalErrors++
	l.metrics.LastErrorTime = time.Now()

	// Update category counters
	l.metrics.ErrorsByCategory[category]++
	switch category {
	case ErrorCategoryNetwork:
		l.metrics.NetworkErrors++
	case ErrorCategoryDatabase:
		l.metrics.DatabaseErrors++
	case ErrorCategoryValidation:
		l.metrics.ValidationErrors++
	case ErrorCategoryParsing:
		l.metrics.ParsingErrors++
	case ErrorCategoryScraping:
		l.metrics.ScrapingErrors++
	case ErrorCategoryBusiness:
		l.metrics.BusinessErrors++
	case ErrorCategorySystem:
		l.metrics.SystemErrors++
	case ErrorCategoryExternal:
		l.metrics.ExternalErrors++
	}

	// Update severity counters
	l.metrics.ErrorsBySeverity[severity]++
	switch severity {
	case ErrorSeverityCritical:
		l.metrics.CriticalErrors++
	case ErrorSeverityHigh:
		l.metrics.HighErrors++
	case ErrorSeverityMedium:
		l.metrics.MediumErrors++
	case ErrorSeverityLow:
		l.metrics.LowErrors++
	}
}

// recordError records an error for tracking
func (l *GMPHistoryErrorLogger) recordError(
	category ErrorCategory,
	severity ErrorSeverity,
	component string,
	operation string,
	message string,
	context map[string]interface{},
) {
	record := ErrorRecord{
		Timestamp:   time.Now(),
		Category:    category,
		Severity:    severity,
		Component:   component,
		Operation:   operation,
		Message:     message,
		ContextData: context,
	}

	l.metrics.RecentErrors = append(l.metrics.RecentErrors, record)

	// Keep only the most recent errors
	if len(l.metrics.RecentErrors) > l.metrics.MaxRecentErrors {
		l.metrics.RecentErrors = l.metrics.RecentErrors[1:]
	}
}

// GetMetrics returns current error metrics
func (l *GMPHistoryErrorLogger) GetMetrics() *ErrorMetrics {
	return l.metrics
}

// GetMetricsSummary returns a formatted summary of error metrics
func (l *GMPHistoryErrorLogger) GetMetricsSummary() map[string]interface{} {
	return map[string]interface{}{
		"total_errors":       l.metrics.TotalErrors,
		"network_errors":     l.metrics.NetworkErrors,
		"database_errors":    l.metrics.DatabaseErrors,
		"validation_errors":  l.metrics.ValidationErrors,
		"parsing_errors":     l.metrics.ParsingErrors,
		"scraping_errors":    l.metrics.ScrapingErrors,
		"business_errors":    l.metrics.BusinessErrors,
		"system_errors":      l.metrics.SystemErrors,
		"external_errors":    l.metrics.ExternalErrors,
		"critical_errors":    l.metrics.CriticalErrors,
		"high_errors":        l.metrics.HighErrors,
		"medium_errors":      l.metrics.MediumErrors,
		"low_errors":         l.metrics.LowErrors,
		"last_error_time":    l.metrics.LastErrorTime.Format(time.RFC3339),
		"errors_by_category": l.metrics.ErrorsByCategory,
		"errors_by_severity": l.metrics.ErrorsBySeverity,
	}
}

// ResetMetrics resets all error metrics
func (l *GMPHistoryErrorLogger) ResetMetrics() {
	l.metrics = &ErrorMetrics{
		ErrorsByCategory: make(map[ErrorCategory]int),
		ErrorsBySeverity: make(map[ErrorSeverity]int),
		RecentErrors:     make([]ErrorRecord, 0),
		MaxRecentErrors:  100,
	}
}

// GetRecentErrors returns the most recent error records
func (l *GMPHistoryErrorLogger) GetRecentErrors(limit int) []ErrorRecord {
	if limit <= 0 || limit > len(l.metrics.RecentErrors) {
		return l.metrics.RecentErrors
	}

	start := len(l.metrics.RecentErrors) - limit
	return l.metrics.RecentErrors[start:]
}

// GetErrorsByCategory returns errors filtered by category
func (l *GMPHistoryErrorLogger) GetErrorsByCategory(category ErrorCategory) []ErrorRecord {
	filtered := make([]ErrorRecord, 0)
	for _, record := range l.metrics.RecentErrors {
		if record.Category == category {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

// GetErrorsBySeverity returns errors filtered by severity
func (l *GMPHistoryErrorLogger) GetErrorsBySeverity(severity ErrorSeverity) []ErrorRecord {
	filtered := make([]ErrorRecord, 0)
	for _, record := range l.metrics.RecentErrors {
		if record.Severity == severity {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

// ShouldAlert determines if an alert should be triggered based on error metrics
func (l *GMPHistoryErrorLogger) ShouldAlert() (bool, string) {
	// Alert if there are critical errors
	if l.metrics.CriticalErrors > 0 {
		return true, fmt.Sprintf("Critical errors detected: %d", l.metrics.CriticalErrors)
	}

	// Alert if database errors exceed threshold
	if l.metrics.DatabaseErrors > 5 {
		return true, fmt.Sprintf("High database error count: %d", l.metrics.DatabaseErrors)
	}

	// Alert if total errors exceed threshold
	if l.metrics.TotalErrors > 50 {
		return true, fmt.Sprintf("High total error count: %d", l.metrics.TotalErrors)
	}

	// Alert if error rate is high (more than 10 errors in last minute)
	recentErrors := 0
	oneMinuteAgo := time.Now().Add(-1 * time.Minute)
	for _, record := range l.metrics.RecentErrors {
		if record.Timestamp.After(oneMinuteAgo) {
			recentErrors++
		}
	}

	if recentErrors > 10 {
		return true, fmt.Sprintf("High error rate: %d errors in last minute", recentErrors)
	}

	return false, ""
}
