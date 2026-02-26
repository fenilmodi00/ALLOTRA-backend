package app

import (
	"net/http/httptest"
	"testing"

	"github.com/fenilmodi00/ipo-backend/config"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

// TestCORSConfiguration verifies that the CORS middleware respects the configuration
func TestCORSConfiguration(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name           string
		allowedOrigins string
		requestOrigin  string
		expectedOrigin string
	}{
		{
			name:           "Allows all origins when set to *",
			allowedOrigins: "*",
			requestOrigin:  "http://example.com",
			expectedOrigin: "*",
		},
		{
			name:           "Allows specific trusted origin",
			allowedOrigins: "http://trusted.com",
			requestOrigin:  "http://trusted.com",
			expectedOrigin: "http://trusted.com",
		},
		{
			name:           "Rejects untrusted origin",
			allowedOrigins: "http://trusted.com",
			requestOrigin:  "http://untrusted.com",
			expectedOrigin: "",
		},
		{
			name:           "Allows multiple trusted origins - first",
			allowedOrigins: "http://trusted1.com, http://trusted2.com",
			requestOrigin:  "http://trusted1.com",
			expectedOrigin: "http://trusted1.com",
		},
		{
			name:           "Allows multiple trusted origins - second",
			allowedOrigins: "http://trusted1.com, http://trusted2.com",
			requestOrigin:  "http://trusted2.com",
			expectedOrigin: "http://trusted2.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new Fiber app
			app := fiber.New()

			// Mock config
			cfg := &config.Config{
				AllowedOrigins: tc.allowedOrigins,
			}

			// Register middleware using the unexported function
			registerMiddleware(app, cfg)

			// Register a dummy route
			app.Get("/", func(c *fiber.Ctx) error {
				return c.SendString("OK")
			})

			// Create request
			req := httptest.NewRequest("GET", "/", nil)
			if tc.requestOrigin != "" {
				req.Header.Set("Origin", tc.requestOrigin)
			}

			// Execute request
			resp, err := app.Test(req)
			assert.NoError(t, err)

			// Verify Access-Control-Allow-Origin header
			actualOrigin := resp.Header.Get("Access-Control-Allow-Origin")
			assert.Equal(t, tc.expectedOrigin, actualOrigin)
		})
	}
}
