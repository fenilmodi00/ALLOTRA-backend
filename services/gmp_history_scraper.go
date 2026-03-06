package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	chromeDPWaitTime = 5 * time.Second
	sharesPerLot     = 1000
)

// GMPPriceHistoryScraper handles web scraping of GMP price history from InvestorGain
type GMPPriceHistoryScraper struct {
	db                 *sql.DB
	logger             *logrus.Logger
	httpClient         *shared.HTTPClientFactory
	requestRateLimiter *shared.HTTPRequestRateLimiter
	circuitBreaker     *shared.CircuitBreaker
	retryConfig        shared.RetryConfig
	apiClient          *InvestorGainAPIClient
}

// NewGMPPriceHistoryScraper creates a new GMP price history scraper
func NewGMPPriceHistoryScraper(db *sql.DB) *GMPPriceHistoryScraper {
	config := shared.NewGMPServiceConfig()
	httpClientFactory := shared.NewHTTPClientFactory(config.HTTPRequestTimeout)

	// Initialize circuit breaker for external service protection (Requirement 6.4)
	circuitBreakerConfig := shared.DefaultCircuitBreakerConfig()
	circuitBreaker := shared.NewCircuitBreaker("investorgain-scraper", circuitBreakerConfig)

	// Initialize retry configuration (Requirement 6.1)
	retryConfig := shared.DefaultRetryConfig()

	return &GMPPriceHistoryScraper{
		db: db,
		logger: func() *logrus.Logger {
			logger := logrus.New()
			logger.SetLevel(logrus.DebugLevel) // Enable debug logging
			return logger
		}(),
		httpClient:         httpClientFactory,
		requestRateLimiter: shared.NewHTTPRequestRateLimiter(config.RequestRateLimit),
		circuitBreaker:     circuitBreaker,
		retryConfig:        retryConfig,
		apiClient:          NewInvestorGainAPIClient(),
	}
}

