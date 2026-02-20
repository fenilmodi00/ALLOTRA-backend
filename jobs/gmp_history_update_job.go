package jobs

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/sirupsen/logrus"
)

// GMPHistoryUpdateJob handles automated collection of historical GMP data for active IPOs
// Implements Requirements 4.1, 4.2, 4.4, 4.5, 6.3 from gmp-price-history spec
type GMPHistoryUpdateJob struct {
	DB                *sql.DB
	GMPHistoryService *services.GMPHistoryService
	errorLogger       *services.GMPHistoryErrorLogger
	ExecutionInterval time.Duration
	ticker            *time.Ticker
	stopChan          chan bool
	stateMu           sync.RWMutex
	runMu             sync.Mutex
	isRunning         bool
	failedIPOs        []FailedIPO // Track failed IPOs for retry scheduling
	lastRunMetrics    *JobMetrics // Store metrics from last run
}

// FailedIPO tracks IPOs that failed processing for retry scheduling
// Implements Requirement 6.3 - Failure recovery and retry scheduling
type FailedIPO struct {
	IPOID         string
	CompanyCode   string
	IPOName       string
	FailureTime   time.Time
	FailureReason string
	RetryCount    int
}

// JobMetrics tracks comprehensive execution metrics
// Implements Requirement 4.4 - Processing metrics collection and reporting
type JobMetrics struct {
	JobStartTime         time.Time
	JobEndTime           time.Time
	Duration             time.Duration
	TotalIPOs            int
	SuccessfulIPOs       int
	FailedIPOs           int
	TotalRecordsAdded    int
	AvgRecordsPerIPO     float64
	AvgProcessingTimeIPO time.Duration
	SuccessRate          float64
	QueueSizeBefore      int
	QueueSizeAfter       int
	QueueItemsProcessed  int
	ErrorSummary         []string
}

// NewGMPHistoryUpdateJob creates a new GMP history update job instance
// Default execution interval is 4 hours as per Requirement 4.1
func NewGMPHistoryUpdateJob(db *sql.DB) *GMPHistoryUpdateJob {
	service := services.NewGMPHistoryService(db)
	return NewGMPHistoryUpdateJobWithService(db, service)
}

// NewGMPHistoryUpdateJobWithService creates a job with an injected service instance.
func NewGMPHistoryUpdateJobWithService(db *sql.DB, service *services.GMPHistoryService) *GMPHistoryUpdateJob {
	if service == nil {
		service = services.NewGMPHistoryService(db)
	}

	return &GMPHistoryUpdateJob{
		DB:                db,
		GMPHistoryService: service,
		errorLogger:       service.GetErrorLogger(),
		ExecutionInterval: 4 * time.Hour, // Requirement 4.1: Run every 4 hours
		stopChan:          make(chan bool),
		failedIPOs:        make([]FailedIPO, 0),
		lastRunMetrics:    nil,
	}
}

// NewGMPHistoryUpdateJobWithInterval creates a job with a custom execution interval
// Useful for testing or custom deployment configurations
func NewGMPHistoryUpdateJobWithInterval(db *sql.DB, interval time.Duration) *GMPHistoryUpdateJob {
	service := services.NewGMPHistoryService(db)
	return &GMPHistoryUpdateJob{
		DB:                db,
		GMPHistoryService: service,
		errorLogger:       service.GetErrorLogger(),
		ExecutionInterval: interval,
		stopChan:          make(chan bool),
		failedIPOs:        make([]FailedIPO, 0),
		lastRunMetrics:    nil,
	}
}

// Start begins the background job with scheduled execution
// The job runs immediately on start, then repeats at the configured interval
func (j *GMPHistoryUpdateJob) Start() {
	logrus.WithField("interval", j.ExecutionInterval.String()).Info("Starting GMP History Update Job...")

	j.ticker = time.NewTicker(j.ExecutionInterval)

	go func() {
		// Run immediately on start
		j.Run()

		// Then run on schedule
		for {
			select {
			case <-j.ticker.C:
				j.Run()
			case <-j.stopChan:
				logrus.Info("GMP History Update Job stopped")
				return
			}
		}
	}()
}

