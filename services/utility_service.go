package services

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
)

// UtilityService provides text processing, normalization, and table parsing utilities
type UtilityService struct {
	serviceMetrics *shared.ServiceMetrics
}

// NewUtilityService creates a new utility service instance
func NewUtilityService() *UtilityService {
	return &UtilityService{
		serviceMetrics: shared.NewServiceMetrics("Utility_Service"),
	}
}

// NormalizeIPOName normalizes an IPO name for matching
// Removes common suffixes, special characters, converts to lowercase, and trims whitespace
func (s *UtilityService) NormalizeIPOName(name string) string {
	// Convert to lowercase
	normalized := strings.ToLower(name)

	// Remove common legal suffixes
	suffixes := []string{" ltd.", " ltd", " limited", " pvt.", " pvt", " private", " ipo"}
	for _, suffix := range suffixes {
		normalized = strings.TrimSuffix(normalized, suffix)
	}

	// Remove special characters and punctuation
	reg := regexp.MustCompile(`[^a-z0-9\s]`)
	normalized = reg.ReplaceAllString(normalized, "")

	// Trim leading and trailing whitespace
	normalized = strings.TrimSpace(normalized)

	return normalized
}

// NormalizeTextContent cleans and standardizes text content for consistent processing
// Uses enhanced scraper patterns for comprehensive text normalization
func (s *UtilityService) NormalizeTextContent(text string) string {
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

// CleanCompanyText normalizes and cleans extracted text content using enhanced scraper patterns
func (s *UtilityService) CleanCompanyText(text string) string {
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

// GenerateCompanyCode generates a company code from an IPO name
// Returns a URL-friendly slug (lowercase, hyphens instead of spaces, no special chars)
func (s *UtilityService) GenerateCompanyCode(name string) string {
	// First normalize the name
	normalized := s.NormalizeIPOName(name)

	// Replace spaces with hyphens
	code := strings.ReplaceAll(normalized, " ", "-")

	// Remove any consecutive hyphens
	reg := regexp.MustCompile(`-+`)
	code = reg.ReplaceAllString(code, "-")

	// Trim hyphens from start and end
	code = strings.Trim(code, "-")

	return code
}

// ExtractCompanyCodeFromText extracts company code from company name using enhanced scraper patterns
// First attempts to extract code from parentheses, then creates abbreviation from company name
func (s *UtilityService) ExtractCompanyCodeFromText(companyName string) string {
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

// ParseDate parses dates with multiple format support
// Supports formats: "Jan 2, 2006", "January 2, 2006", "02-Jan-06", "2006-01-02"
func (s *UtilityService) ParseDate(dateStr string) *time.Time {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return nil
	}

	// Check if it's a "not available" placeholder
	if s.IsNotAvailable(dateStr) {
		return nil
	}

	formats := []string{
		"Mon, Jan 2, 2006",        // "Wed, Dec 10, 2025" - with day of week
		"Monday, January 2, 2006", // Full day and month names
		"Jan 2, 2006",
		"January 2, 2006",
		"02-Jan-06",
		"2-Jan-06",
		"2006-01-02",
		"02/01/2006",
		"2/1/2006",
	}

	for _, format := range formats {
		t, err := time.Parse(format, dateStr)
		if err == nil {
			return &t
		}
	}

	return nil
}

// ParseStandardDateFormats attempts to parse date strings using comprehensive IPO date formats
// Uses enhanced scraper patterns for maximum compatibility
func (s *UtilityService) ParseStandardDateFormats(dateText string) *time.Time {
	if dateText == "" {
		return nil
	}

	// Normalize the date string before parsing
	normalizedDateText := s.NormalizeTextContent(dateText)

	// Standard date formats commonly used in IPO documentation (from enhanced scraper)
	supportedDateFormats := []string{
		"02-01-2006",              // DD-MM-YYYY
		"2-1-2006",                // D-M-YYYY
		"02/01/2006",              // DD/MM/YYYY
		"2/1/2006",                // D/M/YYYY
		"Jan 02, 2006",            // Mon DD, YYYY
		"January 02, 2006",        // Month DD, YYYY
		"02 Jan 2006",             // DD Mon YYYY
		"02 January 2006",         // DD Month YYYY
		"2006-01-02",              // YYYY-MM-DD (ISO format)
		"Mon, Jan 02, 2006",       // Day, Mon DD, YYYY
		"Monday, Jan 02, 2006",    // Weekday, Mon DD, YYYY
		"Mon, Jan 2, 2006",        // Day, Mon D, YYYY (single digit day)
		"Monday, January 2, 2006", // Full day and month names
		"02-Jan-06",               // DD-Mon-YY
		"2-Jan-06",                // D-Mon-YY
	}

	for _, dateFormat := range supportedDateFormats {
		if parsedDate, parseError := time.Parse(dateFormat, normalizedDateText); parseError == nil {
			return &parsedDate
		}
	}

	return nil
}

// ExtractNumeric extracts numeric value from text with currency symbols and formatting
// Handles currency symbols (₹, $, etc.), commas, and other formatting characters
func (s *UtilityService) ExtractNumeric(text string) float64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	// Remove currency symbols
	reg := regexp.MustCompile(`[₹$€£¥]`)
	text = reg.ReplaceAllString(text, "")

	// Remove commas
	text = strings.ReplaceAll(text, ",", "")

	// Remove spaces
	text = strings.ReplaceAll(text, " ", "")

	// Extract first numeric value (including decimals)
	reg = regexp.MustCompile(`-?\d+\.?\d*`)
	match := reg.FindString(text)
	if match == "" {
		return 0
	}

	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0
	}

	return value
}

