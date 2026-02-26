package jobs

import (
	"database/sql"
	"time"

	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/sirupsen/logrus"
)

type GMPUpdateJob struct {
	DB           *sql.DB
	GMPService   *services.GMPService
	CacheService *services.CacheService
}

func NewGMPUpdateJob(db *sql.DB, cacheService *services.CacheService, scraperURL string) *GMPUpdateJob {
	return &GMPUpdateJob{
		DB:           db,
		GMPService:   services.NewGMPServiceWithDB(db),
		CacheService: cacheService,
	}
}

func (j *GMPUpdateJob) Run() {
	startTime := time.Now()
	logrus.Info("Running GMP Update Job with GMPService...")

	// Fetch and save GMP data using the enhanced service (handles modern InvestorGain structure)
	gmpData, err := j.GMPService.FetchAndSaveGMPData()
	if err != nil {
		logrus.Errorf("GMP Update Job failed: error fetching GMP data: %v", err)
		return
	}

	if len(gmpData) == 0 {
		logrus.Warn("GMP Update Job: no GMP data fetched from source")
		return
	}

	// Invalidate IPO cache after successful GMP update
	if j.CacheService != nil {
		logrus.Info("Invalidating IPO cache after GMP update...")
		j.CacheService.Clear()
		logrus.Info("IPO cache invalidated successfully")
	}

	duration := time.Since(startTime)
	logrus.Infof("GMP Update Job completed successfully: processed %d GMP records (took %v)",
		len(gmpData), duration)
}
