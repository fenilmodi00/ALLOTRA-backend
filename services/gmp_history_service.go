package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// GMPHistoryService handles business logic for GMP price history management
type GMPHistoryService struct {
	db                 *sql.DB
	logger             *logrus.Logger
	errorLogger        *GMPHistoryErrorLogger
	scraper            *GMPPriceHistoryScraper
	utilityService     *UtilityService
	requestRateLimiter *shared.HTTPRequestRateLimiter
	resilienceQueue    *DBResilienceQueue
	cache              *CacheService
}

var (
	ErrPriceHistoryNotFound = errors.New("gmp price history not found")
	ErrServiceUnavailable   = errors.New("gmp history service unavailable")
)

// NewGMPHistoryService creates a new GMP history service instance
func NewGMPHistoryService(db *sql.DB) *GMPHistoryService {
	logger := logrus.New()
	errorLogger := NewGMPHistoryErrorLogger(logger)

	var resilienceQueue *DBResilienceQueue
	if db != nil {
		resilienceQueue = NewDBResilienceQueue(db, logger)
		resilienceQueue.Start()
	}

	cache := NewCacheServiceWithConfig(db, 10*time.Minute, 500)

	scraper := NewGMPPriceHistoryScraper(db)

	service := &GMPHistoryService{
		db:              db,
		logger:          logger,
		errorLogger:     errorLogger,
		scraper:         scraper,
		utilityService:  NewUtilityService(),
		resilienceQueue: resilienceQueue,
		cache:           cache,
	}

	if service.scraper != nil {
		service.scraper.SetErrorLogger(errorLogger)
	}

	return service
}

// Close gracefully shuts down the service and its background workers
func (s *GMPHistoryService) Close() {
	if s.resilienceQueue != nil {
		s.resilienceQueue.Stop()
	}
}

// ResolveIPOIdentifier resolves an identifier (UUID or stock_id) to an IPO UUID
// Supports both:
//   - IPO UUID (e.g., d9d0343d-d727-49cf-aa9d-1189c0ecbb3a)
//   - Stock ID (e.g., 2462)
func (s *GMPHistoryService) ResolveIPOIdentifier(identifier string) (string, error) {
	if identifier == "" {
		return "", fmt.Errorf("identifier is required")
	}
	if s.db == nil {
		return "", fmt.Errorf("%w: database connection is not initialized", ErrServiceUnavailable)
	}

	// Try to parse as UUID first
	if _, err := uuid.Parse(identifier); err == nil {
		// It's a valid UUID, return as-is
		return identifier, nil
	}

	// Not a UUID, try to look up by stock_id
	var ipoID string
	query := `SELECT id FROM ipo_list WHERE stock_id = $1 LIMIT 1`
	err := s.db.QueryRow(query, identifier).Scan(&ipoID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no IPO found with stock_id: %s", identifier)
		}
		s.errorLogger.LogDatabaseError(
			"GMPHistoryService",
			"ResolveIPOIdentifier",
			err,
			map[string]interface{}{
				"identifier": identifier,
			},
		)
		return "", fmt.Errorf("failed to resolve identifier: %w", err)
	}

	return ipoID, nil
}

// SavePriceHistory saves price history entries to the database with upsert logic
// Implements Requirement 2.3 - Upsert logic to prevent duplicates
// Implements Requirement 2.2 - Link entries to corresponding IPO
// Implements Requirement 6.5 - Database resilience with queuing and retry
// Implements Requirement 7.2 - Cache invalidation on data updates
func (s *GMPHistoryService) SavePriceHistory(history *models.GMPPriceHistoryCollection) error {
	if history == nil {
		return fmt.Errorf("history collection is nil")
	}

	if len(history.Entries) == 0 {
		s.logger.WithField("ipo_id", history.IPOID).Info("No entries to save")
		return nil
	}

	if s.resilienceQueue == nil {
		return fmt.Errorf("%w: persistence queue is not initialized", ErrServiceUnavailable)
	}

	s.logger.WithFields(logrus.Fields{
		"ipo_id":       history.IPOID,
		"company_code": history.CompanyCode,
		"entry_count":  len(history.Entries),
	}).Info("Saving price history to database")

	// Prepare entries with IDs and timestamps
	for i := range history.Entries {
		// Generate UUID if not present
		if history.Entries[i].ID == "" {
			history.Entries[i].ID = uuid.New().String()
		}

		// Set timestamps
		now := time.Now()
		if history.Entries[i].CreatedAt.IsZero() {
			history.Entries[i].CreatedAt = now
		}
		history.Entries[i].UpdatedAt = now

		// Set IPO ID and company code from collection
		history.Entries[i].IPOID = history.IPOID
		history.Entries[i].CompanyCode = history.CompanyCode
	}

	// Use resilience queue for database operations
	// This provides automatic retry, queuing on failure, and circuit breaker protection
	err := s.resilienceQueue.SaveWithResilience(history)
	if err != nil {
		s.errorLogger.LogDatabaseError(
			"GMPHistoryService",
			"SavePriceHistory",
			err,
			map[string]interface{}{
				"ipo_id":       history.IPOID,
				"company_code": history.CompanyCode,
				"entry_count":  len(history.Entries),
			},
		)
		return err
	}

	// Invalidate cache for this IPO (Requirement 7.2)
	s.InvalidateIPOCache(history.IPOID)

	s.logger.WithFields(logrus.Fields{
		"ipo_id":        history.IPOID,
		"total_entries": len(history.Entries),
	}).Info("Price history saved successfully and cache invalidated")

	return nil
}

