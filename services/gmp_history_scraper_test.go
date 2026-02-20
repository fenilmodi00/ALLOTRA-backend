package services

import (
	"strings"
	"testing"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/sirupsen/logrus"
)

// TestExtractHistoryTable tests the HTML table extraction logic
func TestExtractHistoryTable(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{
		logger: logrus.New(),
	}

	// Sample HTML table structure from InvestorGain
	htmlContent := `
	<table class="table table-bordered">
		<tbody>
			<tr>
				<td>15-Jan-2024</td>
				<td>₹100</td>
				<td>₹10 <img src="fire.png"></td>
				<td>5.5x subscribed</td>
				<td>₹5</td>
				<td>₹110 (<span>10.00%</span>)</td>
				<td>₹50</td>
				<td>Updated 2 hours ago</td>
			</tr>
			<tr>
				<td>14-Jan-2024</td>
				<td>₹100</td>
				<td>₹8</td>
				<td>4.2x subscribed</td>
				<td>₹4</td>
				<td>₹108 (<span>8.00%</span>)</td>
				<td>₹40</td>
				<td>Updated 1 day ago</td>
			</tr>
		</tbody>
	</table>
	`

	entries, errorCount := scraper.ExtractHistoryTable(htmlContent)

	if errorCount != 0 {
		t.Errorf("Expected 0 errors, got %d", errorCount)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	// Verify first entry
	entry1 := entries[0]
	if entry1.IPOPrice != 100 {
		t.Errorf("Expected IPO price 100, got %.2f", entry1.IPOPrice)
	}
	if entry1.GMPValue != 10 {
		t.Errorf("Expected GMP value 10, got %.2f", entry1.GMPValue)
	}
	if entry1.EstimatedListing != 110 {
		t.Errorf("Expected estimated listing 110, got %.2f", entry1.EstimatedListing)
	}
	if entry1.ListingPercent != 10.00 {
		t.Errorf("Expected listing percent 10.00, got %.2f", entry1.ListingPercent)
	}
	if entry1.SubscriptionStatus != "5.5x subscribed" {
		t.Errorf("Expected subscription '5.5x subscribed', got '%s'", entry1.SubscriptionStatus)
	}

	// Verify second entry
	entry2 := entries[1]
	if entry2.GMPValue != 8 {
		t.Errorf("Expected GMP value 8, got %.2f", entry2.GMPValue)
	}
}

// TestExtractHistoryTableWithMalformedData tests error handling for malformed HTML
func TestExtractHistoryTableWithMalformedData(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{
		logger: logrus.New(),
	}

	// HTML with one valid row and one malformed row (missing cells)
	htmlContent := `
	<table class="table table-bordered">
		<tbody>
			<tr>
				<td>15-Jan-2024</td>
				<td>₹100</td>
				<td>₹10</td>
				<td>5.5x subscribed</td>
				<td>₹5</td>
				<td>₹110 (<span>10.00%</span>)</td>
				<td>₹50</td>
				<td>Updated 2 hours ago</td>
			</tr>
			<tr>
				<td>14-Jan-2024</td>
				<td>₹100</td>
				<!-- Missing cells - malformed row -->
			</tr>
		</tbody>
	</table>
	`

	entries, errorCount := scraper.ExtractHistoryTable(htmlContent)

	// Should have 1 error for the malformed row
	if errorCount != 1 {
		t.Errorf("Expected 1 error for malformed row, got %d", errorCount)
	}

	// Should still extract the valid entry
	if len(entries) != 1 {
		t.Errorf("Expected 1 valid entry, got %d", len(entries))
	}
}

// TestValidateHistoryEntry tests data validation rules
// Validates Requirements 5.1, 5.2, 5.3, 5.4
func TestValidateHistoryEntry(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{}

	tests := []struct {
		name        string
		entry       models.GMPPriceHistoryEntry
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid entry",
			entry: models.GMPPriceHistoryEntry{
				RecordDate:       time.Now(),
				IPOPrice:         100,
				GMPValue:         10,
				EstimatedListing: 110,
				ListingPercent:   10.0,
				EstimatedProfit:  10,
				Sub2Sauda:        5,
			},
			expectError: false,
		},
		{
			name: "Negative GMP value (Requirement 5.1)",
			entry: models.GMPPriceHistoryEntry{
				RecordDate:       time.Now(),
				IPOPrice:         100,
				GMPValue:         -10,
				EstimatedListing: 90,
			},
			expectError: true,
			errorMsg:    "GMP value cannot be negative",
		},
		{
			name: "Negative IPO price (Requirement 5.1)",
			entry: models.GMPPriceHistoryEntry{
				RecordDate:       time.Now(),
				IPOPrice:         -100,
				GMPValue:         10,
				EstimatedListing: -90,
			},
			expectError: true,
			errorMsg:    "IPO price cannot be negative",
		},
		{
			name: "Negative estimated listing (Requirement 5.1)",
			entry: models.GMPPriceHistoryEntry{
				RecordDate:       time.Now(),
				IPOPrice:         100,
				GMPValue:         10,
				EstimatedListing: -110,
			},
			expectError: true,
			errorMsg:    "estimated listing price cannot be negative",
		},
		{
			name: "Negative estimated profit (Requirement 5.1)",
			entry: models.GMPPriceHistoryEntry{
				RecordDate:       time.Now(),
				IPOPrice:         100,
				GMPValue:         10,
				EstimatedListing: 110,
				ListingPercent:   10.0,
				EstimatedProfit:  -5,
			},
			expectError: true,
			errorMsg:    "estimated profit cannot be negative",
		},
		{
			name: "Negative sub2 sauda (Requirement 5.1)",
			entry: models.GMPPriceHistoryEntry{
				RecordDate:       time.Now(),
				IPOPrice:         100,
				GMPValue:         10,
				EstimatedListing: 110,
				ListingPercent:   10.0,
				Sub2Sauda:        -5,
			},
			expectError: true,
			errorMsg:    "sub2 sauda value cannot be negative",
		},
		{
			name: "Listing price calculation mismatch (Requirement 5.3)",
			entry: models.GMPPriceHistoryEntry{
				RecordDate:       time.Now(),
				IPOPrice:         100,
				GMPValue:         10,
				EstimatedListing: 120, // Should be 110
				ListingPercent:   10.0,
			},
			expectError: true,
			errorMsg:    "estimated listing price mismatch",
		},
		{
			name: "Percentage calculation mismatch (Requirement 5.4)",
			entry: models.GMPPriceHistoryEntry{
				RecordDate:       time.Now(),
				IPOPrice:         100,
				GMPValue:         10,
				EstimatedListing: 110,
				ListingPercent:   20.0, // Should be 10.0
			},
			expectError: true,
			errorMsg:    "listing percentage mismatch",
		},
		{
			name: "Date too old (Requirement 5.2)",
			entry: models.GMPPriceHistoryEntry{
				RecordDate:       time.Now().AddDate(-3, 0, 0), // 3 years ago
				IPOPrice:         100,
				GMPValue:         10,
				EstimatedListing: 110,
				ListingPercent:   10.0,
			},
			expectError: true,
			errorMsg:    "record date out of reasonable range",
		},
		{
			name: "Date too far in future (Requirement 5.2)",
			entry: models.GMPPriceHistoryEntry{
				RecordDate:       time.Now().AddDate(2, 0, 0), // 2 years in future
				IPOPrice:         100,
				GMPValue:         10,
				EstimatedListing: 110,
				ListingPercent:   10.0,
			},
			expectError: true,
			errorMsg:    "record date out of reasonable range",
		},
		{
			name: "Zero date (Requirement 5.2)",
			entry: models.GMPPriceHistoryEntry{
				RecordDate:       time.Time{},
				IPOPrice:         100,
				GMPValue:         10,
				EstimatedListing: 110,
				ListingPercent:   10.0,
			},
			expectError: true,
			errorMsg:    "record date cannot be zero",
		},
		{
			name: "Valid entry with zero GMP",
			entry: models.GMPPriceHistoryEntry{
				RecordDate:       time.Now(),
				IPOPrice:         100,
				GMPValue:         0,
				EstimatedListing: 100,
				ListingPercent:   0,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := scraper.validateHistoryEntry(&tt.entry)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %s", err.Error())
				}
			}
		})
	}
}

