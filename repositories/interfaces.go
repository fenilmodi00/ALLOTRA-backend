package repositories

import (
	"context"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/google/uuid"
)

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

type IPOFilter struct {
	Status *string
	Limit  int
	Offset int
}

type IPORepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.IPO, error)
	GetByStockID(ctx context.Context, stockID string) (*models.IPO, error)
	List(ctx context.Context, filter IPOFilter) ([]models.IPO, int, error)
}

type GMPRepository interface {
	GetByIPOID(ctx context.Context, ipoID string) (*models.EnhancedGMPData, error)
	ListRecent(ctx context.Context, limit int) ([]map[string]interface{}, error)
}

type GMPHistoryRepository interface {
	GetByIPO(ctx context.Context, ipoID uuid.UUID, dateRange *models.DateRange) ([]models.GMPPriceHistoryEntry, string, error)
	GetArchivalStats(ctx context.Context) (int, *time.Time, *time.Time, error)
}

type CacheRepository interface {
	Store(ctx context.Context, result *models.IPOResultCache) error
	Get(ctx context.Context, ipoID uuid.UUID, panHash string) (*models.IPOResultCache, error)
	CleanupExpired(ctx context.Context) (int64, error)
}

type UpdateLogRepository interface {
	Create(ctx context.Context, entry *models.IPOUpdateLog) error
	GetByIPO(ctx context.Context, ipoID uuid.UUID, limit int) ([]models.IPOUpdateLog, error)
}

type DiagnosticsRepository interface {
	GetIndexUsageStats(ctx context.Context) ([]map[string]interface{}, error)
	AnalyzeQueryPlans(ctx context.Context, sampleIPOID string) (map[string][]string, error)
}