// GetPriceHistoryByIPO retrieves price history for a specific IPO with optional date range filtering
// Implements Requirement 2.2 - Query price history by IPO
// Implements Requirement 3.2 - Date range filtering support
// Implements Requirement 7.1, 7.2 - Caching for performance
func (s *GMPHistoryService) GetPriceHistoryByIPO(ipoID string, dateRange *models.DateRange) (*models.GMPPriceHistoryCollection, error) {
	if ipoID == "" {
		return nil, fmt.Errorf("ipo_id is required")
	}
	if s.db == nil {
		return nil, fmt.Errorf("%w: database connection is not initialized", ErrServiceUnavailable)
	}

	// Build cache key based on IPO ID and date range
	cacheKey := fmt.Sprintf("gmp_history:%s", ipoID)
	if dateRange != nil {
		if !dateRange.StartDate.IsZero() {
			cacheKey += fmt.Sprintf(":start_%s", dateRange.StartDate.Format("2006-01-02"))
		}
		if !dateRange.EndDate.IsZero() {
			cacheKey += fmt.Sprintf(":end_%s", dateRange.EndDate.Format("2006-01-02"))
		}
	}

	// Try to get from cache first (Requirement 7.2)
	if cached, found := s.cache.Get(cacheKey); found {
		if history, ok := cached.(*models.GMPPriceHistoryCollection); ok {
			s.logger.WithFields(logrus.Fields{
				"ipo_id":    ipoID,
				"cache_key": cacheKey,
				"cache_hit": true,
			}).Debug("Price history retrieved from cache")
			return history, nil
		}
	}

	s.logger.WithFields(logrus.Fields{
		"ipo_id":     ipoID,
		"date_range": dateRange != nil,
		"cache_hit":  false,
	}).Info("Retrieving price history from database")

	// Build query with optional date range filtering
	query := `
		SELECT
			h.id, h.ipo_id, h.company_code, h.record_date, h.ipo_price, h.gmp_value,
			h.estimated_listing, h.listing_percent, h.estimated_profit,
			h.subscription_status, h.sub2_sauda, h.last_updated, h.data_source,
			h.created_at, h.updated_at,
			i.name
		FROM gmp_price_history h
		JOIN ipo_list i ON i.id = h.ipo_id
		WHERE h.ipo_id = $1
	`

	args := []interface{}{ipoID}
	argIndex := 2

	// Add date range filtering if provided (Requirement 3.2)
	if dateRange != nil {
		if !dateRange.StartDate.IsZero() {
			query += fmt.Sprintf(" AND h.record_date >= $%d", argIndex)
			args = append(args, dateRange.StartDate)
			argIndex++
		}
		if !dateRange.EndDate.IsZero() {
			query += fmt.Sprintf(" AND h.record_date <= $%d", argIndex)
			args = append(args, dateRange.EndDate)
			argIndex++
		}
	}

	// Order by date descending (most recent first)
	query += " ORDER BY h.record_date DESC"

	// Execute query
	rows, err := s.db.Query(query, args...)
	if err != nil {
		s.errorLogger.LogDatabaseError(
			"GMPHistoryService",
			"GetPriceHistoryByIPO",
			err,
			map[string]interface{}{
				"ipo_id":     ipoID,
				"date_range": dateRange != nil,
				"query":      "gmp_price_history",
			},
		)
		return nil, fmt.Errorf("failed to query price history: %w", err)
	}
	defer rows.Close()

	// Parse results
	var entries []models.GMPPriceHistoryEntry
	var companyCode string
	var ipoName string

	for rows.Next() {
		var entry models.GMPPriceHistoryEntry
		err := rows.Scan(
			&entry.ID,
			&entry.IPOID,
			&entry.CompanyCode,
			&entry.RecordDate,
			&entry.IPOPrice,
			&entry.GMPValue,
			&entry.EstimatedListing,
			&entry.ListingPercent,
			&entry.EstimatedProfit,
			&entry.SubscriptionStatus,
			&entry.Sub2Sauda,
			&entry.LastUpdated,
			&entry.DataSource,
			&entry.CreatedAt,
			&entry.UpdatedAt,
			&ipoName,
		)
		if err != nil {
			s.errorLogger.LogDatabaseError(
				"GMPHistoryService",
				"GetPriceHistoryByIPO.ScanRow",
				err,
				map[string]interface{}{
					"ipo_id": ipoID,
					"row":    "price_history_entry",
				},
			)
			continue
		}

		entries = append(entries, entry)
		if companyCode == "" {
			companyCode = entry.CompanyCode
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating price history rows: %w", err)
	}

	// Check if any entries were found
	if len(entries) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrPriceHistoryNotFound, ipoID)
	}

	if ipoName == "" {
		ipoName = companyCode
	}

	// Build collection
	collection := &models.GMPPriceHistoryCollection{
		IPOID:        ipoID,
		IPOName:      ipoName,
		CompanyCode:  companyCode,
		TotalRecords: len(entries),
		Entries:      entries,
	}

	// Calculate date range from entries
	if len(entries) > 0 {
		// Entries are ordered DESC, so first is latest, last is earliest
		collection.DateRange = &models.DateRange{
			StartDate: entries[len(entries)-1].RecordDate,
			EndDate:   entries[0].RecordDate,
		}
	}

	// Add metadata
	collection.Metadata = &models.CollectionMetadata{
		LastScraped:     time.Now(),
		DataSource:      "investorgain.com",
		ScrapingSuccess: true,
		ErrorCount:      0,
	}

	// Cache the result (Requirement 7.2)
	// Use longer TTL for historical data (15 minutes) as it doesn't change frequently
	s.cache.SetWithTTL(cacheKey, collection, 15*time.Minute)

	s.logger.WithFields(logrus.Fields{
		"ipo_id":        ipoID,
		"entries_found": len(entries),
		"cached":        true,
	}).Info("Price history retrieved successfully and cached")

	return collection, nil
}

