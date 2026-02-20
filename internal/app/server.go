package app

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fenilmodi00/ipo-backend/config"
	"github.com/fenilmodi00/ipo-backend/database"
	"github.com/fenilmodi00/ipo-backend/handlers"
	"github.com/fenilmodi00/ipo-backend/jobs"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	fiberrecover "github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/sirupsen/logrus"
)

type jobRunner interface {
	Run()
}

type lifecycleJob interface {
	Start()
	Stop()
}

func Run(cfg *config.Config) error {
	configureLogging(cfg.LogLevel)

	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	db, err := database.ConnectDB(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	if err := database.RunMigrations(db, "database/migrations"); err != nil {
		logrus.WithError(err).Error("Goose migrations failed, continuing with existing schema")
	}

	cacheConfig := config.DefaultCacheConfig()
	if cfg.CacheTTLHours != "" {
		cacheConfig.DefaultTTL = cfg.GetCacheTTL()
	}

	utilityService := services.NewUtilityService()
	scrapingService := services.NewChittorgarhIPOScrapingService(nil)
	allotmentChecker := services.NewAllotmentChecker()
	ipoService := services.NewIPOService(db)

	cacheService := services.NewCacheServiceWithConfig(
		db,
		cacheConfig.DefaultTTL,
		cacheConfig.MaxSize,
	)
	cachedIPOService := services.NewCachedIPOService(ipoService, cacheService)

	logServiceConfiguration(cacheConfig.DefaultTTL, cacheConfig.MaxSize)

	dailyJob := jobs.NewDailyIPOUpdateJob(scrapingService, ipoService, utilityService)
	resultJob := jobs.NewResultReleaseCheckJob(ipoService)
	cleanupJob := jobs.NewCacheCleanupJob(cacheService)
	gmpJob := jobs.NewGMPUpdateJob(db)
	gmpHistoryService := services.NewGMPHistoryService(db)
	gmpHistoryJob := jobs.NewGMPHistoryUpdateJobWithService(db, gmpHistoryService)

	ipoHandler := handlers.NewIPOHandler(ipoService)
	cacheHandler := handlers.NewCacheHandler(cacheService)
	adminHandler := handlers.NewAdminHandler(db, ipoService, gmpJob, gmpHistoryJob)
	checkHandler := handlers.NewCheckHandler(ipoService, allotmentChecker, cacheService)
	marketHandler := handlers.NewMarketHandler()
	gmpHandler := handlers.NewGMPHandler(db, gmpHistoryService)
	gmpHistoryHandler := handlers.NewGMPHistoryHandler(gmpHistoryService)
	performanceHandler := handlers.NewPerformanceHandler(db, ipoService, cachedIPOService)

	go func() {
		time.Sleep(2 * time.Second)
		if err := cachedIPOService.WarmupCache(context.Background()); err != nil {
			logrus.WithError(err).Warn("Cache warmup failed")
			return
		}
		logrus.Info("Cache warmed up successfully")
	}()

	backgroundCtx, stopBackgroundJobs := context.WithCancel(context.Background())
	defer stopBackgroundJobs()
	var backgroundWG sync.WaitGroup

	startBackgroundJobs(backgroundCtx, &backgroundWG, dailyJob, resultJob, cleanupJob, gmpJob, gmpHistoryJob)

	app := fiber.New(fiber.Config{BodyLimit: 4 * 1024 * 1024})
	registerMiddleware(app)
	registerRoutes(app, db, cfg, ipoHandler, cacheHandler, adminHandler, checkHandler, marketHandler, gmpHandler, gmpHistoryHandler, performanceHandler)

	serverErrors := make(chan error, 1)
	go func() {
		logrus.Infof("Server starting on port %s", cfg.ServerPort)
		if err := app.Listen(":" + cfg.ServerPort); err != nil {
			serverErrors <- err
		}
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-shutdownSignal:
		logrus.WithField("signal", sig.String()).Info("Shutdown signal received")
	case err := <-serverErrors:
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "server closed") {
			return fmt.Errorf("server failed to start or crashed: %w", err)
		}
	}

	gmpJob.Stop()
	gmpHistoryJob.Stop()
	backgroundWG.Wait()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	shutdownErr := make(chan error, 1)
	go func() {
		shutdownErr <- app.Shutdown()
	}()

	select {
	case err := <-shutdownErr:
		if err != nil {
			logrus.WithError(err).Error("Server shutdown failed")
		} else {
			logrus.Info("Server shutdown completed")
		}
	case <-shutdownCtx.Done():
		logrus.Warn("Server shutdown timed out")
	}

	logrus.Info("Application stopped cleanly")
	return nil
}

