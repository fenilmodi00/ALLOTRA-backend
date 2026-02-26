package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type SQLDiagnosticsRepository struct {
	db *sql.DB
}

func NewSQLDiagnosticsRepository(db *sql.DB) *SQLDiagnosticsRepository {
	return &SQLDiagnosticsRepository{db: db}
}

func (r *SQLDiagnosticsRepository) GetIndexUsageStats(ctx context.Context) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT
			schemaname,
			relname as table_name,
			indexrelname as index_name,
			idx_scan as scans,
			idx_tup_read as tuples_read,
			idx_tup_fetch as tuples_fetched
		FROM pg_stat_user_indexes
		WHERE relname IN ('ipo_list', 'ipo_gmp')
		ORDER BY relname, idx_scan DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query index usage stats: %w", err)
	}
	defer rows.Close()

	stats := make([]map[string]interface{}, 0)
	for rows.Next() {
		var schema, table, index string
		var scans, tuplesRead, tuplesFetched int64

		if err := rows.Scan(&schema, &table, &index, &scans, &tuplesRead, &tuplesFetched); err != nil {
			return nil, fmt.Errorf("scan index usage row: %w", err)
		}

		stats = append(stats, map[string]interface{}{
			"schema":         schema,
			"table":          table,
			"index":          index,
			"scans":          scans,
			"tuples_read":    tuplesRead,
			"tuples_fetched": tuplesFetched,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate index usage rows: %w", err)
	}

	return stats, nil
}

func (r *SQLDiagnosticsRepository) AnalyzeQueryPlans(ctx context.Context, sampleIPOID string) (map[string][]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	queries := map[string]string{
		"active_ipos_with_gmp": `
			EXPLAIN (FORMAT TEXT)
			SELECT i.*, g.gmp_value, g.gain_percent
			FROM ipo_list i
			LEFT JOIN ipo_gmp g ON i.company_code = g.company_code
			WHERE i.status = 'LIVE' OR i.status = 'RESULT_OUT'
			ORDER BY i.created_at DESC
			LIMIT 10
		`,
		"single_ipo_with_gmp": `
			EXPLAIN (FORMAT TEXT)
			SELECT i.*, g.gmp_value, g.gain_percent
			FROM ipo_list i
			LEFT JOIN ipo_gmp g ON i.company_code = g.company_code
			WHERE i.id = $1
		`,
	}

	plans := make(map[string][]string, len(queries))
	for name, query := range queries {
		var rows *sql.Rows
		var err error
		if name == "single_ipo_with_gmp" && sampleIPOID != "" {
			rows, err = r.db.QueryContext(ctx, query, sampleIPOID)
		} else if name == "single_ipo_with_gmp" && sampleIPOID == "" {
			query = `
				EXPLAIN (FORMAT TEXT)
				SELECT i.*, g.gmp_value, g.gain_percent
				FROM ipo_list i
				LEFT JOIN ipo_gmp g ON i.company_code = g.company_code
				LIMIT 1
			`
			rows, err = r.db.QueryContext(ctx, query)
		} else {
			rows, err = r.db.QueryContext(ctx, query)
		}

		if err != nil {
			plans[name] = []string{"Error: " + err.Error()}
			continue
		}

		planLines := make([]string, 0)
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan query plan line: %w", err)
			}
			planLines = append(planLines, line)
		}
		rows.Close()

		plans[name] = planLines
	}

	return plans, nil
}
