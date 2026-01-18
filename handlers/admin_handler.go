package handlers

import (
	"time"

	"github.com/fenilmodi00/ipo-backend/jobs"
	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

type AdminHandler struct {
	IPOService *services.IPOService
	GMPJob     *jobs.GMPUpdateJob
}

func NewAdminHandler(ipoService *services.IPOService, gmpJob *jobs.GMPUpdateJob) *AdminHandler {
	return &AdminHandler{
		IPOService: ipoService,
		GMPJob:     gmpJob,
	}
}

func (h *AdminHandler) CreateIPO(c *fiber.Ctx) error {
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
	query := `
		SELECT ipo_name, company_code, gmp_value, gain_percent, estimated_listing, last_updated
		FROM ipo_gmp 
		ORDER BY last_updated DESC
		LIMIT 20
	`

	rows, err := h.IPOService.DB.QueryContext(c.Context(), query)
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