// Stop gracefully stops the background job
func (j *GMPHistoryUpdateJob) Stop() {
	if j.ticker != nil {
		j.ticker.Stop()
	}

	// Signal the goroutine to stop
	select {
	case j.stopChan <- true:
	default:
		// Channel already closed or stop already sent
	}

	// Close the service and its background workers
	if j.GMPHistoryService != nil {
		j.GMPHistoryService.Close()
	}

	logrus.Info("GMP History Update Job shutdown complete")
}

// Run executes a single iteration of the job
// Implements Requirements 4.1, 4.2, 4.4, 4.5, 6.3:
// - 4.1: Scheduled execution every 4 hours
// - 4.2: Prioritizes active IPOs over closed ones
// - 4.4: Processing metrics collection and reporting
// - 4.5: Comprehensive error logging and job status tracking
// - 6.3: Failure recovery and retry scheduling logic
func (j *GMPHistoryUpdateJob) Run() {
	j.runMu.Lock()
	if j.isRunning {
		j.runMu.Unlock()
		logrus.Warn("Skipping GMP History Update Job run; previous run still active")
		return
	}
	j.isRunning = true
	j.runMu.Unlock()

	defer func() {
		j.runMu.Lock()
		j.isRunning = false
		j.runMu.Unlock()
	}()

	startTime := time.Now()

	logrus.WithFields(logrus.Fields{
		"job_name":   "GMP History Update",
		"start_time": startTime.Format(time.RFC3339),
	}).Info("Running GMP History Update Job...")

	// Initialize metrics tracking (Requirement 4.4)
	metrics := &JobMetrics{
		JobStartTime: startTime,
		ErrorSummary: make([]string, 0),
	}

	// Get resilience queue metrics before processing
	queueMetricsBefore := j.GMPHistoryService.GetResilienceQueueMetrics()
	metrics.QueueSizeBefore = j.GMPHistoryService.GetResilienceQueueSize()

	if metrics.QueueSizeBefore > 0 {
		logrus.WithFields(logrus.Fields{
			"queue_size": metrics.QueueSizeBefore,
			"metrics":    queueMetricsBefore,
		}).Info("Resilience queue has pending items from previous failures")
	}

	// Requirement 6.3: Retry failed IPOs from previous runs before processing new ones
	failedCount := j.GetFailedIPOCount()
	if failedCount > 0 {
		logrus.WithField("failed_ipo_count", failedCount).Info("Retrying previously failed IPOs")
		j.retryFailedIPOs(metrics)
	}

	// Process all active IPOs with prioritization (Requirement 4.2)
	// The service handles:
	// - IPO prioritization (LIVE > UPCOMING > CLOSED)
	// - Error isolation (continues on individual failures)
	// - Metrics tracking (success rate, processing time, error counts)
	processingResults, err := j.GMPHistoryService.ProcessAllActiveIPOHistory()

	// Calculate job duration
	metrics.JobEndTime = time.Now()
	metrics.Duration = metrics.JobEndTime.Sub(metrics.JobStartTime)

	// Requirement 4.5: Comprehensive error logging and job status tracking
	if err != nil {
		if j.errorLogger != nil {
			j.errorLogger.LogError(
				services.ErrorCategorySystem,
				services.ErrorSeverityCritical,
				"GMPHistoryUpdateJob",
				"Run.ProcessAllActiveIPOHistory",
				err,
				map[string]interface{}{
					"duration":   metrics.Duration.String(),
					"job_name":   "GMP History Update",
					"queue_size": metrics.QueueSizeBefore,
				},
			)
		}
		logrus.WithFields(logrus.Fields{
			"error":    err.Error(),
			"duration": metrics.Duration.String(),
			"job_name": "GMP History Update",
		}).Error("GMP History Update Job failed with critical error")

		// Store metrics even on failure
		j.stateMu.Lock()
		j.lastRunMetrics = metrics
		j.stateMu.Unlock()
		return
	}

	// Get resilience queue metrics after processing
	queueMetricsAfter := j.GMPHistoryService.GetResilienceQueueMetrics()
	metrics.QueueSizeAfter = j.GMPHistoryService.GetResilienceQueueSize()
	metrics.QueueItemsProcessed = metrics.QueueSizeBefore - metrics.QueueSizeAfter

	// Requirement 6.3: Update failed IPOs list for retry scheduling
	// Convert failed processing results to FailedIPO entries
	for _, failedResult := range processingResults.FailedIPOs {
		j.stateMu.Lock()
		j.failedIPOs = append(j.failedIPOs, FailedIPO{
			IPOID:         failedResult.IPOID,
			CompanyCode:   failedResult.CompanyCode,
			IPOName:       failedResult.IPOName,
			FailureTime:   time.Now(),
			FailureReason: failedResult.ErrorMessage,
			RetryCount:    0,
		})
		j.stateMu.Unlock()
	}

	// Calculate comprehensive job statistics (Requirement 4.4)
	metrics.TotalIPOs = processingResults.TotalProcessed
	metrics.SuccessfulIPOs = processingResults.SuccessCount
	metrics.FailedIPOs = processingResults.FailureCount
	metrics.TotalRecordsAdded = 0
	totalProcessingTime := time.Duration(0)

	for _, result := range processingResults.SuccessfulIPOs {
		metrics.TotalRecordsAdded += result.TotalRecords
		if result.Metadata != nil && result.Metadata.ProcessingTime != "" {
			if duration, err := time.ParseDuration(result.Metadata.ProcessingTime); err == nil {
				totalProcessingTime += duration
			}
		}
	}

	// Calculate averages
	if metrics.TotalIPOs > 0 {
		metrics.AvgRecordsPerIPO = float64(metrics.TotalRecordsAdded) / float64(metrics.TotalIPOs)
		if metrics.SuccessfulIPOs > 0 {
			metrics.AvgProcessingTimeIPO = totalProcessingTime / time.Duration(metrics.SuccessfulIPOs)
		}
		metrics.SuccessRate = float64(metrics.SuccessfulIPOs) / float64(metrics.TotalIPOs) * 100
	}

	// Store metrics for monitoring
	j.stateMu.Lock()
	j.lastRunMetrics = metrics
	j.stateMu.Unlock()

	// Requirement 4.4: Log comprehensive job completion summary with detailed metrics
	logFields := logrus.Fields{
		"job_name":                "GMP History Update",
		"duration":                metrics.Duration.String(),
		"total_ipos_processed":    metrics.TotalIPOs,
		"successful_ipos":         metrics.SuccessfulIPOs,
		"failed_ipos":             metrics.FailedIPOs,
		"success_rate":            fmt.Sprintf("%.2f%%", metrics.SuccessRate),
		"total_records_added":     metrics.TotalRecordsAdded,
		"avg_records_per_ipo":     fmt.Sprintf("%.2f", metrics.AvgRecordsPerIPO),
		"avg_processing_time_ipo": metrics.AvgProcessingTimeIPO.String(),
		"queue_size_before":       metrics.QueueSizeBefore,
		"queue_size_after":        metrics.QueueSizeAfter,
		"queue_items_processed":   metrics.QueueItemsProcessed,
		"next_run":                time.Now().Add(j.ExecutionInterval).Format(time.RFC3339),
	}

	// Add resilience queue metrics if available
	if queueEnabled, ok := queueMetricsAfter["enabled"].(bool); ok && queueEnabled {
		logFields["resilience_queue_enabled"] = true
		if metrics.QueueSizeAfter > 0 {
			logFields["resilience_queue_status"] = "has_pending_items"
		} else {
			logFields["resilience_queue_status"] = "empty"
		}
	}

	// Add retry information if applicable (Requirement 6.3)
	pendingRetries := j.GetFailedIPOCount()
	if pendingRetries > 0 {
		logFields["pending_retries"] = pendingRetries
		logFields["retry_scheduled_for"] = "next_run"
	}

	logrus.WithFields(logFields).Info("GMP History Update Job completed successfully")

	// Requirement 6.3: Log warnings for operational issues
	j.logOperationalWarnings(metrics)
}

