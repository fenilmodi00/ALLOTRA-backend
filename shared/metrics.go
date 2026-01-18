package shared

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ServiceMetrics tracks performance and success metrics for services
type ServiceMetrics struct {
	ServiceName           string                 `json:"service_name"`
	TotalRequests         int64                  `json:"total_requests"`
	SuccessfulRequests    int64                  `json:"successful_requests"`
	FailedRequests        int64                  `json:"failed_requests"`
	TotalProcessingTime   time.Duration          `json:"total_processing_time"`
	AverageProcessingTime time.Duration          `json:"average_processing_time"`
	LastUpdated           time.Time              `json:"last_updated"`
	CustomMetrics         map[string]interface{} `json:"custom_metrics"`
	PerformanceMetrics    *PerformanceMetrics    `json:"performance_metrics"`
	mutex                 sync.RWMutex
}

// NewServiceMetrics creates a new metrics tracker for a service
func NewServiceMetrics(serviceName string) *ServiceMetrics {
	return &ServiceMetrics{
		ServiceName:        serviceName,
		LastUpdated:        time.Now(),
		CustomMetrics:      make(map[string]interface{}),
		PerformanceMetrics: NewPerformanceMetrics(),
	}
}

// RecordRequest records a request with its success status and processing time
func (m *ServiceMetrics) RecordRequest(success bool, processingTime time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.TotalRequests++
	m.TotalProcessingTime += processingTime
	m.AverageProcessingTime = time.Duration(int64(m.TotalProcessingTime) / m.TotalRequests)

	if success {
		m.SuccessfulRequests++
	} else {
		m.FailedRequests++
	}

	m.LastUpdated = time.Now()

	// Record performance metrics
	if m.PerformanceMetrics != nil {
		m.PerformanceMetrics.RecordProcessingTime(processingTime)
	}
}

// GetSuccessRate returns the success rate as a percentage
func (m *ServiceMetrics) GetSuccessRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.TotalRequests == 0 {
		return 0.0
	}

	return float64(m.SuccessfulRequests) / float64(m.TotalRequests) * 100.0
}

// GetFailureRate returns the failure rate as a percentage
func (m *ServiceMetrics) GetFailureRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.TotalRequests == 0 {
		return 0.0
	}

	return float64(m.FailedRequests) / float64(m.TotalRequests) * 100.0
}

// SetCustomMetric sets a custom metric value
func (m *ServiceMetrics) SetCustomMetric(key string, value interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.CustomMetrics[key] = value
	m.LastUpdated = time.Now()
}

// GetCustomMetric gets a custom metric value
func (m *ServiceMetrics) GetCustomMetric(key string) (interface{}, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	value, exists := m.CustomMetrics[key]
	return value, exists
}

// IncrementCustomCounter increments a custom counter metric
func (m *ServiceMetrics) IncrementCustomCounter(key string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if current, exists := m.CustomMetrics[key]; exists {
		if counter, ok := current.(int64); ok {
			m.CustomMetrics[key] = counter + 1
		} else {
			m.CustomMetrics[key] = int64(1)
		}
	} else {
		m.CustomMetrics[key] = int64(1)
	}

	m.LastUpdated = time.Now()
}

// GetSnapshot returns a thread-safe snapshot of current metrics
func (m *ServiceMetrics) GetSnapshot() ServiceMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Create a deep copy of custom metrics
	customMetricsCopy := make(map[string]interface{})
	for k, v := range m.CustomMetrics {
		customMetricsCopy[k] = v
	}

	return ServiceMetrics{
		ServiceName:           m.ServiceName,
		TotalRequests:         m.TotalRequests,
		SuccessfulRequests:    m.SuccessfulRequests,
		FailedRequests:        m.FailedRequests,
		TotalProcessingTime:   m.TotalProcessingTime,
		AverageProcessingTime: m.AverageProcessingTime,
		LastUpdated:           m.LastUpdated,
		CustomMetrics:         customMetricsCopy,
		PerformanceMetrics:    m.PerformanceMetrics,
	}
}

