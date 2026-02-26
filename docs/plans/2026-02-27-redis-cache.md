# Redis Cache Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Completely replace the internal memory cache `CacheService` map structure with Redis for persistence, speed, scalability, and stateless deployments.

**Architecture:** Initialize a Redis client on app startup, pass it into a reimagined `CacheService`, convert existing struct-based caching to use JSON marshaling for Redis storage, and adjust existing endpoints (Get/Set/Delete algorithms) to talk over the network to the Leapcell Redis endpoint.

**Tech Stack:** Go 1.24, Fiber web framework, `github.com/redis/go-redis/v9`

---

### Task 1: Update Application Container and Inject Redis

**Files:**
- Modify: `internal/app/server.go`

**Step 1: Write the failing test**

*(No formal unit test for dependency injection step, but we will test compilation.)*

**Step 2: Write minimal implementation**

Modify `internal/app/server.go` to import `"github.com/redis/go-redis/v9"`, parse the Redis URL, and pass the Redis instance to `NewCacheServiceWithConfig`.

```go
// In internal/app/server.go under import
import "github.com/redis/go-redis/v9"

// Inside Run() function before cacheService initialization:
	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("failed to parse redis url: %w", err)
	}
	redisClient := redis.NewClient(redisOpt)
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logrus.WithError(err).Warn("Failed to connect to Redis, cache may be unavailable")
	}

	cacheService := services.NewCacheServiceWithConfig(
		db,
		redisClient, // <--- Add this argument
		cacheConfig.DefaultTTL,
		cacheConfig.MaxSize, // Note: MaxSize might be ignored or handled differently later
	)
```

**Step 3: Run test to verify it fails**

Run: `go build ./...`
Expected: FAIL with "too many arguments in call to services.NewCacheServiceWithConfig"

**Step 4: Commit**

```bash
git add internal/app/server.go
git commit -m "feat: setup redis client injection"
```

---

### Task 2: Refactor `CacheService` Struct & Constructor

**Files:**
- Modify: `services/cache_service.go`

**Step 1: Write minimal implementation**

Modify `CacheService` to drop the `cache map[string]*CacheEntry` and `sync.RWMutex` in favor of a `*redis.Client`.

```go
// In services/cache_service.go
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

// Omit CacheEntry and IsExpired methods as Redis handles TTL natively

type CacheService struct {
	redisClient *redis.Client
	defaultTTL  time.Duration
	maxSize     int // Maintained for backward compatibility on constructor if needed
	DB          *sql.DB
}

// Update Constructors
func NewCacheService(db *sql.DB, redisClient *redis.Client) *CacheService {
	cs := &CacheService{
		redisClient: redisClient,
		defaultTTL:  5 * time.Minute,
		maxSize:     1000,
		DB:          db,
	}
	// Note: We no longer need memory cleanup routines (go cs.cleanupExpired())
	return cs
}

func NewCacheServiceWithConfig(db *sql.DB, redisClient *redis.Client, defaultTTL time.Duration, maxSize int) *CacheService {
	cs := &CacheService{
		redisClient: redisClient,
		defaultTTL:  defaultTTL,
		maxSize:     maxSize,
		DB:          db,
	}
	return cs
}
```

Remove `cleanupExpired()` and `evictOldest()`.

**Step 2: Run test to verify it passes partially**

Run: `go build ./...`
Expected: FAIL due to missing `Get`, `Set`, `Delete`, `Clear`, `Size` methods which references old map.

**Step 3: Commit**

```bash
git add services/cache_service.go
git commit -m "refactor: convert CacheService struct to redis client"
```

---

### Task 3: Implement Redis Core Methods (`Set`, `Get`, `Delete`)

**Files:**
- Modify: `services/cache_service.go`

**Step 1: Write minimal implementation**

Replace the existing `Get`, `Set`, `Size`, `Clear` and `Delete` methods. Since values must be `json` strings in Redis, we will accept raw bytes or handle JSON conversion up the stack. But to prevent massive breaking changes, let's keep `interface{}` and serialize/deserialize on the fly, storing as string.