// ValidateHistoryData validates a price history entry against business rules
// Implements Requirements 5.1, 5.2, 5.3, 5.4 - Data validation
func (s *GMPHistoryService) ValidateHistoryData(entry *models.GMPPriceHistoryEntry) error {
	if entry == nil {
		return fmt.Errorf("entry is nil")
	}

	// GMP can be negative (grey market discount) - this is valid data
	// IPO price must be non-negative
	if entry.IPOPrice < 0 {
		return fmt.Errorf("IPO price cannot be negative: %.2f", entry.IPOPrice)
	}

	if entry.EstimatedListing < 0 {
		return fmt.Errorf("estimated listing price cannot be negative: %.2f", entry.EstimatedListing)
	}

	// Requirement 5.2: Dates should be within reasonable range
	now := time.Now()
	twoYearsAgo := now.AddDate(-2, 0, 0)
	oneYearFuture := now.AddDate(1, 0, 0)

	if entry.RecordDate.Before(twoYearsAgo) || entry.RecordDate.After(oneYearFuture) {
		return fmt.Errorf("record date out of reasonable range: %s", entry.RecordDate.Format("2006-01-02"))
	}

	// Requirement 5.3: Verify estimated listing price calculation
	expectedListing := entry.IPOPrice + entry.GMPValue
	diff := entry.EstimatedListing - expectedListing
	tolerance := 0.01
	if diff < -tolerance || diff > tolerance {
		return fmt.Errorf("estimated listing price mismatch: expected %.2f, got %.2f", expectedListing, entry.EstimatedListing)
	}

	// Requirement 5.4: Verify percentage calculation
	if entry.IPOPrice > 0 {
		expectedPercent := ((entry.EstimatedListing - entry.IPOPrice) / entry.IPOPrice) * 100
		percentDiff := entry.ListingPercent - expectedPercent
		percentTolerance := 0.1
		if percentDiff < -percentTolerance || percentDiff > percentTolerance {
			return fmt.Errorf("listing percentage mismatch: expected %.2f%%, got %.2f%%", expectedPercent, entry.ListingPercent)
		}
	}

	return nil
}

// ScrapeIPOPriceHistory scrapes price history for a specific IPO
// Implements Requirements 1.1, 1.2, 1.3 - Scrape historical price data
func (s *GMPHistoryService) ScrapeIPOPriceHistory(ipoID string, companyCode string) (*models.GMPPriceHistoryCollection, error) {
	return s.ScrapeIPOPriceHistoryWithName(ipoID, companyCode, "")
}

