package shared

import (
	"errors"
	"testing"
	"time"
)

// TestCircuitBreakerClosedState tests circuit breaker in closed state
func TestCircuitBreakerClosedState(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         3,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test-cb", config)

	// Circuit should start in closed state
	if cb.GetState() != StateClosed {
		t.Errorf("Expected initial state to be CLOSED, got %s", cb.GetState().String())
	}

	// Successful requests should keep circuit closed
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if cb.GetState() != StateClosed {
		t.Errorf("Expected state to remain CLOSED after success, got %s", cb.GetState().String())
	}
}

// TestCircuitBreakerOpensAfterFailures tests circuit breaker opens after max failures
func TestCircuitBreakerOpensAfterFailures(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         3,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test-cb", config)

	testError := errors.New("test error")

	// Execute failing requests up to max failures
	for i := 0; i < config.MaxFailures; i++ {
		err := cb.Execute(func() error {
			return testError
		})

		if err != testError {
			t.Errorf("Expected test error, got %v", err)
		}
	}

	// Circuit should now be open
	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be OPEN after %d failures, got %s", config.MaxFailures, cb.GetState().String())
	}

	// Next request should fail immediately with ErrCircuitOpen
	err := cb.Execute(func() error {
		t.Error("Function should not be executed when circuit is open")
		return nil
	})

	if err != ErrCircuitOpen {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}
}

// TestCircuitBreakerTransitionsToHalfOpen tests circuit breaker transitions to half-open after timeout
func TestCircuitBreakerTransitionsToHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test-cb", config)

	testError := errors.New("test error")

	// Open the circuit
	for i := 0; i < config.MaxFailures; i++ {
		cb.Execute(func() error {
			return testError
		})
	}

	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be OPEN, got %s", cb.GetState().String())
	}

	// Wait for timeout
	time.Sleep(config.Timeout + 50*time.Millisecond)

	// Next request should transition to half-open
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error in half-open state, got %v", err)
	}

	if cb.GetState() != StateHalfOpen {
		t.Errorf("Expected state to be HALF_OPEN after timeout, got %s", cb.GetState().String())
	}
}

// TestCircuitBreakerClosesAfterSuccessfulHalfOpen tests circuit closes after successful half-open requests
func TestCircuitBreakerClosesAfterSuccessfulHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test-cb", config)

	testError := errors.New("test error")

	// Open the circuit
	for i := 0; i < config.MaxFailures; i++ {
		cb.Execute(func() error {
			return testError
		})
	}

	// Wait for timeout
	time.Sleep(config.Timeout + 50*time.Millisecond)

	// Execute successful requests in half-open state
	for i := 0; i < config.HalfOpenMaxRequests; i++ {
		err := cb.Execute(func() error {
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	}

	// Circuit should now be closed
	if cb.GetState() != StateClosed {
		t.Errorf("Expected state to be CLOSED after successful half-open requests, got %s", cb.GetState().String())
	}
}

// TestCircuitBreakerReopensOnHalfOpenFailure tests circuit reopens on failure in half-open state
func TestCircuitBreakerReopensOnHalfOpenFailure(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test-cb", config)

	testError := errors.New("test error")

	// Open the circuit
	for i := 0; i < config.MaxFailures; i++ {
		cb.Execute(func() error {
			return testError
		})
	}

	// Wait for timeout
	time.Sleep(config.Timeout + 50*time.Millisecond)

	// Execute one successful request to transition to half-open
	cb.Execute(func() error {
		return nil
	})

	if cb.GetState() != StateHalfOpen {
		t.Errorf("Expected state to be HALF_OPEN, got %s", cb.GetState().String())
	}

	// Execute failing request in half-open state
	err := cb.Execute(func() error {
		return testError
	})

	if err != testError {
		t.Errorf("Expected test error, got %v", err)
	}

	// Circuit should reopen
	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be OPEN after half-open failure, got %s", cb.GetState().String())
	}
}

// TestCircuitBreakerHalfOpenMaxRequests tests half-open state request limiting
func TestCircuitBreakerHalfOpenMaxRequests(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test-cb", config)

	testError := errors.New("test error")

	// Open the circuit
	for i := 0; i < config.MaxFailures; i++ {
		cb.Execute(func() error {
			return testError
		})
	}

	// Wait for timeout
	time.Sleep(config.Timeout + 50*time.Millisecond)

	// Execute max allowed requests in half-open state
	for i := 0; i < config.HalfOpenMaxRequests; i++ {
		err := cb.Execute(func() error {
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error for request %d, got %v", i+1, err)
		}
	}

	// Circuit should now be closed after successful half-open requests
	if cb.GetState() != StateClosed {
		t.Errorf("Expected state to be CLOSED, got %s", cb.GetState().String())
	}
}

// TestCircuitBreakerReset tests manual reset functionality
func TestCircuitBreakerReset(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test-cb", config)

	testError := errors.New("test error")

	// Open the circuit
	for i := 0; i < config.MaxFailures; i++ {
		cb.Execute(func() error {
			return testError
		})
	}

	if cb.GetState() != StateOpen {
		t.Errorf("Expected state to be OPEN, got %s", cb.GetState().String())
	}

	// Reset the circuit breaker
	cb.Reset()

	// Circuit should be closed
	if cb.GetState() != StateClosed {
		t.Errorf("Expected state to be CLOSED after reset, got %s", cb.GetState().String())
	}

	// Should be able to execute requests
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error after reset, got %v", err)
	}
}

// TestCircuitBreakerMetrics tests metrics collection
func TestCircuitBreakerMetrics(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         3,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test-cb", config)

	testError := errors.New("test error")

	// Execute some successful requests
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return nil
		})
	}

	// Execute some failing requests
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testError
		})
	}

	// Get metrics
	metrics := cb.GetMetrics()

	if metrics["name"] != "test-cb" {
		t.Errorf("Expected name to be 'test-cb', got %v", metrics["name"])
	}

	if metrics["state"] != StateClosed.String() {
		t.Errorf("Expected state to be CLOSED, got %v", metrics["state"])
	}

	if metrics["failure_count"] != 2 {
		t.Errorf("Expected failure_count to be 2, got %v", metrics["failure_count"])
	}

	if metrics["max_failures"] != 3 {
		t.Errorf("Expected max_failures to be 3, got %v", metrics["max_failures"])
	}
}

// TestCircuitBreakerSuccessResetsFailureCount tests that success resets failure count in closed state
func TestCircuitBreakerSuccessResetsFailureCount(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         3,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test-cb", config)

	testError := errors.New("test error")

	// Execute some failing requests (but not enough to open circuit)
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testError
		})
	}

	metrics := cb.GetMetrics()
	if metrics["failure_count"] != 2 {
		t.Errorf("Expected failure_count to be 2, got %v", metrics["failure_count"])
	}

	// Execute successful request
	cb.Execute(func() error {
		return nil
	})

	// Failure count should be reset
	metrics = cb.GetMetrics()
	if metrics["failure_count"] != 0 {
		t.Errorf("Expected failure_count to be reset to 0 after success, got %v", metrics["failure_count"])
	}

	if cb.GetState() != StateClosed {
		t.Errorf("Expected state to remain CLOSED, got %s", cb.GetState().String())
	}
}