func (s *GMPPriceHistoryScraper) getLogger() *logrus.Logger {
	if s.logger != nil {
		return s.logger
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	s.logger = logger
	return s.logger
}

// ScrapedHistoryData represents the complete scraped data from InvestorGain
type ScrapedHistoryData struct {
	IPOName           string                        `json:"ipo_name"`
	CompanyCode       string                        `json:"company_code"`
	IPOPrice          float64                       `json:"ipo_price"`
	CurrentGMP        float64                       `json:"current_gmp"`
	CurrentGMPPercent float64                       `json:"current_gmp_percent"`
	PriceHistory      []models.GMPPriceHistoryEntry `json:"price_history"`
	LastUpdated       time.Time                     `json:"last_updated"`
	ScrapingSuccess   bool                          `json:"scraping_success"`
	ErrorCount        int                           `json:"error_count"`
	ProcessingTime    time.Duration                 `json:"processing_time"`
}

// ScrapeHistoryFromURL scrapes GMP price history from a specific InvestorGain URL
// Implements Requirements 1.1, 1.2, 1.3 - Extract historical price data with error handling
// Implements Requirements 1.5, 6.1, 6.4 - Rate limiting, retry logic, and circuit breaker
func (s *GMPPriceHistoryScraper) ScrapeHistoryFromURL(url string) (*ScrapedHistoryData, error) {
	startTime := time.Now()
	s.logger.WithField("url", url).Info("Starting GMP history scraping")

	var scrapedData *ScrapedHistoryData
	var nonRetryableErr error

	// Execute with circuit breaker protection (Requirement 6.4)
	circuitErr := s.circuitBreaker.Execute(func() error {
		// Execute with retry logic (Requirement 6.1)
		retryErr := shared.RetryWithExponentialBackoff(func() error {
			// Enforce rate limiting (Requirement 1.5)
			s.requestRateLimiter.EnforceRateLimit()

			// Perform the actual scraping
			data, err := s.performScraping(url)
			if err != nil {
				if !shared.IsRetryableError(err) {
					nonRetryableErr = err
					s.logger.WithError(err).Error("Non-retryable error occurred during scraping")
					return nil
				}
				// Check if error is retryable
				s.logger.WithError(err).Warn("Retryable error occurred during scraping")
				return err
			}

			scrapedData = data
			return nil
		}, s.retryConfig, s.logger)

		return retryErr
	})

	if circuitErr != nil {
		if circuitErr == shared.ErrCircuitOpen {
			s.logger.WithFields(logrus.Fields{
				"url":             url,
				"circuit_breaker": "investorgain-scraper",
			}).Error("Circuit breaker is open, external service unavailable")
			return nil, fmt.Errorf("external service unavailable (circuit breaker open): %w", circuitErr)
		}

		s.logger.WithFields(logrus.Fields{
			"url": url,
		}).Error("Failed to execute scraping: ", circuitErr)
		return nil, fmt.Errorf("failed to scrape: %w", circuitErr)
	}

	if nonRetryableErr != nil {
		return nil, fmt.Errorf("scraping failed: %w", nonRetryableErr)
	}

	// Calculate total processing time
	scrapedData.ProcessingTime = time.Since(startTime)

	s.logger.WithFields(logrus.Fields{
		"ipo_name":        scrapedData.IPOName,
		"entries_found":   len(scrapedData.PriceHistory),
		"errors":          scrapedData.ErrorCount,
		"processing_time": scrapedData.ProcessingTime,
	}).Info("GMP history scraping completed successfully")

	return scrapedData, nil
}

// ScrapeHistoryFromAPI scrapes GMP price history using the InvestorGain API
// This is more reliable than HTML scraping as it directly accesses the JSON data
func (s *GMPPriceHistoryScraper) ScrapeHistoryFromAPI(ipoID string, companyCode string) (*ScrapedHistoryData, error) {
	startTime := time.Now()
	s.logger.WithFields(logrus.Fields{
		"ipo_id":       ipoID,
		"company_code": companyCode,
	}).Info("Starting GMP history scraping via API")

	var scrapedData *ScrapedHistoryData
	var nonRetryableErr error

	// Execute with circuit breaker protection
	circuitErr := s.circuitBreaker.Execute(func() error {
		// Execute with retry logic
		retryErr := shared.RetryWithExponentialBackoff(func() error {
			// Enforce rate limiting
			s.requestRateLimiter.EnforceRateLimit()

			// Perform the actual API scraping
			data, err := s.performAPIScraping(ipoID, companyCode)
			if err != nil {
				if !shared.IsRetryableError(err) {
					nonRetryableErr = err
					if errors.Is(err, ErrNoGMPDataAvailable) {
						s.logger.WithError(err).Info("No GMP data available from API for IPO")
					} else {
						s.logger.WithError(err).Error("Non-retryable error occurred during API scraping")
					}
					return nil
				}

				s.logger.WithError(err).Warn("Retryable error occurred during API scraping")
				return err
			}

			scrapedData = data
			return nil
		}, s.retryConfig, s.logger)

		return retryErr
	})

	if circuitErr != nil {
		if circuitErr == shared.ErrCircuitOpen {
			s.logger.WithFields(logrus.Fields{
				"ipo_id":          ipoID,
				"circuit_breaker": "investorgain-scraper",
			}).Error("Circuit breaker is open, external service unavailable")
			return nil, fmt.Errorf("external service unavailable (circuit breaker open): %w", circuitErr)
		}

		s.logger.WithFields(logrus.Fields{
			"ipo_id":         ipoID,
			"retry_attempts": s.retryConfig.MaxAttempts,
		}).Error("API scraping failed")
		return nil, fmt.Errorf("API scraping failed: %w", circuitErr)
	}

	if nonRetryableErr != nil {
		return nil, fmt.Errorf("API scraping failed: %w", nonRetryableErr)
	}

	// Calculate total processing time
	scrapedData.ProcessingTime = time.Since(startTime)

	s.logger.WithFields(logrus.Fields{
		"ipo_name":        scrapedData.IPOName,
		"entries_found":   len(scrapedData.PriceHistory),
		"errors":          scrapedData.ErrorCount,
		"processing_time": scrapedData.ProcessingTime,
	}).Info("GMP history API scraping completed successfully")

	return scrapedData, nil
}

// performAPIScraping performs the actual API scraping operation
func (s *GMPPriceHistoryScraper) performAPIScraping(ipoID, companyCode string) (*ScrapedHistoryData, error) {
	if s.apiClient == nil {
		return nil, fmt.Errorf("API client not initialized")
	}

	// Fetch GMP payload from API
	payload, err := s.apiClient.GetIPOGMPPayload(ipoID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GMP data from API: %w", err)
	}

	// Transform API data to ScrapedHistoryData
	scrapedData := &ScrapedHistoryData{
		CompanyCode:     companyCode,
		IPOName:         companyCode,
		LastUpdated:     time.Now(),
		ScrapingSuccess: true,
		ErrorCount:      0,
		PriceHistory:    make([]models.GMPPriceHistoryEntry, 0, len(payload.DataPoints)),
	}

	// Parse day-wise table first when available (usually has full history)
	if strings.TrimSpace(payload.TableHTML) != "" {
		tableEntries, tableErrors := s.ExtractHistoryTable(payload.TableHTML)
		scrapedData.ErrorCount += tableErrors
		for _, entry := range tableEntries {
			entry.CompanyCode = companyCode
			entry.IPOID = ipoID
			scrapedData.PriceHistory = append(scrapedData.PriceHistory, entry)
		}
	}

	// Fallback to direct API data points if table parsing produced no entries
	if len(scrapedData.PriceHistory) == 0 {
		for _, dp := range payload.DataPoints {
			entry, err := s.parseAPIDataPoint(dp)
			if err != nil {
				s.logger.WithError(err).Warn("Failed to parse API data point, skipping")
				scrapedData.ErrorCount++
				continue
			}
			entry.CompanyCode = companyCode
			entry.IPOID = ipoID
			scrapedData.PriceHistory = append(scrapedData.PriceHistory, entry)
		}
	}

	if len(scrapedData.PriceHistory) == 0 {
		return nil, fmt.Errorf("no valid price history entries found")
	}

	// Ensure entries are ordered newest-first for downstream consumers
	sort.Slice(scrapedData.PriceHistory, func(i, j int) bool {
		return scrapedData.PriceHistory[i].RecordDate.After(scrapedData.PriceHistory[j].RecordDate)
	})

	// Set current GMP from latest entry
	if len(scrapedData.PriceHistory) > 0 {
		scrapedData.CurrentGMP = scrapedData.PriceHistory[0].GMPValue
		scrapedData.IPOPrice = scrapedData.PriceHistory[0].IPOPrice
	}

	return scrapedData, nil
}

func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

// parseAPIDataPoint converts an API data point to a GMPPriceHistoryEntry
func (s *GMPPriceHistoryScraper) parseAPIDataPoint(dp IPOGmpDataPoint) (models.GMPPriceHistoryEntry, error) {
	entry := models.GMPPriceHistoryEntry{
		ID:         uuid.New().String(),
		DataSource: "investorgain.com",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Parse date
	dateValue := firstNonEmptyValue(dp.Date.String(), dp.LegacyDate.String())
	if dateValue == "" {
		return entry, fmt.Errorf("missing date value in API response")
	}

	parsedDate, err := s.parseDateWithStatus(dateValue)
	if err != nil {
		parsedDate, err = s.parseDate(dateValue)
		if err != nil {
			return entry, fmt.Errorf("failed to parse date '%s': %w", dateValue, err)
		}
	}
	entry.RecordDate = parsedDate

	// Set values from API
	entry.GMPValue = s.parseGMPValue(firstNonEmptyValue(dp.GMP.String(), dp.LegacyGMP.String()))
	entry.IPOPrice = s.parseFloat(firstNonEmptyValue(dp.IPOPrice.String(), dp.LegacyIPOPrice.String()))
	entry.EstimatedListing = s.parseEstimatedListingPrice(firstNonEmptyValue(dp.EstimatedListing.String(), dp.LegacyEstimated.String()))
	entry.ListingPercent = s.parseEstimatedListingPercent(firstNonEmptyValue(dp.EstimatedPercent.String(), dp.LegacyEstimatedPct.String()))
	entry.Sub2Sauda = s.parseFloat(firstNonEmptyValue(dp.Sub2.String(), dp.LegacySub2.String()))
	entry.EstimatedProfit = s.parseFloat(firstNonEmptyValue(dp.EstimatedProfit.String(), dp.LegacyEstimatedProfit.String()))
	entry.LastUpdated = firstNonEmptyValue(dp.LastUpdated.String(), dp.LegacyLastUpdated.String())
	entry.SubscriptionStatus = "Not Available"

	if entry.LastUpdated == "" {
		entry.LastUpdated = time.Now().Format("02-01-2006 15:04")
	}

	if entry.EstimatedListing == 0 && entry.IPOPrice > 0 {
		entry.EstimatedListing = entry.IPOPrice + entry.GMPValue
	}

	if entry.IPOPrice == 0 && entry.EstimatedListing > 0 {
		entry.IPOPrice = entry.EstimatedListing - entry.GMPValue
	}

	if entry.ListingPercent == 0 && entry.IPOPrice > 0 {
		entry.ListingPercent = (entry.GMPValue / entry.IPOPrice) * 100
	}

	// Calculate estimated profit (assuming sharesPerLot shares)
	if entry.EstimatedProfit == 0 && entry.GMPValue != 0 {
		entry.EstimatedProfit = entry.GMPValue * sharesPerLot
	}

	if entry.IPOPrice <= 0 && entry.EstimatedListing <= 0 {
		return entry, fmt.Errorf("missing price fields in API response")
	}

	return entry, nil
}

// performScraping performs the actual web scraping operation
// Separated from ScrapeHistoryFromURL to enable retry logic
func (s *GMPPriceHistoryScraper) performScraping(url string) (*ScrapedHistoryData, error) {
	// Setup Chrome with optimized options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-images", true),
		chromedp.Flag("disable-javascript", false), // Need JS for dynamic content
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-setuid-sandbox", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set timeout for scraping operation
	ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	var htmlContent string

	// Navigate and extract HTML content
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(chromeDPWaitTime),
		chromedp.OuterHTML(`html`, &htmlContent, chromedp.ByQuery),
	)

	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"url":     url,
			"timeout": "45s",
		}).WithError(err).Error("Failed to navigate and extract HTML")
		return nil, fmt.Errorf("chromedp navigation failed: %w", err)
	}

	// Extract data from HTML
	scrapedData := &ScrapedHistoryData{
		LastUpdated:     time.Now(),
		ScrapingSuccess: true,
		ErrorCount:      0,
	}

	// Extract IPO name
	scrapedData.IPOName = s.extractIPOName(htmlContent)

	// Extract IPO price
	scrapedData.IPOPrice = s.extractIPOPrice(htmlContent)

	// Extract current GMP
	scrapedData.CurrentGMP = s.extractCurrentGMP(htmlContent)

	// Extract price history from table (Requirement 1.1, 1.2)
	priceHistory, errorCount := s.ExtractHistoryTable(htmlContent)
	scrapedData.PriceHistory = priceHistory
	scrapedData.ErrorCount = errorCount

	return scrapedData, nil
}