```go
// SetWithTTL stores a value in cache with custom TTL
func (cs *CacheService) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	ctx := context.Background()
	
	bytes, err := json.Marshal(value)
	if err != nil {
		logrus.WithError(err).Error("failed to serialize cache value")
		return
	}

	err = cs.redisClient.Set(ctx, key, string(bytes), ttl).Err()
	if err != nil {
		logrus.WithError(err).Error("failed to set redis cache")
	}
}

// Set stores a value in cache with default TTL
func (cs *CacheService) Set(key string, value interface{}) {
	cs.SetWithTTL(key, value, cs.defaultTTL)
}

// Delete removes a value from cache
func (cs *CacheService) Delete(key string) {
	ctx := context.Background()
	cs.redisClient.Del(ctx, key)
}

// Clear removes all values from cache
func (cs *CacheService) Clear() {
	ctx := context.Background()
	cs.redisClient.FlushDB(ctx)
}

// Size returns the approximate number of items in cache using DBSize
func (cs *CacheService) Size() int {
	ctx := context.Background()
	val, err := cs.redisClient.DBSize(ctx).Result()
	if err != nil {
		return 0
	}
	return int(val)
}

// Get raw JSON string. The caller MUST decode it, because we 
// cannot dynamically cast `interface{}` to a strongly typed struct on return.
func (cs *CacheService) GetRaw(key string) (string, bool) {
	ctx := context.Background()
	val, err := cs.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false
	} else if err != nil {
		logrus.WithError(err).Error("failed to get redis cache")
		return "", false
	}
	return val, true
}
```

Remove the old `Get` method, which means all places calling `cs.cache.Get` or `cs.Get` will break. 

**Step 2: Commit**

```bash
git add services/cache_service.go
git commit -m "feat: implement core redis commands with json marshal"
```

---

### Task 4: Fix `CachedIPOService` Get operations

Because `Get` now returns a raw JSON string instead of `interface{}`, we must unmarshal the JSON strings back to their expected structs directly inside the calling service (`CachedIPOService`).

**Files:**
- Modify: `services/cache_service.go` Lines ~180-280

**Step 1: Write minimal implementation**

Update `CachedIPOService` to use JSON Unmarshaling.

```go
func (cis *CachedIPOService) GetActiveIPOsWithGMP(ctx context.Context) ([]models.IPOWithGMP, error) {
	cacheKey := "active_ipos_with_gmp"

	if raw, found := cis.cache.GetRaw(cacheKey); found {
		var cached []models.IPOWithGMP
		if err := json.Unmarshal([]byte(raw), &cached); err == nil {
			return cached, nil
		}
	}
    // ... existing DB fetch ...
	ipos, err := cis.ipoService.GetActiveIPOsWithGMP(ctx)
	if err != nil { return nil, err }
	cis.cache.SetWithTTL(cacheKey, ipos, 5*time.Minute)
	return ipos, nil
}

func (cis *CachedIPOService) GetIPOByIDWithGMP(ctx context.Context, id string) (*models.IPOWithGMP, error) {
	cacheKey := fmt.Sprintf("ipo_with_gmp:%s", id)

	if raw, found := cis.cache.GetRaw(cacheKey); found {
		var cached models.IPOWithGMP
		if err := json.Unmarshal([]byte(raw), &cached); err == nil {
			return &cached, nil
		}
	}
    // ... existing DB fetch ...
}

func (cis *CachedIPOService) GetActiveIPOs(ctx context.Context) ([]models.IPO, error) {
	cacheKey := "active_ipos"

	if raw, found := cis.cache.GetRaw(cacheKey); found {
		var cached []models.IPO
		if err := json.Unmarshal([]byte(raw), &cached); err == nil {
			return cached, nil
		}
	}
	// ... existing DB fetch ...
}

func (cis *CachedIPOService) GetIPOs(ctx context.Context, status string) ([]models.IPO, error) {
	cacheKey := fmt.Sprintf("ipos:%s", status)

	if raw, found := cis.cache.GetRaw(cacheKey); found {
		var cached []models.IPO
		if err := json.Unmarshal([]byte(raw), &cached); err == nil {
			return cached, nil
		}
	}
	// ... existing DB fetch ...
}

func (cis *CachedIPOService) GetIPOByID(ctx context.Context, id string) (*models.IPO, error) {
	cacheKey := fmt.Sprintf("ipo:%s", id)

	if raw, found := cis.cache.GetRaw(cacheKey); found {
		var cached models.IPO
		if err := json.Unmarshal([]byte(raw), &cached); err == nil {
			return &cached, nil
		}
	}
	// ... existing DB fetch ...
}

func (cis *CachedIPOService) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"size": cis.cache.Size(),
		"type": "redis",
	}
}
```

