package models

import "time"

// IPOWithGMP represents an IPO with optional GMP data joined from the ipo_gmp table.
// This model is used by API handlers to return combined IPO and GMP information.
// GMP fields are nullable to handle cases where GMP data is not available.
type IPOWithGMP struct {
	// Embed all IPO fields
	IPO

	// Enhanced GMP fields from ipo_gmp table (nullable)
	GMPValue         *float64   `json:"gmp_value,omitempty"`
	GainPercent      *float64   `json:"gain_percent,omitempty"`
	EstimatedListing *float64   `json:"estimated_listing,omitempty"`
	GMPLastUpdated   *time.Time `json:"gmp_last_updated,omitempty"`

	// New enhanced GMP fields
	GMPStockID            *string             `json:"gmp_stock_id,omitempty"`
	GMPSubscriptionStatus *string             `json:"gmp_subscription_status,omitempty"`
	GMPListingGain        *string             `json:"gmp_listing_gain,omitempty"`
	GMPIPOStatus          *string             `json:"gmp_ipo_status,omitempty"`
	GMPDataSource         *string             `json:"gmp_data_source,omitempty"`
	GMPExtractionMetadata *ExtractionMetadata `json:"gmp_extraction_metadata,omitempty"`
}