// LogSummary logs a comprehensive metrics summary
func (m *ServiceMetrics) LogSummary() {
	snapshot := m.GetSnapshot()
	performanceSnapshot := snapshot.PerformanceMetrics.GetPerformanceSnapshot()

	logrus.WithFields(logrus.Fields{
		"service_name":            snapshot.ServiceName,
		"total_requests":          snapshot.TotalRequests,
		"successful_requests":     snapshot.SuccessfulRequests,
		"failed_requests":         snapshot.FailedRequests,
		"success_rate":            snapshot.GetSuccessRate(),
		"failure_rate":            snapshot.GetFailureRate(),
		"average_processing_time": snapshot.AverageProcessingTime,
		"total_processing_time":   snapshot.TotalProcessingTime,
		"min_processing_time":     performanceSnapshot.MinProcessingTime,
		"max_processing_time":     performanceSnapshot.MaxProcessingTime,
		"p95_processing_time":     performanceSnapshot.P95ProcessingTime,
		"p99_processing_time":     performanceSnapshot.P99ProcessingTime,
		"last_updated":            snapshot.LastUpdated,
		"custom_metrics":          snapshot.CustomMetrics,
	}).Info("Service metrics summary")
}

// Reset resets all metrics to zero
func (m *ServiceMetrics) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.TotalRequests = 0
	m.SuccessfulRequests = 0
	m.FailedRequests = 0
	m.TotalProcessingTime = 0
	m.AverageProcessingTime = 0
	m.LastUpdated = time.Now()
	m.CustomMetrics = make(map[string]interface{})
	m.PerformanceMetrics = NewPerformanceMetrics()

	logrus.WithField("service_name", m.ServiceName).Info("Service metrics reset")
}

// DatabaseMetrics tracks database operation performance and success rates
type DatabaseMetrics struct {
	TotalQueries        int64                  `json:"total_queries"`
	SuccessfulQueries   int64                  `json:"successful_queries"`
	FailedQueries       int64                  `json:"failed_queries"`
	SlowQueries         int64                  `json:"slow_queries"`
	TotalQueryTime      time.Duration          `json:"total_query_time"`
	AverageQueryTime    time.Duration          `json:"average_query_time"`
	ConnectionPoolStats map[string]interface{} `json:"connection_pool_stats"`
	mutex               sync.RWMutex
}

// NewDatabaseMetrics creates a new database metrics tracker
func NewDatabaseMetrics() *DatabaseMetrics {
	return &DatabaseMetrics{
		ConnectionPoolStats: make(map[string]interface{}),
	}
}

// RecordQuery records a database query with its success status and execution time
func (dm *DatabaseMetrics) RecordQuery(success bool, queryTime time.Duration, isSlowQuery bool) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dm.TotalQueries++
	dm.TotalQueryTime += queryTime
	dm.AverageQueryTime = time.Duration(int64(dm.TotalQueryTime) / dm.TotalQueries)

	if success {
		dm.SuccessfulQueries++
	} else {
		dm.FailedQueries++
	}

	if isSlowQuery {
		dm.SlowQueries++
	}
}

// GetQuerySuccessRate returns the query success rate as a percentage
func (dm *DatabaseMetrics) GetQuerySuccessRate() float64 {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	if dm.TotalQueries == 0 {
		return 0.0
	}

	return float64(dm.SuccessfulQueries) / float64(dm.TotalQueries) * 100.0
}

// LogDatabaseSummary logs comprehensive database metrics
func (dm *DatabaseMetrics) LogDatabaseSummary() {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	logrus.WithFields(logrus.Fields{
		"total_queries":         dm.TotalQueries,
		"successful_queries":    dm.SuccessfulQueries,
		"failed_queries":        dm.FailedQueries,
		"slow_queries":          dm.SlowQueries,
		"query_success_rate":    dm.GetQuerySuccessRate(),
		"average_query_time":    dm.AverageQueryTime,
		"total_query_time":      dm.TotalQueryTime,
		"connection_pool_stats": dm.ConnectionPoolStats,
	}).Info("Database metrics summary")
}

// HTTPMetrics tracks HTTP client performance and success rates
type HTTPMetrics struct {
	TotalRequests       int64            `json:"total_requests"`
	SuccessfulRequests  int64            `json:"successful_requests"`
	FailedRequests      int64            `json:"failed_requests"`
	TimeoutRequests     int64            `json:"timeout_requests"`
	RetryAttempts       int64            `json:"retry_attempts"`
	TotalResponseTime   time.Duration    `json:"total_response_time"`
	AverageResponseTime time.Duration    `json:"average_response_time"`
	StatusCodeCounts    map[int]int64    `json:"status_code_counts"`
	ErrorCounts         map[string]int64 `json:"error_counts"`
	mutex               sync.RWMutex
}

