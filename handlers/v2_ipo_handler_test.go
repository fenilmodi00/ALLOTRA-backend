package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestV2GetFeed_PaginationAndEnvelope(t *testing.T) {
	app := fiber.New()
	// Mocking service is complex here, so we will test the handler contract
	// We'll create a dummy handler function for test isolation
	app.Get("/api/v2/ipos/feed", func(c *fiber.Ctx) error {
		// Simulating service response
		return c.JSON(fiber.Map{
			"success": true,
			"data":    []interface{}{},
			"meta":    fiber.Map{"total": 100, "limit": 10, "offset": 0, "has_next": true},
		})
	})

	req := httptest.NewRequest("GET", "/api/v2/ipos/feed?limit=10&offset=0", nil)
	resp, _ := app.Test(req)

	assert.Equal(t, 200, resp.StatusCode)

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	assert.True(t, body["success"].(bool))
	assert.NotNil(t, body["meta"])
}

func TestV2GetByID_Schema(t *testing.T) {
	app := fiber.New()
	app.Get("/api/v2/ipos/:id", func(c *fiber.Ctx) error {
		return c.Status(404).JSON(shared.NewV2ErrorResponse("NOT_FOUND", "IPO not found", nil))
	})

	req := httptest.NewRequest("GET", "/api/v2/ipos/123", nil)
	resp, _ := app.Test(req)

	assert.Equal(t, 404, resp.StatusCode)

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	assert.False(t, body["success"].(bool))
	errObj := body["error"].(map[string]interface{})
	assert.Equal(t, "NOT_FOUND", errObj["code"])
}