// ExtractHistoryTable extracts price history entries from the HTML table
// Implements Requirements 1.1, 1.2, 1.3 - Extract all table rows with error isolation
// Enhanced to handle varying table structures (7, 8, or more columns)
func (s *GMPPriceHistoryScraper) ExtractHistoryTable(htmlContent string) ([]models.GMPPriceHistoryEntry, int) {
	var history []models.GMPPriceHistoryEntry
	errorCount := 0

	// Find all tables with GMP history using robust CSS selectors
	tablePattern := `<table[^>]*>(.*?)</table>`
	tableRe := regexp.MustCompile(`(?is)` + tablePattern)
	allTableMatches := tableRe.FindAllStringSubmatch(htmlContent, -1)

	if len(allTableMatches) == 0 {
		s.logger.Warn("No table elements found in HTML")
		return history, 0
	}

	s.logger.WithField("tables_found", len(allTableMatches)).Debug("Found tables, selecting the best one for price history")

	// Find the table with the most cells per row (likely the price history table)
	var bestTable string
	maxCellsPerRow := 0
	bestTableIndex := -1

	for i, tableMatch := range allTableMatches {
		if len(tableMatch) < 2 {
			continue
		}

		tableContent := tableMatch[1]

		// Extract tbody content (fallback to full table if tbody is absent)
		tbodyPattern := `<tbody[^>]*>(.*?)</tbody>`
		tbodyRe := regexp.MustCompile(`(?is)` + tbodyPattern)
		tbodyMatches := tbodyRe.FindStringSubmatch(tableContent)
		tbodyContent := tableContent
		if len(tbodyMatches) >= 2 {
			tbodyContent = tbodyMatches[1]
		}

		// Extract first row to count cells
		rowPattern := `<tr[^>]*>(.*?)</tr>`
		rowRe := regexp.MustCompile(`(?is)` + rowPattern)
		rows := rowRe.FindAllStringSubmatch(tbodyContent, -1)

		if len(rows) == 0 {
			continue
		}

		// Count cells in first row
		cellPattern := `<td[^>]*>(.*?)</td>`
		cellRe := regexp.MustCompile(`(?is)` + cellPattern)
		cells := cellRe.FindAllStringSubmatch(rows[0][1], -1)
		cellCount := len(cells)

		s.logger.WithFields(logrus.Fields{
			"table_index": i,
			"rows_count":  len(rows),
			"cells_count": cellCount,
		}).Debug("Analyzing table structure")

		// Price history tables typically have 7-8 cells per row
		if cellCount >= 7 && cellCount > maxCellsPerRow {
			maxCellsPerRow = cellCount
			bestTable = tbodyContent
			bestTableIndex = i
		}
	}

	if bestTable == "" {
		s.logger.Warn("No suitable price history table found (need 7+ cells per row)")
		return history, 0
	}

	s.logger.WithFields(logrus.Fields{
		"selected_table": bestTableIndex,
		"cells_per_row":  maxCellsPerRow,
	}).Info("Selected best table for price history extraction")

	// Extract rows from the best table
	rowPattern := `<tr[^>]*>(.*?)</tr>`
	rowRe := regexp.MustCompile(`(?is)` + rowPattern)
	rows := rowRe.FindAllStringSubmatch(bestTable, -1)

	s.logger.WithField("rows_found", len(rows)).Info("Extracting price history rows")

	for rowIdx, row := range rows {
		if len(row) < 2 {
			continue
		}

		// Extract cells from row
		cellPattern := `<td[^>]*>(.*?)</td>`
		cellRe := regexp.MustCompile(`(?is)` + cellPattern)
		cells := cellRe.FindAllStringSubmatch(row[1], -1)

		// Enhanced: Handle different table structures
		if len(cells) < 3 {
			// Skip rows with too few cells (likely header or empty rows).
			// Count non-empty malformed rows as parsing errors for observability.
			s.logger.WithFields(logrus.Fields{
				"row_index":   rowIdx,
				"cells_found": len(cells),
			}).Debug("Skipping row with insufficient cells")
			rowText := strings.TrimSpace(s.cleanText(row[1]))
			if rowText != "" {
				errorCount++
			}
			continue
		}

		// Parse entry with flexible cell count handling
		entry, err := s.parseHistoryEntryFlexible(cells, rowIdx)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"row_index":   rowIdx,
				"cells_found": len(cells),
			}).Warn("Failed to parse history entry, skipping: ", err.Error())
			errorCount++
			continue
		}

		history = append(history, entry)
	}

	return history, errorCount
}

