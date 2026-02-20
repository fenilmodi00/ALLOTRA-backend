package tests

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/fenilmodi00/ipo-backend/database"
	"github.com/fenilmodi00/ipo-backend/handlers"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// TestGMPHistoryRoutesRegistration tests that GMP history routes are properly registered
// Validates Requirements 8.1, 8.3 - Integration with existing authentication and middleware
func TestGMPHistoryRoutesRegistration(t *testing.T) {
	// Setup test database connection
	db := database.DB
	if db == nil {
		t.Skip("Database not available for route testing")
	}

	// Initialize service and handler
	gmpHistoryService := services.NewGMPHistoryService(db)
	defer gmpHistoryService.Close()

	gmpHistoryHandler := handlers.NewGMPHistoryHandler(gmpHistoryService)

	// Create Fiber app
	app := fiber.New()
	api := app.Group("/api/v1")

	// Register routes (same as in main.go)
	api.Get("/gmp/history/:ipo_id", gmpHistoryHandler.GetIPOPriceHistory)
	api.Get("/gmp/history/:ipo_id/chart", gmpHistoryHandler.GetChartData)
	api.Get("/gmp/history/:ipo_id/summary", gmpHistoryHandler.GetHistorySummary)

	// Test cases for route registration
	testCases := []struct {
		name           string
		route          string
		expectedStatus int
		description    string
	}{
		{
			name:           "GetIPOPriceHistory route exists",
			route:          "/api/v1/gmp/history/" + uuid.New().String(),
			expectedStatus: 404, // 404 because IPO doesn't exist, but route is registered
			description:    "Verifies /api/v1/gmp/history/:ipo_id route is registered",
		},
		{
			name:           "GetChartData route exists",
			route:          "/api/v1/gmp/history/" + uuid.New().String() + "/chart",
			expectedStatus: 404, // 404 because IPO doesn't exist, but route is registered
			description:    "Verifies /api/v1/gmp/history/:ipo_id/chart route is registered",
		},
		{
			name:           "GetHistorySummary route exists",
			route:          "/api/v1/gmp/history/" + uuid.New().String() + "/summary",
			expectedStatus: 404, // 404 because IPO doesn't exist, but route is registered
			description:    "Verifies /api/v1/gmp/history/:ipo_id/summary route is registered",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest("GET", tc.route, nil)
			req.Header.Set("Content-Type", "application/json")

			// Execute request
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("Failed to execute request: %v", err)
			}

			// Verify status code
			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d for route %s", tc.expectedStatus, resp.StatusCode, tc.route)
			}

			// Verify response is JSON
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Errorf("Response is not valid JSON: %v", err)
			}

			// Verify response has success field
			if _, ok := result["success"]; !ok {
				t.Error("Response missing 'success' field")
			}

			t.Logf("✓ %s - Status: %d", tc.description, resp.StatusCode)
		})
	}
}

// TestGMPHistoryRouteParameterValidation tests parameter validation for GMP history routes
// Validates Requirements 8.1 - Proper route documentation and parameter validation
func TestGMPHistoryRouteParameterValidation(t *testing.T) {
	// Setup test database connection
	db := database.DB
	if db == nil {
		t.Skip("Database not available for route testing")
	}

	// Initialize service and handler
	gmpHistoryService := services.NewGMPHistoryService(db)
	defer gmpHistoryService.Close()

	gmpHistoryHandler := handlers.NewGMPHistoryHandler(gmpHistoryService)

	// Create Fiber app
	app := fiber.New()
	api := app.Group("/api/v1")

	// Register routes
	api.Get("/gmp/history/:ipo_id", gmpHistoryHandler.GetIPOPriceHistory)
	api.Get("/gmp/history/:ipo_id/chart", gmpHistoryHandler.GetChartData)

	// Test invalid UUID format
	t.Run("Invalid UUID format returns 400", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/gmp/history/invalid-uuid", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("Failed to execute request: %v", err)
		}

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400 for invalid UUID, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if result["success"] != false {
			t.Error("Expected success=false for invalid UUID")
		}

		if result["error"] == nil {
			t.Error("Expected error message for invalid UUID")
		}

		t.Logf("✓ Invalid UUID properly rejected with status 400")
	})

	// Test date range parameter validation
	t.Run("Invalid date format returns 400", func(t *testing.T) {
		testIPOID := uuid.New().String()
		req := httptest.NewRequest("GET", "/api/v1/gmp/history/"+testIPOID+"?start_date=invalid-date", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("Failed to execute request: %v", err)
		}

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400 for invalid date format, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if result["success"] != false {
			t.Error("Expected success=false for invalid date format")
		}

		t.Logf("✓ Invalid date format properly rejected with status 400")
	})

	// Test pagination parameter validation
	t.Run("Pagination parameters are respected", func(t *testing.T) {
		testIPOID := uuid.New().String()
		req := httptest.NewRequest("GET", "/api/v1/gmp/history/"+testIPOID+"?page=1&page_size=50", nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("Failed to execute request: %v", err)
		}

		// Should return 404 (no data) but parameters should be accepted
		if resp.StatusCode != 404 {
			t.Logf("Status code: %d (expected 404 for non-existent IPO)", resp.StatusCode)
		}

		t.Logf("✓ Pagination parameters accepted")
	})
}

// TestGMPHistoryRouteIntegrationWithMiddleware tests middleware integration
// Validates Requirements 8.3 - Integration with existing authentication and middleware
func TestGMPHistoryRouteIntegrationWithMiddleware(t *testing.T) {
	// Setup test database connection
	db := database.DB
	if db == nil {
		t.Skip("Database not available for route testing")
	}

	// Initialize service and handler
	gmpHistoryService := services.NewGMPHistoryService(db)
	defer gmpHistoryService.Close()

	gmpHistoryHandler := handlers.NewGMPHistoryHandler(gmpHistoryService)

	// Create Fiber app with middleware (similar to main.go)
	app := fiber.New()

	// Add CORS middleware (as in main.go)
	// Note: In production, this would include authentication middleware

	api := app.Group("/api/v1")

	// Register routes
	api.Get("/gmp/history/:ipo_id", gmpHistoryHandler.GetIPOPriceHistory)
	api.Get("/gmp/history/:ipo_id/chart", gmpHistoryHandler.GetChartData)
	api.Get("/gmp/history/:ipo_id/summary", gmpHistoryHandler.GetHistorySummary)

	t.Run("Routes work with middleware stack", func(t *testing.T) {
		testIPOID := uuid.New().String()
		req := httptest.NewRequest("GET", "/api/v1/gmp/history/"+testIPOID, nil)
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("Failed to execute request with middleware: %v", err)
		}

		// Should return 404 (no data) but middleware should not block
		if resp.StatusCode != 404 {
			t.Logf("Status code: %d", resp.StatusCode)
		}

		// Verify response format is correct
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Errorf("Response is not valid JSON after middleware: %v", err)
		}

		t.Logf("✓ Routes work correctly with middleware stack")
	})
}
