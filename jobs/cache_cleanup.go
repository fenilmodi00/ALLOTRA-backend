package jobs

import (
	"github.com/sirupsen/logrus"
)

type CacheCleanupJob struct{}

func NewCacheCleanupJob() *CacheCleanupJob {
	return &CacheCleanupJob{}
}

func (j *CacheCleanupJob) Run() {
	logrus.Info("Cache Cleanup Job: No-op - Redis handles TTL natively")
}
