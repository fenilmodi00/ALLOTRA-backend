package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
)

type SQLGMPRepository struct {
	db *sql.DB
}

func NewSQLGMPRepository(db *sql.DB) *SQLGMPRepository {
	return &SQLGMPRepository{db: db}
}

func (r *SQLGMPRepository) GetByIPOID(ctx context.Context, ipoID string) (*models.EnhancedGMPData, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT g.id, g.ipo_name, g.company_code, g.ipo_price, g.gmp_value,
		       g.estimated_listing, g.gain_percent, g.sub2, g.kostak, g.last_updated,
		       g.stock_id, g.subscription_status, g.listing_gain, g.ipo_status,
		       g.data_source, g.extraction_metadata
		FROM ipo_list i
		JOIN ipo_gmp g ON (
			(i.stock_id IS NOT NULL AND g.stock_id IS NOT NULL AND i.stock_id = g.stock_id)
			OR i.company_code = g.company_code
		)
		WHERE i.id = $1
		ORDER BY
			CASE
				WHEN i.stock_id IS NOT NULL AND g.stock_id IS NOT NULL AND i.stock_id = g.stock_id THEN 1
				WHEN i.company_code = g.company_code THEN 2
				ELSE 3
			END,
			g.last_updated DESC
		LIMIT 1
	`

	var gmp models.EnhancedGMPData
	var extractionMetadata sql.NullString

	err := r.db.QueryRowContext(ctx, query, ipoID).Scan(
		&gmp.ID,
		&gmp.IPOName,
		&gmp.CompanyCode,
		&gmp.IPOPrice,
		&gmp.GMPValue,
		&gmp.EstimatedListing,
		&gmp.GainPercent,
		&gmp.Sub2,
		&gmp.Kostak,
		&gmp.LastUpdated,
		&gmp.StockID,
		&gmp.SubscriptionStatus,
		&gmp.ListingGain,
		&gmp.IPOStatus,
		&gmp.DataSource,
		&extractionMetadata,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query GMP by IPO id: %w", err)
	}

	if extractionMetadata.Valid && extractionMetadata.String != "" {
		var metadata models.ExtractionMetadata
		if err := json.Unmarshal([]byte(extractionMetadata.String), &metadata); err == nil {
			gmp.ExtractionMetadata = &metadata
		}
	}

	return &gmp, nil
}

func (r *SQLGMPRepository) ListRecent(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT ipo_name, company_code, gmp_value, gain_percent, estimated_listing, last_updated
		FROM ipo_gmp
		ORDER BY last_updated DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent GMP rows: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]interface{}, 0, limit)
	for rows.Next() {
		var ipoName, companyCode string
		var gmpValue, gainPercent, estimatedListing float64
		var lastUpdated interface{}

		if err := rows.Scan(&ipoName, &companyCode, &gmpValue, &gainPercent, &estimatedListing, &lastUpdated); err != nil {
			continue
		}

		results = append(results, map[string]interface{}{
			"ipo_name":          ipoName,
			"company_code":      companyCode,
			"gmp_value":         gmpValue,
			"gain_percent":      gainPercent,
			"estimated_listing": estimatedListing,
			"last_updated":      lastUpdated,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent GMP rows: %w", err)
	}

	return results, nil
}