// TestBuildIPOHistoryURL tests URL construction
func TestBuildIPOHistoryURL(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{}

	tests := []struct {
		name        string
		companyCode string
		ipoID       string
		expectedURL string
		expectError bool
	}{
		{
			name:        "Valid URL construction",
			companyCode: "armour-security-india",
			ipoID:       "1587",
			expectedURL: "https://www.investorgain.com/gmp/armour-security-india-ipo/1587/",
			expectError: false,
		},
		{
			name:        "Empty company code",
			companyCode: "",
			ipoID:       "1587",
			expectedURL: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := scraper.BuildIPOHistoryURL(tt.companyCode, tt.ipoID)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %s", err.Error())
				}
				if url != tt.expectedURL {
					t.Errorf("Expected URL '%s', got '%s'", tt.expectedURL, url)
				}
			}
		})
	}
}

// TestCleanText tests HTML cleaning functionality
func TestCleanText(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove HTML tags",
			input:    "<span>Hello World</span>",
			expected: "Hello World",
		},
		{
			name:     "Remove multiple spaces",
			input:    "Hello    World",
			expected: "Hello World",
		},
		{
			name:     "Remove &nbsp;",
			input:    "Hello&nbsp;World",
			expected: "HelloWorld",
		},
		{
			name:     "Complex HTML with entities",
			input:    "<div>Price: &nbsp;₹100&amp;more</div>",
			expected: "Price: ₹100&more",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scraper.cleanText(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestParseFloat tests float parsing functionality
func TestParseFloat(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{}

	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{
			name:     "Simple number",
			input:    "100",
			expected: 100,
		},
		{
			name:     "Number with rupee symbol",
			input:    "₹100",
			expected: 100,
		},
		{
			name:     "Number with comma",
			input:    "1,000",
			expected: 1000,
		},
		{
			name:     "Percentage",
			input:    "10.5%",
			expected: 10.5,
		},
		{
			name:     "Complex format",
			input:    "₹1,234.56",
			expected: 1234.56,
		},
		{
			name:     "Invalid input",
			input:    "invalid",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scraper.parseFloat(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %.2f, got %.2f", tt.expected, result)
			}
		})
	}
}