// parseHistoryEntryFlexible parses a single history entry with flexible column handling
// Handles tables with varying column counts (7, 8, or more columns)
func (s *GMPPriceHistoryScraper) parseHistoryEntryFlexible(cells [][]string, rowIndex int) (models.GMPPriceHistoryEntry, error) {
	entry := models.GMPPriceHistoryEntry{
		ID:         uuid.New().String(),
		DataSource: "investorgain.com",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	cellCount := len(cells)
	s.logger.WithFields(logrus.Fields{
		"row_index":   rowIndex,
		"cells_found": cellCount,
	}).Debug("Parsing row with flexible column handling")

	// Handle different table structures based on column count
	switch {
	case cellCount >= 8:
		// Full 8-column table (enhanced case)
		return s.parseEightColumnEntry(cells)
	case cellCount == 7:
		// 7-column table (missing one field, usually last updated)
		return s.parseSevenColumnEntry(cells)
	case cellCount >= 3:
		// Minimal table (3+ columns: date, gmp, price info)
		return s.parseMinimalEntry(cells)
	default:
		return entry, fmt.Errorf("insufficient data: only %d columns found", cellCount)
	}
}

// parseEightColumnEntry parses the enhanced 8-column table
// Based on the Armour Security structure: Date, IPO Price, GMP, Subscription, Sub2 Sauda, Estimated Listing, Estimated Profit, Last Updated
func (s *GMPPriceHistoryScraper) parseEightColumnEntry(cells [][]string) (models.GMPPriceHistoryEntry, error) {
	entry := models.GMPPriceHistoryEntry{
		ID:         uuid.New().String(),
		DataSource: "investorgain.com",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Parse date (cell 0) - enhanced to handle status text
	dateStr := s.cleanText(cells[0][1])
	parsedDate, err := s.parseDateWithStatus(dateStr)
	if err != nil {
		return entry, fmt.Errorf("failed to parse date '%s': %w", dateStr, err)
	}
	entry.RecordDate = parsedDate

	// Parse IPO price (cell 1)
	entry.IPOPrice = s.parseFloat(cells[1][1])

	// Parse GMP value (cell 2)
	entry.GMPValue = s.parseGMPValue(cells[2][1])

	// Parse subscription status (cell 3)
	entry.SubscriptionStatus = s.normalizeSubscriptionStatus(s.cleanText(cells[3][1]))

	// Parse Sub2 Sauda (cell 4)
	entry.Sub2Sauda = s.parseFloat(cells[4][1])

	// Parse estimated listing price and percent (cell 5)
	entry.EstimatedListing = s.parseEstimatedListingPrice(cells[5][1])
	entry.ListingPercent = s.parseEstimatedListingPercent(cells[5][1])

	// Parse estimated profit (cell 6)
	entry.EstimatedProfit = s.parseFloat(cells[6][1])

	// Parse last updated (cell 7)
	entry.LastUpdated = s.cleanText(cells[7][1])

	return entry, nil
}

// parseSevenColumnEntry parses 7-column table (KRM Ayurveda structure)
// Structure: Date, IPO Price, GMP, Subscription, Estimated Listing, Estimated Profit, Last Updated
func (s *GMPPriceHistoryScraper) parseSevenColumnEntry(cells [][]string) (models.GMPPriceHistoryEntry, error) {
	entry := models.GMPPriceHistoryEntry{
		ID:         uuid.New().String(),
		DataSource: "investorgain.com",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Parse date (cell 0)
	dateStr := s.cleanText(cells[0][1])
	parsedDate, err := s.parseDateWithStatus(dateStr)
	if err != nil {
		return entry, fmt.Errorf("failed to parse date '%s': %w", dateStr, err)
	}
	entry.RecordDate = parsedDate

	// Parse IPO price (cell 1)
	entry.IPOPrice = s.parseFloat(cells[1][1])

	// Parse GMP value (cell 2)
	entry.GMPValue = s.parseGMPValue(cells[2][1])

	// Parse subscription status (cell 3) - fix the subscription format
	subscriptionText := s.cleanText(cells[3][1])
	if subscriptionText != "" && subscriptionText != "0" {
		entry.SubscriptionStatus = subscriptionText + "x subscribed"
	} else {
		entry.SubscriptionStatus = "Not Available"
	}

	// Parse estimated listing price and percent (cell 4) - CORRECTED INDEX
	entry.EstimatedListing = s.parseEstimatedListingPrice(cells[4][1])
	entry.ListingPercent = s.parseEstimatedListingPercent(cells[4][1])

	// Parse estimated profit (cell 5) - CORRECTED INDEX
	entry.EstimatedProfit = s.parseFloat(cells[5][1])

	// Parse last updated (cell 6) - CORRECTED: Parse actual timestamp
	entry.LastUpdated = s.cleanText(cells[6][1])

	// Set Sub2 Sauda to 0 for 7-column tables (not present)
	entry.Sub2Sauda = 0

	return entry, nil
}

// parseMinimalEntry parses minimal table with basic data (3+ columns)
func (s *GMPPriceHistoryScraper) parseMinimalEntry(cells [][]string) (models.GMPPriceHistoryEntry, error) {
	entry := models.GMPPriceHistoryEntry{
		ID:         uuid.New().String(),
		DataSource: "investorgain.com",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Parse date (cell 0)
	dateStr := s.cleanText(cells[0][1])
	parsedDate, err := s.parseDateWithStatus(dateStr)
	if err != nil {
		return entry, fmt.Errorf("failed to parse date '%s': %w", dateStr, err)
	}
	entry.RecordDate = parsedDate

	// Parse GMP value (cell 1 or 2, depending on structure)
	gmpCellIndex := 1
	if len(cells) > 2 {
		// Try to find which cell contains GMP data
		for i := 1; i < len(cells) && i < 3; i++ {
			cellText := s.cleanText(cells[i][1])
			if strings.Contains(cellText, "₹") || regexp.MustCompile(`\d+`).MatchString(cellText) {
				gmpCellIndex = i
				break
			}
		}
	}

	entry.GMPValue = s.parseGMPValue(cells[gmpCellIndex][1])

	// Try to extract IPO price from GMP cell or estimate it
	if entry.GMPValue > 0 {
		// Look for IPO price in the same cell or estimate from percentage
		gmpText := s.cleanText(cells[gmpCellIndex][1])
		if strings.Contains(gmpText, "(") && strings.Contains(gmpText, "%)") {
			// Extract percentage and calculate IPO price
			percentMatch := regexp.MustCompile(`\((\d+(?:\.\d+)?)%\)`).FindStringSubmatch(gmpText)
			if len(percentMatch) > 1 {
				percentage, _ := strconv.ParseFloat(percentMatch[1], 64)
				if percentage > 0 {
					entry.IPOPrice = entry.GMPValue / (percentage / 100)
					entry.ListingPercent = percentage
				}
			}
		}
	}

	// If we still don't have IPO price, try to find it in other cells
	if entry.IPOPrice == 0 {
		for i := 1; i < len(cells); i++ {
			cellText := s.cleanText(cells[i][1])
			// Look for patterns like "IPO Price: ₹100" or just "₹100"
			if price := s.parseFloat(cellText); price > 0 && price < 10000 { // Reasonable IPO price range
				entry.IPOPrice = price
				break
			}
		}
	}

	// Calculate estimated listing if we have both values
	if entry.IPOPrice > 0 && entry.GMPValue >= 0 {
		entry.EstimatedListing = entry.IPOPrice + entry.GMPValue
		if entry.IPOPrice > 0 {
			entry.ListingPercent = (entry.GMPValue / entry.IPOPrice) * 100
		}
	}

	// Set defaults for missing fields
	entry.SubscriptionStatus = "Not Available"
	entry.Sub2Sauda = 0
	entry.EstimatedProfit = entry.GMPValue * 1000 // Estimate based on 1000 shares
	entry.LastUpdated = "Not Available"

	return entry, nil
}

// parseDateWithStatus parses dates that may include status text
// Examples: "19-01-2026 Allotment", "16-01-2026 Close", "13-01-2026 Open"
func (s *GMPPriceHistoryScraper) parseDateWithStatus(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("empty date string")
	}

	// Remove status keywords
	statusKeywords := []string{
		" Allotment", " Close", " Open", " Listed", " Result",
		" Announcement", " Launch", " End", " Start", " Listing",
		" allotment", " close", " open", " listed", " result",
		" announcement", " launch", " end", " start", " listing",
	}

	cleanDate := dateStr
	for _, keyword := range statusKeywords {
		cleanDate = strings.ReplaceAll(cleanDate, keyword, "")
	}
	cleanDate = strings.TrimSpace(cleanDate)

	// Try multiple date formats
	formats := []string{
		"02-Jan-2006",
		"2-Jan-2006",
		"02-01-2006",
		"2-1-2006",
		"2006-01-02",
		"02/01/2006",
		"2/1/2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, cleanDate); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s (cleaned: %s)", dateStr, cleanDate)
}

// BuildIPOHistoryURL constructs the InvestorGain URL for an IPO
// It first attempts to find the numeric ID from the InvestorGain listing page
func (s *GMPPriceHistoryScraper) BuildIPOHistoryURL(companyCode string, ipoName string) (string, error) {
	if companyCode == "" {
		return "", fmt.Errorf("company code is required")
	}

	logger := s.getLogger()

	if matched, _ := regexp.MatchString(`^\d+$`, strings.TrimSpace(ipoName)); matched {
		numericID := strings.TrimSpace(ipoName)
		url := fmt.Sprintf("https://www.investorgain.com/gmp/%s-ipo/%s/", strings.ToLower(companyCode), numericID)
		logger.WithFields(logrus.Fields{
			"company_code": companyCode,
			"numeric_id":   numericID,
			"url":          url,
			"source":       "provided_numeric_id",
		}).Info("Built InvestorGain URL")
		return url, nil
	}

	// Try to find the numeric ID from the listing page
	numericID, err := s.FindInvestorGainNumericID(companyCode, ipoName)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"company_code": companyCode,
			"ipo_name":     ipoName,
			"error":        err.Error(),
		}).Warn("Failed to find InvestorGain numeric ID, cannot build URL")
		return "", fmt.Errorf("failed to find InvestorGain numeric ID: %w", err)
	}

	// InvestorGain URL pattern: https://www.investorgain.com/gmp/{company-code}-ipo/{numeric_id}/
	baseURL := "https://www.investorgain.com/gmp/"
	url := fmt.Sprintf("%s%s-ipo/%s/", baseURL, strings.ToLower(companyCode), numericID)

	logger.WithFields(logrus.Fields{
		"company_code": companyCode,
		"numeric_id":   numericID,
		"url":          url,
	}).Info("Built InvestorGain URL")

	return url, nil
}

