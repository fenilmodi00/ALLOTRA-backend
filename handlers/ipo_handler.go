package handlers

import (
	"strconv"

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
	limit, offset := parsePagination(c, 50, 200)
	ipos, err := h.Service.GetIPOsWithOptimizedQuery(c.Context(), status, limit, offset)
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
	limit, offset := parsePagination(c, 50, 200)
	ipos, err := h.Service.GetActiveIPOsPaginated(c.Context(), limit, offset)
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
	limit, offset := parsePagination(c, 50, 200)
	ipos, err := h.Service.GetActiveIPOsWithGMPPaginated(c.Context(), "", limit, offset)
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

func parsePagination(c *fiber.Ctx, defaultLimit, maxLimit int) (int, int) {
	limit := defaultLimit
	offset := 0

	if rawLimit := c.Query("limit"); rawLimit != "" {
		if parsedLimit, err := strconv.Atoi(rawLimit); err == nil {
			if parsedLimit > 0 {
				limit = parsedLimit
			}
		}
	}

	if rawOffset := c.Query("offset"); rawOffset != "" {
		if parsedOffset, err := strconv.Atoi(rawOffset); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	if limit > maxLimit {
		limit = maxLimit
	}

	return limit, offset
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
