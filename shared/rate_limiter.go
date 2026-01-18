package shared

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HTTPRequestRateLimiter implements thread-safe rate limiting for HTTP requests
type HTTPRequestRateLimiter struct {
	minimumDelay    time.Duration // Minimum delay between requests
	lastRequestTime time.Time     // Timestamp of the last request
	mutex           sync.Mutex    // Ensures thread-safe access
	requestCount    int64         // Total number of requests processed
}

// NewHTTPRequestRateLimiter creates a new rate limiter with the specified minimum delay
func NewHTTPRequestRateLimiter(minimumDelay time.Duration) *HTTPRequestRateLimiter {
	return &HTTPRequestRateLimiter{
		minimumDelay:    minimumDelay,
		lastRequestTime: time.Now(),
		requestCount:    0,
	}
}

// EnforceRateLimit blocks execution until the minimum delay has elapsed since the last request
func (limiter *HTTPRequestRateLimiter) EnforceRateLimit() {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	elapsedTime := time.Since(limiter.lastRequestTime)
	if elapsedTime < limiter.minimumDelay {
		remainingDelay := limiter.minimumDelay - elapsedTime

		logrus.WithFields(logrus.Fields{
			"component":       "HTTPRequestRateLimiter",
			"elapsed_time":    elapsedTime,
			"minimum_delay":   limiter.minimumDelay,
			"remaining_delay": remainingDelay,
			"request_count":   limiter.requestCount + 1,
		}).Debug("Enforcing rate limit delay")

		time.Sleep(remainingDelay)
	}

	limiter.lastRequestTime = time.Now()
	limiter.requestCount++
}

// GetRequestCount returns the total number of requests processed
func (limiter *HTTPRequestRateLimiter) GetRequestCount() int64 {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	return limiter.requestCount
}

// GetLastRequestTime returns the timestamp of the last request
func (limiter *HTTPRequestRateLimiter) GetLastRequestTime() time.Time {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()
	return limiter.lastRequestTime
}

// UpdateMinimumDelay updates the minimum delay between requests
func (limiter *HTTPRequestRateLimiter) UpdateMinimumDelay(newDelay time.Duration) {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	oldDelay := limiter.minimumDelay
	limiter.minimumDelay = newDelay

	logrus.WithFields(logrus.Fields{
		"component": "HTTPRequestRateLimiter",
		"old_delay": oldDelay,
		"new_delay": newDelay,
	}).Info("Updated rate limiter minimum delay")
}

// Reset resets the rate limiter state
func (limiter *HTTPRequestRateLimiter) Reset() {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	limiter.lastRequestTime = time.Now()
	limiter.requestCount = 0

	logrus.WithField("component", "HTTPRequestRateLimiter").Debug("Reset rate limiter state")
}