// FindInvestorGainNumericID finds the numeric ID for an IPO from the InvestorGain listing page
// Enhanced with multiple matching strategies and fuzzy matching
func (s *GMPPriceHistoryScraper) FindInvestorGainNumericID(companyCode string, ipoName string) (string, error) {
	logger := s.getLogger()

	logger.WithFields(logrus.Fields{
		"company_code": companyCode,
		"ipo_name":     ipoName,
	}).Info("Searching for InvestorGain numeric ID")

	if numericID, err := s.findNumericIDFromAPIURLList(companyCode, ipoName); err == nil {
		logger.WithFields(logrus.Fields{
			"company_code": companyCode,
			"ipo_name":     ipoName,
			"numeric_id":   numericID,
			"source":       "api_url_list",
		}).Info("Found InvestorGain numeric ID")
		return numericID, nil
	} else {
		logger.WithFields(logrus.Fields{
			"company_code": companyCode,
			"ipo_name":     ipoName,
			"error":        err.Error(),
		}).Debug("API URL list lookup failed, falling back to listing page scraping")
	}

	// Setup Chrome
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-images", true),
		chromedp.Flag("disable-javascript", false),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-setuid-sandbox", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var htmlContent string

	// Navigate to the listing page and extract HTML
	err := chromedp.Run(ctx,
		chromedp.Navigate("https://www.investorgain.com/report/live-ipo-gmp/331/all/"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(5*time.Second),
		chromedp.OuterHTML(`html`, &htmlContent, chromedp.ByQuery),
	)

	if err != nil {
		return "", fmt.Errorf("failed to load listing page: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"html_length":  len(htmlContent),
		"contains_gmp": strings.Contains(htmlContent, "/gmp/"),
		"contains_ipo": strings.Contains(htmlContent, "-ipo/"),
	}).Debug("Loaded InvestorGain listing page")

	// Extract numeric ID using multiple matching strategies
	return s.findNumericIDWithMultipleStrategies(htmlContent, companyCode, ipoName)
}

func (s *GMPPriceHistoryScraper) findNumericIDFromAPIURLList(companyCode, ipoName string) (string, error) {
	if s.apiClient == nil {
		return "", fmt.Errorf("api client not initialized")
	}

	entries, err := s.apiClient.GetIPOUrlList()
	if err != nil {
		return "", fmt.Errorf("failed to fetch IPO URL list: %w", err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("IPO URL list is empty")
	}

	type candidate struct {
		code      string
		name      string
		numericID string
	}

	candidates := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		numericID := strings.TrimSpace(entry.NumericID)
		if numericID == "" {
			_, parsedNumericID := parseInvestorGainGMPURL(entry.URL)
			numericID = parsedNumericID
		}

		code := normalizeCompanyCode(entry.CompanyCode)
		if code == "" {
			code = normalizeCompanyCode(entry.URLCode)
		}
		if code == "" {
			parsedCode, _ := parseInvestorGainGMPURL(entry.URL)
			code = parsedCode
		}

		if code == "" || numericID == "" {
			continue
		}

		name := strings.TrimSpace(entry.CompanyName)
		if name == "" {
			name = code
		}

		candidates = append(candidates, candidate{
			code:      code,
			name:      name,
			numericID: numericID,
		})
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("IPO URL list has no parseable entries")
	}

	normalizedCode := normalizeCompanyCode(companyCode)
	normalizedName := s.normalizeIPOName(ipoName)
	nameWords := strings.Fields(normalizedName)

exactMatch:
	for _, item := range candidates {
		if item.code == normalizedCode {
			exactMatches := 0
			exactMatchID := item.numericID
			for _, c := range candidates {
				if c.code == normalizedCode {
					exactMatches++
					exactMatchID = c.numericID
				}
			}
			if exactMatches == 1 {
				itemNameScore := s.calculateNameSimilarity(normalizedName, s.normalizeIPOName(item.name), nameWords)
				if itemNameScore > 0.7 {
					return exactMatchID, nil
				}
				s.logger.WithFields(logrus.Fields{
					"company_code": companyCode,
					"ipo_name":     ipoName,
					"matched_name": item.name,
					"name_score":   itemNameScore,
					"numeric_id":   exactMatchID,
				}).Warn("Exact code match found but name doesn't match well - verifying identity")
				if itemNameScore > 0.5 {
					return exactMatchID, nil
				}
				break exactMatch
			}
			break exactMatch
		}
	}

	variations := s.generateCompanyCodeVariations(normalizedCode)
	matchedIDs := make(map[string]int)
	for _, variation := range variations {
		normalizedVariation := normalizeCompanyCode(variation)
		// Only use exact equality — never substring matching.
		// strings.Contains() caused short codes like "krl" to match inside
		// longer unrelated codes (e.g., "sparkrl"), returning wrong numeric IDs.
		for _, item := range candidates {
			if item.code == normalizedVariation {
				matchedIDs[item.numericID]++
			}
		}
	}

	if len(matchedIDs) == 1 {
		for id := range matchedIDs {
			return id, nil
		}
	} else if len(matchedIDs) > 1 {
		s.logger.WithFields(logrus.Fields{
			"company_code": companyCode,
			"matched_ids":  matchedIDs,
		}).Warn("Multiple fuzzy matches found - rejecting ambiguous match")
	}

	bestScore := 0.0
	bestNumericID := ""
	matchCount := 0
	for _, item := range candidates {
		score := s.calculateNameSimilarity(normalizedName, s.normalizeIPOName(item.name), nameWords)
		if score > bestScore && score > 0.6 {
			bestScore = score
			bestNumericID = item.numericID
			matchCount = 1
		} else if score == bestScore && score > 0.6 {
			matchCount++
		}
	}

	if matchCount == 1 && bestNumericID != "" {
		return bestNumericID, nil
	} else if matchCount > 1 {
		s.logger.WithFields(logrus.Fields{
			"company_code": companyCode,
			"ipo_name":     ipoName,
			"match_count":  matchCount,
			"best_score":   bestScore,
		}).Warn("Multiple name similarity matches - rejecting ambiguous match")
	}

	return "", NewGMPHistoryError(FailureTypeNOIGID, fmt.Sprintf("IPO %s (%s): not found in InvestorGain API URL list", ipoName, companyCode), nil)
}

// findNumericIDWithMultipleStrategies tries multiple matching approaches
func (s *GMPPriceHistoryScraper) findNumericIDWithMultipleStrategies(htmlContent, companyCode, ipoName string) (string, error) {
	// Strategy 1: Exact company code match
	if numericID, found := s.findByExactCompanyCode(htmlContent, companyCode); found {
		s.logger.WithFields(logrus.Fields{
			"company_code": companyCode,
			"numeric_id":   numericID,
			"match_method": "exact_company_code",
		}).Info("Found InvestorGain numeric ID")
		return numericID, nil
	}

	// Strategy 2: Fuzzy company code match (handle variations)
	if numericID, found := s.findByFuzzyCompanyCode(htmlContent, companyCode); found {
		s.logger.WithFields(logrus.Fields{
			"company_code": companyCode,
			"numeric_id":   numericID,
			"match_method": "fuzzy_company_code",
		}).Info("Found InvestorGain numeric ID")
		return numericID, nil
	}

	// Strategy 3: Company name similarity match
	if numericID, found := s.findByCompanyName(htmlContent, ipoName); found {
		s.logger.WithFields(logrus.Fields{
			"ipo_name":     ipoName,
			"numeric_id":   numericID,
			"match_method": "company_name_similarity",
		}).Info("Found InvestorGain numeric ID")
		return numericID, nil
	}

	// Strategy 4: Partial name match (keywords)
	if numericID, found := s.findByPartialName(htmlContent, ipoName); found {
		s.logger.WithFields(logrus.Fields{
			"ipo_name":     ipoName,
			"numeric_id":   numericID,
			"match_method": "partial_name_match",
		}).Info("Found InvestorGain numeric ID")
		return numericID, nil
	}

	return "", NewGMPHistoryError(FailureTypeNOIGID, fmt.Sprintf("IPO %s (%s): not found on InvestorGain listing page", ipoName, companyCode), nil)
}

// findByExactCompanyCode finds IPO by exact company code match
func (s *GMPPriceHistoryScraper) findByExactCompanyCode(htmlContent, companyCode string) (string, bool) {
	normalizedCode := strings.ToLower(companyCode)

	linkPattern := `<a[^>]*href="/gmp/([^/]+)-ipo/(\d+)/"[^>]*>([^<]+)</a>`
	linkRe := regexp.MustCompile(linkPattern)
	matches := linkRe.FindAllStringSubmatch(htmlContent, -1)

	s.logger.WithFields(logrus.Fields{
		"company_code":  companyCode,
		"normalized":    normalizedCode,
		"total_matches": len(matches),
	}).Debug("Searching for exact company code match")

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		urlCode := match[1]
		numericID := match[2]

		if urlCode == normalizedCode {
			s.logger.WithFields(logrus.Fields{
				"company_code": companyCode,
				"url_code":     urlCode,
				"numeric_id":   numericID,
			}).Info("Found exact company code match")
			return numericID, true
		}
	}

	s.logger.WithFields(logrus.Fields{
		"company_code": companyCode,
		"normalized":   normalizedCode,
	}).Warn("No exact company code match found")
	return "", false
}