func (s *GMPHistoryService) ScrapeIPOPriceHistoryWithName(ipoID string, companyCode string, ipoName string) (*models.GMPPriceHistoryCollection, error) {
	if ipoID == "" || companyCode == "" {
		return nil, fmt.Errorf("ipoID and companyCode are required")
	}
	if s.db == nil || s.scraper == nil {
		return nil, fmt.Errorf("%w: scraping dependencies are not initialized", ErrServiceUnavailable)
	}

	if ipoName == "" {
		err := s.db.QueryRow("SELECT name FROM ipo_list WHERE id = $1", ipoID).Scan(&ipoName)
		if err != nil {
			return nil, fmt.Errorf("failed to get IPO name: %w", err)
		}
	}

	s.logger.WithFields(logrus.Fields{
		"ipo_id":       ipoID,
		"ipo_name":     ipoName,
		"company_code": companyCode,
	}).Info("Scraping IPO price history")

	// Build URL for scraping (using IPO name to find numeric ID)
	url, err := s.scraper.BuildIPOHistoryURL(companyCode, ipoName)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	// Scrape data from InvestorGain
	scrapedData, err := s.scraper.ScrapeHistoryFromURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape history: %w", err)
	}

	// Validate scraped data
	if err := s.scraper.ValidateScrapedData(scrapedData); err != nil {
		return nil, fmt.Errorf("scraped data validation failed: %w", err)
	}

	// Use the IPO name we already fetched from database
	// (no need to fetch again)

	// Build collection
	collection := &models.GMPPriceHistoryCollection{
		IPOID:        ipoID,
		IPOName:      ipoName,
		CompanyCode:  companyCode,
		TotalRecords: len(scrapedData.PriceHistory),
		Entries:      scrapedData.PriceHistory,
	}

	// Calculate date range
	if len(scrapedData.PriceHistory) > 0 {
		minDate := scrapedData.PriceHistory[0].RecordDate
		maxDate := scrapedData.PriceHistory[0].RecordDate

		for _, entry := range scrapedData.PriceHistory {
			if entry.RecordDate.Before(minDate) {
				minDate = entry.RecordDate
			}
			if entry.RecordDate.After(maxDate) {
				maxDate = entry.RecordDate
			}
		}

		collection.DateRange = &models.DateRange{
			StartDate: minDate,
			EndDate:   maxDate,
		}
	}

	// Add metadata
	collection.Metadata = &models.CollectionMetadata{
		LastScraped:     scrapedData.LastUpdated,
		DataSource:      "investorgain.com",
		ScrapingSuccess: scrapedData.ScrapingSuccess,
		ErrorCount:      scrapedData.ErrorCount,
		ProcessingTime:  scrapedData.ProcessingTime.String(),
	}

	s.logger.WithFields(logrus.Fields{
		"ipo_id":        ipoID,
		"entries_found": len(scrapedData.PriceHistory),
		"errors":        scrapedData.ErrorCount,
	}).Info("IPO price history scraped successfully")

	return collection, nil
}

// ProcessingMetrics tracks detailed metrics for IPO processing
type ProcessingMetrics struct {
	TotalIPOs         int
	SuccessCount      int
	ErrorCount        int
	TotalRecordsAdded int
	ProcessingTime    time.Duration
	StartTime         time.Time
	EndTime           time.Time
	ErrorDetails      []string
}

