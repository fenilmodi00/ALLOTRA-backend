package handlers

import (
	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
)

type CacheHandler struct {
	Service *services.CacheService
}

func NewCacheHandler(service *services.CacheService) *CacheHandler {
	return &CacheHandler{Service: service}
}

func (h *CacheHandler) StoreResult(c *fiber.Ctx) error {
	var result models.IPOResultCache
	if err := c.BodyParser(&result); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if err := h.Service.StoreResult(c.Context(), &result); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Result cached successfully",
	})
}

func (h *CacheHandler) GetCachedResult(c *fiber.Ctx) error {
	ipoID := c.Params("ipo_id")
	panHash := c.Params("pan_hash")

	result, err := h.Service.GetCachedResult(c.Context(), ipoID, panHash)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}
	if result == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "No cached result found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}
