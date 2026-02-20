package shared

import (
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// CircuitState represents the current state of the circuit breaker
type CircuitState int

const (
	// StateClosed - Circuit is closed, requests are allowed
	StateClosed CircuitState = iota
	// StateOpen - Circuit is open, requests are blocked
	StateOpen
	// StateHalfOpen - Circuit is testing if service has recovered
	StateHalfOpen
)

// String returns the string representation of the circuit state
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig holds configuration for the circuit breaker
type CircuitBreakerConfig struct {
	// MaxFailures is the number of consecutive failures before opening the circuit
	MaxFailures int
	// Timeout is the duration to wait before attempting to close the circuit
	Timeout time.Duration
	// HalfOpenMaxRequests is the number of requests allowed in half-open state
	HalfOpenMaxRequests int
}

// DefaultCircuitBreakerConfig returns default configuration
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		MaxFailures:         5,
		Timeout:             60 * time.Second,
		HalfOpenMaxRequests: 3,
	}
}

// CircuitBreaker implements the circuit breaker pattern for external service protection
// Implements Requirement 6.4 - Circuit breaker pattern for external service protection
type CircuitBreaker struct {
	name                string
	config              CircuitBreakerConfig
	state               CircuitState
	failureCount        int
	successCount        int
	lastFailureTime     time.Time
	lastStateChangeTime time.Time
	halfOpenRequests    int
	mutex               sync.RWMutex
	logger              *logrus.Logger
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:                name,
		config:              config,
		state:               StateClosed,
		failureCount:        0,
		successCount:        0,
		lastStateChangeTime: time.Now(),
		halfOpenRequests:    0,
		logger:              logrus.New(),
	}
}

// ErrCircuitOpen is returned when the circuit breaker is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// ErrTooManyRequests is returned when too many requests are made in half-open state
var ErrTooManyRequests = errors.New("too many requests in half-open state")

// Execute runs the given function if the circuit breaker allows it
// Returns ErrCircuitOpen if the circuit is open
func (cb *CircuitBreaker) Execute(fn func() error) error {
	// Check if we can execute the request
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute the function
	err := fn()

	// Record the result
	cb.afterRequest(err)

	return err
}

// beforeRequest checks if the request should be allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	switch cb.state {
	case StateClosed:
		// Allow request
		return nil

	case StateOpen:
		// Check if timeout has elapsed
		if time.Since(cb.lastStateChangeTime) > cb.config.Timeout {
			// Transition to half-open state
			cb.transitionToHalfOpen()
			return nil
		}
		// Circuit is still open
		cb.logger.WithFields(logrus.Fields{
			"circuit_breaker": cb.name,
			"state":           cb.state.String(),
			"time_remaining":  cb.config.Timeout - time.Since(cb.lastStateChangeTime),
		}).Warn("Circuit breaker is open, blocking request")
		return ErrCircuitOpen

	case StateHalfOpen:
		// Allow limited requests in half-open state
		if cb.halfOpenRequests >= cb.config.HalfOpenMaxRequests {
			cb.logger.WithFields(logrus.Fields{
				"circuit_breaker":        cb.name,
				"state":                  cb.state.String(),
				"half_open_requests":     cb.halfOpenRequests,
				"max_half_open_requests": cb.config.HalfOpenMaxRequests,
			}).Warn("Too many requests in half-open state")
			return ErrTooManyRequests
		}
		cb.halfOpenRequests++
		return nil

	default:
		return nil
	}
}

// afterRequest records the result of the request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onSuccess handles a successful request
func (cb *CircuitBreaker) onSuccess() {
	cb.successCount++

	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failureCount = 0

	case StateHalfOpen:
		// If we've had enough successful requests, close the circuit
		if cb.successCount >= cb.config.HalfOpenMaxRequests {
			cb.transitionToClosed()
		}
	}

	cb.logger.WithFields(logrus.Fields{
		"circuit_breaker": cb.name,
		"state":           cb.state.String(),
		"success_count":   cb.successCount,
		"failure_count":   cb.failureCount,
	}).Debug("Circuit breaker recorded success")
}

// onFailure handles a failed request
func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		// Open circuit if we've exceeded max failures
		if cb.failureCount >= cb.config.MaxFailures {
			cb.transitionToOpen()
		}

	case StateHalfOpen:
		// Any failure in half-open state reopens the circuit
		cb.transitionToOpen()
	}

	cb.logger.WithFields(logrus.Fields{
		"circuit_breaker": cb.name,
		"state":           cb.state.String(),
		"failure_count":   cb.failureCount,
		"max_failures":    cb.config.MaxFailures,
	}).Warn("Circuit breaker recorded failure")
}

// transitionToOpen transitions the circuit breaker to open state
func (cb *CircuitBreaker) transitionToOpen() {
	cb.state = StateOpen
	cb.lastStateChangeTime = time.Now()
	cb.halfOpenRequests = 0

	cb.logger.WithFields(logrus.Fields{
		"circuit_breaker": cb.name,
		"state":           cb.state.String(),
		"failure_count":   cb.failureCount,
		"timeout":         cb.config.Timeout,
	}).Error("Circuit breaker opened due to failures")
}

// transitionToHalfOpen transitions the circuit breaker to half-open state
func (cb *CircuitBreaker) transitionToHalfOpen() {
	cb.state = StateHalfOpen
	cb.lastStateChangeTime = time.Now()
	cb.halfOpenRequests = 0
	cb.successCount = 0

	cb.logger.WithFields(logrus.Fields{
		"circuit_breaker": cb.name,
		"state":           cb.state.String(),
	}).Info("Circuit breaker transitioned to half-open state")
}

// transitionToClosed transitions the circuit breaker to closed state
func (cb *CircuitBreaker) transitionToClosed() {
	cb.state = StateClosed
	cb.lastStateChangeTime = time.Now()
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenRequests = 0

	cb.logger.WithFields(logrus.Fields{
		"circuit_breaker": cb.name,
		"state":           cb.state.String(),
	}).Info("Circuit breaker closed after successful recovery")
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetMetrics returns current metrics for the circuit breaker
func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return map[string]interface{}{
		"name":                   cb.name,
		"state":                  cb.state.String(),
		"failure_count":          cb.failureCount,
		"success_count":          cb.successCount,
		"last_failure_time":      cb.lastFailureTime,
		"last_state_change":      cb.lastStateChangeTime,
		"half_open_requests":     cb.halfOpenRequests,
		"max_failures":           cb.config.MaxFailures,
		"timeout":                cb.config.Timeout,
		"half_open_max_requests": cb.config.HalfOpenMaxRequests,
	}
}

// Reset resets the circuit breaker to its initial state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenRequests = 0
	cb.lastStateChangeTime = time.Now()

	cb.logger.WithFields(logrus.Fields{
		"circuit_breaker": cb.name,
	}).Info("Circuit breaker reset to closed state")
}