// NewHTTPMetrics creates a new HTTP metrics tracker
func NewHTTPMetrics() *HTTPMetrics {
	return &HTTPMetrics{
		StatusCodeCounts: make(map[int]int64),
		ErrorCounts:      make(map[string]int64),
	}
}

// RecordHTTPRequest records an HTTP request with its result
func (hm *HTTPMetrics) RecordHTTPRequest(success bool, statusCode int, responseTime time.Duration, errorType string, isTimeout bool) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	hm.TotalRequests++
	hm.TotalResponseTime += responseTime
	hm.AverageResponseTime = time.Duration(int64(hm.TotalResponseTime) / hm.TotalRequests)

	if success {
		hm.SuccessfulRequests++
	} else {
		hm.FailedRequests++
	}

	if isTimeout {
		hm.TimeoutRequests++
	}

	// Track status codes
	hm.StatusCodeCounts[statusCode]++

	// Track error types
	if errorType != "" {
		hm.ErrorCounts[errorType]++
	}
}

// RecordRetryAttempt records a retry attempt
func (hm *HTTPMetrics) RecordRetryAttempt() {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	hm.RetryAttempts++
}

// GetHTTPSuccessRate returns the HTTP success rate as a percentage
func (hm *HTTPMetrics) GetHTTPSuccessRate() float64 {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	if hm.TotalRequests == 0 {
		return 0.0
	}

	return float64(hm.SuccessfulRequests) / float64(hm.TotalRequests) * 100.0
}

// LogHTTPSummary logs comprehensive HTTP metrics
func (hm *HTTPMetrics) LogHTTPSummary() {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	logrus.WithFields(logrus.Fields{
		"total_requests":        hm.TotalRequests,
		"successful_requests":   hm.SuccessfulRequests,
		"failed_requests":       hm.FailedRequests,
		"timeout_requests":      hm.TimeoutRequests,
		"retry_attempts":        hm.RetryAttempts,
		"http_success_rate":     hm.GetHTTPSuccessRate(),
		"average_response_time": hm.AverageResponseTime,
		"total_response_time":   hm.TotalResponseTime,
		"status_code_counts":    hm.StatusCodeCounts,
		"error_counts":          hm.ErrorCounts,
	}).Info("HTTP metrics summary")
}

// MetricsReporter defines the interface for metrics reporting across all services
type MetricsReporter interface {
	RecordRequest(success bool, processingTime time.Duration)
	RecordCustomMetric(key string, value interface{})
	IncrementCounter(key string)
	GetSuccessRate() float64
	GetFailureRate() float64
	LogSummary()
	Reset()
	GetSnapshot() interface{}
}

// PerformanceMetrics tracks detailed performance measurements
type PerformanceMetrics struct {
	MinProcessingTime time.Duration `json:"min_processing_time"`
	MaxProcessingTime time.Duration `json:"max_processing_time"`
	P95ProcessingTime time.Duration `json:"p95_processing_time"`
	P99ProcessingTime time.Duration `json:"p99_processing_time"`
	RequestsPerSecond float64       `json:"requests_per_second"`
	mutex             sync.RWMutex
	processingTimes   []time.Duration
}

// NewPerformanceMetrics creates a new performance metrics tracker
func NewPerformanceMetrics() *PerformanceMetrics {
	return &PerformanceMetrics{
		processingTimes: make([]time.Duration, 0, 1000), // Pre-allocate for 1000 samples
	}
}

// RecordProcessingTime records a processing time and updates performance metrics
func (pm *PerformanceMetrics) RecordProcessingTime(duration time.Duration) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Update min/max
	if pm.MinProcessingTime == 0 || duration < pm.MinProcessingTime {
		pm.MinProcessingTime = duration
	}
	if duration > pm.MaxProcessingTime {
		pm.MaxProcessingTime = duration
	}

	// Store processing time for percentile calculations (keep last 1000 samples)
	if len(pm.processingTimes) >= 1000 {
		// Remove oldest sample
		pm.processingTimes = pm.processingTimes[1:]
	}
	pm.processingTimes = append(pm.processingTimes, duration)

	// Calculate percentiles
	pm.calculatePercentiles()
}

