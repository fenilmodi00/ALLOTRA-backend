package models

import "time"

type GMPData struct {
	ID               string     `json:"id"`
	IPOName          string     `json:"ipo_name"`
	CompanyCode      string     `json:"company_code"`
	IPOPrice         float64    `json:"ipo_price"`
	GMPValue         float64    `json:"gmp_value"`
	EstimatedListing float64    `json:"estimated_listing"`
	GainPercent      float64    `json:"gain_percent"`
	Sub2             float64    `json:"sub2"`
	Kostak           float64    `json:"kostak"`
	ListingDate      *time.Time `json:"listing_date,omitempty"`
	LastUpdated      time.Time  `json:"last_updated"`
}

// EnhancedGMPData represents the enhanced GMP data structure with new fields
type EnhancedGMPData struct {
	// Existing fields
	ID               string     `json:"id"`
	IPOName          string     `json:"ipo_name"`
	CompanyCode      string     `json:"company_code"`
	IPOPrice         float64    `json:"ipo_price"`
	GMPValue         float64    `json:"gmp_value"`
	EstimatedListing float64    `json:"estimated_listing"`
	GainPercent      float64    `json:"gain_percent"`
	Sub2             float64    `json:"sub2"`
	Kostak           float64    `json:"kostak"`
	ListingDate      *time.Time `json:"listing_date,omitempty"`
	LastUpdated      time.Time  `json:"last_updated"`

	// New enhanced fields
	StockID            *string             `json:"stock_id"`            // Link to IPO table
	SubscriptionStatus *string             `json:"subscription_status"` // e.g., "10.5x subscribed"
	ListingGain        *string             `json:"listing_gain"`        // e.g., "+15.2%", "-5.8%"
	Rating             *int                `json:"rating"`              // Fire icons rating (1-5)
	UpdatedOn          *string             `json:"updated_on"`          // Last updated timestamp text
	IPOStatus          *string             `json:"ipo_status"`          // Upcoming, Open, Listed
	DataSource         string              `json:"data_source"`         // "investorgain.com"
	ExtractionMetadata *ExtractionMetadata `json:"extraction_metadata,omitempty"`
}

// ExtractionMetadata tracks parsing success and metadata for GMP extraction
type ExtractionMetadata struct {
	ExtractedFields   []string  `json:"extracted_fields"`
	FailedFields      []string  `json:"failed_fields"`
	ParsingConfidence float64   `json:"parsing_confidence"`
	TableStructure    string    `json:"table_structure"`
	LastSuccessfulRun time.Time `json:"last_successful_run"`
}

// StockIDCache represents cached stock ID resolution results
type StockIDCache struct {
	GMPName     string    `json:"gmp_name"`
	StockID     string    `json:"stock_id"`
	CompanyCode string    `json:"company_code"`
	MatchMethod string    `json:"match_method"`
	Confidence  float64   `json:"confidence"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}
