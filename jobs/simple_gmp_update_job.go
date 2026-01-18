package jobs

import (
	"database/sql"
	"time"

	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/sirupsen/logrus"
)

// SimpleGMPUpdateJob handles periodic GMP data updates
type SimpleGMPUpdateJob struct {
	gmpService *services.SimpleGMPService
	logger     *logrus.Logger
	isRunning  bool
}

// NewSimpleGMPUpdateJob creates a new simple GMP update job
func NewSimpleGMPUpdateJob(db *sql.DB) *SimpleGMPUpdateJob {
	return &SimpleGMPUpdateJob{
		gmpService: services.NewSimpleGMPService(db),
		logger:     logrus.New(),
		isRunning:  false,
	}
}

// Run executes the GMP update job
func (j *SimpleGMPUpdateJob) Run() error {
	if j.isRunning {
		j.logger.Warn("GMP update job already running, skipping")
		return nil
	}

	j.isRunning = true
	defer func() {
		j.isRunning = false
	}()

	startTime := time.Now()
	j.logger.Info("Starting simple GMP update job")

	// Fetch and save GMP data
	gmpData, err := j.gmpService.FetchAndSaveGMPData()
	if err != nil {
		j.logger.WithError(err).Error("Failed to update GMP data")
		return err
	}

	processingTime := time.Since(startTime)
	j.logger.WithFields(logrus.Fields{
		"records_updated": len(gmpData),
		"processing_time": processingTime,
		"records_per_sec": float64(len(gmpData)) / processingTime.Seconds(),
	}).Info("Successfully completed GMP update job")

	return nil
}

// StartPeriodicUpdates starts periodic GMP data updates
func (j *SimpleGMPUpdateJob) StartPeriodicUpdates(interval time.Duration) {
	j.logger.WithField("interval", interval).Info("Starting periodic GMP updates")

	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := j.Run(); err != nil {
				j.logger.WithError(err).Error("Periodic GMP update failed")
			}
		}
	}()
}

// IsRunning returns whether the job is currently running
func (j *SimpleGMPUpdateJob) IsRunning() bool {
	return j.isRunning
}