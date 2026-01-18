package services

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
)

// CacheEntry represents a cached item with expiration
type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

// IsExpired checks if the cache entry has expired
func (ce *CacheEntry) IsExpired() bool {
	return time.Now().After(ce.ExpiresAt)
}

// CacheService provides unified caching solution with both in-memory and database persistence.
// This consolidated service eliminates the need for separate memory and database cache implementations.
// It supports:
// - In-memory caching with TTL and automatic cleanup
// - Database persistence for IPO results
// - Thread-safe operations with read/write locks
// - Configurable TTL for different cache types
type CacheService struct {
	cache      map[string]*CacheEntry
	mutex      sync.RWMutex
	defaultTTL time.Duration
	maxSize    int
	DB         *sql.DB // Database for persistent caching
}

// NewCacheService creates a new consolidated cache service with default TTL.
// This replaces the need for separate memory and database cache services.
func NewCacheService(db *sql.DB) *CacheService {
	cs := &CacheService{
		cache:      make(map[string]*CacheEntry),
		defaultTTL: 5 * time.Minute, // Default 5 minute TTL
		maxSize:    1000,            // Default max size
		DB:         db,
	}

	// Start cleanup goroutine
	go cs.cleanupExpired()

	return cs
}

// NewCacheServiceWithConfig creates a cache service with custom configuration
func NewCacheServiceWithConfig(db *sql.DB, defaultTTL time.Duration, maxSize int) *CacheService {
	cs := &CacheService{
		cache:      make(map[string]*CacheEntry),
		defaultTTL: defaultTTL,
		maxSize:    maxSize,
		DB:         db,
	}

	// Start cleanup goroutine
	go cs.cleanupExpired()

	return cs
}

// Get retrieves a value from cache
func (cs *CacheService) Get(key string) (interface{}, bool) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	entry, exists := cs.cache[key]
	if !exists || entry.IsExpired() {
		return nil, false
	}

	return entry.Data, true
}

// Set stores a value in cache with default TTL
func (cs *CacheService) Set(key string, value interface{}) {
	cs.SetWithTTL(key, value, cs.defaultTTL)
}

// SetWithTTL stores a value in cache with custom TTL
func (cs *CacheService) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// Check if we're at max size and need to evict
	if len(cs.cache) >= cs.maxSize {
		cs.evictOldest()
	}

	cs.cache[key] = &CacheEntry{
		Data:      value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// evictOldest removes the oldest entry from cache (simple FIFO eviction)
func (cs *CacheService) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range cs.cache {
		if oldestKey == "" || entry.ExpiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.ExpiresAt
		}
	}

	if oldestKey != "" {
		delete(cs.cache, oldestKey)
	}
}

// Delete removes a value from cache
func (cs *CacheService) Delete(key string) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	delete(cs.cache, key)
}

// Clear removes all values from cache
func (cs *CacheService) Clear() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	cs.cache = make(map[string]*CacheEntry)
}

// Size returns the number of items in cache
func (cs *CacheService) Size() int {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	return len(cs.cache)
}

// cleanupExpired removes expired entries from cache
func (cs *CacheService) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	for range ticker.C {
		cs.mutex.Lock()
		for key, entry := range cs.cache {
			if entry.IsExpired() {
				delete(cs.cache, key)
			}
		}
		cs.mutex.Unlock()
	}
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
	if cached, found := cis.cache.Get(cacheKey); found {
		if ipos, ok := cached.([]models.IPOWithGMP); ok {
			return ipos, nil
		}
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
	if cached, found := cis.cache.Get(cacheKey); found {
		if ipo, ok := cached.(*models.IPOWithGMP); ok {
			return ipo, nil
		}
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
	if cached, found := cis.cache.Get(cacheKey); found {
		if ipos, ok := cached.([]models.IPO); ok {
			return ipos, nil
		}
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
	if cached, found := cis.cache.Get(cacheKey); found {
		if ipos, ok := cached.([]models.IPO); ok {
			return ipos, nil
		}
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
	if cached, found := cis.cache.Get(cacheKey); found {
		if ipo, ok := cached.(*models.IPO); ok {
			return ipo, nil
		}
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
	// This is a simple approach - in production, you might want to use cache tags
	cis.cache.Clear()
}

// GetCacheStats returns cache statistics
func (cis *CachedIPOService) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"size": cis.cache.Size(),
		"type": "in-memory",
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

// CleanupExpiredDB removes expired cache entries from database
func (cs *CacheService) CleanupExpiredDB(ctx context.Context) error {
	query := `DELETE FROM ipo_result_cache WHERE expires_at < NOW()`

	result, err := cs.DB.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("Cleaned up %d expired database cache entries\n", rowsAffected)

	return nil
}