// findByFuzzyCompanyCode finds IPO by fuzzy company code matching
func (s *GMPPriceHistoryScraper) findByFuzzyCompanyCode(htmlContent, companyCode string) (string, bool) {
	normalizedCode := strings.ToLower(companyCode)

	// Generate variations of the company code
	variations := s.generateCompanyCodeVariations(normalizedCode)

	linkPattern := `<a[^>]*href="/gmp/([^/]+)-ipo/(\d+)/"[^>]*>([^<]+)</a>`
	linkRe := regexp.MustCompile(linkPattern)
	matches := linkRe.FindAllStringSubmatch(htmlContent, -1)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		urlCode := match[1]
		numericID := match[2]

		// Check if any variation matches — exact equality only.
		// Never use strings.Contains() here: short codes like "krl"
		// would substring-match inside unrelated URL slugs.
		for _, variation := range variations {
			if urlCode == variation {
				s.logger.WithFields(logrus.Fields{
					"original_code": companyCode,
					"matched_code":  urlCode,
					"variation":     variation,
				}).Debug("Fuzzy company code match found")
				return numericID, true
			}
		}
	}
	return "", false
}

// findByCompanyName finds IPO by company name similarity
func (s *GMPPriceHistoryScraper) findByCompanyName(htmlContent, ipoName string) (string, bool) {
	normalizedName := s.normalizeIPOName(ipoName)
	nameWords := strings.Fields(normalizedName)

	linkPattern := `<a[^>]*href="/gmp/([^/]+)-ipo/(\d+)/"[^>]*>([^<]+)</a>`
	linkRe := regexp.MustCompile(linkPattern)
	matches := linkRe.FindAllStringSubmatch(htmlContent, -1)

	bestMatch := ""
	bestScore := 0.0
	bestNumericID := ""

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		numericID := match[2]
		linkText := match[3]

		normalizedLinkText := s.normalizeIPOName(linkText)
		score := s.calculateNameSimilarity(normalizedName, normalizedLinkText, nameWords)

		if score > bestScore && score > 0.5 { // Lowered from 60% to 50% similarity threshold
			bestScore = score
			bestMatch = linkText
			bestNumericID = numericID
		}
	}

	if bestNumericID != "" {
		s.logger.WithFields(logrus.Fields{
			"original_name": ipoName,
			"matched_name":  bestMatch,
			"similarity":    fmt.Sprintf("%.2f", bestScore),
		}).Debug("Company name similarity match found")
		return bestNumericID, true
	}

	return "", false
}

// findByPartialName finds IPO by partial name matching (keywords)
func (s *GMPPriceHistoryScraper) findByPartialName(htmlContent, ipoName string) (string, bool) {
	// Extract key words from IPO name (ignore common words)
	normalizedName := s.normalizeIPOName(ipoName)
	words := strings.Fields(normalizedName)

	// Filter out common words
	commonWords := map[string]bool{
		"ltd": true, "limited": true, "pvt": true, "private": true,
		"ipo": true, "sme": true, "india": true, "indian": true,
		"company": true, "corp": true, "corporation": true,
		"technologies": true, "tech": true, "systems": true,
	}

	keyWords := make([]string, 0)
	for _, word := range words {
		if len(word) > 2 && !commonWords[word] {
			keyWords = append(keyWords, word)
		}
	}

	if len(keyWords) == 0 {
		return "", false
	}

	linkPattern := `<a[^>]*href="/gmp/([^/]+)-ipo/(\d+)/"[^>]*>([^<]+)</a>`
	linkRe := regexp.MustCompile(linkPattern)
	matches := linkRe.FindAllStringSubmatch(htmlContent, -1)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		numericID := match[2]
		linkText := match[3]

		normalizedLinkText := s.normalizeIPOName(linkText)

		// Check if at least 2 key words are present
		matchCount := 0
		for _, keyWord := range keyWords {
			if strings.Contains(normalizedLinkText, keyWord) {
				matchCount++
			}
		}

		if matchCount >= 2 || (len(keyWords) == 1 && matchCount == 1) {
			s.logger.WithFields(logrus.Fields{
				"original_name": ipoName,
				"matched_name":  linkText,
				"key_words":     keyWords,
				"matches":       matchCount,
			}).Debug("Partial name match found")
			return numericID, true
		}
	}

	return "", false
}

// generateCompanyCodeVariations generates possible variations of a company code
func (s *GMPPriceHistoryScraper) generateCompanyCodeVariations(code string) []string {
	variations := []string{code}

	// Add variations with different separators
	variations = append(variations, strings.ReplaceAll(code, "-", ""))
	variations = append(variations, strings.ReplaceAll(code, "-", "_"))
	variations = append(variations, strings.ReplaceAll(code, "_", "-"))

	// Only strip URL-noise suffixes that never distinguish companies.
	// DO NOT strip semantically meaningful suffixes like "-india", "-ltd",
	// "-technologies", "-systems" etc. — those differentiate companies
	// (e.g., "manilam-industries-india" vs "manilam-industries" are DIFFERENT IPOs).
	noiseSuffixes := []string{"-ipo-details", "-details", "-ipo"}
	for _, suffix := range noiseSuffixes {
		if strings.HasSuffix(code, suffix) {
			variations = append(variations, strings.TrimSuffix(code, suffix))
		}
	}

	// Add variations with noise suffixes only
	for _, suffix := range noiseSuffixes {
		if !strings.HasSuffix(code, suffix) {
			variations = append(variations, code+suffix)
		}
	}

	return variations
}

// calculateNameSimilarity calculates similarity score between two company names
func (s *GMPPriceHistoryScraper) calculateNameSimilarity(name1, name2 string, words1 []string) float64 {
	if name1 == name2 {
		return 1.0
	}

	words2 := strings.Fields(name2)

	// Calculate word overlap with enhanced matching
	matchCount := 0
	totalWords := len(words1)

	for _, word1 := range words1 {
		// Skip very short words
		if len(word1) <= 2 {
			totalWords--
			continue
		}

		for _, word2 := range words2 {
			// Exact match (highest priority)
			if word1 == word2 {
				matchCount++
				break
			}
			// Substring match (medium priority)
			if len(word1) > 3 && len(word2) > 3 {
				if strings.Contains(word1, word2) || strings.Contains(word2, word1) {
					matchCount++
					break
				}
			}
		}
	}

	if totalWords == 0 {
		return 0.0
	}

	// Calculate base similarity as percentage of matching words
	similarity := float64(matchCount) / float64(totalWords)

	// Bonus for substring matches
	if strings.Contains(name1, name2) || strings.Contains(name2, name1) {
		similarity += 0.2
	}

	// Bonus for having key company identifiers in common
	keyWords := []string{"security", "technologies", "media", "labs", "systems", "industries"}
	for _, keyWord := range keyWords {
		if strings.Contains(name1, keyWord) && strings.Contains(name2, keyWord) {
			similarity += 0.1
			break
		}
	}

	// Cap at 1.0
	if similarity > 1.0 {
		similarity = 1.0
	}

	return similarity
}

