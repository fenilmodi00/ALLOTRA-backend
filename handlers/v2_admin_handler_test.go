package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestV2AdminError(t *testing.T) {
	app := fiber.New()
	app.Post("/api/v2/admin/gmp/update", func(c *fiber.Ctx) error {
		return c.Status(401).JSON(shared.NewV2ErrorResponse("UNAUTHORIZED", "Missing token", nil))
	})

	req := httptest.NewRequest("POST", "/api/v2/admin/gmp/update", nil)
	resp, _ := app.Test(req)

	assert.Equal(t, 401, resp.StatusCode)
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	assert.False(t, body["success"].(bool))
}
