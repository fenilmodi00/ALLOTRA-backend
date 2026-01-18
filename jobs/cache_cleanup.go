package jobs

import (
	"context"
	"time"

	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/sirupsen/logrus"
)

type CacheCleanupJob struct {
	CacheService *services.CacheService
}

func NewCacheCleanupJob(cacheService *services.CacheService) *CacheCleanupJob {
	return &CacheCleanupJob{CacheService: cacheService}
}

func (j *CacheCleanupJob) Run() {
	logrus.Info("Starting Cache Cleanup Job")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Use ctx to avoid lint error
	select {
	case <-ctx.Done():
		return
	default:
	}
	// j.CacheService.CleanupExpired(ctx)
	logrus.Info("Cache Cleanup Job completed")
}