// ParseNumericValueAsFloat extracts and parses floating-point numbers from formatted text
// Uses enhanced scraper patterns for comprehensive numeric processing
func (s *UtilityService) ParseNumericValueAsFloat(numericText string) *float64 {
	if numericText == "" {
		return nil
	}

	// Normalize the numeric string (removes currency symbols and prefixes)
	normalizedText := s.NormalizeTextContent(numericText)

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

	// Parse the extracted number
	if parsedValue, parseError := strconv.ParseFloat(numberMatch, 64); parseError == nil {
		return &parsedValue
	}

	return nil
}

// ParsePriceBand parses price band text like "₹95 - ₹100" or "95-100" into separate values
// Uses enhanced scraper patterns for comprehensive price band parsing
func (s *UtilityService) ParsePriceBand(priceBandText string) []float64 {
	if priceBandText == "" {
		return nil
	}

	// Normalize the text first
	cleanText := s.NormalizeTextContent(priceBandText)

	// Remove additional currency symbols and formatting
	cleanText = strings.ReplaceAll(cleanText, "₹", "")
	cleanText = strings.ReplaceAll(cleanText, "$", "")
	cleanText = strings.ReplaceAll(cleanText, ",", "")
	cleanText = strings.TrimSpace(cleanText)

	// Try different separator patterns
	separators := []string{" - ", "-", " to ", "to", " ~ ", "~"}

	for _, separator := range separators {
		if strings.Contains(cleanText, separator) {
			parts := strings.Split(cleanText, separator)
			if len(parts) >= 2 {
				var prices []float64
				for i := 0; i < 2; i++ {
					if price := s.ParseNumericValueAsFloat(strings.TrimSpace(parts[i])); price != nil {
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
	if price := s.ParseNumericValueAsFloat(cleanText); price != nil {
		return []float64{*price}
	}

	return nil
}

// ExtractPercentage extracts percentage value without the percent symbol
func (s *UtilityService) ExtractPercentage(text string) float64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	// Remove percent symbol
	text = strings.ReplaceAll(text, "%", "")

	// Use ExtractNumeric to handle the rest
	return s.ExtractNumeric(text)
}

// NormalizeString normalizes empty strings to nil
// Treats empty or whitespace-only strings as nil
func (s *UtilityService) NormalizeString(str string) *string {
	str = strings.TrimSpace(str)
	if str == "" {
		return nil
	}
	return &str
}

// IsNotAvailable checks if a value indicates "not available"
// Detects placeholders like "TBA", "To Be Announced", "N/A", etc.
func (s *UtilityService) IsNotAvailable(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))

	notAvailableValues := []string{
		"tba",
		"to be announced",
		"to be decided",
		"tbd",
		"n/a",
		"na",
		"not available",
		"not applicable",
		"not disclosed",
		"awaited",
		"pending",
		"coming soon",
		"will be updated",
		"yet to be announced",
		"--",
		"-",
		"",
		"nil",
		"null",
	}

	for _, na := range notAvailableValues {
		if text == na {
			return true
		}
	}

	return false
}

// NormalizeSymbol normalizes a stock symbol to uppercase alphanumeric format
// Removes special characters and whitespace, converts to uppercase
func (s *UtilityService) NormalizeSymbol(text string) *string {
	text = strings.TrimSpace(text)
	if text == "" || s.IsNotAvailable(text) {
		return nil
	}

	// Convert to uppercase
	text = strings.ToUpper(text)

	// Remove special characters, keep only alphanumeric
	reg := regexp.MustCompile(`[^A-Z0-9]`)
	text = reg.ReplaceAllString(text, "")

	if text == "" {
		return nil
	}

	return &text
}

// ExtractSignedPercentage extracts percentage value with sign handling
// Preserves positive and negative signs in the result
func (s *UtilityService) ExtractSignedPercentage(text string) *float64 {
	text = strings.TrimSpace(text)
	if text == "" || s.IsNotAvailable(text) {
		return nil
	}

	// Remove percent symbol
	text = strings.ReplaceAll(text, "%", "")
	text = strings.TrimSpace(text)

	// Extract numeric value with sign
	reg := regexp.MustCompile(`[+-]?\s*\d+\.?\d*`)
	match := reg.FindString(text)
	if match == "" {
		return nil
	}

	// Remove spaces between sign and number
	match = strings.ReplaceAll(match, " ", "")

	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return nil
	}

	return &value
}

// TableRow represents a parsed table row with label and value
type TableRow struct {
	Index      int     // Row index in the table
	Label      string  // The row label (first column)
	Value      string  // The row value (second column)
	Confidence float64 // Confidence score for the match
}

// ParseHTMLTable parses an HTML table element and extracts rows with flexible matching
// Returns all rows found in the table with their confidence scores
func (s *UtilityService) ParseHTMLTable(table *colly.HTMLElement) []TableRow {
	var rows []TableRow

	// Parse all table rows
	table.ForEach("tr", func(_ int, tr *colly.HTMLElement) {
		// Extract cells from the row
		var cells []string
		tr.ForEach("td, th", func(_ int, cell *colly.HTMLElement) {
			cellText := s.extractCellValue(cell)
			cells = append(cells, cellText)
		})

		// Skip rows with less than 2 cells
		if len(cells) < 2 {
			return
		}

		// Create table row with label and value
		label := strings.TrimSpace(cells[0])
		value := strings.TrimSpace(cells[1])

		// Skip empty rows
		if label == "" && value == "" {
			return
		}

		// Calculate confidence based on label quality
		confidence := s.calculateLabelConfidence(label)

		row := TableRow{
			Label:      label,
			Value:      value,
			Confidence: confidence,
		}

		rows = append(rows, row)
		logrus.Debugf("Parsed table row: %s -> %s (confidence: %.2f)", label, value, confidence)
	})

	return rows
}

// FindTableRowByLabel finds a table row by matching the label with fuzzy matching
// Returns the best matching row and its confidence score
func (s *UtilityService) FindTableRowByLabel(rows []TableRow, targetLabels []string) (TableRow, bool) {
	var bestMatch TableRow
	var bestScore float64 = 0.0
	var found bool = false

	for _, row := range rows {
		normalizedRowLabel := s.normalizeLabel(row.Label)

		for _, targetLabel := range targetLabels {
			normalizedTargetLabel := s.normalizeLabel(targetLabel)
			score := s.calculateMatchScore(normalizedRowLabel, normalizedTargetLabel)

			// Combine match score with row confidence
			combinedScore := score * row.Confidence

			if combinedScore > bestScore {
				bestScore = combinedScore
				bestMatch = row
				found = true
			}
		}
	}

	// Only return matches with reasonable confidence
	if bestScore < 0.3 {
		return TableRow{}, false
	}

	logrus.Debugf("Best label match: '%s' with score %.2f", bestMatch.Label, bestScore)
	return bestMatch, found
}

// GetTargetLabelsForField returns possible label variations for a given field
func (s *UtilityService) GetTargetLabelsForField(fieldName string) []string {
	labelMap := map[string][]string{
		"open_date": {
			"ipo open date", "open date", "opening date", "opens on",
			"subscription open", "subscription opens", "opens",
		},
		"close_date": {
			"ipo close date", "close date", "closing date", "closes on",
			"subscription close", "subscription closes", "closes",
		},
		"listing_date": {
			"listing date", "listing", "lists on", "listing on",
			"expected listing", "tentative listing",
		},
		"result_date": {
			"allotment date", "result date", "allotment", "result",
			"allotment finalization", "basis of allotment",
		},
		"registrar": {
			"registrar", "registrar to issue", "registrar name",
			"registrar to the issue", "registrar and share transfer agent",
		},
		"symbol": {
			"symbol", "nse symbol", "bse symbol", "ticker",
			"scrip code", "stock symbol", "trading symbol",
		},
		"subscription_status": {
			"subscription", "subscription status", "subscribed",
			"subscription level", "overall subscription",
		},
		"listing_gain": {
			"listing gain", "listing gains", "listing performance",
			"listing premium", "first day gain", "debut gain",
		},
		"price_band": {
			"price band", "issue price", "price range",
			"band", "price per share", "issue price band",
		},
		"issue_size": {
			"issue size", "size", "total issue size",
			"public issue size", "fresh issue size",
		},
		"minimum_investment": {
			"minimum investment", "lot size", "minimum lot",
			"application lot", "minimum shares", "min investment",
		},
	}

	if labels, exists := labelMap[fieldName]; exists {
		return labels
	}

	// Return the field name itself as fallback
	return []string{fieldName}
}

// extractCellValue extracts text content from a table cell
// Handles various cell formats and nested elements
func (s *UtilityService) extractCellValue(cell *colly.HTMLElement) string {
	// Get the text content, handling nested elements
	text := strings.TrimSpace(cell.Text)

	// If the cell is empty, try to get content from nested elements
	if text == "" {
		cell.ForEach("span, div, p, a", func(_ int, nested *colly.HTMLElement) {
			if text == "" {
				text = strings.TrimSpace(nested.Text)
			}
		})
	}

	// Clean up the text
	text = s.cleanCellText(text)

	return text
}

// normalizeLabel normalizes a label for matching
func (s *UtilityService) normalizeLabel(label string) string {
	// Convert to lowercase
	normalized := strings.ToLower(label)

	// Remove common punctuation and special characters
	normalized = strings.ReplaceAll(normalized, ":", "")
	normalized = strings.ReplaceAll(normalized, ".", "")
	normalized = strings.ReplaceAll(normalized, ",", "")
	normalized = strings.ReplaceAll(normalized, "(", "")
	normalized = strings.ReplaceAll(normalized, ")", "")
	normalized = strings.ReplaceAll(normalized, "-", " ")
	normalized = strings.ReplaceAll(normalized, "_", " ")

	// Normalize whitespace
	normalized = strings.Join(strings.Fields(normalized), " ")

	return strings.TrimSpace(normalized)
}

// calculateMatchScore calculates similarity score between two normalized labels
func (s *UtilityService) calculateMatchScore(label1, label2 string) float64 {
	// Exact match gets highest score
	if label1 == label2 {
		return 1.0
	}

	// Check if one contains the other
	if strings.Contains(label1, label2) || strings.Contains(label2, label1) {
		return 0.8
	}

	// Calculate word overlap score
	words1 := strings.Fields(label1)
	words2 := strings.Fields(label2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Count matching words
	matchingWords := 0
	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if word1 == word2 {
				matchingWords++
				break
			}
		}
	}

	// Calculate Jaccard similarity (intersection / union)
	totalWords := len(words1) + len(words2) - matchingWords
	if totalWords == 0 {
		return 0.0
	}

	score := float64(matchingWords) / float64(totalWords)

	// Boost score for partial matches
	if matchingWords > 0 {
		score = math.Max(score, 0.4)
	}

	return score
}