// ProcessAllActiveIPOHistory scrapes and saves price history for all active IPOs
// Implements Requirement 4.2 - Prioritize active IPOs
// Implements Requirement 4.3 - Error isolation
// Implements Requirement 4.4 - Metrics tracking
// Returns ProcessingResults containing both successful and failed IPO processing attempts
func (s *GMPHistoryService) ProcessAllActiveIPOHistory() (*models.ProcessingResults, error) {
	startTime := time.Now()
	s.logger.Info("Processing price history for all active IPOs")

	// Create job log entry
	jobLogID := uuid.New().String()
	jobLog := &models.GMPHistoryJobLog{
		ID:              jobLogID,
		JobStartTime:    startTime,
		ExecutionStatus: models.JobStatusRunning,
		CreatedAt:       startTime,
	}

	// Initialize job log in database
	if err := s.createJobLog(jobLog); err != nil {
		s.logger.WithError(err).Warn("Failed to create job log entry, continuing without logging")
	}

	// Initialize metrics tracking (Requirement 4.4)
	metrics := &ProcessingMetrics{
		StartTime:    startTime,
		ErrorDetails: make([]string, 0),
	}

	// Initialize results
	results := &models.ProcessingResults{
		SuccessfulIPOs: make([]models.GMPPriceHistoryCollection, 0),
		FailedIPOs:     make([]models.IPOProcessingResult, 0),
	}

	// Query active and recently closed IPOs with priority-based ordering (Requirement 4.2)
	query := `
		SELECT id, company_code, name, status, close_date
		FROM ipo_list
		WHERE status IN ($2, $3, $4)
		  AND (close_date IS NULL OR close_date >= $1)
		ORDER BY 
			CASE 
				WHEN status = $2 THEN 1
				WHEN status = $3 THEN 2
				WHEN status = $4 THEN 3
				ELSE 4
			END,
			close_date DESC NULLS FIRST
		LIMIT 100
	`

	// Get IPOs from last 3 months
	threeMonthsAgo := time.Now().AddDate(0, -3, 0)
	rows, err := s.db.Query(query, threeMonthsAgo, models.StatusLive, models.StatusUpcoming, models.StatusClosed)
	if err != nil {
		metrics.EndTime = time.Now()
		metrics.ProcessingTime = metrics.EndTime.Sub(metrics.StartTime)
		s.updateJobLogOnError(jobLogID, metrics, fmt.Sprintf("Failed to query active IPOs: %v", err))
		return nil, fmt.Errorf("failed to query active IPOs: %w", err)
	}
	defer rows.Close()

	var ipos []struct {
		ID          string
		CompanyCode string
		Name        string
		Status      string
		CloseDate   *time.Time
	}

	for rows.Next() {
		var ipo struct {
			ID          string
			CompanyCode string
			Name        string
			Status      string
			CloseDate   *time.Time
		}
		err := rows.Scan(&ipo.ID, &ipo.CompanyCode, &ipo.Name, &ipo.Status, &ipo.CloseDate)
		if err != nil {
			s.logger.WithError(err).Error("Failed to scan IPO row")
			metrics.ErrorDetails = append(metrics.ErrorDetails, fmt.Sprintf("Failed to scan IPO row: %v", err))
			continue
		}
		ipos = append(ipos, ipo)
	}

	if err := rows.Err(); err != nil {
		metrics.EndTime = time.Now()
		metrics.ProcessingTime = metrics.EndTime.Sub(metrics.StartTime)
		s.updateJobLogOnError(jobLogID, metrics, fmt.Sprintf("Error iterating IPO rows: %v", err))
		return nil, fmt.Errorf("error iterating IPO rows: %w", err)
	}

	metrics.TotalIPOs = len(ipos)
	results.TotalProcessed = len(ipos)
	s.logger.WithField("ipo_count", len(ipos)).Info("Found IPOs to process")

	// Process each IPO with error isolation (Requirement 4.3)
	for i, ipo := range ipos {
		ipoStartTime := time.Now()

		s.logger.WithFields(logrus.Fields{
			"ipo_id":       ipo.ID,
			"company_code": ipo.CompanyCode,
			"ipo_name":     ipo.Name,
			"progress":     fmt.Sprintf("%d/%d", i+1, len(ipos)),
		}).Info("Processing IPO price history")

		// Scrape price history
		collection, err := s.ScrapeIPOPriceHistoryWithName(ipo.ID, ipo.CompanyCode, ipo.Name)
		if err != nil {
			// Log error but continue processing (Requirement 4.3 - Error isolation)
			errorMsg := fmt.Sprintf("IPO %s (%s): Failed to scrape - %v", ipo.Name, ipo.ID, err)
			s.logger.WithFields(logrus.Fields{
				"ipo_id": ipo.ID,
				"error":  err.Error(),
			}).Error("Failed to scrape IPO price history, continuing with next IPO")

			metrics.ErrorCount++
			metrics.ErrorDetails = append(metrics.ErrorDetails, errorMsg)

			// Add to failed IPOs list
			results.FailedIPOs = append(results.FailedIPOs, models.IPOProcessingResult{
				IPOID:          ipo.ID,
				CompanyCode:    ipo.CompanyCode,
				IPOName:        ipo.Name,
				Success:        false,
				RecordsAdded:   0,
				ErrorMessage:   err.Error(),
				ProcessingTime: time.Since(ipoStartTime),
			})
			results.FailureCount++
			continue
		}

		// Save to database
		err = s.SavePriceHistory(collection)
		if err != nil {
			// Log error but continue processing (Requirement 4.3 - Error isolation)
			errorMsg := fmt.Sprintf("IPO %s (%s): Failed to save - %v", ipo.Name, ipo.ID, err)
			s.logger.WithFields(logrus.Fields{
				"ipo_id": ipo.ID,
				"error":  err.Error(),
			}).Error("Failed to save IPO price history, continuing with next IPO")

			metrics.ErrorCount++
			metrics.ErrorDetails = append(metrics.ErrorDetails, errorMsg)

			// Add to failed IPOs list
			results.FailedIPOs = append(results.FailedIPOs, models.IPOProcessingResult{
				IPOID:          ipo.ID,
				CompanyCode:    ipo.CompanyCode,
				IPOName:        ipo.Name,
				Success:        false,
				RecordsAdded:   0,
				ErrorMessage:   err.Error(),
				ProcessingTime: time.Since(ipoStartTime),
			})
			results.FailureCount++
			continue
		}

		// Update metrics on success
		results.SuccessfulIPOs = append(results.SuccessfulIPOs, *collection)
		results.SuccessCount++
		metrics.SuccessCount++
		metrics.TotalRecordsAdded += collection.TotalRecords

		ipoProcessingTime := time.Since(ipoStartTime)
		s.logger.WithFields(logrus.Fields{
			"ipo_id":          ipo.ID,
			"entries_saved":   collection.TotalRecords,
			"processing_time": ipoProcessingTime.String(),
		}).Info("IPO price history processed successfully")
	}

	// Finalize metrics
	metrics.EndTime = time.Now()
	metrics.ProcessingTime = metrics.EndTime.Sub(metrics.StartTime)

	// Calculate success rate
	successRate := 0.0
	avgTimePerIPO := "0.00s"
	if metrics.TotalIPOs > 0 {
		successRate = float64(metrics.SuccessCount) / float64(metrics.TotalIPOs) * 100
		avgTimePerIPO = fmt.Sprintf("%.2fs", metrics.ProcessingTime.Seconds()/float64(metrics.TotalIPOs))
	}

	// Log comprehensive metrics (Requirement 4.4)
	s.logger.WithFields(logrus.Fields{
		"total_ipos":          metrics.TotalIPOs,
		"success_count":       metrics.SuccessCount,
		"error_count":         metrics.ErrorCount,
		"success_rate":        fmt.Sprintf("%.2f%%", successRate),
		"total_records_added": metrics.TotalRecordsAdded,
		"processing_time":     metrics.ProcessingTime.String(),
		"avg_time_per_ipo":    avgTimePerIPO,
	}).Info("Completed processing all active IPO price history")

	// Update job log with final status
	if err := s.updateJobLogOnSuccess(jobLogID, metrics); err != nil {
		s.logger.WithError(err).Warn("Failed to update job log with final status")
	}

	return results, nil
}

