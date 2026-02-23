package jobs

import (
	"context"
	"strings"
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
	MatcherService  *services.MatcherService
}

func NewDailyIPOUpdateJob(
	scrapingService *services.ChittorgarhIPOScrapingService,
	growwScraper *services.GrowwScraperService,
	growwMapper *services.GrowwMapper,
	ipoService *services.IPOService,
	utilityService *services.UtilityService,
) *DailyIPOUpdateJob {
	matcherService := services.NewMatcherService(utilityService)
	return &DailyIPOUpdateJob{
		ScrapingService: scrapingService,
		GrowwScraper:    growwScraper,
		GrowwMapper:     growwMapper,
		IPOService:      ipoService,
		UtilityService:  utilityService,
		MatcherService:  matcherService,
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

	// Pre-compute normalized Groww slugs for optimized entity resolution
	growwSlugCache := j.buildGrowwSlugCache(growwSlugs)
	logrus.Infof("Discovered %d slugs from Groww (cache built for entity resolution)", len(pendingSlugs))

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
		chittorgarhName := item.IPONewsTitle
		chittorgarhSlug := item.URLRewriteFolderName
		delete(pendingSlugs, chittorgarhSlug)

		needsRefresh, lastUpdated, err := j.IPOService.NeedsRefresh(ctx, chittorgarhSlug)
		if err != nil {
			logrus.WithError(err).Warnf("Failed to check TTL for %s, proceeding with scrape", chittorgarhSlug)
		} else if !needsRefresh {
			logrus.WithFields(logrus.Fields{
				"slug":         chittorgarhSlug,
				"last_updated": lastUpdated,
			}).Debug("Skipping IPO - TTL not expired (Smart TTL)")
			continue
		}

		logrus.WithFields(logrus.Fields{
			"ipo_index":  i + 1,
			"total_ipos": len(items),
			"ipo_name":   chittorgarhName,
			"slug":       chittorgarhSlug,
		}).Infof("Processing IPO %d/%d: %s", i+1, len(items), chittorgarhName)

		var ipoModel *models.IPO

		// 3. Entity Resolution: Use pre-built cache for optimized slug matching
		matchedGrowwSlug := j.findMatchingGrowwSlug(chittorgarhSlug, growwSlugCache)
		if matchedGrowwSlug != "" {
			logrus.WithFields(logrus.Fields{
				"chittorgarh_slug": chittorgarhSlug,
				"groww_slug":       matchedGrowwSlug,
			}).Info("Matched Chittorgarh IPO to Groww using Entity Resolution")
		}

		// 4. Primary Source: Try Groww Details API (using matched slug or exact slug match)
		growwSlugToTry := matchedGrowwSlug
		if growwSlugToTry == "" {
			growwSlugToTry = chittorgarhSlug
		}

		growwData := j.GrowwScraper.ScrapeIPO(ctx, growwSlugToTry)
		if growwData.DetailsError == "" && growwData.Details != nil {
			ipoModel = j.GrowwMapper.MapToIPO(growwData, &item)

			if growwData.CMSError != "" {
				logrus.WithFields(logrus.Fields{
					"slug":       growwSlugToTry,
					"chitt_slug": chittorgarhSlug,
					"cms_error":  growwData.CMSError,
				}).Info("Groww details fetched; CMS missing (non-fatal)")
			} else {
				logrus.WithFields(logrus.Fields{
					"slug":       growwSlugToTry,
					"chitt_slug": chittorgarhSlug,
				}).Info("Scraped rich data from Groww successfully (matched via Entity Resolution)")
			}
			growwWins++
		} else {
			// 5. Fallback Source: Chittorgarh Basic JSON Metadata
			logrus.WithFields(logrus.Fields{
				"slug":       growwSlugToTry,
				"chitt_slug": chittorgarhSlug,
				"error":      growwData.DetailsError,
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

		circuitBreakerFailedSlugs := make(map[string]bool)
		maxRetries := 2
		retryDelay := 30 * time.Second

		for retry := 0; retry <= maxRetries; retry++ {
			if retry > 0 && len(circuitBreakerFailedSlugs) > 0 {
				logrus.Infof("Retry %d/%d: Processing %d slugs that failed due to circuit breaker",
					retry, maxRetries, len(circuitBreakerFailedSlugs))
				time.Sleep(retryDelay)
			}

			currentBatch := make(map[string]bool)
			if retry == 0 {
				for slug := range pendingSlugs {
					currentBatch[slug] = true
				}
			} else {
				for slug := range circuitBreakerFailedSlugs {
					currentBatch[slug] = true
				}
				circuitBreakerFailedSlugs = make(map[string]bool)
			}

			for slug := range currentBatch {
				growwData := j.GrowwScraper.ScrapeIPO(ctx, slug)

				if strings.Contains(growwData.DetailsError, "circuit breaker is open") ||
					strings.Contains(growwData.CMSError, "circuit breaker is open") {
					circuitBreakerFailedSlugs[slug] = true
					logrus.WithFields(logrus.Fields{
						"slug":        slug,
						"retry":       retry,
						"max_retries": maxRetries,
					}).Warn("Circuit breaker open - queuing for retry")
					continue
				}

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
				} else if retry == maxRetries {
					logrus.WithFields(logrus.Fields{
						"slug":          slug,
						"details_error": growwData.DetailsError,
						"cms_error":     growwData.CMSError,
					}).Warn("Failed to scrape IPO after all retries")
					failureCount++
				}
				time.Sleep(2 * time.Second)
			}

			if len(circuitBreakerFailedSlugs) == 0 {
				break
			}
		}

		if len(circuitBreakerFailedSlugs) > 0 {
			logrus.Warnf("Still have %d slugs that failed after all retries - consider manual review",
				len(circuitBreakerFailedSlugs))
			for slug := range circuitBreakerFailedSlugs {
				logrus.Warnf("Pending IPO slug requiring manual review: %s", slug)
			}
		}
	}

	// Log comprehensive job completion summary
	totalProcessed := successCount + partialSuccessCount + failureCount
	logrus.WithFields(logrus.Fields{
		"total_processed":       totalProcessed,
		"full_success":          successCount,
		"partial_success":       partialSuccessCount,
		"failures":              failureCount,
		"groww_successes":       growwWins,
		"chittorgarh_fallbacks": chittWins,
		"overall_success_rate":  float64(successCount+partialSuccessCount) / float64(totalProcessed) * 100,
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

type growwSlugCache struct {
	original   string
	normalized string
}

func (j *DailyIPOUpdateJob) buildGrowwSlugCache(slugs []string) []growwSlugCache {
	cache := make([]growwSlugCache, 0, len(slugs))
	for _, slug := range slugs {
		normalized := j.MatcherService.NormalizeSlugForMatching(slug)
		cache = append(cache, growwSlugCache{
			original:   slug,
			normalized: normalized,
		})
	}
	return cache
}

func (j *DailyIPOUpdateJob) findMatchingGrowwSlug(chittorgarhSlug string, cache []growwSlugCache) string {
	chittNormalized := j.MatcherService.NormalizeSlugForMatching(chittorgarhSlug)

	for _, cached := range cache {
		if cached.normalized == chittNormalized {
			return cached.original
		}
	}

	for _, cached := range cache {
		matchResult := j.MatcherService.MatchBySlug(chittorgarhSlug, cached.original)
		if matchResult.IsMatch && matchResult.Confidence >= 85.0 {
			return cached.original
		}
	}

	return ""
}
