package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/google/uuid"
)

type SQLIPORepository struct {
	db *sql.DB
}

func NewSQLIPORepository(db *sql.DB) *SQLIPORepository {
	return &SQLIPORepository{db: db}
}

func (r *SQLIPORepository) GetByID(ctx context.Context, id uuid.UUID) (*models.IPO, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT id, stock_id, name, company_code, symbol, registrar,
		       open_date, close_date, result_date, listing_date,
		       price_band_low, price_band_high, issue_size, min_qty, min_amount,
		       status, subscription_status, listing_gain,
		       logo_url, description, about, slug,
		       form_url, form_fields, form_headers, parser_config,
		       strengths, risks, created_at, updated_at, created_by
		FROM ipo_list
		WHERE id = $1
	`

	ipo, err := scanIPO(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return nil, err
	}
	return ipo, nil
}

func (r *SQLIPORepository) GetByStockID(ctx context.Context, stockID string) (*models.IPO, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT id, stock_id, name, company_code, symbol, registrar,
		       open_date, close_date, result_date, listing_date,
		       price_band_low, price_band_high, issue_size, min_qty, min_amount,
		       status, subscription_status, listing_gain,
		       logo_url, description, about, slug,
		       form_url, form_fields, form_headers, parser_config,
		       strengths, risks, created_at, updated_at, created_by
		FROM ipo_list
		WHERE stock_id = $1
	`

	ipo, err := scanIPO(r.db.QueryRowContext(ctx, query, stockID))
	if err != nil {
		return nil, err
	}
	return ipo, nil
}

func (r *SQLIPORepository) List(ctx context.Context, filter IPOFilter) ([]models.IPO, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	limit := filter.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := ""
	args := make([]interface{}, 0, 3)
	argN := 1
	if filter.Status != nil && strings.TrimSpace(*filter.Status) != "" {
		where = fmt.Sprintf(" WHERE status = $%d", argN)
		args = append(args, *filter.Status)
		argN++
	}

	countQuery := "SELECT COUNT(*) FROM ipo_list" + where
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count IPO rows: %w", err)
	}

	listQuery := fmt.Sprintf(`
		SELECT id, stock_id, name, company_code, symbol, registrar,
		       open_date, close_date, result_date, listing_date,
		       price_band_low, price_band_high, issue_size, min_qty, min_amount,
		       status, subscription_status, listing_gain,
		       logo_url, description, about, slug,
		       form_url, form_fields, form_headers, parser_config,
		       strengths, risks, created_at, updated_at, created_by
		FROM ipo_list%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argN, argN+1)
	args = append(args, limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list IPO rows: %w", err)
	}
	defer rows.Close()

	ipos := make([]models.IPO, 0, limit)
	for rows.Next() {
		ipo, err := scanIPORow(rows)
		if err != nil {
			return nil, 0, err
		}
		ipos = append(ipos, *ipo)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate IPO rows: %w", err)
	}

	return ipos, total, nil
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanIPO(row rowScanner) (*models.IPO, error) {
	ipo, err := scanIPORow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return ipo, nil
}

func scanIPORow(row rowScanner) (*models.IPO, error) {
	var ipo models.IPO
	var formFields, formHeaders, parserConfig, strengths, risks []byte

	err := row.Scan(
		&ipo.ID,
		&ipo.StockID,
		&ipo.Name,
		&ipo.CompanyCode,
		&ipo.Symbol,
		&ipo.Registrar,
		&ipo.OpenDate,
		&ipo.CloseDate,
		&ipo.ResultDate,
		&ipo.ListingDate,
		&ipo.PriceBandLow,
		&ipo.PriceBandHigh,
		&ipo.IssueSize,
		&ipo.MinQty,
		&ipo.MinAmount,
		&ipo.Status,
		&ipo.SubscriptionStatus,
		&ipo.ListingGain,
		&ipo.LogoURL,
		&ipo.Description,
		&ipo.About,
		&ipo.Slug,
		&ipo.FormURL,
		&formFields,
		&formHeaders,
		&parserConfig,
		&strengths,
		&risks,
		&ipo.CreatedAt,
		&ipo.UpdatedAt,
		&ipo.CreatedBy,
	)
	if err != nil {
		return nil, err
	}

	ipo.FormFields = formFields
	ipo.FormHeaders = formHeaders
	ipo.ParserConfig = parserConfig
	ipo.Strengths = strengths
	ipo.Risks = risks

	return &ipo, nil
}
