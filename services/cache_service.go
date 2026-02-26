package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// CacheService provides unified caching solution with Redis and database persistence.
// Redis handles TTL expiration and eviction natively, replacing the previous in-memory map.
type CacheService struct {
	redisClient *redis.Client
	defaultTTL  time.Duration
	DB          *sql.DB
}

// NewCacheService creates a new cache service backed by Redis with default settings.
func NewCacheService(db *sql.DB, redisClient *redis.Client) *CacheService {
	return &CacheService{
		redisClient: redisClient,
		defaultTTL:  5 * time.Minute,
		DB:          db,
	}
}

// NewCacheServiceWithConfig creates a cache service backed by Redis with custom configuration.
func NewCacheServiceWithConfig(db *sql.DB, redisClient *redis.Client, defaultTTL time.Duration, maxSize int) *CacheService {
	_ = maxSize // Redis handles eviction, parameter kept for API compatibility
	return &CacheService{
		redisClient: redisClient,
		defaultTTL:  defaultTTL,
		DB:          db,
	}
}

// GetRaw retrieves a raw JSON string from Redis by key.
// Returns ("", false) on redis.Nil or any error.
// Returns ("", false) gracefully if redisClient is nil (for testing).
func (cs *CacheService) GetRaw(key string) (string, bool) {
	if cs.redisClient == nil {
		return "", false
	}
	ctx := context.Background()
	val, err := cs.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			logrus.WithError(err).WithField("key", key).Error("Redis GET failed")
		}
		return "", false
	}
	return val, true
}

// Set stores a value in Redis with the default TTL.
func (cs *CacheService) Set(key string, value interface{}) {
	cs.SetWithTTL(key, value, cs.defaultTTL)
}

// SetWithTTL stores a value in Redis with a custom TTL.
// The value is JSON-marshalled before storage.
// No-op if redisClient is nil (for testing).
func (cs *CacheService) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	if cs.redisClient == nil {
		return
	}
	ctx := context.Background()
	data, err := json.Marshal(value)
	if err != nil {
		logrus.WithError(err).WithField("key", key).Error("Failed to marshal cache value")
		return
	}
	if err := cs.redisClient.Set(ctx, key, string(data), ttl).Err(); err != nil {
		logrus.WithError(err).WithField("key", key).Error("Redis SET failed")
	}
}

// Delete removes a value from Redis.
// No-op if redisClient is nil (for testing).
func (cs *CacheService) Delete(key string) {
	if cs.redisClient == nil {
		return
	}
	ctx := context.Background()
	if err := cs.redisClient.Del(ctx, key).Err(); err != nil {
		logrus.WithError(err).WithField("key", key).Error("Redis DEL failed")
	}
}

// Clear removes all keys in the current Redis database.
// No-op if redisClient is nil (for testing).
func (cs *CacheService) Clear() {
	if cs.redisClient == nil {
		return
	}
	ctx := context.Background()
	if err := cs.redisClient.FlushDB(ctx).Err(); err != nil {
		logrus.WithError(err).Error("Redis FLUSHDB failed")
	}
}

// Size returns the number of keys in the current Redis database.
// Returns 0 if redisClient is nil (for testing).
func (cs *CacheService) Size() int {
	if cs.redisClient == nil {
		return 0
	}
	ctx := context.Background()
	val, err := cs.redisClient.DBSize(ctx).Result()
	if err != nil {
		logrus.WithError(err).Error("Redis DBSIZE failed")
		return 0
	}
	return int(val)
}

// CachedIPOService wraps IPOService with caching capabilities
type CachedIPOService struct {
	ipoService *IPOService
	cache      *CacheService
}

// NewCachedIPOService creates a new cached IPO service
func NewCachedIPOService(ipoService *IPOService, cache *CacheService) *CachedIPOService {
	return &CachedIPOService{
		ipoService: ipoService,
		cache:      cache,
	}
}

