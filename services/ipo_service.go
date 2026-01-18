package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/sirupsen/logrus"
)

// IPOAuditLogger provides comprehensive audit logging for IPO operations
type IPOAuditLogger struct {
	serviceName string
}

// NewIPOAuditLogger creates a new audit logger
func NewIPOAuditLogger() *IPOAuditLogger {
	return &IPOAuditLogger{
		serviceName: "ipo-service",
	}
}

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	ServiceName string                 `json:"service_name"`
	Operation   string                 `json:"operation"`
	EntityType  string                 `json:"entity_type"`
	EntityID    string                 `json:"entity_id"`
	UserID      *string                `json:"user_id,omitempty"`
	Changes     map[string]interface{} `json:"changes,omitempty"`
	BeforeData  interface{}            `json:"before_data,omitempty"`
	AfterData   interface{}            `json:"after_data,omitempty"`
	Success     bool                   `json:"success"`
	ErrorMsg    *string                `json:"error_msg,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// LogIPOCreation logs IPO creation with comprehensive details
func (a *IPOAuditLogger) LogIPOCreation(ipo *models.IPO, userID *string, success bool, errorMsg *string) {
	entry := AuditEntry{
		Timestamp:   time.Now(),
		ServiceName: a.serviceName,
		Operation:   "CREATE",
		EntityType:  "IPO",
		EntityID:    ipo.StockID,
		UserID:      userID,
		AfterData:   ipo,
		Success:     success,
		ErrorMsg:    errorMsg,
		Metadata: map[string]interface{}{
			"company_name": ipo.Name,
			"company_code": ipo.CompanyCode,
			"status":       ipo.Status,
		},
	}

	a.logAuditEntry(entry)
}

// LogIPOUpdate logs IPO updates with before/after comparison
func (a *IPOAuditLogger) LogIPOUpdate(beforeIPO, afterIPO *models.IPO, userID *string, success bool, errorMsg *string) {
	changes := a.calculateIPOChanges(beforeIPO, afterIPO)

	entry := AuditEntry{
		Timestamp:   time.Now(),
		ServiceName: a.serviceName,
		Operation:   "UPDATE",
		EntityType:  "IPO",
		EntityID:    afterIPO.StockID,
		UserID:      userID,
		Changes:     changes,
		BeforeData:  beforeIPO,
		AfterData:   afterIPO,
		Success:     success,
		ErrorMsg:    errorMsg,
		Metadata: map[string]interface{}{
			"company_name":  afterIPO.Name,
			"company_code":  afterIPO.CompanyCode,
			"status":        afterIPO.Status,
			"changes_count": len(changes),
		},
	}

	a.logAuditEntry(entry)
}

// LogBatchOperation logs batch operations with summary statistics
func (a *IPOAuditLogger) LogBatchOperation(operation string, totalCount, successCount, failureCount int, userID *string, errors []string) {
	entry := AuditEntry{
		Timestamp:   time.Now(),
		ServiceName: a.serviceName,
		Operation:   "BATCH_" + operation,
		EntityType:  "IPO",
		EntityID:    "BATCH",
		UserID:      userID,
		Success:     failureCount == 0,
		Metadata: map[string]interface{}{
			"total_count":   totalCount,
			"success_count": successCount,
			"failure_count": failureCount,
			"success_rate":  float64(successCount) / float64(totalCount),
			"errors":        errors,
		},
	}

	if failureCount > 0 {
		errorSummary := fmt.Sprintf("Batch operation had %d failures out of %d total operations", failureCount, totalCount)
		entry.ErrorMsg = &errorSummary
	}

	a.logAuditEntry(entry)
}

// calculateIPOChanges compares two IPO objects and returns the changes
func (a *IPOAuditLogger) calculateIPOChanges(before, after *models.IPO) map[string]interface{} {
	changes := make(map[string]interface{})

	if before.Name != after.Name {
		changes["name"] = map[string]interface{}{"before": before.Name, "after": after.Name}
	}
	if before.CompanyCode != after.CompanyCode {
		changes["company_code"] = map[string]interface{}{"before": before.CompanyCode, "after": after.CompanyCode}
	}
	if before.Status != after.Status {
		changes["status"] = map[string]interface{}{"before": before.Status, "after": after.Status}
	}
	if before.Registrar != after.Registrar {
		changes["registrar"] = map[string]interface{}{"before": before.Registrar, "after": after.Registrar}
	}

	// Compare price band
	if (before.PriceBandLow == nil) != (after.PriceBandLow == nil) ||
		(before.PriceBandLow != nil && after.PriceBandLow != nil && *before.PriceBandLow != *after.PriceBandLow) {
		changes["price_band_low"] = map[string]interface{}{"before": before.PriceBandLow, "after": after.PriceBandLow}
	}
	if (before.PriceBandHigh == nil) != (after.PriceBandHigh == nil) ||
		(before.PriceBandHigh != nil && after.PriceBandHigh != nil && *before.PriceBandHigh != *after.PriceBandHigh) {
		changes["price_band_high"] = map[string]interface{}{"before": before.PriceBandHigh, "after": after.PriceBandHigh}
	}

	// Compare dates
	if !a.compareDates(before.OpenDate, after.OpenDate) {
		changes["open_date"] = map[string]interface{}{"before": before.OpenDate, "after": after.OpenDate}
	}
	if !a.compareDates(before.CloseDate, after.CloseDate) {
		changes["close_date"] = map[string]interface{}{"before": before.CloseDate, "after": after.CloseDate}
	}
	if !a.compareDates(before.ListingDate, after.ListingDate) {
		changes["listing_date"] = map[string]interface{}{"before": before.ListingDate, "after": after.ListingDate}
	}

	// Compare optional fields
	if !a.compareStringPointers(before.Symbol, after.Symbol) {
		changes["symbol"] = map[string]interface{}{"before": before.Symbol, "after": after.Symbol}
	}
	if !a.compareStringPointers(before.Description, after.Description) {
		changes["description"] = map[string]interface{}{"before": before.Description, "after": after.Description}
	}

	return changes
}

// compareDates compares two time pointers
func (a *IPOAuditLogger) compareDates(date1, date2 *time.Time) bool {
	if date1 == nil && date2 == nil {
		return true
	}
	if date1 == nil || date2 == nil {
		return false
	}
	return date1.Equal(*date2)
}

// compareStringPointers compares two string pointers
func (a *IPOAuditLogger) compareStringPointers(str1, str2 *string) bool {
	if str1 == nil && str2 == nil {
		return true
	}
	if str1 == nil || str2 == nil {
		return false
	}
	return *str1 == *str2
}

// logAuditEntry logs the audit entry using structured logging
func (a *IPOAuditLogger) logAuditEntry(entry AuditEntry) {
	logFields := logrus.Fields{
		"audit_timestamp": entry.Timestamp,
		"service_name":    entry.ServiceName,
		"operation":       entry.Operation,
		"entity_type":     entry.EntityType,
		"entity_id":       entry.EntityID,
		"success":         entry.Success,
	}

	if entry.UserID != nil {
		logFields["user_id"] = *entry.UserID
	}

	if entry.ErrorMsg != nil {
		logFields["error_msg"] = *entry.ErrorMsg
	}

	if len(entry.Changes) > 0 {
		logFields["changes"] = entry.Changes
	}

	if entry.Metadata != nil {
		for key, value := range entry.Metadata {
			logFields["meta_"+key] = value
		}
	}

	if entry.Success {
		logrus.WithFields(logFields).Info("Audit log entry")
	} else {
		logrus.WithFields(logFields).Warn("Audit log entry - operation failed")
	}
}

type IPOService struct {
	DB             *sql.DB
	UtilityService *UtilityService
	auditLogger    *IPOAuditLogger
	dbOptimizer    *DatabaseOptimizer
	serviceMetrics *shared.ServiceMetrics
	dbMetrics      *shared.DatabaseMetrics
	httpMetrics    *shared.HTTPMetrics
}

// DatabaseOptimizer provides database optimization features
type DatabaseOptimizer struct {
	db             *sql.DB
	connectionPool *shared.DatabaseConfig
	retryConfig    *RetryConfig
	queryOptimizer *QueryOptimizer
}

// Note: ConnectionPoolConfig is now replaced by shared.DatabaseConfig

// RetryConfig holds retry configuration for database operations
type RetryConfig struct {
	MaxRetries    int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// QueryOptimizer provides query optimization features
type QueryOptimizer struct {
	enablePreparedStatements bool
	enableQueryLogging       bool
	slowQueryThreshold       time.Duration
}

// NewDatabaseOptimizer creates a new database optimizer
func NewDatabaseOptimizer(db *sql.DB) *DatabaseOptimizer {
	config := shared.NewDefaultUnifiedConfiguration()

	return &DatabaseOptimizer{
		db:             db,
		connectionPool: &config.Database,
		retryConfig: &RetryConfig{
			MaxRetries:    3,
			BaseDelay:     100 * time.Millisecond,
			MaxDelay:      2 * time.Second,
			BackoffFactor: 2.0,
		},
		queryOptimizer: &QueryOptimizer{
			enablePreparedStatements: true,
			enableQueryLogging:       true,
			slowQueryThreshold:       500 * time.Millisecond,
		},
	}
}

// ConfigureConnectionPool configures the database connection pool
func (opt *DatabaseOptimizer) ConfigureConnectionPool() {
	opt.db.SetMaxOpenConns(opt.connectionPool.MaxOpenConns)
	opt.db.SetMaxIdleConns(opt.connectionPool.MaxIdleConns)
	opt.db.SetConnMaxLifetime(opt.connectionPool.ConnMaxLifetime)
	opt.db.SetConnMaxIdleTime(opt.connectionPool.ConnMaxIdleTime)

	logrus.WithFields(logrus.Fields{
		"max_open_conns":     opt.connectionPool.MaxOpenConns,
		"max_idle_conns":     opt.connectionPool.MaxIdleConns,
		"conn_max_lifetime":  opt.connectionPool.ConnMaxLifetime,
		"conn_max_idle_time": opt.connectionPool.ConnMaxIdleTime,
	}).Info("Database connection pool configured")
}

// ExecuteWithRetry executes a database operation with exponential backoff retry
func (opt *DatabaseOptimizer) ExecuteWithRetry(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= opt.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate delay with exponential backoff
			delay := time.Duration(float64(opt.retryConfig.BaseDelay) *
				math.Pow(opt.retryConfig.BackoffFactor, float64(attempt-1)))

			if delay > opt.retryConfig.MaxDelay {
				delay = opt.retryConfig.MaxDelay
			}

			logrus.WithFields(logrus.Fields{
				"attempt": attempt,
				"delay":   delay,
				"error":   lastErr,
			}).Warn("Retrying database operation")

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		startTime := time.Now()
		err := operation()
		duration := time.Since(startTime)

		// Log slow queries
		if opt.queryOptimizer.enableQueryLogging && duration > opt.queryOptimizer.slowQueryThreshold {
			logrus.WithFields(logrus.Fields{
				"duration": duration,
				"attempt":  attempt,
			}).Warn("Slow database query detected")
		}

		if err == nil {
			if attempt > 0 {
				logrus.WithFields(logrus.Fields{
					"attempt":  attempt,
					"duration": duration,
				}).Info("Database operation succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !opt.isRetryableError(err) {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Debug("Non-retryable database error")
			return err
		}
	}

	logrus.WithFields(logrus.Fields{
		"max_retries": opt.retryConfig.MaxRetries,
		"final_error": lastErr,
	}).Error("Database operation failed after all retries")

	return fmt.Errorf("database operation failed after %d retries: %w", opt.retryConfig.MaxRetries, lastErr)
}

// isRetryableError determines if a database error is retryable
func (opt *DatabaseOptimizer) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Common retryable database errors
	retryableErrors := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"deadlock",
		"lock wait timeout",
		"connection lost",
		"server shutdown",
	}

	for _, retryableErr := range retryableErrors {
		if strings.Contains(errStr, retryableErr) {
			return true
		}
	}

	return false
}

func NewIPOService(db *sql.DB) *IPOService {
	utilityService := NewUtilityService()
	dbOptimizer := NewDatabaseOptimizer(db)

	// Configure connection pool
	dbOptimizer.ConfigureConnectionPool()

	return &IPOService{
		DB:             db,
		UtilityService: utilityService,
		auditLogger:    NewIPOAuditLogger(),
		dbOptimizer:    dbOptimizer,
		serviceMetrics: shared.NewServiceMetrics("IPO_Service"),
		dbMetrics:      shared.NewDatabaseMetrics(),
		httpMetrics:    shared.NewHTTPMetrics(),
	}
}

func (s *IPOService) recalculateStatus(ipo *models.IPO) {
	ipo.Status = s.UtilityService.CalculateIPOStatus(ipo.OpenDate, ipo.CloseDate, ipo.ListingDate)
}

// recalculateStatusWithGMP updates the status of an IPOWithGMP based on current time and dates
func (s *IPOService) recalculateStatusWithGMP(ipo *models.IPOWithGMP) {
	ipo.Status = s.UtilityService.CalculateIPOStatus(ipo.OpenDate, ipo.CloseDate, ipo.ListingDate)
}

// CalculateEnhancedIPOMetrics calculates enhanced metrics for IPO analysis
func (s *IPOService) CalculateEnhancedIPOMetrics(ipo *models.IPO) map[string]interface{} {
	metrics := make(map[string]interface{})

	// Calculate price band metrics
	if ipo.PriceBandLow != nil && ipo.PriceBandHigh != nil {
		priceBandRange := *ipo.PriceBandHigh - *ipo.PriceBandLow
		priceBandMidpoint := (*ipo.PriceBandLow + *ipo.PriceBandHigh) / 2
		priceBandSpread := (priceBandRange / *ipo.PriceBandLow) * 100

		metrics["price_band_range"] = priceBandRange
		metrics["price_band_midpoint"] = priceBandMidpoint
		metrics["price_band_spread_percent"] = priceBandSpread
	}

	// Calculate investment metrics
	if ipo.MinQty != nil && ipo.PriceBandLow != nil {
		minInvestmentLow := float64(*ipo.MinQty) * *ipo.PriceBandLow
		metrics["min_investment_low"] = minInvestmentLow

		if ipo.PriceBandHigh != nil {
			minInvestmentHigh := float64(*ipo.MinQty) * *ipo.PriceBandHigh
			metrics["min_investment_high"] = minInvestmentHigh
			metrics["min_investment_range"] = minInvestmentHigh - minInvestmentLow
		}
	}

	// Calculate timeline metrics
	if ipo.OpenDate != nil && ipo.CloseDate != nil {
		subscriptionDuration := ipo.CloseDate.Sub(*ipo.OpenDate)
		metrics["subscription_duration_days"] = subscriptionDuration.Hours() / 24

		// Calculate days until open/close
		now := time.Now()
		if ipo.OpenDate.After(now) {
			daysUntilOpen := ipo.OpenDate.Sub(now)
			metrics["days_until_open"] = daysUntilOpen.Hours() / 24
		}
		if ipo.CloseDate.After(now) {
			daysUntilClose := ipo.CloseDate.Sub(now)
			metrics["days_until_close"] = daysUntilClose.Hours() / 24
		}
	}

	// Calculate listing timeline
	if ipo.ListingDate != nil {
		now := time.Now()
		if ipo.ListingDate.After(now) {
			daysUntilListing := ipo.ListingDate.Sub(now)
			metrics["days_until_listing"] = daysUntilListing.Hours() / 24
		} else {
			daysSinceListing := now.Sub(*ipo.ListingDate)
			metrics["days_since_listing"] = daysSinceListing.Hours() / 24
		}
	}

	// Parse and calculate issue size metrics
	if ipo.IssueSize != nil && *ipo.IssueSize != "" {
		issueSizeValue := s.UtilityService.ExtractNumeric(*ipo.IssueSize)
		if issueSizeValue > 0 {
			metrics["issue_size_numeric"] = issueSizeValue

			// Categorize issue size
			if issueSizeValue < 100 {
				metrics["issue_size_category"] = "Small"
			} else if issueSizeValue < 500 {
				metrics["issue_size_category"] = "Medium"
			} else {
				metrics["issue_size_category"] = "Large"
			}
		}
	}

	// Calculate listing gain metrics
	if ipo.ListingGain != nil && *ipo.ListingGain != "" {
		listingGainValue := s.UtilityService.ExtractSignedPercentage(*ipo.ListingGain)
		if listingGainValue != nil {
			metrics["listing_gain_percent"] = *listingGainValue

			// Categorize listing performance
			if *listingGainValue > 50 {
				metrics["listing_performance"] = "Excellent"
			} else if *listingGainValue > 20 {
				metrics["listing_performance"] = "Good"
			} else if *listingGainValue > 0 {
				metrics["listing_performance"] = "Positive"
			} else if *listingGainValue == 0 {
				metrics["listing_performance"] = "Flat"
			} else {
				metrics["listing_performance"] = "Negative"
			}
		}
	}

	return metrics
}

// CalculateIPOValuation calculates estimated valuation metrics
func (s *IPOService) CalculateIPOValuation(ipo *models.IPO) map[string]interface{} {
	valuation := make(map[string]interface{})

	// Calculate market cap estimates
	if ipo.PriceBandLow != nil && ipo.PriceBandHigh != nil && ipo.IssueSize != nil {
		issueSizeValue := s.UtilityService.ExtractNumeric(*ipo.IssueSize)
		if issueSizeValue > 0 {
			// Estimate shares based on issue size and price band
			estimatedSharesLow := (issueSizeValue * 10000000) / *ipo.PriceBandHigh // Convert Cr to actual value
			estimatedSharesHigh := (issueSizeValue * 10000000) / *ipo.PriceBandLow

			valuation["estimated_shares_low"] = estimatedSharesLow
			valuation["estimated_shares_high"] = estimatedSharesHigh

			// Calculate market cap range
			marketCapLow := estimatedSharesHigh * *ipo.PriceBandLow / 10000000 // Convert back to Cr
			marketCapHigh := estimatedSharesLow * *ipo.PriceBandHigh / 10000000

			valuation["estimated_market_cap_low_cr"] = marketCapLow
			valuation["estimated_market_cap_high_cr"] = marketCapHigh
		}
	}

	return valuation
}

// CalculateRiskMetrics calculates risk assessment metrics
func (s *IPOService) CalculateRiskMetrics(ipo *models.IPO) map[string]interface{} {
	riskMetrics := make(map[string]interface{})
	riskScore := 0.0
	riskFactors := []string{}

	// Price band spread risk
	if ipo.PriceBandLow != nil && ipo.PriceBandHigh != nil {
		priceBandSpread := ((*ipo.PriceBandHigh - *ipo.PriceBandLow) / *ipo.PriceBandLow) * 100
		riskMetrics["price_band_spread_percent"] = priceBandSpread

		if priceBandSpread > 20 {
			riskScore += 2.0
			riskFactors = append(riskFactors, "High price band spread")
		} else if priceBandSpread > 10 {
			riskScore += 1.0
			riskFactors = append(riskFactors, "Moderate price band spread")
		}
	}

	// Timeline risk
	if ipo.OpenDate != nil && ipo.CloseDate != nil {
		subscriptionDuration := ipo.CloseDate.Sub(*ipo.OpenDate)
		subscriptionDays := subscriptionDuration.Hours() / 24

		if subscriptionDays < 3 {
			riskScore += 1.5
			riskFactors = append(riskFactors, "Short subscription period")
		} else if subscriptionDays > 10 {
			riskScore += 1.0
			riskFactors = append(riskFactors, "Extended subscription period")
		}
	}

	// Issue size risk
	if ipo.IssueSize != nil && *ipo.IssueSize != "" {
		issueSizeValue := s.UtilityService.ExtractNumeric(*ipo.IssueSize)
		if issueSizeValue > 1000 {
			riskScore += 1.5
			riskFactors = append(riskFactors, "Large issue size")
		} else if issueSizeValue < 50 {
			riskScore += 1.0
			riskFactors = append(riskFactors, "Small issue size")
		}
	}

	// Minimum investment risk
	if ipo.MinAmount != nil && *ipo.MinAmount > 200000 {
		riskScore += 1.0
		riskFactors = append(riskFactors, "High minimum investment")
	}

	// Calculate overall risk level
	var riskLevel string
	if riskScore >= 5.0 {
		riskLevel = "High"
	} else if riskScore >= 3.0 {
		riskLevel = "Medium"
	} else if riskScore >= 1.0 {
		riskLevel = "Low"
	} else {
		riskLevel = "Very Low"
	}

	riskMetrics["risk_score"] = riskScore
	riskMetrics["risk_level"] = riskLevel
	riskMetrics["risk_factors"] = riskFactors

	return riskMetrics
}

// GetIPOsWithOptimizedQuery retrieves IPOs using optimized query patterns
func (s *IPOService) GetIPOsWithOptimizedQuery(ctx context.Context, status string, limit, offset int) ([]models.IPO, error) {
	// Use prepared statement for better performance
	baseQuery := `SELECT id, name, company_code, description, price_band_low, price_band_high, 
              issue_size, open_date, close_date, result_date, registrar, stock_id, 
              form_url, form_fields, form_headers, parser_config, status, subscription_status,
              symbol, slug, listing_date, listing_gain, min_qty, min_amount,
              logo_url, about, strengths, risks, created_at, updated_at, created_by
              FROM ipo_list`

	var query string
	var args []interface{}
	argIndex := 1

	// Build WHERE clause dynamically
	var conditions []string
	if status != "" && status != "all" {
		switch status {
		case "live":
			conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
			args = append(args, "LIVE")
			argIndex++
		case "upcoming":
			conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
			args = append(args, "UPCOMING")
			argIndex++
		case "closed":
			conditions = append(conditions, fmt.Sprintf("status IN ($%d, $%d)", argIndex, argIndex+1))
			args = append(args, "CLOSED", "RESULT_OUT")
			argIndex += 2
		}
	}

	// Add WHERE clause if conditions exist
	if len(conditions) > 0 {
		query = baseQuery + " WHERE " + strings.Join(conditions, " AND ")
	} else {
		query = baseQuery
	}

	// Add ORDER BY and LIMIT/OFFSET for pagination
	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, limit)
		argIndex++
	}

	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, offset)
	}

	// Execute query with retry logic
	var rows *sql.Rows
	var err error

	err = s.dbOptimizer.ExecuteWithRetry(ctx, func() error {
		rows, err = s.DB.QueryContext(ctx, query, args...)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query IPOs with optimization: %w", err)
	}
	defer rows.Close()

	var ipos []models.IPO
	for rows.Next() {
		var ipo models.IPO
		var formFields, formHeaders, parserConfig, strengths, risks []byte
		err := rows.Scan(
			&ipo.ID, &ipo.Name, &ipo.CompanyCode, &ipo.Description, &ipo.PriceBandLow, &ipo.PriceBandHigh,
			&ipo.IssueSize, &ipo.OpenDate, &ipo.CloseDate, &ipo.ResultDate, &ipo.Registrar, &ipo.StockID,
			&ipo.FormURL, &formFields, &formHeaders, &parserConfig, &ipo.Status, &ipo.SubscriptionStatus,
			&ipo.Symbol, &ipo.Slug, &ipo.ListingDate, &ipo.ListingGain, &ipo.MinQty, &ipo.MinAmount,
			&ipo.LogoURL, &ipo.About, &strengths, &risks, &ipo.CreatedAt, &ipo.UpdatedAt, &ipo.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan IPO row: %w", err)
		}
		ipo.FormFields = json.RawMessage(formFields)
		ipo.FormHeaders = json.RawMessage(formHeaders)
		ipo.ParserConfig = json.RawMessage(parserConfig)
		ipo.Strengths = json.RawMessage(strengths)
		ipo.Risks = json.RawMessage(risks)

		// Recalculate status based on current time
		s.recalculateStatus(&ipo)

		ipos = append(ipos, ipo)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating IPO rows: %w", err)
	}

	return ipos, nil
}

func (s *IPOService) GetActiveIPOs(ctx context.Context) ([]models.IPO, error) {
	// Optimized query with IN clause instead of OR - including all fields
	query := `SELECT id, name, company_code, description, price_band_low, price_band_high, 
              issue_size, open_date, close_date, result_date, registrar, stock_id, 
              form_url, form_fields, form_headers, parser_config, status, subscription_status,
              symbol, slug, listing_date, listing_gain, min_qty, min_amount,
              logo_url, about, strengths, risks, created_at, updated_at, created_by
              FROM ipo_list WHERE status IN ('LIVE', 'RESULT_OUT') ORDER BY created_at DESC LIMIT 100`

	rows, err := s.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active IPOs: %w", err)
	}
	defer rows.Close()

	var ipos []models.IPO
	for rows.Next() {
		var ipo models.IPO
		var formFields, formHeaders, parserConfig, strengths, risks []byte
		err := rows.Scan(
			&ipo.ID, &ipo.Name, &ipo.CompanyCode, &ipo.Description, &ipo.PriceBandLow, &ipo.PriceBandHigh,
			&ipo.IssueSize, &ipo.OpenDate, &ipo.CloseDate, &ipo.ResultDate, &ipo.Registrar, &ipo.StockID,
			&ipo.FormURL, &formFields, &formHeaders, &parserConfig, &ipo.Status, &ipo.SubscriptionStatus,
			&ipo.Symbol, &ipo.Slug, &ipo.ListingDate, &ipo.ListingGain, &ipo.MinQty, &ipo.MinAmount,
			&ipo.LogoURL, &ipo.About, &strengths, &risks, &ipo.CreatedAt, &ipo.UpdatedAt, &ipo.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan IPO row: %w", err)
		}
		ipo.FormFields = json.RawMessage(formFields)
		ipo.FormHeaders = json.RawMessage(formHeaders)
		ipo.ParserConfig = json.RawMessage(parserConfig)
		ipo.Strengths = json.RawMessage(strengths)
		ipo.Risks = json.RawMessage(risks)

		// Recalculate status based on current time
		s.recalculateStatus(&ipo)

		ipos = append(ipos, ipo)
	}
	return ipos, nil
}

func (s *IPOService) GetIPOs(ctx context.Context, status string) ([]models.IPO, error) {
	baseQuery := `SELECT id, name, company_code, description, price_band_low, price_band_high, 
              issue_size, open_date, close_date, result_date, registrar, stock_id, 
              form_url, form_fields, form_headers, parser_config, status, subscription_status,
              symbol, slug, listing_date, listing_gain, min_qty, min_amount,
              logo_url, about, strengths, risks, created_at, updated_at, created_by
              FROM ipo_list`

	var query string
	var args []interface{}

	// Handle status filtering
	switch status {
	case "live":
		query = baseQuery + ` WHERE status = 'LIVE'`
	case "upcoming":
		query = baseQuery + ` WHERE status = 'UPCOMING'`
	case "closed":
		query = baseQuery + ` WHERE status = 'CLOSED' OR status = 'RESULT_OUT'`
	case "all", "":
		query = baseQuery // No filter, return all
	default:
		// If an invalid status is provided, treat it as "all"
		query = baseQuery
	}

	query += ` ORDER BY created_at DESC`

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query IPOs: %w", err)
	}
	defer rows.Close()

	var ipos []models.IPO
	for rows.Next() {
		var ipo models.IPO
		var formFields, formHeaders, parserConfig, strengths, risks []byte
		err := rows.Scan(
			&ipo.ID, &ipo.Name, &ipo.CompanyCode, &ipo.Description, &ipo.PriceBandLow, &ipo.PriceBandHigh,
			&ipo.IssueSize, &ipo.OpenDate, &ipo.CloseDate, &ipo.ResultDate, &ipo.Registrar, &ipo.StockID,
			&ipo.FormURL, &formFields, &formHeaders, &parserConfig, &ipo.Status, &ipo.SubscriptionStatus,
			&ipo.Symbol, &ipo.Slug, &ipo.ListingDate, &ipo.ListingGain, &ipo.MinQty, &ipo.MinAmount,
			&ipo.LogoURL, &ipo.About, &strengths, &risks, &ipo.CreatedAt, &ipo.UpdatedAt, &ipo.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan IPO row: %w", err)
		}
		ipo.FormFields = json.RawMessage(formFields)
		ipo.FormHeaders = json.RawMessage(formHeaders)
		ipo.ParserConfig = json.RawMessage(parserConfig)
		ipo.Strengths = json.RawMessage(strengths)
		ipo.Risks = json.RawMessage(risks)

		// Recalculate status based on current time
		s.recalculateStatus(&ipo)

		ipos = append(ipos, ipo)
	}
	return ipos, nil
}

func (s *IPOService) GetIPOByID(ctx context.Context, id string) (*models.IPO, error) {
	query := `SELECT id, name, company_code, description, price_band_low, price_band_high, 
              issue_size, open_date, close_date, result_date, registrar, stock_id, 
              form_url, form_fields, form_headers, parser_config, status, subscription_status,
              symbol, slug, listing_date, listing_gain, min_qty, min_amount,
              logo_url, about, strengths, risks, created_at, updated_at, created_by
              FROM ipo_list WHERE id = $1`

	row := s.DB.QueryRowContext(ctx, query, id)
	var ipo models.IPO
	var formFields, formHeaders, parserConfig, strengths, risks []byte
	err := row.Scan(
		&ipo.ID, &ipo.Name, &ipo.CompanyCode, &ipo.Description, &ipo.PriceBandLow, &ipo.PriceBandHigh,
		&ipo.IssueSize, &ipo.OpenDate, &ipo.CloseDate, &ipo.ResultDate, &ipo.Registrar, &ipo.StockID,
		&ipo.FormURL, &formFields, &formHeaders, &parserConfig, &ipo.Status, &ipo.SubscriptionStatus,
		&ipo.Symbol, &ipo.Slug, &ipo.ListingDate, &ipo.ListingGain, &ipo.MinQty, &ipo.MinAmount,
		&ipo.LogoURL, &ipo.About, &strengths, &risks, &ipo.CreatedAt, &ipo.UpdatedAt, &ipo.CreatedBy,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan IPO: %w", err)
	}
	ipo.FormFields = json.RawMessage(formFields)
	ipo.FormHeaders = json.RawMessage(formHeaders)
	ipo.ParserConfig = json.RawMessage(parserConfig)
	ipo.Strengths = json.RawMessage(strengths)
	ipo.Risks = json.RawMessage(risks)

	// Recalculate status based on current time
	s.recalculateStatus(&ipo)

	return &ipo, nil
}

// GetIPOByStockID returns an IPO by its stock ID
func (s *IPOService) GetIPOByStockID(ctx context.Context, stockID string) (*models.IPO, error) {
	query := `SELECT id, name, company_code, description, price_band_low, price_band_high, 
              issue_size, open_date, close_date, result_date, registrar, stock_id, 
              form_url, form_fields, form_headers, parser_config, status, subscription_status,
              symbol, slug, listing_date, listing_gain, min_qty, min_amount,
              logo_url, about, strengths, risks, created_at, updated_at, created_by
              FROM ipo_list WHERE stock_id = $1`

	row := s.DB.QueryRowContext(ctx, query, stockID)
	var ipo models.IPO
	var formFields, formHeaders, parserConfig, strengths, risks []byte
	err := row.Scan(
		&ipo.ID, &ipo.Name, &ipo.CompanyCode, &ipo.Description, &ipo.PriceBandLow, &ipo.PriceBandHigh,
		&ipo.IssueSize, &ipo.OpenDate, &ipo.CloseDate, &ipo.ResultDate, &ipo.Registrar, &ipo.StockID,
		&ipo.FormURL, &formFields, &formHeaders, &parserConfig, &ipo.Status, &ipo.SubscriptionStatus,
		&ipo.Symbol, &ipo.Slug, &ipo.ListingDate, &ipo.ListingGain, &ipo.MinQty, &ipo.MinAmount,
		&ipo.LogoURL, &ipo.About, &strengths, &risks, &ipo.CreatedAt, &ipo.UpdatedAt, &ipo.CreatedBy,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan IPO: %w", err)
	}
	ipo.FormFields = json.RawMessage(formFields)
	ipo.FormHeaders = json.RawMessage(formHeaders)
	ipo.ParserConfig = json.RawMessage(parserConfig)
	ipo.Strengths = json.RawMessage(strengths)
	ipo.Risks = json.RawMessage(risks)

	// Recalculate status based on current time
	s.recalculateStatus(&ipo)

	return &ipo, nil
}

func (s *IPOService) CreateIPO(ctx context.Context, ipo *models.IPO) error {
	// Generate derived fields if missing
	if ipo.CompanyCode == "" {
		ipo.CompanyCode = s.UtilityService.GenerateCompanyCode(ipo.Name)
	}
	if ipo.Slug == nil || *ipo.Slug == "" {
		slug := s.UtilityService.GenerateSlug(ipo.Name)
		ipo.Slug = &slug
	}

	query := `INSERT INTO ipo_list (name, company_code, description, price_band_low, price_band_high, 
              issue_size, open_date, close_date, result_date, registrar, stock_id, 
              form_url, form_fields, form_headers, parser_config, status, created_by) 
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17) RETURNING id`

	err := s.DB.QueryRowContext(ctx, query,
		ipo.Name, ipo.CompanyCode, ipo.Description, ipo.PriceBandLow, ipo.PriceBandHigh,
		ipo.IssueSize, ipo.OpenDate, ipo.CloseDate, ipo.ResultDate, ipo.Registrar, ipo.StockID,
		ipo.FormURL, ipo.FormFields, ipo.FormHeaders, ipo.ParserConfig, ipo.Status, ipo.CreatedBy,
	).Scan(&ipo.ID)

	// Log audit entry for creation attempt
	var errorMsg *string
	if err != nil {
		errStr := err.Error()
		errorMsg = &errStr
	}
	s.auditLogger.LogIPOCreation(ipo, ipo.CreatedBy, err == nil, errorMsg)

	if err != nil {
		return fmt.Errorf("failed to create IPO: %w", err)
	}

	// Log successful creation
	logrus.WithFields(logrus.Fields{
		"ipo_name":     ipo.Name,
		"company_code": ipo.CompanyCode,
	}).Info("IPO created successfully")

	return nil
}

func (s *IPOService) UpsertIPO(ctx context.Context, item models.IPO) error {
	// Get existing IPO for audit comparison if it exists
	var existingIPO *models.IPO
	if existing, err := s.GetIPOByStockID(ctx, item.StockID); err == nil && existing != nil {
		existingIPO = existing
	}

	// Generate derived fields if missing
	if item.CompanyCode == "" {
		item.CompanyCode = s.UtilityService.GenerateCompanyCode(item.Name)
	}
	if item.Slug == nil || *item.Slug == "" {
		slug := s.UtilityService.GenerateSlug(item.Name)
		item.Slug = &slug
	}

	query := `
		INSERT INTO ipo_list (
			name, company_code, symbol, slug, 
			description, price_band_low, price_band_high, issue_size,
			open_date, close_date, listing_date, result_date,
			listing_gain, min_qty, min_amount,
			logo_url, about, strengths, risks,
			status, registrar, stock_id, form_url, form_fields, parser_config
		) VALUES (
			$1, $2, $3, $4, 
			$5, $6, $7, $8,
			$9, $10, $11, $12,
			$13, $14, $15,
			$16, $17, $18, $19,
			$20, $21, $22, '', '{}', '{}'
		)
		ON CONFLICT (stock_id) DO UPDATE SET
			name = EXCLUDED.name,
			company_code = EXCLUDED.company_code,
			symbol = EXCLUDED.symbol,
			slug = EXCLUDED.slug,
			description = EXCLUDED.description,
			price_band_low = EXCLUDED.price_band_low,
			price_band_high = EXCLUDED.price_band_high,
			issue_size = EXCLUDED.issue_size,
			open_date = EXCLUDED.open_date,
			close_date = EXCLUDED.close_date,
			listing_date = EXCLUDED.listing_date,
			result_date = EXCLUDED.result_date,
			listing_gain = EXCLUDED.listing_gain,
			min_qty = EXCLUDED.min_qty,
			min_amount = EXCLUDED.min_amount,
			logo_url = EXCLUDED.logo_url,
			about = EXCLUDED.about,
			strengths = EXCLUDED.strengths,
			risks = EXCLUDED.risks,
			status = EXCLUDED.status,
			registrar = EXCLUDED.registrar,
			updated_at = CURRENT_TIMESTAMP;
	`

	// Ensure JSON fields are valid
	if len(item.Strengths) == 0 {
		item.Strengths = json.RawMessage("[]")
	}
	if len(item.Risks) == 0 {
		item.Risks = json.RawMessage("[]")
	}

	// Set default values if not provided
	status := item.Status
	if status == "" {
		status = "Active"
	}

	registrar := item.Registrar
	if registrar == "" {
		registrar = "Unknown"
	}

	_, err := s.DB.ExecContext(ctx, query,
		item.Name, item.CompanyCode, item.Symbol, item.Slug,
		item.Description, item.PriceBandLow, item.PriceBandHigh, item.IssueSize,
		item.OpenDate, item.CloseDate, item.ListingDate, item.ResultDate,
		item.ListingGain, item.MinQty, item.MinAmount,
		item.LogoURL, item.About, item.Strengths, item.Risks,
		status, registrar, item.StockID,
	)

	// Log audit entry for upsert operation
	var errorMsg *string
	if err != nil {
		errStr := err.Error()
		errorMsg = &errStr
	}

	if existingIPO != nil {
		// This was an update
		s.auditLogger.LogIPOUpdate(existingIPO, &item, item.CreatedBy, err == nil, errorMsg)
	} else {
		// This was a creation
		s.auditLogger.LogIPOCreation(&item, item.CreatedBy, err == nil, errorMsg)
	}

	// Log successful upsert
	if err == nil {
		logrus.WithFields(logrus.Fields{
			"ipo_name":     item.Name,
			"company_code": item.CompanyCode,
			"stock_id":     item.StockID,
		}).Info("IPO upserted successfully")
	}

	return err
}

// GetActiveIPOsWithGMP returns all IPOs that have GMP data available, joined by company_code or name
// Uses INNER JOIN to ensure only IPOs with corresponding GMP data are returned
// Matches on: company_code OR case-insensitive name comparison
func (s *IPOService) GetActiveIPOsWithGMP(ctx context.Context) ([]models.IPOWithGMP, error) {
	// Query to get all IPOs that have corresponding GMP data (INNER JOIN ensures only IPOs with GMP data)
	query := `
		SELECT 
			i.id, i.name, i.company_code, i.description, i.price_band_low, i.price_band_high,
			i.issue_size, i.open_date, i.close_date, i.result_date, i.registrar, i.stock_id,
			i.form_url, i.form_fields, i.form_headers, i.parser_config, i.status, i.subscription_status,
			i.symbol, i.slug, i.listing_date, i.listing_gain, i.min_qty, i.min_amount,
			i.logo_url, i.about, i.strengths, i.risks, i.created_at, i.updated_at, i.created_by,
			g.gmp_value, g.gain_percent, g.estimated_listing, g.last_updated,
			g.stock_id, g.subscription_status, g.listing_gain, g.ipo_status, 
			g.data_source, g.extraction_metadata
		FROM ipo_list i
		INNER JOIN ipo_gmp g ON (
			-- Primary: Use stock_id for linking when available
			(i.stock_id IS NOT NULL AND g.stock_id IS NOT NULL AND i.stock_id = g.stock_id)
			-- Fallback: Exact company code match
			OR i.company_code = g.company_code 
			-- Fallback: Exact name match
			OR LOWER(TRIM(i.name)) = LOWER(TRIM(g.ipo_name))
			-- Fuzzy matching: Remove common suffixes and check if names contain each other
			OR LOWER(REPLACE(REPLACE(REPLACE(REPLACE(i.name, ' Ltd.', ''), ' Limited', ''), ' IPO', ''), ' Inc.', '')) 
			   LIKE '%' || LOWER(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(g.ipo_name, ' Ltd.', ''), ' Limited', ''), ' IPO', ''), ' BSE SME', ''), ' NSE SME', ''), ' L@', '')) || '%'
			-- Reverse fuzzy matching
			OR LOWER(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(g.ipo_name, ' Ltd.', ''), ' Limited', ''), ' IPO', ''), ' BSE SME', ''), ' NSE SME', ''), ' L@', '')) 
			   LIKE '%' || LOWER(REPLACE(REPLACE(REPLACE(REPLACE(i.name, ' Ltd.', ''), ' Limited', ''), ' IPO', ''), ' Inc.', '')) || '%'
			-- Match first few words (for cases like "KSH International" matching "KSH International IPO")
			OR LOWER(SPLIT_PART(TRIM(i.name), ' ', 1) || ' ' || SPLIT_PART(TRIM(i.name), ' ', 2)) = 
			   LOWER(SPLIT_PART(TRIM(g.ipo_name), ' ', 1) || ' ' || SPLIT_PART(TRIM(g.ipo_name), ' ', 2))
		)
		ORDER BY 
			-- Prioritize stock_id matches
			CASE 
				WHEN i.stock_id IS NOT NULL AND g.stock_id IS NOT NULL AND i.stock_id = g.stock_id THEN 1
				WHEN i.company_code = g.company_code THEN 2
				ELSE 3
			END,
			CASE 
				WHEN CURRENT_TIMESTAMP BETWEEN COALESCE(i.open_date, '1900-01-01') AND COALESCE(i.close_date, '2100-01-01') THEN 1
				WHEN i.open_date IS NOT NULL AND i.open_date > CURRENT_TIMESTAMP THEN 2
				WHEN i.close_date IS NOT NULL AND i.close_date > CURRENT_TIMESTAMP - INTERVAL '30 days' THEN 3
				ELSE 4
			END,
			g.last_updated DESC,
			i.created_at DESC
		LIMIT 100
	`

	rows, err := s.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active IPOs with GMP: %w", err)
	}
	defer rows.Close()

	var ipos []models.IPOWithGMP
	for rows.Next() {
		var ipo models.IPOWithGMP
		var formFields, formHeaders, parserConfig, strengths, risks []byte
		var extractionMetadataBytes sql.NullString

		err := rows.Scan(
			&ipo.ID, &ipo.Name, &ipo.CompanyCode, &ipo.Description, &ipo.PriceBandLow, &ipo.PriceBandHigh,
			&ipo.IssueSize, &ipo.OpenDate, &ipo.CloseDate, &ipo.ResultDate, &ipo.Registrar, &ipo.StockID,
			&ipo.FormURL, &formFields, &formHeaders, &parserConfig, &ipo.Status, &ipo.SubscriptionStatus,
			&ipo.Symbol, &ipo.Slug, &ipo.ListingDate, &ipo.ListingGain, &ipo.MinQty, &ipo.MinAmount,
			&ipo.LogoURL, &ipo.About, &strengths, &risks, &ipo.CreatedAt, &ipo.UpdatedAt, &ipo.CreatedBy,
			&ipo.GMPValue, &ipo.GainPercent, &ipo.EstimatedListing, &ipo.GMPLastUpdated,
			&ipo.GMPStockID, &ipo.GMPSubscriptionStatus, &ipo.GMPListingGain, &ipo.GMPIPOStatus,
			&ipo.GMPDataSource, &extractionMetadataBytes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan IPO with GMP row: %w", err)
		}

		// Convert byte arrays to json.RawMessage
		ipo.FormFields = json.RawMessage(formFields)
		ipo.FormHeaders = json.RawMessage(formHeaders)
		ipo.ParserConfig = json.RawMessage(parserConfig)
		ipo.Strengths = json.RawMessage(strengths)
		ipo.Risks = json.RawMessage(risks)

		// Parse extraction metadata JSON if present
		if extractionMetadataBytes.Valid && extractionMetadataBytes.String != "" {
			var metadata models.ExtractionMetadata
			if err := json.Unmarshal([]byte(extractionMetadataBytes.String), &metadata); err == nil {
				ipo.GMPExtractionMetadata = &metadata
			}
		}

		// Recalculate status based on current time
		s.recalculateStatusWithGMP(&ipo)

		ipos = append(ipos, ipo)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating IPO with GMP rows: %w", err)
	}

	return ipos, nil
}

// GetIPOByIDWithGMP returns a single IPO with GMP data joined by company_code
func (s *IPOService) GetIPOByIDWithGMP(ctx context.Context, id string) (*models.IPOWithGMP, error) {
	query := `
		SELECT 
			i.id, i.name, i.company_code, i.description, i.price_band_low, i.price_band_high,
			i.issue_size, i.open_date, i.close_date, i.result_date, i.registrar, i.stock_id,
			i.form_url, i.form_fields, i.form_headers, i.parser_config, i.status, i.subscription_status,
			i.symbol, i.slug, i.listing_date, i.listing_gain, i.min_qty, i.min_amount,
			i.logo_url, i.about, i.strengths, i.risks, i.created_at, i.updated_at, i.created_by,
			g.gmp_value, g.gain_percent, g.estimated_listing, g.last_updated,
			g.stock_id, g.subscription_status, g.listing_gain, g.ipo_status, 
			g.data_source, g.extraction_metadata
		FROM ipo_list i
		LEFT JOIN ipo_gmp g ON (
			-- Primary: Use stock_id for linking when available
			(i.stock_id IS NOT NULL AND g.stock_id IS NOT NULL AND i.stock_id = g.stock_id)
			-- Fallback: company_code match
			OR i.company_code = g.company_code
		)
		WHERE i.id = $1
		ORDER BY 
			-- Prioritize stock_id matches
			CASE 
				WHEN i.stock_id IS NOT NULL AND g.stock_id IS NOT NULL AND i.stock_id = g.stock_id THEN 1
				WHEN i.company_code = g.company_code THEN 2
				ELSE 3
			END,
			g.last_updated DESC
		LIMIT 1
	`

	row := s.DB.QueryRowContext(ctx, query, id)
	var ipo models.IPOWithGMP
	var formFields, formHeaders, parserConfig, strengths, risks []byte
	var extractionMetadataBytes sql.NullString

	err := row.Scan(
		&ipo.ID, &ipo.Name, &ipo.CompanyCode, &ipo.Description, &ipo.PriceBandLow, &ipo.PriceBandHigh,
		&ipo.IssueSize, &ipo.OpenDate, &ipo.CloseDate, &ipo.ResultDate, &ipo.Registrar, &ipo.StockID,
		&ipo.FormURL, &formFields, &formHeaders, &parserConfig, &ipo.Status, &ipo.SubscriptionStatus,
		&ipo.Symbol, &ipo.Slug, &ipo.ListingDate, &ipo.ListingGain, &ipo.MinQty, &ipo.MinAmount,
		&ipo.LogoURL, &ipo.About, &strengths, &risks, &ipo.CreatedAt, &ipo.UpdatedAt, &ipo.CreatedBy,
		&ipo.GMPValue, &ipo.GainPercent, &ipo.EstimatedListing, &ipo.GMPLastUpdated,
		&ipo.GMPStockID, &ipo.GMPSubscriptionStatus, &ipo.GMPListingGain, &ipo.GMPIPOStatus,
		&ipo.GMPDataSource, &extractionMetadataBytes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan IPO with GMP: %w", err)
	}

	// Convert byte arrays to json.RawMessage
	ipo.FormFields = json.RawMessage(formFields)
	ipo.FormHeaders = json.RawMessage(formHeaders)
	ipo.ParserConfig = json.RawMessage(parserConfig)
	ipo.Strengths = json.RawMessage(strengths)
	ipo.Risks = json.RawMessage(risks)

	// Parse extraction metadata JSON if present
	if extractionMetadataBytes.Valid && extractionMetadataBytes.String != "" {
		var metadata models.ExtractionMetadata
		if err := json.Unmarshal([]byte(extractionMetadataBytes.String), &metadata); err == nil {
			ipo.GMPExtractionMetadata = &metadata
		}
	}

	// Recalculate status based on current time
	s.recalculateStatusWithGMP(&ipo)

	return &ipo, nil
}

// GetServiceMetrics returns the current service metrics
func (s *IPOService) GetServiceMetrics() *shared.ServiceMetrics {
	return s.serviceMetrics
}

// GetDatabaseMetrics returns the current database metrics
func (s *IPOService) GetDatabaseMetrics() *shared.DatabaseMetrics {
	return s.dbMetrics
}

// GetHTTPMetrics returns the current HTTP metrics
func (s *IPOService) GetHTTPMetrics() *shared.HTTPMetrics {
	return s.httpMetrics
}

// LogMetricsSummary logs comprehensive metrics summary for all tracked metrics
func (s *IPOService) LogMetricsSummary() {
	if s.serviceMetrics != nil {
		s.serviceMetrics.LogSummary()
	}
	if s.dbMetrics != nil {
		s.dbMetrics.LogDatabaseSummary()
	}
	if s.httpMetrics != nil {
		s.httpMetrics.LogHTTPSummary()
	}
}

// RecordServiceOperation records a service operation with metrics tracking
func (s *IPOService) RecordServiceOperation(operationName string, success bool, processingTime time.Duration) {
	if s.serviceMetrics != nil {
		s.serviceMetrics.RecordRequest(success, processingTime)
		s.serviceMetrics.IncrementCustomCounter(operationName)
	}
}

// RecordDatabaseOperation records a database operation with metrics tracking
func (s *IPOService) RecordDatabaseOperation(success bool, queryTime time.Duration, isSlowQuery bool) {
	if s.dbMetrics != nil {
		s.dbMetrics.RecordQuery(success, queryTime, isSlowQuery)
	}
}

// RecordHTTPOperation records an HTTP operation with metrics tracking
func (s *IPOService) RecordHTTPOperation(success bool, statusCode int, responseTime time.Duration, errorType string, isTimeout bool) {
	if s.httpMetrics != nil {
		s.httpMetrics.RecordHTTPRequest(success, statusCode, responseTime, errorType, isTimeout)
	}
}

// GetMetricsSnapshot returns a comprehensive snapshot of all metrics
func (s *IPOService) GetMetricsSnapshot() map[string]interface{} {
	snapshot := make(map[string]interface{})

	if s.serviceMetrics != nil {
		snapshot["service_metrics"] = s.serviceMetrics.GetSnapshot()
	}

	if s.dbMetrics != nil {
		snapshot["database_metrics"] = map[string]interface{}{
			"total_queries":         s.dbMetrics.TotalQueries,
			"successful_queries":    s.dbMetrics.SuccessfulQueries,
			"failed_queries":        s.dbMetrics.FailedQueries,
			"slow_queries":          s.dbMetrics.SlowQueries,
			"query_success_rate":    s.dbMetrics.GetQuerySuccessRate(),
			"average_query_time":    s.dbMetrics.AverageQueryTime,
			"connection_pool_stats": s.dbMetrics.ConnectionPoolStats,
		}
	}

	if s.httpMetrics != nil {
		snapshot["http_metrics"] = map[string]interface{}{
			"total_requests":        s.httpMetrics.TotalRequests,
			"successful_requests":   s.httpMetrics.SuccessfulRequests,
			"failed_requests":       s.httpMetrics.FailedRequests,
			"timeout_requests":      s.httpMetrics.TimeoutRequests,
			"retry_attempts":        s.httpMetrics.RetryAttempts,
			"http_success_rate":     s.httpMetrics.GetHTTPSuccessRate(),
			"average_response_time": s.httpMetrics.AverageResponseTime,
			"status_code_counts":    s.httpMetrics.StatusCodeCounts,
			"error_counts":          s.httpMetrics.ErrorCounts,
		}
	}

	return snapshot
}

// ResetMetrics resets all metrics to zero
func (s *IPOService) ResetMetrics() {
	if s.serviceMetrics != nil {
		s.serviceMetrics.Reset()
	}
	if s.dbMetrics != nil {
		s.dbMetrics = shared.NewDatabaseMetrics()
	}
	if s.httpMetrics != nil {
		s.httpMetrics = shared.NewHTTPMetrics()
	}

	logrus.WithField("service", "IPO_Service").Info("All metrics reset")
}
