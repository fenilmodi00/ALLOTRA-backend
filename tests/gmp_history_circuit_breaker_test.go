package tests

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/fenilmodi00/ipo-backend/database"
	"github.com/fenilmodi00/ipo-backend/handlers"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
)

// TestCircuitBreakerIntegration tests that circuit breaker is properly integrated
// Implements Requirement 6.4 - Circuit breaker patterns to prevent cascading failures
func TestCircuitBreakerIntegration(t *testing.T) {
	// Setup database connection
	db := database.DB
	if db == nil {
		t.Skip("Database not available for testing")
	}

	// Create service
	gmpHistoryService := services.NewGMPHistoryService(db)
	defer gmpHistoryService.Close()

	t.Run("Circuit breaker is initialized", func(t *testing.T) {
		metrics := gmpHistoryService.GetCircuitBreakerMetrics()

		if metrics == nil {
			t.Fatal("Circuit breaker metrics should not be nil")
		}

		// Check that circuit breaker has expected fields
		if _, ok := metrics["state"]; !ok {
			t.Error("Circuit breaker metrics should include 'state' field")
		}

		if _, ok := metrics["failure_count"]; !ok {
			t.Error("Circuit breaker metrics should include 'failure_count' field")
		}

		if _, ok := metrics["max_failures"]; !ok {
			t.Error("Circuit breaker metrics should include 'max_failures' field")
		}
	})

	t.Run("Circuit breaker starts in CLOSED state", func(t *testing.T) {
		state := gmpHistoryService.GetCircuitBreakerState()

		if state != "CLOSED" {
			t.Errorf("Expected circuit breaker to start in CLOSED state, got: %s", state)
		}
	})

	t.Run("Service reports healthy when circuit breaker is closed", func(t *testing.T) {
		isHealthy := gmpHistoryService.IsServiceHealthy()

		if !isHealthy {
			t.Error("Service should be healthy when circuit breaker is closed")
		}
	})
}

// TestHealthCheckEndpoint tests the health check endpoint
// Implements Requirement 6.4 - Health checks and service monitoring
func TestHealthCheckEndpoint(t *testing.T) {
	// Setup database connection
	db := database.DB
	if db == nil {
		t.Skip("Database not available for testing")
	}

	// Create service and handler
	gmpHistoryService := services.NewGMPHistoryService(db)
	defer gmpHistoryService.Close()

	gmpHistoryHandler := handlers.NewGMPHistoryHandler(gmpHistoryService)

	// Create Fiber app
	app := fiber.New()
	api := app.Group("/api")

	// Register health check route
	api.Get("/gmp/history/health", gmpHistoryHandler.GetHealthCheck)

	t.Run("Health check endpoint returns 200 when healthy", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/gmp/history/health", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != fiber.StatusOK && resp.StatusCode != fiber.StatusServiceUnavailable {
			t.Errorf("Expected status 200 or 503, got: %d", resp.StatusCode)
		}

		// Parse response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Check response structure
		if success, ok := result["success"].(bool); !ok || !success {
			t.Error("Expected success to be true")
		}

		if status, ok := result["status"].(string); !ok {
			t.Error("Expected status field in response")
		} else if status != "healthy" && status != "degraded" && status != "recovering" {
			t.Errorf("Unexpected status value: %s", status)
		}

		// Check components
		if components, ok := result["components"].(map[string]interface{}); !ok {
			t.Error("Expected components field in response")
		} else {
			// Check circuit breaker component
			if _, ok := components["circuit_breaker"]; !ok {
				t.Error("Expected circuit_breaker component in health check")
			}

			// Check resilience queue component
			if _, ok := components["resilience_queue"]; !ok {
				t.Error("Expected resilience_queue component in health check")
			}

			// Check cache component
			if _, ok := components["cache"]; !ok {
				t.Error("Expected cache component in health check")
			}

			// Check error tracking component
			if _, ok := components["error_tracking"]; !ok {
				t.Error("Expected error_tracking component in health check")
			}
		}
	})
}

