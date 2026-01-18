package handlers

import (
	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/gofiber/fiber/v2"
)

type MarketHandler struct{}

func NewMarketHandler() *MarketHandler {
	return &MarketHandler{}
}

// GetMarketIndices returns current market indices with mock data
func (h *MarketHandler) GetMarketIndices(c *fiber.Ctx) error {
	// Mock data for market indices
	indices := []models.MarketIndex{
		{
			ID:            "nifty50",
			Name:          "NIFTY 50",
			Value:         21453.95,
			Change:        125.30,
			ChangePercent: 0.59,
			IsPositive:    true,
		},
		{
			ID:            "sensex",
			Name:          "SENSEX",
			Value:         71315.09,
			Change:        418.75,
			ChangePercent: 0.59,
			IsPositive:    true,
		},
		{
			ID:            "banknifty",
			Name:          "BANK NIFTY",
			Value:         45892.35,
			Change:        -89.45,
			ChangePercent: -0.19,
			IsPositive:    false,
		},
		{
			ID:            "niftymidcap",
			Name:          "NIFTY MIDCAP 100",
			Value:         48765.20,
			Change:        234.80,
			ChangePercent: 0.48,
			IsPositive:    true,
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    indices,
	})
}
