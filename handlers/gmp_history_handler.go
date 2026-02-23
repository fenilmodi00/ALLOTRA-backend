package handlers

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// GMPHistoryHandler handles HTTP requests for GMP price history endpoints
type GMPHistoryHandler struct {
	Service *services.GMPHistoryService
}

// NewGMPHistoryHandler creates a new GMP history handler instance
func NewGMPHistoryHandler(service *services.GMPHistoryService) *GMPHistoryHandler {
	return &GMPHistoryHandler{
		Service: service,
	}
}

// handleServiceError provides graceful degradation for service errors
// Implements Requirement 6.4 - Graceful degradation for service failures
func (h *GMPHistoryHandler) handleServiceError(c *fiber.Ctx, err error, ipoID string) error {
	if errors.Is(err, services.ErrPriceHistoryNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "No price history found for this IPO",
			"message": fmt.Sprintf("No historical GMP data exists for IPO ID: %s", ipoID),
		})
	}

	if errors.Is(err, services.ErrServiceUnavailable) {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "Service temporarily unavailable",
			"message": "The GMP history service is not ready. Please try again later.",
		})
	}

	// Graceful degradation: Check if service is degraded (Requirement 6.4)
	if h.Service != nil && !h.Service.IsServiceHealthy() {
		cbState := h.Service.GetCircuitBreakerState()
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "Service temporarily unavailable",
			"message": "The GMP history service is currently experiencing issues. Please try again later.",
			"details": fiber.Map{
				"circuit_breaker_state": cbState,
				"retry_after":           "60 seconds",
				"reason":                "External service unavailable or circuit breaker open",
			},
		})
	}

	// Generic internal error
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"success": false,
		"error":   "Failed to retrieve price history",
		"message": "An internal error occurred while fetching price history data",
	})
}

// GetIPOPriceHistory retrieves complete price history for a specific IPO
// GET /api/gmp/history/{identifier}
// The identifier can be either:
//   - IPO UUID (e.g., d9d0343d-d727-49cf-aa9d-1189c0ecbb3a)
//   - Stock ID (e.g., 2462)
//
// Returns all historical GMP data (typically 10-15 days) without pagination
// Implements Requirements 3.1, 3.3, 3.5
func (h *GMPHistoryHandler) GetIPOPriceHistory(c *fiber.Ctx) error {
	identifier := c.Params("ipo_id")
	if h.Service == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "Service not initialized",
			"message": "GMP history service is unavailable",
		})
	}

	if !isValidIPOIdentifier(identifier) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid identifier",
			"message": "The provided identifier must be either a valid IPO UUID or numeric stock ID",
		})
	}

	if err := validateDateRangeQuery(c); err != nil {
		return err
	}

	// Try to resolve identifier to IPO ID (UUID)
	ipoID, err := h.Service.ResolveIPOIdentifier(identifier)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid identifier",
			"message": "The provided identifier must be either a valid IPO UUID or stock ID",
		})
	}

	// Get complete price history from service (no date range filtering)
	history, err := h.Service.GetPriceHistoryByIPO(ipoID, nil)
	if err != nil {
		return h.handleServiceError(c, err, ipoID)
	}

	// Clean up entries - remove fields that should be in metadata
	cleanedEntries := make([]map[string]any, len(history.Entries))
	for i, entry := range history.Entries {
		cleanedEntries[i] = map[string]any{
			"id":                entry.ID,
			"ipo_id":            entry.IPOID,
			"company_code":      entry.CompanyCode,
			"record_date":       entry.RecordDate,
			"ipo_price":         entry.IPOPrice,
			"gmp_value":         entry.GMPValue,
			"estimated_listing": entry.EstimatedListing,
			"listing_percent":   entry.ListingPercent,
			"last_updated":      entry.LastUpdated,
		}
	}

	// Build metadata with moved fields (Requirement 3.5)
	metadata := map[string]any{
		"data_source":  "investorgain.com",
		"last_updated": time.Now().Format(time.RFC3339),
	}

	if history.Metadata != nil {
		metadata["last_scraped"] = history.Metadata.LastScraped.Format(time.RFC3339)
		metadata["scraping_success"] = history.Metadata.ScrapingSuccess
		if history.Metadata.ProcessingTime != "" {
			metadata["processing_time"] = history.Metadata.ProcessingTime
		}
	}

	// Add fields moved from entries to metadata
	if len(history.Entries) > 0 {
		firstEntry := history.Entries[0]
		metadata["estimated_profit"] = firstEntry.EstimatedProfit
		metadata["subscription_status"] = firstEntry.SubscriptionStatus
		metadata["sub2_sauda"] = firstEntry.Sub2Sauda
		metadata["created_at"] = firstEntry.CreatedAt.Format(time.RFC3339)
		metadata["updated_at"] = firstEntry.UpdatedAt.Format(time.RFC3339)
	}

	// Return cleaned response (Requirements 3.1, 3.5)
	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"ipo_id":        history.IPOID,
			"ipo_name":      history.IPOName,
			"company_code":  history.CompanyCode,
			"total_records": len(history.Entries),
			"entries":       cleanedEntries,
			"metadata":      metadata,
		},
	})
}

