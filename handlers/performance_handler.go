package handlers

import (
	"context"
	"database/sql"
	"time"

	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
)

type PerformanceHandler struct {
	DB               *sql.DB
	IPOService       *services.IPOService
	CachedIPOService *services.CachedIPOService
}

func NewPerformanceHandler(db *sql.DB, ipoService *services.IPOService, cachedIPOService *services.CachedIPOService) *PerformanceHandler {
	return &PerformanceHandler{
		DB:               db,
		IPOService:       ipoService,
		CachedIPOService: cachedIPOService,
	}
}

// GetPerformanceMetrics returns current performance metrics
func (h *PerformanceHandler) GetPerformanceMetrics(c *fiber.Ctx) error {
	ctx := context.Background()

	// Test query performance
	metrics := make(map[string]interface{})

	// Test 1: GetActiveIPOsWithGMP performance
	start := time.Now()
	ipos, err := h.IPOService.GetActiveIPOsWithGMP(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to test GetActiveIPOsWithGMP: " + err.Error(),
		})
	}
	metrics["get_active_ipos_with_gmp"] = map[string]interface{}{
		"duration_ms": time.Since(start).Milliseconds(),
		"count":       len(ipos),
		"cached":      false,
	}

	// Test 2: Cached query performance
	if h.CachedIPOService != nil {
		start = time.Now()
		cachedIPOs, err := h.CachedIPOService.GetActiveIPOsWithGMP(ctx)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to test cached GetActiveIPOsWithGMP: " + err.Error(),
			})
		}
		metrics["get_active_ipos_with_gmp_cached"] = map[string]interface{}{
			"duration_ms": time.Since(start).Milliseconds(),
			"count":       len(cachedIPOs),
			"cached":      true,
		}

		// Cache statistics
		metrics["cache_stats"] = h.CachedIPOService.GetCacheStats()
	}

	// Test 3: Database connection pool stats
	dbStats := h.DB.Stats()
	metrics["database_stats"] = map[string]interface{}{
		"open_connections":     dbStats.OpenConnections,
		"in_use":               dbStats.InUse,
		"idle":                 dbStats.Idle,
		"wait_count":           dbStats.WaitCount,
		"wait_duration_ms":     dbStats.WaitDuration.Milliseconds(),
		"max_idle_closed":      dbStats.MaxIdleClosed,
		"max_idle_time_closed": dbStats.MaxIdleTimeClosed,
		"max_lifetime_closed":  dbStats.MaxLifetimeClosed,
	}

	// Test 4: Index usage statistics
	indexStats, err := h.getIndexUsageStats(ctx)
	if err != nil {
		metrics["index_stats_error"] = err.Error()
	} else {
		metrics["index_stats"] = indexStats
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    metrics,
	})
}

// RunPerformanceTest runs a comprehensive performance test
func (h *PerformanceHandler) RunPerformanceTest(c *fiber.Ctx) error {
	ctx := context.Background()

	results := make(map[string]interface{})

	// Test 1: Query performance under load
	iterations := 10
	var totalDuration time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, err := h.IPOService.GetActiveIPOsWithGMP(ctx)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Performance test failed: " + err.Error(),
			})
		}
		totalDuration += time.Since(start)
	}

	avgDuration := totalDuration / time.Duration(iterations)
	results["load_test"] = map[string]interface{}{
		"iterations":        iterations,
		"total_duration_ms": totalDuration.Milliseconds(),
		"avg_duration_ms":   avgDuration.Milliseconds(),
		"queries_per_sec":   float64(iterations) / totalDuration.Seconds(),
	}

	// Test 2: Cache performance comparison
	if h.CachedIPOService != nil {
		// Clear cache first
		h.CachedIPOService.InvalidateAllIPOCache()

		// Test uncached performance
		start := time.Now()
		_, err := h.CachedIPOService.GetActiveIPOsWithGMP(ctx)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Cache test failed: " + err.Error(),
			})
		}
		uncachedDuration := time.Since(start)

		// Test cached performance
		start = time.Now()
		_, err = h.CachedIPOService.GetActiveIPOsWithGMP(ctx)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Cache test failed: " + err.Error(),
			})
		}
		cachedDuration := time.Since(start)

		speedup := float64(uncachedDuration) / float64(cachedDuration)

		results["cache_performance"] = map[string]interface{}{
			"uncached_duration_ms": uncachedDuration.Milliseconds(),
			"cached_duration_ms":   cachedDuration.Milliseconds(),
			"speedup_factor":       speedup,
		}
	}

	// Test 3: Query plan analysis
	queryPlans, err := h.analyzeQueryPlans(ctx)
	if err != nil {
		results["query_plan_error"] = err.Error()
	} else {
		results["query_plans"] = queryPlans
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    results,
	})
}

// ClearCache clears all cached data
func (h *PerformanceHandler) ClearCache(c *fiber.Ctx) error {
	if h.CachedIPOService != nil {
		h.CachedIPOService.InvalidateAllIPOCache()
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Cache cleared successfully",
		})
	}

	return c.JSON(fiber.Map{
		"success": false,
		"message": "Cache service not available",
	})
}

// WarmupCache pre-loads frequently accessed data
func (h *PerformanceHandler) WarmupCache(c *fiber.Ctx) error {
	if h.CachedIPOService != nil {
		ctx := context.Background()
		start := time.Now()

		err := h.CachedIPOService.WarmupCache(ctx)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Cache warmup failed: " + err.Error(),
			})
		}

		duration := time.Since(start)

		return c.JSON(fiber.Map{
			"success":     true,
			"message":     "Cache warmed up successfully",
			"duration_ms": duration.Milliseconds(),
		})
	}

	return c.JSON(fiber.Map{
		"success": false,
		"message": "Cache service not available",
	})
}

// getIndexUsageStats retrieves database index usage statistics
func (h *PerformanceHandler) getIndexUsageStats(ctx context.Context) ([]map[string]interface{}, error) {
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

	rows, err := h.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []map[string]interface{}

	for rows.Next() {
		var schema, table, index string
		var scans, tuplesRead, tuplesFetched int64

		if err := rows.Scan(&schema, &table, &index, &scans, &tuplesRead, &tuplesFetched); err != nil {
			return nil, err
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

	return stats, nil
}

// analyzeQueryPlans analyzes execution plans for key queries
func (h *PerformanceHandler) analyzeQueryPlans(ctx context.Context) (map[string][]string, error) {
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

	plans := make(map[string][]string)

	for name, query := range queries {
		rows, err := h.DB.QueryContext(ctx, query)
		if err != nil {
			plans[name] = []string{"Error: " + err.Error()}
			continue
		}

		var planLines []string
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				rows.Close()
				return nil, err
			}
			planLines = append(planLines, line)
		}
		rows.Close()

		plans[name] = planLines
	}

	return plans, nil
}