// GetActiveIPOsWithGMP returns active IPOs with GMP data, using cache when possible
func (cis *CachedIPOService) GetActiveIPOsWithGMP(ctx context.Context) ([]models.IPOWithGMP, error) {
	cacheKey := "active_ipos_with_gmp"

	// Try to get from cache first
	if raw, found := cis.cache.GetRaw(cacheKey); found {
		var ipos []models.IPOWithGMP
		if err := json.Unmarshal([]byte(raw), &ipos); err == nil {
			return ipos, nil
		}
		logrus.WithField("key", cacheKey).Error("Failed to unmarshal cached value")
	}

	// Cache miss - fetch from database
	ipos, err := cis.ipoService.GetActiveIPOsWithGMP(ctx)
	if err != nil {
		return nil, err
	}

	// Cache the result for 5 minutes (active IPOs don't change frequently)
	cis.cache.SetWithTTL(cacheKey, ipos, 5*time.Minute)

	return ipos, nil
}

// GetIPOByIDWithGMP returns a single IPO with GMP data, using cache when possible
func (cis *CachedIPOService) GetIPOByIDWithGMP(ctx context.Context, id string) (*models.IPOWithGMP, error) {
	cacheKey := fmt.Sprintf("ipo_with_gmp:%s", id)

	// Try to get from cache first
	if raw, found := cis.cache.GetRaw(cacheKey); found {
		var ipo models.IPOWithGMP
		if err := json.Unmarshal([]byte(raw), &ipo); err == nil {
			return &ipo, nil
		}
		logrus.WithField("key", cacheKey).Error("Failed to unmarshal cached value")
	}

	// Cache miss - fetch from database
	ipo, err := cis.ipoService.GetIPOByIDWithGMP(ctx, id)
	if err != nil {
		return nil, err
	}

	if ipo != nil {
		// Cache the result for 10 minutes (individual IPOs are accessed frequently)
		cis.cache.SetWithTTL(cacheKey, ipo, 10*time.Minute)
	}

	return ipo, nil
}

// GetActiveIPOs returns active IPOs using cache when possible
func (cis *CachedIPOService) GetActiveIPOs(ctx context.Context) ([]models.IPO, error) {
	cacheKey := "active_ipos"

	// Try to get from cache first
	if raw, found := cis.cache.GetRaw(cacheKey); found {
		var ipos []models.IPO
		if err := json.Unmarshal([]byte(raw), &ipos); err == nil {
			return ipos, nil
		}
		logrus.WithField("key", cacheKey).Error("Failed to unmarshal cached value")
	}

	// Cache miss - fetch from database
	ipos, err := cis.ipoService.GetActiveIPOs(ctx)
	if err != nil {
		return nil, err
	}

	// Cache the result for 5 minutes
	cis.cache.SetWithTTL(cacheKey, ipos, 5*time.Minute)

	return ipos, nil
}

// GetIPOs returns IPOs with status filter, using cache when possible
func (cis *CachedIPOService) GetIPOs(ctx context.Context, status string) ([]models.IPO, error) {
	cacheKey := fmt.Sprintf("ipos:%s", status)

	// Try to get from cache first
	if raw, found := cis.cache.GetRaw(cacheKey); found {
		var ipos []models.IPO
		if err := json.Unmarshal([]byte(raw), &ipos); err == nil {
			return ipos, nil
		}
		logrus.WithField("key", cacheKey).Error("Failed to unmarshal cached value")
	}

	// Cache miss - fetch from database
	ipos, err := cis.ipoService.GetIPOs(ctx, status)
	if err != nil {
		return nil, err
	}

	// Cache the result for 3 minutes (filtered results may change more frequently)
	cis.cache.SetWithTTL(cacheKey, ipos, 3*time.Minute)

	return ipos, nil
}

// GetIPOByID returns a single IPO by ID, using cache when possible
func (cis *CachedIPOService) GetIPOByID(ctx context.Context, id string) (*models.IPO, error) {
	cacheKey := fmt.Sprintf("ipo:%s", id)

	// Try to get from cache first
	if raw, found := cis.cache.GetRaw(cacheKey); found {
		var ipo models.IPO
		if err := json.Unmarshal([]byte(raw), &ipo); err == nil {
			return &ipo, nil
		}
		logrus.WithField("key", cacheKey).Error("Failed to unmarshal cached value")
	}

	// Cache miss - fetch from database
	ipo, err := cis.ipoService.GetIPOByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if ipo != nil {
		// Cache the result for 15 minutes (individual IPO details are relatively static)
		cis.cache.SetWithTTL(cacheKey, ipo, 15*time.Minute)
	}

	return ipo, nil
}