// GetChartData retrieves price history formatted for chart visualization
// GET /api/gmp/history/{identifier}/chart
// The identifier can be either:
//   - IPO UUID (e.g., d9d0343d-d727-49cf-aa9d-1189c0ecbb3a)
//   - Stock ID (e.g., 2462)
//
// Returns all historical GMP data formatted for charts (typically 10-15 days)
// Implements Requirements 3.1, 3.5
func (h *GMPHistoryHandler) GetChartData(c *fiber.Ctx) error {
	identifier := c.Params("ipo_id")
	if h.Service == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "Service not initialized",
			"message": "GMP history service is unavailable",
		})
	}

	if !isValidIPOIdentifier(identifier) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid identifier",
			"message": "The provided identifier must be either a valid IPO UUID or numeric stock ID",
		})
	}

	if err := validateDateRangeQuery(c); err != nil {
		return err
	}

	// Try to resolve identifier to IPO ID (UUID)
	ipoID, err := h.Service.ResolveIPOIdentifier(identifier)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid identifier",
			"message": "The provided identifier must be either a valid IPO UUID or stock ID",
		})
	}

	// Get complete price history from service (no date range filtering)
	history, err := h.Service.GetPriceHistoryByIPO(ipoID, nil)
	if err != nil {
		return h.handleServiceError(c, err, ipoID)
	}

	// Transform to chart data format (Requirement 3.1)
	chartResponse := transformToChartData(history)

	// Enhance metadata (Requirement 3.5)
	chartResponse.Metadata.TotalRecords = len(history.Entries)

	// Also fetch current GMP from ipo_gmp table for consistency with /with-gmp endpoint
	if h.Service != nil && h.Service.DB != nil {
		currentGMP, err := h.Service.GetCurrentGMP(ipoID)
		if err == nil && currentGMP != nil {
			chartResponse.CurrentGMP = &models.CurrentGMP{
				GMPValue:           currentGMP.GMPValue,
				GainPercent:        currentGMP.GainPercent,
				EstimatedListing:   currentGMP.EstimatedListing,
				LastUpdated:        currentGMP.LastUpdated,
				StockID:            currentGMP.StockID,
				SubscriptionStatus: currentGMP.SubscriptionStatus,
				IPOStatus:          currentGMP.IPOStatus,
			}
			chartResponse.Metadata.GMPDataSource = "ipo_gmp"
		}
	}

	// Return chart-optimized response (Requirements 3.1, 3.5)
	return c.JSON(fiber.Map{
		"success": true,
		"data":    chartResponse,
	})
}

