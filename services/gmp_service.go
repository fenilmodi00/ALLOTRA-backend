package services

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
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
	ipoScrapingService *ChittorgarhIPOScrapingService
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
		ipoScrapingService: NewChittorgarhIPOScrapingService(nil),
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
