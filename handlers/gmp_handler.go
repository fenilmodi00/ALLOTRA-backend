package handlers

import (
	"database/sql"
	"fmt"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/repositories"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// GMPHistoryServiceInterface defines the interface for history service operations
type GMPHistoryServiceInterface interface {
	GetPriceHistoryByIPO(ipoID string, dateRange *models.DateRange) (*models.GMPPriceHistoryCollection, error)
}

type GMPHandler struct {
	Repo           repositories.GMPRepository
	HistoryService GMPHistoryServiceInterface
}

func NewGMPHandler(db *sql.DB, historyService *services.GMPHistoryService) *GMPHandler {
	return &GMPHandler{
		Repo:           repositories.NewSQLGMPRepository(db),
		HistoryService: historyService,
	}
}

// GetGMPByIPO retrieves GMP data for a specific IPO
func (h *GMPHandler) GetGMPByIPO(c *fiber.Ctx) error {
	ipoID := c.Params("id")

	// Validate UUID format
	if _, err := uuid.Parse(ipoID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid IPO ID format",
		})
	}

	gmpData, err := h.Repo.GetByIPOID(c.Context(), ipoID)
	if err == repositories.ErrNotFound {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "GMP data not found for this IPO",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch GMP data",
		})
	}

	// Build response with history links and summary (Requirement 8.1)
	response := fiber.Map{
		"success": true,
		"data":    gmpData,
	}

	// Add history information if history service is available
	if h.HistoryService != nil {
		historyInfo := h.buildHistoryInfo(ipoID)
		if historyInfo != nil {
			response["history"] = historyInfo
		}
	}

	return c.JSON(response)
}

// buildHistoryInfo creates history links and summary for the GMP response
// Implements Requirement 8.1 - Add history data links to existing GMP API responses
func (h *GMPHandler) buildHistoryInfo(ipoID string) fiber.Map {
	// Return nil if history service is not available (backward compatibility)
	if h.HistoryService == nil {
		return nil
	}

	// Check if history data exists for this IPO
	history, err := h.HistoryService.GetPriceHistoryByIPO(ipoID, nil)

	// Build base history info with links
	historyInfo := fiber.Map{
		"available": false,
		"links": fiber.Map{
			"full_history": fmt.Sprintf("/api/gmp/history/%s", ipoID),
			"chart_data":   fmt.Sprintf("/api/gmp/history/%s/chart", ipoID),
			"summary":      fmt.Sprintf("/api/gmp/history/%s/summary", ipoID),
		},
	}

	// If history data exists, add summary information
	if err == nil && history != nil && len(history.Entries) > 0 {
		historyInfo["available"] = true
		historyInfo["total_records"] = history.TotalRecords

		// Add date range
		if history.DateRange != nil {
			historyInfo["date_range"] = fiber.Map{
				"start": history.DateRange.StartDate.Format("2006-01-02"),
				"end":   history.DateRange.EndDate.Format("2006-01-02"),
			}
		}

		// Calculate and add summary statistics
		summary := h.calculateHistorySummary(history)
		if summary != nil {
			historyInfo["summary"] = summary
		}
	}

	return historyInfo
}

// calculateHistorySummary generates a brief summary of historical GMP data
// Includes trend direction, recent changes, and key statistics
func (h *GMPHandler) calculateHistorySummary(history *models.GMPPriceHistoryCollection) fiber.Map {
	if history == nil || len(history.Entries) == 0 {
		return nil
	}

	entries := history.Entries

	// Calculate basic statistics
	maxGMP := entries[0].GMPValue
	minGMP := entries[0].GMPValue
	sumGMP := 0.0

	for _, entry := range entries {
		if entry.GMPValue > maxGMP {
			maxGMP = entry.GMPValue
		}
		if entry.GMPValue < minGMP {
			minGMP = entry.GMPValue
		}
		sumGMP += entry.GMPValue
	}

	avgGMP := sumGMP / float64(len(entries))

	// Latest GMP is from the most recent entry (entries are ordered DESC by date)
	latestGMP := entries[0].GMPValue
	latestDate := entries[0].RecordDate.Format("2006-01-02")

	// Determine trend direction (7-day trend if available)
	trendDirection := "stable"
	trendChange := 0.0

	if len(entries) >= 2 {
		// Compare latest with entry from 7 days ago (or oldest if less than 7 days)
		compareIndex := 1
		if len(entries) > 7 {
			compareIndex = 7
		} else {
			compareIndex = len(entries) - 1
		}

		previousGMP := entries[compareIndex].GMPValue
		trendChange = latestGMP - previousGMP
		threshold := 5.0 // 5 rupees threshold for trend detection

		if trendChange > threshold {
			trendDirection = "up"
		} else if trendChange < -threshold {
			trendDirection = "down"
		}
	}

	// Calculate percentage change
	trendChangePercent := 0.0
	if len(entries) >= 2 {
		compareIndex := 1
		if len(entries) > 7 {
			compareIndex = 7
		} else {
			compareIndex = len(entries) - 1
		}
		previousGMP := entries[compareIndex].GMPValue
		if previousGMP > 0 {
			trendChangePercent = ((latestGMP - previousGMP) / previousGMP) * 100
		}
	}

	summary := fiber.Map{
		"latest_gmp":           latestGMP,
		"latest_date":          latestDate,
		"trend_direction":      trendDirection,
		"trend_change":         trendChange,
		"trend_change_percent": trendChangePercent,
		"max_gmp":              maxGMP,
		"min_gmp":              minGMP,
		"average_gmp":          avgGMP,
	}

	// Add recent high/low information
	if len(entries) >= 7 {
		// Calculate 7-day high/low
		sevenDayHigh := entries[0].GMPValue
		sevenDayLow := entries[0].GMPValue

		for i := 0; i < 7 && i < len(entries); i++ {
			if entries[i].GMPValue > sevenDayHigh {
				sevenDayHigh = entries[i].GMPValue
			}
			if entries[i].GMPValue < sevenDayLow {
				sevenDayLow = entries[i].GMPValue
			}
		}

		summary["seven_day_high"] = sevenDayHigh
		summary["seven_day_low"] = sevenDayLow
	}

	return summary
}