// GetHistorySummary retrieves a summary overview of price history for an IPO
// GET /api/gmp/history/{identifier}/summary
// The identifier can be either:
//   - IPO UUID (e.g., d9d0343d-d727-49cf-aa9d-1189c0ecbb3a)
//   - Stock ID (e.g., 2462)
//
// Implements Requirements 3.1, 3.5
func (h *GMPHistoryHandler) GetHistorySummary(c *fiber.Ctx) error {
	identifier := c.Params("ipo_id")
	if h.Service == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "Service not initialized",
			"message": "GMP history service is unavailable",
		})
	}

	if !isValidIPOIdentifier(identifier) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid identifier",
			"message": "The provided identifier must be either a valid IPO UUID or numeric stock ID",
		})
	}

	if err := validateDateRangeQuery(c); err != nil {
		return err
	}

	// Try to resolve identifier to IPO ID (UUID)
	ipoID, err := h.Service.ResolveIPOIdentifier(identifier)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid identifier",
			"message": "The provided identifier must be either a valid IPO UUID or stock ID",
		})
	}

	// Get full price history (no date range filtering for summary)
	history, err := h.Service.GetPriceHistoryByIPO(ipoID, nil)
	if err != nil {
		return h.handleServiceError(c, err, ipoID)
	}

	// Calculate summary statistics
	summary := calculateSummaryStatistics(history)

	// Build simple metadata (Requirement 3.5)
	metadata := map[string]any{
		"data_source":   "investorgain.com",
		"last_updated":  time.Now().Format(time.RFC3339),
		"total_records": history.TotalRecords,
	}

	if history.Metadata != nil {
		metadata["last_scraped"] = history.Metadata.LastScraped.Format(time.RFC3339)
		metadata["scraping_success"] = history.Metadata.ScrapingSuccess
		metadata["processing_time"] = history.Metadata.ProcessingTime
	}

	// Return summary response with metadata (Requirement 3.5)
	return c.JSON(fiber.Map{
		"success":  true,
		"data":     summary,
		"metadata": metadata,
	})
}

// transformToChartData converts price history collection to chart-optimized format
// Implements Requirement 3.1 - Chart-optimized JSON format
func transformToChartData(history *models.GMPPriceHistoryCollection) *models.ChartDataResponse {
	// Build chart points
	chartPoints := make([]models.ChartPoint, 0, len(history.Entries))
	for _, entry := range history.Entries {
		chartPoints = append(chartPoints, models.ChartPoint{
			Date:             entry.RecordDate.Format("2006-01-02"),
			GMPValue:         entry.GMPValue,
			IPOPrice:         entry.IPOPrice,
			EstimatedListing: entry.EstimatedListing,
			ListingPercent:   entry.ListingPercent,
		})
	}

	// Calculate statistics
	statistics := calculateStatistics(history.Entries)

	// Build IPO basic info
	ipoInfo := models.IPOBasicInfo{
		IPOID:       history.IPOID,
		IPOName:     history.IPOName,
		CompanyCode: history.CompanyCode,
	}

	// Get IPO price from first entry (all entries should have same IPO price)
	if len(history.Entries) > 0 {
		ipoInfo.IPOPrice = history.Entries[0].IPOPrice
	}

	// Build metadata (Requirement 3.5)
	metadata := models.ChartMetadata{
		DataSource:   "investorgain.com",
		LastUpdated:  time.Now(),
		TotalRecords: history.TotalRecords,
	}

	if history.DateRange != nil {
		metadata.DateRangeStart = history.DateRange.StartDate.Format("2006-01-02")
		metadata.DateRangeEnd = history.DateRange.EndDate.Format("2006-01-02")
	}

	return &models.ChartDataResponse{
		IPOInfo:    ipoInfo,
		ChartData:  chartPoints,
		Statistics: statistics,
		Metadata:   metadata,
	}
}