func registerMiddleware(app *fiber.App) {
	app.Use(fiberrecover.New())
	app.Use(logger.New())
	app.Use(cors.New())
}

func registerRoutes(
	app *fiber.App,
	db *sql.DB,
	cfg *config.Config,
	ipoHandler *handlers.IPOHandler,
	cacheHandler *handlers.CacheHandler,
	adminHandler *handlers.AdminHandler,
	checkHandler *handlers.CheckHandler,
	marketHandler *handlers.MarketHandler,
	gmpHandler *handlers.GMPHandler,
	gmpHistoryHandler *handlers.GMPHistoryHandler,
	performanceHandler *handlers.PerformanceHandler,
) {
	app.Get("/health", func(c *fiber.Ctx) error {
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		dbErr := db.PingContext(pingCtx)
		status := "ok"
		statusCode := fiber.StatusOK
		details := fiber.Map{"database": "ok"}

		if dbErr != nil {
			status = "degraded"
			statusCode = fiber.StatusServiceUnavailable
			details["database"] = dbErr.Error()
		}

		return c.Status(statusCode).JSON(fiber.Map{
			"status":    status,
			"timestamp": time.Now().Unix(),
			"details":   details,
		})
	})

	app.Get("/ready", func(c *fiber.Ctx) error {
		pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(pingCtx); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":  "not_ready",
				"reason":  "database_unreachable",
				"message": err.Error(),
			})
		}

		return c.JSON(fiber.Map{"status": "ready"})
	})

	api := app.Group("/api/v1")
	api.Get("/ipos", ipoHandler.GetIPOs)
	api.Get("/ipos/active", ipoHandler.GetActiveIPOs)
	api.Get("/ipos/active-with-gmp", ipoHandler.GetActiveIPOsWithGMP)
	api.Get("/ipos/:ipo_id/form-config", ipoHandler.GetIPOFormConfig)
	api.Get("/ipos/:id/gmp", gmpHandler.GetGMPByIPO)
	api.Get("/ipos/:id/with-gmp", ipoHandler.GetIPOByIDWithGMP)
	api.Get("/ipos/:id", ipoHandler.GetIPOByID)

	api.Get("/gmp/history/health", gmpHistoryHandler.GetHealthCheck)
	api.Get("/gmp/history/metrics", gmpHistoryHandler.GetServiceMetrics)
	api.Post("/gmp/backfill", gmpHistoryHandler.BackfillGMPHistory)
	api.Get("/gmp/history/:ipo_id", gmpHistoryHandler.GetIPOPriceHistory)
	api.Get("/gmp/history/:ipo_id/chart", gmpHistoryHandler.GetChartData)
	api.Get("/gmp/history/:ipo_id/summary", gmpHistoryHandler.GetHistorySummary)

	api.Get("/market/indices", marketHandler.GetMarketIndices)
	api.Post("/cache/store", cacheHandler.StoreResult)
	api.Get("/cache/:ipo_id/:pan_hash", cacheHandler.GetCachedResult)
	api.Post("/check", checkHandler.CheckAllotment)

	admin := api.Group("/admin", adminAuthMiddleware(cfg.AdminToken))
	admin.Post("/ipos", adminHandler.CreateIPO)
	admin.Post("/gmp/update", adminHandler.TriggerGMPUpdate)
	admin.Get("/gmp/data", adminHandler.GetGMPData)
	admin.Post("/gmp-history/update", adminHandler.TriggerGMPHistoryUpdate)
	admin.Get("/gmp-history/status", adminHandler.GetGMPHistoryJobStatus)
	admin.Get("/gmp-history/metrics", adminHandler.GetGMPHistoryJobMetrics)

	perf := api.Group("/performance")
	perf.Get("/metrics", performanceHandler.GetPerformanceMetrics)
	perf.Post("/test", performanceHandler.RunPerformanceTest)
	perf.Delete("/cache", performanceHandler.ClearCache)
	perf.Post("/cache/warmup", performanceHandler.WarmupCache)
}

