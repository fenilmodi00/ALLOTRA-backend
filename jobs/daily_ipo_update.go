package jobs

import (
	"context"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/sirupsen/logrus"
)

type DailyIPOUpdateJob struct {
	ScrapingService *services.ChittorgarhIPOScrapingService
	GrowwScraper    *services.GrowwScraperService
	GrowwMapper     *services.GrowwMapper
	IPOService      *services.IPOService
	UtilityService  *services.UtilityService
}

func NewDailyIPOUpdateJob(
	scrapingService *services.ChittorgarhIPOScrapingService,
	growwScraper *services.GrowwScraperService,
	growwMapper *services.GrowwMapper,
	ipoService *services.IPOService,
	utilityService *services.UtilityService,
) *DailyIPOUpdateJob {
	return &DailyIPOUpdateJob{
		ScrapingService: scrapingService,
		GrowwScraper:    growwScraper,
		GrowwMapper:     growwMapper,
		IPOService:      ipoService,
		UtilityService:  utilityService,
	}
}

func (j *DailyIPOUpdateJob) Run() {
	logrus.Info("Starting Daily IPO Update Job with Groww primary + Chittorgarh fallback")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	// 1. Discover all active slugs on Groww
	logrus.Info("Discovering all available IPO slugs from Groww...")
	growwSlugs, err := j.GrowwScraper.DiscoverSlugs(ctx)
	if err != nil {
		logrus.Errorf("Failed to discover Groww slugs: %v", err)
	}
	
	pendingSlugs := make(map[string]bool)
	for _, slug := range growwSlugs {
		pendingSlugs[slug] = true
	}
	logrus.Infof("Discovered %d slugs from Groww", len(pendingSlugs))

	// 2. Fetch primary discovery list from Chittorgarh (for absolute maximum coverage)
	logrus.Info("Fetching IPO list from Chittorgarh scraping service...")
	items, err := j.ScrapingService.FetchAvailableIPOList()
	if err != nil {
		logrus.Errorf("Failed to run Daily IPO Update Job: failed to fetch IPO list: %v", err)
		return
	}

	logrus.Infof("Fetched %d IPOs from Chittorgarh for processing", len(items))

	successCount := 0
	failureCount := 0
	partialSuccessCount := 0
	growwWins := 0
	chittWins := 0

	for i, item := range items {
		slug := item.URLRewriteFolderName
		delete(pendingSlugs, slug) // Mark as processed from Chittorgarh list
		
		logrus.WithFields(logrus.Fields{
			"ipo_index":  i + 1,
			"total_ipos": len(items),
			"ipo_name":   item.IPONewsTitle,
			"slug":       slug,
		}).Infof("Processing IPO %d/%d: %s", i+1, len(items), item.IPONewsTitle)

		var ipoModel *models.IPO

		// 3. Primary Source: Try Groww Details API
		growwData := j.GrowwScraper.ScrapeIPO(ctx, slug)
		if growwData.DetailsError == "" && growwData.Details != nil {
			// Groww succeeded! Map its rich data.
			ipoModel = j.GrowwMapper.MapToIPO(growwData, &item)
			growwWins++
			logrus.WithField("slug", slug).Info("Scraped rich data from Groww successfully")
		} else {
			// 4. Fallback Source: Chittorgarh Basic JSON Metadata
			logrus.WithFields(logrus.Fields{
				"slug":  slug,
				"error": growwData.DetailsError,
			}).Warn("Groww failed or 404ed, falling back to Chittorgarh")
			
			ipoModel, err = j.ScrapingService.ScrapeDetailedIPOInformation(item)
			if err != nil {
				logrus.Errorf("Failed to scrape details for %s from fallback: %v", item.IPONewsTitle, err)
				failureCount++
				continue
			}
			chittWins++
		}

		// Ensure company code exists
		if ipoModel.CompanyCode == "" {
			ipoModel.CompanyCode = j.UtilityService.GenerateCompanyCode(ipoModel.Name)
		}

		// Analyze data completeness
		completeness := j.analyzeDataCompleteness(ipoModel)

		// Persist to ipos table
		if err := j.IPOService.UpsertIPO(ctx, *ipoModel); err != nil {
			logrus.Errorf("Failed to upsert IPO %s to ipos table: %v", item.IPONewsTitle, err)
			failureCount++
			continue
		}

		// Categorize success type
		if completeness.CriticalFieldsComplete {
			if completeness.OverallCompleteness >= 80.0 {
				successCount++
			} else {
				partialSuccessCount++
			}
		} else {
			partialSuccessCount++
		}

		// Rate limiting
		if i < len(items)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	// 5. Scrape remaining Groww slugs not found in Chittorgarh's list (Edge Case)
	if len(pendingSlugs) > 0 {
		logrus.Infof("Processing %d remaining Groww slugs not found in Chittorgarh...", len(pendingSlugs))
		for slug := range pendingSlugs {
			growwData := j.GrowwScraper.ScrapeIPO(ctx, slug)
			if growwData.DetailsError == "" && growwData.Details != nil {
				ipoModel := j.GrowwMapper.MapToIPO(growwData, nil)
				
				if err := j.IPOService.UpsertIPO(ctx, *ipoModel); err != nil {
					logrus.Errorf("Failed to upsert Groww-only IPO %s: %v", slug, err)
					failureCount++
				} else {
					successCount++
					growwWins++
					logrus.Infof("Successfully saved Groww-only IPO: %s", slug)
				}
				time.Sleep(2 * time.Second)
			}
		}
	}

	// Log comprehensive job completion summary
	totalProcessed := successCount + partialSuccessCount + failureCount
	logrus.WithFields(logrus.Fields{
		"total_processed":      totalProcessed,
		"full_success":         successCount,
		"partial_success":      partialSuccessCount,
		"failures":             failureCount,
		"groww_successes":      growwWins,
		"chittorgarh_fallbacks": chittWins,
		"overall_success_rate": float64(successCount+partialSuccessCount) / float64(totalProcessed) * 100,
	}).Infof("Daily IPO Update Job completed")
}

// DataCompleteness represents the completeness analysis of an IPO record
type DataCompleteness struct {
	TotalFields            int      `json:"total_fields"`
	PopulatedFields        int      `json:"populated_fields"`
	CriticalFields         int      `json:"critical_fields"`
	CriticalFieldsComplete bool     `json:"critical_fields_complete"`
	OverallCompleteness    float64  `json:"overall_completeness"`
	CriticalCompleteness   float64  `json:"critical_completeness"`
	MissingCriticalFields  []string `json:"missing_critical_fields"`
	MissingOptionalFields  []string `json:"missing_optional_fields"`
}

// analyzeDataCompleteness analyzes the completeness of IPO data
func (j *DailyIPOUpdateJob) analyzeDataCompleteness(ipo *models.IPO) DataCompleteness {
	// Define critical fields that should always be present
	criticalFields := map[string]interface{}{
		"name":            ipo.Name,
		"company_code":    ipo.CompanyCode,
		"price_band_low":  ipo.PriceBandLow,
		"price_band_high": ipo.PriceBandHigh,
	}

	// Define all trackable fields
	allFields := map[string]interface{}{
		"name":                ipo.Name,
		"company_code":        ipo.CompanyCode,
		"description":         ipo.Description,
		"price_band_low":      ipo.PriceBandLow,
		"price_band_high":     ipo.PriceBandHigh,
		"issue_size":          ipo.IssueSize,
		"open_date":           ipo.OpenDate,
		"close_date":          ipo.CloseDate,
		"listing_date":        ipo.ListingDate,
		"result_date":         ipo.ResultDate,
		"min_qty":             ipo.MinQty,
		"min_amount":          ipo.MinAmount,
		"symbol":              ipo.Symbol,
		"slug":                ipo.Slug,
		"about":               ipo.About,
		"subscription_status": ipo.SubscriptionStatus,
		"listing_gain":        ipo.ListingGain,
		"strengths":           ipo.Strengths,
		"risks":               ipo.Risks,
		"registrar":           ipo.Registrar,
	}

	// Count populated fields
	populatedFields := 0
	criticalFieldsComplete := 0
	var missingCriticalFields []string
	var missingOptionalFields []string

	// Check critical fields
	for fieldName, value := range criticalFields {
		if j.isFieldPopulated(value) {
			criticalFieldsComplete++
		} else {
			missingCriticalFields = append(missingCriticalFields, fieldName)
		}
	}

	// Check all fields
	for fieldName, value := range allFields {
		if j.isFieldPopulated(value) {
			populatedFields++
		} else {
			// Check if it's a critical field
			if _, isCritical := criticalFields[fieldName]; !isCritical {
				missingOptionalFields = append(missingOptionalFields, fieldName)
			}
		}
	}

	// Calculate completeness percentages
	overallCompleteness := float64(populatedFields) / float64(len(allFields)) * 100
	criticalCompleteness := float64(criticalFieldsComplete) / float64(len(criticalFields)) * 100
	allCriticalComplete := criticalFieldsComplete == len(criticalFields)

	return DataCompleteness{
		TotalFields:            len(allFields),
		PopulatedFields:        populatedFields,
		CriticalFields:         len(criticalFields),
		CriticalFieldsComplete: allCriticalComplete,
		OverallCompleteness:    overallCompleteness,
		CriticalCompleteness:   criticalCompleteness,
		MissingCriticalFields:  missingCriticalFields,
		MissingOptionalFields:  missingOptionalFields,
	}
}

// isFieldPopulated checks if a field has meaningful data using utility service
func (j *DailyIPOUpdateJob) isFieldPopulated(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return v != "" && !j.UtilityService.IsNotAvailable(v)
	case *string:
		return v != nil && *v != "" && !j.UtilityService.IsNotAvailable(*v)
	case *int:
		return v != nil
	case *float64:
		return v != nil
	case *time.Time:
		return v != nil
	case []byte:
		return len(v) > 0
	case nil:
		return false
	default:
		return true // Assume populated for unknown types
	}
}