// calculateStatistics computes statistical summary from price history entries
func calculateStatistics(entries []models.GMPPriceHistoryEntry) models.ChartStatistics {
	if len(entries) == 0 {
		return models.ChartStatistics{
			TrendDirection: "stable",
		}
	}

	// Initialize with first entry
	maxGMP := entries[0].GMPValue
	minGMP := entries[0].GMPValue
	sumGMP := 0.0

	// Calculate min, max, and sum
	for _, entry := range entries {
		if entry.GMPValue > maxGMP {
			maxGMP = entry.GMPValue
		}
		if entry.GMPValue < minGMP {
			minGMP = entry.GMPValue
		}
		sumGMP += entry.GMPValue
	}

	// Calculate average
	avgGMP := sumGMP / float64(len(entries))

	// Latest GMP is from the most recent entry (entries are ordered DESC by date)
	latestGMP := entries[0].GMPValue

	// Determine trend direction
	trendDirection := "stable"
	if len(entries) >= 2 {
		// Compare latest with previous
		previousGMP := entries[1].GMPValue
		diff := latestGMP - previousGMP
		threshold := 5.0 // 5 rupees threshold for trend detection

		if diff > threshold {
			trendDirection = "up"
		} else if diff < -threshold {
			trendDirection = "down"
		}
	}

	return models.ChartStatistics{
		MaxGMP:         maxGMP,
		MinGMP:         minGMP,
		AverageGMP:     avgGMP,
		LatestGMP:      latestGMP,
		TrendDirection: trendDirection,
	}
}

// calculateSummaryStatistics creates a summary overview of price history
func calculateSummaryStatistics(history *models.GMPPriceHistoryCollection) map[string]any {
	statistics := calculateStatistics(history.Entries)

	summary := map[string]any{
		"ipo_id":        history.IPOID,
		"ipo_name":      history.IPOName,
		"company_code":  history.CompanyCode,
		"total_records": history.TotalRecords,
		"statistics":    statistics,
	}

	// Add date range if available
	if history.DateRange != nil {
		summary["date_range"] = map[string]string{
			"start": history.DateRange.StartDate.Format("2006-01-02"),
			"end":   history.DateRange.EndDate.Format("2006-01-02"),
		}
	}

	// Add metadata
	if history.Metadata != nil {
		summary["metadata"] = map[string]any{
			"data_source":      history.Metadata.DataSource,
			"last_scraped":     history.Metadata.LastScraped.Format(time.RFC3339),
			"scraping_success": history.Metadata.ScrapingSuccess,
		}
	}

	// Add recent entries (last 5)
	recentCount := 5
	if len(history.Entries) < recentCount {
		recentCount = len(history.Entries)
	}

	recentEntries := make([]map[string]any, recentCount)
	for i := 0; i < recentCount; i++ {
		entry := history.Entries[i]
		recentEntries[i] = map[string]any{
			"date":              entry.RecordDate.Format("2006-01-02"),
			"gmp_value":         entry.GMPValue,
			"estimated_listing": entry.EstimatedListing,
			"listing_percent":   entry.ListingPercent,
		}
	}
	summary["recent_entries"] = recentEntries

	return summary
}

// GetHealthCheck provides health status of the GMP history service
// GET /api/gmp/history/health
// Implements Requirement 6.4 - Health checks and service monitoring
func (h *GMPHistoryHandler) GetHealthCheck(c *fiber.Ctx) error {
	if h.Service == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"status":  "unhealthy",
			"message": "Service not initialized",
		})
	}

	// Get circuit breaker metrics from scraper
	circuitBreakerMetrics := h.Service.GetCircuitBreakerMetrics()

	// Get resilience queue metrics
	resilienceMetrics := h.Service.GetResilienceQueueMetrics()

	// Get cache statistics
	cacheStats := h.Service.GetCacheStats()

	// Error metrics no longer available (error logger removed)
	errorMetrics := map[string]interface{}{"enabled": false}

	// Determine overall health status
	status := "healthy"
	circuitState := "unknown"
	if cbState, ok := circuitBreakerMetrics["state"].(string); ok {
		circuitState = cbState
		if cbState == "OPEN" {
			status = "degraded"
		} else if cbState == "HALF_OPEN" {
			status = "recovering"
		}
	}

	// Check resilience queue size
	queueSize := 0
	if size, ok := resilienceMetrics["queue_size"].(int); ok {
		queueSize = size
		if size > 100 {
			status = "degraded"
		}
	}

	// Build health response
	healthResponse := fiber.Map{
		"success":   true,
		"status":    status,
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "gmp-price-history",
		"components": fiber.Map{
			"circuit_breaker": fiber.Map{
				"status":  circuitState,
				"metrics": circuitBreakerMetrics,
			},
			"resilience_queue": fiber.Map{
				"status":     "operational",
				"queue_size": queueSize,
				"metrics":    resilienceMetrics,
			},
			"cache": fiber.Map{
				"status":  "operational",
				"metrics": cacheStats,
			},
			"error_tracking": fiber.Map{
				"status":  "operational",
				"metrics": errorMetrics,
			},
		},
	}

	// Set appropriate HTTP status code based on health
	httpStatus := fiber.StatusOK
	if status == "degraded" {
		httpStatus = fiber.StatusServiceUnavailable
	} else if status == "recovering" {
		httpStatus = fiber.StatusOK // Still accepting requests in half-open state
	}

	return c.Status(httpStatus).JSON(healthResponse)
}