// TestParseDate tests date parsing functionality
func TestParseDate(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{}

	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "Format: 02-Jan-2006",
			input:       "15-Jan-2024",
			expectError: false,
		},
		{
			name:        "Format: 2-Jan-2006",
			input:       "5-Jan-2024",
			expectError: false,
		},
		{
			name:        "Format: 2006-01-02",
			input:       "2024-01-15",
			expectError: false,
		},
		{
			name:        "Invalid date",
			input:       "invalid-date",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := scraper.parseDate(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %s", err.Error())
				}
			}
		})
	}
}

// TestNormalizeSubscriptionStatus tests subscription status normalization
// Validates Requirement 5.5 - Subscription data normalization
func TestNormalizeSubscriptionStatus(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Pattern 1: "X times" or "X x"
		{
			name:     "10 times format",
			input:    "10 times",
			expected: "10x subscribed",
		},
		{
			name:     "5 x format",
			input:    "5 x",
			expected: "5x subscribed",
		},
		{
			name:     "2.5 times format",
			input:    "2.5 times",
			expected: "2.5x subscribed",
		},
		{
			name:     "10x format (already normalized)",
			input:    "10x",
			expected: "10x subscribed",
		},

		// Pattern 2: "subscribed X times"
		{
			name:     "subscribed 10 times",
			input:    "subscribed 10 times",
			expected: "10x subscribed",
		},
		{
			name:     "subscribed 5 x",
			input:    "subscribed 5 x",
			expected: "5x subscribed",
		},

		// Pattern 3: "oversubscribed" variations
		{
			name:     "oversubscribed",
			input:    "oversubscribed",
			expected: "oversubscribed",
		},
		{
			name:     "over subscribed",
			input:    "over subscribed",
			expected: "oversubscribed",
		},
		{
			name:     "over-subscribed",
			input:    "over-subscribed",
			expected: "oversubscribed",
		},
		{
			name:     "OVERSUBSCRIBED (uppercase)",
			input:    "OVERSUBSCRIBED",
			expected: "oversubscribed",
		},

		// Pattern 4: "undersubscribed" variations
		{
			name:     "undersubscribed",
			input:    "undersubscribed",
			expected: "undersubscribed",
		},
		{
			name:     "under subscribed",
			input:    "under subscribed",
			expected: "undersubscribed",
		},
		{
			name:     "under-subscribed",
			input:    "under-subscribed",
			expected: "undersubscribed",
		},

		// Pattern 5: "not subscribed"
		{
			name:     "not subscribed",
			input:    "not subscribed",
			expected: "not subscribed",
		},
		{
			name:     "no subscription",
			input:    "no subscription",
			expected: "not subscribed",
		},

		// Pattern 6: "fully subscribed"
		{
			name:     "fully subscribed",
			input:    "fully subscribed",
			expected: "fully subscribed",
		},
		{
			name:     "full subscription",
			input:    "full subscription",
			expected: "fully subscribed",
		},

		// Pattern 7: Percentage format
		{
			name:     "150% format",
			input:    "150%",
			expected: "150% subscribed",
		},
		{
			name:     "75% format",
			input:    "75%",
			expected: "75% subscribed",
		},
		{
			name:     "100.5% format",
			input:    "100.5%",
			expected: "100.5% subscribed",
		},

		// Pattern 8: Just a number
		{
			name:     "Just number 10",
			input:    "10",
			expected: "10x subscribed",
		},
		{
			name:     "Just number 2.5",
			input:    "2.5",
			expected: "2.5x subscribed",
		},

		// Edge cases
		{
			name:     "Empty string",
			input:    "",
			expected: "Not Available",
		},
		{
			name:     "Mixed case with extra spaces",
			input:    "  10  TIMES  ",
			expected: "10x subscribed",
		},
		{
			name:     "Unknown format - return as is",
			input:    "pending",
			expected: "pending",
		},
		{
			name:     "Complex text - return as is",
			input:    "awaiting results",
			expected: "awaiting results",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scraper.normalizeSubscriptionStatus(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeSubscriptionStatus(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestValidateScrapedData tests the overall scraped data validation
// Validates Requirement 1.4
func TestValidateScrapedData(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{}

	tests := []struct {
		name        string
		data        *ScrapedHistoryData
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid scraped data",
			data: &ScrapedHistoryData{
				IPOName:  "Test IPO",
				IPOPrice: 100.0,
				PriceHistory: []models.GMPPriceHistoryEntry{
					{
						RecordDate:       time.Now(),
						IPOPrice:         100.0,
						GMPValue:         10.0,
						EstimatedListing: 110.0,
						ListingPercent:   10.0,
					},
				},
			},
			expectError: false,
		},
		{
			name:        "Nil data",
			data:        nil,
			expectError: true,
			errorMsg:    "scraped data is nil",
		},
		{
			name: "Empty IPO name",
			data: &ScrapedHistoryData{
				IPOName:  "",
				IPOPrice: 100.0,
				PriceHistory: []models.GMPPriceHistoryEntry{
					{
						RecordDate:       time.Now(),
						IPOPrice:         100.0,
						GMPValue:         10.0,
						EstimatedListing: 110.0,
						ListingPercent:   10.0,
					},
				},
			},
			expectError: true,
			errorMsg:    "IPO name is empty",
		},
		{
			name: "Invalid IPO price",
			data: &ScrapedHistoryData{
				IPOName:  "Test IPO",
				IPOPrice: -100.0,
				PriceHistory: []models.GMPPriceHistoryEntry{
					{
						RecordDate:       time.Now(),
						IPOPrice:         100.0,
						GMPValue:         10.0,
						EstimatedListing: 110.0,
						ListingPercent:   10.0,
					},
				},
			},
			expectError: true,
			errorMsg:    "IPO price must be positive",
		},
		{
			name: "No price history entries",
			data: &ScrapedHistoryData{
				IPOName:      "Test IPO",
				IPOPrice:     100.0,
				PriceHistory: []models.GMPPriceHistoryEntry{},
			},
			expectError: true,
			errorMsg:    "no price history entries found",
		},
		{
			name: "Invalid entry in price history",
			data: &ScrapedHistoryData{
				IPOName:  "Test IPO",
				IPOPrice: 100.0,
				PriceHistory: []models.GMPPriceHistoryEntry{
					{
						RecordDate:       time.Now(),
						IPOPrice:         100.0,
						GMPValue:         -10.0, // Invalid negative GMP
						EstimatedListing: 90.0,
						ListingPercent:   -10.0,
					},
				},
			},
			expectError: true,
			errorMsg:    "validation failed for entry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := scraper.ValidateScrapedData(tt.data)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got no error", tt.errorMsg)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', but got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}
