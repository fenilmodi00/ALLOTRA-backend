package jobs

import (
	"database/sql"
	"sync"
	"time"

	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/sirupsen/logrus"
)

type GMPUpdateJob struct {
	DB           *sql.DB
	GMPService   *services.GMPService
	CacheService *services.CacheService
	ticker       *time.Ticker
	stopChan     chan struct{}
	stopOnce     sync.Once
	stateMu      sync.Mutex
	isRunning    bool
}

func NewGMPUpdateJob(db *sql.DB, cacheService *services.CacheService) *GMPUpdateJob {
	return &GMPUpdateJob{
		DB:           db,
		GMPService:   services.NewGMPServiceWithDB(db),
		CacheService: cacheService,
		stopChan:     make(chan struct{}),
	}
}

func (j *GMPUpdateJob) Start() {
	j.stateMu.Lock()
	if j.isRunning {
		j.stateMu.Unlock()
		logrus.Warn("GMP Update Job is already running")
		return
	}
	j.isRunning = true
	ticker := time.NewTicker(1 * time.Hour)
	j.ticker = ticker
	j.stateMu.Unlock()

	logrus.Info("Starting GMP Update Job (runs every 1 hour)...")

	go func(localTicker *time.Ticker, stop <-chan struct{}) {
		defer func() {
			j.stateMu.Lock()
			j.isRunning = false
			j.ticker = nil
			j.stateMu.Unlock()
		}()

		// Run immediately on start
		j.Run()

		for {
			select {
			case <-localTicker.C:
				j.Run()
			case <-stop:
				logrus.Info("GMP Update Job stopped")
				return
			}
		}
	}(ticker, j.stopChan)
}

func (j *GMPUpdateJob) Stop() {
	j.stateMu.Lock()
	if !j.isRunning {
		j.stateMu.Unlock()
		return
	}
	ticker := j.ticker
	j.stateMu.Unlock()

	if ticker != nil {
		ticker.Stop()
	}

	j.stopOnce.Do(func() {
		close(j.stopChan)
	})

	logrus.Info("GMP Update Job shutdown complete")
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
