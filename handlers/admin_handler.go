package handlers

import (
	"context"
	"database/sql"
	"time"

	"github.com/fenilmodi00/ipo-backend/jobs"
	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

type IPOAdminService interface {
	CreateIPO(ctx context.Context, ipo *models.IPO) error
}

type GMPJobRunner interface {
	Run()
}

type GMPHistoryJobRunner interface {
	Run()
	GetJobStatus() map[string]interface{}
	GetLastRunMetrics() *jobs.JobMetrics
}

type AdminHandler struct {
	DB            *sql.DB
	IPOService    IPOAdminService
	GMPJob        GMPJobRunner
	GMPHistoryJob GMPHistoryJobRunner
}

func NewAdminHandler(db *sql.DB, ipoService IPOAdminService, gmpJob GMPJobRunner, gmpHistoryJob GMPHistoryJobRunner) *AdminHandler {
	return &AdminHandler{
		DB:            db,
		IPOService:    ipoService,
		GMPJob:        gmpJob,
		GMPHistoryJob: gmpHistoryJob,
	}
}

func (h *AdminHandler) CreateIPO(c *fiber.Ctx) error {
	if h.IPOService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "IPO service unavailable",
		})
	}

	var ipo models.IPO
	if err := c.BodyParser(&ipo); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if err := h.IPOService.CreateIPO(c.Context(), &ipo); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    ipo,
	})
}

// TriggerGMPUpdate manually triggers the GMP update job
func (h *AdminHandler) TriggerGMPUpdate(c *fiber.Ctx) error {
	if h.GMPJob == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "GMP update job unavailable",
		})
	}

	logrus.Info("Manual GMP update triggered via admin endpoint")

	startTime := time.Now()

	// Run the GMP update job
	h.GMPJob.Run()

	duration := time.Since(startTime)

	return c.JSON(fiber.Map{
		"success":   true,
		"message":   "GMP update job completed",
		"duration":  duration.String(),
		"timestamp": time.Now(),
	})
}

// GetGMPData returns all GMP data in the database for debugging
func (h *AdminHandler) GetGMPData(c *fiber.Ctx) error {
	if h.DB == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "Database unavailable",
		})
	}

	query := `
		SELECT ipo_name, company_code, gmp_value, gain_percent, estimated_listing, last_updated
		FROM ipo_gmp 
		ORDER BY last_updated DESC
		LIMIT 20
	`

	rows, err := h.DB.QueryContext(c.Context(), query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to query GMP data: " + err.Error(),
		})
	}
	defer rows.Close()

	var gmpData []map[string]interface{}
	for rows.Next() {
		var ipoName, companyCode string
		var gmpValue, gainPercent, estimatedListing float64
		var lastUpdated time.Time

		err := rows.Scan(&ipoName, &companyCode, &gmpValue, &gainPercent, &estimatedListing, &lastUpdated)
		if err != nil {
			continue
		}

		gmpData = append(gmpData, map[string]interface{}{
			"ipo_name":          ipoName,
			"company_code":      companyCode,
			"gmp_value":         gmpValue,
			"gain_percent":      gainPercent,
			"estimated_listing": estimatedListing,
			"last_updated":      lastUpdated,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    gmpData,
		"count":   len(gmpData),
	})
}

// TriggerGMPHistoryUpdate manually triggers the GMP history update job
func (h *AdminHandler) TriggerGMPHistoryUpdate(c *fiber.Ctx) error {
	if h.GMPHistoryJob == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "GMP history update job unavailable",
		})
	}

	logrus.Info("Manual GMP history update triggered via admin endpoint")

	startTime := time.Now()

	// Run the GMP history update job
	h.GMPHistoryJob.Run()

	duration := time.Since(startTime)

	return c.JSON(fiber.Map{
		"success":   true,
		"message":   "GMP history update job completed",
		"duration":  duration.String(),
		"timestamp": time.Now(),
	})
}

// GetGMPHistoryJobStatus returns the status and metrics of the GMP history job
func (h *AdminHandler) GetGMPHistoryJobStatus(c *fiber.Ctx) error {
	if h.GMPHistoryJob == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "GMP history update job unavailable",
		})
	}

	status := h.GMPHistoryJob.GetJobStatus()

	return c.JSON(fiber.Map{
		"success": true,
		"data":    status,
	})
}

// GetGMPHistoryJobMetrics returns detailed metrics from the last job run
func (h *AdminHandler) GetGMPHistoryJobMetrics(c *fiber.Ctx) error {
	if h.GMPHistoryJob == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"error":   "GMP history update job unavailable",
		})
	}

	metrics := h.GMPHistoryJob.GetLastRunMetrics()

	if metrics == nil {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "No metrics available - job has not run yet",
			"data":    nil,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": map[string]interface{}{
			"job_start_time":          metrics.JobStartTime,
			"job_end_time":            metrics.JobEndTime,
			"duration":                metrics.Duration.String(),
			"total_ipos":              metrics.TotalIPOs,
			"successful_ipos":         metrics.SuccessfulIPOs,
			"failed_ipos":             metrics.FailedIPOs,
			"total_records_added":     metrics.TotalRecordsAdded,
			"avg_records_per_ipo":     metrics.AvgRecordsPerIPO,
			"avg_processing_time_ipo": metrics.AvgProcessingTimeIPO.String(),
			"success_rate":            metrics.SuccessRate,
			"queue_size_before":       metrics.QueueSizeBefore,
			"queue_size_after":        metrics.QueueSizeAfter,
			"queue_items_processed":   metrics.QueueItemsProcessed,
			"error_summary":           metrics.ErrorSummary,
		},
	})
}