// GetServiceMetrics provides detailed metrics about the service
// GET /api/gmp/history/metrics
// Implements Requirement 6.4 - Service monitoring
func (h *GMPHistoryHandler) GetServiceMetrics(c *fiber.Ctx) error {
	if h.Service == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "Service not initialized",
		})
	}

	// Gather all metrics
	metrics := fiber.Map{
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "gmp-price-history",
		"circuit_breaker": fiber.Map{
			"metrics": h.Service.GetCircuitBreakerMetrics(),
		},
		"resilience_queue": fiber.Map{
			"metrics": h.Service.GetResilienceQueueMetrics(),
		},
		"cache": fiber.Map{
			"metrics": h.Service.GetCacheStats(),
		},
		"errors": fiber.Map{
			"metrics": map[string]interface{}{"enabled": false},
		},
	}

	// Get archival statistics if available
	archivalStats, err := h.Service.GetArchivalStatistics()
	if err == nil {
		metrics["archival"] = archivalStats
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    metrics,
	})
}

// BackfillGMPHistory triggers a backfill of GMP history for all active IPOs
// POST /api/v1/gmp/backfill
// This endpoint initiates a background job to scrape GMP history for all active IPOs
func (h *GMPHistoryHandler) BackfillGMPHistory(c *fiber.Ctx) error {
	if h.Service == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "Service not initialized",
			"message": "GMP history service is unavailable",
		})
	}

	jobID := uuid.New().String()

	// Start the backfill process in a goroutine to not block the response
	go func() {
		results, err := h.Service.ProcessAllActiveIPOHistory()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"operation": "backfill",
				"job_id":    jobID,
			}).WithError(err).Error("Backfill job failed")
			return
		}

		logrus.WithFields(logrus.Fields{
			"operation":       "backfill_complete",
			"job_id":          jobID,
			"total_processed": results.TotalProcessed,
			"success_count":   results.SuccessCount,
			"failure_count":   results.FailureCount,
		}).Info("Backfill job completed")
	}()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Backfill job started",
		"data": fiber.Map{
			"job_id":  jobID,
			"status":  "started",
			"message": "GMP history backfill has been initiated for all active IPOs",
			"endpoints": fiber.Map{
				"health":  "/api/v1/gmp/history/health",
				"metrics": "/api/v1/gmp/history/metrics",
			},
		},
	})
}

func isValidIPOIdentifier(identifier string) bool {
	if identifier == "" {
		return false
	}

	if _, err := strconv.Atoi(identifier); err == nil {
		return true
	}

	_, err := uuid.Parse(identifier)
	return err == nil
}

func validateDateRangeQuery(c *fiber.Ctx) error {
	startDateRaw := c.Query("start_date")
	endDateRaw := c.Query("end_date")

	if startDateRaw == "" && endDateRaw == "" {
		return nil
	}

	var startDate time.Time
	if startDateRaw != "" {
		parsedStart, err := time.Parse("2006-01-02", startDateRaw)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid start_date format. Use YYYY-MM-DD",
				"message": "start_date must use YYYY-MM-DD format",
			})
		}
		startDate = parsedStart
	}

	if endDateRaw != "" {
		endDate, err := time.Parse("2006-01-02", endDateRaw)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid end_date format. Use YYYY-MM-DD",
				"message": "end_date must use YYYY-MM-DD format",
			})
		}
		if !startDate.IsZero() && startDate.After(endDate) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid date range",
				"message": "start_date cannot be after end_date",
			})
		}
	}

	return nil
}