// logFieldPopulation logs field population status
func (j *DailyIPOUpdateJob) logFieldPopulation(ipo *models.IPO, completeness DataCompleteness) {
	// Log field population
	logrus.WithFields(logrus.Fields{
		"ipo_name":              ipo.Name,
		"overall_completeness":  completeness.OverallCompleteness,
		"critical_completeness": completeness.CriticalCompleteness,
		"populated_fields":      completeness.PopulatedFields,
		"total_fields":          completeness.TotalFields,
		"critical_fields_ok":    completeness.CriticalFieldsComplete,
	}).Infof("IPO %s data analysis: %.1f%% complete",
		ipo.Name, completeness.OverallCompleteness)

	// Log missing critical fields as warnings
	if len(completeness.MissingCriticalFields) > 0 {
		logrus.WithFields(logrus.Fields{
			"ipo_name":       ipo.Name,
			"missing_fields": completeness.MissingCriticalFields,
		}).Warnf("IPO %s missing critical fields: %v", ipo.Name, completeness.MissingCriticalFields)
	}

	// Log missing optional fields as debug (only if many are missing)
	if len(completeness.MissingOptionalFields) > 5 {
		logrus.WithFields(logrus.Fields{
			"ipo_name":       ipo.Name,
			"missing_count":  len(completeness.MissingOptionalFields),
			"missing_fields": completeness.MissingOptionalFields,
		}).Debugf("IPO %s missing %d optional fields", ipo.Name, len(completeness.MissingOptionalFields))
	}

	// Log successful extractions for high-value fields
	successfulExtractions := []string{}
	if ipo.OpenDate != nil {
		successfulExtractions = append(successfulExtractions, "open_date")
	}
	if ipo.CloseDate != nil {
		successfulExtractions = append(successfulExtractions, "close_date")
	}
	if ipo.ListingDate != nil {
		successfulExtractions = append(successfulExtractions, "listing_date")
	}
	if ipo.Symbol != nil {
		successfulExtractions = append(successfulExtractions, "symbol")
	}
	if ipo.Registrar != "" {
		successfulExtractions = append(successfulExtractions, "registrar")
	}
	if ipo.SubscriptionStatus != nil {
		successfulExtractions = append(successfulExtractions, "subscription_status")
	}
	if ipo.ListingGain != nil {
		successfulExtractions = append(successfulExtractions, "listing_gain")
	}

	if len(successfulExtractions) > 0 {
		logrus.WithFields(logrus.Fields{
			"ipo_name":               ipo.Name,
			"successful_extractions": successfulExtractions,
		}).Debugf("IPO %s successfully extracted key fields: %v", ipo.Name, successfulExtractions)
	}
}
