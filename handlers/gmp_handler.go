package handlers

import (
	"database/sql"
	"encoding/json"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type GMPHandler struct {
	DB *sql.DB
}

func NewGMPHandler(db *sql.DB) *GMPHandler {
	return &GMPHandler{DB: db}
}

// GetGMPByIPO retrieves GMP data for a specific IPO
func (h *GMPHandler) GetGMPByIPO(c *fiber.Ctx) error {
	ipoID := c.Params("id")

	// Validate UUID format
	if _, err := uuid.Parse(ipoID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid IPO ID format",
		})
	}

	// First, get the IPO details to find the stock_id and company_code
	var stockID *string
	var companyCode string
	err := h.DB.QueryRow(`
		SELECT stock_id, company_code FROM ipo_list WHERE id = $1
	`, ipoID).Scan(&stockID, &companyCode)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "IPO not found",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Database error",
		})
	}

	// Now query enhanced GMP data using stock_id as primary key, company_code as fallback
	var gmpData models.EnhancedGMPData
	var extractionMetadataBytes sql.NullString
	var query string
	var args []interface{}

	if stockID != nil && *stockID != "" {
		// Use stock_id as primary linking key
		query = `
			SELECT id, ipo_name, company_code, ipo_price, gmp_value, 
			       estimated_listing, gain_percent, sub2, kostak, last_updated,
			       stock_id, subscription_status, listing_gain, ipo_status, 
			       data_source, extraction_metadata
			FROM ipo_gmp 
			WHERE (stock_id = $1 OR company_code = $2)
			ORDER BY 
				CASE WHEN stock_id = $1 THEN 1 ELSE 2 END,
				last_updated DESC
			LIMIT 1
		`
		args = []interface{}{*stockID, companyCode}
	} else {
		// Fallback to company_code matching only
		query = `
			SELECT id, ipo_name, company_code, ipo_price, gmp_value, 
			       estimated_listing, gain_percent, sub2, kostak, last_updated,
			       stock_id, subscription_status, listing_gain, ipo_status, 
			       data_source, extraction_metadata
			FROM ipo_gmp 
			WHERE company_code = $1
			ORDER BY last_updated DESC
			LIMIT 1
		`
		args = []interface{}{companyCode}
	}

	err = h.DB.QueryRow(query, args...).Scan(
		&gmpData.ID,
		&gmpData.IPOName,
		&gmpData.CompanyCode,
		&gmpData.IPOPrice,
		&gmpData.GMPValue,
		&gmpData.EstimatedListing,
		&gmpData.GainPercent,
		&gmpData.Sub2,
		&gmpData.Kostak,
		&gmpData.LastUpdated,
		&gmpData.StockID,
		&gmpData.SubscriptionStatus,
		&gmpData.ListingGain,
		&gmpData.IPOStatus,
		&gmpData.DataSource,
		&extractionMetadataBytes,
	)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "GMP data not found for this IPO",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to fetch GMP data",
		})
	}

	// Parse extraction metadata JSON if present
	if extractionMetadataBytes.Valid && extractionMetadataBytes.String != "" {
		var metadata models.ExtractionMetadata
		if err := json.Unmarshal([]byte(extractionMetadataBytes.String), &metadata); err == nil {
			gmpData.ExtractionMetadata = &metadata
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    gmpData,
	})
}
