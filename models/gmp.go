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

// GMPPriceHistoryEntry represents a single historical GMP price record
type GMPPriceHistoryEntry struct {
	ID                 string    `json:"id" db:"id"`
	IPOID              string    `json:"ipo_id" db:"ipo_id"`
	CompanyCode        string    `json:"company_code" db:"company_code"`
	RecordDate         time.Time `json:"record_date" db:"record_date"`
	IPOPrice           float64   `json:"ipo_price" db:"ipo_price"`
	GMPValue           float64   `json:"gmp_value" db:"gmp_value"`
	EstimatedListing   float64   `json:"estimated_listing" db:"estimated_listing"`
	ListingPercent     float64   `json:"listing_percent" db:"listing_percent"`
	EstimatedProfit    float64   `json:"estimated_profit" db:"estimated_profit"`
	SubscriptionStatus string    `json:"subscription_status" db:"subscription_status"`
	Sub2Sauda          float64   `json:"sub2_sauda" db:"sub2_sauda"`
	LastUpdated        string    `json:"last_updated" db:"last_updated"`
	DataSource         string    `json:"data_source" db:"data_source"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
}

// GMPPriceHistoryCollection represents a collection of historical price entries for an IPO
type GMPPriceHistoryCollection struct {
	IPOID        string                 `json:"ipo_id"`
	IPOName      string                 `json:"ipo_name"`
	CompanyCode  string                 `json:"company_code"`
	TotalRecords int                    `json:"total_records"`
	DateRange    *DateRange             `json:"date_range"`
	Entries      []GMPPriceHistoryEntry `json:"entries"`
	Metadata     *CollectionMetadata    `json:"metadata"`
}

// DateRange represents a date range for filtering historical data
type DateRange struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

// CollectionMetadata contains metadata about the price history collection
type CollectionMetadata struct {
	LastScraped     time.Time `json:"last_scraped"`
	DataSource      string    `json:"data_source"`
	ScrapingSuccess bool      `json:"scraping_success"`
	ErrorCount      int       `json:"error_count"`
	ProcessingTime  string    `json:"processing_time"`
}

// ChartDataResponse represents the API response optimized for chart visualization
type ChartDataResponse struct {
	IPOInfo    IPOBasicInfo    `json:"ipo_info"`
	ChartData  []ChartPoint    `json:"chart_data"`
	Statistics ChartStatistics `json:"statistics"`
	Metadata   ChartMetadata   `json:"metadata"`
}

// IPOBasicInfo contains basic IPO information for chart context
type IPOBasicInfo struct {
	IPOID       string  `json:"ipo_id"`
	IPOName     string  `json:"ipo_name"`
	CompanyCode string  `json:"company_code"`
	IPOPrice    float64 `json:"ipo_price"`
	Status      string  `json:"status"`
}

// ChartPoint represents a single data point for chart visualization
type ChartPoint struct {
	Date             string  `json:"date"`
	GMPValue         float64 `json:"gmp_value"`
	EstimatedListing float64 `json:"estimated_listing"`
	ListingPercent   float64 `json:"listing_percent"`
}

// ChartStatistics contains statistical summary of the price history
type ChartStatistics struct {
	MaxGMP         float64 `json:"max_gmp"`
	MinGMP         float64 `json:"min_gmp"`
	AverageGMP     float64 `json:"average_gmp"`
	LatestGMP      float64 `json:"latest_gmp"`
	TrendDirection string  `json:"trend_direction"` // "up", "down", "stable"
}

// ChartMetadata contains metadata for the chart data response
type ChartMetadata struct {
	DataSource     string    `json:"data_source"`
	LastUpdated    time.Time `json:"last_updated"`
	TotalRecords   int       `json:"total_records"`
	DateRangeStart string    `json:"date_range_start,omitempty"`
	DateRangeEnd   string    `json:"date_range_end,omitempty"`
	CurrentPage    int       `json:"current_page,omitempty"`
	PageSize       int       `json:"page_size,omitempty"`
	TotalPages     int       `json:"total_pages,omitempty"`
}

// GMPHistoryJobLog represents a job execution log entry
type GMPHistoryJobLog struct {
	ID                string     `json:"id" db:"id"`
	JobStartTime      time.Time  `json:"job_start_time" db:"job_start_time"`
	JobEndTime        *time.Time `json:"job_end_time,omitempty" db:"job_end_time"`
	IPOsProcessed     int        `json:"ipos_processed" db:"ipos_processed"`
	SuccessfulScrapes int        `json:"successful_scrapes" db:"successful_scrapes"`
	FailedScrapes     int        `json:"failed_scrapes" db:"failed_scrapes"`
	TotalRecordsAdded int        `json:"total_records_added" db:"total_records_added"`
	ExecutionStatus   string     `json:"execution_status" db:"execution_status"`
	ErrorSummary      string     `json:"error_summary,omitempty" db:"error_summary"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
}

// IPOProcessingResult represents the result of processing a single IPO
// Used for tracking both successful and failed processing attempts
type IPOProcessingResult struct {
	IPOID          string
	CompanyCode    string
	IPOName        string
	Success        bool
	RecordsAdded   int
	ErrorMessage   string
	ProcessingTime time.Duration
}

// ProcessingResults aggregates results from processing multiple IPOs
type ProcessingResults struct {
	SuccessfulIPOs []GMPPriceHistoryCollection
	FailedIPOs     []IPOProcessingResult
	TotalProcessed int
	SuccessCount   int
	FailureCount   int
}
