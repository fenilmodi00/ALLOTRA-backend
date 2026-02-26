package repositories

import (
	"context"

	"github.com/fenilmodi00/ipo-backend/models"
)

type GMPRepository interface {
	GetByIPOID(ctx context.Context, ipoID string) (*models.EnhancedGMPData, error)
	ListRecent(ctx context.Context, limit int) ([]map[string]interface{}, error)
}

type DiagnosticsRepository interface {
	GetIndexUsageStats(ctx context.Context) ([]map[string]interface{}, error)
	AnalyzeQueryPlans(ctx context.Context, sampleIPOID string) (map[string][]string, error)
}
