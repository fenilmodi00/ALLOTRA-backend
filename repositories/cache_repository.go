package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/google/uuid"
)

type SQLCacheRepository struct {
	db *sql.DB
}

func NewSQLCacheRepository(db *sql.DB) *SQLCacheRepository {
	return &SQLCacheRepository{db: db}
}

func (r *SQLCacheRepository) Store(ctx context.Context, result *models.IPOResultCache) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := `
		INSERT INTO ipo_result_cache (
			pan_hash, ipo_id, status, shares_allotted, application_number,
			refund_status, source, user_agent, timestamp, expires_at,
			confidence_score, duplicate_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (pan_hash, ipo_id) DO UPDATE SET
			status = EXCLUDED.status,
			shares_allotted = EXCLUDED.shares_allotted,
			application_number = EXCLUDED.application_number,
			refund_status = EXCLUDED.refund_status,
			source = EXCLUDED.source,
			user_agent = EXCLUDED.user_agent,
			timestamp = EXCLUDED.timestamp,
			expires_at = EXCLUDED.expires_at,
			confidence_score = EXCLUDED.confidence_score,
			duplicate_count = ipo_result_cache.duplicate_count + 1
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		result.PanHash,
		result.IPOID,
		result.Status,
		result.SharesAllotted,
		result.ApplicationNumber,
		result.RefundStatus,
		result.Source,
		result.UserAgent,
		result.Timestamp,
		result.ExpiresAt,
		result.ConfidenceScore,
		result.DuplicateCount,
	)
	if err != nil {
		return fmt.Errorf("store cache result: %w", err)
	}

	return nil
}

func (r *SQLCacheRepository) Get(ctx context.Context, ipoID uuid.UUID, panHash string) (*models.IPOResultCache, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT id, pan_hash, ipo_id, status, shares_allotted, application_number,
		       refund_status, source, user_agent, timestamp, expires_at,
		       confidence_score, duplicate_count
		FROM ipo_result_cache
		WHERE ipo_id = $1 AND pan_hash = $2 AND expires_at > NOW()
	`

	var result models.IPOResultCache
	err := r.db.QueryRowContext(ctx, query, ipoID, panHash).Scan(
		&result.ID,
		&result.PanHash,
		&result.IPOID,
		&result.Status,
		&result.SharesAllotted,
		&result.ApplicationNumber,
		&result.RefundStatus,
		&result.Source,
		&result.UserAgent,
		&result.Timestamp,
		&result.ExpiresAt,
		&result.ConfidenceScore,
		&result.DuplicateCount,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get cache result: %w", err)
	}

	return &result, nil
}

func (r *SQLCacheRepository) CleanupExpired(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `DELETE FROM ipo_result_cache WHERE expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired cache rows: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read rows affected: %w", err)
	}
	return rows, nil
}
