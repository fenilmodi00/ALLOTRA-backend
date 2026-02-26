package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// GMPExtractionMetrics tracks success rates and performance of GMP data extraction
type GMPExtractionMetrics struct {
	TotalAttempts    int `json:"total_attempts"`
	SuccessfulParsed int `json:"successful_parsed"`
	FailedParsed     int `json:"failed_parsed"`
	HTTPErrors       int `json:"http_errors"`
	ProcessingErrors int `json:"processing_errors"`
}

// NewGMPExtractionMetrics creates a new GMP extraction metrics tracker
func NewGMPExtractionMetrics() *GMPExtractionMetrics {
	return &GMPExtractionMetrics{}
}

// RecordAttempt records a GMP extraction attempt
func (m *GMPExtractionMetrics) RecordAttempt(success bool) {
	m.TotalAttempts++
	if success {
		m.SuccessfulParsed++
	} else {
		m.FailedParsed++
	}
}

// RecordHTTPError records an HTTP error
func (m *GMPExtractionMetrics) RecordHTTPError() {
	m.HTTPErrors++
}

// RecordProcessingError records a processing error
func (m *GMPExtractionMetrics) RecordProcessingError() {
	m.ProcessingErrors++
}

// GetSuccessRate returns the success rate as a percentage
func (m *GMPExtractionMetrics) GetSuccessRate() float64 {
	if m.TotalAttempts == 0 {
		return 0.0
	}
	return float64(m.SuccessfulParsed) / float64(m.TotalAttempts) * 100.0
}

// LogSummary logs a comprehensive GMP extraction metrics summary
func (m *GMPExtractionMetrics) LogSummary() {
	logrus.WithFields(logrus.Fields{
		"total_attempts":    m.TotalAttempts,
		"successful_parsed": m.SuccessfulParsed,
		"failed_parsed":     m.FailedParsed,
		"success_rate":      m.GetSuccessRate(),
		"http_errors":       m.HTTPErrors,
		"processing_errors": m.ProcessingErrors,
	}).Info("GMP extraction metrics summary")
}

// EnhancedGMPService implements the enhanced scraper architecture patterns
type EnhancedGMPService struct {
	baseURL            string
	db                 *sql.DB
	httpClient         *http.Client
	requestRateLimiter *shared.HTTPRequestRateLimiter
	utilityService     *UtilityService
	configuration      *shared.ServiceConfig
	extractionMetrics  *GMPExtractionMetrics
	serviceMetrics     *shared.ServiceMetrics
	httpClientFactory  *shared.HTTPClientFactory
}

// NewEnhancedGMPService creates a new enhanced GMP service with configuration-driven initialization
func NewEnhancedGMPService(config *shared.ServiceConfig, db *sql.DB) *EnhancedGMPService {
	if config == nil {
		gmpConfig := shared.NewGMPServiceConfig()
		// Ensure we use the 'all' view for scraping
		if !strings.HasSuffix(gmpConfig.BaseURL, "all/") {
			if strings.HasSuffix(gmpConfig.BaseURL, "/") {
				gmpConfig.BaseURL += "all/"
			} else {
				gmpConfig.BaseURL += "/all/"
			}
		}
		config = &gmpConfig
	}

	// Create HTTP client factory and optimized client
	httpClientFactory := shared.NewHTTPClientFactory(config.HTTPRequestTimeout)
	httpClient := httpClientFactory.CreateOptimizedHTTPClient(config.HTTPRequestTimeout)

	// Create service metrics if enabled
	var serviceMetrics *shared.ServiceMetrics
	if config.EnableMetrics {
		serviceMetrics = shared.NewServiceMetrics("GMP_Service")
	}

	service := &EnhancedGMPService{
		baseURL:            config.BaseURL,
		db:                 db,
		httpClient:         httpClient,
		requestRateLimiter: shared.NewHTTPRequestRateLimiter(config.RequestRateLimit),
		utilityService:     NewUtilityService(),
		configuration:      config,
		extractionMetrics:  NewGMPExtractionMetrics(),
		serviceMetrics:     serviceMetrics,
		httpClientFactory:  httpClientFactory,
	}

	logrus.WithFields(logrus.Fields{
		"component":    "EnhancedGMPService",
		"base_url":     service.baseURL,
		"http_timeout": config.HTTPRequestTimeout,
		"rate_limit":   config.RequestRateLimit,
		"db_enabled":   db != nil,
	}).Info("Enhanced GMP service initialized with configuration-driven architecture")

	return service
}

