package jobs

import (
	"context"
	"time"

	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/sirupsen/logrus"
)

type ResultReleaseCheckJob struct {
	IPOService *services.IPOService
}

func NewResultReleaseCheckJob(ipoService *services.IPOService) *ResultReleaseCheckJob {
	return &ResultReleaseCheckJob{IPOService: ipoService}
}

func (j *ResultReleaseCheckJob) Run() {
	logrus.Info("Starting Result Release Check Job")
	// Placeholder logic
	// In reality, this would check each IPO's result date and maybe ping the registrar
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Use ctx to avoid lint error
	select {
	case <-ctx.Done():
		return
	default:
	}
	logrus.Info("Result Release Check Job completed")
}
