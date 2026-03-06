package handlers

import (
	"time"

	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/gofiber/fiber/v2"
)

// V2AdminHandler wraps admin endpoints with v2 response envelope
type V2AdminHandler struct {
	LegacyHandler *AdminHandler
}

// NewV2AdminHandler creates a new V2 admin handler
func NewV2AdminHandler(h *AdminHandler) *V2AdminHandler {
	return &V2AdminHandler{LegacyHandler: h}
}

// TriggerGMPUpdate wraps the v1 GMP update trigger in a v2 response envelope
func (h *V2AdminHandler) TriggerGMPUpdate(c *fiber.Ctx) error {
	if h.LegacyHandler.GMPJob == nil {
		return c.Status(503).JSON(shared.NewV2ErrorResponse("SERVICE_UNAVAILABLE", "GMP update job unavailable", nil))
	}

	startTime := time.Now()

	// Run the GMP update job
	h.LegacyHandler.GMPJob.Run()

	duration := time.Since(startTime)

	data := map[string]interface{}{
		"message":  "GMP update job completed",
		"duration": duration.String(),
	}

	return c.JSON(shared.NewV2Response(data))
}

// TriggerDailyUpdate wraps the v1 daily update trigger in a v2 response envelope
func (h *V2AdminHandler) TriggerDailyUpdate(c *fiber.Ctx) error {
	if h.LegacyHandler.DailyJob == nil {
		return c.Status(503).JSON(shared.NewV2ErrorResponse("SERVICE_UNAVAILABLE", "Daily update job unavailable", nil))
	}

	startTime := time.Now()

	// Run the daily IPO update job
	h.LegacyHandler.DailyJob.Run()

	duration := time.Since(startTime)

	data := map[string]interface{}{
		"message":  "Daily IPO update job completed",
		"duration": duration.String(),
	}

	return c.JSON(shared.NewV2Response(data))
}

// TriggerGMPHistoryUpdate wraps the v1 GMP history update trigger in a v2 response envelope
func (h *V2AdminHandler) TriggerGMPHistoryUpdate(c *fiber.Ctx) error {
	if h.LegacyHandler.GMPHistoryJob == nil {
		return c.Status(503).JSON(shared.NewV2ErrorResponse("SERVICE_UNAVAILABLE", "GMP history update job unavailable", nil))
	}

	startTime := time.Now()

	// Run the GMP history update job
	h.LegacyHandler.GMPHistoryJob.Run()

	duration := time.Since(startTime)

	data := map[string]interface{}{
		"message":  "GMP history update job completed",
		"duration": duration.String(),
	}

	return c.JSON(shared.NewV2Response(data))
}

// GetGMPHistoryJobStatus wraps the v1 job status in a v2 response envelope
func (h *V2AdminHandler) GetGMPHistoryJobStatus(c *fiber.Ctx) error {
	if h.LegacyHandler.GMPHistoryJob == nil {
		return c.Status(503).JSON(shared.NewV2ErrorResponse("SERVICE_UNAVAILABLE", "GMP history update job unavailable", nil))
	}

	status := h.LegacyHandler.GMPHistoryJob.GetJobStatus()

	return c.JSON(shared.NewV2Response(status))
}

// GetGMPHistoryJobMetrics wraps the v1 job metrics in a v2 response envelope
func (h *V2AdminHandler) GetGMPHistoryJobMetrics(c *fiber.Ctx) error {
	if h.LegacyHandler.GMPHistoryJob == nil {
		return c.Status(503).JSON(shared.NewV2ErrorResponse("SERVICE_UNAVAILABLE", "GMP history update job unavailable", nil))
	}

	metrics := h.LegacyHandler.GMPHistoryJob.GetLastRunMetrics()

	if metrics == nil {
		return c.JSON(shared.NewV2Response(map[string]interface{}{
			"message": "No metrics available - job has not run yet",
			"data":    nil,
		}))
	}

	data := map[string]interface{}{
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
	}

	return c.JSON(shared.NewV2Response(data))
}

// CreateIPO wraps the v1 IPO creation in a v2 response envelope
func (h *V2AdminHandler) CreateIPO(c *fiber.Ctx) error {
	if err := h.LegacyHandler.CreateIPO(c); err != nil {
		return err
	}
	return nil
}

// GetGMPData wraps the v1 GMP data retrieval in a v2 response envelope
func (h *V2AdminHandler) GetGMPData(c *fiber.Ctx) error {
	if err := h.LegacyHandler.GetGMPData(c); err != nil {
		return err
	}
	return nil
}

// TriggerRegistrarResolve wraps the v1 registrar code resolution in a v2 response envelope
func (h *V2AdminHandler) TriggerRegistrarResolve(c *fiber.Ctx) error {
	if h.LegacyHandler.RegistrarCodeService == nil {
		return c.Status(503).JSON(shared.NewV2ErrorResponse("SERVICE_UNAVAILABLE", "Registrar code service unavailable", nil))
	}

	start := time.Now()
	codes, err := h.LegacyHandler.RegistrarCodeService.GetUnresolvedForToday(c.Context())
	if err != nil {
		return c.Status(500).JSON(shared.NewV2ErrorResponse("INTERNAL_ERROR", err.Error(), nil))
	}

	resolved := 0
	for _, code := range codes {
		if code.IPOName == nil {
			continue
		}
		_, err := h.LegacyHandler.RegistrarCodeService.ResolveCode(c.Context(), code.IPOID, code.RegistrarShortCode, *code.IPOName)
		if err == nil {
			resolved++
		}
	}

	data := map[string]interface{}{
		"resolved_count": resolved,
		"total":          len(codes),
		"duration":       time.Since(start).String(),
	}

	return c.JSON(shared.NewV2Response(data))
}