// createJobLog creates a new job log entry in the database
func (s *GMPHistoryService) createJobLog(jobLog *models.GMPHistoryJobLog) error {
	query := `
		INSERT INTO gmp_history_job_log (
			id, job_start_time, execution_status, created_at
		) VALUES ($1, $2, $3, $4)
	`

	_, err := s.db.Exec(query,
		jobLog.ID,
		jobLog.JobStartTime,
		jobLog.ExecutionStatus,
		jobLog.CreatedAt,
	)

	return err
}

// updateJobLogOnSuccess updates the job log with successful completion metrics
func (s *GMPHistoryService) updateJobLogOnSuccess(jobLogID string, metrics *ProcessingMetrics) error {
	query := `
		UPDATE gmp_history_job_log
		SET 
			job_end_time = $1,
			ipos_processed = $2,
			successful_scrapes = $3,
			failed_scrapes = $4,
			total_records_added = $5,
			execution_status = $6,
			error_summary = $7
		WHERE id = $8
	`

	errorSummary := ""
	if len(metrics.ErrorDetails) > 0 {
		// Limit error summary to first 10 errors to avoid huge text fields
		maxErrors := 10
		if len(metrics.ErrorDetails) > maxErrors {
			errorSummary = fmt.Sprintf("%s\n... and %d more errors",
				joinStrings(metrics.ErrorDetails[:maxErrors], "\n"),
				len(metrics.ErrorDetails)-maxErrors,
			)
		} else {
			errorSummary = joinStrings(metrics.ErrorDetails, "\n")
		}
	}

	status := "completed"
	if metrics.ErrorCount > 0 && metrics.SuccessCount == 0 {
		status = "failed"
	} else if metrics.ErrorCount > 0 {
		status = "completed_with_errors"
	}

	_, err := s.db.Exec(query,
		metrics.EndTime,
		metrics.TotalIPOs,
		metrics.SuccessCount,
		metrics.ErrorCount,
		metrics.TotalRecordsAdded,
		status,
		errorSummary,
		jobLogID,
	)

	return err
}

// updateJobLogOnError updates the job log when a critical error occurs
func (s *GMPHistoryService) updateJobLogOnError(jobLogID string, metrics *ProcessingMetrics, errorMsg string) {
	query := `
		UPDATE gmp_history_job_log
		SET 
			job_end_time = $1,
			ipos_processed = $2,
			successful_scrapes = $3,
			failed_scrapes = $4,
			execution_status = $5,
			error_summary = $6
		WHERE id = $7
	`

	_, err := s.db.Exec(query,
		metrics.EndTime,
		metrics.TotalIPOs,
		metrics.SuccessCount,
		metrics.ErrorCount,
		"failed",
		errorMsg,
		jobLogID,
	)

	if err != nil {
		s.logger.WithError(err).Error("Failed to update job log on error")
	}
}

