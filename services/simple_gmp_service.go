package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// SimpleGMPService provides a fast, efficient GMP scraping service
type SimpleGMPService struct {
	db     *sql.DB
	logger *logrus.Logger
}

// NewSimpleGMPService creates a new simple GMP service
func NewSimpleGMPService(db *sql.DB) *SimpleGMPService {
	return &SimpleGMPService{
		db:     db,
		logger: logrus.New(),
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

// FetchGMPData scrapes GMP data from InvestorGain efficiently
func (s *SimpleGMPService) FetchGMPData() ([]models.EnhancedGMPData, error) {
	startTime := time.Now()
	s.logger.Info("Starting fast GMP data extraction from InvestorGain")

	// Scrape raw data
	rawData, err := s.scrapeInvestorGainData()
	if err != nil {
		s.logger.WithError(err).Error("Failed to scrape InvestorGain data")
		return nil, fmt.Errorf("failed to scrape GMP data: %w", err)
	}

	s.logger.WithField("raw_records", len(rawData)).Info("Successfully scraped raw GMP data")

	// Convert to enhanced GMP data
	var gmpList []models.EnhancedGMPData
	for i, raw := range rawData {
		enhanced := s.convertToEnhancedGMP(raw, i)
		if enhanced != nil {
			gmpList = append(gmpList, *enhanced)
		}
	}

	processingTime := time.Since(startTime)
	s.logger.WithFields(logrus.Fields{
		"total_records":   len(gmpList),
		"processing_time": processingTime,
		"records_per_sec": float64(len(gmpList)) / processingTime.Seconds(),
	}).Info("Completed GMP data extraction")

	return gmpList, nil
}

// scrapeInvestorGainData performs the actual web scraping
func (s *SimpleGMPService) scrapeInvestorGainData() ([]GMPScrapingResult, error) {
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

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var rawTableData []map[string]interface{}
	var updatedOnText string

	// Navigate and extract data efficiently
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate("https://www.investorgain.com/report/live-ipo-gmp/331/all/"),

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
func (s *SimpleGMPService) convertToEnhancedGMP(raw GMPScrapingResult, index int) *models.EnhancedGMPData {
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

func (s *SimpleGMPService) cleanCompanyName(name string) string {
	name = strings.TrimSpace(name)
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")

	// Remove exchange suffixes
	suffixes := []string{"BSE SME", "NSE SME", "BSE", "NSE", "IPO"}
	for _, suffix := range suffixes {
		name = strings.TrimSuffix(name, " "+suffix)
	}

	return name
}

func (s *SimpleGMPService) generateCompanyCode(companyName string) string {
	if companyName == "" {
		return ""
	}

	// Convert to uppercase and remove special characters
	code := strings.ToUpper(companyName)
	code = regexp.MustCompile(`[^A-Z\s]`).ReplaceAllString(code, "")

	// Take first letter of each word, max 6 characters
	words := strings.Fields(code)
	result := ""
	for _, word := range words {
		if len(word) > 0 {
			result += string(word[0])
		}
		if len(result) >= 6 {
			break
		}
	}

	// If too short, pad with company name characters
	if len(result) < 3 && len(words) > 0 {
		firstWord := words[0]
		for i := 1; i < len(firstWord) && len(result) < 6; i++ {
			result += string(firstWord[i])
		}
	}

	return result
}

func (s *SimpleGMPService) parseGMPString(gmpText string) (float64, float64) {
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

func (s *SimpleGMPService) parseLowHighString(lhText string) (float64, float64) {
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

func (s *SimpleGMPService) calculateConfidence(raw GMPScrapingResult) float64 {
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
func (s *SimpleGMPService) SaveGMPData(gmpList []models.EnhancedGMPData) error {
	if s.db == nil {
		s.logger.Warn("Database not available, skipping save")
		return nil
	}

	if len(gmpList) == 0 {
		return nil
	}

	s.logger.WithField("records", len(gmpList)).Info("Saving GMP data to database")

	// Use transaction for efficiency
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare insert statement with all fields
	stmt, err := tx.Prepare(`
		INSERT INTO ipo_gmp (
			id, ipo_name, company_code, ipo_price, gmp_value, 
			estimated_listing, gain_percent, sub2, kostak, last_updated, 
			data_source, stock_id, subscription_status, listing_gain, 
			ipo_status, extraction_metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (ipo_name) DO UPDATE SET
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
			gmp.ID, gmp.IPOName, gmp.CompanyCode, gmp.IPOPrice,
			gmp.GMPValue, gmp.EstimatedListing, gmp.GainPercent,
			gmp.Sub2, gmp.Kostak, gmp.LastUpdated, gmp.DataSource,
			gmp.StockID, gmp.SubscriptionStatus, gmp.ListingGain,
			gmp.IPOStatus, string(metadataJSON),
		)
		if err != nil {
			s.logger.WithError(err).WithField("company", gmp.IPOName).Error("Failed to save GMP record")
			continue
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.WithField("records", len(gmpList)).Info("Successfully saved GMP data")
	return nil
}

// FetchAndSaveGMPData combines fetching and saving in one operation
func (s *SimpleGMPService) FetchAndSaveGMPData() ([]models.EnhancedGMPData, error) {
	gmpData, err := s.FetchGMPData()
	if err != nil {
		return nil, err
	}

	if err := s.SaveGMPData(gmpData); err != nil {
		s.logger.WithError(err).Warn("Failed to save GMP data, but returning scraped data")
	}

	return gmpData, nil
}
