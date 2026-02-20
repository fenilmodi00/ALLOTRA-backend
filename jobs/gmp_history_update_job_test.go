package jobs

import (
	"testing"
	"time"
)

// TestGMPHistoryUpdateJobInitialization verifies the job can be created and configured
func TestGMPHistoryUpdateJobInitialization(t *testing.T) {
	// Test default initialization
	job := NewGMPHistoryUpdateJob(nil)

	if job == nil {
		t.Fatal("Expected job to be initialized, got nil")
	}

	if job.ExecutionInterval != 4*time.Hour {
		t.Errorf("Expected default interval of 4 hours, got %v", job.ExecutionInterval)
	}

	if job.GMPHistoryService == nil {
		t.Error("Expected GMPHistoryService to be initialized")
	}

	if job.stopChan == nil {
		t.Error("Expected stopChan to be initialized")
	}

	if job.failedIPOs == nil {
		t.Error("Expected failedIPOs slice to be initialized")
	}
}

// TestGMPHistoryUpdateJobCustomInterval verifies custom interval configuration
func TestGMPHistoryUpdateJobCustomInterval(t *testing.T) {
	customInterval := 2 * time.Hour
	job := NewGMPHistoryUpdateJobWithInterval(nil, customInterval)

	if job.ExecutionInterval != customInterval {
		t.Errorf("Expected interval of %v, got %v", customInterval, job.ExecutionInterval)
	}
}

// TestGMPHistoryUpdateJobSetInterval verifies interval can be updated
func TestGMPHistoryUpdateJobSetInterval(t *testing.T) {
	job := NewGMPHistoryUpdateJob(nil)

	newInterval := 6 * time.Hour
	job.SetExecutionInterval(newInterval)

	if job.ExecutionInterval != newInterval {
		t.Errorf("Expected interval to be updated to %v, got %v", newInterval, job.ExecutionInterval)
	}
}

// TestGMPHistoryUpdateJobGetters verifies getter methods work correctly
func TestGMPHistoryUpdateJobGetters(t *testing.T) {
	job := NewGMPHistoryUpdateJob(nil)

	// Test GetExecutionInterval
	interval := job.GetExecutionInterval()
	if interval != 4*time.Hour {
		t.Errorf("Expected GetExecutionInterval to return 4 hours, got %v", interval)
	}

	// Test GetNextRunTime (should be approximately 4 hours from now)
	nextRun := job.GetNextRunTime()
	expectedTime := time.Now().Add(4 * time.Hour)
	timeDiff := nextRun.Sub(expectedTime)
	if timeDiff > time.Second || timeDiff < -time.Second {
		t.Errorf("Expected GetNextRunTime to be approximately 4 hours from now, got %v", nextRun)
	}

	// Test GetFailedIPOCount
	count := job.GetFailedIPOCount()
	if count != 0 {
		t.Errorf("Expected GetFailedIPOCount to return 0 initially, got %d", count)
	}

	// Test GetLastRunMetrics (should be nil initially)
	metrics := job.GetLastRunMetrics()
	if metrics != nil {
		t.Error("Expected GetLastRunMetrics to return nil initially")
	}

	// Test GetJobStatus
	status := job.GetJobStatus()
	if status == nil {
		t.Fatal("Expected GetJobStatus to return a map, got nil")
	}

	if status["execution_interval"] != "4h0m0s" {
		t.Errorf("Expected execution_interval in status to be '4h0m0s', got %v", status["execution_interval"])
	}

	if status["failed_ipo_count"] != 0 {
		t.Errorf("Expected failed_ipo_count in status to be 0, got %v", status["failed_ipo_count"])
	}

	if status["is_running"] != false {
		t.Errorf("Expected is_running in status to be false, got %v", status["is_running"])
	}
}

// TestGMPHistoryUpdateJobFailedIPOTracking verifies failed IPO tracking
func TestGMPHistoryUpdateJobFailedIPOTracking(t *testing.T) {
	job := NewGMPHistoryUpdateJob(nil)

	// Add a failed IPO
	job.failedIPOs = append(job.failedIPOs, FailedIPO{
		IPOID:         "test-ipo-1",
		CompanyCode:   "TEST",
		IPOName:       "Test IPO",
		FailureTime:   time.Now(),
		FailureReason: "Test failure",
		RetryCount:    0,
	})

	// Verify count
	if job.GetFailedIPOCount() != 1 {
		t.Errorf("Expected GetFailedIPOCount to return 1, got %d", job.GetFailedIPOCount())
	}

	// Verify list
	failedIPOs := job.GetFailedIPOs()
	if len(failedIPOs) != 1 {
		t.Errorf("Expected GetFailedIPOs to return 1 item, got %d", len(failedIPOs))
	}

	if failedIPOs[0].IPOID != "test-ipo-1" {
		t.Errorf("Expected failed IPO ID to be 'test-ipo-1', got %s", failedIPOs[0].IPOID)
	}

	// Clear failed IPOs
	job.ClearFailedIPOs()
	if job.GetFailedIPOCount() != 0 {
		t.Errorf("Expected GetFailedIPOCount to return 0 after clearing, got %d", job.GetFailedIPOCount())
	}
}