// joinStrings joins a slice of strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// GetResilienceQueueMetrics returns metrics about the database resilience queue
// Useful for monitoring and health checks
func (s *GMPHistoryService) GetResilienceQueueMetrics() map[string]interface{} {
	if s.resilienceQueue == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	metrics := s.resilienceQueue.GetQueueMetrics()
	metrics["enabled"] = true
	return metrics
}

// GetResilienceQueueSize returns the current size of the resilience queue
func (s *GMPHistoryService) GetResilienceQueueSize() int {
	if s.resilienceQueue == nil {
		return 0
	}
	return s.resilienceQueue.GetQueueSize()
}

// GetArchivalStatistics returns statistics about archived and active data
func (s *GMPHistoryService) GetArchivalStatistics() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count active records and compute date bounds in one query
	var activeRecords int
	var oldestDate sql.NullTime
	var newestDate sql.NullTime
	err := s.db.QueryRow("SELECT COUNT(*), MIN(record_date), MAX(record_date) FROM gmp_price_history").Scan(&activeRecords, &oldestDate, &newestDate)
	if err != nil {
		return nil, fmt.Errorf("failed to count active records: %w", err)
	}
	stats["active_records"] = activeRecords

	// Count archived records (if archive table exists)
	var archivedRecords int
	err = s.db.QueryRow("SELECT COUNT(*) FROM gmp_price_history_archive").Scan(&archivedRecords)
	if err != nil {
		// Archive table might not exist yet
		archivedRecords = 0
	}
	stats["archived_records"] = archivedRecords

	if oldestDate.Valid {
		stats["oldest_active_date"] = oldestDate.Time.Format("2006-01-02")
		stats["oldest_active_age_days"] = int(time.Since(oldestDate.Time).Hours() / 24)
	}

	if newestDate.Valid {
		stats["newest_active_date"] = newestDate.Time.Format("2006-01-02")
	}

	// Get total archival operations count
	var archivalOpsCount int
	err = s.db.QueryRow("SELECT COUNT(*) FROM gmp_archival_log").Scan(&archivalOpsCount)
	if err == nil {
		stats["total_archival_operations"] = archivalOpsCount
	}

	// Get last archival operation
	var lastArchivalTime sql.NullTime
	var lastArchivalRecords sql.NullInt64
	err = s.db.QueryRow(`
		SELECT end_time, records_archived 
		FROM gmp_archival_log 
		WHERE archival_status = 'success' 
		ORDER BY end_time DESC 
		LIMIT 1
	`).Scan(&lastArchivalTime, &lastArchivalRecords)
	if err == nil && lastArchivalTime.Valid {
		stats["last_archival_time"] = lastArchivalTime.Time.Format("2006-01-02 15:04:05")
		if lastArchivalRecords.Valid {
			stats["last_archival_records"] = lastArchivalRecords.Int64
		}
	}

	return stats, nil
}

// InvalidateIPOCache removes cache entries for a specific IPO
// Implements Requirement 7.2 - Cache invalidation logic for data updates
func (s *GMPHistoryService) InvalidateIPOCache(ipoID string) {
	if s.cache == nil {
		return
	}

	s.logger.WithField("ipo_id", ipoID).Debug("Invalidating cache for IPO")

	// Pattern to match all cache keys for this IPO
	// Cache keys are in format: gmp_history:{ipo_id}[:start_{date}][:end_{date}]
	// Since we don't have a pattern-based delete, we'll delete the base key
	// and rely on TTL for date-range specific caches
	baseKey := fmt.Sprintf("gmp_history:%s", ipoID)
	s.cache.Delete(baseKey)

	// Also delete chart data cache if it exists
	chartKey := fmt.Sprintf("gmp_history_chart:%s", ipoID)
	s.cache.Delete(chartKey)

	s.logger.WithField("ipo_id", ipoID).Debug("Cache invalidated for IPO")
}

// InvalidateAllCache clears all GMP history cache entries
// Useful for bulk updates or maintenance operations
func (s *GMPHistoryService) InvalidateAllCache() {
	if s.cache == nil {
		return
	}

	s.logger.Info("Invalidating all GMP history cache")
	s.cache.Clear()
	s.logger.Info("All GMP history cache cleared")
}

