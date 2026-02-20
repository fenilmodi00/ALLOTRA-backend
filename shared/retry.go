package shared

import (
	"fmt"
	"math"
	"time"

	"github.com/sirupsen/logrus"
)

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts (including the initial attempt)
	MaxAttempts int
	// InitialDelay is the initial delay before the first retry
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// Multiplier is the factor by which the delay increases after each retry
	Multiplier float64
	// Jitter adds randomness to prevent thundering herd
	Jitter bool
}

// DefaultRetryConfig returns default retry configuration
// Implements Requirement 6.1 - Retry up to 3 times with exponential backoff
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// RetryWithExponentialBackoff executes a function with exponential backoff retry logic
// Implements Requirement 6.1 - Network retry with exponential backoff
func RetryWithExponentialBackoff(fn RetryableFunc, config RetryConfig, logger *logrus.Logger) error {
	// Handle edge case: no attempts allowed
	if config.MaxAttempts <= 0 {
		return fmt.Errorf("no retry attempts configured (MaxAttempts: %d)", config.MaxAttempts)
	}

	var lastErr error

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Execute the function
		err := fn()
		if err == nil {
			// Success
			if attempt > 1 && logger != nil {
				logger.WithFields(logrus.Fields{
					"component": "RetryWithExponentialBackoff",
					"attempt":   attempt,
				}).Info("Function succeeded after retry")
			}
			return nil
		}

		// Record the error
		lastErr = err

		// If this was the last attempt, don't sleep
		if attempt == config.MaxAttempts {
			break
		}

		// Calculate backoff delay
		delay := calculateBackoffDelay(attempt, config)

		if logger != nil {
			logger.WithFields(logrus.Fields{
				"component":    "RetryWithExponentialBackoff",
				"attempt":      attempt,
				"max_attempts": config.MaxAttempts,
				"delay":        delay,
				"error":        err.Error(),
			}).Warn("Function failed, retrying after backoff")
		}

		// Sleep before next attempt
		time.Sleep(delay)
	}

	// All attempts failed
	if logger != nil {
		logger.WithFields(logrus.Fields{
			"component":    "RetryWithExponentialBackoff",
			"max_attempts": config.MaxAttempts,
			"final_error":  lastErr.Error(),
		}).Error("Function failed after all retry attempts")
	}

	return fmt.Errorf("failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// calculateBackoffDelay calculates the delay for the next retry attempt
func calculateBackoffDelay(attempt int, config RetryConfig) time.Duration {
	// Calculate exponential delay: initialDelay * multiplier^(attempt-1)
	delay := float64(config.InitialDelay) * math.Pow(config.Multiplier, float64(attempt-1))

	// Cap at max delay
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	// Add jitter if enabled (±10% randomness)
	if config.Jitter {
		jitterFactor := 0.9 + (0.2 * float64(time.Now().UnixNano()%100) / 100.0)
		delay = delay * jitterFactor
	}

	return time.Duration(delay)
}
