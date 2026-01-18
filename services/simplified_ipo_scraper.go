package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/sirupsen/logrus"
)

// IPOScraperConfiguration holds configuration parameters for the IPO scraper service
type IPOScraperConfiguration struct {
	BaseURL            string        // Target website base URL
	HTTPRequestTimeout time.Duration // Maximum time to wait for HTTP responses
	RequestRateLimit   time.Duration // Minimum delay between consecutive requests
	MaxRetryAttempts   int           // Maximum number of retry attempts for failed requests
}

// NewDefaultIPOScraperConfiguration returns production-ready default configuration
func NewDefaultIPOScraperConfiguration() *IPOScraperConfiguration {
	return &IPOScraperConfiguration{
		BaseURL:            "https://www.chittorgarh.com",
		HTTPRequestTimeout: 30 * time.Second,
		RequestRateLimit:   1 * time.Second,
		MaxRetryAttempts:   3,
	}
}

// Note: HTTPRequestRateLimiter is now imported from shared package

// HTMLDataExtractor handles extraction and normalization of IPO data from HTML documents
type HTMLDataExtractor struct {
	// Stateless service for extracting structured data from HTML
}

// ExtractionMetrics tracks success rates and performance of HTML extraction
type ExtractionMetrics struct {
	DescriptionAttempts int
	DescriptionSuccess  int
	AboutAttempts       int
	AboutSuccess        int
	HTMLParseErrors     int
	TextCleaningErrors  int
}

// NewExtractionMetrics creates a new metrics tracker
func NewExtractionMetrics() *ExtractionMetrics {
	return &ExtractionMetrics{}
}

// LogSummary logs a summary of extraction metrics
func (m *ExtractionMetrics) LogSummary() {
	descriptionSuccessRate := 0.0
	if m.DescriptionAttempts > 0 {
		descriptionSuccessRate = float64(m.DescriptionSuccess) / float64(m.DescriptionAttempts) * 100
	}

	aboutSuccessRate := 0.0
	if m.AboutAttempts > 0 {
		aboutSuccessRate = float64(m.AboutSuccess) / float64(m.AboutAttempts) * 100
	}

	logrus.WithFields(logrus.Fields{
		"description_attempts":     m.DescriptionAttempts,
		"description_success":      m.DescriptionSuccess,
		"description_success_rate": fmt.Sprintf("%.1f%%", descriptionSuccessRate),
		"about_attempts":           m.AboutAttempts,
		"about_success":            m.AboutSuccess,
		"about_success_rate":       fmt.Sprintf("%.1f%%", aboutSuccessRate),
		"html_parse_errors":        m.HTMLParseErrors,
		"text_cleaning_errors":     m.TextCleaningErrors,
	}).Info("HTML extraction metrics summary")
}

// NewHTMLDataExtractor creates a new HTML data extraction service
func NewHTMLDataExtractor() *HTMLDataExtractor {
	return &HTMLDataExtractor{}
}

// IPOBasicInformation contains fundamental IPO details
type IPOBasicInformation struct {
	CompanyName   string
	CompanyCode   string
	StockSymbol   *string
	RegistrarName string
}

// IPODateInformation contains all IPO-related dates
type IPODateInformation struct {
	SubscriptionOpenDate  *time.Time
	SubscriptionCloseDate *time.Time
	AllotmentResultDate   *time.Time
	StockListingDate      *time.Time
}

// IPOPricingInformation contains pricing and investment details
type IPOPricingInformation struct {
	PriceBandMinimum        *float64
	PriceBandMaximum        *float64
	TotalIssueSize          *string
	MinimumLotQuantity      *int
	MinimumInvestmentAmount *int
}

// IPOStatusInformation contains current status and performance metrics
type IPOStatusInformation struct {
	CurrentStatus      string
	SubscriptionStatus *string
	ListingPerformance *string
}

