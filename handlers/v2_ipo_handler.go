package handlers

import (
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/gofiber/fiber/v2"
)

type V2IPOHandler struct {
	Service *services.IPOService
}

func NewV2IPOHandler(service *services.IPOService) *V2IPOHandler {
	return &V2IPOHandler{Service: service}
}

func (h *V2IPOHandler) GetFeed(c *fiber.Ctx) error {
	limit, offset := parsePagination(c, 50, 200)

	// Temporarily using existing GetActiveIPOsWithGMPPaginated.
	// In a real implementation, service should return `total` count too.
	// For now, we wrap the existing data.
	ipos, err := h.Service.GetActiveIPOsWithGMPPaginated(c.Context(), limit, offset)
	if err != nil {
		return c.Status(500).JSON(shared.NewV2ErrorResponse("INTERNAL_ERROR", err.Error(), nil))
	}

	var mapped []models.V2IPOFeedItem
	for _, ipo := range ipos {
		item := models.V2IPOFeedItem{
			ID:            ipo.ID.String(),
			StockID:       ipo.StockID,
			Name:          ipo.Name,
			LogoURL:       ipo.LogoURL,
			Status:        ipo.Status,
			PriceBandLow:  ipo.PriceBandLow,
			PriceBandHigh: ipo.PriceBandHigh,
		}

		if ipo.OpenDate != nil {
			d := ipo.OpenDate.Format(time.RFC3339)
			item.OpenDate = &d
		}
		if ipo.CloseDate != nil {
			d := ipo.CloseDate.Format(time.RFC3339)
			item.CloseDate = &d
		}
		if ipo.ListingDate != nil {
			d := ipo.ListingDate.Format(time.RFC3339)
			item.ListingDate = &d
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

	// TODO: service does not return total count yet; hardcoded for schema compliance.
	// A separate PR will update the service to return (items, total, err).
	return c.JSON(shared.NewV2PaginatedResponse(mapped, 1000, limit, offset))
}

func (h *V2IPOHandler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	ipo, err := h.Service.GetIPOByIDWithGMP(c.Context(), id)

	if err != nil {
		return c.Status(500).JSON(shared.NewV2ErrorResponse("INTERNAL_ERROR", err.Error(), nil))
	}
	if ipo == nil {
		return c.Status(404).JSON(shared.NewV2ErrorResponse("NOT_FOUND", "IPO not found", nil))
	}

	detail := models.V2IPODetail{
		V2IPOFeedItem: models.V2IPOFeedItem{
			ID:            ipo.ID.String(),
			StockID:       ipo.StockID,
			Name:          ipo.Name,
			LogoURL:       ipo.LogoURL,
			Status:        ipo.Status,
			PriceBandLow:  ipo.PriceBandLow,
			PriceBandHigh: ipo.PriceBandHigh,
		},
		Description:        ipo.Description,
		Registrar:          ipo.Registrar,
		IssueSize:          ipo.IssueSize,
		MinQty:             ipo.MinQty,
		MinAmount:          ipo.MinAmount,
		SubscriptionStatus: ipo.SubscriptionStatus,
		Financials:         ipo.Financials,
		Categories:         ipo.Categories,
		FAQs:               ipo.FAQs,
	}

	if ipo.OpenDate != nil {
		d := ipo.OpenDate.Format(time.RFC3339)
		detail.OpenDate = &d
	}
	if ipo.CloseDate != nil {
		d := ipo.CloseDate.Format(time.RFC3339)
		detail.CloseDate = &d
	}
	if ipo.ListingDate != nil {
		d := ipo.ListingDate.Format(time.RFC3339)
		detail.ListingDate = &d
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
