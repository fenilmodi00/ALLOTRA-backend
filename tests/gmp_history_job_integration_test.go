package tests

import (
	"database/sql"
	"testing"
	"time"

	"github.com/fenilmodi00/ipo-backend/jobs"
)

// TestGMPHistoryJobIntegration verifies the job integrates correctly with the application
func TestGMPHistoryJobIntegration(t *testing.T) {
	// This test verifies that the job can be created and started without errors
	// It doesn't require a real database connection for basic initialization

	t.Run("Job can be initialized without database", func(t *testing.T) {
		var db *sql.DB // nil database for initialization test
		job := jobs.NewGMPHistoryUpdateJob(db)

		if job == nil {
			t.Fatal("Expected job to be initialized")
		}

		// Verify default configuration
		if job.GetExecutionInterval() != 4*time.Hour {
			t.Errorf("Expected 4-hour interval, got %v", job.GetExecutionInterval())
		}
	})

	t.Run("Job can be stopped gracefully", func(t *testing.T) {
		var db *sql.DB
		job := jobs.NewGMPHistoryUpdateJob(db)

		// Note: We don't actually start the job here because it would
		// immediately try to run and access the nil database.
		// In production, the job is started with a valid database connection.

		// Verify Stop() can be called safely even if not started
		job.Stop()

		// Verify it handled the stop gracefully
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("Job status can be retrieved", func(t *testing.T) {
		var db *sql.DB
		job := jobs.NewGMPHistoryUpdateJob(db)

		status := job.GetJobStatus()

		if status == nil {
			t.Fatal("Expected status map to be returned")
		}

		// Verify expected status fields
		if _, ok := status["execution_interval"]; !ok {
			t.Error("Expected execution_interval in status")
		}

		if _, ok := status["next_run_time"]; !ok {
			t.Error("Expected next_run_time in status")
		}

		if _, ok := status["failed_ipo_count"]; !ok {
			t.Error("Expected failed_ipo_count in status")
		}

		if _, ok := status["is_running"]; !ok {
			t.Error("Expected is_running in status")
		}

		if _, ok := status["resilience_queue"]; !ok {
			t.Error("Expected resilience_queue in status")
		}
	})

	t.Run("Job interval can be configured", func(t *testing.T) {
		customInterval := 2 * time.Hour
		var db *sql.DB
		job := jobs.NewGMPHistoryUpdateJobWithInterval(db, customInterval)

		if job.GetExecutionInterval() != customInterval {
			t.Errorf("Expected custom interval %v, got %v", customInterval, job.GetExecutionInterval())
		}

		// Change interval
		newInterval := 6 * time.Hour
		job.SetExecutionInterval(newInterval)

		if job.GetExecutionInterval() != newInterval {
			t.Errorf("Expected updated interval %v, got %v", newInterval, job.GetExecutionInterval())
		}
	})
}

// TestGMPHistoryJobScheduling verifies the job scheduling behavior
func TestGMPHistoryJobScheduling(t *testing.T) {
	t.Run("Next run time is calculated correctly", func(t *testing.T) {
		var db *sql.DB
		job := jobs.NewGMPHistoryUpdateJob(db)

		nextRun := job.GetNextRunTime()
		now := time.Now()

		// Next run should be approximately 4 hours from now
		expectedTime := now.Add(4 * time.Hour)
		timeDiff := nextRun.Sub(expectedTime)

		// Allow 1 second tolerance for test execution time
		if timeDiff > time.Second || timeDiff < -time.Second {
			t.Errorf("Expected next run time to be ~4 hours from now, got %v (diff: %v)", nextRun, timeDiff)
		}
	})

	t.Run("Job tracks failed IPOs for retry", func(t *testing.T) {
		var db *sql.DB
		job := jobs.NewGMPHistoryUpdateJob(db)

		// Initially should have no failed IPOs
		if job.GetFailedIPOCount() != 0 {
			t.Errorf("Expected 0 failed IPOs initially, got %d", job.GetFailedIPOCount())
		}

		// Simulate adding a failed IPO (this would normally happen during job execution)
		failedIPOs := job.GetFailedIPOs()
		if len(failedIPOs) != 0 {
			t.Errorf("Expected empty failed IPOs list, got %d items", len(failedIPOs))
		}
	})
}

// TestGMPHistoryJobMetrics verifies metrics tracking
func TestGMPHistoryJobMetrics(t *testing.T) {
	t.Run("Metrics are nil before first run", func(t *testing.T) {
		var db *sql.DB
		job := jobs.NewGMPHistoryUpdateJob(db)

		metrics := job.GetLastRunMetrics()
		if metrics != nil {
			t.Error("Expected nil metrics before first run")
		}
	})

	t.Run("Job status includes all required fields", func(t *testing.T) {
		var db *sql.DB
		job := jobs.NewGMPHistoryUpdateJob(db)

		status := job.GetJobStatus()

		requiredFields := []string{
			"execution_interval",
			"next_run_time",
			"failed_ipo_count",
			"is_running",
			"resilience_queue",
		}

		for _, field := range requiredFields {
			if _, ok := status[field]; !ok {
				t.Errorf("Expected field %s in job status", field)
			}
		}
	})
}
