package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/google/uuid"
)

type SQLGMPHistoryRepository struct {
	db *sql.DB
}

func NewSQLGMPHistoryRepository(db *sql.DB) *SQLGMPHistoryRepository {
	return &SQLGMPHistoryRepository{db: db}
}

func (r *SQLGMPHistoryRepository) GetByIPO(ctx context.Context, ipoID uuid.UUID, dateRange *models.DateRange) ([]models.GMPPriceHistoryEntry, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	baseQuery := `
		SELECT
			h.id,
			h.ipo_id,
			h.company_code,
			h.record_date,
			h.ipo_price,
			h.gmp_value,
			h.estimated_listing,
			h.listing_percent,
			h.estimated_profit,
			h.subscription_status,
			h.sub2_sauda,
			h.last_updated,
			h.data_source,
			h.created_at,
			h.updated_at,
			i.name
		FROM gmp_price_history h
		JOIN ipo_list i ON i.id = h.ipo_id
		WHERE h.ipo_id = $1
	`

	args := []interface{}{ipoID}
	argN := 2
	if dateRange != nil {
		baseQuery += fmt.Sprintf(" AND h.record_date >= $%d AND h.record_date <= $%d", argN, argN+1)
		args = append(args, dateRange.StartDate, dateRange.EndDate)
		argN += 2
	}

	baseQuery += " ORDER BY h.record_date DESC"

	rows, err := r.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, "", fmt.Errorf("query GMP history by IPO: %w", err)
	}
	defer rows.Close()

	entries := make([]models.GMPPriceHistoryEntry, 0)
	ipoName := ""

	for rows.Next() {
		var entry models.GMPPriceHistoryEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.IPOID,
			&entry.CompanyCode,
			&entry.RecordDate,
			&entry.IPOPrice,
			&entry.GMPValue,
			&entry.EstimatedListing,
			&entry.ListingPercent,
			&entry.EstimatedProfit,
			&entry.SubscriptionStatus,
			&entry.Sub2Sauda,
			&entry.LastUpdated,
			&entry.DataSource,
			&entry.CreatedAt,
			&entry.UpdatedAt,
			&ipoName,
		); err != nil {
			return nil, "", fmt.Errorf("scan GMP history row: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("iterate GMP history rows: %w", err)
	}

	if len(entries) == 0 {
		return nil, "", ErrNotFound
	}

	return entries, ipoName, nil
}

func (r *SQLGMPHistoryRepository) GetArchivalStats(ctx context.Context) (int, *time.Time, *time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `SELECT COUNT(*), MIN(record_date), MAX(record_date) FROM gmp_price_history`
	var count int
	var minDate, maxDate sql.NullTime

	err := r.db.QueryRowContext(ctx, query).Scan(&count, &minDate, &maxDate)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("query archival stats: %w", err)
	}

	var minPtr, maxPtr *time.Time
	if minDate.Valid {
		t := minDate.Time
		minPtr = &t
	}
	if maxDate.Valid {
		t := maxDate.Time
		maxPtr = &t
	}

	return count, minPtr, maxPtr, nil
}
