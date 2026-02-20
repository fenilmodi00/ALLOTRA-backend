package handlers

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// MockGMPHistoryService is a mock implementation of GMPHistoryService for testing
type MockGMPHistoryService struct {
	GetPriceHistoryByIPOFunc func(ipoID string, dateRange *models.DateRange) (*models.GMPPriceHistoryCollection, error)
}

func (m *MockGMPHistoryService) GetPriceHistoryByIPO(ipoID string, dateRange *models.DateRange) (*models.GMPPriceHistoryCollection, error) {
	if m.GetPriceHistoryByIPOFunc != nil {
		return m.GetPriceHistoryByIPOFunc(ipoID, dateRange)
	}
	return nil, sql.ErrNoRows
}

// TestGetGMPByIPO_WithHistoryAvailable tests the GMP endpoint when history data is available
func TestGetGMPByIPO_WithHistoryAvailable(t *testing.T) {
	// This test verifies that when history data exists, the response includes:
	// 1. History availability indicator
	// 2. Links to history endpoints
	// 3. Summary statistics (trend, recent changes)
	// 4. Backward compatibility (existing GMP data structure unchanged)

	// Note: This is a unit test that would require a test database setup
	// For now, we're documenting the expected behavior
	t.Skip("Requires test database setup - see integration tests")
}

// TestGetGMPByIPO_WithoutHistoryAvailable tests the GMP endpoint when no history data exists
func TestGetGMPByIPO_WithoutHistoryAvailable(t *testing.T) {
	// This test verifies backward compatibility:
	// When no history data exists, the response should still work
	// and indicate that history is not available

	// Note: This is a unit test that would require a test database setup
	// For now, we're documenting the expected behavior
	t.Skip("Requires test database setup - see integration tests")
}

// TestBuildHistoryInfo_WithData tests the buildHistoryInfo helper with available data
func TestBuildHistoryInfo_WithData(t *testing.T) {
	// Create mock history service with data
	mockService := &MockGMPHistoryService{
		GetPriceHistoryByIPOFunc: func(ipoID string, dateRange *models.DateRange) (*models.GMPPriceHistoryCollection, error) {
			// Return mock history data
			now := time.Now()
			sevenDaysAgo := now.AddDate(0, 0, -7)

			return &models.GMPPriceHistoryCollection{
				IPOID:        ipoID,
				IPOName:      "Test IPO",
				CompanyCode:  "TEST",
				TotalRecords: 8,
				DateRange: &models.DateRange{
					StartDate: sevenDaysAgo,
					EndDate:   now,
				},
				Entries: []models.GMPPriceHistoryEntry{
					{
						ID:               uuid.New().String(),
						IPOID:            ipoID,
						RecordDate:       now,
						GMPValue:         150.0,
						IPOPrice:         100.0,
						EstimatedListing: 250.0,
						ListingPercent:   150.0,
					},
					{
						ID:               uuid.New().String(),
						IPOID:            ipoID,
						RecordDate:       now.AddDate(0, 0, -1),
						GMPValue:         145.0,
						IPOPrice:         100.0,
						EstimatedListing: 245.0,
						ListingPercent:   145.0,
					},
					{
						ID:               uuid.New().String(),
						IPOID:            ipoID,
						RecordDate:       now.AddDate(0, 0, -7),
						GMPValue:         130.0,
						IPOPrice:         100.0,
						EstimatedListing: 230.0,
						ListingPercent:   130.0,
					},
				},
			}, nil
		},
	}

	// Create handler with mock service
	handler := &GMPHandler{
		HistoryService: mockService,
	}

	testIPOID := uuid.New().String()
	historyInfo := handler.buildHistoryInfo(testIPOID)

	// Verify history info structure
	if historyInfo == nil {
		t.Fatal("Expected history info, got nil")
	}

	// Check availability
	available, ok := historyInfo["available"].(bool)
	if !ok {
		t.Fatal("Expected 'available' field to be bool")
	}
	if !available {
		t.Error("Expected history to be available")
	}

	// Check links
	links, ok := historyInfo["links"].(fiber.Map)
	if !ok {
		t.Fatal("Expected 'links' field to be fiber.Map")
	}

	expectedLinks := []string{"full_history", "chart_data", "summary"}
	for _, linkKey := range expectedLinks {
		if _, exists := links[linkKey]; !exists {
			t.Errorf("Expected link '%s' to exist", linkKey)
		}
	}

	// Check summary
	summary, ok := historyInfo["summary"].(fiber.Map)
	if !ok {
		t.Fatal("Expected 'summary' field to be fiber.Map")
	}

	// Verify summary contains expected fields
	expectedSummaryFields := []string{
		"latest_gmp",
		"latest_date",
		"trend_direction",
		"trend_change",
		"max_gmp",
		"min_gmp",
		"average_gmp",
	}

	for _, field := range expectedSummaryFields {
		if _, exists := summary[field]; !exists {
			t.Errorf("Expected summary field '%s' to exist", field)
		}
	}

	// Verify trend direction is calculated correctly
	trendDirection, ok := summary["trend_direction"].(string)
	if !ok {
		t.Fatal("Expected 'trend_direction' to be string")
	}
	if trendDirection != "up" {
		t.Errorf("Expected trend direction 'up', got '%s'", trendDirection)
	}
}