// GMPService maintains backward compatibility
type GMPService struct {
	*EnhancedGMPService
}

// NewGMPService creates a new GMP service with enhanced architecture (backward compatible)
func NewGMPService() *GMPService {
	enhanced := NewEnhancedGMPService(nil, nil)
	return &GMPService{
		EnhancedGMPService: enhanced,
	}
}

// NewGMPServiceWithDB creates a new GMP service with database support for enhanced features
func NewGMPServiceWithDB(db *sql.DB) *GMPService {
	enhanced := NewEnhancedGMPService(nil, db)
	return &GMPService{
		EnhancedGMPService: enhanced,
	}
}

// GMPScrapingResult represents the raw scraped data from InvestorGain
type GMPScrapingResult struct {
	CompanyName     string  `json:"company_name"`
	Exchange        string  `json:"exchange"`       // BSE SME, NSE SME, etc.
	Status          string  `json:"status"`         // U, O, C (Upcoming, Open, Closed)
	GMPValue        float64 `json:"gmp_value"`      // ₹25
	GMPPercentage   float64 `json:"gmp_percentage"` // 30.86%
	LowValue        float64 `json:"low_value"`      // L/H (₹): 25 ↓ / 25 ↑
	HighValue       float64 `json:"high_value"`
	Rating          int     `json:"rating"`           // Number of fire icons (1-5)
	Subscription    string  `json:"subscription"`     // 5.6x, 526.56x, or "-"
	IPOPrice        float64 `json:"ipo_price"`        // Calculated from GMP percentage
	UpdatedOn       string  `json:"updated_on"`       // Raw updated text
	ListingGain     string  `json:"listing_gain"`     // Listing gain percentage like "+15.2%" or "-5.8%"
	RatingText      string  `json:"rating_text"`      // Raw rating text with fire emojis
	SubscriptionRaw string  `json:"subscription_raw"` // Raw subscription text for better parsing
}

// FetchGMPData scrapes the GMP table from InvestorGain using chromedp with enhanced architecture
func (s *EnhancedGMPService) FetchGMPData() ([]models.EnhancedGMPData, error) {
	startTime := time.Now()

	logger := logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"method":    "FetchGMPData",
		"base_url":  s.baseURL,
	})

	logger.Info("Starting GMP data extraction with enhanced architecture")

	// Record metrics if enabled
	defer func() {
		processingTime := time.Since(startTime)
		if s.serviceMetrics != nil {
			s.serviceMetrics.RecordRequest(true, processingTime)
		}
		logger.WithField("processing_time", processingTime).Debug("GMP data extraction completed")
	}()

	// Enforce rate limiting
	s.requestRateLimiter.EnforceRateLimit()

	// Record extraction attempt
	s.extractionMetrics.RecordAttempt(false) // Will be updated to true on success (partially)

	// Scrape raw data
	rawData, err := s.scrapeInvestorGainData()
	if err != nil {
		s.extractionMetrics.RecordHTTPError()
		if s.serviceMetrics != nil {
			s.serviceMetrics.RecordRequest(false, time.Since(startTime))
		}

		wrappedError := shared.NewServiceError(
			shared.ErrorCategoryNetwork,
			"CHROMEDP_SCRAPING_FAILED",
			"Failed to scrape GMP data with chromedp",
			"GMP_Service",
			"FetchGMPData",
			true,
			err,
		)
		wrappedError.LogError()
		return nil, wrappedError
	}

	logger.WithField("raw_records", len(rawData)).Info("Successfully scraped raw GMP data")

	// Convert to enhanced GMP data
	var gmpList []models.EnhancedGMPData
	successfulRecords := 0
	processingErrors := 0

	for i, raw := range rawData {
		enhanced := s.convertToEnhancedGMP(raw, i)
		if enhanced != nil {
			gmpList = append(gmpList, *enhanced)
			successfulRecords++
			// Record success for this item
			s.extractionMetrics.SuccessfulParsed++
		} else {
			processingErrors++
			s.extractionMetrics.RecordProcessingError()
		}
	}
	// Correct total attempts since we are batch processing
	s.extractionMetrics.TotalAttempts += len(rawData)
	// We incremented successful parsed in loop, failed parsed can be inferred or we can adjust
	s.extractionMetrics.FailedParsed += processingErrors


	successRate := 0.0
	if len(rawData) > 0 {
		successRate = float64(successfulRecords) / float64(len(rawData)) * 100.0
	}

	logger.WithFields(logrus.Fields{
		"total_raw_records":  len(rawData),
		"successful_records": successfulRecords,
		"processing_errors":  processingErrors,
		"success_rate":       successRate,
		"processing_time":    time.Since(startTime),
	}).Info("Successfully completed GMP data extraction")

	return gmpList, nil
}

