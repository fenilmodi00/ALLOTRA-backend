package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type V2IPOHandler struct {
	Service *services.IPOService
}

func NewV2IPOHandler(service *services.IPOService) *V2IPOHandler {
	return &V2IPOHandler{Service: service}
}

func computeCategory(issueSize *string) string {
	if issueSize == nil || *issueSize == "" {
		return "mainboard"
	}
	sizeStr := strings.ReplaceAll(*issueSize, ",", "")
	var sizeValue float64
	_, err := fmt.Sscanf(sizeStr, "%f", &sizeValue)
	if err != nil {
		return "mainboard"
	}
	if sizeValue > 100000 {
		sizeValue = sizeValue / 10000000
	}
	if sizeValue > 0 && sizeValue < 25 {
		return "sme"
	}
	return "mainboard"
}

func formatDatePtr(t *time.Time) *string {
	if t == nil {
		tba := "TBA"
		return &tba
	}
	d := t.Format(time.RFC3339)
	return &d
}

func (h *V2IPOHandler) GetFeed(c *fiber.Ctx) error {
	limit, offset := parsePagination(c, 50, 200)
	statusFilter := c.Query("status", "all")

	ipos, total, err := h.Service.GetActiveIPOsWithGMPPaginatedWithCount(c.Context(), statusFilter, limit, offset)
	if err != nil {
		return c.Status(500).JSON(shared.NewV2ErrorResponse("INTERNAL_ERROR", "Failed to fetch IPOs", nil))
	}

	var mapped []models.V2IPOFeedItem
	for _, ipo := range ipos {
		item := models.V2IPOFeedItem{
			ID:            ipo.ID.String(),
			StockID:       ipo.StockID,
			Name:          ipo.Name,
			LogoURL:       ipo.LogoURL,
			Status:        ipo.Status,
			Category:      computeCategory(ipo.IssueSize),
			PriceBandLow:  ipo.PriceBandLow,
			PriceBandHigh: ipo.PriceBandHigh,
			OpenDate:      formatDatePtr(ipo.OpenDate),
			CloseDate:     formatDatePtr(ipo.CloseDate),
			ListingDate:   formatDatePtr(ipo.ListingDate),
		}

		if ipo.GMPValue != nil {
			item.GMP = &models.V2GMPNested{
				Value:              ipo.GMPValue,
				GainPercent:        ipo.GainPercent,
				EstimatedListing:   ipo.EstimatedListing,
				SubscriptionStatus: ipo.GMPSubscriptionStatus,
			}
		}
		mapped = append(mapped, item)
	}

	return c.JSON(shared.NewV2PaginatedResponse(mapped, total, limit, offset))
}

func (h *V2IPOHandler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")

	// Validate UUID format
	if _, err := uuid.Parse(id); err != nil {
		return c.Status(400).JSON(shared.NewV2ErrorResponse("VALIDATION_ERROR", "Invalid IPO ID format", map[string]string{"field": "id", "expected": "valid UUID"}))
	}

	ipo, err := h.Service.GetIPOByIDWithGMP(c.Context(), id)

	if err != nil {
		return c.Status(500).JSON(shared.NewV2ErrorResponse("INTERNAL_ERROR", "Failed to fetch IPO", nil))
	}
	if ipo == nil {
		return c.Status(404).JSON(shared.NewV2ErrorResponse("NOT_FOUND", "IPO not found", nil))
	}

	// Calculate min investment
	var minInvestment *float64
	if ipo.MinAmount != nil {
		m := float64(*ipo.MinAmount)
		minInvestment = &m
	}

	detail := models.V2IPODetail{
		V2IPOFeedItem: models.V2IPOFeedItem{
			ID:            ipo.ID.String(),
			StockID:       ipo.StockID,
			Name:          ipo.Name,
			LogoURL:       ipo.LogoURL,
			Status:        ipo.Status,
			Category:      computeCategory(ipo.IssueSize),
			PriceBandLow:  ipo.PriceBandLow,
			PriceBandHigh: ipo.PriceBandHigh,
			OpenDate:      formatDatePtr(ipo.OpenDate),
			CloseDate:     formatDatePtr(ipo.CloseDate),
			ListingDate:   formatDatePtr(ipo.ListingDate),
		},
		Description:        ipo.Description,
		Registrar:          ipo.Registrar,
		IssueSize:          ipo.IssueSize,
		MinQty:             ipo.MinQty,
		MinAmount:          ipo.MinAmount,
		MinInvestment:      minInvestment,
		SubscriptionStatus: ipo.SubscriptionStatus,
		Financials:         ipo.Financials,
		Categories:         ipo.Categories,
		FAQs:               ipo.FAQs,
		AllotmentDate:      formatDatePtr(ipo.ResultDate),
	}

	if ipo.GMPValue != nil {
		detail.GMP = &models.V2GMPNested{
			Value:              ipo.GMPValue,
			GainPercent:        ipo.GainPercent,
			EstimatedListing:   ipo.EstimatedListing,
			SubscriptionStatus: ipo.GMPSubscriptionStatus,
		}
	}

	return c.JSON(shared.NewV2Response(detail))
}
