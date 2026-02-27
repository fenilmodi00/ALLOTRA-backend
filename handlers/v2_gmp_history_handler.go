package handlers

import (
	"strconv"

	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type V2GMPHistoryHandler struct {
	LegacyHandler *GMPHistoryHandler
}

func NewV2GMPHistoryHandler(h *GMPHistoryHandler) *V2GMPHistoryHandler {
	return &V2GMPHistoryHandler{LegacyHandler: h}
}

// GetChartData wraps the v1 chart logic in a v2 response envelope
func (h *V2GMPHistoryHandler) GetChartData(c *fiber.Ctx) error {
	identifier := c.Params("ipo_id")
	if identifier == "" {
		return c.Status(400).JSON(shared.NewV2ErrorResponse("BAD_REQUEST", "IPO ID is required", nil))
	}

	if !isValidIPOIdentifierV2(identifier) {
		return c.Status(400).JSON(shared.NewV2ErrorResponse("BAD_REQUEST", "Invalid IPO identifier. Must be a valid UUID or numeric stock ID", nil))
	}

	// Resolve identifier to IPO ID (UUID)
	ipoID, err := h.LegacyHandler.Service.ResolveIPOIdentifier(identifier)
	if err != nil {
		return c.Status(400).JSON(shared.NewV2ErrorResponse("BAD_REQUEST", "Invalid IPO identifier", nil))
	}

	chartData, err := h.LegacyHandler.Service.GetPriceHistoryByIPO(ipoID, nil)
	if err != nil {
		return c.Status(500).JSON(shared.NewV2ErrorResponse("INTERNAL_ERROR", "Failed to get chart data", nil))
	}

	// Transform to chart data format (Requirement 3.1)
	chartResponse := transformToChartData(chartData)

	return c.JSON(shared.NewV2Response(chartResponse))
}

// GetIPOPriceHistory wraps the v1 price history in a v2 response envelope
func (h *V2GMPHistoryHandler) GetIPOPriceHistory(c *fiber.Ctx) error {
	identifier := c.Params("ipo_id")
	if identifier == "" {
		return c.Status(400).JSON(shared.NewV2ErrorResponse("BAD_REQUEST", "IPO ID is required", nil))
	}

	if !isValidIPOIdentifierV2(identifier) {
		return c.Status(400).JSON(shared.NewV2ErrorResponse("BAD_REQUEST", "Invalid IPO identifier. Must be a valid UUID or numeric stock ID", nil))
	}

	// Resolve identifier to IPO ID (UUID)
	ipoID, err := h.LegacyHandler.Service.ResolveIPOIdentifier(identifier)
	if err != nil {
		return c.Status(400).JSON(shared.NewV2ErrorResponse("BAD_REQUEST", "Invalid IPO identifier", nil))
	}

	history, err := h.LegacyHandler.Service.GetPriceHistoryByIPO(ipoID, nil)
	if err != nil {
		return c.Status(500).JSON(shared.NewV2ErrorResponse("INTERNAL_ERROR", "Failed to get price history", nil))
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

	data := map[string]any{
		"ipo_id":        history.IPOID,
		"ipo_name":      history.IPOName,
		"company_code":  history.CompanyCode,
		"total_records": len(history.Entries),
		"entries":       cleanedEntries,
	}

	return c.JSON(shared.NewV2Response(data))
}

// GetHistorySummary wraps the v1 summary in a v2 response envelope
func (h *V2GMPHistoryHandler) GetHistorySummary(c *fiber.Ctx) error {
	identifier := c.Params("ipo_id")
	if identifier == "" {
		return c.Status(400).JSON(shared.NewV2ErrorResponse("BAD_REQUEST", "IPO ID is required", nil))
	}

	if !isValidIPOIdentifierV2(identifier) {
		return c.Status(400).JSON(shared.NewV2ErrorResponse("BAD_REQUEST", "Invalid IPO identifier. Must be a valid UUID or numeric stock ID", nil))
	}

	// Resolve identifier to IPO ID (UUID)
	ipoID, err := h.LegacyHandler.Service.ResolveIPOIdentifier(identifier)
	if err != nil {
		return c.Status(400).JSON(shared.NewV2ErrorResponse("BAD_REQUEST", "Invalid IPO identifier", nil))
	}

	history, err := h.LegacyHandler.Service.GetPriceHistoryByIPO(ipoID, nil)
	if err != nil {
		return c.Status(500).JSON(shared.NewV2ErrorResponse("INTERNAL_ERROR", "Failed to get history summary", nil))
	}

	// Calculate summary statistics
	summary := calculateSummaryStatistics(history)

	return c.JSON(shared.NewV2Response(summary))
}

func isValidIPOIdentifierV2(identifier string) bool {
	if identifier == "" {
		return false
	}
	if _, err := strconv.Atoi(identifier); err == nil {
		return true
	}
	_, err := uuid.Parse(identifier)
	return err == nil
}