// FetchGMPData maintains backward compatibility
func (s *GMPService) FetchGMPData() ([]models.EnhancedGMPData, error) {
	return s.EnhancedGMPService.FetchGMPData()
}

// scrapeInvestorGainData performs the actual web scraping
func (s *EnhancedGMPService) scrapeInvestorGainData() ([]GMPScrapingResult, error) {
	// Setup Chrome with minimal options for speed
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-images", true),
		chromedp.Flag("disable-javascript", false), // Need JS for dynamic content
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, s.configuration.HTTPRequestTimeout)
	defer cancel()

	var rawTableData []map[string]interface{}
	var updatedOnText string

	// Navigate and extract data efficiently
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(s.baseURL),

		// Wait for table and extract data in one go
		chromedp.WaitVisible("table tbody tr", chromedp.ByQuery),
		chromedp.Sleep(5*time.Second), // Increased wait time for dynamic content

		// Extract updated timestamp
		chromedp.Evaluate(`
			(function() {
				const elements = document.querySelectorAll('*');
				for (let el of elements) {
					const text = el.textContent || '';
					if (text.toLowerCase().includes('updated') && text.match(/\d{1,2}[-/]\w{3}|\d{1,2}:\d{2}/)) {
						return text.trim();
					}
				}
				return '';
			})();
		`, &updatedOnText),

		// Extract table data with improved parsing for the actual table structure
		chromedp.Evaluate(`
			(function() {
				// Find the main data table by ID
				const dataTable = document.getElementById('report_table');
				if (!dataTable) {
					console.log('No report_table found');
					return [];
				}

				const tbody = dataTable.querySelector('tbody');
				if (!tbody) {
					console.log('No tbody found in report_table');
					return [];
				}

				const rows = Array.from(tbody.querySelectorAll('tr'));
				console.log('Found data rows:', rows.length);

				return rows.map((row, index) => {
					const cells = Array.from(row.querySelectorAll('td'));
					if (cells.length < 3) return null; // Skip incomplete rows

					// Based on the table structure we saw:
					// Column 0: Name (Company name with status indicators)
					// Column 1: GMP (GMP value and percentage)
					// Column 2: Rating (Fire emojis)
					// Column 3: Subscription (subscription multiplier)
					// Additional columns may contain other data

					const nameCell = cells[0] ? cells[0].textContent.trim() : '';
					const gmpCell = cells[1] ? cells[1].textContent.trim() : '';
					const ratingCell = cells[2] ? cells[2].textContent.trim() : '';
					const subscriptionCell = cells[3] ? cells[3].textContent.trim() : '';

					// Extract company name (remove status indicators and exchange info)
					let companyName = nameCell;
					companyName = companyName.replace(/\s*(BSE|NSE)\s*(SME)?\s*[UOC]?\s*$/i, '').trim();
					companyName = companyName.replace(/\s*IPO\s*$/i, '').trim();

					// Extract status from name cell
					let status = '';
					const statusMatch = nameCell.match(/\b([UOC])\b/);
					if (statusMatch) status = statusMatch[1];

					// Extract exchange info
					let exchange = '';
					if (nameCell.includes('BSE SME')) exchange = 'BSE SME';
					else if (nameCell.includes('NSE SME')) exchange = 'NSE SME';
					else if (nameCell.includes('BSE')) exchange = 'BSE';
					else if (nameCell.includes('NSE')) exchange = 'NSE';

					// Count fire emojis for rating
					const fireCount = (ratingCell.match(/🔥/g) || []).length;

					// Clean subscription data
					let subscription = subscriptionCell || '-';
					const subMatch = subscription.match(/(\d+(?:\.\d+)?x)/i);
					if (subMatch) {
						subscription = subMatch[1];
					}

					// Look for listing gain in any cell
					let listingGain = '';
					for (let i = 0; i < cells.length; i++) {
						const cellText = cells[i].textContent.trim();
						const gainMatch = cellText.match(/([+-]\d+(?:\.\d+)?%)/);
						if (gainMatch && !cellText.includes('GMP')) {
							listingGain = gainMatch[1];
							break;
						}
					}

					console.log('Row', index, ':', {
						name: companyName,
						gmp: gmpCell,
						rating: fireCount,
						subscription: subscription,
						status: status,
						exchange: exchange
					});

					return {
						companyName: companyName,
						exchange: exchange,
						status: status,
						gmpText: gmpCell,
						lowHighText: '', // Not easily available in this format
						rating: fireCount,
						ratingText: ratingCell,
						subscription: subscription,
						subscriptionRaw: subscriptionCell,
						listingGain: listingGain
					};
				}).filter(item => item && item.companyName && item.companyName.length > 2);
			})();
		`, &rawTableData),
	)

	if err != nil {
		return nil, fmt.Errorf("chromedp execution failed: %w", err)
	}

	// Convert raw data to structured format
	var results []GMPScrapingResult
	for _, item := range rawTableData {
		result := GMPScrapingResult{
			UpdatedOn: updatedOnText,
		}

		// Extract string fields
		if name, ok := item["companyName"].(string); ok {
			result.CompanyName = s.cleanCompanyName(name)
		}
		if exchange, ok := item["exchange"].(string); ok {
			result.Exchange = exchange
		}
		if status, ok := item["status"].(string); ok {
			result.Status = status
		}
		if sub, ok := item["subscription"].(string); ok {
			result.Subscription = sub
		}
		if subRaw, ok := item["subscriptionRaw"].(string); ok {
			result.SubscriptionRaw = subRaw
		}
		if ratingText, ok := item["ratingText"].(string); ok {
			result.RatingText = ratingText
		}
		if listingGain, ok := item["listingGain"].(string); ok {
			result.ListingGain = listingGain
		}

		// Parse GMP data
		if gmpText, ok := item["gmpText"].(string); ok {
			result.GMPValue, result.GMPPercentage = s.parseGMPString(gmpText)
		}

		// Parse L/H data
		if lhText, ok := item["lowHighText"].(string); ok {
			result.LowValue, result.HighValue = s.parseLowHighString(lhText)
		}

		// Extract rating
		if rating, ok := item["rating"].(float64); ok {
			result.Rating = int(rating)
		}

		// Calculate IPO price from GMP percentage
		if result.GMPValue > 0 && result.GMPPercentage > 0 {
			result.IPOPrice = result.GMPValue / (result.GMPPercentage / 100)
		}

		// Parse updated timestamp
		result.UpdatedOn = updatedOnText

		results = append(results, result)
	}

	return results, nil
}