// calculatePercentiles calculates P95 and P99 processing times
func (pm *PerformanceMetrics) calculatePercentiles() {
	if len(pm.processingTimes) == 0 {
		return
	}

	// Sort processing times for percentile calculation
	times := make([]time.Duration, len(pm.processingTimes))
	copy(times, pm.processingTimes)

	// Simple sort for percentile calculation
	for i := 0; i < len(times); i++ {
		for j := i + 1; j < len(times); j++ {
			if times[i] > times[j] {
				times[i], times[j] = times[j], times[i]
			}
		}
	}

	// Calculate P95 and P99
	p95Index := int(float64(len(times)) * 0.95)
	p99Index := int(float64(len(times)) * 0.99)

	if p95Index < len(times) {
		pm.P95ProcessingTime = times[p95Index]
	}
	if p99Index < len(times) {
		pm.P99ProcessingTime = times[p99Index]
	}
}

// GetPerformanceSnapshot returns a thread-safe snapshot of performance metrics
func (pm *PerformanceMetrics) GetPerformanceSnapshot() PerformanceMetrics {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	return PerformanceMetrics{
		MinProcessingTime: pm.MinProcessingTime,
		MaxProcessingTime: pm.MaxProcessingTime,
		P95ProcessingTime: pm.P95ProcessingTime,
		P99ProcessingTime: pm.P99ProcessingTime,
		RequestsPerSecond: pm.RequestsPerSecond,
	}
}

// ExtractionMetrics tracks success rates and performance of data extraction
type ExtractionMetrics struct {
	DescriptionAttempts int `json:"description_attempts"`
	DescriptionSuccess  int `json:"description_success"`
	AboutAttempts       int `json:"about_attempts"`
	AboutSuccess        int `json:"about_success"`
	HTMLParseErrors     int `json:"html_parse_errors"`
	TextCleaningErrors  int `json:"text_cleaning_errors"`
	mutex               sync.RWMutex
}

// NewExtractionMetrics creates a new extraction metrics tracker
func NewExtractionMetrics() *ExtractionMetrics {
	return &ExtractionMetrics{}
}

// RecordDescriptionAttempt records a description extraction attempt
func (m *ExtractionMetrics) RecordDescriptionAttempt(success bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.DescriptionAttempts++
	if success {
		m.DescriptionSuccess++
	}
}

// RecordAboutAttempt records an about extraction attempt
func (m *ExtractionMetrics) RecordAboutAttempt(success bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.AboutAttempts++
	if success {
		m.AboutSuccess++
	}
}

// RecordHTMLParseError records an HTML parsing error
func (m *ExtractionMetrics) RecordHTMLParseError() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.HTMLParseErrors++
}

// RecordTextCleaningError records a text cleaning error
func (m *ExtractionMetrics) RecordTextCleaningError() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.TextCleaningErrors++
}

// GetDescriptionSuccessRate returns the description extraction success rate
func (m *ExtractionMetrics) GetDescriptionSuccessRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.DescriptionAttempts == 0 {
		return 0.0
	}

	return float64(m.DescriptionSuccess) / float64(m.DescriptionAttempts) * 100.0
}

// GetAboutSuccessRate returns the about extraction success rate
func (m *ExtractionMetrics) GetAboutSuccessRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.AboutAttempts == 0 {
		return 0.0
	}

	return float64(m.AboutSuccess) / float64(m.AboutAttempts) * 100.0
}

// LogSummary logs a comprehensive extraction metrics summary
func (m *ExtractionMetrics) LogSummary() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	descriptionSuccessRate := m.GetDescriptionSuccessRate()
	aboutSuccessRate := m.GetAboutSuccessRate()

	logrus.WithFields(logrus.Fields{
		"description_attempts":     m.DescriptionAttempts,
		"description_success":      m.DescriptionSuccess,
		"description_success_rate": descriptionSuccessRate,
		"about_attempts":           m.AboutAttempts,
		"about_success":            m.AboutSuccess,
		"about_success_rate":       aboutSuccessRate,
		"html_parse_errors":        m.HTMLParseErrors,
		"text_cleaning_errors":     m.TextCleaningErrors,
	}).Info("Extraction metrics summary")
}