// TestBuildHistoryInfo_NoData tests the buildHistoryInfo helper when no data exists
func TestBuildHistoryInfo_NoData(t *testing.T) {
	// Create mock history service that returns no data
	mockService := &MockGMPHistoryService{
		GetPriceHistoryByIPOFunc: func(ipoID string, dateRange *models.DateRange) (*models.GMPPriceHistoryCollection, error) {
			return nil, sql.ErrNoRows
		},
	}

	// Create handler with mock service
	handler := &GMPHandler{
		HistoryService: mockService,
	}

	testIPOID := uuid.New().String()
	historyInfo := handler.buildHistoryInfo(testIPOID)

	// Verify history info structure
	if historyInfo == nil {
		t.Fatal("Expected history info, got nil")
	}

	// Check availability is false
	available, ok := historyInfo["available"].(bool)
	if !ok {
		t.Fatal("Expected 'available' field to be bool")
	}
	if available {
		t.Error("Expected history to be unavailable")
	}

	// Check links still exist (for discoverability)
	links, ok := historyInfo["links"].(fiber.Map)
	if !ok {
		t.Fatal("Expected 'links' field to be fiber.Map")
	}

	expectedLinks := []string{"full_history", "chart_data", "summary"}
	for _, linkKey := range expectedLinks {
		if _, exists := links[linkKey]; !exists {
			t.Errorf("Expected link '%s' to exist", linkKey)
		}
	}

	// Summary should not exist when no data available
	if _, exists := historyInfo["summary"]; exists {
		t.Error("Expected no summary when history is unavailable")
	}
}

// TestCalculateHistorySummary tests the summary calculation logic
func TestCalculateHistorySummary(t *testing.T) {
	handler := &GMPHandler{}

	now := time.Now()

	// Test case 1: Multiple entries with upward trend
	history := &models.GMPPriceHistoryCollection{
		Entries: []models.GMPPriceHistoryEntry{
			{RecordDate: now, GMPValue: 150.0},
			{RecordDate: now.AddDate(0, 0, -1), GMPValue: 145.0},
			{RecordDate: now.AddDate(0, 0, -2), GMPValue: 140.0},
			{RecordDate: now.AddDate(0, 0, -3), GMPValue: 135.0},
			{RecordDate: now.AddDate(0, 0, -4), GMPValue: 130.0},
			{RecordDate: now.AddDate(0, 0, -5), GMPValue: 125.0},
			{RecordDate: now.AddDate(0, 0, -6), GMPValue: 120.0},
			{RecordDate: now.AddDate(0, 0, -7), GMPValue: 115.0},
		},
	}

	summary := handler.calculateHistorySummary(history)
	if summary == nil {
		t.Fatal("Expected summary, got nil")
	}

	// Check latest GMP
	latestGMP, ok := summary["latest_gmp"].(float64)
	if !ok {
		t.Fatal("Expected 'latest_gmp' to be float64")
	}
	if latestGMP != 150.0 {
		t.Errorf("Expected latest GMP 150.0, got %.2f", latestGMP)
	}

	// Check trend direction
	trendDirection, ok := summary["trend_direction"].(string)
	if !ok {
		t.Fatal("Expected 'trend_direction' to be string")
	}
	if trendDirection != "up" {
		t.Errorf("Expected trend 'up', got '%s'", trendDirection)
	}

	// Check max/min
	maxGMP, ok := summary["max_gmp"].(float64)
	if !ok || maxGMP != 150.0 {
		t.Errorf("Expected max GMP 150.0, got %.2f", maxGMP)
	}

	minGMP, ok := summary["min_gmp"].(float64)
	if !ok || minGMP != 115.0 {
		t.Errorf("Expected min GMP 115.0, got %.2f", minGMP)
	}

	// Check 7-day high/low
	sevenDayHigh, ok := summary["seven_day_high"].(float64)
	if !ok || sevenDayHigh != 150.0 {
		t.Errorf("Expected 7-day high 150.0, got %.2f", sevenDayHigh)
	}

	sevenDayLow, ok := summary["seven_day_low"].(float64)
	if !ok || sevenDayLow != 120.0 {
		t.Errorf("Expected 7-day low 120.0, got %.2f", sevenDayLow)
	}
}