// convertToEnhancedGMP converts scraped data to EnhancedGMPData model
func (s *EnhancedGMPService) convertToEnhancedGMP(raw GMPScrapingResult, index int) *models.EnhancedGMPData {
	if raw.CompanyName == "" {
		return nil
	}

	now := time.Now()

	enhanced := &models.EnhancedGMPData{
		ID:               uuid.New().String(),
		IPOName:          raw.CompanyName,
		CompanyCode:      s.generateCompanyCode(raw.CompanyName),
		IPOPrice:         raw.IPOPrice,
		GMPValue:         raw.GMPValue,
		EstimatedListing: raw.IPOPrice + raw.GMPValue,
		GainPercent:      raw.GMPPercentage,
		Sub2:             0, // Not available from this source
		Kostak:           0, // Not available from this source
		LastUpdated:      now,
		DataSource:       "investorgain.com",
	}

	// Set subscription status - use the cleaned subscription data
	if raw.Subscription != "" && raw.Subscription != "-" {
		enhanced.SubscriptionStatus = &raw.Subscription
	}

	// Set listing gain if available
	if raw.ListingGain != "" {
		enhanced.ListingGain = &raw.ListingGain
	}

	// Set rating if available
	if raw.Rating > 0 {
		enhanced.Rating = &raw.Rating
	}

	// Set updated on timestamp
	if raw.UpdatedOn != "" {
		enhanced.UpdatedOn = &raw.UpdatedOn
	}

	// Set IPO status based on the status code
	if raw.Status != "" {
		statusMap := map[string]string{
			"U": "Upcoming",
			"O": "Open",
			"C": "Closed",
		}
		if fullStatus, exists := statusMap[raw.Status]; exists {
			enhanced.IPOStatus = &fullStatus
		}
	}

	// Create extraction metadata with all extracted fields
	extractedFields := []string{"ipo_name", "gmp_value", "gain_percent"}
	failedFields := []string{}

	if raw.GMPPercentage > 0 {
		extractedFields = append(extractedFields, "ipo_price", "estimated_listing")
	}
	if raw.Subscription != "" && raw.Subscription != "-" {
		extractedFields = append(extractedFields, "subscription_status")
	}
	if raw.ListingGain != "" {
		extractedFields = append(extractedFields, "listing_gain")
	}
	if raw.Rating > 0 {
		extractedFields = append(extractedFields, "rating")
	}
	if raw.Status != "" {
		extractedFields = append(extractedFields, "ipo_status")
	}

	// Check for missing critical fields
	if raw.Subscription == "" || raw.Subscription == "-" {
		failedFields = append(failedFields, "subscription_status")
	}
	if raw.ListingGain == "" {
		failedFields = append(failedFields, "listing_gain")
	}
	if raw.Rating == 0 {
		failedFields = append(failedFields, "rating")
	}

	enhanced.ExtractionMetadata = &models.ExtractionMetadata{
		ExtractedFields:   extractedFields,
		FailedFields:      failedFields,
		ParsingConfidence: s.calculateConfidence(raw),
		TableStructure:    "investorgain_standard",
		LastSuccessfulRun: now,
	}

	return enhanced
}

