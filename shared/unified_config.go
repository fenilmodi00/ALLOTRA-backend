package shared

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// UnifiedConfiguration holds all configuration parameters for the entire application
type UnifiedConfiguration struct {
	Service  ServiceConfig  `json:"service"`
	Database DatabaseConfig `json:"database"`
	Batch    BatchConfig    `json:"batch"`
	Cache    CacheConfig    `json:"cache"`
	Logging  LoggingConfig  `json:"logging"`
}

// ServiceConfig holds HTTP service configuration
type ServiceConfig struct {
	BaseURL            string        `json:"base_url"`
	HTTPRequestTimeout time.Duration `json:"http_timeout"`
	RequestRateLimit   time.Duration `json:"rate_limit"`
	MaxRetryAttempts   int           `json:"max_retries"`
	EnableMetrics      bool          `json:"enable_metrics"`
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `json:"conn_max_idle_time"`
	PingTimeout     time.Duration `json:"ping_timeout"`
}

// BatchConfig holds batch processing configuration
type BatchConfig struct {
	BatchSize       int           `json:"batch_size"`
	MaxConcurrency  int           `json:"max_concurrency"`
	Timeout         time.Duration `json:"timeout"`
	ErrorIsolation  bool          `json:"error_isolation"`
	TransactionMode bool          `json:"transaction_mode"`
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	DefaultTTL time.Duration `json:"default_ttl"`
	MaxSize    int           `json:"max_size"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level       string `json:"level"`
	Format      string `json:"format"`
	Output      string `json:"output"`
	EnableJSON  bool   `json:"enable_json"`
	ServiceName string `json:"service_name"`
}

// NewDefaultUnifiedConfiguration returns production-ready default configuration
func NewDefaultUnifiedConfiguration() *UnifiedConfiguration {
	return &UnifiedConfiguration{
		Service: ServiceConfig{
			BaseURL:            "https://www.chittorgarh.com",
			HTTPRequestTimeout: 30 * time.Second,
			RequestRateLimit:   1 * time.Second,
			MaxRetryAttempts:   3,
			EnableMetrics:      true,
		},
		Database: DatabaseConfig{
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
			PingTimeout:     5 * time.Second,
		},
		Batch: BatchConfig{
			BatchSize:       50,
			MaxConcurrency:  5,
			Timeout:         30 * time.Second,
			ErrorIsolation:  true,
			TransactionMode: true,
		},
		Cache: CacheConfig{
			DefaultTTL: 15 * time.Minute,
			MaxSize:    1000,
		},
		Logging: LoggingConfig{
			Level:       "info",
			Format:      "json",
			Output:      "stdout",
			EnableJSON:  true,
			ServiceName: "ipo-backend",
		},
	}
}

// NewGMPServiceConfig returns GMP-specific service configuration
func NewGMPServiceConfig() ServiceConfig {
	return ServiceConfig{
		BaseURL:            "https://www.investorgain.com/report/live-ipo-gmp/331/",
		HTTPRequestTimeout: 30 * time.Second,
		RequestRateLimit:   1 * time.Second,
		MaxRetryAttempts:   3,
		EnableMetrics:      true,
	}
}

// NewIPOScraperConfig returns IPO scraper-specific service configuration
func NewIPOScraperConfig() ServiceConfig {
	return ServiceConfig{
		BaseURL:            "https://www.chittorgarh.com",
		HTTPRequestTimeout: 30 * time.Second,
		RequestRateLimit:   2 * time.Second, // More conservative for scraping
		MaxRetryAttempts:   3,
		EnableMetrics:      true,
	}
}

// ValidateAndApplyDefaults validates configuration and applies defaults for invalid values
func (c *UnifiedConfiguration) ValidateAndApplyDefaults() {
	logger := logrus.WithField("component", "UnifiedConfiguration")

	// Validate Service Config
	if c.Service.BaseURL == "" {
		c.Service.BaseURL = "https://www.chittorgarh.com"
		logger.Debug("Applied default Service.BaseURL")
	}

	if c.Service.HTTPRequestTimeout <= 0 {
		c.Service.HTTPRequestTimeout = 30 * time.Second
		logger.Debug("Applied default Service.HTTPRequestTimeout")
	}

	if c.Service.RequestRateLimit <= 0 {
		c.Service.RequestRateLimit = 1 * time.Second
		logger.Debug("Applied default Service.RequestRateLimit")
	}

	if c.Service.MaxRetryAttempts <= 0 {
		c.Service.MaxRetryAttempts = 3
		logger.Debug("Applied default Service.MaxRetryAttempts")
	}

	// Validate Database Config
	if c.Database.MaxOpenConns <= 0 {
		c.Database.MaxOpenConns = 25
		logger.Debug("Applied default Database.MaxOpenConns")
	}

	if c.Database.MaxIdleConns <= 0 {
		c.Database.MaxIdleConns = 5
		logger.Debug("Applied default Database.MaxIdleConns")
	}

	if c.Database.ConnMaxLifetime <= 0 {
		c.Database.ConnMaxLifetime = 5 * time.Minute
		logger.Debug("Applied default Database.ConnMaxLifetime")
	}

	// Validate Batch Config
	if c.Batch.BatchSize <= 0 {
		c.Batch.BatchSize = 50
		logger.Debug("Applied default Batch.BatchSize")
	}

	if c.Batch.MaxConcurrency <= 0 {
		c.Batch.MaxConcurrency = 5
		logger.Debug("Applied default Batch.MaxConcurrency")
	}

	if c.Batch.Timeout <= 0 {
		c.Batch.Timeout = 30 * time.Second
		logger.Debug("Applied default Batch.Timeout")
	}

	// Validate Cache Config
	if c.Cache.DefaultTTL <= 0 {
		c.Cache.DefaultTTL = 15 * time.Minute
		logger.Debug("Applied default Cache.DefaultTTL")
	}

	if c.Cache.MaxSize <= 0 {
		c.Cache.MaxSize = 1000
		logger.Debug("Applied default Cache.MaxSize")
	}

	// Validate Logging Config
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
		logger.Debug("Applied default Logging.Level")
	}

	if c.Logging.Format == "" {
		c.Logging.Format = "json"
		logger.Debug("Applied default Logging.Format")
	}

	if c.Logging.ServiceName == "" {
		c.Logging.ServiceName = "ipo-backend"
		logger.Debug("Applied default Logging.ServiceName")
	}
}

// ToJSON serializes the configuration to JSON
func (c *UnifiedConfiguration) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// LoadFromJSON deserializes configuration from JSON
func (c *UnifiedConfiguration) LoadFromJSON(jsonData []byte) error {
	if err := json.Unmarshal(jsonData, c); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}
	c.ValidateAndApplyDefaults()
	return nil
}

// Clone creates a deep copy of the configuration
func (c *UnifiedConfiguration) Clone() *UnifiedConfiguration {
	jsonData, _ := c.ToJSON()
	clone := &UnifiedConfiguration{}
	clone.LoadFromJSON(jsonData)
	return clone
}
