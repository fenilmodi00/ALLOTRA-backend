package models

import (
	"encoding/json"
	"testing"
	"time"
)

// TestGMPPriceHistoryEntryStructure tests the GMPPriceHistoryEntry model structure
func TestGMPPriceHistoryEntryStructure(t *testing.T) {
	entry := GMPPriceHistoryEntry{
		ID:                 "test-id",
		IPOID:              "ipo-id",
		CompanyCode:        "TEST",
		RecordDate:         time.Now(),
		IPOPrice:           100.0,
		GMPValue:           50.0,
		EstimatedListing:   150.0,
		ListingPercent:     50.0,
		EstimatedProfit:    50.0,
		SubscriptionStatus: "10x subscribed",
		Sub2Sauda:          45.0,
		LastUpdated:        "2024-01-01",
		DataSource:         "investorgain.com",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal GMPPriceHistoryEntry: %v", err)
	}

	// Test JSON unmarshaling
	var decoded GMPPriceHistoryEntry
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal GMPPriceHistoryEntry: %v", err)
	}

	// Verify key fields
	if decoded.CompanyCode != entry.CompanyCode {
		t.Errorf("Expected company_code %s, got %s", entry.CompanyCode, decoded.CompanyCode)
	}
	if decoded.IPOPrice != entry.IPOPrice {
		t.Errorf("Expected ipo_price %.2f, got %.2f", entry.IPOPrice, decoded.IPOPrice)
	}
	if decoded.GMPValue != entry.GMPValue {
		t.Errorf("Expected gmp_value %.2f, got %.2f", entry.GMPValue, decoded.GMPValue)
	}
}

// TestGMPPriceHistoryCollection tests the collection model
func TestGMPPriceHistoryCollection(t *testing.T) {
	now := time.Now()
	collection := GMPPriceHistoryCollection{
		IPOID:        "test-ipo-id",
		IPOName:      "Test IPO",
		CompanyCode:  "TEST",
		TotalRecords: 2,
		DateRange: &DateRange{
			StartDate: now.AddDate(0, 0, -7),
			EndDate:   now,
		},
		Entries: []GMPPriceHistoryEntry{
			{
				ID:               "entry-1",
				IPOID:            "test-ipo-id",
				CompanyCode:      "TEST",
				RecordDate:       now.AddDate(0, 0, -7),
				IPOPrice:         100.0,
				GMPValue:         40.0,
				EstimatedListing: 140.0,
				ListingPercent:   40.0,
			},
			{
				ID:               "entry-2",
				IPOID:            "test-ipo-id",
				CompanyCode:      "TEST",
				RecordDate:       now,
				IPOPrice:         100.0,
				GMPValue:         50.0,
				EstimatedListing: 150.0,
				ListingPercent:   50.0,
			},
		},
		Metadata: &CollectionMetadata{
			LastScraped:     now,
			DataSource:      "investorgain.com",
			ScrapingSuccess: true,
			ErrorCount:      0,
			ProcessingTime:  "2.5s",
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(collection)
	if err != nil {
		t.Fatalf("Failed to marshal GMPPriceHistoryCollection: %v", err)
	}

	// Test JSON unmarshaling
	var decoded GMPPriceHistoryCollection
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal GMPPriceHistoryCollection: %v", err)
	}

	// Verify structure
	if decoded.IPOID != collection.IPOID {
		t.Errorf("Expected ipo_id %s, got %s", collection.IPOID, decoded.IPOID)
	}
	if decoded.TotalRecords != collection.TotalRecords {
		t.Errorf("Expected total_records %d, got %d", collection.TotalRecords, decoded.TotalRecords)
	}
	if len(decoded.Entries) != len(collection.Entries) {
		t.Errorf("Expected %d entries, got %d", len(collection.Entries), len(decoded.Entries))
	}
	if decoded.Metadata == nil {
		t.Error("Expected metadata to be present")
	}
}

// TestChartDataResponse tests the chart data response model
func TestChartDataResponse(t *testing.T) {
	response := ChartDataResponse{
		IPOInfo: IPOBasicInfo{
			IPOID:       "test-ipo",
			IPOName:     "Test IPO",
			CompanyCode: "TEST",
			IPOPrice:    100.0,
			Status:      "LIVE",
		},
		ChartData: []ChartPoint{
			{
				Date:             "2024-01-01",
				GMPValue:         40.0,
				EstimatedListing: 140.0,
				ListingPercent:   40.0,
			},
			{
				Date:             "2024-01-02",
				GMPValue:         50.0,
				EstimatedListing: 150.0,
				ListingPercent:   50.0,
			},
		},
		Statistics: ChartStatistics{
			MaxGMP:         50.0,
			MinGMP:         40.0,
			AverageGMP:     45.0,
			LatestGMP:      50.0,
			TrendDirection: "up",
		},
		Metadata: ChartMetadata{
			DataSource:     "investorgain.com",
			LastUpdated:    time.Now(),
			TotalRecords:   2,
			DateRangeStart: "2024-01-01",
			DateRangeEnd:   "2024-01-02",
		},
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal ChartDataResponse: %v", err)
	}

	// Test JSON unmarshaling
	var decoded ChartDataResponse
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ChartDataResponse: %v", err)
	}

	// Verify structure
	if decoded.IPOInfo.IPOName != response.IPOInfo.IPOName {
		t.Errorf("Expected ipo_name %s, got %s", response.IPOInfo.IPOName, decoded.IPOInfo.IPOName)
	}
	if len(decoded.ChartData) != len(response.ChartData) {
		t.Errorf("Expected %d chart points, got %d", len(response.ChartData), len(decoded.ChartData))
	}
	if decoded.Statistics.TrendDirection != response.Statistics.TrendDirection {
		t.Errorf("Expected trend %s, got %s", response.Statistics.TrendDirection, decoded.Statistics.TrendDirection)
	}
}