// normalizeIPOName normalizes an IPO name for matching
func (s *GMPPriceHistoryScraper) normalizeIPOName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Remove common suffixes and exchange indicators (enhanced list)
	suffixes := []string{
		" ipo gmp", " ipo", " sme", " nse sme", " bse sme", " nse ipo", " bse ipo",
		" nse", " bse", " limited", " ltd", " pvt", " private limited",
		" pvt ltd", " pvt. ltd.", " ltd.", " company", " corp", " corporation",
		" inc", " llc", " india", " indian", " technologies", " tech", " systems",
		" industries", " group", " enterprises", " solutions", " services",
	}

	// Apply suffixes multiple times to handle combinations like "India Ltd."
	for i := 0; i < 3; i++ {
		originalName := name
		for _, suffix := range suffixes {
			name = strings.TrimSuffix(name, suffix)
		}
		// If no change in this iteration, break
		if name == originalName {
			break
		}
	}

	// Remove special characters and extra spaces
	name = regexp.MustCompile(`[^a-z0-9\s]`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	return name
}

// ValidateScrapedData validates the scraped data against business rules
// Implements Requirement 1.4 - Validate extracted data
// Enhanced with more lenient validation for varying data quality
func (s *GMPPriceHistoryScraper) ValidateScrapedData(data *ScrapedHistoryData) error {
	logger := s.getLogger()

	if data == nil {
		return fmt.Errorf("scraped data is nil")
	}

	if data.IPOName == "" {
		return fmt.Errorf("IPO name is empty")
	}

	if data.IPOPrice <= 0 {
		logger.WithFields(logrus.Fields{
			"ipo_name": data.IPOName,
		}).Debug("IPO price missing in source data; continuing with partial history records")
	}

	if len(data.PriceHistory) == 0 {
		return fmt.Errorf("no price history entries found")
	}

	// Validate each entry and skip invalid data, continue processing if at least one valid entry exists
	validEntries := 0
	validPriceHistory := make([]models.GMPPriceHistoryEntry, 0, len(data.PriceHistory))
	for idx := range data.PriceHistory {
		entry := &data.PriceHistory[idx]
		if err := s.validateHistoryEntryLenient(entry); err != nil {
			logger.WithFields(logrus.Fields{
				"entry_index": idx,
				"error":       err.Error(),
				"record_date": entry.RecordDate.Format("2006-01-02"),
			}).Warn("Skipping invalid entry - continuing with remaining entries")
			continue
		}
		validEntries++
		validPriceHistory = append(validPriceHistory, *entry)
	}

	if validEntries == 0 {
		return fmt.Errorf("no valid price history entries found after validation")
	}

	data.PriceHistory = validPriceHistory
	logger.WithFields(logrus.Fields{
		"original_entries": len(data.PriceHistory),
		"valid_entries":    validEntries,
	}).Info("Filtered to valid entries only")

	logger.WithFields(logrus.Fields{
		"total_entries": len(data.PriceHistory),
		"valid_entries": validEntries,
	}).Info("Data validation completed with lenient rules")

	return nil
}

// validateHistoryEntryLenient validates a single history entry with more lenient rules
func (s *GMPPriceHistoryScraper) validateHistoryEntryLenient(entry *models.GMPPriceHistoryEntry) error {
	logger := s.getLogger()

	// GMP can be negative (grey market discount), but IPO price must be non-negative
	if entry.IPOPrice < 0 {
		return fmt.Errorf("IPO price cannot be negative: %.2f", entry.IPOPrice)
	}

	// Validate that record date is not zero
	if entry.RecordDate.IsZero() {
		return fmt.Errorf("record date cannot be zero")
	}

	// Date range validation (more lenient)
	now := time.Now()
	threeYearsAgo := now.AddDate(-3, 0, 0)
	twoYearsFuture := now.AddDate(2, 0, 0)

	if entry.RecordDate.Before(threeYearsAgo) || entry.RecordDate.After(twoYearsFuture) {
		return fmt.Errorf("record date out of reasonable range: %s", entry.RecordDate.Format("2006-01-02"))
	}

	// Skip estimated listing validation if values seem incorrect
	// This is common with varying table structures
	if entry.EstimatedListing > 0 && entry.IPOPrice > 0 && entry.GMPValue >= 0 {
		expectedListing := entry.IPOPrice + entry.GMPValue
		diff := entry.EstimatedListing - expectedListing
		tolerance := entry.IPOPrice * 0.5 // 50% tolerance for parsing errors

		if diff < -tolerance || diff > tolerance {
			// Don't fail validation, just fix the value
			entry.EstimatedListing = expectedListing
			logger.WithFields(logrus.Fields{
				"original_listing":  entry.EstimatedListing + diff,
				"corrected_listing": entry.EstimatedListing,
				"ipo_price":         entry.IPOPrice,
				"gmp_value":         entry.GMPValue,
			}).Debug("Corrected estimated listing price")
		}
	}

	// Recalculate percentage if needed
	if entry.IPOPrice > 0 && entry.EstimatedListing > 0 {
		expectedPercent := ((entry.EstimatedListing - entry.IPOPrice) / entry.IPOPrice) * 100
		entry.ListingPercent = expectedPercent
	}

	return nil
}

// validateHistoryEntry validates a single history entry
// Implements Requirements 5.1, 5.2, 5.3, 5.4 - Data validation rules
func (s *GMPPriceHistoryScraper) validateHistoryEntry(entry *models.GMPPriceHistoryEntry) error {
	// GMP can be negative (grey market discount) - this is valid data
	// IPO price must be non-negative
	if entry.IPOPrice < 0 {
		return fmt.Errorf("IPO price cannot be negative: %.2f", entry.IPOPrice)
	}

	if entry.EstimatedListing < 0 {
		return fmt.Errorf("estimated listing price cannot be negative: %.2f", entry.EstimatedListing)
	}

	if entry.EstimatedProfit < 0 {
		return fmt.Errorf("estimated profit cannot be negative: %.2f", entry.EstimatedProfit)
	}

	if entry.Sub2Sauda < 0 {
		return fmt.Errorf("sub2 sauda value cannot be negative: %.2f", entry.Sub2Sauda)
	}

	// Requirement 5.3: Verify estimated listing price calculation
	// EstimatedListing should equal IPOPrice + GMPValue
	expectedListing := entry.IPOPrice + entry.GMPValue
	diff := entry.EstimatedListing - expectedListing
	tolerance := 0.01 // Allow 1 paisa tolerance for rounding
	if diff < -tolerance || diff > tolerance {
		return fmt.Errorf("estimated listing price mismatch: expected %.2f (IPO %.2f + GMP %.2f), got %.2f",
			expectedListing, entry.IPOPrice, entry.GMPValue, entry.EstimatedListing)
	}

	// Requirement 5.4: Verify percentage calculation
	// ListingPercent should match the gain percentage from IPO price to estimated listing
	if entry.IPOPrice > 0 {
		expectedPercent := ((entry.EstimatedListing - entry.IPOPrice) / entry.IPOPrice) * 100
		percentDiff := entry.ListingPercent - expectedPercent
		percentTolerance := 0.1 // Allow 0.1% tolerance for rounding
		if percentDiff < -percentTolerance || percentDiff > percentTolerance {
			return fmt.Errorf("listing percentage mismatch: expected %.2f%%, got %.2f%%",
				expectedPercent, entry.ListingPercent)
		}
	}

	// Validate that record date is not zero (check this first)
	if entry.RecordDate.IsZero() {
		return fmt.Errorf("record date cannot be zero")
	}

	// Requirement 5.2: Dates should be within reasonable range
	now := time.Now()
	twoYearsAgo := now.AddDate(-2, 0, 0)
	oneYearFuture := now.AddDate(1, 0, 0)

	if entry.RecordDate.Before(twoYearsAgo) || entry.RecordDate.After(oneYearFuture) {
		return fmt.Errorf("record date out of reasonable range: %s (must be between %s and %s)",
			entry.RecordDate.Format("2006-01-02"),
			twoYearsAgo.Format("2006-01-02"),
			oneYearFuture.Format("2006-01-02"))
	}

	return nil
}