// retryFailedIPOs attempts to reprocess IPOs that failed in previous runs
// Implements Requirement 6.3 - Failure recovery and retry scheduling
func (j *GMPHistoryUpdateJob) retryFailedIPOs(metrics *JobMetrics) {
	retryStartTime := time.Now()
	retriedIPOs := make([]FailedIPO, 0)
	stillFailedIPOs := make([]FailedIPO, 0)

	j.stateMu.RLock()
	failedSnapshot := make([]FailedIPO, len(j.failedIPOs))
	copy(failedSnapshot, j.failedIPOs)
	j.stateMu.RUnlock()

	logrus.WithField("retry_count", len(failedSnapshot)).Info("Starting retry of failed IPOs")

	for _, failedIPO := range failedSnapshot {
		// Skip if retry count exceeds maximum (3 retries)
		if failedIPO.RetryCount >= 3 {
			logrus.WithFields(logrus.Fields{
				"ipo_id":       failedIPO.IPOID,
				"ipo_name":     failedIPO.IPOName,
				"retry_count":  failedIPO.RetryCount,
				"last_failure": failedIPO.FailureReason,
			}).Warn("IPO exceeded maximum retry count, skipping")
			continue
		}

		logrus.WithFields(logrus.Fields{
			"ipo_id":       failedIPO.IPOID,
			"company_code": failedIPO.CompanyCode,
			"retry_count":  failedIPO.RetryCount + 1,
		}).Info("Retrying failed IPO")

		// Attempt to scrape and save
		collection, err := j.GMPHistoryService.ScrapeIPOPriceHistoryWithName(failedIPO.IPOID, failedIPO.CompanyCode, failedIPO.IPOName)
		if err != nil {
			// Still failing, increment retry count and keep in failed list
			failedIPO.RetryCount++
			failedIPO.FailureTime = time.Now()
			failedIPO.FailureReason = err.Error()
			stillFailedIPOs = append(stillFailedIPOs, failedIPO)

			metrics.FailedIPOs++
			metrics.ErrorSummary = append(metrics.ErrorSummary,
				fmt.Sprintf("Retry failed for %s: %v", failedIPO.IPOName, err))

			if j.errorLogger != nil {
				j.errorLogger.LogScrapingError(
					"GMPHistoryUpdateJob",
					"retryFailedIPOs.Scrape",
					err,
					map[string]interface{}{
						"ipo_id":       failedIPO.IPOID,
						"ipo_name":     failedIPO.IPOName,
						"company_code": failedIPO.CompanyCode,
						"retry_count":  failedIPO.RetryCount,
					},
				)
			} else {
				logrus.WithFields(logrus.Fields{
					"ipo_id": failedIPO.IPOID,
					"error":  err.Error(),
				}).Error("Retry failed for IPO")
			}
			continue
		}

		// Save to database
		err = j.GMPHistoryService.SavePriceHistory(collection)
		if err != nil {
			// Save failed, keep in failed list
			failedIPO.RetryCount++
			failedIPO.FailureTime = time.Now()
			failedIPO.FailureReason = err.Error()
			stillFailedIPOs = append(stillFailedIPOs, failedIPO)

			metrics.FailedIPOs++
			metrics.ErrorSummary = append(metrics.ErrorSummary,
				fmt.Sprintf("Retry save failed for %s: %v", failedIPO.IPOName, err))

			logrus.WithFields(logrus.Fields{
				"ipo_id": failedIPO.IPOID,
				"error":  err.Error(),
			}).Error("Retry save failed for IPO")
			continue
		}

		// Success! Remove from failed list
		retriedIPOs = append(retriedIPOs, failedIPO)
		metrics.SuccessfulIPOs++
		metrics.TotalRecordsAdded += collection.TotalRecords

		logrus.WithFields(logrus.Fields{
			"ipo_id":        failedIPO.IPOID,
			"records_added": collection.TotalRecords,
			"retry_count":   failedIPO.RetryCount + 1,
		}).Info("Successfully retried failed IPO")
	}

	// Update failed IPOs list
	j.stateMu.Lock()
	j.failedIPOs = stillFailedIPOs
	remaining := len(j.failedIPOs)
	j.stateMu.Unlock()

	retryDuration := time.Since(retryStartTime)

	logrus.WithFields(logrus.Fields{
		"total_retried":    len(retriedIPOs) + len(stillFailedIPOs),
		"successful":       len(retriedIPOs),
		"still_failed":     len(stillFailedIPOs),
		"retry_duration":   retryDuration.String(),
		"pending_for_next": remaining,
	}).Info("Completed retry of failed IPOs")
}

