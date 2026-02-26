package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
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

	panHash := hashPAN(req.PAN)

	// 1. Check Cache First
	cached, _ := h.CacheService.GetCachedResult(c.Context(), req.IPOID, panHash)
	if cached != nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true,
			"data":    cached,
			"source":  "cache",
		})
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
	// Use context.Background() for the goroutine to avoid issues with request context cancellation
	go h.CacheService.StoreResult(context.Background(), &result)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// hashPAN generates a SHA-256 hash of the PAN for privacy
func hashPAN(pan string) string {
	hash := sha256.Sum256([]byte(pan))
	return hex.EncodeToString(hash[:])
}