// ExtractBasicInformation extracts fundamental IPO details from HTML document
func (extractor *HTMLDataExtractor) ExtractBasicInformation(document *goquery.Document) IPOBasicInformation {
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
	companyName := extractor.extractTextUsingSelectors(document, companyNameSelectors...)
	information.CompanyName = extractor.normalizeTextContent(companyName)

	// Extract company code from name or dedicated field
	information.CompanyCode = extractor.extractCompanyCodeFromText(information.CompanyName)

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
	if stockSymbol := extractor.extractTextUsingSelectors(document, symbolSelectors...); stockSymbol != "" {
		normalizedSymbol := extractor.normalizeTextContent(stockSymbol)
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
	registrarName := extractor.extractTextUsingSelectors(document, registrarSelectors...)
	information.RegistrarName = extractor.normalizeTextContent(registrarName)

	return information
}

// ExtractDateInformation extracts all IPO-related dates from HTML document
func (extractor *HTMLDataExtractor) ExtractDateInformation(document *goquery.Document) IPODateInformation {
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
	if openDateText := extractor.extractTextUsingSelectors(document, openDateSelectors...); openDateText != "" {
		if parsedDate := extractor.parseStandardDateFormats(openDateText); parsedDate != nil {
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
	if closeDateText := extractor.extractTextUsingSelectors(document, closeDateSelectors...); closeDateText != "" {
		if parsedDate := extractor.parseStandardDateFormats(closeDateText); parsedDate != nil {
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
	if resultDateText := extractor.extractTextUsingSelectors(document, resultDateSelectors...); resultDateText != "" {
		if parsedDate := extractor.parseStandardDateFormats(resultDateText); parsedDate != nil {
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
	if listingDateText := extractor.extractTextUsingSelectors(document, listingDateSelectors...); listingDateText != "" {
		if parsedDate := extractor.parseStandardDateFormats(listingDateText); parsedDate != nil {
			information.StockListingDate = parsedDate
		}
	}

	return information
}

// ExtractPricingInformation extracts pricing and investment details from HTML document
func (extractor *HTMLDataExtractor) ExtractPricingInformation(document *goquery.Document) IPOPricingInformation {
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

	if priceBandText := extractor.extractTextUsingSelectors(document, priceBandSelectors...); priceBandText != "" {
		// Parse price band like "₹95 - ₹100" or "95-100"
		prices := extractor.parsePriceBand(priceBandText)
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
	if issueSizeText := extractor.extractTextUsingSelectors(document, issueSizeSelectors...); issueSizeText != "" {
		normalizedSize := extractor.normalizeTextContent(issueSizeText)
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
	if minimumQuantityText := extractor.extractTextUsingSelectors(document, minQtySelectors...); minimumQuantityText != "" {
		if parsedQuantity := extractor.parseNumericValueAsInteger(minimumQuantityText); parsedQuantity != nil {
			information.MinimumLotQuantity = parsedQuantity
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
	if minimumAmountText := extractor.extractTextUsingSelectors(document, minAmountSelectors...); minimumAmountText != "" {
		if parsedAmount := extractor.parseNumericValueAsInteger(minimumAmountText); parsedAmount != nil {
			information.MinimumInvestmentAmount = parsedAmount
		}
	}

	return information
}

// ExtractStatusInformation extracts current status and performance metrics from HTML document
func (extractor *HTMLDataExtractor) ExtractStatusInformation(document *goquery.Document) IPOStatusInformation {
	information := IPOStatusInformation{}

	// Extract current IPO status
	currentStatus := extractor.extractTextUsingSelectors(document, ".status", "[data-status]", "td:contains('Status') + td")
	information.CurrentStatus = extractor.normalizeTextContent(currentStatus)
	if information.CurrentStatus == "" {
		information.CurrentStatus = "Unknown" // Provide sensible default
	}

	// Extract subscription status if available
	if subscriptionStatusText := extractor.extractTextUsingSelectors(document, ".subscription-status", "[data-subscription]", "td:contains('Subscription') + td"); subscriptionStatusText != "" {
		normalizedStatus := extractor.normalizeTextContent(subscriptionStatusText)
		information.SubscriptionStatus = &normalizedStatus
	}

	// Extract listing performance if available
	if listingPerformanceText := extractor.extractTextUsingSelectors(document, ".listing-gain", "[data-listing-gain]", "td:contains('Listing Gain') + td"); listingPerformanceText != "" {
		normalizedPerformance := extractor.normalizeTextContent(listingPerformanceText)
		information.ListingPerformance = &normalizedPerformance
	}

	return information
}

// Private helper methods for HTML data extraction and text processing

// ExtractCompanyDescription extracts company description from HTML document
func (extractor *HTMLDataExtractor) ExtractCompanyDescription(document *goquery.Document) *string {
	logger := logrus.WithFields(logrus.Fields{
		"component": "HTMLDataExtractor",
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

	extractedText, selectorUsed := extractor.extractTextFromSelectorsWithLogging(document, descriptionSelectors, "description")
	if extractedText == "" {
		logger.Warn("No description content found with any selector")
		return nil
	}

	logger.WithFields(logrus.Fields{
		"selector_used":    selectorUsed,
		"raw_text_length":  len(extractedText),
		"raw_text_preview": extractor.truncateForLogging(extractedText, 100),
	}).Debug("Raw description text extracted")

	// Clean and process the text with error handling
	cleanedText, err := extractor.cleanCompanyTextWithErrorHandling(extractedText, "description")
	if err != nil {
		logger.WithError(err).Error("Failed to clean description text")
		return nil
	}

	// Remove navigation elements first
	cleanedText = extractor.removeNavigationElements(cleanedText)

	// Then remove standard boilerplate
	cleanedText = extractor.removeBoilerplateTextWithLogging(cleanedText, "description")
	cleanedText = extractor.truncateText(cleanedText, 2000)

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
		"final_text_preview": extractor.truncateForLogging(cleanedText, 100),
	}).Info("Successfully extracted and cleaned description")

	return &cleanedText
}

// ExtractCompanyAbout extracts detailed company information from HTML document
func (extractor *HTMLDataExtractor) ExtractCompanyAbout(document *goquery.Document) *string {
	logger := logrus.WithFields(logrus.Fields{
		"component": "HTMLDataExtractor",
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

	extractedText, selectorUsed := extractor.extractTextFromSelectorsWithLogging(document, aboutSelectors, "about")
	if extractedText == "" {
		logger.Warn("No about content found with any selector")
		return nil
	}

	logger.WithFields(logrus.Fields{
		"selector_used":    selectorUsed,
		"raw_text_length":  len(extractedText),
		"raw_text_preview": extractor.truncateForLogging(extractedText, 100),
	}).Debug("Raw about text extracted")

	// Clean and process the text with error handling
	cleanedText, err := extractor.cleanCompanyTextWithErrorHandling(extractedText, "about")
	if err != nil {
		logger.WithError(err).Error("Failed to clean about text")
		return nil
	}

	// Remove navigation elements first
	cleanedText = extractor.removeNavigationElements(cleanedText)

	// Then remove standard boilerplate
	cleanedText = extractor.removeBoilerplateTextWithLogging(cleanedText, "about")
	cleanedText = extractor.truncateText(cleanedText, 5000)

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
		"final_text_preview": extractor.truncateForLogging(cleanedText, 100),
	}).Info("Successfully extracted and cleaned about")

	return &cleanedText
}

// extractTextFromSelectors attempts multiple CSS selectors and combines text from all matching elements
func (extractor *HTMLDataExtractor) extractTextFromSelectors(document *goquery.Document, selectors []string) string {
	var combinedText strings.Builder

	for _, selector := range selectors {
		document.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				if combinedText.Len() > 0 {
					combinedText.WriteString(" ")
				}
				combinedText.WriteString(text)
			}
		})

		// If we found content with this selector, return it
		if combinedText.Len() > 0 {
			return combinedText.String()
		}
	}

	return ""
}

// extractTextFromSelectorsWithLogging attempts multiple CSS selectors with detailed logging
func (extractor *HTMLDataExtractor) extractTextFromSelectorsWithLogging(document *goquery.Document, selectors []string, fieldType string) (string, string) {
	logger := logrus.WithFields(logrus.Fields{
		"component":  "HTMLDataExtractor",
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

// truncateForLogging safely truncates text for logging purposes
func (extractor *HTMLDataExtractor) truncateForLogging(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

// cleanCompanyText normalizes and cleans extracted text content
func (extractor *HTMLDataExtractor) cleanCompanyText(text string) string {
	if text == "" {
		return ""
	}

	// Remove HTML tags if any remain
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	text = htmlTagRegex.ReplaceAllString(text, "")

	// Normalize whitespace
	whitespaceRegex := regexp.MustCompile(`\s+`)
	text = whitespaceRegex.ReplaceAllString(text, " ")

	// Remove leading and trailing whitespace
	text = strings.TrimSpace(text)

	// Handle UTF-8 encoding issues by removing non-printable characters
	printableRegex := regexp.MustCompile(`[^\x20-\x7E\p{L}\p{N}\p{P}\p{S}]`)
	text = printableRegex.ReplaceAllString(text, "")

	return text
}

// cleanCompanyTextWithErrorHandling normalizes and cleans extracted text content with comprehensive error handling
func (extractor *HTMLDataExtractor) cleanCompanyTextWithErrorHandling(text string, fieldType string) (string, error) {
	logger := logrus.WithFields(logrus.Fields{
		"component":  "HTMLDataExtractor",
		"field_type": fieldType,
		"method":     "cleanCompanyTextWithErrorHandling",
	})

	if text == "" {
		logger.Debug("Empty text provided for cleaning")
		return "", nil
	}

	originalLength := len(text)
	logger.WithField("original_length", originalLength).Debug("Starting text cleaning")

	// Remove HTML tags if any remain with error handling
	defer func() {
		if r := recover(); r != nil {
			logger.WithField("panic", r).Error("Panic occurred during HTML tag removal")
		}
	}()

	htmlTagRegex, err := regexp.Compile(`<[^>]*>`)
	if err != nil {
		logger.WithError(err).Error("Failed to compile HTML tag regex")
		return "", fmt.Errorf("failed to compile HTML tag regex: %w", err)
	}
	text = htmlTagRegex.ReplaceAllString(text, "")

	// Normalize whitespace with error handling
	whitespaceRegex, err := regexp.Compile(`\s+`)
	if err != nil {
		logger.WithError(err).Error("Failed to compile whitespace regex")
		return "", fmt.Errorf("failed to compile whitespace regex: %w", err)
	}
	text = whitespaceRegex.ReplaceAllString(text, " ")

	// Remove leading and trailing whitespace
	text = strings.TrimSpace(text)

	// Handle UTF-8 encoding issues by removing non-printable characters with error handling
	printableRegex, err := regexp.Compile(`[^\x20-\x7E\p{L}\p{N}\p{P}\p{S}]`)
	if err != nil {
		logger.WithError(err).Error("Failed to compile printable characters regex")
		return "", fmt.Errorf("failed to compile printable characters regex: %w", err)
	}
	text = printableRegex.ReplaceAllString(text, "")

	finalLength := len(text)
	logger.WithFields(logrus.Fields{
		"original_length": originalLength,
		"final_length":    finalLength,
		"reduction":       originalLength - finalLength,
	}).Debug("Text cleaning completed")

	return text, nil
}

// removeNavigationElements removes Chittorgarh-specific navigation elements from extracted text
func (extractor *HTMLDataExtractor) removeNavigationElements(text string) string {
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
func (extractor *HTMLDataExtractor) removeBoilerplateText(text string) string {
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

// removeBoilerplateTextWithLogging removes common boilerplate phrases with detailed logging
func (extractor *HTMLDataExtractor) removeBoilerplateTextWithLogging(text string, fieldType string) string {
	logger := logrus.WithFields(logrus.Fields{
		"component":  "HTMLDataExtractor",
		"field_type": fieldType,
		"method":     "removeBoilerplateTextWithLogging",
	})

	if text == "" {
		logger.Debug("Empty text provided for boilerplate removal")
		return ""
	}

	originalText := text
	originalLength := len(text)

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

	patternsMatched := 0
	for i, pattern := range boilerplatePatterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"pattern_index": i,
				"pattern":       pattern,
				"error":         err,
			}).Warn("Failed to compile boilerplate regex pattern")
			continue
		}

		if regex.MatchString(text) {
			patternsMatched++
			text = regex.ReplaceAllString(text, "")
			logger.WithFields(logrus.Fields{
				"pattern":     pattern,
				"text_before": extractor.truncateForLogging(originalText, 50),
				"text_after":  extractor.truncateForLogging(text, 50),
			}).Debug("Removed boilerplate pattern")
		}
	}

	// Ensure proper punctuation at the end
	text = strings.TrimSpace(text)
	if text != "" && !strings.HasSuffix(text, ".") && !strings.HasSuffix(text, "!") && !strings.HasSuffix(text, "?") {
		text += "."
		logger.Debug("Added punctuation to end of text")
	}

	finalLength := len(text)
	logger.WithFields(logrus.Fields{
		"original_length":    originalLength,
		"final_length":       finalLength,
		"patterns_matched":   patternsMatched,
		"characters_removed": originalLength - finalLength,
	}).Debug("Boilerplate removal completed")

	return text
}

// truncateText truncates text to specified maximum length with ellipsis
func (extractor *HTMLDataExtractor) truncateText(text string, maxLength int) string {
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

// extractTextUsingSelectors attempts multiple CSS selectors and returns the first non-empty result
func (extractor *HTMLDataExtractor) extractTextUsingSelectors(document *goquery.Document, selectors ...string) string {
	for _, selector := range selectors {
		if extractedText := strings.TrimSpace(document.Find(selector).First().Text()); extractedText != "" {
			return extractedText
		}
	}
	return ""
}

// normalizeTextContent cleans and standardizes text content for consistent processing
func (extractor *HTMLDataExtractor) normalizeTextContent(text string) string {
	if text == "" {
		return ""
	}

	// Remove leading and trailing whitespace
	text = strings.TrimSpace(text)

	// Normalize multiple whitespace characters to single spaces
	whitespaceRegex := regexp.MustCompile(`\s+`)
	text = whitespaceRegex.ReplaceAllString(text, " ")

	// Remove common currency symbols and prefixes
	text = strings.ReplaceAll(text, "₹", "")
	text = strings.ReplaceAll(text, "Rs.", "")
	text = strings.ReplaceAll(text, "Rs ", "")

	return strings.TrimSpace(text)
}

// parseStandardDateFormats attempts to parse date strings using common IPO date formats
func (extractor *HTMLDataExtractor) parseStandardDateFormats(dateText string) *time.Time {
	if dateText == "" {
		return nil
	}

	// Normalize the date string before parsing
	normalizedDateText := extractor.normalizeTextContent(dateText)

	// Standard date formats commonly used in IPO documentation
	supportedDateFormats := []string{
		"02-01-2006",           // DD-MM-YYYY
		"2-1-2006",             // D-M-YYYY
		"02/01/2006",           // DD/MM/YYYY
		"2/1/2006",             // D/M/YYYY
		"Jan 02, 2006",         // Mon DD, YYYY
		"January 02, 2006",     // Month DD, YYYY
		"02 Jan 2006",          // DD Mon YYYY
		"02 January 2006",      // DD Month YYYY
		"2006-01-02",           // YYYY-MM-DD (ISO format)
		"Mon, Jan 02, 2006",    // Day, Mon DD, YYYY
		"Monday, Jan 02, 2006", // Weekday, Mon DD, YYYY
	}

	for _, dateFormat := range supportedDateFormats {
		if parsedDate, parseError := time.Parse(dateFormat, normalizedDateText); parseError == nil {
			return &parsedDate
		}
	}

	return nil
}

// parseNumericValueAsFloat extracts and parses floating-point numbers from formatted text
func (extractor *HTMLDataExtractor) parseNumericValueAsFloat(numericText string) *float64 {
	if numericText == "" {
		return nil
	}

	// Normalize the numeric string (removes currency symbols and prefixes)
	normalizedText := extractor.normalizeTextContent(numericText)

	// Remove remaining currency symbols and thousands separators
	currencyRegex := regexp.MustCompile(`[$,]`)
	cleanedText := currencyRegex.ReplaceAllString(normalizedText, "")
	cleanedText = strings.TrimSpace(cleanedText)

	// Validate that the cleaned string contains only valid numeric characters
	validNumericRegex := regexp.MustCompile(`^[\d.]+$`)
	if !validNumericRegex.MatchString(cleanedText) {
		return nil
	}

	// Extract the first valid number from the string
	numberRegex := regexp.MustCompile(`\d+\.?\d*`)
	numberMatch := numberRegex.FindString(cleanedText)
	if numberMatch == "" {
		return nil
	}

	if parsedValue, parseError := strconv.ParseFloat(numberMatch, 64); parseError == nil {
		return &parsedValue
	}

	return nil
}

// parseNumericValueAsInteger extracts and parses integer values from formatted text
func (extractor *HTMLDataExtractor) parseNumericValueAsInteger(numericText string) *int {
	if numericText == "" {
		return nil
	}

	// Normalize the numeric string (removes currency symbols and prefixes)
	normalizedText := extractor.normalizeTextContent(numericText)

	// Remove remaining currency symbols and thousands separators
	currencyRegex := regexp.MustCompile(`[$,]`)
	cleanedText := currencyRegex.ReplaceAllString(normalizedText, "")
	cleanedText = strings.TrimSpace(cleanedText)

	// Validate that the cleaned string contains only digits
	validIntegerRegex := regexp.MustCompile(`^\d+$`)
	if !validIntegerRegex.MatchString(cleanedText) {
		return nil
	}

	// Extract the first valid integer from the string
	integerRegex := regexp.MustCompile(`\d+`)
	integerMatch := integerRegex.FindString(cleanedText)
	if integerMatch == "" {
		return nil
	}

	if parsedValue, parseError := strconv.Atoi(integerMatch); parseError == nil {
		return &parsedValue
	}

	return nil
}

// parsePriceBand extracts price range from text like "₹95 - ₹100" or "95-100"
func (extractor *HTMLDataExtractor) parsePriceBand(priceBandText string) []float64 {
	if priceBandText == "" {
		return nil
	}

	// Normalize the text
	normalizedText := extractor.normalizeTextContent(priceBandText)

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
					if price := extractor.parseNumericValueAsFloat(strings.TrimSpace(parts[i])); price != nil {
						prices = append(prices, *price)
					}
				}
				if len(prices) == 2 {
					return prices
				}
			}
		}
	}

	// If no separator found, try to extract single price
	if price := extractor.parseNumericValueAsFloat(cleanText); price != nil {
		return []float64{*price}
	}

	return nil
}
func (extractor *HTMLDataExtractor) extractCompanyCodeFromText(companyName string) string {
	if companyName == "" {
		return ""
	}

	// First, attempt to extract code from parentheses (e.g., "Company Name (CODE)")
	parenthesesRegex := regexp.MustCompile(`\(([^)]+)\)`)
	parenthesesMatches := parenthesesRegex.FindStringSubmatch(companyName)
	if len(parenthesesMatches) > 1 {
		return strings.TrimSpace(parenthesesMatches[1])
	}

	// If no parentheses found, create abbreviation from company name
	companyWords := strings.Fields(companyName)
	if len(companyWords) > 0 {
		// Use first word if it's short enough to be a code
		if len(companyWords[0]) <= 5 {
			return strings.ToUpper(companyWords[0])
		}

		// Create abbreviation from first letters of each word
		var codeBuilder strings.Builder
		for _, word := range companyWords {
			if len(word) > 0 && codeBuilder.Len() < 5 {
				codeBuilder.WriteByte(word[0])
			}
		}
		return strings.ToUpper(codeBuilder.String())
	}

	return companyName
}

// ChittorgarhIPOScrapingService is the main service for scraping IPO data from Chittorgarh.com
type ChittorgarhIPOScrapingService struct {
	baseURL            string
	httpClient         *http.Client
	requestRateLimiter *shared.HTTPRequestRateLimiter
	htmlDataExtractor  *HTMLDataExtractor
	utilityService     *UtilityService
	configuration      *IPOScraperConfiguration
	extractionMetrics  *ExtractionMetrics
}

// NewChittorgarhIPOScrapingService creates a new IPO scraping service with the specified configuration
func NewChittorgarhIPOScrapingService(config *IPOScraperConfiguration) *ChittorgarhIPOScrapingService {
	if config == nil {
		config = NewDefaultIPOScraperConfiguration()
	} else {
		// Validate configuration and apply defaults for invalid values
		if config.BaseURL == "" {
			config.BaseURL = "https://www.chittorgarh.com"
		}
		if config.HTTPRequestTimeout <= 0 {
			config.HTTPRequestTimeout = 30 * time.Second
		}
		if config.RequestRateLimit <= 0 {
			config.RequestRateLimit = 1 * time.Second
		}
		if config.MaxRetryAttempts < 0 {
			config.MaxRetryAttempts = 3
		}
	}

	// Create optimized HTTP client for web scraping with connection pooling and timeouts
	httpClient := &http.Client{
		Timeout: config.HTTPRequestTimeout,
		Transport: &http.Transport{
			// Connection pool configuration for efficient resource utilization
			MaxIdleConns:        100,              // Maximum idle connections across all hosts
			MaxIdleConnsPerHost: 10,               // Maximum idle connections per host
			IdleConnTimeout:     90 * time.Second, // Duration to keep idle connections alive

			// Enable connection reuse for better performance
			DisableKeepAlives: false,

			// Timeout configurations for robust error handling
			TLSHandshakeTimeout:   10 * time.Second, // Maximum time for TLS handshake
			ResponseHeaderTimeout: 10 * time.Second, // Maximum time to wait for response headers
			ExpectContinueTimeout: 1 * time.Second,  // Maximum time to wait for 100-continue response

			// Enable compression to reduce bandwidth usage
			DisableCompression: false,
		},
	}

	return &ChittorgarhIPOScrapingService{
		baseURL:            config.BaseURL,
		httpClient:         httpClient,
		requestRateLimiter: shared.NewHTTPRequestRateLimiter(config.RequestRateLimit),
		htmlDataExtractor:  NewHTMLDataExtractor(),
		utilityService:     NewUtilityService(),
		configuration:      config,
		extractionMetrics:  NewExtractionMetrics(),
	}
}

// ChittorgarhIPOListItem represents an individual IPO entry from the Chittorgarh API response
type ChittorgarhIPOListItem struct {
	ID                   int    `json:"id"`
	IPONewsTitle         string `json:"ipo_news_title"`
	URLRewriteFolderName string `json:"urlrewrite_folder_name"`
	LogoURL              string `json:"logo_url"`
}

// FetchAvailableIPOList retrieves the complete list of IPOs from Chittorgarh's internal API
func (service *ChittorgarhIPOScrapingService) FetchAvailableIPOList() ([]ChittorgarhIPOListItem, error) {
	apiEndpointURL := "https://webnodejs.chittorgarh.com/cloud/ipo/list-read"

	// Enforce rate limiting before making the request
	service.requestRateLimiter.EnforceRateLimit()

	// Create HTTP request with appropriate headers
	httpRequest, requestError := http.NewRequest("GET", apiEndpointURL, nil)
	if requestError != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", requestError)
	}

	// Set browser-like headers to avoid detection as automated scraper
	service.setBrowserLikeHeaders(httpRequest, "application/json, text/plain, */*")

	// Execute HTTP request with retry logic and exponential backoff
	httpResponse, executionError := service.executeHTTPRequestWithRetry(httpRequest)
	if executionError != nil {
		return nil, fmt.Errorf("failed to fetch IPO list: %w", executionError)
	}
	defer httpResponse.Body.Close()

	// Parse JSON response into structured data
	var apiResponse struct {
		Status          int                      `json:"status"`
		Message         int                      `json:"msg"`
		IPODropDownList []ChittorgarhIPOListItem `json:"ipoDropDownList"`
	}

	if jsonParseError := json.NewDecoder(httpResponse.Body).Decode(&apiResponse); jsonParseError != nil {
		return nil, fmt.Errorf("failed to parse IPO list JSON response: %w", jsonParseError)
	}

	// Validate API response structure and content
	if apiResponse.Status == 0 && len(apiResponse.IPODropDownList) == 0 {
		return nil, fmt.Errorf("API returned empty response with status code: %d", apiResponse.Status)
	}

	return apiResponse.IPODropDownList, nil
}

// ScrapeDetailedIPOInformation extracts comprehensive IPO data from a specific IPO detail page
func (service *ChittorgarhIPOScrapingService) ScrapeDetailedIPOInformation(ipoListItem ChittorgarhIPOListItem) (*models.IPO, error) {
	logger := logrus.WithFields(logrus.Fields{
		"component": "ChittorgarhIPOScrapingService",
		"method":    "ScrapeDetailedIPOInformation",
		"ipo_id":    ipoListItem.ID,
		"ipo_title": ipoListItem.IPONewsTitle,
	})

	logger.Info("Starting detailed IPO information scraping")

	// Enforce rate limiting before making the request
	service.requestRateLimiter.EnforceRateLimit()

	// Construct URL for the IPO detail page - use the correct Chittorgarh URL format
	ipoDetailPageURL := fmt.Sprintf("%s/ipo/%s/%d/", service.baseURL, ipoListItem.URLRewriteFolderName, ipoListItem.ID)
	logger.WithField("url", ipoDetailPageURL).Debug("Constructed IPO detail page URL")

	// Create HTTP request with appropriate headers
	httpRequest, requestError := http.NewRequest("GET", ipoDetailPageURL, nil)
	if requestError != nil {
		logger.WithError(requestError).Error("Failed to create HTTP request")
		return nil, fmt.Errorf("failed to create HTTP request for IPO %d: %w", ipoListItem.ID, requestError)
	}

	// Set browser-like headers for HTML content
	service.setBrowserLikeHeaders(httpRequest, "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")

	// Execute HTTP request with retry logic and exponential backoff
	httpResponse, executionError := service.executeHTTPRequestWithRetry(httpRequest)
	if executionError != nil {
		logger.WithError(executionError).Error("Failed to fetch IPO detail page after retries")
		// Return partial IPO data when detailed scraping fails
		partialIPOData := service.createPartialIPOFromListItem(ipoListItem)
		return partialIPOData, fmt.Errorf("failed to fetch IPO detail page: %w", executionError)
	}
	defer httpResponse.Body.Close()

	logger.WithField("status_code", httpResponse.StatusCode).Debug("Successfully fetched IPO detail page")

	// Read the entire response body as text to extract JSON data
	bodyBytes, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		logger.WithError(readError).Error("Failed to read response body")
		partialIPOData := service.createPartialIPOFromListItem(ipoListItem)
		return partialIPOData, fmt.Errorf("failed to read response body for IPO %d: %w", ipoListItem.ID, readError)
	}

	bodyText := string(bodyBytes)
	logger.WithField("body_length", len(bodyText)).Debug("Read response body")

	// Parse HTML document for both JSON and HTML extraction paths
	htmlDocument, parseError := goquery.NewDocumentFromReader(strings.NewReader(bodyText))
	if parseError != nil {
		logger.WithError(parseError).Error("Failed to parse HTML document")
		service.extractionMetrics.HTMLParseErrors++
		partialIPOData := service.createPartialIPOFromListItem(ipoListItem)
		return partialIPOData, fmt.Errorf("failed to parse HTML document for IPO %d: %w", ipoListItem.ID, parseError)
	}

	logger.Debug("Successfully parsed HTML document")

	// Try to extract JSON data from the JavaScript embedded in the page
	ipoData, jsonError := service.extractIPODataFromJSONWithLogging(bodyText, ipoListItem, htmlDocument)
	if jsonError != nil {
		logger.WithError(jsonError).Warn("JSON extraction failed, falling back to HTML parsing")
		// Fallback to HTML parsing if JSON extraction fails
		// Extract structured data from HTML document (fallback method)
		basicInformation := service.htmlDataExtractor.ExtractBasicInformation(htmlDocument)
		dateInformation := service.htmlDataExtractor.ExtractDateInformation(htmlDocument)
		pricingInformation := service.htmlDataExtractor.ExtractPricingInformation(htmlDocument)
		statusInformation := service.htmlDataExtractor.ExtractStatusInformation(htmlDocument)

		// Create comprehensive IPO model from extracted data
		ipoData = service.buildIPOModelFromExtractedDataWithLogging(
			ipoListItem,
			basicInformation,
			dateInformation,
			pricingInformation,
			statusInformation,
			htmlDocument,
		)
	} else {
		logger.Info("Successfully extracted IPO data from JSON")
		// Even if JSON extraction succeeded, try to get additional fields from HTML
		statusInformation := service.htmlDataExtractor.ExtractStatusInformation(htmlDocument)

		// Enhance JSON data with HTML-extracted fields
		if statusInformation.SubscriptionStatus != nil && ipoData.SubscriptionStatus == nil {
			ipoData.SubscriptionStatus = statusInformation.SubscriptionStatus
			logger.Debug("Enhanced JSON data with subscription status from HTML")
		}
		if statusInformation.ListingPerformance != nil && ipoData.ListingGain == nil {
			ipoData.ListingGain = statusInformation.ListingPerformance
			logger.Debug("Enhanced JSON data with listing performance from HTML")
		}
	}

	logger.WithFields(logrus.Fields{
		"ipo_name":        ipoData.Name,
		"company_code":    ipoData.CompanyCode,
		"has_description": ipoData.Description != nil,
		"has_about":       ipoData.About != nil,
	}).Info("Completed detailed IPO information scraping")

	return ipoData, nil
}

// Private helper methods for HTTP request handling and data processing

// setBrowserLikeHeaders configures HTTP request headers to mimic browser behavior
func (service *ChittorgarhIPOScrapingService) setBrowserLikeHeaders(request *http.Request, acceptHeader string) {
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	request.Header.Set("Accept", acceptHeader)
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	request.Header.Set("Cache-Control", "no-cache")
}

// executeHTTPRequestWithRetry executes HTTP requests with exponential backoff retry logic
func (service *ChittorgarhIPOScrapingService) executeHTTPRequestWithRetry(request *http.Request) (*http.Response, error) {
	var httpResponse *http.Response
	var lastExecutionError error

	for attemptNumber := 0; attemptNumber <= service.configuration.MaxRetryAttempts; attemptNumber++ {
		if attemptNumber > 0 {
			// Calculate exponential backoff duration with jitter to prevent thundering herd
			baseBackoffDuration := time.Duration(1<<uint(attemptNumber-1)) * time.Second
			jitterDuration := time.Duration(float64(baseBackoffDuration) * 0.1 * (0.5 + 0.5*float64(attemptNumber%3)/2))
			totalBackoffDuration := baseBackoffDuration + jitterDuration
			time.Sleep(totalBackoffDuration)
		}

		httpResponse, lastExecutionError = service.httpClient.Do(request)
		if lastExecutionError == nil && httpResponse.StatusCode == http.StatusOK {
			return httpResponse, nil // Successful execution
		}

		// Store detailed error information for potential return
		if lastExecutionError != nil {
			lastExecutionError = fmt.Errorf("attempt %d failed with network error: %w", attemptNumber+1, lastExecutionError)
		} else {
			lastExecutionError = fmt.Errorf("attempt %d failed with HTTP %d: %s", attemptNumber+1, httpResponse.StatusCode, http.StatusText(httpResponse.StatusCode))
			httpResponse.Body.Close() // Clean up response body before retrying
		}
	}

	// All retry attempts exhausted
	totalAttempts := service.configuration.MaxRetryAttempts + 1
	return nil, fmt.Errorf("HTTP request failed after %d attempts: %w", totalAttempts, lastExecutionError)
}

// createPartialIPOFromListItem creates a partial IPO model when detailed scraping fails
func (service *ChittorgarhIPOScrapingService) createPartialIPOFromListItem(listItem ChittorgarhIPOListItem) *models.IPO {
	currentTimestamp := time.Now()

	partialIPO := &models.IPO{
		Name:        listItem.IPONewsTitle,
		CompanyCode: service.htmlDataExtractor.extractCompanyCodeFromText(listItem.IPONewsTitle),
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

// buildIPOModelFromExtractedData constructs a comprehensive IPO model from extracted data
func (service *ChittorgarhIPOScrapingService) buildIPOModelFromExtractedData(
	listItem ChittorgarhIPOListItem,
	basicInfo IPOBasicInformation,
	dateInfo IPODateInformation,
	pricingInfo IPOPricingInformation,
	statusInfo IPOStatusInformation,
	htmlDocument *goquery.Document,
) *models.IPO {
	currentTimestamp := time.Now()

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
		ipoModel.CompanyCode = service.htmlDataExtractor.extractCompanyCodeFromText(listItem.IPONewsTitle)
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
	if htmlDescription := service.htmlDataExtractor.ExtractCompanyDescription(htmlDocument); htmlDescription != nil {
		ipoModel.Description = htmlDescription
		fmt.Printf("HTML extraction: Found description for IPO %d (%s)\n", listItem.ID, ipoModel.Name)
	} else {
		fmt.Printf("No description found for IPO %d (%s) in HTML\n", listItem.ID, ipoModel.Name)
	}

	if htmlAbout := service.htmlDataExtractor.ExtractCompanyAbout(htmlDocument); htmlAbout != nil {
		ipoModel.About = htmlAbout
		fmt.Printf("HTML extraction: Found about for IPO %d (%s)\n", listItem.ID, ipoModel.Name)
	} else {
		fmt.Printf("No about found for IPO %d (%s) in HTML\n", listItem.ID, ipoModel.Name)
	}

	// Calculate status based on dates (override scraped status with dynamic calculation)
	ipoModel.Status = service.utilityService.CalculateIPOStatus(ipoModel.OpenDate, ipoModel.CloseDate, ipoModel.ListingDate)
	ipoModel.SubscriptionStatus = statusInfo.SubscriptionStatus
	ipoModel.ListingGain = statusInfo.ListingPerformance

	// Set logo URL from list item
	if listItem.LogoURL != "" {
		ipoModel.LogoURL = &listItem.LogoURL
	}

	return ipoModel
}

// buildIPOModelFromExtractedDataWithLogging constructs a comprehensive IPO model from extracted data with detailed logging
func (service *ChittorgarhIPOScrapingService) buildIPOModelFromExtractedDataWithLogging(
	listItem ChittorgarhIPOListItem,
	basicInfo IPOBasicInformation,
	dateInfo IPODateInformation,
	pricingInfo IPOPricingInformation,
	statusInfo IPOStatusInformation,
	htmlDocument *goquery.Document,
) *models.IPO {
	logger := logrus.WithFields(logrus.Fields{
		"component": "ChittorgarhIPOScrapingService",
		"method":    "buildIPOModelFromExtractedDataWithLogging",
		"ipo_id":    listItem.ID,
		"ipo_title": listItem.IPONewsTitle,
	})

	logger.Debug("Building IPO model from extracted HTML data")

	currentTimestamp := time.Now()

	ipoModel := &models.IPO{
		StockID:   strconv.Itoa(listItem.ID),
		CreatedAt: currentTimestamp,
		UpdatedAt: currentTimestamp,
	}

	// Set basic information with fallbacks to list item data
	if basicInfo.CompanyName != "" {
		ipoModel.Name = basicInfo.CompanyName
		logger.WithField("source", "html_extraction").Debug("Set company name from HTML extraction")
	} else {
		ipoModel.Name = listItem.IPONewsTitle
		logger.WithField("source", "list_item").Debug("Set company name from list item fallback")
	}

	if basicInfo.CompanyCode != "" {
		ipoModel.CompanyCode = basicInfo.CompanyCode
		logger.WithField("source", "html_extraction").Debug("Set company code from HTML extraction")
	} else {
		ipoModel.CompanyCode = service.htmlDataExtractor.extractCompanyCodeFromText(listItem.IPONewsTitle)
		logger.WithField("source", "generated").Debug("Generated company code from title")
	}

	if basicInfo.RegistrarName != "" {
		ipoModel.Registrar = basicInfo.RegistrarName
		logger.WithField("registrar", basicInfo.RegistrarName).Debug("Set registrar from HTML extraction")
	} else {
		ipoModel.Registrar = "Unknown"
		logger.Debug("Set registrar to Unknown (fallback)")
	}

	// Set optional basic information
	if basicInfo.StockSymbol != nil {
		ipoModel.Symbol = basicInfo.StockSymbol
		logger.WithField("symbol", *basicInfo.StockSymbol).Debug("Set stock symbol from HTML extraction")
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

	// Extract description and about from HTML with metrics tracking
	service.extractionMetrics.DescriptionAttempts++
	if htmlDescription := service.htmlDataExtractor.ExtractCompanyDescription(htmlDocument); htmlDescription != nil {
		ipoModel.Description = htmlDescription
		service.extractionMetrics.DescriptionSuccess++
		logger.WithFields(logrus.Fields{
			"extraction_type": "description",
			"text_length":     len(*htmlDescription),
			"text_preview":    service.truncateForLogging(*htmlDescription, 100),
		}).Info("Successfully extracted description from HTML")
	} else {
		logger.WithField("extraction_type", "description").Warn("No description found in HTML")
	}

	service.extractionMetrics.AboutAttempts++
	if htmlAbout := service.htmlDataExtractor.ExtractCompanyAbout(htmlDocument); htmlAbout != nil {
		ipoModel.About = htmlAbout
		service.extractionMetrics.AboutSuccess++
		logger.WithFields(logrus.Fields{
			"extraction_type": "about",
			"text_length":     len(*htmlAbout),
			"text_preview":    service.truncateForLogging(*htmlAbout, 100),
		}).Info("Successfully extracted about from HTML")
	} else {
		logger.WithField("extraction_type", "about").Warn("No about found in HTML")
	}

	// Calculate status based on dates (override scraped status with dynamic calculation)
	ipoModel.Status = service.utilityService.CalculateIPOStatus(ipoModel.OpenDate, ipoModel.CloseDate, ipoModel.ListingDate)
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

// truncateForLogging safely truncates text for logging purposes (service-level method)
func (service *ChittorgarhIPOScrapingService) truncateForLogging(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

// CleanupResources properly closes the scraping service and releases system resources
func (service *ChittorgarhIPOScrapingService) CleanupResources() error {
	logger := logrus.WithFields(logrus.Fields{
		"component": "ChittorgarhIPOScrapingService",
		"method":    "CleanupResources",
	})

	logger.Info("Starting cleanup of scraping service resources")

	// Log final extraction metrics before cleanup
	service.extractionMetrics.LogSummary()

	if service.httpClient != nil && service.httpClient.Transport != nil {
		if httpTransport, isHTTPTransport := service.httpClient.Transport.(*http.Transport); isHTTPTransport {
			httpTransport.CloseIdleConnections()
			logger.Debug("Closed idle HTTP connections")
		}
	}

	logger.Info("Completed cleanup of scraping service resources")
	return nil
}

// GetExtractionMetrics returns the current extraction metrics
func (service *ChittorgarhIPOScrapingService) GetExtractionMetrics() *ExtractionMetrics {
	return service.extractionMetrics
}

// ResetExtractionMetrics resets the extraction metrics counters
func (service *ChittorgarhIPOScrapingService) ResetExtractionMetrics() {
	service.extractionMetrics = NewExtractionMetrics()
	logrus.WithField("component", "ChittorgarhIPOScrapingService").Info("Reset extraction metrics")
}

// ProcessAllAvailableIPOs scrapes all available IPOs with optimized batch processing and error isolation
func (service *ChittorgarhIPOScrapingService) ProcessAllAvailableIPOs() ([]*models.IPO, error) {
	// Fetch the complete list of available IPOs
	availableIPOItems, fetchError := service.FetchAvailableIPOList()
	if fetchError != nil {
		return nil, fmt.Errorf("failed to fetch available IPO list: %w", fetchError)
	}

	if len(availableIPOItems) == 0 {
		return []*models.IPO{}, nil // Return empty slice for no available IPOs
	}

	// Pre-allocate results slice with exact capacity for memory optimization
	scrapingResults := make([]*models.IPO, 0, len(availableIPOItems))

	// Error tracking with memory-conscious approach
	const maxTrackedErrors = 10
	var collectedErrors []error
	var totalErrorCount int

	// Process each IPO sequentially with rate limiting and error isolation
	for itemIndex, ipoItem := range availableIPOItems {
		scrapedIPOData, scrapingError := service.ScrapeDetailedIPOInformation(ipoItem)

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

	// Generate comprehensive error summary for partial success scenarios
	if len(scrapingResults) > 0 && totalErrorCount > 0 {
		errorSummary := service.buildBatchProcessingErrorSummary(len(scrapingResults), totalErrorCount, collectedErrors)
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

// ProcessAllAvailableIPOsWithContext scrapes all IPOs with context support for cancellation and timeout
func (service *ChittorgarhIPOScrapingService) ProcessAllAvailableIPOsWithContext(ctx context.Context) ([]*models.IPO, error) {
	// Fetch the complete list of available IPOs
	availableIPOItems, fetchError := service.FetchAvailableIPOList()
	if fetchError != nil {
		return nil, fmt.Errorf("failed to fetch available IPO list: %w", fetchError)
	}

	if len(availableIPOItems) == 0 {
		return []*models.IPO{}, nil // Return empty slice for no available IPOs
	}

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
			return scrapingResults, fmt.Errorf("batch processing cancelled after %d/%d IPOs: %w", itemIndex, len(availableIPOItems), ctx.Err())
		default:
		}

		scrapedIPOData, scrapingError := service.ScrapeDetailedIPOInformation(ipoItem)

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

	// Generate comprehensive error summary for partial success scenarios
	if len(scrapingResults) > 0 && totalErrorCount > 0 {
		errorSummary := service.buildBatchProcessingErrorSummary(len(scrapingResults), totalErrorCount, collectedErrors)
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
func (service *ChittorgarhIPOScrapingService) buildBatchProcessingErrorSummary(successCount, totalErrorCount int, sampleErrors []error) string {
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

// ChittorgarhIPOData represents the IPO data structure from Chittorgarh's JSON
type ChittorgarhIPOData struct {
	ID                   int     `json:"id"`
	CompanyName          string  `json:"company_name"`
	IssueOpenDate        string  `json:"issue_open_date"`
	IssueCloseDate       string  `json:"issue_close_date"`
	IssuePriceLower      float64 `json:"issue_price_lower"`
	IssuePriceUpper      float64 `json:"issue_price_upper"`
	NSESymbol            string  `json:"nse_symbol"`
	RegistrarName        string  `json:"registrar_name"`
	TimetableListingDate string  `json:"timetable_listing_dt"`
	TimetableResultDate  string  `json:"timetable_boa_dt"`
	MarketLotSize        int     `json:"market_lot_size"`
	MinimumOrderQuantity int     `json:"minimum_order_quantity"`
	IssueSizeInAmt       string  `json:"issue_size_in_amt"`
	URLRewriteFolderName string  `json:"urlrewrite_folder_name"`
	Description          string  `json:"description"`
	About                string  `json:"about"`
}

// extractIPODataFromJSON extracts IPO data from JSON embedded in the page
func (service *ChittorgarhIPOScrapingService) extractIPODataFromJSON(bodyText string, ipoListItem ChittorgarhIPOListItem, htmlDocument *goquery.Document) (*models.IPO, error) {
	// Look for the JSON data pattern in the JavaScript
	// The data is embedded in a complex nested structure, we need to find the complete JSON object

	// First, find the start of the ipoData JSON
	startPattern := `\\"ipoData\\":\s*\[`
	startRegex := regexp.MustCompile(startPattern)
	startMatch := startRegex.FindStringIndex(bodyText)

	if startMatch == nil {
		return nil, fmt.Errorf("could not find ipoData start pattern in page content")
	}

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
	// Replace \" with " and \\ with \
	unescapedJSON := strings.ReplaceAll(jsonStr, `\"`, `"`)
	unescapedJSON = strings.ReplaceAll(unescapedJSON, `\\`, `\`)

	// Parse the JSON data
	var ipoData ChittorgarhIPOData
	if err := json.Unmarshal([]byte(unescapedJSON), &ipoData); err != nil {
		return nil, fmt.Errorf("failed to parse IPO JSON data: %w", err)
	}

	// Convert to our IPO model
	return service.convertChittorgarhDataToIPO(ipoData, ipoListItem, htmlDocument)
}

// extractIPODataFromJSONWithLogging extracts IPO data from JSON embedded in the page with comprehensive logging
func (service *ChittorgarhIPOScrapingService) extractIPODataFromJSONWithLogging(bodyText string, ipoListItem ChittorgarhIPOListItem, htmlDocument *goquery.Document) (*models.IPO, error) {
	logger := logrus.WithFields(logrus.Fields{
		"component": "ChittorgarhIPOScrapingService",
		"method":    "extractIPODataFromJSONWithLogging",
		"ipo_id":    ipoListItem.ID,
		"ipo_title": ipoListItem.IPONewsTitle,
	})

	logger.Debug("Starting JSON extraction from page content")

	// Look for the JSON data pattern in the JavaScript
	// The data is embedded in a complex nested structure, we need to find the complete JSON object

	// First, find the start of the ipoData JSON
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

	logger.WithFields(logrus.Fields{
		"start_position": startMatch[0],
		"pattern":        startPattern,
	}).Debug("Found ipoData start pattern")

	// Find the opening brace after the array start
	searchStart := startMatch[1]
	openBraceIndex := strings.Index(bodyText[searchStart:], "{")
	if openBraceIndex == -1 {
		logger.Warn("Could not find opening brace for ipoData JSON")
		return nil, fmt.Errorf("could not find opening brace for ipoData JSON")
	}

	jsonStart := searchStart + openBraceIndex
	logger.WithField("json_start_position", jsonStart).Debug("Found JSON opening brace")

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
		logger.Warn("Could not find closing brace for ipoData JSON")
		return nil, fmt.Errorf("could not find closing brace for ipoData JSON")
	}

	jsonStr := bodyText[jsonStart:jsonEnd]
	logger.WithFields(logrus.Fields{
		"json_length":  len(jsonStr),
		"json_preview": service.truncateForLogging(jsonStr, 200),
	}).Debug("Extracted JSON string")

	// Unescape the JSON string (it's escaped for JavaScript)
	// Replace \" with " and \\ with \
	unescapedJSON := strings.ReplaceAll(jsonStr, `\"`, `"`)
	unescapedJSON = strings.ReplaceAll(unescapedJSON, `\\`, `\`)

	logger.WithField("unescaped_length", len(unescapedJSON)).Debug("Unescaped JSON string")

	// Parse the JSON data
	var ipoData ChittorgarhIPOData
	if err := json.Unmarshal([]byte(unescapedJSON), &ipoData); err != nil {
		logger.WithError(err).Error("Failed to parse IPO JSON data")
		return nil, fmt.Errorf("failed to parse IPO JSON data: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"company_name":    ipoData.CompanyName,
		"has_description": ipoData.Description != "",
		"has_about":       ipoData.About != "",
	}).Debug("Successfully parsed JSON data")

	// Convert to our IPO model
	return service.convertChittorgarhDataToIPOWithLogging(ipoData, ipoListItem, htmlDocument)
}

// convertChittorgarhDataToIPO converts Chittorgarh JSON data to our IPO model
func (service *ChittorgarhIPOScrapingService) convertChittorgarhDataToIPO(data ChittorgarhIPOData, listItem ChittorgarhIPOListItem, htmlDocument *goquery.Document) (*models.IPO, error) {
	currentTimestamp := time.Now()

	ipo := &models.IPO{
		StockID:   strconv.Itoa(data.ID),
		Name:      data.CompanyName,
		CreatedAt: currentTimestamp,
		UpdatedAt: currentTimestamp,
	}

	// Set company code from name
	ipo.CompanyCode = service.htmlDataExtractor.extractCompanyCodeFromText(data.CompanyName)

	// Set registrar
	if data.RegistrarName != "" {
		ipo.Registrar = data.RegistrarName
	} else {
		ipo.Registrar = "Unknown"
	}

	// Set symbol
	if data.NSESymbol != "" {
		ipo.Symbol = &data.NSESymbol
	}

	// Set price band
	if data.IssuePriceLower > 0 {
		ipo.PriceBandLow = &data.IssuePriceLower
	}
	if data.IssuePriceUpper > 0 {
		ipo.PriceBandHigh = &data.IssuePriceUpper
	}

	// Set dates
	if openDate := service.parseChittorgarhDate(data.IssueOpenDate); openDate != nil {
		ipo.OpenDate = openDate
	}
	if closeDate := service.parseChittorgarhDate(data.IssueCloseDate); closeDate != nil {
		ipo.CloseDate = closeDate
	}
	if listingDate := service.parseChittorgarhDate(data.TimetableListingDate); listingDate != nil {
		ipo.ListingDate = listingDate
	}
	if resultDate := service.parseChittorgarhDate(data.TimetableResultDate); resultDate != nil {
		ipo.ResultDate = resultDate
	}

	// Set lot size and minimum amount
	if data.MarketLotSize > 0 {
		ipo.MinQty = &data.MarketLotSize
	}
	if data.MinimumOrderQuantity > 0 && ipo.MinQty == nil {
		ipo.MinQty = &data.MinimumOrderQuantity
	}

	// Calculate minimum amount
	if ipo.MinQty != nil && ipo.PriceBandHigh != nil {
		minAmount := int(float64(*ipo.MinQty) * (*ipo.PriceBandHigh))
		ipo.MinAmount = &minAmount
	}

	// Set issue size
	if data.IssueSizeInAmt != "" {
		ipo.IssueSize = &data.IssueSizeInAmt
	}

	// Set description and about if available from JSON, otherwise try HTML fallback
	if data.Description != "" {
		ipo.Description = &data.Description
		fmt.Printf("JSON extraction: Found description for IPO %d (%s)\n", data.ID, data.CompanyName)
	} else {
		// HTML fallback for description
		if htmlDescription := service.htmlDataExtractor.ExtractCompanyDescription(htmlDocument); htmlDescription != nil {
			ipo.Description = htmlDescription
			fmt.Printf("HTML fallback: Extracted description for IPO %d (%s)\n", data.ID, data.CompanyName)
		} else {
			fmt.Printf("No description found for IPO %d (%s) in JSON or HTML\n", data.ID, data.CompanyName)
		}
	}

	if data.About != "" {
		ipo.About = &data.About
		fmt.Printf("JSON extraction: Found about for IPO %d (%s)\n", data.ID, data.CompanyName)
	} else {
		// HTML fallback for about
		if htmlAbout := service.htmlDataExtractor.ExtractCompanyAbout(htmlDocument); htmlAbout != nil {
			ipo.About = htmlAbout
			fmt.Printf("HTML fallback: Extracted about for IPO %d (%s)\n", data.ID, data.CompanyName)
		} else {
			fmt.Printf("No about found for IPO %d (%s) in JSON or HTML\n", data.ID, data.CompanyName)
		}
	}

	// Generate slug from company name
	if ipo.Name != "" {
		slug := service.generateSlugFromName(ipo.Name)
		ipo.Slug = &slug
	}

	// Set logo URL - prefer the one from API list, fallback to generated
	if listItem.LogoURL != "" {
		// Use the logo URL from the API list (most accurate)
		fullLogoURL := fmt.Sprintf("https://www.chittorgarh.net/images/ipo/%s", listItem.LogoURL)
		ipo.LogoURL = &fullLogoURL
	} else if data.URLRewriteFolderName != "" {
		// Fallback: generate logo URL using the standard Chittorgarh pattern
		// Remove -ipo suffix from URLRewriteFolderName for logo URL generation
		logoFolderName := data.URLRewriteFolderName
		if strings.HasSuffix(logoFolderName, "-ipo") {
			logoFolderName = strings.TrimSuffix(logoFolderName, "-ipo")
		}

		// Try both underscore and hyphen patterns since Chittorgarh is inconsistent
		// We'll use hyphens as the primary pattern since it matches the URLRewriteFolderName format
		logoURL := fmt.Sprintf("https://www.chittorgarh.net/images/ipo/%s-logo.png", logoFolderName)
		ipo.LogoURL = &logoURL
	}

	// Calculate status based on dates
	ipo.Status = service.utilityService.CalculateIPOStatus(ipo.OpenDate, ipo.CloseDate, ipo.ListingDate)

	return ipo, nil
}

// convertChittorgarhDataToIPOWithLogging converts Chittorgarh JSON data to our IPO model with comprehensive logging
func (service *ChittorgarhIPOScrapingService) convertChittorgarhDataToIPOWithLogging(data ChittorgarhIPOData, listItem ChittorgarhIPOListItem, htmlDocument *goquery.Document) (*models.IPO, error) {
	logger := logrus.WithFields(logrus.Fields{
		"component":    "ChittorgarhIPOScrapingService",
		"method":       "convertChittorgarhDataToIPOWithLogging",
		"ipo_id":       data.ID,
		"company_name": data.CompanyName,
	})

	logger.Debug("Converting Chittorgarh JSON data to IPO model")

	currentTimestamp := time.Now()

	ipo := &models.IPO{
		StockID:   strconv.Itoa(data.ID),
		Name:      data.CompanyName,
		CreatedAt: currentTimestamp,
		UpdatedAt: currentTimestamp,
	}

	// Set company code from name
	ipo.CompanyCode = service.htmlDataExtractor.extractCompanyCodeFromText(data.CompanyName)
	logger.WithField("company_code", ipo.CompanyCode).Debug("Generated company code")

	// Set registrar
	if data.RegistrarName != "" {
		ipo.Registrar = data.RegistrarName
		logger.WithField("registrar", data.RegistrarName).Debug("Set registrar from JSON")
	} else {
		ipo.Registrar = "Unknown"
		logger.Debug("Set registrar to Unknown (fallback)")
	}

	// Set symbol
	if data.NSESymbol != "" {
		ipo.Symbol = &data.NSESymbol
		logger.WithField("symbol", data.NSESymbol).Debug("Set symbol from JSON")
	}

	// Set price band
	if data.IssuePriceLower > 0 {
		ipo.PriceBandLow = &data.IssuePriceLower
	}
	if data.IssuePriceUpper > 0 {
		ipo.PriceBandHigh = &data.IssuePriceUpper
	}
	if ipo.PriceBandLow != nil && ipo.PriceBandHigh != nil {
		logger.WithFields(logrus.Fields{
			"price_band_low":  *ipo.PriceBandLow,
			"price_band_high": *ipo.PriceBandHigh,
		}).Debug("Set price band from JSON")
	}

	// Set dates
	if openDate := service.parseChittorgarhDate(data.IssueOpenDate); openDate != nil {
		ipo.OpenDate = openDate
	}
	if closeDate := service.parseChittorgarhDate(data.IssueCloseDate); closeDate != nil {
		ipo.CloseDate = closeDate
	}
	if listingDate := service.parseChittorgarhDate(data.TimetableListingDate); listingDate != nil {
		ipo.ListingDate = listingDate
	}
	if resultDate := service.parseChittorgarhDate(data.TimetableResultDate); resultDate != nil {
		ipo.ResultDate = resultDate
	}

	// Set lot size and minimum amount
	if data.MarketLotSize > 0 {
		ipo.MinQty = &data.MarketLotSize
	}
	if data.MinimumOrderQuantity > 0 && ipo.MinQty == nil {
		ipo.MinQty = &data.MinimumOrderQuantity
	}

	// Calculate minimum amount
	if ipo.MinQty != nil && ipo.PriceBandHigh != nil {
		minAmount := int(float64(*ipo.MinQty) * (*ipo.PriceBandHigh))
		ipo.MinAmount = &minAmount
		logger.WithFields(logrus.Fields{
			"min_qty":    *ipo.MinQty,
			"price_high": *ipo.PriceBandHigh,
			"min_amount": minAmount,
		}).Debug("Calculated minimum amount")
	}

	// Set issue size
	if data.IssueSizeInAmt != "" {
		ipo.IssueSize = &data.IssueSizeInAmt
		logger.WithField("issue_size", data.IssueSizeInAmt).Debug("Set issue size from JSON")
	}

	// Set description and about if available from JSON, otherwise try HTML fallback with metrics tracking
	service.extractionMetrics.DescriptionAttempts++
	if data.Description != "" {
		ipo.Description = &data.Description
		service.extractionMetrics.DescriptionSuccess++
		logger.WithFields(logrus.Fields{
			"source":       "json",
			"text_length":  len(data.Description),
			"text_preview": service.truncateForLogging(data.Description, 100),
		}).Info("Found description in JSON data")
	} else {
		// HTML fallback for description
		logger.Debug("Description not found in JSON, attempting HTML fallback")
		if htmlDescription := service.htmlDataExtractor.ExtractCompanyDescription(htmlDocument); htmlDescription != nil {
			ipo.Description = htmlDescription
			service.extractionMetrics.DescriptionSuccess++
			logger.WithFields(logrus.Fields{
				"source":       "html_fallback",
				"text_length":  len(*htmlDescription),
				"text_preview": service.truncateForLogging(*htmlDescription, 100),
			}).Info("Successfully extracted description from HTML fallback")
		} else {
			logger.Warn("No description found in JSON or HTML")
		}
	}

	service.extractionMetrics.AboutAttempts++
	if data.About != "" {
		ipo.About = &data.About
		service.extractionMetrics.AboutSuccess++
		logger.WithFields(logrus.Fields{
			"source":       "json",
			"text_length":  len(data.About),
			"text_preview": service.truncateForLogging(data.About, 100),
		}).Info("Found about in JSON data")
	} else {
		// HTML fallback for about
		logger.Debug("About not found in JSON, attempting HTML fallback")
		if htmlAbout := service.htmlDataExtractor.ExtractCompanyAbout(htmlDocument); htmlAbout != nil {
			ipo.About = htmlAbout
			service.extractionMetrics.AboutSuccess++
			logger.WithFields(logrus.Fields{
				"source":       "html_fallback",
				"text_length":  len(*htmlAbout),
				"text_preview": service.truncateForLogging(*htmlAbout, 100),
			}).Info("Successfully extracted about from HTML fallback")
		} else {
			logger.Warn("No about found in JSON or HTML")
		}
	}

	// Generate slug from company name
	if ipo.Name != "" {
		slug := service.generateSlugFromName(ipo.Name)
		ipo.Slug = &slug
		logger.WithField("slug", slug).Debug("Generated slug from company name")
	}

	// Set logo URL - prefer the one from API list, fallback to generated
	if listItem.LogoURL != "" {
		// Use the logo URL from the API list (most accurate)
		fullLogoURL := fmt.Sprintf("https://www.chittorgarh.net/images/ipo/%s", listItem.LogoURL)
		ipo.LogoURL = &fullLogoURL
		logger.WithField("logo_url", fullLogoURL).Debug("Set logo URL from API list")
	} else if data.URLRewriteFolderName != "" {
		// Fallback: generate logo URL using the standard Chittorgarh pattern
		// Remove -ipo suffix from URLRewriteFolderName for logo URL generation
		logoFolderName := data.URLRewriteFolderName
		if strings.HasSuffix(logoFolderName, "-ipo") {
			logoFolderName = strings.TrimSuffix(logoFolderName, "-ipo")
		}

		// Try both underscore and hyphen patterns since Chittorgarh is inconsistent
		// We'll use hyphens as the primary pattern since it matches the URLRewriteFolderName format
		logoURL := fmt.Sprintf("https://www.chittorgarh.net/images/ipo/%s-logo.png", logoFolderName)
		ipo.LogoURL = &logoURL
		logger.WithField("logo_url", logoURL).Debug("Generated logo URL from folder name")
	}

	// Calculate status based on dates
	ipo.Status = service.utilityService.CalculateIPOStatus(ipo.OpenDate, ipo.CloseDate, ipo.ListingDate)
	logger.WithField("calculated_status", ipo.Status).Debug("Calculated IPO status")

	logger.WithFields(logrus.Fields{
		"final_name":         ipo.Name,
		"final_company_code": ipo.CompanyCode,
		"has_description":    ipo.Description != nil,
		"has_about":          ipo.About != nil,
		"final_status":       ipo.Status,
	}).Debug("Completed conversion from JSON to IPO model")

	return ipo, nil
}

// parseChittorgarhDate parses dates in Chittorgarh format
func (service *ChittorgarhIPOScrapingService) parseChittorgarhDate(dateStr string) *time.Time {
	if dateStr == "" {
		return nil
	}

	// Common Chittorgarh date formats
	formats := []string{
		"January 2, 2006",
		"Jan 2, 2006",
		"2 January 2006",
		"2 Jan 2006",
		"Monday, January 2, 2006",
		"Mon, Jan 2, 2006",
		"2006-01-02",
		"02-01-2006",
		"02/01/2006",
	}

	for _, format := range formats {
		if parsedDate, err := time.Parse(format, dateStr); err == nil {
			return &parsedDate
		}
	}

	return nil
}

// generateSlugFromName creates a URL-friendly slug from company name
func (service *ChittorgarhIPOScrapingService) generateSlugFromName(name string) string {
	if name == "" {
		return ""
	}

	// Convert to lowercase
	slug := strings.ToLower(name)

	// Replace spaces and special characters with hyphens
	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-")

	// Remove leading and trailing hyphens
	slug = strings.Trim(slug, "-")

	// Remove multiple consecutive hyphens
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")

	return slug
}