// InvalidateIPOCache removes IPO-related cache entries
func (cis *CachedIPOService) InvalidateIPOCache(ipoID string) {
	// Remove specific IPO caches
	cis.cache.Delete(fmt.Sprintf("ipo:%s", ipoID))
	cis.cache.Delete(fmt.Sprintf("ipo_with_gmp:%s", ipoID))

	// Remove list caches (they may contain the updated IPO)
	cis.cache.Delete("active_ipos")
	cis.cache.Delete("active_ipos_with_gmp")
	cis.cache.Delete("ipos:all")
	cis.cache.Delete("ipos:live")
	cis.cache.Delete("ipos:upcoming")
	cis.cache.Delete("ipos:closed")
}

// InvalidateAllIPOCache removes all IPO-related cache entries
func (cis *CachedIPOService) InvalidateAllIPOCache() {
	cis.cache.Clear()
}

// GetCacheStats returns cache statistics
func (cis *CachedIPOService) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"size": cis.cache.Size(),
		"type": "redis",
	}
}

// WarmupCache pre-loads frequently accessed data into cache
func (cis *CachedIPOService) WarmupCache(ctx context.Context) error {
	// Pre-load active IPOs
	_, err := cis.GetActiveIPOs(ctx)
	if err != nil {
		return fmt.Errorf("failed to warmup active IPOs cache: %w", err)
	}

	// Pre-load active IPOs with GMP
	_, err = cis.GetActiveIPOsWithGMP(ctx)
	if err != nil {
		return fmt.Errorf("failed to warmup active IPOs with GMP cache: %w", err)
	}

	return nil
}

// Database cache methods for IPO results

// StoreResult stores an IPO result in the database cache
func (cs *CacheService) StoreResult(ctx context.Context, result *models.IPOResultCache) error {
	query := `
		INSERT INTO ipo_result_cache (
			pan_hash, ipo_id, status, shares_allotted, application_number,
			refund_status, source, user_agent, timestamp, expires_at,
			confidence_score, duplicate_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (pan_hash, ipo_id) DO UPDATE SET
			status = EXCLUDED.status,
			shares_allotted = EXCLUDED.shares_allotted,
			application_number = EXCLUDED.application_number,
			refund_status = EXCLUDED.refund_status,
			timestamp = EXCLUDED.timestamp,
			duplicate_count = ipo_result_cache.duplicate_count + 1
	`

	_, err := cs.DB.ExecContext(ctx, query,
		result.PanHash, result.IPOID, result.Status, result.SharesAllotted,
		result.ApplicationNumber, result.RefundStatus, result.Source,
		result.UserAgent, result.Timestamp, result.ExpiresAt,
		result.ConfidenceScore, result.DuplicateCount,
	)

	return err
}

// GetCachedResult retrieves a cached IPO result from database
func (cs *CacheService) GetCachedResult(ctx context.Context, ipoID, panHash string) (*models.IPOResultCache, error) {
	query := `
		SELECT id, pan_hash, ipo_id, status, shares_allotted, application_number,
		       refund_status, source, user_agent, timestamp, expires_at,
		       confidence_score, duplicate_count
		FROM ipo_result_cache
		WHERE ipo_id = $1 AND pan_hash = $2 AND expires_at > NOW()
	`

	var result models.IPOResultCache
	err := cs.DB.QueryRowContext(ctx, query, ipoID, panHash).Scan(
		&result.ID, &result.PanHash, &result.IPOID, &result.Status,
		&result.SharesAllotted, &result.ApplicationNumber, &result.RefundStatus,
		&result.Source, &result.UserAgent, &result.Timestamp, &result.ExpiresAt,
		&result.ConfidenceScore, &result.DuplicateCount,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &result, nil
}
