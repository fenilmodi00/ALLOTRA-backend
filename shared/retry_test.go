package shared

import (
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestRetryWithExponentialBackoffSuccess tests successful execution without retries
func TestRetryWithExponentialBackoffSuccess(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attemptCount := 0
	fn := func() error {
		attemptCount++
		return nil
	}

	logger := logrus.New()
	err := RetryWithExponentialBackoff(fn, config, logger)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attemptCount != 1 {
		t.Errorf("Expected 1 attempt, got %d", attemptCount)
	}
}

// TestRetryWithExponentialBackoffEventualSuccess tests successful execution after retries
func TestRetryWithExponentialBackoffEventualSuccess(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attemptCount := 0
	testError := errors.New("temporary error")

	fn := func() error {
		attemptCount++
		if attemptCount < 3 {
			return testError
		}
		return nil
	}

	logger := logrus.New()
	startTime := time.Now()
	err := RetryWithExponentialBackoff(fn, config, logger)
	duration := time.Since(startTime)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}

	// Should have waited at least initialDelay + (initialDelay * multiplier)
	// = 10ms + 20ms = 30ms
	expectedMinDelay := 30 * time.Millisecond
	if duration < expectedMinDelay {
		t.Errorf("Expected duration >= %v, got %v", expectedMinDelay, duration)
	}
}

// TestRetryWithExponentialBackoffAllFailures tests failure after all retries exhausted
func TestRetryWithExponentialBackoffAllFailures(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attemptCount := 0
	testError := errors.New("persistent error")

	fn := func() error {
		attemptCount++
		return testError
	}

	logger := logrus.New()
	err := RetryWithExponentialBackoff(fn, config, logger)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}

	// Error should wrap the original error
	if !errors.Is(err, testError) {
		t.Errorf("Expected error to wrap test error, got %v", err)
	}
}

// TestRetryWithExponentialBackoffMaxDelay tests that delay is capped at max delay
func TestRetryWithExponentialBackoffMaxDelay(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     30 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attemptCount := 0
	testError := errors.New("test error")

	fn := func() error {
		attemptCount++
		return testError
	}

	logger := logrus.New()
	startTime := time.Now()
	RetryWithExponentialBackoff(fn, config, logger)
	duration := time.Since(startTime)

	// With exponential backoff: 10ms, 20ms, 40ms (capped to 30ms), 80ms (capped to 30ms)
	// Total: 10 + 20 + 30 + 30 = 90ms
	expectedMinDelay := 90 * time.Millisecond
	expectedMaxDelay := 150 * time.Millisecond // Allow some overhead

	if duration < expectedMinDelay {
		t.Errorf("Expected duration >= %v, got %v", expectedMinDelay, duration)
	}

	if duration > expectedMaxDelay {
		t.Errorf("Expected duration <= %v, got %v", expectedMaxDelay, duration)
	}
}

// TestCalculateBackoffDelay tests backoff delay calculation
func TestCalculateBackoffDelay(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1000 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	tests := []struct {
		attempt     int
		expectedMin time.Duration
		expectedMax time.Duration
		description string
	}{
		{1, 100 * time.Millisecond, 100 * time.Millisecond, "First retry: 100ms"},
		{2, 200 * time.Millisecond, 200 * time.Millisecond, "Second retry: 200ms"},
		{3, 400 * time.Millisecond, 400 * time.Millisecond, "Third retry: 400ms"},
		{4, 800 * time.Millisecond, 800 * time.Millisecond, "Fourth retry: 800ms"},
		{5, 1000 * time.Millisecond, 1000 * time.Millisecond, "Fifth retry: capped at 1000ms"},
	}

	for _, tt := range tests {
		delay := calculateBackoffDelay(tt.attempt, config)

		if delay < tt.expectedMin || delay > tt.expectedMax {
			t.Errorf("%s: expected delay between %v and %v, got %v",
				tt.description, tt.expectedMin, tt.expectedMax, delay)
		}
	}
}

// TestCalculateBackoffDelayWithJitter tests backoff delay calculation with jitter
func TestCalculateBackoffDelayWithJitter(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1000 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       true,
	}

	// Run multiple times to test jitter randomness
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = calculateBackoffDelay(1, config)
	}

	// With jitter, delays should vary (±10% of base delay)
	// Base delay for attempt 1: 100ms
	// With jitter: 90ms to 110ms
	minExpected := 90 * time.Millisecond
	maxExpected := 110 * time.Millisecond

	for i, delay := range delays {
		if delay < minExpected || delay > maxExpected {
			t.Errorf("Delay %d: expected between %v and %v, got %v",
				i, minExpected, maxExpected, delay)
		}
	}
}

// TestDefaultRetryConfig tests default retry configuration
func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts to be 3, got %d", config.MaxAttempts)
	}

	if config.InitialDelay != 1*time.Second {
		t.Errorf("Expected InitialDelay to be 1s, got %v", config.InitialDelay)
	}

	if config.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay to be 30s, got %v", config.MaxDelay)
	}

	if config.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier to be 2.0, got %f", config.Multiplier)
	}

	if !config.Jitter {
		t.Error("Expected Jitter to be true")
	}
}

// TestRetryWithZeroAttempts tests behavior with zero max attempts
func TestRetryWithZeroAttempts(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  0,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attemptCount := 0
	fn := func() error {
		attemptCount++
		return errors.New("test error")
	}

	logger := logrus.New()
	err := RetryWithExponentialBackoff(fn, config, logger)

	// Should fail immediately with configuration error
	if err == nil {
		t.Error("Expected error, got nil")
	}

	if attemptCount != 0 {
		t.Errorf("Expected 0 attempts, got %d", attemptCount)
	}
}

// TestRetryWithOneAttempt tests behavior with single attempt (no retries)
func TestRetryWithOneAttempt(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:  1,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}

	attemptCount := 0
	testError := errors.New("test error")

	fn := func() error {
		attemptCount++
		return testError
	}

	logger := logrus.New()
	startTime := time.Now()
	err := RetryWithExponentialBackoff(fn, config, logger)
	duration := time.Since(startTime)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if attemptCount != 1 {
		t.Errorf("Expected 1 attempt, got %d", attemptCount)
	}

	// Should not have any delay since no retries
	if duration > 50*time.Millisecond {
		t.Errorf("Expected minimal duration, got %v", duration)
	}
}
