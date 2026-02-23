package handlers

import (
	"regexp"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type V2AllotmentHandler struct {
	IPOService       *services.IPOService
	AllotmentChecker *services.AllotmentChecker
	CacheService     *services.CacheService
}

func NewV2AllotmentHandler(ipo *services.IPOService, allotmentChecker *services.AllotmentChecker, cache *services.CacheService) *V2AllotmentHandler {
	return &V2AllotmentHandler{
		IPOService:       ipo,
		AllotmentChecker: allotmentChecker,
		CacheService:     cache,
	}
}

type V2AllotmentRequest struct {
	IPOID string `json:"ipo_id"`
	PAN   string `json:"pan"`
}

type V2AllotmentResponse struct {
	Status         string `json:"status"`
	SharesApplied  int    `json:"shares_applied"`
	SharesAllotted int    `json:"shares_allotted"`
	Message        string `json:"message"`
}

var panRegex = regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]$`)

func (h *V2AllotmentHandler) CheckAllotment(c *fiber.Ctx) error {
	var req V2AllotmentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(shared.NewV2ErrorResponse("VALIDATION_ERROR", "Invalid request body", nil))
	}

	// Validate IPO ID
	if _, err := uuid.Parse(req.IPOID); err != nil {
		return c.Status(400).JSON(shared.NewV2ErrorResponse("VALIDATION_ERROR", "Invalid IPO ID format", map[string]string{"field": "ipo_id", "expected": "valid UUID"}))
	}

	// Validate PAN format
	if !panRegex.MatchString(req.PAN) {
		return c.Status(422).JSON(shared.NewV2ErrorResponse("VALIDATION_ERROR", "Invalid PAN format", map[string]string{"field": "pan", "expected": "ABCDE1234F format"}))
	}

	// Get IPO Details
	ipo, err := h.IPOService.GetIPOByID(c.Context(), req.IPOID)
	if err != nil {
		return c.Status(500).JSON(shared.NewV2ErrorResponse("INTERNAL_ERROR", "Failed to fetch IPO details", nil))
	}
	if ipo == nil {
		return c.Status(404).JSON(shared.NewV2ErrorResponse("NOT_FOUND", "IPO not found", map[string]string{"field": "ipo_id"}))
	}

	// Check Allotment Status
	status, sharesAllotted, err := h.AllotmentChecker.CheckAllotmentStatus(c.Context(), ipo, req.PAN)
	if err != nil {
		return c.Status(502).JSON(shared.NewV2ErrorResponse("BAD_GATEWAY", "Failed to check allotment status", nil))
	}

	// Build response message
	message := getAllotmentMessage(status)

	sharesApplied := 0
	if ipo.MinQty != nil {
		sharesApplied = *ipo.MinQty
	}

	response := V2AllotmentResponse{
		Status:         status,
		SharesApplied:  sharesApplied,
		SharesAllotted: sharesAllotted,
		Message:        message,
	}

	// Cache result (fire and forget)
	result := models.IPOResultCache{
		PanHash:        req.PAN,
		IPOID:          ipo.ID,
		Status:         status,
		SharesAllotted: sharesAllotted,
		Source:         "v2_check",
		Timestamp:      time.Now(),
	}
	go h.CacheService.StoreResult(c.Context(), &result)

	return c.JSON(shared.NewV2Response(response))
}

func getAllotmentMessage(status string) string {
	switch status {
	case "ALLOTTED":
		return "Congratulations! Shares have been allotted."
	case "NOT_ALLOTTED":
		return "Sorry, no shares were allotted."
	case "PENDING":
		return "Allotment result is yet to be declared."
	default:
		return "Unable to determine allotment status."
	}
}