// WarmupCache pre-loads frequently accessed IPO history data into cache
// Implements Requirement 7.2 - Cache warming for popular IPOs
func (s *GMPHistoryService) WarmupCache(ctx context.Context) error {
	if s.cache == nil {
		return fmt.Errorf("cache service not initialized")
	}

	s.logger.Info("Starting cache warmup for popular IPOs")
	startTime := time.Now()

	// Query for active and recently closed IPOs (most likely to be accessed)
	query := `
		SELECT id, company_code, name, status
		FROM ipo_list
		WHERE status IN ($2, $3, $4)
		  AND (close_date IS NULL OR close_date >= $1)
		ORDER BY 
			CASE 
				WHEN status = $2 THEN 1
				WHEN status = $3 THEN 2
				WHEN status = $4 THEN 3
				ELSE 4
			END,
			close_date DESC NULLS FIRST
		LIMIT 20
	`

	// Get IPOs from last 2 months (most popular)
	twoMonthsAgo := time.Now().AddDate(0, -2, 0)
	rows, err := s.db.QueryContext(ctx, query, twoMonthsAgo, models.StatusLive, models.StatusUpcoming, models.StatusClosed)
	if err != nil {
		return fmt.Errorf("failed to query popular IPOs: %w", err)
	}
	defer rows.Close()

	var ipos []struct {
		ID          string
		CompanyCode string
		Name        string
		Status      string
	}

	for rows.Next() {
		var ipo struct {
			ID          string
			CompanyCode string
			Name        string
			Status      string
		}
		err := rows.Scan(&ipo.ID, &ipo.CompanyCode, &ipo.Name, &ipo.Status)
		if err != nil {
			s.logger.WithError(err).Error("Failed to scan IPO row during warmup")
			continue
		}
		ipos = append(ipos, ipo)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating IPO rows during warmup: %w", err)
	}

	s.logger.WithField("ipo_count", len(ipos)).Info("Found popular IPOs for cache warmup")

	// Pre-load price history for each popular IPO
	successCount := 0
	errorCount := 0

	for _, ipo := range ipos {
		// Get price history (this will cache it)
		_, err := s.GetPriceHistoryByIPO(ipo.ID, nil)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"ipo_id":   ipo.ID,
				"ipo_name": ipo.Name,
				"error":    err.Error(),
			}).Warn("Failed to warmup cache for IPO")
			errorCount++
			continue
		}

		successCount++
		s.logger.WithFields(logrus.Fields{
			"ipo_id":   ipo.ID,
			"ipo_name": ipo.Name,
		}).Debug("Cache warmed up for IPO")
	}

	duration := time.Since(startTime)
	avgWarmupTime := "0.00ms"
	if len(ipos) > 0 {
		avgWarmupTime = fmt.Sprintf("%.2fms", duration.Seconds()*1000/float64(len(ipos)))
	}

	s.logger.WithFields(logrus.Fields{
		"total_ipos":       len(ipos),
		"success_count":    successCount,
		"error_count":      errorCount,
		"duration":         duration.String(),
		"avg_time_per_ipo": avgWarmupTime,
	}).Info("Cache warmup completed")

	if errorCount > 0 && successCount == 0 {
		return fmt.Errorf("cache warmup failed: all %d IPOs failed to load", errorCount)
	}

	return nil
}

// GetCacheStats returns statistics about the cache
func (s *GMPHistoryService) GetCacheStats() map[string]interface{} {
	if s.cache == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	return map[string]interface{}{
		"enabled": true,
		"size":    s.cache.Size(),
		"type":    "in-memory",
	}
}

// GetErrorLogger returns the error logger instance
func (s *GMPHistoryService) GetErrorLogger() *GMPHistoryErrorLogger {
	return s.errorLogger
}

// GetErrorMetrics returns current error metrics
func (s *GMPHistoryService) GetErrorMetrics() map[string]interface{} {
	if s.errorLogger == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	return s.errorLogger.GetMetricsSummary()
}

// GetCircuitBreakerMetrics returns circuit breaker metrics from the scraper
// Implements Requirement 6.4 - Circuit breaker monitoring
func (s *GMPHistoryService) GetCircuitBreakerMetrics() map[string]interface{} {
	if s.scraper == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	return s.scraper.GetCircuitBreakerMetrics()
}

// GetCircuitBreakerState returns the current state of the circuit breaker
// Useful for health checks and monitoring
func (s *GMPHistoryService) GetCircuitBreakerState() string {
	metrics := s.GetCircuitBreakerMetrics()
	if state, ok := metrics["state"].(string); ok {
		return state
	}
	return "UNKNOWN"
}

// IsServiceHealthy checks if the service is healthy and operational
// Returns true if circuit breaker is closed and resilience queue is not overloaded
// Implements Requirement 6.4 - Health checks
func (s *GMPHistoryService) IsServiceHealthy() bool {
	// Check circuit breaker state
	cbState := s.GetCircuitBreakerState()
	if cbState == "OPEN" {
		return false
	}

	// Check resilience queue size
	queueSize := s.GetResilienceQueueSize()
	if queueSize > 100 {
		return false
	}

	return true
}