// calculateLabelConfidence calculates confidence score for a label
func (s *UtilityService) calculateLabelConfidence(label string) float64 {
	if label == "" {
		return 0.0
	}

	// Start with base confidence
	confidence := 0.5

	// Boost confidence for labels that look like field names
	normalizedLabel := s.normalizeLabel(label)

	// Common IPO field indicators
	ipoKeywords := []string{
		"date", "price", "size", "investment", "registrar", "symbol",
		"subscription", "listing", "gain", "open", "close", "result",
		"allotment", "issue", "band", "minimum", "lot", "shares",
	}

	for _, keyword := range ipoKeywords {
		if strings.Contains(normalizedLabel, keyword) {
			confidence += 0.2
			break
		}
	}

	// Boost confidence for labels with colons (common in data tables)
	if strings.Contains(label, ":") {
		confidence += 0.1
	}

	// Reduce confidence for very short labels
	if len(normalizedLabel) < 3 {
		confidence -= 0.2
	}

	// Reduce confidence for labels that look like values
	if s.IsNotAvailable(label) {
		confidence -= 0.3
	}

	// Ensure confidence is between 0 and 1
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// cleanCellText cleans up extracted cell text
func (s *UtilityService) cleanCellText(text string) string {
	// Remove extra whitespace
	text = strings.Join(strings.Fields(text), " ")

	// Remove common HTML artifacts
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")
	text = strings.ReplaceAll(text, "\r", " ")

	return strings.TrimSpace(text)
}

// CalculateIPOStatus calculates the current status of an IPO based on its dates
// Returns status based on current time relative to IPO timeline:
// - Before open date: "UPCOMING"
// - Between open and close date: "ACTIVE"
// - After close date: "CLOSED"
// - After listing date: "LISTED"
func (s *UtilityService) CalculateIPOStatus(openDate, closeDate, listingDate *time.Time) string {
	now := time.Now()

	// If we have a listing date and it's passed, IPO is listed
	if listingDate != nil && now.After(*listingDate) {
		return "LISTED"
	}

	// If we have a close date and it's passed, IPO is closed
	if closeDate != nil && now.After(*closeDate) {
		return "CLOSED"
	}

	// If we have an open date and it's passed but close date hasn't, IPO is active
	if openDate != nil && now.After(*openDate) {
		return "ACTIVE"
	}

	// If we have an open date and it's in the future, IPO is upcoming
	if openDate != nil && now.Before(*openDate) {
		return "UPCOMING"
	}

	// If we don't have enough date information, return unknown
	return "UNKNOWN"
}

// GenerateSlug creates URL-friendly identifiers following enhanced scraper patterns
func (s *UtilityService) GenerateSlug(text string) string {
	if text == "" {
		return ""
	}

	// Convert to lowercase
	slug := strings.ToLower(text)

	// Remove common legal suffixes
	suffixes := []string{" ltd.", " ltd", " limited", " pvt.", " pvt", " private", " ipo", " inc.", " inc", " corp.", " corp", " company", " co."}
	for _, suffix := range suffixes {
		slug = strings.TrimSuffix(slug, suffix)
	}

	// Replace spaces and special characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Remove consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	return slug
}

// GetServiceMetrics returns the current service metrics
func (s *UtilityService) GetServiceMetrics() *shared.ServiceMetrics {
	return s.serviceMetrics
}

// LogMetricsSummary logs comprehensive metrics summary
func (s *UtilityService) LogMetricsSummary() {
	if s.serviceMetrics != nil {
		s.serviceMetrics.LogSummary()
	}
}

// RecordOperation records a utility service operation with metrics tracking
func (s *UtilityService) RecordOperation(operationName string, success bool, processingTime time.Duration) {
	if s.serviceMetrics != nil {
		s.serviceMetrics.RecordRequest(success, processingTime)
		s.serviceMetrics.IncrementCustomCounter(operationName)
	}
}

// RecordTextProcessingOperation records text processing operations with detailed metrics
func (s *UtilityService) RecordTextProcessingOperation(operationType string, inputLength int, outputLength int, processingTime time.Duration) {
	if s.serviceMetrics != nil {
		s.serviceMetrics.RecordRequest(true, processingTime)
		s.serviceMetrics.SetCustomMetric(operationType+"_input_length", inputLength)
		s.serviceMetrics.SetCustomMetric(operationType+"_output_length", outputLength)
		s.serviceMetrics.SetCustomMetric(operationType+"_compression_ratio", float64(outputLength)/float64(inputLength))
		s.serviceMetrics.IncrementCustomCounter(operationType)
	}
}

// RecordValidationOperation records validation operations with success/failure tracking
func (s *UtilityService) RecordValidationOperation(validationType string, success bool, processingTime time.Duration) {
	if s.serviceMetrics != nil {
		s.serviceMetrics.RecordRequest(success, processingTime)
		s.serviceMetrics.IncrementCustomCounter(validationType)
		if success {
			s.serviceMetrics.IncrementCustomCounter(validationType + "_success")
		} else {
			s.serviceMetrics.IncrementCustomCounter(validationType + "_failure")
		}
	}
}

// GetMetricsSnapshot returns a snapshot of utility service metrics
func (s *UtilityService) GetMetricsSnapshot() map[string]interface{} {
	if s.serviceMetrics != nil {
		snapshot := s.serviceMetrics.GetSnapshot()
		return map[string]interface{}{
			"service_metrics": snapshot,
		}
	}
	return make(map[string]interface{})
}

// ResetMetrics resets all metrics to zero
func (s *UtilityService) ResetMetrics() {
	if s.serviceMetrics != nil {
		s.serviceMetrics.Reset()
	}
	logrus.WithField("service", "Utility_Service").Info("Utility service metrics reset")
}
