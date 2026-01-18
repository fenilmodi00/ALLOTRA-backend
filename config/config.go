package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

type Config struct {
	ServerPort      string
	DatabaseURL     string
	AdminToken      string
	CacheTTLHours   string
	LogLevel        string
	IPOAlertsAPIKey string
}

// SimplifiedRateLimitConfig holds simplified rate limiting configuration
type SimplifiedRateLimitConfig struct {
	RequestsPerSecond float64       `json:"requests_per_second"`
	PolitenessDelay   time.Duration `json:"politeness_delay"`
}

// DefaultRateLimitConfig returns default rate limiting configuration for politeness
func DefaultRateLimitConfig() *SimplifiedRateLimitConfig {
	return &SimplifiedRateLimitConfig{
		RequestsPerSecond: 2.0,                    // 2 requests per second for politeness
		PolitenessDelay:   500 * time.Millisecond, // Additional delay between requests
	}
}

// SimplifiedCacheConfig holds simplified cache configuration
type SimplifiedCacheConfig struct {
	DefaultTTL time.Duration `json:"default_ttl"`
	MaxSize    int           `json:"max_size"`
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() *SimplifiedCacheConfig {
	return &SimplifiedCacheConfig{
		DefaultTTL: 5 * time.Minute, // Default 5 minute TTL
		MaxSize:    1000,            // Maximum 1000 items in memory
	}
}

// GetCacheTTL returns the cache TTL from environment or default
func (c *Config) GetCacheTTL() time.Duration {
	if c.CacheTTLHours == "" {
		return 24 * time.Hour
	}

	hours, err := strconv.Atoi(c.CacheTTLHours)
	if err != nil {
		logrus.Warnf("Invalid CACHE_TTL_HOURS value: %s, using default 24 hours", c.CacheTTLHours)
		return 24 * time.Hour
	}

	return time.Duration(hours) * time.Hour
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		logrus.Warn("Error loading .env file, using system environment variables")
	}

	return &Config{
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		AdminToken:      getEnv("ADMIN_TOKEN", ""),
		CacheTTLHours:   getEnv("CACHE_TTL_HOURS", "24"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		IPOAlertsAPIKey: getEnv("IPO_ALERTS_API_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