// logOperationalWarnings logs warnings for operational issues
// Implements Requirement 4.5 - Comprehensive error logging
func (j *GMPHistoryUpdateJob) logOperationalWarnings(metrics *JobMetrics) {
	// Log warning if queue size increased (indicates database issues)
	if metrics.QueueSizeAfter > metrics.QueueSizeBefore {
		logrus.WithFields(logrus.Fields{
			"queue_size_increase": metrics.QueueSizeAfter - metrics.QueueSizeBefore,
			"queue_size_before":   metrics.QueueSizeBefore,
			"queue_size_after":    metrics.QueueSizeAfter,
		}).Warn("Resilience queue size increased - possible database connectivity issues")
	}

	// Log success if queue was drained
	if metrics.QueueSizeBefore > 0 && metrics.QueueSizeAfter == 0 {
		logrus.WithField("items_processed", metrics.QueueSizeBefore).Info("Resilience queue successfully drained")
	}

	// Log warning if success rate is low
	if metrics.SuccessRate < 80.0 && metrics.TotalIPOs > 0 {
		logrus.WithFields(logrus.Fields{
			"success_rate":    fmt.Sprintf("%.2f%%", metrics.SuccessRate),
			"failed_ipos":     metrics.FailedIPOs,
			"successful_ipos": metrics.SuccessfulIPOs,
		}).Warn("Low success rate detected - check error logs for details")
	}

	// Log warning if no records were added
	if metrics.TotalRecordsAdded == 0 && metrics.TotalIPOs > 0 {
		logrus.WithField("ipos_processed", metrics.TotalIPOs).Warn("No records added despite processing IPOs - possible data source issues")
	}

	// Log warning if job took too long
	if metrics.Duration > 30*time.Minute {
		logrus.WithFields(logrus.Fields{
			"duration":   metrics.Duration.String(),
			"ipos_count": metrics.TotalIPOs,
		}).Warn("Job execution took longer than expected - consider optimization")
	}
}