// Helper methods for parsing

func (s *EnhancedGMPService) cleanCompanyName(name string) string {
	name = strings.TrimSpace(name)
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")

	// Remove exchange suffixes
	suffixes := []string{"BSE SME", "NSE SME", "BSE", "NSE", "IPO"}
	for _, suffix := range suffixes {
		name = strings.TrimSuffix(name, " "+suffix)
	}

	return name
}

// generateCompanyCode produces a slug-style company code matching ipo_list format.
// e.g. "Omnitech Engineering IPO U" → "omnitech-engineering"
func (s *EnhancedGMPService) generateCompanyCode(companyName string) string {
	if companyName == "" {
		return ""
	}

	name := companyName

	// Strip listing info like "L@876.00 (-2.67%)" or "CAllotted"
	name = regexp.MustCompile(`L@[\d.,]+\s*\([^)]*\)`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`C?Allotted`).ReplaceAllString(name, "")

	// Strip trailing status / exchange markers (order matters: longer first)
	stripSuffixes := []string{
		"BSE SME", "NSE SME", "BSE", "NSE",
		"IPO U", "IPO O", "IPO C", "IPO",
		"Ltd.", "Ltd", "Limited", "Pvt.", "Pvt",
		"& Co.", "& Co",
		"Details", // e.g. "Yaap Digital IPO Details"
	}
	for _, sfx := range stripSuffixes {
		// Case-insensitive suffix removal
		re := regexp.MustCompile(`(?i)\s*` + regexp.QuoteMeta(sfx) + `\s*$`)
		name = re.ReplaceAllString(name, "")
	}

	// Trim, lowercase, replace non-alphanumeric runs with a hyphen
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")

	return name
}

