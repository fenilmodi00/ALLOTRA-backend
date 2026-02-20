package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/google/uuid"
)

type SQLUpdateLogRepository struct {
	db *sql.DB
}

func NewSQLUpdateLogRepository(db *sql.DB) *SQLUpdateLogRepository {
	return &SQLUpdateLogRepository{db: db}
}

func (r *SQLUpdateLogRepository) Create(ctx context.Context, entry *models.IPOUpdateLog) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	query := `
		INSERT INTO ipo_update_log (ipo_id, field_name, old_value, new_value, source, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		entry.IPOID,
		entry.FieldName,
		entry.OldValue,
		entry.NewValue,
		entry.Source,
		entry.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("create update log entry: %w", err)
	}

	return nil
}

func (r *SQLUpdateLogRepository) GetByIPO(ctx context.Context, ipoID uuid.UUID, limit int) ([]models.IPOUpdateLog, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, ipo_id, field_name, old_value, new_value, source, timestamp
		FROM ipo_update_log
		WHERE ipo_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, ipoID, limit)
	if err != nil {
		return nil, fmt.Errorf("list update log entries: %w", err)
	}
	defer rows.Close()

	entries := make([]models.IPOUpdateLog, 0)
	for rows.Next() {
		var entry models.IPOUpdateLog
		if err := rows.Scan(
			&entry.ID,
			&entry.IPOID,
			&entry.FieldName,
			&entry.OldValue,
			&entry.NewValue,
			&entry.Source,
			&entry.Timestamp,
		); err != nil {
			return nil, fmt.Errorf("scan update log entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate update log entries: %w", err)
	}

	return entries, nil
}