// GetNextRunTime returns the scheduled time for the next job execution
func (j *GMPHistoryUpdateJob) GetNextRunTime() time.Time {
	return time.Now().Add(j.ExecutionInterval)
}

// GetExecutionInterval returns the configured execution interval
func (j *GMPHistoryUpdateJob) GetExecutionInterval() time.Duration {
	return j.ExecutionInterval
}

// SetExecutionInterval updates the execution interval
// Note: This will take effect on the next scheduled run
func (j *GMPHistoryUpdateJob) SetExecutionInterval(interval time.Duration) {
	j.ExecutionInterval = interval

	// If the job is already running, restart the ticker
	if j.ticker != nil {
		j.ticker.Stop()
		j.ticker = time.NewTicker(interval)
	}

	logrus.WithField("new_interval", interval.String()).Info("GMP History Update Job interval updated")
}

// GetLastRunMetrics returns metrics from the last job execution
// Implements Requirement 4.4 - Processing metrics collection and reporting
func (j *GMPHistoryUpdateJob) GetLastRunMetrics() *JobMetrics {
	j.stateMu.RLock()
	defer j.stateMu.RUnlock()

	if j.lastRunMetrics == nil {
		return nil
	}

	copyMetrics := *j.lastRunMetrics
	if len(j.lastRunMetrics.ErrorSummary) > 0 {
		copyMetrics.ErrorSummary = append([]string(nil), j.lastRunMetrics.ErrorSummary...)
	}

	return &copyMetrics

}

