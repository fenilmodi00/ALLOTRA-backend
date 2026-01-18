package jobs

import (
	"database/sql"
	"time"

	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/sirupsen/logrus"
)

type GMPUpdateJob struct {
	DB               *sql.DB
	SimpleGMPService *services.SimpleGMPService
}

func NewGMPUpdateJob(db *sql.DB) *GMPUpdateJob {
	return &GMPUpdateJob{
		DB:               db,
		SimpleGMPService: services.NewSimpleGMPService(db),
	}
}

func (j *GMPUpdateJob) Start() {
	logrus.Info("Starting GMP Update Job (runs every 1 hour)...")
	ticker := time.NewTicker(1 * time.Hour) // Run every 1 hour

	go func() {
		// Run immediately on start
		j.Run()

		for range ticker.C {
			j.Run()
		}
	}()
}

func (j *GMPUpdateJob) Run() {
	startTime := time.Now()
	logrus.Info("Running GMP Update Job with SimpleGMPService...")

	// Fetch and save GMP data using the simple service (handles modern InvestorGain structure)
	gmpData, err := j.SimpleGMPService.FetchAndSaveGMPData()
	if err != nil {
		logrus.Errorf("GMP Update Job failed: error fetching GMP data: %v", err)
		return
	}

	if len(gmpData) == 0 {
		logrus.Warn("GMP Update Job: no GMP data fetched from source")
		return
	}

	duration := time.Since(startTime)
	logrus.Infof("GMP Update Job completed successfully: processed %d GMP records (took %v)",
		len(gmpData), duration)
}
