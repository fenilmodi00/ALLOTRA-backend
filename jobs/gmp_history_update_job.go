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
	logger            *logrus.Logger
	stateMu           sync.RWMutex
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
func NewGMPHistoryUpdateJob(db *sql.DB) *GMPHistoryUpdateJob {
	service := services.NewGMPHistoryService(db)
	return NewGMPHistoryUpdateJobWithService(db, service)
}

// NewGMPHistoryUpdateJobWithService creates a job with an injected service instance.
func NewGMPHistoryUpdateJobWithService(db *sql.DB, service *services.GMPHistoryService) *GMPHistoryUpdateJob {
	if service == nil {
		service = services.NewGMPHistoryService(db)
	}

	logger := logrus.New()

	return &GMPHistoryUpdateJob{
		DB:                db,
		GMPHistoryService: service,
		logger:            logger,
		failedIPOs:        make([]FailedIPO, 0),
		lastRunMetrics:    nil,
	}
}

// Run executes a single iteration of the job
// Implements Requirements 4.1, 4.2, 4.4, 4.5, 6.3:
// - 4.1: Scheduled execution via external cron
// - 4.2: Prioritizes active IPOs over closed ones
// - 4.4: Processing metrics collection and reporting
// - 4.5: Comprehensive error logging and job status tracking
// - 6.3: Failure recovery and retry scheduling logic
func (j *GMPHistoryUpdateJob) Run() {
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
		j.logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"duration":   metrics.Duration.String(),
			"job_name":   "GMP History Update",
			"queue_size": metrics.QueueSizeBefore,
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
func (