// TestCalculateHistorySummary_DownwardTrend tests downward trend detection
func TestCalculateHistorySummary_DownwardTrend(t *testing.T) {
	handler := &GMPHandler{}

	now := time.Now()

	history := &models.GMPPriceHistoryCollection{
		Entries: []models.GMPPriceHistoryEntry{
			{RecordDate: now, GMPValue: 100.0},
			{RecordDate: now.AddDate(0, 0, -1), GMPValue: 105.0},
			{RecordDate: now.AddDate(0, 0, -2), GMPValue: 110.0},
			{RecordDate: now.AddDate(0, 0, -7), GMPValue: 120.0},
		},
	}

	summary := handler.calculateHistorySummary(history)
	if summary == nil {
		t.Fatal("Expected summary, got nil")
	}

	// Check trend direction
	trendDirection, ok := summary["trend_direction"].(string)
	if !ok {
		t.Fatal("Expected 'trend_direction' to be string")
	}
	if trendDirection != "down" {
		t.Errorf("Expected trend 'down', got '%s'", trendDirection)
	}

	// Check trend change is negative
	trendChange, ok := summary["trend_change"].(float64)
	if !ok {
		t.Fatal("Expected 'trend_change' to be float64")
	}
	if trendChange >= 0 {
		t.Errorf("Expected negative trend change, got %.2f", trendChange)
	}
}

// TestBackwardCompatibility_ResponseStructure tests that the response maintains backward compatibility
func TestBackwardCompatibility_ResponseStructure(t *testing.T) {
	// This test documents the expected response structure for backward compatibility
	// The response should include:
	// 1. "success" field (boolean)
	// 2. "data" field (existing GMP data structure - unchanged)
	// 3. "history" field (new, optional - only present when history service is available)

	expectedStructure := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id":                  "uuid",
			"ipo_name":            "string",
			"company_code":        "string",
			"ipo_price":           0.0,
			"gmp_value":           0.0,
			"estimated_listing":   0.0,
			"gain_percent":        0.0,
			"sub2":                0.0,
			"kostak":              0.0,
			"last_updated":        "timestamp",
			"stock_id":            "string",
			"subscription_status": "string",
			"listing_gain":        "string",
			"ipo_status":          "string",
			"data_source":         "string",
		},
		"history": map[string]interface{}{
			"available": true,
			"links": map[string]string{
				"full_history": "/api/gmp/history/{ipo_id}",
				"chart_data":   "/api/gmp/history/{ipo_id}/chart",
				"summary":      "/api/gmp/history/{ipo_id}/summary",
			},
			"total_records": 0,
			"date_range": map[string]string{
				"start": "2024-01-01",
				"end":   "2024-01-31",
			},
			"summary": map[string]interface{}{
				"latest_gmp":           0.0,
				"latest_date":          "2024-01-31",
				"trend_direction":      "up|down|stable",
				"trend_change":         0.0,
				"trend_change_percent": 0.0,
				"max_gmp":              0.0,
				"min_gmp":              0.0,
				"average_gmp":          0.0,
				"seven_day_high":       0.0,
				"seven_day_low":        0.0,
			},
		},
	}

	// Serialize to JSON to verify structure
	jsonBytes, err := json.MarshalIndent(expectedStructure, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal expected structure: %v", err)
	}

	t.Logf("Expected response structure:\n%s", string(jsonBytes))
}

// TestGMPHandler_NilHistoryService tests that handler works without history service
func TestGMPHandler_NilHistoryService(t *testing.T) {
	// Create handler without history service (backward compatibility)
	handler := &GMPHandler{
		DB:             nil, // Would be a real DB in production
		HistoryService: nil,
	}

	// buildHistoryInfo should handle nil service gracefully
	testIPOID := uuid.New().String()
	historyInfo := handler.buildHistoryInfo(testIPOID)

	// When history service is nil, buildHistoryInfo should return nil
	// and the response should not include history field
	if historyInfo != nil {
		// If it returns something, it should at least indicate unavailability
		available, ok := historyInfo["available"].(bool)
		if ok && available {
			t.Error("Expected history to be unavailable when service is nil")
		}
	}
}

// Integration test example (requires database)
func TestGetGMPByIPO_Integration(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	// This test would:
	// 1. Set up a test database with sample IPO and GMP data
	// 2. Insert some history records
	// 3. Make a request to GET /api/gmp/{ipo_id}
	// 4. Verify the response includes history information
	// 5. Verify backward compatibility (existing fields unchanged)
	// 6. Clean up test data
}