// GetFailedIPOCount returns the number of IPOs pending retry
// Implements Requirement 6.3 - Failure recovery tracking
func (j *GMPHistoryUpdateJob) GetFailedIPOCount() int {
	j.stateMu.RLock()
	defer j.stateMu.RUnlock()
	return len(j.failedIPOs)
}

// GetFailedIPOs returns the list of IPOs pending retry
// Implements Requirement 6.3 - Failure recovery tracking
func (j *GMPHistoryUpdateJob) GetFailedIPOs() []FailedIPO {
	j.stateMu.RLock()
	defer j.stateMu.RUnlock()

	copyFailed := make([]FailedIPO, len(j.failedIPOs))
	copy(copyFailed, j.failedIPOs)
	return copyFailed
}

// ClearFailedIPOs clears the list of failed IPOs
// Useful for manual intervention or testing
func (j *GMPHistoryUpdateJob) ClearFailedIPOs() {
	j.stateMu.Lock()
	j.failedIPOs = make([]FailedIPO, 0)
	j.stateMu.Unlock()
	logrus.Info("Cleared failed IPOs list")
}

// GetJobStatus returns a comprehensive status report of the job
// Implements Requirement 4.4 - Processing metrics collection and reporting
func (j *GMPHistoryUpdateJob) GetJobStatus() map[string]interface{} {
	status := map[string]interface{}{
		"execution_interval": j.ExecutionInterval.String(),
		"next_run_time":      j.GetNextRunTime().Format(time.RFC3339),
		"failed_ipo_count":   j.GetFailedIPOCount(),
		"is_running":         j.ticker != nil,
	}

	if j.lastRunMetrics != nil {
		status["last_run"] = map[string]interface{}{
			"start_time":              j.lastRunMetrics.JobStartTime.Format(time.RFC3339),
			"end_time":                j.lastRunMetrics.JobEndTime.Format(time.RFC3339),
			"duration":                j.lastRunMetrics.Duration.String(),
			"total_ipos":              j.lastRunMetrics.TotalIPOs,
			"successful_ipos":         j.lastRunMetrics.SuccessfulIPOs,
			"failed_ipos":             j.lastRunMetrics.FailedIPOs,
			"success_rate":            fmt.Sprintf("%.2f%%", j.lastRunMetrics.SuccessRate),
			"total_records_added":     j.lastRunMetrics.TotalRecordsAdded,
			"avg_records_per_ipo":     fmt.Sprintf("%.2f", j.lastRunMetrics.AvgRecordsPerIPO),
			"avg_processing_time_ipo": j.lastRunMetrics.AvgProcessingTimeIPO.String(),
			"queue_size_before":       j.lastRunMetrics.QueueSizeBefore,
			"queue_size_after":        j.lastRunMetrics.QueueSizeAfter,
			"queue_items_processed":   j.lastRunMetrics.QueueItemsProcessed,
		}
	}

	// Add resilience queue status
	queueMetrics := j.GMPHistoryService.GetResilienceQueueMetrics()
	status["resilience_queue"] = queueMetrics

	return status
}