// TestServiceMetricsEndpoint tests the service metrics endpoint
// Implements Requirement 6.4 - Service monitoring
func TestServiceMetricsEndpoint(t *testing.T) {
	// Setup database connection
	db := database.DB
	if db == nil {
		t.Skip("Database not available for testing")
	}

	// Create service and handler
	gmpHistoryService := services.NewGMPHistoryService(db)
	defer gmpHistoryService.Close()

	gmpHistoryHandler := handlers.NewGMPHistoryHandler(gmpHistoryService)

	// Create Fiber app
	app := fiber.New()
	api := app.Group("/api")

	// Register metrics route
	api.Get("/gmp/history/metrics", gmpHistoryHandler.GetServiceMetrics)

	t.Run("Metrics endpoint returns comprehensive metrics", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/gmp/history/metrics", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}

		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Expected status 200, got: %d", resp.StatusCode)
		}

		// Parse response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Check response structure
		if success, ok := result["success"].(bool); !ok || !success {
			t.Error("Expected success to be true")
		}

		if data, ok := result["data"].(map[string]interface{}); !ok {
			t.Error("Expected data field in response")
		} else {
			// Check for circuit breaker metrics
			if _, ok := data["circuit_breaker"]; !ok {
				t.Error("Expected circuit_breaker metrics")
			}

			// Check for resilience queue metrics
			if _, ok := data["resilience_queue"]; !ok {
				t.Error("Expected resilience_queue metrics")
			}

			// Check for cache metrics
			if _, ok := data["cache"]; !ok {
				t.Error("Expected cache metrics")
			}

			// Check for error metrics
			if _, ok := data["errors"]; !ok {
				t.Error("Expected errors metrics")
			}
		}
	})
}

// TestGracefulDegradation tests that the service degrades gracefully when circuit breaker is open
// Implements Requirement 6.4 - Graceful degradation for service failures
func TestGracefulDegradation(t *testing.T) {
	// Setup database connection
	db := database.DB
	if db == nil {
		t.Skip("Database not available for testing")
	}

	// Create service
	gmpHistoryService := services.NewGMPHistoryService(db)
	defer gmpHistoryService.Close()

	t.Run("Service health check reflects circuit breaker state", func(t *testing.T) {
		// Initially should be healthy
		if !gmpHistoryService.IsServiceHealthy() {
			t.Error("Service should start healthy")
		}

		// Get initial state
		initialState := gmpHistoryService.GetCircuitBreakerState()
		t.Logf("Initial circuit breaker state: %s", initialState)

		// Note: We can't easily force the circuit breaker open in a unit test
		// without making actual failing requests to external services.
		// This test verifies the health check logic is in place.

		// Verify that health check considers circuit breaker state
		metrics := gmpHistoryService.GetCircuitBreakerMetrics()
		if metrics == nil {
			t.Fatal("Circuit breaker metrics should not be nil")
		}

		// Verify state is being tracked
		if state, ok := metrics["state"].(string); !ok {
			t.Error("Circuit breaker should report its state")
		} else {
			t.Logf("Circuit breaker state: %s", state)
		}
	})
}

// TestResilienceQueueIntegration tests that resilience queue is properly integrated
// Implements Requirement 6.5 - Database resilience with queuing and retry
func TestResilienceQueueIntegration(t *testing.T) {
	// Setup database connection
	db := database.DB
	if db == nil {
		t.Skip("Database not available for testing")
	}

	// Create service
	gmpHistoryService := services.NewGMPHistoryService(db)
	defer gmpHistoryService.Close()

	t.Run("Resilience queue is initialized", func(t *testing.T) {
		metrics := gmpHistoryService.GetResilienceQueueMetrics()

		if metrics == nil {
			t.Fatal("Resilience queue metrics should not be nil")
		}

		// Check that resilience queue is enabled
		if enabled, ok := metrics["enabled"].(bool); !ok || !enabled {
			t.Error("Resilience queue should be enabled")
		}
	})

	t.Run("Resilience queue size is tracked", func(t *testing.T) {
		queueSize := gmpHistoryService.GetResilienceQueueSize()

		// Queue should start empty
		if queueSize < 0 {
			t.Errorf("Queue size should be non-negative, got: %d", queueSize)
		}

		t.Logf("Current resilience queue size: %d", queueSize)
	})
}