func adminAuthMiddleware(adminToken string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if adminToken == "" {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"success": false,
				"error":   "Admin endpoints are disabled",
				"message": "ADMIN_TOKEN is not configured",
			})
		}

		authHeader := strings.TrimSpace(c.Get("Authorization"))
		token := strings.TrimSpace(c.Get("X-Admin-Token"))
		if token == "" && strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		}

		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) != 1 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   "Unauthorized",
				"message": "Valid admin token is required",
			})
		}

		return c.Next()
	}
}

func configureLogging(level string) {
	if level == "" {
		logrus.SetLevel(logrus.InfoLevel)
		return
	}

	parsedLevel, err := logrus.ParseLevel(strings.ToLower(level))
	if err != nil {
		logrus.WithError(err).Warnf("Invalid LOG_LEVEL '%s', defaulting to info", level)
		logrus.SetLevel(logrus.InfoLevel)
		return
	}

	logrus.SetLevel(parsedLevel)
}

func logServiceConfiguration(cacheTTL time.Duration, cacheMaxSize int) {
	defaultConfig := services.NewDefaultIPOScraperConfiguration()
	logrus.Println("Simplified IPO backend services initialized:")
	logrus.Printf("  - Simplified IPO scraper (rate limit: %v, timeout: %v)",
		defaultConfig.RequestRateLimit, defaultConfig.HTTPRequestTimeout)
	logrus.Printf("  - Allotment checker (rate limit: %v)", 2*time.Second)
	logrus.Printf("  - Unified cache service (TTL: %v, max size: %d)", cacheTTL, cacheMaxSize)
	logrus.Println("  - Utility service (text processing and normalization)")
	logrus.Println("  - Simplified IPO service (lifecycle analyzer removed)")
}

func startBackgroundJobs(
	ctx context.Context,
	wg *sync.WaitGroup,
	dailyJob jobRunner,
	resultJob jobRunner,
	cleanupJob jobRunner,
	gmpJob lifecycleJob,
	gmpHistoryJob lifecycleJob,
) {
	safeJobRun("daily_ipo_update_startup", dailyJob.Run)
	gmpJob.Start()
	gmpHistoryJob.Start()

	dailyTicker := time.NewTicker(8 * time.Hour)
	hourlyTicker := time.NewTicker(1 * time.Hour)
	cleanupTicker := time.NewTicker(12 * time.Hour)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer dailyTicker.Stop()
		defer hourlyTicker.Stop()
		defer cleanupTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				logrus.Info("Background scheduler stopped")
				return
			case <-dailyTicker.C:
				safeJobRun("daily_ipo_update", dailyJob.Run)
			case <-hourlyTicker.C:
				safeJobRun("result_release_check", resultJob.Run)
			case <-cleanupTicker.C:
				safeJobRun("cache_cleanup", cleanupJob.Run)
			}
		}
	}()
}

func safeJobRun(name string, run func()) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logrus.WithFields(logrus.Fields{
				"job":   name,
				"panic": fmt.Sprintf("%v", recovered),
			}).Error("Background job panicked and was isolated")
		}
	}()

	run()
}
