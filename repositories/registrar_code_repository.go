package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
)

type PostgresRegistrarCodeRepository struct {
	db *sql.DB
}

func NewPostgresRegistrarCodeRepository(db *sql.DB) *PostgresRegistrarCodeRepository {
	return &PostgresRegistrarCodeRepository{db: db}
}

// Upsert inserts or updates a registrar code record
func (r *PostgresRegistrarCodeRepository) Upsert(ctx context.Context, code *models.RegistrarCode) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO ipo_registrar_codes (
			id, ipo_id, registrar_short_code, registrar_company_code,
			ipo_name, match_score, is_resolved, last_attempted_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
		ON CONFLICT (ipo_id, registrar_short_code) DO UPDATE SET
			registrar_company_code = EXCLUDED.registrar_company_code,
			ipo_name = EXCLUDED.ipo_name,
			match_score = EXCLUDED.match_score,
			is_resolved = EXCLUDED.is_resolved,
			last_attempted_at = EXCLUDED.last_attempted_at,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		code.ID,
		code.IPOID,
		code.RegistrarShortCode,
		code.RegistrarCompanyCode,
		code.IPOName,
		code.MatchScore,
		code.IsResolved,
		code.LastAttemptedAt,
		code.CreatedAt,
		code.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert registrar code: %w", err)
	}

	return nil
}

// GetByIPOAndRegistrar retrieves a registrar code by IPO ID and registrar short code
func (r *PostgresRegistrarCodeRepository) GetByIPOAndRegistrar(ctx context.Context, ipoID string, registrarShortCode string) (*models.RegistrarCode, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT id, ipo_id, registrar_short_code, registrar_company_code,
		       ipo_name, match_score, is_resolved, last_attempted_at,
		       created_at, updated_at
		FROM ipo_registrar_codes
		WHERE ipo_id = $1 AND registrar_short_code = $2
	`

	var code models.RegistrarCode
	err := r.db.QueryRowContext(ctx, query, ipoID, registrarShortCode).Scan(
		&code.ID,
		&code.IPOID,
		&code.RegistrarShortCode,
		&code.RegistrarCompanyCode,
		&code.IPOName,
		&code.MatchScore,
		&code.IsResolved,
		&code.LastAttemptedAt,
		&code.CreatedAt,
		&code.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query registrar code by ipo and registrar: %w", err)
	}

	return &code, nil
}

// GetUnresolvedByResultDate retrieves all unresolved registrar codes for a given result date
func (r *PostgresRegistrarCodeRepository) GetUnresolvedByResultDate(ctx context.Context, date time.Time) ([]*models.RegistrarCode, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT rc.id, rc.ipo_id, rc.registrar_short_code, rc.registrar_company_code,
		       rc.ipo_name, rc.match_score, rc.is_resolved, rc.last_attempted_at,
		       rc.created_at, rc.updated_at
		FROM ipo_registrar_codes rc
		JOIN ipo_list i ON rc.ipo_id = i.id
		WHERE rc.is_resolved = false AND DATE(i.result_date) = DATE($1)
		ORDER BY rc.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, date)
	if err != nil {
		return nil, fmt.Errorf("query unresolved registrar codes by result date: %w", err)
	}
	defer rows.Close()

	codes := make([]*models.RegistrarCode, 0)
	for rows.Next() {
		var code models.RegistrarCode
		if err := rows.Scan(
			&code.ID,
			&code.IPOID,
			&code.RegistrarShortCode,
			&code.RegistrarCompanyCode,
			&code.IPOName,
			&code.MatchScore,
			&code.IsResolved,
			&code.LastAttemptedAt,
			&code.CreatedAt,
			&code.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan registrar code row: %w", err)
		}
		codes = append(codes, &code)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate registrar code rows: %w", err)
	}

	return codes, nil
}