func (s *EnhancedGMPService) parseGMPString(gmpText string) (float64, float64) {
	if gmpText == "" {
		return 0, 0
	}

	// Clean text
	gmpText = strings.ReplaceAll(gmpText, "₹", "")
	gmpText = strings.ReplaceAll(gmpText, ",", "")
	gmpText = strings.TrimSpace(gmpText)

	// Pattern: "25 (30.86%)" or "145 (83.33%)"
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*\((\d+(?:\.\d+)?)%\)`)
	matches := re.FindStringSubmatch(gmpText)

	if len(matches) >= 3 {
		value, _ := strconv.ParseFloat(matches[1], 64)
		percentage, _ := strconv.ParseFloat(matches[2], 64)
		return value, percentage
	}

	// Fallback: extract just the number
	re = regexp.MustCompile(`(\d+(?:\.\d+)?)`)
	matches = re.FindStringSubmatch(gmpText)
	if len(matches) >= 1 {
		value, _ := strconv.ParseFloat(matches[1], 64)
		return value, 0
	}

	return 0, 0
}

func (s *EnhancedGMPService) parseLowHighString(lhText string) (float64, float64) {
	if lhText == "" {
		return 0, 0
	}

	// Clean text
	lhText = strings.ReplaceAll(lhText, "₹", "")
	lhText = strings.ReplaceAll(lhText, "L/H (₹):", "")
	lhText = strings.ReplaceAll(lhText, ",", "")
	lhText = strings.TrimSpace(lhText)

	// Pattern: "25 ↓ / 25 ↑" or "65 ↓ / 145 ↑"
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*[↓▼⬇]\s*/\s*(\d+(?:\.\d+)?)\s*[↑▲⬆]`)
	matches := re.FindStringSubmatch(lhText)

	if len(matches) >= 3 {
		low, _ := strconv.ParseFloat(matches[1], 64)
		high, _ := strconv.ParseFloat(matches[2], 64)
		return low, high
	}

	return 0, 0
}

func (s *EnhancedGMPService) calculateConfidence(raw GMPScrapingResult) float64 {
	confidence := 0.0

	// Base confidence for having company name
	if raw.CompanyName != "" {
		confidence += 25.0
	}

	// GMP value confidence
	if raw.GMPValue > 0 {
		confidence += 30.0
	}

	// GMP percentage confidence
	if raw.GMPPercentage > 0 {
		confidence += 20.0
	}

	// Subscription data confidence
	if raw.Subscription != "" && raw.Subscription != "-" {
		confidence += 10.0
	}

	// Rating confidence
	if raw.Rating > 0 {
		confidence += 5.0
	}

	// Listing gain confidence
	if raw.ListingGain != "" {
		confidence += 5.0
	}

	// Status confidence
	if raw.Status != "" {
		confidence += 5.0
	}

	return confidence
}