// TestGMPHistoryJobLog tests the job log model
func TestGMPHistoryJobLog(t *testing.T) {
	now := time.Now()
	endTime := now.Add(5 * time.Minute)

	jobLog := GMPHistoryJobLog{
		ID:                "job-id",
		JobStartTime:      now,
		JobEndTime:        &endTime,
		IPOsProcessed:     10,
		SuccessfulScrapes: 8,
		FailedScrapes:     2,
		TotalRecordsAdded: 150,
		ExecutionStatus:   "completed",
		ErrorSummary:      "2 IPOs failed due to network timeout",
		CreatedAt:         now,
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(jobLog)
	if err != nil {
		t.Fatalf("Failed to marshal GMPHistoryJobLog: %v", err)
	}

	// Test JSON unmarshaling
	var decoded GMPHistoryJobLog
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal GMPHistoryJobLog: %v", err)
	}

	// Verify structure
	if decoded.IPOsProcessed != jobLog.IPOsProcessed {
		t.Errorf("Expected ipos_processed %d, got %d", jobLog.IPOsProcessed, decoded.IPOsProcessed)
	}
	if decoded.SuccessfulScrapes != jobLog.SuccessfulScrapes {
		t.Errorf("Expected successful_scrapes %d, got %d", jobLog.SuccessfulScrapes, decoded.SuccessfulScrapes)
	}
	if decoded.ExecutionStatus != jobLog.ExecutionStatus {
		t.Errorf("Expected execution_status %s, got %s", jobLog.ExecutionStatus, decoded.ExecutionStatus)
	}
}

// TestListingCalculationValidation tests that estimated listing equals IPO price + GMP
func TestListingCalculationValidation(t *testing.T) {
	testCases := []struct {
		name            string
		ipoPrice        float64
		gmpValue        float64
		expectedListing float64
		shouldBeValid   bool
	}{
		{
			name:            "Valid calculation",
			ipoPrice:        100.0,
			gmpValue:        50.0,
			expectedListing: 150.0,
			shouldBeValid:   true,
		},
		{
			name:            "Valid with decimals",
			ipoPrice:        125.50,
			gmpValue:        37.25,
			expectedListing: 162.75,
			shouldBeValid:   true,
		},
		{
			name:            "Zero GMP",
			ipoPrice:        100.0,
			gmpValue:        0.0,
			expectedListing: 100.0,
			shouldBeValid:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := GMPPriceHistoryEntry{
				IPOPrice:         tc.ipoPrice,
				GMPValue:         tc.gmpValue,
				EstimatedListing: tc.expectedListing,
			}

			// Verify the calculation
			calculatedListing := entry.IPOPrice + entry.GMPValue
			diff := calculatedListing - entry.EstimatedListing

			// Allow small floating point differences (< 0.01)
			if diff < -0.01 || diff > 0.01 {
				t.Errorf("Listing calculation mismatch: %.2f + %.2f = %.2f, but got %.2f",
					entry.IPOPrice, entry.GMPValue, calculatedListing, entry.EstimatedListing)
			}
		})
	}
}