// Helper functions for parsing HTML content

func (s *GMPPriceHistoryScraper) extractIPOName(html string) string {
	// Extract IPO Name from h1 tag
	namePattern := `<h1[^>]*>([^<]+(?:SME )?IPO GMP)</h1>`
	if match := regexp.MustCompile(namePattern).FindStringSubmatch(html); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func (s *GMPPriceHistoryScraper) extractIPOPrice(html string) float64 {
	// Extract IPO price from text
	pricePattern := `price band of (\d+\.?\d*)`
	if match := regexp.MustCompile(pricePattern).FindStringSubmatch(html); len(match) > 1 {
		price, _ := strconv.ParseFloat(match[1], 64)
		return price
	}
	return 0
}

func (s *GMPPriceHistoryScraper) extractCurrentGMP(html string) float64 {
	// Extract current GMP value
	gmpPattern := `last GMP is ₹(\d+)`
	if match := regexp.MustCompile(gmpPattern).FindStringSubmatch(html); len(match) > 1 {
		gmp, _ := strconv.ParseFloat(match[1], 64)
		return gmp
	}
	return 0
}

func (s *GMPPriceHistoryScraper) cleanText(text string) string {
	text = html.UnescapeString(text)

	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	text = re.ReplaceAllString(text, "")
	// Clean whitespace
	text = strings.TrimSpace(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	// Remove &nbsp; and other HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", "")
	text = strings.ReplaceAll(text, "&amp;", "&")
	return text
}

func (s *GMPPriceHistoryScraper) parseFloat(text string) float64 {
	text = s.cleanText(text)
	text = strings.ReplaceAll(text, "₹", "")
	text = strings.ReplaceAll(text, ",", "")
	text = strings.ReplaceAll(text, "%", "")
	text = strings.ReplaceAll(text, "+", "")
	text = strings.TrimSpace(text)

	if text == "" || text == "--" || text == "-" || strings.EqualFold(text, "na") || strings.EqualFold(text, "n/a") {
		return 0
	}

	if numericMatch := regexp.MustCompile(`[-+]?\d*\.?\d+`).FindString(text); numericMatch != "" {
		text = numericMatch
	}

	val, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	return val
}

func (s *GMPPriceHistoryScraper) parseGMPValue(cellContent string) float64 {
	cleaned := s.cleanText(cellContent)
	cleaned = strings.ReplaceAll(cleaned, "₹", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" || cleaned == "--" || cleaned == "-" || strings.EqualFold(cleaned, "na") || strings.EqualFold(cleaned, "n/a") {
		return 0
	}

	if numericMatch := regexp.MustCompile(`[-+]?\d*\.?\d+`).FindString(cleaned); numericMatch != "" {
		val, err := strconv.ParseFloat(numericMatch, 64)
		if err == nil {
			return val
		}
	}

	return 0
}

func (s *GMPPriceHistoryScraper) parseEstimatedListingPrice(cellContent string) float64 {
	// Extract price like "₹61 (<span...>7.02%</span>)"
	re := regexp.MustCompile(`₹(\d+\.?\d*)`)
	matches := re.FindStringSubmatch(cellContent)
	if len(matches) > 1 {
		val, _ := strconv.ParseFloat(matches[1], 64)
		return val
	}

	return s.parseFloat(cellContent)
}

func (s *GMPPriceHistoryScraper) parseEstimatedListingPercent(cellContent string) float64 {
	// Extract percentage like "₹61 (<span...>7.02%</span>)"
	re := regexp.MustCompile(`(\d+\.?\d*)%`)
	matches := re.FindStringSubmatch(cellContent)
	if len(matches) > 1 {
		val, _ := strconv.ParseFloat(matches[1], 64)
		return val
	}
	return 0
}

func (s *GMPPriceHistoryScraper) parseDate(dateStr string) (time.Time, error) {
	// Try multiple date formats
	formats := []string{
		"02-Jan-2006",
		"2-Jan-2006",
		"02-01-2006",
		"2-1-2006",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// normalizeSubscriptionStatus normalizes subscription status text to consistent patterns
// Implements Requirement 5.5 - Subscription data normalization
func (s *GMPPriceHistoryScraper) normalizeSubscriptionStatus(status string) string {
	if status == "" {
		return "Not Available"
	}

	// Convert to lowercase for pattern matching
	lowerStatus := strings.ToLower(status)

	// Pattern 1: "X times" or "X x" -> "Xx subscribed"
	// Examples: "10 times", "5 x", "2.5 times" -> "10x subscribed", "5x subscribed", "2.5x subscribed"
	timesPattern := regexp.MustCompile(`(\d+\.?\d*)\s*(?:times|x)`)
	if matches := timesPattern.FindStringSubmatch(lowerStatus); len(matches) > 1 {
		return fmt.Sprintf("%sx subscribed", matches[1])
	}

	// Pattern 2: "subscribed X times" -> "Xx subscribed"
	// Example: "subscribed 10 times" -> "10x subscribed"
	subscribedTimesPattern := regexp.MustCompile(`subscribed\s+(\d+\.?\d*)\s*(?:times|x)?`)
	if matches := subscribedTimesPattern.FindStringSubmatch(lowerStatus); len(matches) > 1 {
		return fmt.Sprintf("%sx subscribed", matches[1])
	}

	// Pattern 3: "oversubscribed" variations
	// Examples: "oversubscribed", "over subscribed", "over-subscribed"
	if strings.Contains(lowerStatus, "oversubscribed") ||
		strings.Contains(lowerStatus, "over subscribed") ||
		strings.Contains(lowerStatus, "over-subscribed") {
		return "oversubscribed"
	}

	// Pattern 4: "undersubscribed" variations
	// Examples: "undersubscribed", "under subscribed", "under-subscribed"
	if strings.Contains(lowerStatus, "undersubscribed") ||
		strings.Contains(lowerStatus, "under subscribed") ||
		strings.Contains(lowerStatus, "under-subscribed") {
		return "undersubscribed"
	}

	// Pattern 5: "not subscribed" or "no subscription"
	if strings.Contains(lowerStatus, "not subscribed") ||
		strings.Contains(lowerStatus, "no subscription") {
		return "not subscribed"
	}

	// Pattern 6: "fully subscribed"
	if strings.Contains(lowerStatus, "fully subscribed") ||
		strings.Contains(lowerStatus, "full subscription") {
		return "fully subscribed"
	}

	// Pattern 7: Percentage format "X%" -> "X% subscribed"
	// Example: "150%", "75%" -> "150% subscribed", "75% subscribed"
	percentPattern := regexp.MustCompile(`(\d+\.?\d*)%`)
	if matches := percentPattern.FindStringSubmatch(lowerStatus); len(matches) > 1 {
		return fmt.Sprintf("%s%% subscribed", matches[1])
	}

	// Pattern 8: Just a number -> assume it's times subscribed
	// Example: "10", "2.5" -> "10x subscribed", "2.5x subscribed"
	numberPattern := regexp.MustCompile(`^(\d+\.?\d*)$`)
	if matches := numberPattern.FindStringSubmatch(strings.TrimSpace(lowerStatus)); len(matches) > 1 {
		return fmt.Sprintf("%sx subscribed", matches[1])
	}

	// If no pattern matches, return cleaned original status
	return strings.TrimSpace(status)
}

// GetCircuitBreakerMetrics returns current circuit breaker metrics for monitoring
func (s *GMPPriceHistoryScraper) GetCircuitBreakerMetrics() map[string]interface{} {
	return s.circuitBreaker.GetMetrics()
}

// ResetCircuitBreaker resets the circuit breaker to closed state
// Useful for manual recovery or testing
func (s *GMPPriceHistoryScraper) ResetCircuitBreaker() {
	s.circuitBreaker.Reset()
	s.logger.Info("Circuit breaker manually reset")
}
