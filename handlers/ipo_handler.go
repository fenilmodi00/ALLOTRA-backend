package handlers

import (
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
)

type IPOHandler struct {
	Service *services.IPOService
}

func NewIPOHandler(service *services.IPOService) *IPOHandler {
	return &IPOHandler{Service: service}
}

func (h *IPOHandler) GetIPOs(c *fiber.Ctx) error {
	status := c.Query("status", "all")
	ipos, err := h.Service.GetIPOs(c.Context(), status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    ipos,
	})
}

func (h *IPOHandler) GetActiveIPOs(c *fiber.Ctx) error {
	ipos, err := h.Service.GetActiveIPOs(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    ipos,
	})
}

func (h *IPOHandler) GetIPOFormConfig(c *fiber.Ctx) error {
	id := c.Params("ipo_id")
	ipo, err := h.Service.GetIPOByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}
	if ipo == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "IPO not found",
		})
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    ipo,
	})
}

func (h *IPOHandler) GetIPOByID(c *fiber.Ctx) error {
	id := c.Params("id")
	ipo, err := h.Service.GetIPOByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}
	if ipo == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "IPO not found",
		})
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    ipo,
	})
}

// GetActiveIPOsWithGMP returns active IPOs with GMP data joined by company_code
func (h *IPOHandler) GetActiveIPOsWithGMP(c *fiber.Ctx) error {
	ipos, err := h.Service.GetActiveIPOsWithGMP(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    ipos,
	})
}

// GetIPOByIDWithGMP returns a single IPO with GMP data joined by company_code
func (h *IPOHandler) GetIPOByIDWithGMP(c *fiber.Ctx) error {
	id := c.Params("id")
	ipo, err := h.Service.GetIPOByIDWithGMP(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}
	if ipo == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "IPO not found",
		})
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    ipo,
	})
}
