package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/shared"
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

type GMPData struct {
	IPOName          string
	CompanyCode      string
	IPOPrice         float64
	GMPValue         float64
	EstimatedListing float64
	GainPercent      float64
	Sub2             float64
	Kostak           float64
	ListingDate      *time.Time
}

// FetchGMPData scrapes the GMP table from InvestorGain using chromedp with enhanced architecture
func (s *EnhancedGMPService) FetchGMPData() ([]GMPData, error) {
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
	s.extractionMetrics.RecordAttempt(false) // Will be updated to true on success

	// Define allocator options for efficiency with enhanced configuration
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.Flag("mute-audio", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, s.configuration.HTTPRequestTimeout)
	defer cancel()

	var rawData []map[string]string

	// Run tasks with enhanced error handling
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(s.baseURL),
		chromedp.WaitVisible("div#reportData table tbody tr", chromedp.ByQuery),
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('div#reportData table tbody tr')).map(row => {
				const cols = Array.from(row.querySelectorAll('td')).map(td => td.innerText.trim());
				return {
					ipoName: cols[0] || '',
					gmpRaw:  cols[1] || '',
					price:   cols[5] || '',
					listingDate: cols[10] || ''
				};
			}).filter(r => r && r.ipoName !== '' && r.ipoName !== 'IPO Name')
		`, &rawData),
	)

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

	var gmpList []GMPData
	successfulRecords := 0
	processingErrors := 0

	logger.WithField("raw_records_count", len(rawData)).Info("Starting enhanced text processing for GMP records")

	for recordIndex, item := range rawData {
		recordLogger := logger.WithFields(logrus.Fields{
			"record_index":    recordIndex,
			"processing_step": "text_processing",
		})

		// Enhanced text processing for IPO name
		originalName := item["ipoName"]
		normalizedName := s.utilityService.NormalizeTextContent(originalName)
		cleanedName := s.utilityService.CleanCompanyText(normalizedName)

		// Enhanced numeric processing for price field
		originalPrice := item["price"]
		normalizedPrice := s.utilityService.NormalizeTextContent(originalPrice)
		price := s.utilityService.ExtractNumeric(normalizedPrice)

		// Enhanced GMP string processing
		originalGMP := item["gmpRaw"]
		gmpValue, gainPercent := s.parseGMPString(originalGMP)

		// Calculate estimated listing price
		estimatedListing := price + gmpValue

		// Enhanced date processing
		originalDate := item["listingDate"]
		normalizedDate := s.utilityService.NormalizeTextContent(originalDate)
		listingDate := s.utilityService.ParseStandardDateFormats(normalizedDate)

		// Create GMP record
		gmp := GMPData{
			IPOName:          cleanedName,
			CompanyCode:      s.utilityService.GenerateCompanyCode(cleanedName),
			IPOPrice:         price,
			GMPValue:         gmpValue,
			EstimatedListing: estimatedListing,
			GainPercent:      gainPercent,
			Sub2:             0, // Not available in current view
			Kostak:           0, // Not available in current view
			ListingDate:      listingDate,
		}

		// Validate processed record
		recordValid := true
		if cleanedName == "" || (price <= 0 && gmpValue == 0) {
			recordValid = false
		}

		if recordValid {
			gmpList = append(gmpList, gmp)
			successfulRecords++
			recordLogger.Info("Successfully processed GMP record")
		} else {
			processingErrors++
			s.extractionMetrics.RecordProcessingError()
			recordLogger.Warn("Failed to process GMP record due to validation issues")
		}
	}

	// Update metrics for successful extraction
	s.extractionMetrics.SuccessfulParsed++
	s.extractionMetrics.FailedParsed--

	logger.WithFields(logrus.Fields{
		"total_raw_records":  len(rawData),
		"successful_records": successfulRecords,
		"processing_errors":  processingErrors,
		"success_rate":       float64(successfulRecords) / float64(len(rawData)) * 100.0,
		"processing_time":    time.Since(startTime),
	}).Info("Successfully completed GMP data extraction")

	return gmpList, nil
}

// FetchGMPData maintains backward compatibility
func (s *GMPService) FetchGMPData() ([]GMPData, error) {
	return s.EnhancedGMPService.FetchGMPData()
}

// parseGMPString extracts GMP value and percentage from string like "₹21 (25.61%)"
func (s *EnhancedGMPService) parseGMPString(gmpText string) (float64, float64) {
	// Use enhanced utility service for comprehensive text normalization
	normalizedText := s.utilityService.NormalizeTextContent(gmpText)

	// Remove currency symbols using utility service patterns
	cleanedText := strings.ReplaceAll(normalizedText, "₹", "")
	cleanedText = strings.ReplaceAll(cleanedText, ",", "")
	cleanedText = strings.TrimSpace(cleanedText)

	// Split by "(" to separate value and percentage
	parts := strings.Split(cleanedText, "(")
	if len(parts) < 2 {
		// Try to parse just the value if no percentage
		val := s.utilityService.ExtractNumeric(cleanedText)
		return val, 0.0
	}

	// Parse Value using enhanced utility service
	valStr := strings.TrimSpace(parts[0])
	val := s.utilityService.ExtractNumeric(valStr)

	// Parse Percentage using enhanced utility service
	pctStr := strings.TrimSpace(parts[1])
	pctStr = strings.ReplaceAll(pctStr, ")", "")
	pct := s.utilityService.ExtractPercentage(pctStr)

	return val, pct
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

// ============================================================================
// IPO Scraping Functionality (Merged from simplified_ipo_scraper.go)
// ============================================================================

// FetchAvailableIPOList retrieves the complete list of IPOs from Chittorgarh's internal API
func (s *EnhancedGMPService) FetchAvailableIPOList() ([]ChittorgarhIPOListItem, error) {
	apiEndpointURL := "https://webnodejs.chittorgarh.com/cloud/ipo/list-read"

	logger := logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"method":    "FetchAvailableIPOList",
		"url":       apiEndpointURL,
	})

	logger.Info("Fetching available IPO list from Chittorgarh API")

	// Enforce rate limiting before making the request
	s.requestRateLimiter.EnforceRateLimit()

	// Create HTTP request with appropriate headers
	httpRequest, requestError := http.NewRequest("GET", apiEndpointURL, nil)
	if requestError != nil {
		logger.WithError(requestError).Error("Failed to create HTTP request")
		return nil, fmt.Errorf("failed to create HTTP request: %w", requestError)
	}

	// Set browser-like headers to avoid detection as automated scraper
	shared.SetBrowserLikeHeaders(httpRequest, "application/json, text/plain, */*")

	// Execute HTTP request with retry logic using shared functionality
	httpResponse, executionError := shared.ExecuteHTTPRequestWithRetry(s.httpClient, httpRequest, 3)
	if executionError != nil {
		logger.WithError(executionError).Error("Failed to fetch IPO list after retries")
		return nil, fmt.Errorf("failed to fetch IPO list: %w", executionError)
	}
	defer httpResponse.Body.Close()

	logger.WithField("status_code", httpResponse.StatusCode).Debug("Successfully fetched IPO list")

	// Parse JSON response into structured data
	var apiResponse struct {
		Status          int                      `json:"status"`
		Message         int                      `json:"msg"`
		IPODropDownList []ChittorgarhIPOListItem `json:"ipoDropDownList"`
	}

	if jsonParseError := json.NewDecoder(httpResponse.Body).Decode(&apiResponse); jsonParseError != nil {
		logger.WithError(jsonParseError).Error("Failed to parse IPO list JSON response")
		return nil, fmt.Errorf("failed to parse IPO list JSON response: %w", jsonParseError)
	}

	// Validate API response structure and content
	if apiResponse.Status == 0 && len(apiResponse.IPODropDownList) == 0 {
		logger.WithField("status", apiResponse.Status).Warn("API returned empty response")
		return nil, fmt.Errorf("API returned empty response with status code: %d", apiResponse.Status)
	}

	logger.WithField("ipo_count", len(apiResponse.IPODropDownList)).Info("Successfully fetched IPO list")
	return apiResponse.IPODropDownList, nil
}

// ScrapeDetailedIPOInformation extracts comprehensive IPO data from a specific IPO detail page
func (s *EnhancedGMPService) ScrapeDetailedIPOInformation(ipoListItem ChittorgarhIPOListItem) (*models.IPO, error) {
	logger := logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"method":    "ScrapeDetailedIPOInformation",
		"ipo_id":    ipoListItem.ID,
		"ipo_title": ipoListItem.IPONewsTitle,
	})

	logger.Info("Starting detailed IPO information scraping")

	// Enforce rate limiting before making the request
	s.requestRateLimiter.EnforceRateLimit()

	// Construct URL for the IPO detail page
	ipoDetailPageURL := fmt.Sprintf("https://www.chittorgarh.com/ipo/%s/%d/", ipoListItem.URLRewriteFolderName, ipoListItem.ID)
	logger.WithField("url", ipoDetailPageURL).Debug("Constructed IPO detail page URL")

	// Create HTTP request with appropriate headers
	httpRequest, requestError := http.NewRequest("GET", ipoDetailPageURL, nil)
	if requestError != nil {
		logger.WithError(requestError).Error("Failed to create HTTP request")
		return nil, fmt.Errorf("failed to create HTTP request for IPO %d: %w", ipoListItem.ID, requestError)
	}

	// Set browser-like headers for HTML content
	shared.SetBrowserLikeHeaders(httpRequest, "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")

	// Execute HTTP request with retry logic using shared functionality
	httpResponse, executionError := shared.ExecuteHTTPRequestWithRetry(s.httpClient, httpRequest, 3)
	if executionError != nil {
		logger.WithError(executionError).Error("Failed to fetch IPO detail page after retries")
		// Return partial IPO data when detailed scraping fails
		partialIPOData := s.createPartialIPOFromListItem(ipoListItem)
		return partialIPOData, fmt.Errorf("failed to fetch IPO detail page: %w", executionError)
	}
	defer httpResponse.Body.Close()

	logger.WithField("status_code", httpResponse.StatusCode).Debug("Successfully fetched IPO detail page")

	// Read the entire response body as text to extract JSON data
	bodyBytes, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		logger.WithError(readError).Error("Failed to read response body")
		partialIPOData := s.createPartialIPOFromListItem(ipoListItem)
		return partialIPOData, fmt.Errorf("failed to read response body for IPO %d: %w", ipoListItem.ID, readError)
	}

	bodyText := string(bodyBytes)
	logger.WithField("body_length", len(bodyText)).Debug("Read response body")

	// Parse HTML document for both JSON and HTML extraction paths
	htmlDocument, parseError := goquery.NewDocumentFromReader(strings.NewReader(bodyText))
	if parseError != nil {
		logger.WithError(parseError).Error("Failed to parse HTML document")
		partialIPOData := s.createPartialIPOFromListItem(ipoListItem)
		return partialIPOData, fmt.Errorf("failed to parse HTML document for IPO %d: %w", ipoListItem.ID, parseError)
	}

	logger.Debug("Successfully parsed HTML document")

	// Try to extract JSON data from the JavaScript embedded in the page
	ipoData, jsonError := s.extractIPODataFromJSON(bodyText, ipoListItem, htmlDocument)
	if jsonError != nil {
		logger.WithError(jsonError).Warn("JSON extraction failed, falling back to HTML parsing")
		// Fallback to HTML parsing if JSON extraction fails
		ipoData = s.buildIPOModelFromHTMLExtraction(ipoListItem, htmlDocument)
	} else {
		logger.Info("Successfully extracted IPO data from JSON")
	}

	logger.WithFields(logrus.Fields{
		"ipo_name":        ipoData.Name,
		"company_code":    ipoData.CompanyCode,
		"has_description": ipoData.Description != nil,
		"has_about":       ipoData.About != nil,
	}).Info("Completed detailed IPO information scraping")

	return ipoData, nil
}

// createPartialIPOFromListItem creates a partial IPO model when detailed scraping fails
func (s *EnhancedGMPService) createPartialIPOFromListItem(listItem ChittorgarhIPOListItem) *models.IPO {
	currentTimestamp := time.Now()

	partialIPO := &models.IPO{
		Name:        listItem.IPONewsTitle,
		CompanyCode: s.utilityService.GenerateCompanyCode(listItem.IPONewsTitle),
		StockID:     strconv.Itoa(listItem.ID),
		Status:      "Unknown",
		Registrar:   "Unknown",
		CreatedAt:   currentTimestamp,
		UpdatedAt:   currentTimestamp,
	}

	if listItem.LogoURL != "" {
		partialIPO.LogoURL = &listItem.LogoURL
	}

	return partialIPO
}

// buildIPOModelFromHTMLExtraction constructs an IPO model from HTML extraction
func (s *EnhancedGMPService) buildIPOModelFromHTMLExtraction(listItem ChittorgarhIPOListItem, htmlDocument *goquery.Document) *models.IPO {
	logger := logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"method":    "buildIPOModelFromHTMLExtraction",
		"ipo_id":    listItem.ID,
	})

	logger.Debug("Building IPO model from HTML extraction")

	currentTimestamp := time.Now()
	extractor := NewHTMLDataExtractor()

	// Extract structured data from HTML document
	basicInfo := extractor.ExtractBasicInformation(htmlDocument)
	dateInfo := extractor.ExtractDateInformation(htmlDocument)
	pricingInfo := extractor.ExtractPricingInformation(htmlDocument)
	statusInfo := extractor.ExtractStatusInformation(htmlDocument)

	ipoModel := &models.IPO{
		StockID:   strconv.Itoa(listItem.ID),
		CreatedAt: currentTimestamp,
		UpdatedAt: currentTimestamp,
	}

	// Set basic information with fallbacks to list item data
	if basicInfo.CompanyName != "" {
		ipoModel.Name = basicInfo.CompanyName
	} else {
		ipoModel.Name = listItem.IPONewsTitle
	}

	if basicInfo.CompanyCode != "" {
		ipoModel.CompanyCode = basicInfo.CompanyCode
	} else {
		ipoModel.CompanyCode = s.utilityService.GenerateCompanyCode(listItem.IPONewsTitle)
	}

	if basicInfo.RegistrarName != "" {
		ipoModel.Registrar = basicInfo.RegistrarName
	} else {
		ipoModel.Registrar = "Unknown"
	}

	// Set optional basic information
	if basicInfo.StockSymbol != nil {
		ipoModel.Symbol = basicInfo.StockSymbol
	}

	// Set date information
	ipoModel.OpenDate = dateInfo.SubscriptionOpenDate
	ipoModel.CloseDate = dateInfo.SubscriptionCloseDate
	ipoModel.ResultDate = dateInfo.AllotmentResultDate
	ipoModel.ListingDate = dateInfo.StockListingDate

	// Set pricing information
	ipoModel.PriceBandLow = pricingInfo.PriceBandMinimum
	ipoModel.PriceBandHigh = pricingInfo.PriceBandMaximum
	ipoModel.IssueSize = pricingInfo.TotalIssueSize
	ipoModel.MinQty = pricingInfo.MinimumLotQuantity
	ipoModel.MinAmount = pricingInfo.MinimumInvestmentAmount

	// Extract description and about from HTML
	if htmlDescription := extractor.ExtractCompanyDescription(htmlDocument); htmlDescription != nil {
		ipoModel.Description = htmlDescription
	}

	if htmlAbout := extractor.ExtractCompanyAbout(htmlDocument); htmlAbout != nil {
		ipoModel.About = htmlAbout
	}

	// Calculate status based on dates
	ipoModel.Status = s.utilityService.CalculateIPOStatus(ipoModel.OpenDate, ipoModel.CloseDate, ipoModel.ListingDate)
	ipoModel.SubscriptionStatus = statusInfo.SubscriptionStatus
	ipoModel.ListingGain = statusInfo.ListingPerformance

	// Set logo URL from list item
	if listItem.LogoURL != "" {
		ipoModel.LogoURL = &listItem.LogoURL
	}

	logger.WithFields(logrus.Fields{
		"final_name":         ipoModel.Name,
		"final_company_code": ipoModel.CompanyCode,
		"has_description":    ipoModel.Description != nil,
		"has_about":          ipoModel.About != nil,
		"final_status":       ipoModel.Status,
	}).Debug("Completed IPO model building from HTML data")

	return ipoModel
}

// extractIPODataFromJSON extracts IPO data from JSON embedded in the page
func (s *EnhancedGMPService) extractIPODataFromJSON(bodyText string, ipoListItem ChittorgarhIPOListItem, htmlDocument *goquery.Document) (*models.IPO, error) {
	logger := logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"method":    "extractIPODataFromJSON",
		"ipo_id":    ipoListItem.ID,
	})

	logger.Debug("Starting JSON extraction from page content")

	// Look for the JSON data pattern in the JavaScript
	startPattern := `\\"ipoData\\":\s*\[`
	startRegex, err := regexp.Compile(startPattern)
	if err != nil {
		logger.WithError(err).Error("Failed to compile start pattern regex")
		return nil, fmt.Errorf("failed to compile start pattern regex: %w", err)
	}

	startMatch := startRegex.FindStringIndex(bodyText)
	if startMatch == nil {
		logger.Warn("Could not find ipoData start pattern in page content")
		return nil, fmt.Errorf("could not find ipoData start pattern in page content")
	}

	logger.WithField("start_position", startMatch[0]).Debug("Found ipoData start pattern")

	// Find the opening brace after the array start
	searchStart := startMatch[1]
	openBraceIndex := strings.Index(bodyText[searchStart:], "{")
	if openBraceIndex == -1 {
		return nil, fmt.Errorf("could not find opening brace for ipoData JSON")
	}

	jsonStart := searchStart + openBraceIndex

	// Now find the matching closing brace by counting braces
	braceCount := 0
	jsonEnd := -1

	for i := jsonStart; i < len(bodyText); i++ {
		char := bodyText[i]
		if char == '{' {
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount == 0 {
				jsonEnd = i + 1
				break
			}
		}
	}

	if jsonEnd == -1 {
		return nil, fmt.Errorf("could not find closing brace for ipoData JSON")
	}

	jsonStr := bodyText[jsonStart:jsonEnd]

	// Unescape the JSON string (it's escaped for JavaScript)
	unescapedJSON := strings.ReplaceAll(jsonStr, `\"`, `"`)
	unescapedJSON = strings.ReplaceAll(unescapedJSON, `\\`, `\`)

	logger.WithField("json_length", len(unescapedJSON)).Debug("Extracted and unescaped JSON")

	// Parse the JSON data
	var ipoData ChittorgarhIPOData
	if err := json.Unmarshal([]byte(unescapedJSON), &ipoData); err != nil {
		logger.WithError(err).Error("Failed to parse IPO JSON data")
		return nil, fmt.Errorf("failed to parse IPO JSON data: %w", err)
	}

	logger.WithField("company_name", ipoData.CompanyName).Debug("Successfully parsed JSON data")

	// Convert to our IPO model
	return s.convertChittorgarhDataToIPO(ipoData, ipoListItem, htmlDocument)
}

// convertChittorgarhDataToIPO converts Chittorgarh JSON data to our IPO model
func (s *EnhancedGMPService) convertChittorgarhDataToIPO(data ChittorgarhIPOData, listItem ChittorgarhIPOListItem, htmlDocument *goquery.Document) (*models.IPO, error) {
	logger := logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"method":    "convertChittorgarhDataToIPO",
		"ipo_id":    data.ID,
	})

	logger.Debug("Converting Chittorgarh JSON data to IPO model")

	currentTimestamp := time.Now()

	ipoModel := &models.IPO{
		StockID:     strconv.Itoa(data.ID),
		Name:        data.CompanyName,
		CompanyCode: s.utilityService.GenerateCompanyCode(data.CompanyName),
		CreatedAt:   currentTimestamp,
		UpdatedAt:   currentTimestamp,
	}

	// Set dates
	if data.IssueOpenDate != "" {
		if openDate := s.utilityService.ParseStandardDateFormats(data.IssueOpenDate); openDate != nil {
			ipoModel.OpenDate = openDate
		}
	}

	if data.IssueCloseDate != "" {
		if closeDate := s.utilityService.ParseStandardDateFormats(data.IssueCloseDate); closeDate != nil {
			ipoModel.CloseDate = closeDate
		}
	}

	if data.TimetableListingDate != "" {
		if listingDate := s.utilityService.ParseStandardDateFormats(data.TimetableListingDate); listingDate != nil {
			ipoModel.ListingDate = listingDate
		}
	}

	if data.TimetableResultDate != "" {
		if resultDate := s.utilityService.ParseStandardDateFormats(data.TimetableResultDate); resultDate != nil {
			ipoModel.ResultDate = resultDate
		}
	}

	// Set pricing information
	if data.IssuePriceLower > 0 {
		ipoModel.PriceBandLow = &data.IssuePriceLower
	}
	if data.IssuePriceUpper > 0 {
		ipoModel.PriceBandHigh = &data.IssuePriceUpper
	}

	if data.IssueSizeInAmt != "" {
		ipoModel.IssueSize = &data.IssueSizeInAmt
	}

	if data.MarketLotSize > 0 {
		ipoModel.MinQty = &data.MarketLotSize
	}

	// Set other information
	if data.NSESymbol != "" {
		ipoModel.Symbol = &data.NSESymbol
	}

	if data.RegistrarName != "" {
		ipoModel.Registrar = data.RegistrarName
	} else {
		ipoModel.Registrar = "Unknown"
	}

	// Set description and about if available
	if data.Description != "" {
		cleanedDescription := s.utilityService.CleanCompanyText(data.Description)
		if cleanedDescription != "" {
			ipoModel.Description = &cleanedDescription
		}
	}

	if data.About != "" {
		cleanedAbout := s.utilityService.CleanCompanyText(data.About)
		if cleanedAbout != "" {
			ipoModel.About = &cleanedAbout
		}
	}

	// Calculate status based on dates
	ipoModel.Status = s.utilityService.CalculateIPOStatus(ipoModel.OpenDate, ipoModel.CloseDate, ipoModel.ListingDate)

	// Set logo URL from list item
	if listItem.LogoURL != "" {
		ipoModel.LogoURL = &listItem.LogoURL
	}

	logger.WithFields(logrus.Fields{
		"final_name":         ipoModel.Name,
		"final_company_code": ipoModel.CompanyCode,
		"has_description":    ipoModel.Description != nil,
		"has_about":          ipoModel.About != nil,
		"final_status":       ipoModel.Status,
	}).Debug("Completed conversion from Chittorgarh JSON data")

	return ipoModel, nil
}

// ProcessAllAvailableIPOs scrapes all available IPOs with optimized batch processing and error isolation
func (s *EnhancedGMPService) ProcessAllAvailableIPOs() ([]*models.IPO, error) {
	logger := logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"method":    "ProcessAllAvailableIPOs",
	})

	logger.Info("Starting batch processing of all available IPOs")

	// Fetch the complete list of available IPOs
	availableIPOItems, fetchError := s.FetchAvailableIPOList()
	if fetchError != nil {
		logger.WithError(fetchError).Error("Failed to fetch available IPO list")
		return nil, fmt.Errorf("failed to fetch available IPO list: %w", fetchError)
	}

	if len(availableIPOItems) == 0 {
		logger.Info("No IPOs available for processing")
		return []*models.IPO{}, nil
	}

	logger.WithField("total_ipos", len(availableIPOItems)).Info("Processing IPOs with error isolation")

	// Pre-allocate results slice with exact capacity for memory optimization
	scrapingResults := make([]*models.IPO, 0, len(availableIPOItems))

	// Error tracking with memory-conscious approach
	const maxTrackedErrors = 10
	var collectedErrors []error
	var totalErrorCount int

	// Process each IPO sequentially with rate limiting and error isolation
	for itemIndex, ipoItem := range availableIPOItems {
		scrapedIPOData, scrapingError := s.ScrapeDetailedIPOInformation(ipoItem)

		if scrapingError != nil {
			totalErrorCount++

			// Collect sample errors for reporting (memory-limited)
			if len(collectedErrors) < maxTrackedErrors {
				collectedErrors = append(collectedErrors, fmt.Errorf("failed to scrape IPO %d (%s): %w", ipoItem.ID, ipoItem.IPONewsTitle, scrapingError))
			}

			// Include partial data if available (error isolation)
			if scrapedIPOData != nil {
				scrapingResults = append(scrapingResults, scrapedIPOData)
			}
			continue
		}

		// Successfully scraped IPO data
		if scrapedIPOData != nil {
			scrapingResults = append(scrapingResults, scrapedIPOData)
		}

		// Memory optimization: Trigger garbage collection for large batches
		if (itemIndex+1)%50 == 0 && len(availableIPOItems) > 100 {
			// Optional GC trigger to prevent memory buildup during large batch processing
		}
	}

	logger.WithFields(logrus.Fields{
		"successful_ipos": len(scrapingResults),
		"total_errors":    totalErrorCount,
		"success_rate":    float64(len(scrapingResults)) / float64(len(availableIPOItems)) * 100.0,
	}).Info("Completed batch processing of IPOs")

	// Generate comprehensive error summary for partial success scenarios
	if len(scrapingResults) > 0 && totalErrorCount > 0 {
		errorSummary := s.buildBatchProcessingErrorSummary(len(scrapingResults), totalErrorCount, collectedErrors)
		return scrapingResults, fmt.Errorf("%s", errorSummary)
	}

	// Handle complete failure scenarios
	if len(scrapingResults) == 0 && totalErrorCount > 0 {
		if len(collectedErrors) > 0 {
			return nil, fmt.Errorf("failed to scrape any IPOs: %d errors occurred, first error: %w", totalErrorCount, collectedErrors[0])
		}
		return nil, fmt.Errorf("failed to scrape any IPOs: %d errors occurred", totalErrorCount)
	}

	// Complete success
	return scrapingResults, nil
}

// buildBatchProcessingErrorSummary creates a comprehensive error summary for batch processing results
func (s *EnhancedGMPService) buildBatchProcessingErrorSummary(successCount, totalErrorCount int, sampleErrors []error) string {
	var summaryBuilder strings.Builder
	summaryBuilder.WriteString(fmt.Sprintf("batch processing completed with %d successes and %d failures", successCount, totalErrorCount))

	// Include sample errors for debugging (limited to prevent memory issues)
	sampleSize := len(sampleErrors)
	if sampleSize > 3 {
		sampleSize = 3
	}

	for i := 0; i < sampleSize; i++ {
		summaryBuilder.WriteString(fmt.Sprintf("; %s", sampleErrors[i].Error()))
	}

	if totalErrorCount > len(sampleErrors) {
		summaryBuilder.WriteString(fmt.Sprintf("; and %d additional errors", totalErrorCount-len(sampleErrors)))
	}

	return summaryBuilder.String()
}

// ============================================================================
// HTML Data Extraction Methods (Merged from simplified_ipo_scraper.go)
// ============================================================================

// ExtractBasicInformation extracts fundamental IPO details from HTML document
func (s *EnhancedGMPService) ExtractBasicInformation(document *goquery.Document) IPOBasicInformation {
	information := IPOBasicInformation{}

	// Extract company name using multiple fallback selectors for Chittorgarh
	companyNameSelectors := []string{
		"h1.page-title",
		"h1",
		".company-name",
		".ipo-title",
		"title", // fallback to page title
		"h2",
	}
	companyName := s.extractTextUsingSelectors(document, companyNameSelectors...)
	information.CompanyName = s.utilityService.NormalizeTextContent(companyName)

	// Extract company code from name or dedicated field
	information.CompanyCode = s.utilityService.GenerateCompanyCode(information.CompanyName)

	// Extract stock symbol if available with better selectors
	symbolSelectors := []string{
		"td:contains('Symbol') + td",
		"td:contains('Stock Symbol') + td",
		"td:contains('NSE Symbol') + td",
		"td:contains('BSE Symbol') + td",
		"td:contains('Ticker') + td",
		".symbol",
		".stock-symbol",
		"[data-symbol]",
	}
	if stockSymbol := s.extractTextUsingSelectors(document, symbolSelectors...); stockSymbol != "" {
		normalizedSymbol := s.utilityService.NormalizeTextContent(stockSymbol)
		information.StockSymbol = &normalizedSymbol
	}

	// Extract registrar information with better selectors
	registrarSelectors := []string{
		"td:contains('Registrar') + td",
		"td:contains('Registrar to Issue') + td",
		"td:contains('Registrar & Transfer Agent') + td",
		"td:contains('R&T Agent') + td",
		".registrar",
		"[data-registrar]",
	}
	registrarName := s.extractTextUsingSelectors(document, registrarSelectors...)
	information.RegistrarName = s.utilityService.NormalizeTextContent(registrarName)

	return information
}

// ExtractDateInformation extracts all IPO-related dates from HTML document
func (s *EnhancedGMPService) ExtractDateInformation(document *goquery.Document) IPODateInformation {
	information := IPODateInformation{}

	// Extract subscription open date with better selectors
	openDateSelectors := []string{
		"td:contains('Open Date') + td",
		"td:contains('Opening Date') + td",
		"td:contains('Subscription Open') + td",
		"td:contains('Issue Open') + td",
		"td:contains('Opens On') + td",
		".open-date",
		"[data-open-date]",
	}
	if openDateText := s.extractTextUsingSelectors(document, openDateSelectors...); openDateText != "" {
		if parsedDate := s.utilityService.ParseStandardDateFormats(openDateText); parsedDate != nil {
			information.SubscriptionOpenDate = parsedDate
		}
	}

	// Extract subscription close date with better selectors
	closeDateSelectors := []string{
		"td:contains('Close Date') + td",
		"td:contains('Closing Date') + td",
		"td:contains('Subscription Close') + td",
		"td:contains('Issue Close') + td",
		"td:contains('Closes On') + td",
		".close-date",
		"[data-close-date]",
	}
	if closeDateText := s.extractTextUsingSelectors(document, closeDateSelectors...); closeDateText != "" {
		if parsedDate := s.utilityService.ParseStandardDateFormats(closeDateText); parsedDate != nil {
			information.SubscriptionCloseDate = parsedDate
		}
	}

	// Extract allotment result date with better selectors
	resultDateSelectors := []string{
		"td:contains('Allotment Date') + td",
		"td:contains('Result Date') + td",
		"td:contains('Allotment Result') + td",
		"td:contains('Basis of Allotment') + td",
		".result-date",
		"[data-result-date]",
	}
	if resultDateText := s.extractTextUsingSelectors(document, resultDateSelectors...); resultDateText != "" {
		if parsedDate := s.utilityService.ParseStandardDateFormats(resultDateText); parsedDate != nil {
			information.AllotmentResultDate = parsedDate
		}
	}

	// Extract stock listing date with better selectors
	listingDateSelectors := []string{
		"td:contains('Listing Date') + td",
		"td:contains('Expected Listing') + td",
		"td:contains('Tentative Listing') + td",
		"td:contains('Listing On') + td",
		".listing-date",
		"[data-listing-date]",
	}
	if listingDateText := s.extractTextUsingSelectors(document, listingDateSelectors...); listingDateText != "" {
		if parsedDate := s.utilityService.ParseStandardDateFormats(listingDateText); parsedDate != nil {
			information.StockListingDate = parsedDate
		}
	}

	return information
}

// ExtractPricingInformation extracts pricing and investment details from HTML document
func (s *EnhancedGMPService) ExtractPricingInformation(document *goquery.Document) IPOPricingInformation {
	information := IPOPricingInformation{}

	// Extract price band - try multiple selectors for Chittorgarh format
	priceBandSelectors := []string{
		"td:contains('Price Band') + td",
		"td:contains('Issue Price') + td",
		"td:contains('Price Range') + td",
		".price-band",
		"[data-price-band]",
		"td:contains('Band') + td",
	}

	if priceBandText := s.extractTextUsingSelectors(document, priceBandSelectors...); priceBandText != "" {
		// Parse price band like "₹95 - ₹100" or "95-100"
		prices := s.parsePriceBand(priceBandText)
		if len(prices) >= 2 {
			information.PriceBandMinimum = &prices[0]
			information.PriceBandMaximum = &prices[1]
		} else if len(prices) == 1 {
			// Single price
			information.PriceBandMinimum = &prices[0]
			information.PriceBandMaximum = &prices[0]
		}
	}

	// Extract total issue size
	issueSizeSelectors := []string{
		"td:contains('Issue Size') + td",
		"td:contains('Total Issue') + td",
		"td:contains('Size') + td",
		".issue-size",
		"[data-issue-size]",
	}
	if issueSizeText := s.extractTextUsingSelectors(document, issueSizeSelectors...); issueSizeText != "" {
		normalizedSize := s.utilityService.NormalizeTextContent(issueSizeText)
		information.TotalIssueSize = &normalizedSize
	}

	// Extract minimum lot quantity
	minQtySelectors := []string{
		"td:contains('Lot Size') + td",
		"td:contains('Min Qty') + td",
		"td:contains('Minimum Quantity') + td",
		"td:contains('Application Lot') + td",
		".min-qty",
		"[data-min-qty]",
	}
	if minimumQuantityText := s.extractTextUsingSelectors(document, minQtySelectors...); minimumQuantityText != "" {
		if parsedQuantity := s.utilityService.ParseNumericValueAsFloat(minimumQuantityText); parsedQuantity != nil {
			intQuantity := int(*parsedQuantity)
			information.MinimumLotQuantity = &intQuantity
		}
	}

	// Extract minimum investment amount
	minAmountSelectors := []string{
		"td:contains('Min Investment') + td",
		"td:contains('Min Amount') + td",
		"td:contains('Minimum Amount') + td",
		"td:contains('Application Amount') + td",
		".min-amount",
		"[data-min-amount]",
	}
	if minimumAmountText := s.extractTextUsingSelectors(document, minAmountSelectors...); minimumAmountText != "" {
		if parsedAmount := s.utilityService.ParseNumericValueAsFloat(minimumAmountText); parsedAmount != nil {
			intAmount := int(*parsedAmount)
			information.MinimumInvestmentAmount = &intAmount
		}
	}

	return information
}

// ExtractStatusInformation extracts current status and performance metrics from HTML document
func (s *EnhancedGMPService) ExtractStatusInformation(document *goquery.Document) IPOStatusInformation {
	information := IPOStatusInformation{}

	// Extract current IPO status
	currentStatus := s.extractTextUsingSelectors(document, ".status", "[data-status]", "td:contains('Status') + td")
	information.CurrentStatus = s.utilityService.NormalizeTextContent(currentStatus)
	if information.CurrentStatus == "" {
		information.CurrentStatus = "Unknown" // Provide sensible default
	}

	// Extract subscription status if available
	if subscriptionStatusText := s.extractTextUsingSelectors(document, ".subscription-status", "[data-subscription]", "td:contains('Subscription') + td"); subscriptionStatusText != "" {
		normalizedStatus := s.utilityService.NormalizeTextContent(subscriptionStatusText)
		information.SubscriptionStatus = &normalizedStatus
	}

	// Extract listing performance if available
	if listingPerformanceText := s.extractTextUsingSelectors(document, ".listing-gain", "[data-listing-gain]", "td:contains('Listing Gain') + td"); listingPerformanceText != "" {
		normalizedPerformance := s.utilityService.NormalizeTextContent(listingPerformanceText)
		information.ListingPerformance = &normalizedPerformance
	}

	return information
}

// ExtractCompanyDescription extracts company description from HTML document
func (s *EnhancedGMPService) ExtractCompanyDescription(document *goquery.Document) *string {
	logger := logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"method":    "ExtractCompanyDescription",
	})

	logger.Debug("Starting description extraction")

	// CSS selectors for description content - improved specificity and coverage
	descriptionSelectors := []string{
		// Specific class-based selectors
		".company-description",
		".about-company",
		".business-overview",
		".company-profile",
		".ipo-description",
		".company-summary",
		".business-summary",

		// Content-specific containers (more specific)
		".content-area .company-description",
		".main-content .business-overview",
		".ipo-details .company-profile",
		".content-wrapper .company-summary",

		// Table-based selectors (common in Chittorgarh) - expanded coverage
		"td:contains('Company Description') + td",
		"td:contains('Business Overview') + td",
		"td:contains('About Company') + td",
		"td:contains('Company Profile') + td",
		"td:contains('Business Description') + td",
		"td:contains('Company Summary') + td",
		"td:contains('Business Summary') + td",
		"td:contains('Company Business') + td",
		"td:contains('Business Activities') + td",
		"td:contains('Main Business') + td",

		// Paragraph and div selectors (more specific)
		"div.content p:contains('Company Description')",
		"div.content p:contains('Business Overview')",
		"div.content p:contains('About Company')",
		"section.company-info p:contains('About')",
		"div.ipo-content p:contains('Business')",

		// Header-based selectors (content after headers)
		"h3:contains('Company Description') + p",
		"h3:contains('Business Overview') + p",
		"h3:contains('About Company') + p",
		"h4:contains('Company Description') + p",
		"h4:contains('Business Overview') + p",
		"h2:contains('Company Description') + p",

		// Broader selectors for content sections
		"div:contains('Company Description') p",
		"div:contains('Business Overview') p",
		"div:contains('About Company') p",
		"section:contains('Company Description') p",
		"section:contains('Business Overview') p",

		// Fallback broader selectors (more aggressive)
		"p:contains('Company Description')",
		"p:contains('Business Overview')",
		"p:contains('About the Company')",
		"p:contains('Company Business')",
		"p:contains('Business Activities')",
		"div:contains('Company Description')",
		"div:contains('Business Overview')",
		"section:contains('Company Description')",
		"section:contains('Business Overview')",

		// Generic business content selectors
		"p:contains('business')",
		"p:contains('company')",
		"div:contains('business activities')",
		"div:contains('main business')",
	}

	logger.WithField("selectors_count", len(descriptionSelectors)).Debug("Attempting description extraction with multiple selectors")

	extractedText, selectorUsed := s.extractTextFromSelectorsWithLogging(document, descriptionSelectors, "description")
	if extractedText == "" {
		logger.Warn("No description content found with any selector")
		return nil
	}

	logger.WithFields(logrus.Fields{
		"selector_used":    selectorUsed,
		"raw_text_length":  len(extractedText),
		"raw_text_preview": s.truncateForLogging(extractedText, 100),
	}).Debug("Raw description text extracted")

	// Clean and process the text with error handling
	cleanedText := s.utilityService.CleanCompanyText(extractedText)

	// Remove navigation elements first
	cleanedText = s.removeNavigationElements(cleanedText)

	// Then remove standard boilerplate
	cleanedText = s.removeBoilerplateText(cleanedText)
	cleanedText = s.truncateText(cleanedText, 2000)

	// Validate minimum length and quality
	if len(cleanedText) < 10 {
		logger.WithFields(logrus.Fields{
			"cleaned_text_length": len(cleanedText),
			"minimum_required":    10,
		}).Warn("Description text too short after cleaning, rejecting")
		return nil
	}

	logger.WithFields(logrus.Fields{
		"final_text_length":  len(cleanedText),
		"final_text_preview": s.truncateForLogging(cleanedText, 100),
	}).Info("Successfully extracted and cleaned description")

	return &cleanedText
}

// ExtractCompanyAbout extracts detailed company information from HTML document
func (s *EnhancedGMPService) ExtractCompanyAbout(document *goquery.Document) *string {
	logger := logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"method":    "ExtractCompanyAbout",
	})

	logger.Debug("Starting about extraction")

	// CSS selectors for about content - improved specificity and coverage
	aboutSelectors := []string{
		// Specific class-based selectors
		".company-about",
		".company-details",
		".company-profile",
		".ipo-about",
		".company-info",
		".company-information",
		".business-details",
		".business-profile",

		// Content-specific containers (avoid navigation)
		".content-area .company-about",
		".main-content .company-details",
		".ipo-details .company-info",
		".content-wrapper .business-model",
		".content-wrapper .company-information",

		// Table-based selectors (common in Chittorgarh) - expanded coverage
		"td:contains('About') + td",
		"td:contains('Company Details') + td",
		"td:contains('Business Model') + td",
		"td:contains('Company Profile') + td",
		"td:contains('About Company') + td",
		"td:contains('Company Information') + td",
		"td:contains('Business Details') + td",
		"td:contains('Company Background') + td",
		"td:contains('Business Profile') + td",
		"td:contains('Company Overview') + td",
		"td:contains('Business Activities') + td",
		"td:contains('Products and Services') + td",

		// Header-based selectors (content after headers)
		"h3:contains('About') + p",
		"h3:contains('Company Details') + p",
		"h3:contains('Business Model') + p",
		"h4:contains('About') + p",
		"h4:contains('Company Details') + p",
		"h2:contains('About') + p",
		"h2:contains('Company Details') + p",

		// More specific div selectors (avoid navigation)
		"div.content div:contains('About Us')",
		"div.content div:contains('Company Details')",
		"div.main-content div:contains('Business Model')",
		"section.company-info div:contains('About')",
		"div.ipo-content div:contains('Company')",

		// Paragraph selectors with business content
		"p:contains('About Us')",
		"p:contains('Company Details')",
		"p:contains('Business Model')",
		"p:contains('Products and Services')",
		"p:contains('Company Background')",

		// Broader selectors for content sections
		"div:contains('About Us') p",
		"div:contains('Company Details') p",
		"div:contains('Business Model') p",
		"section:contains('About') p",
		"section:contains('Company Details') p",

		// Fallback broader selectors (last resort)
		"section:contains('About')",
		"section:contains('Company Details')",
		"div:contains('About Us')",
		"div:contains('Company Details')",
		"div:contains('Business Model')",
		"div:contains('Company Information')",
		"div:contains('Business Profile')",

		// Generic content selectors for company information
		"div:contains('company information')",
		"div:contains('business activities')",
		"div:contains('products and services')",
		"p:contains('company information')",
		"p:contains('business activities')",
	}

	logger.WithField("selectors_count", len(aboutSelectors)).Debug("Attempting about extraction with multiple selectors")

	extractedText, selectorUsed := s.extractTextFromSelectorsWithLogging(document, aboutSelectors, "about")
	if extractedText == "" {
		logger.Warn("No about content found with any selector")
		return nil
	}

	logger.WithFields(logrus.Fields{
		"selector_used":    selectorUsed,
		"raw_text_length":  len(extractedText),
		"raw_text_preview": s.truncateForLogging(extractedText, 100),
	}).Debug("Raw about text extracted")

	// Clean and process the text
	cleanedText := s.utilityService.CleanCompanyText(extractedText)

	// Remove navigation elements first
	cleanedText = s.removeNavigationElements(cleanedText)

	// Then remove standard boilerplate
	cleanedText = s.removeBoilerplateText(cleanedText)
	cleanedText = s.truncateText(cleanedText, 5000)

	// Validate minimum length and quality
	if len(cleanedText) < 10 {
		logger.WithFields(logrus.Fields{
			"cleaned_text_length": len(cleanedText),
			"minimum_required":    10,
		}).Warn("About text too short after cleaning, rejecting")
		return nil
	}

	logger.WithFields(logrus.Fields{
		"final_text_length":  len(cleanedText),
		"final_text_preview": s.truncateForLogging(cleanedText, 100),
	}).Info("Successfully extracted and cleaned about")

	return &cleanedText
}

// Helper methods for HTML extraction

// extractTextFromSelectorsWithLogging attempts multiple CSS selectors with detailed logging
func (s *EnhancedGMPService) extractTextFromSelectorsWithLogging(document *goquery.Document, selectors []string, fieldType string) (string, string) {
	logger := logrus.WithFields(logrus.Fields{
		"component":  "EnhancedGMPService",
		"field_type": fieldType,
	})

	for i, selector := range selectors {
		logger.WithFields(logrus.Fields{
			"selector_index": i + 1,
			"selector":       selector,
		}).Debug("Trying CSS selector")

		var combinedText strings.Builder
		var elementsFound int

		document.Find(selector).Each(func(j int, s *goquery.Selection) {
			elementsFound++
			text := strings.TrimSpace(s.Text())
			if text != "" {
				if combinedText.Len() > 0 {
					combinedText.WriteString(" ")
				}
				combinedText.WriteString(text)
			}
		})

		currentText := combinedText.String()

		logger.WithFields(logrus.Fields{
			"selector":       selector,
			"elements_found": elementsFound,
			"text_length":    len(currentText),
		}).Debug("Selector results")

		// If we found content with this selector, return it (first match wins)
		if len(currentText) > 0 {
			logger.WithFields(logrus.Fields{
				"successful_selector": selector,
				"text_length":         len(currentText),
			}).Debug("Found content with selector")
			return currentText, selector
		}
	}

	logger.WithField("selectors_tried", len(selectors)).Warn("No content found with any selector")
	return "", ""
}

// extractTextUsingSelectors attempts multiple CSS selectors and returns the first non-empty result
func (s *EnhancedGMPService) extractTextUsingSelectors(document *goquery.Document, selectors ...string) string {
	for _, selector := range selectors {
		if extractedText := strings.TrimSpace(document.Find(selector).First().Text()); extractedText != "" {
			return extractedText
		}
	}
	return ""
}

// removeNavigationElements removes Chittorgarh-specific navigation elements from extracted text
func (s *EnhancedGMPService) removeNavigationElements(text string) string {
	if text == "" {
		return ""
	}

	// Navigation elements specific to Chittorgarh pages - using more aggressive patterns
	navigationPatterns := []string{
		// Remove dashboard and navigation elements (anywhere in text)
		`(?i)\bdashboard\s*ipo\s*list\b`,
		`(?i)\bipo\s*list\s*ipo\s*list\b`,
		`(?i)\bdashboard\b`,
		`(?i)\bipo\s*list\b`,

		// Remove IPO details navigation (anywhere in text)
		`(?i)\bipo\s*details\b`,
		`(?i)\bbookbuilding\s*ipo\b`,
		`(?i)\|\s*₹\d+\s*cr\s*\|`,
		`(?i)₹\d+\s*cr\b`,

		// Remove common navigation links (anywhere in text)
		`(?i)\bmessages\b`,
		`(?i)\bgmp\b`,
		`(?i)\bdocs\b`,
		`(?i)\brhp\b`,
		`(?i)\bdrhp\b`,
		`(?i)\banchor\s*investor\s*link\b`,
		`(?i)\bsubscription\b`,
		`(?i)\breviews\b`,
		`(?i)\ballotment\b`,
		`(?i)\bstock\s*price\b`,
		`(?i)\bfinal\s*prospectus\b`,

		// Remove listing information (anywhere in text)
		`(?i)\blisting\s*at\s*bse\b`,
		`(?i)\blisting\s*at\s*nse\b`,
		`(?i)\blisted\s*at\s*bse\b`,
		`(?i)\blisted\s*at\s*nse\b`,
		`(?i)\bbse\s*nse\b`,
		`(?i)\bnse\s*bse\b`,

		// Remove additional navigation elements found in testing
		`(?i)\bipo\s*news\b`,
		`(?i)\bipo\s*calendar\b`,
		`(?i)\bipo\s*performance\b`,
		`(?i)\bipo\s*analysis\b`,
		`(?i)\bipo\s*rating\b`,
		`(?i)\bipo\s*recommendation\b`,
		`(?i)\bipo\s*apply\b`,
		`(?i)\bapply\s*online\b`,
		`(?i)\bipo\s*forms\b`,
		`(?i)\bipo\s*documents\b`,

		// Remove menu and navigation text
		`(?i)\bmenu\b`,
		`(?i)\bnavigation\b`,
		`(?i)\bhome\b`,
		`(?i)\bback\s*to\s*top\b`,
		`(?i)\bshare\s*this\b`,
		`(?i)\bprint\s*this\b`,
		`(?i)\bemail\s*this\b`,

		// Remove common separators and formatting (anywhere in text)
		`(?i)\s*\|\s*`,
		`(?i)\s*-\s*`,
		`(?i)\s*•\s*`,
		`(?i)\s*→\s*`,
		`(?i)\s*»\s*`,

		// Remove standalone numbers and currency amounts that are navigation artifacts
		`(?i)^\s*\d+\s*$`,
		`(?i)^\s*₹\s*\d+\s*$`,
		`(?i)^\s*rs\.?\s*\d+\s*$`,

		// Remove common call-to-action phrases
		`(?i)\bclick\s*here\b`,
		`(?i)\bread\s*more\b`,
		`(?i)\bmore\s*details\b`,
		`(?i)\bview\s*details\b`,
		`(?i)\bsee\s*more\b`,
		`(?i)\blearn\s*more\b`,
		`(?i)\bfind\s*out\s*more\b`,

		// Remove date and time artifacts
		`(?i)\bupdated\s*on\b`,
		`(?i)\bpublished\s*on\b`,
		`(?i)\blast\s*updated\b`,
		`(?i)\bposted\s*on\b`,
	}

	for _, pattern := range navigationPatterns {
		regex := regexp.MustCompile(pattern)
		text = regex.ReplaceAllString(text, " ")
	}

	// Clean up multiple spaces and trim
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	return text
}

// removeBoilerplateText removes common boilerplate phrases from extracted text
func (s *EnhancedGMPService) removeBoilerplateText(text string) string {
	if text == "" {
		return ""
	}

	// Common boilerplate phrases to remove
	boilerplatePatterns := []string{
		`(?i)^company description:\s*`,
		`(?i)^about us:\s*`,
		`(?i)^about the company:\s*`,
		`(?i)^business overview:\s*`,
		`(?i)^company details:\s*`,
		`(?i)^business model:\s*`,
		`(?i)^about:\s*`,
		`(?i)\s*read more\s*$`,
		`(?i)\s*click here for more\s*$`,
		`(?i)\s*more details\s*$`,
	}

	for _, pattern := range boilerplatePatterns {
		regex := regexp.MustCompile(pattern)
		text = regex.ReplaceAllString(text, "")
	}

	// Ensure proper punctuation at the end
	text = strings.TrimSpace(text)
	if text != "" && !strings.HasSuffix(text, ".") && !strings.HasSuffix(text, "!") && !strings.HasSuffix(text, "?") {
		text += "."
	}

	return text
}

// truncateText truncates text to specified maximum length with ellipsis
func (s *EnhancedGMPService) truncateText(text string, maxLength int) string {
	if text == "" || len(text) <= maxLength {
		return text
	}

	// Find the last space before the max length to avoid cutting words
	truncateAt := maxLength
	for i := maxLength - 1; i >= maxLength-50 && i >= 0; i-- {
		if text[i] == ' ' {
			truncateAt = i
			break
		}
	}

	return text[:truncateAt] + "..."
}

// parsePriceBand extracts price range from text like "₹95 - ₹100" or "95-100"
func (s *EnhancedGMPService) parsePriceBand(priceBandText string) []float64 {
	if priceBandText == "" {
		return nil
	}

	// Normalize the text
	normalizedText := s.utilityService.NormalizeTextContent(priceBandText)

	// Remove currency symbols and extra spaces
	cleanText := strings.ReplaceAll(normalizedText, "₹", "")
	cleanText = strings.ReplaceAll(cleanText, "Rs.", "")
	cleanText = strings.ReplaceAll(cleanText, "Rs ", "")
	cleanText = strings.TrimSpace(cleanText)

	// Try different separators
	separators := []string{" - ", "-", " to ", "to", " ~ ", "~"}

	for _, separator := range separators {
		if strings.Contains(cleanText, separator) {
			parts := strings.Split(cleanText, separator)
			if len(parts) >= 2 {
				var prices []float64
				for i := 0; i < 2 && i < len(parts); i++ {
					if price := s.utilityService.ExtractNumeric(strings.TrimSpace(parts[i])); price > 0 {
						prices = append(prices, price)
					}
				}
				if len(prices) == 2 {
					return prices
				}
			}
		}
	}

	// If no separator found, try to extract single price
	if price := s.utilityService.ExtractNumeric(cleanText); price > 0 {
		return []float64{price}
	}

	return nil
}

// truncateForLogging safely truncates text for logging purposes
func (s *EnhancedGMPService) truncateForLogging(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

// ProcessAllAvailableIPOsWithContext scrapes all IPOs with context support for cancellation and timeout
func (s *EnhancedGMPService) ProcessAllAvailableIPOsWithContext(ctx context.Context) ([]*models.IPO, error) {
	logger := logrus.WithFields(logrus.Fields{
		"component": "EnhancedGMPService",
		"method":    "ProcessAllAvailableIPOsWithContext",
	})

	logger.Info("Starting batch processing of all available IPOs with context")

	// Fetch the complete list of available IPOs
	availableIPOItems, fetchError := s.FetchAvailableIPOList()
	if fetchError != nil {
		logger.WithError(fetchError).Error("Failed to fetch available IPO list")
		return nil, fmt.Errorf("failed to fetch available IPO list: %w", fetchError)
	}

	if len(availableIPOItems) == 0 {
		logger.Info("No IPOs available for processing")
		return []*models.IPO{}, nil
	}

	logger.WithField("total_ipos", len(availableIPOItems)).Info("Processing IPOs with context and error isolation")

	// Pre-allocate results slice with exact capacity for memory optimization
	scrapingResults := make([]*models.IPO, 0, len(availableIPOItems))

	// Error tracking with memory-conscious approach
	const maxTrackedErrors = 10
	var collectedErrors []error
	var totalErrorCount int

	// Process each IPO sequentially with context cancellation support
	for itemIndex, ipoItem := range availableIPOItems {
		// Check for context cancellation before processing each item
		select {
		case <-ctx.Done():
			logger.WithFields(logrus.Fields{
				"processed_count": itemIndex,
				"total_count":     len(availableIPOItems),
			}).Warn("Batch processing cancelled by context")
			return scrapingResults, fmt.Errorf("batch processing cancelled after %d/%d IPOs: %w", itemIndex, len(availableIPOItems), ctx.Err())
		default:
		}

		scrapedIPOData, scrapingError := s.ScrapeDetailedIPOInformation(ipoItem)

		if scrapingError != nil {
			totalErrorCount++

			// Collect sample errors for reporting (memory-limited)
			if len(collectedErrors) < maxTrackedErrors {
				collectedErrors = append(collectedErrors, fmt.Errorf("failed to scrape IPO %d (%s): %w", ipoItem.ID, ipoItem.IPONewsTitle, scrapingError))
			}

			// Include partial data if available (error isolation)
			if scrapedIPOData != nil {
				scrapingResults = append(scrapingResults, scrapedIPOData)
			}
			continue
		}

		// Successfully scraped IPO data
		if scrapedIPOData != nil {
			scrapingResults = append(scrapingResults, scrapedIPOData)
		}

		// Memory optimization: Trigger garbage collection for large batches
		if (itemIndex+1)%50 == 0 && len(availableIPOItems) > 100 {
			// Optional GC trigger to prevent memory buildup during large batch processing
		}
	}

	logger.WithFields(logrus.Fields{
		"successful_ipos": len(scrapingResults),
		"total_errors":    totalErrorCount,
		"success_rate":    float64(len(scrapingResults)) / float64(len(availableIPOItems)) * 100.0,
	}).Info("Completed batch processing of IPOs with context")

	// Generate comprehensive error summary for partial success scenarios
	if len(scrapingResults) > 0 && totalErrorCount > 0 {
		errorSummary := s.buildBatchProcessingErrorSummary(len(scrapingResults), totalErrorCount, collectedErrors)
		return scrapingResults, fmt.Errorf("%s", errorSummary)
	}

	// Handle complete failure scenarios
	if len(scrapingResults) == 0 && totalErrorCount > 0 {
		if len(collectedErrors) > 0 {
			return nil, fmt.Errorf("failed to scrape any IPOs: %d errors occurred, first error: %w", totalErrorCount, collectedErrors[0])
		}
		return nil, fmt.Errorf("failed to scrape any IPOs: %d errors occurred", totalErrorCount)
	}

	// Complete success
	return scrapingResults, nil
}
