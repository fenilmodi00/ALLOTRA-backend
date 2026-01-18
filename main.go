package main

import (
	"context"
	"log"
	"time"

	"github.com/fenilmodi00/ipo-backend/config"
	"github.com/fenilmodi00/ipo-backend/database"
	"github.com/fenilmodi00/ipo-backend/handlers"
	"github.com/fenilmodi00/ipo-backend/jobs"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	// Load config
	cfg := config.LoadConfig()

	// Connect to database
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Run migrations
	if err := database.Migrate("database/schema.sql"); err != nil {
		log.Printf("Migration warning: %v", err)
	}

	// Initialize simplified service configurations
	cacheConfig := config.DefaultCacheConfig()

	// Override cache TTL from environment if specified
	if cfg.CacheTTLHours != "" {
		cacheConfig.DefaultTTL = cfg.GetCacheTTL()
	}

	// Initialize consolidated services with simplified configuration
	utilityService := services.NewUtilityService()
	scrapingService := services.NewChittorgarhIPOScrapingService(nil) // Use simplified scraper with default config
	allotmentChecker := services.NewAllotmentChecker()                // Separate service for allotment checking

	// Use Enhanced GMP Service with default configuration
	// gmpConfig := shared.NewGMPServiceConfig()
	// gmpService := services.NewEnhancedGMPService(&gmpConfig, database.DB)

	ipoService := services.NewIPOService(database.DB)

	// Initialize caching layer with simplified configuration
	cacheService := services.NewCacheServiceWithConfig(
		database.DB,
		cacheConfig.DefaultTTL,
		cacheConfig.MaxSize,
	)
	cachedIPOService := services.NewCachedIPOService(ipoService, cacheService)

	// Configure scraping service with simplified rate limiting
	// Note: Rate limiting is now handled internally by the simplified scraper

	// Log simplified service initialization
	defaultConfig := services.NewDefaultIPOScraperConfiguration()
	log.Println("Simplified IPO backend services initialized:")
	log.Printf("  - Simplified IPO scraper (rate limit: %v, timeout: %v)",
		defaultConfig.RequestRateLimit, defaultConfig.HTTPRequestTimeout)
	log.Printf("  - Allotment checker (rate limit: %v)", 2*time.Second)
	log.Printf("  - Unified cache service (TTL: %v, max size: %d)",
		cacheConfig.DefaultTTL, cacheConfig.MaxSize)
	log.Println("  - Utility service (text processing and normalization)")
	log.Println("  - Simplified IPO service (lifecycle analyzer removed)")

	// Initialize Jobs with consolidated services first
	dailyJob := jobs.NewDailyIPOUpdateJob(scrapingService, ipoService, utilityService)
	resultJob := jobs.NewResultReleaseCheckJob(ipoService)
	cleanupJob := jobs.NewCacheCleanupJob(cacheService)
	gmpJob := jobs.NewGMPUpdateJob(database.DB)

	// Initialize handlers with consolidated services
	ipoHandler := handlers.NewIPOHandler(ipoService)
	cacheHandler := handlers.NewCacheHandler(cacheService)
	adminHandler := handlers.NewAdminHandler(ipoService, gmpJob)
	checkHandler := handlers.NewCheckHandler(ipoService, allotmentChecker, cacheService)
	marketHandler := handlers.NewMarketHandler()
	gmpHandler := handlers.NewGMPHandler(database.DB)
	performanceHandler := handlers.NewPerformanceHandler(database.DB, ipoService, cachedIPOService)

	// Warmup cache on startup
	go func() {
		time.Sleep(2 * time.Second) // Wait for database to be ready
		if err := cachedIPOService.WarmupCache(context.Background()); err != nil {
			log.Printf("Cache warmup failed: %v", err)
		} else {
			log.Println("Cache warmed up successfully")
		}
	}()

	// Start Background Jobs with simplified scheduling
	go func() {
		// Run immediately on startup
		go dailyJob.Run()

		// Start GMP job with its own internal ticker (runs every 1 hour)
		gmpJob.Start()

		// Schedule other jobs with simplified timing
		dailyTicker := time.NewTicker(8 * time.Hour)
		hourlyTicker := time.NewTicker(1 * time.Hour)
		cleanupTicker := time.NewTicker(12 * time.Hour)

		for {
			select {
			case <-dailyTicker.C:
				dailyJob.Run()
			case <-hourlyTicker.C:
				resultJob.Run()
			case <-cleanupTicker.C:
				cleanupJob.Run()
			}
		}
	}()

	// Setup Fiber
	app := fiber.New()

	// Middleware
	app.Use(logger.New())
	app.Use(cors.New())

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"timestamp": time.Now().Unix(),
		})
	})

	// Routes
	api := app.Group("/api/v1")

	// IPO Routes
	api.Get("/ipos", ipoHandler.GetIPOs)
	api.Get("/ipos/active", ipoHandler.GetActiveIPOs)
	api.Get("/ipos/active-with-gmp", ipoHandler.GetActiveIPOsWithGMP) // New: Returns active IPOs with GMP data joined
	api.Get("/ipos/:ipo_id/form-config", ipoHandler.GetIPOFormConfig)
	api.Get("/ipos/:id/gmp", gmpHandler.GetGMPByIPO)
	api.Get("/ipos/:id/with-gmp", ipoHandler.GetIPOByIDWithGMP) // New: Returns single IPO with GMP data joined
	api.Get("/ipos/:id", ipoHandler.GetIPOByID)

	// Market Routes
	api.Get("/market/indices", marketHandler.GetMarketIndices)

	// Cache Routes
	api.Post("/cache/store", cacheHandler.StoreResult)
	api.Get("/cache/:ipo_id/:pan_hash", cacheHandler.GetCachedResult)

	// Check Route
	api.Post("/check", checkHandler.CheckAllotment)

	// Admin Routes
	admin := api.Group("/admin")
	// TODO: Add auth middleware
	admin.Post("/ipos", adminHandler.CreateIPO)
	admin.Post("/gmp/update", adminHandler.TriggerGMPUpdate)
	admin.Get("/gmp/data", adminHandler.GetGMPData)

	// Performance Routes
	perf := api.Group("/performance")
	perf.Get("/metrics", performanceHandler.GetPerformanceMetrics)
	perf.Post("/test", performanceHandler.RunPerformanceTest)
	perf.Delete("/cache", performanceHandler.ClearCache)
	perf.Post("/cache/warmup", performanceHandler.WarmupCache)

	// Start server
	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := app.Listen(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
