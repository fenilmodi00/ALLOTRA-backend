package handlers

import (
	"regexp"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/fenilmodi00/ipo-backend/tools/registrars"
	"github.com/fenilmodi00/ipo-backend/tools/registrars/bigshare"
	"github.com/fenilmodi00/ipo-backend/tools/registrars/kfin"
	"github.com/fenilmodi00/ipo-backend/tools/registrars/mufg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type V2AllotmentHandler struct {
	IPOService           *services.IPOService
	AllotmentChecker     *services.AllotmentChecker
	CacheService         *services.CacheService
	RegistrarCodeService *services.RegistrarCodeService
}

func NewV2AllotmentHandler(ipo *services.IPOService, allotmentChecker *services.AllotmentChecker, cache *services.CacheService, registrarCodeService *services.RegistrarCodeService) *V2AllotmentHandler {
	return &V2AllotmentHandler{
		IPOService:           ipo,
		AllotmentChecker:     allotmentChecker,
		CacheService:         cache,
		RegistrarCodeService: registrarCodeService,
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

	// Determine registrar short code
	registrarShortCode := ""
	if ipo.Registrar != "" {
		client := registrars.GetClient(ipo.Registrar)
		if client != nil {
			// Map client to short code (KFIN, BIGSHARE, MUFG)
			switch client.(type) {
			case *kfin.Client:
				registrarShortCode = "KFIN"
			case *bigshare.Client:
				registrarShortCode = "BIGSHARE"
			case *mufg.Client:
				registrarShortCode = "MUFG"
			}
		}
	}

	if registrarShortCode == "" {
		return c.Status(503).JSON(shared.NewV2ErrorResponse("SERVICE_UNAVAILABLE", "Registrar not supported", nil))
	}

	// Try to get resolved code
	resolvedCode, err := h.RegistrarCodeService.GetResolvedCode(c.Context(), ipo.ID, registrarShortCode)
	if err != nil || resolvedCode == nil {
		// Attempt live resolution
		resolvedCode, err = h.RegistrarCodeService.ResolveCode(c.Context(), ipo.ID, registrarShortCode, ipo.Name)
		if err != nil || resolvedCode == nil || !resolvedCode.IsResolved {
			return c.Status(503).JSON(shared.NewV2ErrorResponse("SERVICE_UNAVAILABLE", "Company code not yet resolved", nil))
		}
	}

	if resolvedCode.RegistrarCompanyCode == nil || *resolvedCode.RegistrarCompanyCode == "" {
		resolvedCode, err = h.RegistrarCodeService.ResolveCode(c.Context(), ipo.ID, registrarShortCode, ipo.Name)
		if err != nil || resolvedCode == nil || !resolvedCode.IsResolved || resolvedCode.RegistrarCompanyCode == nil || *resolvedCode.RegistrarCompanyCode == "" {
			return c.Status(503).JSON(shared.NewV2ErrorResponse("SERVICE_UNAVAILABLE", "Company code not yet resolved", nil))
		}
	}

	client := registrars.GetClient(registrarShortCode)
	if client == nil {
		return c.Status(503).JSON(shared.NewV2ErrorResponse("SERVICE_UNAVAILABLE", "Unsupported registrar: "+registrarShortCode, nil))
	}

	allotmentResult, err := client.CheckAllotment(c.Context(), *resolvedCode.RegistrarCompanyCode, req.PAN)
	if err != nil {
		return c.Status(502).JSON(shared.NewV2ErrorResponse("BAD_GATEWAY", "Failed to check allotment status", nil))
	}
	if allotmentResult == nil {
		return c.Status(502).JSON(shared.NewV2ErrorResponse("BAD_GATEWAY", "Failed to check allotment status", nil))
	}

	status := allotmentResult.Status
	sharesApplied := allotmentResult.SharesApplied
	sharesAllotted := allotmentResult.SharesAllotted

	// Build response message
	message := getAllotmentMessage(status)

	response := V2AllotmentResponse{
		Status:         status,
		SharesApplied:  sharesApplied,
		SharesAllotted: sharesAllotted,
		Message:        message,
	}

	// Cache result (fire and forget)
	result := models.IPOResultCache{
		PanHash:           req.PAN,
		IPOID:             ipo.ID,
		Status:            status,
		SharesAllotted:    sharesAllotted,
		ApplicationNumber: allotmentResult.ApplicationNo,
		Source:            "v2_check",
		Timestamp:         time.Now(),
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