**Step 2: Commit**

```bash
git add services/cache_service.go
git commit -m "refactor: implement json unmarshaling for CachedIPOService"
```

---

### Task 5: Refactor `GMPHistoryService` Get Operations

Similar to `CachedIPOService`, `GMPHistoryService` uses `s.cache.Get()`. Need to convert to `GetRaw()` and unmarshal strings.

**Files:**
- Modify: `services/gmp_history_service.go`

**Step 1: Write minimal implementation**

Locate `func (s *GMPHistoryService) GetPriceHistoryByIPO` and change the retrieval mechanism.

```go
// Inside GetPriceHistoryByIPO
	if raw, found := s.cache.GetRaw(cacheKey); found {
		var collection models.GMPPriceHistoryCollection
		if err := json.Unmarshal([]byte(raw), &collection); err == nil {
			s.logger.WithFields(logrus.Fields{
				"ipo_id":    ipoID,
				"cache_key": cacheKey,
				"cache_hit": true,
			}).Debug("Price history retrieved from cache")
			return &collection, nil
		}
	}
```

Locate `s.cache.Clear()` (in `InvalidateAllCache`) and `s.cache.Delete()` (in `InvalidateIPOCache`) - these should already map correctly since they use string keys.
Inside `GetCacheStats`, update cache type: `"type": "redis"`.
Update the constructor: `NewGMPHistoryService` initialize the internal `s.cache` instance properly since we altered the constructor dependencies.

```go
// Change NewGMPHistoryService logic where it makes a mock db:
func NewGMPHistoryService(db *sql.DB, redisClient *redis.Client) *GMPHistoryService { 
	// Make sure to add redis client argument in internal/app/server.go
	// and pass it in!
	cache := NewCacheServiceWithConfig(db, redisClient, 10*time.Minute, 500)
    // ...
```
*(You will need to update the server.go file calling `NewGMPHistoryService(db)` to be `NewGMPHistoryService(db, redisClient)`).*

**Step 2: Commit**

```bash
git add services/gmp_history_service.go internal/app/server.go
git commit -m "refactor: apply redis caching on gmp history operations"
```

---

### Task 6: Resolve Build Issues & Unit Tests

**Files:**
- Modify: `tests/gmp_history_cache_test.go`
- Modify: `tests/gmp_history_circuit_breaker_test.go`

Since we modified signatures (`NewCacheService` / `NewGMPHistoryService`), some test setups will fail because `redisClient` expects to be passed down.

**Step 1: Write minimal implementation**

In your tests (`tests/gmp_history_cache_test.go`, etc):
Mock the redis client via `miniredis` or simply return empty structs if testing purely conceptually, otherwise adjust the constructors to pass `nil` to test fallback flows if Redis is down.

```go
// Inside test files where NewGMPHistoryService(db) is called:
// Create a dummy redis options:
opt, _ := redis.ParseURL("redis://localhost:6379")
dummyRedis := redis.NewClient(opt)
service := services.NewGMPHistoryService(db, dummyRedis)
```

**Step 2: Run test to verify it passes**

Run: `go build ./...`
Run: `go test ./...`
Expected: Build passes, tests might pass or fail depending on if your local environment runs Redis. 

**Step 3: Commit**

```bash
git add tests/
git commit -m "test: fix constructor invocations for tests"
```
