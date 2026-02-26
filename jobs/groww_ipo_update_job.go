// Package jobs contains the Groww IPO daily update job.
//
// This job runs once per day (configurable via ticker) to discover all IPOs
// from the Groww dashboard and scrape their full details. During the testing
// phase the job only logs results — no DB writes occur until the schema is
// finalised and the data quality is confirmed to exceed Chittorgarh/InvestorGain.
package jobs

import (
	"context"
	"sync"
	"time"

	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/sirupsen/logrus"
)

// GrowwIPOUpdateJob runs a daily discover + scrape cycle against Groww's JSON APIs.
// It follows the same lifecycle pattern as GMPUpdateJob (Start/Stop/Run).
type GrowwIPOUpdateJob struct {
	scraper    *services.GrowwScraperService
	mapper     *services.GrowwMapper
	ipoService *services.IPOService
	ticker     *time.Ticker
	stopChan   chan struct{}
	stopOnce   sync.Once
	stateMu    sync.Mutex
	isRunning  bool
}

// NewGrowwIPOUpdateJob constructs a ready-to-start job.
func NewGrowwIPOUpdateJob() *GrowwIPOUpdateJob {
	return &GrowwIPOUpdateJob{
		scraper:  services.NewGrowwScraperService(),
		stopChan: make(chan struct{}),
	}
}

// NewGrowwIPOUpdateJobWithDependencies constructs a Groww job that persists
// scraped data via mapper and IPO service.
func NewGrowwIPOUpdateJobWithDependencies(
	scraper *services.GrowwScraperService,
	mapper *services.GrowwMapper,
	ipoService *services.IPOService,
) *GrowwIPOUpdateJob {
	if scraper == nil {
		scraper = services.NewGrowwScraperService()
	}

	return &GrowwIPOUpdateJob{
		scraper:    scraper,
		mapper:     mapper,
		ipoService: ipoService,
		stopChan:   make(chan struct{}),
	}
}

// Start launches the job on a 24-hour ticker. An initial run fires immediately
// on startup (after a 10-second warm-up delay so the DB is fully ready).
func (j *GrowwIPOUpdateJob) Start() {
	j.stateMu.Lock()
	if j.isRunning {
		j.stateMu.Unlock()
		logrus.Warn("GrowwIPOUpdateJob is already running")
		return
	}
	j.isRunning = true
	ticker := time.NewTicker(24 * time.Hour)
	j.ticker = ticker
	j.stateMu.Unlock()

	logrus.Info("Starting GrowwIPOUpdateJob (runs every 24 hours)")

	go func(localTicker *time.Ticker, stop <-chan struct{}) {
		defer func() {
			j.stateMu.Lock()
			j.isRunning = false
			j.ticker = nil
			j.stateMu.Unlock()
		}()

		// Short warm-up delay so the rest of the server is ready.
		select {
		case <-time.After(10 * time.Second):
		case <-stop:
			return
		}

		// Fire immediately on start.
		j.Run()

		for {
			select {
			case <-localTicker.C:
				j.Run()
			case <-stop:
				logrus.Info("GrowwIPOUpdateJob stopped")
				return
			}
		}
	}(ticker, j.stopChan)
}

// Stop signals the background goroutine to exit cleanly.
func (j *GrowwIPOUpdateJob) Stop() {
	j.stopOnce.Do(func() {
		close(j.stopChan)
		j.stateMu.Lock()
		if j.ticker != nil {
			j.ticker.Stop()
		}
		j.stateMu.Unlock()
		logrus.Info("GrowwIPOUpdateJob stop signal sent")
	})
}

// Run performs one full discover-and-scrape cycle synchronously.
// During the testing phase results are logged only — no DB writes.
func (j *GrowwIPOUpdateJob) Run() {
	log := logrus.WithField("job", "GrowwIPOUpdateJob")
	log.Info("Groww IPO update job started")

	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	result, err := j.scraper.DiscoverAndScrapeAll(ctx)
	if err != nil {
		log.WithError(err).Error("Groww IPO update job failed during discovery")
		return
	}

	duration := time.Since(start)

	log.WithFields(logrus.Fields{
		"duration":         duration.String(),
		"total_discovered": result.TotalDiscovered,
		"total_scraped":    result.TotalScraped,
		"successful":       result.Successful,
		"failed":           result.Failed,
	}).Info("Groww IPO update job completed")

	// Log per-IPO summary at debug level — useful for quality auditing.
	for _, scraped := range result.Results {
		if scraped == nil {
			continue
		}
		fields := logrus.Fields{
			"slug":       scraped.Slug,
			"details_ok": scraped.DetailsError == "",
			"cms_ok":     scraped.CMSError == "",
		}
		if scraped.Details != nil {
			fields["company"] = scraped.Details.CompanyName
			fields["status"] = scraped.Details.Status
		}
		if scraped.DetailsError != "" {
			fields["details_error"] = scraped.DetailsError
		}
		log.WithFields(fields).Debug("Groww IPO scrape result")

		if j.mapper == nil || j.ipoService == nil || scraped.Details == nil {
			continue
		}

		ipoModel := j.mapper.MapToIPO(scraped, nil)
		if ipoModel == nil {
			log.WithField("slug", scraped.Slug).Warn("Mapper returned nil IPO model")
			continue
		}

		if upsertErr := j.ipoService.UpsertIPO(ctx, *ipoModel); upsertErr != nil {
			log.WithError(upsertErr).WithField("slug", scraped.Slug).Error("Failed to upsert Groww IPO")
			continue
		}

		log.WithField("slug", scraped.Slug).Info("Groww IPO upserted")
	}
}