// SaveGMPData saves GMP data to database efficiently
func (s *EnhancedGMPService) SaveGMPData(gmpList []models.EnhancedGMPData) error {
	if s.db == nil {
		logrus.WithField("component", "EnhancedGMPService").Warn("Database not available, skipping save")
		return nil
	}

	if len(gmpList) == 0 {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"records": len(gmpList),
	}).Info("Saving GMP data to database")

	// Use transaction for efficiency
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare insert statement with all fields
	stmt, err := tx.Prepare(`
		INSERT INTO ipo_gmp (
			id, ipo_name, ipo_id, company_code, ipo_price, gmp_value,
			estimated_listing, gain_percent, sub2, kostak, last_updated,
			data_source, stock_id, subscription_status, listing_gain,
			ipo_status, extraction_metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (ipo_name) DO UPDATE SET
			ipo_id      = COALESCE(EXCLUDED.ipo_id, ipo_gmp.ipo_id),
			company_code = COALESCE(EXCLUDED.company_code, ipo_gmp.company_code),
			stock_id    = COALESCE(EXCLUDED.stock_id, ipo_gmp.stock_id),
			gmp_value = EXCLUDED.gmp_value,
			gain_percent = EXCLUDED.gain_percent,
			estimated_listing = EXCLUDED.estimated_listing,
			subscription_status = EXCLUDED.subscription_status,
			listing_gain = EXCLUDED.listing_gain,
			ipo_status = EXCLUDED.ipo_status,
			extraction_metadata = EXCLUDED.extraction_metadata,
			last_updated = EXCLUDED.last_updated
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert/update records
	for _, gmp := range gmpList {
		// Convert extraction metadata to JSON
		var metadataJSON []byte
		if gmp.ExtractionMetadata != nil {
			metadataJSON, _ = json.Marshal(gmp.ExtractionMetadata)
		}

		_, err := stmt.Exec(
			gmp.ID, gmp.IPOName, gmp.IPOID, gmp.CompanyCode, gmp.IPOPrice,
			gmp.GMPValue, gmp.EstimatedListing, gmp.GainPercent,
			gmp.Sub2, gmp.Kostak, gmp.LastUpdated, gmp.DataSource,
			gmp.StockID, gmp.SubscriptionStatus, gmp.ListingGain,
			gmp.IPOStatus, string(metadataJSON),
		)
		if err != nil {
			logrus.WithField("component", "EnhancedGMPService").WithError(err).WithField("company", gmp.IPOName).Error("Failed to save GMP record")
			return fmt.Errorf("failed to save GMP record for %s: %w", gmp.IPOName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"records": len(gmpList),
	}).Info("Successfully saved GMP data")
	return nil
}

// FetchAndSaveGMPData combines fetching and saving in one operation
func (s *EnhancedGMPService) FetchAndSaveGMPData() ([]models.EnhancedGMPData, error) {
	gmpData, err := s.FetchGMPData()
	if err != nil {
		return nil, err
	}

	// Reconcile company_code and stock_id against ipo_list before saving
	if s.db != nil {
		s.reconcileWithIPOList(gmpData)
	}

	if err := s.SaveGMPData(gmpData); err != nil {
		logrus.WithField("component", "EnhancedGMPService").WithError(err).Warn("Failed to save GMP data, but returning scraped data")
	}

	return gmpData, nil
}

// FetchAndSaveGMPData maintains backward compatibility
func (s *GMPService) FetchAndSaveGMPData() ([]models.EnhancedGMPData, error) {
	return s.EnhancedGMPService.FetchAndSaveGMPData()
}

// reconcileWithIPOList loads all ipo_list rows and matches each GMP record by
// slug-normalised company name, then populates the correct company_code, ipo_id
// and stock_id so the JOIN in GetActiveIPOsWithGMP succeeds.
func (s *EnhancedGMPService) reconcileWithIPOList(gmpData []models.EnhancedGMPData) {
	type ipoRow struct {
		id          string
		companyCode string
		stockID     string
		name        string
		slug        string // normalised for matching
	}

	rows, err := s.db.Query(`SELECT id, company_code, COALESCE(stock_id, ''), name FROM ipo_list`)
	if err != nil {
		logrus.WithField("component", "EnhancedGMPService").WithError(err).Warn("reconcileWithIPOList: failed to load ipo_list")
		return
	}
	defer rows.Close()

	var ipos []ipoRow
	for rows.Next() {
		var r ipoRow
		if err := rows.Scan(&r.id, &r.companyCode, &r.stockID, &r.name); err != nil {
			continue
		}
		r.slug = s.normaliseForMatch(r.name)
		ipos = append(ipos, r)
	}

	// Build a lookup map: slug → ipoRow
	bySlug := make(map[string]ipoRow, len(ipos))
	for _, r := range ipos {
		bySlug[r.slug] = r
	}

	matched, unmatched := 0, 0
	for i := range gmpData {
		gmpSlug := s.normaliseForMatch(gmpData[i].IPOName)
		match, ok := bySlug[gmpSlug]
		if !ok {
			// Try matching against the already-generated company_code slug
			match, ok = bySlug[gmpData[i].CompanyCode]
		}
		if !ok {
			// Partial prefix match as last resort
			for _, r := range ipos {
				if strings.HasPrefix(r.slug, gmpSlug) || strings.HasPrefix(gmpSlug, r.slug) {
					match = r
					ok = true
					break
				}
			}
		}

		if ok {
			gmpData[i].CompanyCode = match.companyCode
			if match.id != "" {
				gmpData[i].IPOID = &match.id
			}
			if match.stockID != "" {
				gmpData[i].StockID = &match.stockID
			}
			matched++
		} else {
			unmatched++
			logrus.WithFields(logrus.Fields{
				"component": "EnhancedGMPService",
				"ipo_name": gmpData[i].IPOName,
				"slug":     gmpSlug,
			}).Debug("reconcileWithIPOList: no match found in ipo_list")
		}
	}

	logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"matched":   matched,
		"unmatched": unmatched,
	}).Info("reconcileWithIPOList: finished matching GMP records to ipo_list")
}

// normaliseForMatch produces a comparable slug from any company name string,
// stripping common suffixes, punctuation and status markers.
func (s *EnhancedGMPService) normaliseForMatch(name string) string {
	name = strings.TrimSpace(name)

	// Strip listing info e.g. "L@876.00 (-2.67%)"
	name = regexp.MustCompile(`L@[\d.,]+\s*\([^)]*\)`).ReplaceAllString(name, "")
	// Strip allotment markers
	name = regexp.MustCompile(`C?Allotted`).ReplaceAllString(name, "")

	stripSuffixes := []string{
		"BSE SME", "NSE SME", "BSE", "NSE",
		"IPO U", "IPO O", "IPO C", "IPO",
		"Ltd.", "Ltd", "Limited", "Pvt.", "Pvt",
		"& Co.", "& Co",
		"Details",
		"India", // avoid over-stripping; only as suffix
	}
	for _, sfx := range stripSuffixes {
		re := regexp.MustCompile(`(?i)\s*` + regexp.QuoteMeta(sfx) + `\s*$`)
		name = re.ReplaceAllString(name, "")
	}

	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(name, "")
	return name
}

// GetConfiguration returns the current service configuration
func (s *EnhancedGMPService) GetConfiguration() *shared.ServiceConfig {
	return s.configuration
}

// GetExtractionMetrics returns the current extraction metrics
func (s *EnhancedGMPService) GetExtractionMetrics() *GMPExtractionMetrics {
	return s.extractionMetrics
}

// GetServiceMetrics returns the current service metrics
func (s *EnhancedGMPService) GetServiceMetrics() *shared.ServiceMetrics {
	return s.serviceMetrics
}

// LogMetricsSummary logs comprehensive metrics summary
func (s *EnhancedGMPService) LogMetricsSummary() {
	s.extractionMetrics.LogSummary()
	if s.serviceMetrics != nil {
		s.serviceMetrics.LogSummary()
	}
}

// Cleanup properly cleans up service resources
func (s *EnhancedGMPService) Cleanup() {
	logger := logrus.WithField("component", "EnhancedGMPService")

	// Cleanup HTTP client resources
	if s.httpClientFactory != nil {
		s.httpClientFactory.CleanupHTTPClient(s.httpClient)
		logger.Debug("Cleaned up HTTP client resources")
	}

	// Log final metrics summary
	s.LogMetricsSummary()

	logger.Info("Enhanced GMP service cleanup completed")
}
