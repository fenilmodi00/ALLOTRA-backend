package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

type CheckHandler struct {
	IPOService       *services.IPOService
	AllotmentChecker *services.AllotmentChecker
	CacheService     *services.CacheService
}

func NewCheckHandler(ipo *services.IPOService, allotmentChecker *services.AllotmentChecker, cache *services.CacheService) *CheckHandler {
	return &CheckHandler{
		IPOService:       ipo,
		AllotmentChecker: allotmentChecker,
		CacheService:     cache,
	}
}

func (h *CheckHandler) CheckAllotment(c *fiber.Ctx) error {
	type Request struct {
		IPOID string `json:"ipo_id"`
		PAN   string `json:"pan"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	// Hash PAN for privacy/security before any storage or logging
	hash := sha256.Sum256([]byte(req.PAN))
	panHash := hex.EncodeToString(hash[:])

	// 1. Check Cache First
	if h.CacheService != nil {
		cached, err := h.CacheService.GetCachedResult(c.Context(), req.IPOID, panHash)
		if err == nil && cached != nil {
			return c.JSON(fiber.Map{
				"success": true,
				"data":    cached,
			})
		}
	}

	// 2. Get IPO Details
	ipo, err := h.IPOService.GetIPOByID(c.Context(), req.IPOID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ipo == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "IPO not found"})
	}

	// 3. Check Allotment Status
	status, shares, err := h.AllotmentChecker.CheckAllotmentStatus(c.Context(), ipo, req.PAN)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "Failed to check status: " + err.Error()})
	}

	// 4. Cache Result
	result := models.IPOResultCache{
		PanHash:        panHash,
		IPOID:          ipo.ID,
		Status:         status,
		SharesAllotted: shares,
		Source:         "live_check",
		Timestamp:      time.Now(),
	}

	if h.CacheService != nil {
		// Use a detached context for caching so it doesn't get cancelled if request ends
		go func(res models.IPOResultCache) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := h.CacheService.StoreResult(ctx, &res); err != nil {
				logrus.WithError(err).Warn("Failed to cache allotment result")
			}
		}(result)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}
