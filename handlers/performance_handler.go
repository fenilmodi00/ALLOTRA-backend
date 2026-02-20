package handlers

import (
	"context"
	"database/sql"
	"time"

	"github.com/fenilmodi00/ipo-backend/repositories"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
)

type PerformanceHandler struct {
	DB               *sql.DB
	DiagnosticsRepo  repositories.DiagnosticsRepository
	IPOService       *services.IPOService
	CachedIPOService *services.CachedIPOService
}

func NewPerformanceHandler(db *sql.DB, ipoService *services.IPOService, cachedIPOService *services.CachedIPOService) *PerformanceHandler {
	return &PerformanceHandler{
		DB:               db,
		DiagnosticsRepo:  repositories.NewSQLDiagnosticsRepository(db),
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
	indexStats, err := h.DiagnosticsRepo.GetIndexUsageStats(ctx)
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
	queryPlans, err := h.DiagnosticsRepo.AnalyzeQueryPlans(ctx, "")
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
