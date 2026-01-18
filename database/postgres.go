package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fenilmodi00/ipo-backend/shared"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

var DB *sql.DB

// Note: EnhancedConnectionConfig is now replaced by shared.DatabaseConfig
// Use shared.NewDefaultUnifiedConfiguration().Database for database configuration

// Connect establishes database connection with enhanced configuration
func Connect(dbURL string) error {
	config := shared.NewDefaultUnifiedConfiguration().Database
	return ConnectWithConfig(dbURL, &config)
}

// ConnectWithConfig establishes database connection with custom configuration
func ConnectWithConfig(dbURL string, config *shared.DatabaseConfig) error {
	var err error
	DB, err = sql.Open("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	// Apply enhanced connection pool configuration
	DB.SetMaxOpenConns(config.MaxOpenConns)
	DB.SetMaxIdleConns(config.MaxIdleConns)
	DB.SetConnMaxLifetime(config.ConnMaxLifetime)
	DB.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.PingTimeout)
	defer cancel()

	if err = DB.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"max_open_conns":     config.MaxOpenConns,
		"max_idle_conns":     config.MaxIdleConns,
		"conn_max_lifetime":  config.ConnMaxLifetime,
		"conn_max_idle_time": config.ConnMaxIdleTime,
	}).Info("Connected to database successfully with enhanced configuration")

	return nil
}

func Close() {
	if DB != nil {
		DB.Close()
		logrus.Info("Database connection closed")
	}
}

// ValidateAndOptimizeSchema performs comprehensive schema validation and optimization
func ValidateAndOptimizeSchema() error {
	if DB == nil {
		return fmt.Errorf("database connection not established")
	}

	logrus.Info("Starting database schema validation and optimization")

	// Create schema validator
	validator := NewSchemaValidator(DB)

	// Validate schema compatibility
	report, err := validator.ValidateSchemaCompatibility()
	if err != nil {
		return fmt.Errorf("failed to validate schema compatibility: %w", err)
	}

	// Log validation results
	if !report.OverallValid {
		logrus.WithFields(logrus.Fields{
			"total_issues":    report.TotalIssues,
			"critical_issues": report.CriticalIssues,
		}).Warn("Schema validation found issues")

		// Generate and log detailed report
		detailedReport := validator.GenerateSchemaReport(report)
		logrus.Debug("Schema validation report:\n" + detailedReport)
	} else {
		logrus.Info("Schema validation passed successfully")
	}

	// Create missing indexes if any
	var missingIndexes []string
	for _, result := range report.ValidationResults {
		missingIndexes = append(missingIndexes, result.MissingIndexes...)
	}

	if len(missingIndexes) > 0 {
		logrus.WithField("missing_indexes_count", len(missingIndexes)).Info("Creating missing indexes")
		if err := validator.CreateMissingIndexes(missingIndexes); err != nil {
			return fmt.Errorf("failed to create missing indexes: %w", err)
		}
	}

	logrus.Info("Completed database schema validation and optimization")
	return nil
}

// ValidateMigrationState performs comprehensive migration validation
func ValidateMigrationState() error {
	if DB == nil {
		return fmt.Errorf("database connection not established")
	}

	logrus.Info("Starting database migration validation")

	// Create migration validator
	validator := NewMigrationValidator(DB)

	// Validate migration state
	result, err := validator.ValidateMigrationStateDetailed()
	if err != nil {
		return fmt.Errorf("failed to validate migration state: %w", err)
	}

	// Log validation results
	if !result.IsValid {
		logrus.WithFields(logrus.Fields{
			"missing_tables":      len(result.MissingTables),
			"missing_columns":     len(result.MissingColumns),
			"missing_indexes":     len(result.MissingIndexes),
			"missing_constraints": len(result.MissingConstraints),
			"performance_issues":  len(result.PerformanceIssues),
		}).Warn("Migration validation found issues")

		// Generate and log detailed report
		detailedReport := validator.GenerateMigrationReport(result)
		logrus.Debug("Migration validation report:\n" + detailedReport)
	} else {
		logrus.Info("Migration validation passed successfully")
	}

	// Apply optimizations
	if err := validator.ApplyOptimizations(result); err != nil {
		return fmt.Errorf("failed to apply database optimizations: %w", err)
	}

	logrus.Info("Completed database migration validation")
	return nil
}

// GetConnectionStats returns current database connection pool statistics
func GetConnectionStats() sql.DBStats {
	if DB == nil {
		return sql.DBStats{}
	}
	return DB.Stats()
}

// HealthCheck performs a comprehensive database health check
func HealthCheck() error {
	if DB == nil {
		return fmt.Errorf("database connection not established")
	}

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := DB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check connection pool health
	stats := DB.Stats()
	if stats.OpenConnections == 0 {
		return fmt.Errorf("no open database connections")
	}

	logrus.WithFields(logrus.Fields{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration,
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	}).Debug("Database connection pool health check")

	return nil
}

func Migrate(schemaPath string) error {
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// Parse SQL statements more intelligently
	statements := parseSQLStatements(string(content))

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)

		if stmt == "" {
			continue
		}

		_, err = DB.Exec(stmt)
		if err != nil {
			// Log the error but continue with other statements for migration scripts
			// that handle existing tables
			logrus.Warnf("Migration statement failed (continuing): %v", err)
		}
	}

	logrus.Info("Database migration completed successfully")
	return nil
}

// parseSQLStatements parses SQL content into individual statements
// This handles multi-line statements and comments properly
func parseSQLStatements(content string) []string {
	var statements []string
	var currentStatement strings.Builder

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comment-only lines
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}

		// Add line to current statement
		if currentStatement.Len() > 0 {
			currentStatement.WriteString(" ")
		}
		currentStatement.WriteString(line)

		// If line ends with semicolon, we have a complete statement
		if strings.HasSuffix(line, ";") {
			stmt := strings.TrimSuffix(currentStatement.String(), ";")
			stmt = strings.TrimSpace(stmt)
			if stmt != "" {
				statements = append(statements, stmt)
			}
			currentStatement.Reset()
		}
	}

	// Handle any remaining statement without semicolon
	if currentStatement.Len() > 0 {
		stmt := strings.TrimSpace(currentStatement.String())
		if stmt != "" {
			statements = append(statements, stmt)
		}
	}

	return statements
}

// ValidationResult represents the result of schema validation
type ValidationResult struct {
	TableName          string
	IsValid            bool
	MissingColumns     []string
	MissingIndexes     []string
	InvalidConstraints []string
	Recommendations    []string
}

// SchemaCompatibilityReport contains comprehensive schema validation results
type SchemaCompatibilityReport struct {
	ValidationResults []ValidationResult
	OverallValid      bool
	TotalIssues       int
	CriticalIssues    int
	Recommendations   []string
}

// SchemaValidator validates database schema compatibility with enhanced data structures
type SchemaValidator struct {
	db     *sql.DB
	logger *logrus.Logger
}

// NewSchemaValidator creates a new schema validator instance
func NewSchemaValidator(db *sql.DB) *SchemaValidator {
	return &SchemaValidator{
		db:     db,
		logger: logrus.New(),
	}
}

// ValidateSchemaCompatibility performs comprehensive schema validation for enhanced data structures
func (v *SchemaValidator) ValidateSchemaCompatibility() (*SchemaCompatibilityReport, error) {
	v.logger.Info("Starting comprehensive schema compatibility validation")

	report := &SchemaCompatibilityReport{
		ValidationResults: make([]ValidationResult, 0),
		OverallValid:      true,
	}

	// Validate core IPO table structure
	ipoTableResult, err := v.validateIPOTableStructure()
	if err != nil {
		return nil, fmt.Errorf("failed to validate IPO table structure: %w", err)
	}
	report.ValidationResults = append(report.ValidationResults, *ipoTableResult)

	// Validate GMP table structure
	gmpTableResult, err := v.validateGMPTableStructure()
	if err != nil {
		return nil, fmt.Errorf("failed to validate GMP table structure: %w", err)
	}
	report.ValidationResults = append(report.ValidationResults, *gmpTableResult)

	// Validate result cache table structure
	cacheTableResult, err := v.validateResultCacheTableStructure()
	if err != nil {
		return nil, fmt.Errorf("failed to validate result cache table structure: %w", err)
	}
	report.ValidationResults = append(report.ValidationResults, *cacheTableResult)

	// Validate audit log table structure
	auditTableResult, err := v.validateAuditLogTableStructure()
	if err != nil {
		return nil, fmt.Errorf("failed to validate audit log table structure: %w", err)
	}
	report.ValidationResults = append(report.ValidationResults, *auditTableResult)

	// Validate indexes for optimized queries
	indexResult, err := v.validateOptimizedIndexes()
	if err != nil {
		return nil, fmt.Errorf("failed to validate optimized indexes: %w", err)
	}
	report.ValidationResults = append(report.ValidationResults, *indexResult)

	// Calculate overall validation status
	for _, result := range report.ValidationResults {
		if !result.IsValid {
			report.OverallValid = false
			report.TotalIssues += len(result.MissingColumns) + len(result.MissingIndexes) + len(result.InvalidConstraints)

			// Critical issues are missing columns or invalid constraints
			report.CriticalIssues += len(result.MissingColumns) + len(result.InvalidConstraints)
		}
		report.Recommendations = append(report.Recommendations, result.Recommendations...)
	}

	v.logger.WithFields(logrus.Fields{
		"overall_valid":   report.OverallValid,
		"total_issues":    report.TotalIssues,
		"critical_issues": report.CriticalIssues,
	}).Info("Completed schema compatibility validation")

	return report, nil
}

// validateIPOTableStructure validates the main IPO table structure
func (v *SchemaValidator) validateIPOTableStructure() (*ValidationResult, error) {
	result := &ValidationResult{
		TableName:          "ipo_list",
		IsValid:            true,
		MissingColumns:     make([]string, 0),
		MissingIndexes:     make([]string, 0),
		InvalidConstraints: make([]string, 0),
		Recommendations:    make([]string, 0),
	}

	// Check if table exists
	exists, err := v.tableExists("ipo_list")
	if err != nil {
		return nil, fmt.Errorf("failed to check if ipo_list table exists: %w", err)
	}
	if !exists {
		result.IsValid = false
		result.MissingColumns = append(result.MissingColumns, "entire table missing")
		result.Recommendations = append(result.Recommendations, "Create ipo_list table with complete schema")
		return result, nil
	}

	// Required columns for enhanced data structures
	requiredColumns := map[string]string{
		"id":                  "uuid",
		"stock_id":            "varchar(100)",
		"name":                "varchar(255)",
		"company_code":        "varchar(50)",
		"symbol":              "varchar(50)",
		"registrar":           "varchar(255)",
		"open_date":           "timestamp",
		"close_date":          "timestamp",
		"result_date":         "timestamp",
		"listing_date":        "timestamp",
		"price_band_low":      "decimal(10,2)",
		"price_band_high":     "decimal(10,2)",
		"issue_size":          "varchar(100)",
		"min_qty":             "integer",
		"min_amount":          "integer",
		"status":              "varchar(50)",
		"subscription_status": "varchar(100)",
		"listing_gain":        "varchar(50)",
		"logo_url":            "varchar(500)",
		"description":         "text",
		"about":               "text",
		"slug":                "varchar(255)",
		"form_url":            "varchar(500)",
		"form_fields":         "jsonb",
		"form_headers":        "jsonb",
		"parser_config":       "jsonb",
		"strengths":           "jsonb",
		"risks":               "jsonb",
		"created_at":          "timestamp",
		"updated_at":          "timestamp",
		"created_by":          "varchar(100)",
	}

	// Check for missing columns
	existingColumns, err := v.getTableColumns("ipo_list")
	if err != nil {
		return nil, fmt.Errorf("failed to get ipo_list columns: %w", err)
	}

	for columnName, expectedType := range requiredColumns {
		if actualType, exists := existingColumns[columnName]; !exists {
			result.IsValid = false
			result.MissingColumns = append(result.MissingColumns, fmt.Sprintf("%s (%s)", columnName, expectedType))
		} else if !v.isCompatibleType(actualType, expectedType) {
			result.InvalidConstraints = append(result.InvalidConstraints,
				fmt.Sprintf("column %s has type %s, expected %s", columnName, actualType, expectedType))
		}
	}

	// Check for required constraints
	constraints, err := v.getTableConstraints("ipo_list")
	if err != nil {
		return nil, fmt.Errorf("failed to get ipo_list constraints: %w", err)
	}

	requiredConstraints := []string{
		"ipo_list_name_not_empty",
		"ipo_list_company_code_not_empty",
		"ipo_list_registrar_not_empty",
		"ipo_list_status_not_empty",
		"ipo_list_date_logic",
		"ipo_list_price_band_logic",
		"ipo_list_min_qty_positive",
		"ipo_list_min_amount_positive",
	}

	for _, constraintName := range requiredConstraints {
		if !v.constraintExists(constraints, constraintName) {
			result.InvalidConstraints = append(result.InvalidConstraints, constraintName)
			result.IsValid = false
		}
	}

	// Add recommendations for enhanced data processing
	if len(result.MissingColumns) > 0 || len(result.InvalidConstraints) > 0 {
		result.Recommendations = append(result.Recommendations,
			"Update ipo_list table schema to support enhanced scraper data structures")
	}

	return result, nil
}

// validateGMPTableStructure validates the GMP table structure
func (v *SchemaValidator) validateGMPTableStructure() (*ValidationResult, error) {
	result := &ValidationResult{
		TableName:          "ipo_gmp",
		IsValid:            true,
		MissingColumns:     make([]string, 0),
		MissingIndexes:     make([]string, 0),
		InvalidConstraints: make([]string, 0),
		Recommendations:    make([]string, 0),
	}

	// Check if table exists
	exists, err := v.tableExists("ipo_gmp")
	if err != nil {
		return nil, fmt.Errorf("failed to check if ipo_gmp table exists: %w", err)
	}
	if !exists {
		result.IsValid = false
		result.MissingColumns = append(result.MissingColumns, "entire table missing")
		result.Recommendations = append(result.Recommendations, "Create ipo_gmp table with complete schema")
		return result, nil
	}

	// Required columns for GMP data
	requiredColumns := map[string]string{
		"id":                "varchar(100)",
		"ipo_name":          "varchar(255)",
		"company_code":      "varchar(50)",
		"ipo_price":         "decimal(10,2)",
		"gmp_value":         "decimal(10,2)",
		"estimated_listing": "decimal(10,2)",
		"gain_percent":      "decimal(10,2)",
		"sub2":              "decimal(10,2)",
		"kostak":            "decimal(10,2)",
		"listing_date":      "timestamp",
		"last_updated":      "timestamp",
		// Enhanced GMP columns
		"stock_id":            "varchar(100)",
		"subscription_status": "varchar(100)",
		"listing_gain":        "varchar(50)",
		"ipo_status":          "varchar(50)",
		"data_source":         "varchar(100)",
		"extraction_metadata": "jsonb",
	}

	// Check for missing columns
	existingColumns, err := v.getTableColumns("ipo_gmp")
	if err != nil {
		return nil, fmt.Errorf("failed to get ipo_gmp columns: %w", err)
	}

	for columnName, expectedType := range requiredColumns {
		if actualType, exists := existingColumns[columnName]; !exists {
			result.IsValid = false
			result.MissingColumns = append(result.MissingColumns, fmt.Sprintf("%s (%s)", columnName, expectedType))
		} else if !v.isCompatibleType(actualType, expectedType) {
			result.InvalidConstraints = append(result.InvalidConstraints,
				fmt.Sprintf("column %s has type %s, expected %s", columnName, actualType, expectedType))
		}
	}

	return result, nil
}

// validateResultCacheTableStructure validates the result cache table structure
func (v *SchemaValidator) validateResultCacheTableStructure() (*ValidationResult, error) {
	result := &ValidationResult{
		TableName:          "ipo_result_cache",
		IsValid:            true,
		MissingColumns:     make([]string, 0),
		MissingIndexes:     make([]string, 0),
		InvalidConstraints: make([]string, 0),
		Recommendations:    make([]string, 0),
	}

	// Check if table exists
	exists, err := v.tableExists("ipo_result_cache")
	if err != nil {
		return nil, fmt.Errorf("failed to check if ipo_result_cache table exists: %w", err)
	}
	if !exists {
		result.IsValid = false
		result.MissingColumns = append(result.MissingColumns, "entire table missing")
		result.Recommendations = append(result.Recommendations, "Create ipo_result_cache table for enhanced caching")
		return result, nil
	}

	// Required columns for result caching
	requiredColumns := map[string]string{
		"id":                 "uuid",
		"pan_hash":           "varchar(255)",
		"ipo_id":             "uuid",
		"status":             "varchar(100)",
		"shares_allotted":    "integer",
		"application_number": "varchar(100)",
		"refund_status":      "varchar(100)",
		"source":             "varchar(100)",
		"user_agent":         "text",
		"timestamp":          "timestamp",
		"expires_at":         "timestamp",
		"confidence_score":   "integer",
		"duplicate_count":    "integer",
	}

	// Check for missing columns
	existingColumns, err := v.getTableColumns("ipo_result_cache")
	if err != nil {
		return nil, fmt.Errorf("failed to get ipo_result_cache columns: %w", err)
	}

	for columnName, expectedType := range requiredColumns {
		if actualType, exists := existingColumns[columnName]; !exists {
			result.IsValid = false
			result.MissingColumns = append(result.MissingColumns, fmt.Sprintf("%s (%s)", columnName, expectedType))
		} else if !v.isCompatibleType(actualType, expectedType) {
			result.InvalidConstraints = append(result.InvalidConstraints,
				fmt.Sprintf("column %s has type %s, expected %s", columnName, actualType, expectedType))
		}
	}

	return result, nil
}

// validateAuditLogTableStructure validates the audit log table structure
func (v *SchemaValidator) validateAuditLogTableStructure() (*ValidationResult, error) {
	result := &ValidationResult{
		TableName:          "ipo_update_log",
		IsValid:            true,
		MissingColumns:     make([]string, 0),
		MissingIndexes:     make([]string, 0),
		InvalidConstraints: make([]string, 0),
		Recommendations:    make([]string, 0),
	}

	// Check if table exists
	exists, err := v.tableExists("ipo_update_log")
	if err != nil {
		return nil, fmt.Errorf("failed to check if ipo_update_log table exists: %w", err)
	}
	if !exists {
		result.IsValid = false
		result.MissingColumns = append(result.MissingColumns, "entire table missing")
		result.Recommendations = append(result.Recommendations, "Create ipo_update_log table for audit trail")
		return result, nil
	}

	// Required columns for audit logging
	requiredColumns := map[string]string{
		"id":         "uuid",
		"ipo_id":     "uuid",
		"field_name": "varchar(100)",
		"old_value":  "text",
		"new_value":  "text",
		"source":     "varchar(100)",
		"timestamp":  "timestamp",
	}

	// Check for missing columns
	existingColumns, err := v.getTableColumns("ipo_update_log")
	if err != nil {
		return nil, fmt.Errorf("failed to get ipo_update_log columns: %w", err)
	}

	for columnName, expectedType := range requiredColumns {
		if actualType, exists := existingColumns[columnName]; !exists {
			result.IsValid = false
			result.MissingColumns = append(result.MissingColumns, fmt.Sprintf("%s (%s)", columnName, expectedType))
		} else if !v.isCompatibleType(actualType, expectedType) {
			result.InvalidConstraints = append(result.InvalidConstraints,
				fmt.Sprintf("column %s has type %s, expected %s", columnName, actualType, expectedType))
		}
	}

	return result, nil
}

// validateOptimizedIndexes validates that all required indexes exist for optimized queries
func (v *SchemaValidator) validateOptimizedIndexes() (*ValidationResult, error) {
	result := &ValidationResult{
		TableName:          "database_indexes",
		IsValid:            true,
		MissingColumns:     make([]string, 0),
		MissingIndexes:     make([]string, 0),
		InvalidConstraints: make([]string, 0),
		Recommendations:    make([]string, 0),
	}

	// Get existing indexes
	existingIndexes, err := v.getAllIndexes()
	if err != nil {
		return nil, fmt.Errorf("failed to get existing indexes: %w", err)
	}

	// Required indexes for optimized query performance
	requiredIndexes := map[string]string{
		// IPO table indexes
		"idx_ipo_stock_id":            "ipo_list(stock_id)",
		"idx_ipo_company_code":        "ipo_list(company_code)",
		"idx_ipo_symbol":              "ipo_list(symbol) WHERE symbol IS NOT NULL",
		"idx_ipo_status":              "ipo_list(status)",
		"idx_ipo_status_dates":        "ipo_list(status, open_date, close_date)",
		"idx_ipo_open_date":           "ipo_list(open_date) WHERE open_date IS NOT NULL",
		"idx_ipo_close_date":          "ipo_list(close_date) WHERE close_date IS NOT NULL",
		"idx_ipo_result_date":         "ipo_list(result_date) WHERE result_date IS NOT NULL",
		"idx_ipo_listing_date":        "ipo_list(listing_date) WHERE listing_date IS NOT NULL",
		"idx_ipo_registrar":           "ipo_list(registrar)",
		"idx_ipo_list_api":            "ipo_list(status, created_at DESC)",
		"idx_ipo_date_range":          "ipo_list(open_date, close_date) WHERE open_date IS NOT NULL AND close_date IS NOT NULL",
		"idx_ipo_price_band":          "ipo_list(price_band_low, price_band_high) WHERE price_band_low IS NOT NULL AND price_band_high IS NOT NULL",
		"idx_ipo_subscription_status": "ipo_list(subscription_status) WHERE subscription_status IS NOT NULL",
		"idx_ipo_name_gin":            "ipo_list USING gin(to_tsvector('english', name))",
		"idx_ipo_recent":              "ipo_list(created_at DESC, status) WHERE created_at >= CURRENT_DATE - INTERVAL '1 year'",

		// GMP table indexes
		"idx_ipo_gmp_company_code": "ipo_gmp(company_code)",
		"idx_ipo_gmp_ipo_name":     "ipo_gmp(ipo_name)",
		"idx_ipo_gmp_last_updated": "ipo_gmp(last_updated DESC)",
		"idx_ipo_gmp_listing_date": "ipo_gmp(listing_date) WHERE listing_date IS NOT NULL",
		// Enhanced GMP indexes
		"idx_ipo_gmp_stock_id":            "ipo_gmp(stock_id) WHERE stock_id IS NOT NULL",
		"idx_ipo_gmp_subscription_status": "ipo_gmp(subscription_status) WHERE subscription_status IS NOT NULL",
		"idx_ipo_gmp_listing_gain":        "ipo_gmp(listing_gain) WHERE listing_gain IS NOT NULL",
		"idx_ipo_gmp_ipo_status":          "ipo_gmp(ipo_status) WHERE ipo_status IS NOT NULL",

		// Result cache table indexes
		"idx_ipo_result_cache_pan_hash":     "ipo_result_cache(pan_hash)",
		"idx_ipo_result_cache_ipo_id":       "ipo_result_cache(ipo_id)",
		"idx_ipo_result_cache_expires_at":   "ipo_result_cache(expires_at)",
		"idx_ipo_result_cache_timestamp":    "ipo_result_cache(timestamp DESC)",
		"idx_ipo_result_cache_unique_check": "ipo_result_cache(pan_hash, ipo_id, application_number) WHERE application_number IS NOT NULL",
		"idx_ipo_result_cache_pan_ipo":      "ipo_result_cache(pan_hash, ipo_id)",

		// Update log table indexes
		"idx_ipo_update_log_ipo_id":     "ipo_update_log(ipo_id)",
		"idx_ipo_update_log_timestamp":  "ipo_update_log(timestamp DESC)",
		"idx_ipo_update_log_field_name": "ipo_update_log(field_name)",
		"idx_ipo_update_log_source":     "ipo_update_log(source) WHERE source IS NOT NULL",
	}

	// Check for missing indexes
	for indexName, indexDefinition := range requiredIndexes {
		if !v.indexExists(existingIndexes, indexName) {
			result.IsValid = false
			result.MissingIndexes = append(result.MissingIndexes, fmt.Sprintf("%s ON %s", indexName, indexDefinition))
		}
	}

	// Add recommendations for missing indexes
	if len(result.MissingIndexes) > 0 {
		result.Recommendations = append(result.Recommendations,
			"Create missing indexes to optimize query performance for enhanced scraper operations")
	}

	return result, nil
}

// Helper methods for schema validation

// tableExists checks if a table exists in the database (schema validator version)
func (v *SchemaValidator) tableExists(tableName string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = $1
		)
	`
	var exists bool
	err := v.db.QueryRow(query, tableName).Scan(&exists)
	return exists, err
}

// getTableColumns returns a map of column names to their data types (schema validator version)
func (v *SchemaValidator) getTableColumns(tableName string) (map[string]string, error) {
	query := `
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_schema = 'public' AND table_name = $1
	`
	rows, err := v.db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]string)
	for rows.Next() {
		var columnName, dataType string
		if err := rows.Scan(&columnName, &dataType); err != nil {
			return nil, err
		}
		columns[columnName] = dataType
	}

	return columns, rows.Err()
}

// getTableConstraints returns a list of constraint names for a table (schema validator version)
func (v *SchemaValidator) getTableConstraints(tableName string) ([]string, error) {
	query := `
		SELECT constraint_name 
		FROM information_schema.table_constraints 
		WHERE table_schema = 'public' AND table_name = $1
	`
	rows, err := v.db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints []string
	for rows.Next() {
		var constraintName string
		if err := rows.Scan(&constraintName); err != nil {
			return nil, err
		}
		constraints = append(constraints, constraintName)
	}

	return constraints, rows.Err()
}

// getAllIndexes returns a list of all index names in the database (schema validator version)
func (v *SchemaValidator) getAllIndexes() ([]string, error) {
	query := `
		SELECT indexname 
		FROM pg_indexes 
		WHERE schemaname = 'public'
	`
	rows, err := v.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []string
	for rows.Next() {
		var indexName string
		if err := rows.Scan(&indexName); err != nil {
			return nil, err
		}
		indexes = append(indexes, indexName)
	}

	return indexes, rows.Err()
}

// isCompatibleType checks if the actual column type is compatible with the expected type
func (v *SchemaValidator) isCompatibleType(actualType, expectedType string) bool {
	// Normalize types for comparison
	actualType = strings.ToLower(strings.TrimSpace(actualType))
	expectedType = strings.ToLower(strings.TrimSpace(expectedType))

	// Handle common PostgreSQL type variations
	typeMapping := map[string][]string{
		"uuid":          {"uuid"},
		"varchar(100)":  {"character varying", "varchar", "text"},
		"varchar(255)":  {"character varying", "varchar", "text"},
		"varchar(50)":   {"character varying", "varchar", "text"},
		"varchar(500)":  {"character varying", "varchar", "text"},
		"text":          {"text", "character varying", "varchar"},
		"timestamp":     {"timestamp without time zone", "timestamp", "timestamptz"},
		"decimal(10,2)": {"numeric", "decimal", "real", "double precision"},
		"integer":       {"integer", "int", "int4"},
		"jsonb":         {"jsonb", "json"},
	}

	if compatibleTypes, exists := typeMapping[expectedType]; exists {
		for _, compatibleType := range compatibleTypes {
			if strings.Contains(actualType, compatibleType) {
				return true
			}
		}
	}

	// Direct match
	return strings.Contains(actualType, expectedType) || strings.Contains(expectedType, actualType)
}

// constraintExists checks if a constraint exists in the list of constraints (schema validator version)
func (v *SchemaValidator) constraintExists(constraints []string, constraintName string) bool {
	for _, constraint := range constraints {
		if constraint == constraintName {
			return true
		}
	}
	return false
}

// indexExists checks if an index exists in the list of indexes (schema validator version)
func (v *SchemaValidator) indexExists(indexes []string, indexName string) bool {
	for _, index := range indexes {
		if index == indexName {
			return true
		}
	}
	return false
}

// CreateMissingIndexes creates any missing indexes identified during validation
func (v *SchemaValidator) CreateMissingIndexes(missingIndexes []string) error {
	v.logger.WithField("missing_indexes_count", len(missingIndexes)).Info("Creating missing database indexes")

	// Index creation statements
	indexStatements := map[string]string{
		"idx_ipo_stock_id":            "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_stock_id ON ipo_list(stock_id)",
		"idx_ipo_company_code":        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_company_code ON ipo_list(company_code)",
		"idx_ipo_symbol":              "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_symbol ON ipo_list(symbol) WHERE symbol IS NOT NULL",
		"idx_ipo_status":              "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_status ON ipo_list(status)",
		"idx_ipo_status_dates":        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_status_dates ON ipo_list(status, open_date, close_date)",
		"idx_ipo_open_date":           "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_open_date ON ipo_list(open_date) WHERE open_date IS NOT NULL",
		"idx_ipo_close_date":          "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_close_date ON ipo_list(close_date) WHERE close_date IS NOT NULL",
		"idx_ipo_result_date":         "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_result_date ON ipo_list(result_date) WHERE result_date IS NOT NULL",
		"idx_ipo_listing_date":        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_listing_date ON ipo_list(listing_date) WHERE listing_date IS NOT NULL",
		"idx_ipo_registrar":           "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_registrar ON ipo_list(registrar)",
		"idx_ipo_list_api":            "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_list_api ON ipo_list(status, created_at DESC)",
		"idx_ipo_date_range":          "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_date_range ON ipo_list(open_date, close_date) WHERE open_date IS NOT NULL AND close_date IS NOT NULL",
		"idx_ipo_price_band":          "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_price_band ON ipo_list(price_band_low, price_band_high) WHERE price_band_low IS NOT NULL AND price_band_high IS NOT NULL",
		"idx_ipo_subscription_status": "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_subscription_status ON ipo_list(subscription_status) WHERE subscription_status IS NOT NULL",
		"idx_ipo_name_gin":            "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_name_gin ON ipo_list USING gin(to_tsvector('english', name))",
		"idx_ipo_recent":              "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_recent ON ipo_list(created_at DESC, status) WHERE created_at >= CURRENT_DATE - INTERVAL '1 year'",
		"idx_ipo_gmp_company_code":    "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_gmp_company_code ON ipo_gmp(company_code)",
		"idx_ipo_gmp_ipo_name":        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_gmp_ipo_name ON ipo_gmp(ipo_name)",
		"idx_ipo_gmp_last_updated":    "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_gmp_last_updated ON ipo_gmp(last_updated DESC)",
		"idx_ipo_gmp_listing_date":    "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_gmp_listing_date ON ipo_gmp(listing_date) WHERE listing_date IS NOT NULL",
		// Enhanced GMP indexes
		"idx_ipo_gmp_stock_id":              "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_gmp_stock_id ON ipo_gmp(stock_id) WHERE stock_id IS NOT NULL",
		"idx_ipo_gmp_subscription_status":   "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_gmp_subscription_status ON ipo_gmp(subscription_status) WHERE subscription_status IS NOT NULL",
		"idx_ipo_gmp_listing_gain":          "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_gmp_listing_gain ON ipo_gmp(listing_gain) WHERE listing_gain IS NOT NULL",
		"idx_ipo_gmp_ipo_status":            "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_gmp_ipo_status ON ipo_gmp(ipo_status) WHERE ipo_status IS NOT NULL",
		"idx_ipo_result_cache_pan_hash":     "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_result_cache_pan_hash ON ipo_result_cache(pan_hash)",
		"idx_ipo_result_cache_ipo_id":       "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_result_cache_ipo_id ON ipo_result_cache(ipo_id)",
		"idx_ipo_result_cache_expires_at":   "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_result_cache_expires_at ON ipo_result_cache(expires_at)",
		"idx_ipo_result_cache_timestamp":    "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_result_cache_timestamp ON ipo_result_cache(timestamp DESC)",
		"idx_ipo_result_cache_unique_check": "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_result_cache_unique_check ON ipo_result_cache(pan_hash, ipo_id, application_number) WHERE application_number IS NOT NULL",
		"idx_ipo_result_cache_pan_ipo":      "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_result_cache_pan_ipo ON ipo_result_cache(pan_hash, ipo_id)",
		"idx_ipo_update_log_ipo_id":         "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_update_log_ipo_id ON ipo_update_log(ipo_id)",
		"idx_ipo_update_log_timestamp":      "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_update_log_timestamp ON ipo_update_log(timestamp DESC)",
		"idx_ipo_update_log_field_name":     "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_update_log_field_name ON ipo_update_log(field_name)",
		"idx_ipo_update_log_source":         "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_update_log_source ON ipo_update_log(source) WHERE source IS NOT NULL",
	}

	for _, missingIndex := range missingIndexes {
		// Extract index name from the missing index description
		indexName := strings.Split(missingIndex, " ON ")[0]

		if statement, exists := indexStatements[indexName]; exists {
			v.logger.WithField("index_name", indexName).Info("Creating missing index")

			// Use a timeout for index creation to prevent hanging
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			_, err := v.db.ExecContext(ctx, statement)
			if err != nil {
				v.logger.WithFields(logrus.Fields{
					"index_name": indexName,
					"error":      err,
				}).Error("Failed to create index")
				return fmt.Errorf("failed to create index %s: %w", indexName, err)
			}

			v.logger.WithField("index_name", indexName).Info("Successfully created index")
		} else {
			v.logger.WithField("index_name", indexName).Warn("No creation statement found for missing index")
		}
	}

	v.logger.Info("Completed creating missing database indexes")
	return nil
}

// GenerateSchemaReport generates a comprehensive report of the schema validation results
func (v *SchemaValidator) GenerateSchemaReport(report *SchemaCompatibilityReport) string {
	var reportBuilder strings.Builder

	reportBuilder.WriteString("=== Database Schema Compatibility Report ===\n\n")
	reportBuilder.WriteString(fmt.Sprintf("Overall Status: %s\n", map[bool]string{true: "VALID", false: "INVALID"}[report.OverallValid]))
	reportBuilder.WriteString(fmt.Sprintf("Total Issues: %d\n", report.TotalIssues))
	reportBuilder.WriteString(fmt.Sprintf("Critical Issues: %d\n\n", report.CriticalIssues))

	for _, result := range report.ValidationResults {
		reportBuilder.WriteString(fmt.Sprintf("Table: %s - Status: %s\n", result.TableName, map[bool]string{true: "VALID", false: "INVALID"}[result.IsValid]))

		if len(result.MissingColumns) > 0 {
			reportBuilder.WriteString("  Missing Columns:\n")
			for _, column := range result.MissingColumns {
				reportBuilder.WriteString(fmt.Sprintf("    - %s\n", column))
			}
		}

		if len(result.MissingIndexes) > 0 {
			reportBuilder.WriteString("  Missing Indexes:\n")
			for _, index := range result.MissingIndexes {
				reportBuilder.WriteString(fmt.Sprintf("    - %s\n", index))
			}
		}

		if len(result.InvalidConstraints) > 0 {
			reportBuilder.WriteString("  Invalid Constraints:\n")
			for _, constraint := range result.InvalidConstraints {
				reportBuilder.WriteString(fmt.Sprintf("    - %s\n", constraint))
			}
		}

		reportBuilder.WriteString("\n")
	}

	if len(report.Recommendations) > 0 {
		reportBuilder.WriteString("Recommendations:\n")
		for _, recommendation := range report.Recommendations {
			reportBuilder.WriteString(fmt.Sprintf("  - %s\n", recommendation))
		}
	}

	return reportBuilder.String()
}

// MigrationValidationResult represents the result of migration validation
type MigrationValidationResult struct {
	IsValid            bool
	MissingTables      []string
	MissingColumns     []string
	MissingIndexes     []string
	MissingConstraints []string
	PerformanceIssues  []string
	Recommendations    []string
	ValidationErrors   []error
}

// MigrationValidator handles database migration validation and optimization
type MigrationValidator struct {
	db     *sql.DB
	logger *logrus.Logger
}

// NewMigrationValidator creates a new migration validator instance
func NewMigrationValidator(db *sql.DB) *MigrationValidator {
	return &MigrationValidator{
		db:     db,
		logger: logrus.New(),
	}
}

// ValidateMigrationStateDetailed performs comprehensive validation of the database migration state
func (mv *MigrationValidator) ValidateMigrationStateDetailed() (*MigrationValidationResult, error) {
	mv.logger.Info("Starting comprehensive migration state validation")

	result := &MigrationValidationResult{
		IsValid:            true,
		MissingTables:      make([]string, 0),
		MissingColumns:     make([]string, 0),
		MissingIndexes:     make([]string, 0),
		MissingConstraints: make([]string, 0),
		PerformanceIssues:  make([]string, 0),
		Recommendations:    make([]string, 0),
		ValidationErrors:   make([]error, 0),
	}

	// Validate core table structure
	if err := mv.validateCoreTableStructure(result); err != nil {
		result.ValidationErrors = append(result.ValidationErrors, err)
		result.IsValid = false
	}

	// Validate indexes for performance
	if err := mv.validatePerformanceIndexes(result); err != nil {
		result.ValidationErrors = append(result.ValidationErrors, err)
		result.IsValid = false
	}

	// Validate constraints for data integrity
	if err := mv.validateDataIntegrityConstraints(result); err != nil {
		result.ValidationErrors = append(result.ValidationErrors, err)
		result.IsValid = false
	}

	// Validate connection pool configuration
	if err := mv.validateConnectionPoolConfiguration(result); err != nil {
		result.ValidationErrors = append(result.ValidationErrors, err)
	}

	// Generate recommendations based on findings
	mv.generateOptimizationRecommendations(result)

	mv.logger.WithFields(logrus.Fields{
		"is_valid":            result.IsValid,
		"missing_tables":      len(result.MissingTables),
		"missing_columns":     len(result.MissingColumns),
		"missing_indexes":     len(result.MissingIndexes),
		"missing_constraints": len(result.MissingConstraints),
		"performance_issues":  len(result.PerformanceIssues),
		"validation_errors":   len(result.ValidationErrors),
	}).Info("Completed migration state validation")

	return result, nil
}

// validateCoreTableStructure validates that all required tables exist with proper structure
func (mv *MigrationValidator) validateCoreTableStructure(result *MigrationValidationResult) error {
	// Required tables for enhanced service alignment
	requiredTables := map[string][]string{
		"ipo_list": {
			"id", "stock_id", "name", "company_code", "symbol", "registrar",
			"open_date", "close_date", "result_date", "listing_date",
			"price_band_low", "price_band_high", "issue_size", "min_qty", "min_amount",
			"status", "subscription_status", "listing_gain",
			"logo_url", "description", "about", "slug",
			"form_url", "form_fields", "form_headers", "parser_config",
			"strengths", "risks", "created_at", "updated_at", "created_by",
		},
		"ipo_gmp": {
			"id", "ipo_name", "company_code", "ipo_price", "gmp_value",
			"estimated_listing", "gain_percent", "sub2", "kostak",
			"listing_date", "last_updated",
		},
		"ipo_result_cache": {
			"id", "pan_hash", "ipo_id", "status", "shares_allotted",
			"application_number", "refund_status", "source", "user_agent",
			"timestamp", "expires_at", "confidence_score", "duplicate_count",
		},
		"ipo_update_log": {
			"id", "ipo_id", "field_name", "old_value", "new_value",
			"source", "timestamp",
		},
	}

	for tableName, requiredColumns := range requiredTables {
		// Check if table exists
		exists, err := mv.tableExists(tableName)
		if err != nil {
			return fmt.Errorf("failed to check if table %s exists: %w", tableName, err)
		}

		if !exists {
			result.MissingTables = append(result.MissingTables, tableName)
			result.IsValid = false
			continue
		}

		// Check if all required columns exist
		existingColumns, err := mv.getTableColumns(tableName)
		if err != nil {
			return fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
		}

		for _, columnName := range requiredColumns {
			if _, exists := existingColumns[columnName]; !exists {
				result.MissingColumns = append(result.MissingColumns, fmt.Sprintf("%s.%s", tableName, columnName))
				result.IsValid = false
			}
		}
	}

	return nil
}

// validatePerformanceIndexes validates that all performance-critical indexes exist
func (mv *MigrationValidator) validatePerformanceIndexes(result *MigrationValidationResult) error {
	// Get existing indexes
	existingIndexes, err := mv.getAllIndexes()
	if err != nil {
		return fmt.Errorf("failed to get existing indexes: %w", err)
	}

	// Critical indexes for enhanced service alignment performance
	criticalIndexes := []string{
		"idx_ipo_stock_id",
		"idx_ipo_company_code",
		"idx_ipo_status",
		"idx_ipo_status_dates",
		"idx_ipo_open_date",
		"idx_ipo_close_date",
		"idx_ipo_registrar",
		"idx_ipo_list_api",
		"idx_ipo_gmp_company_code",
		"idx_ipo_gmp_ipo_name",
		"idx_ipo_result_cache_pan_hash",
		"idx_ipo_result_cache_ipo_id",
		"idx_ipo_update_log_ipo_id",
		"idx_ipo_update_log_timestamp",
	}

	// Check for missing critical indexes
	for _, indexName := range criticalIndexes {
		if !mv.indexExists(existingIndexes, indexName) {
			result.MissingIndexes = append(result.MissingIndexes, indexName)
			result.IsValid = false
		}
	}

	// Check for performance issues with existing indexes
	if err := mv.analyzeIndexPerformance(result); err != nil {
		return fmt.Errorf("failed to analyze index performance: %w", err)
	}

	return nil
}

// validateDataIntegrityConstraints validates that all data integrity constraints exist
func (mv *MigrationValidator) validateDataIntegrityConstraints(result *MigrationValidationResult) error {
	// Required constraints for data integrity
	requiredConstraints := map[string][]string{
		"ipo_list": {
			"ipo_list_name_not_empty",
			"ipo_list_company_code_not_empty",
			"ipo_list_registrar_not_empty",
			"ipo_list_status_not_empty",
			"ipo_list_date_logic",
			"ipo_list_price_band_logic",
			"ipo_list_min_qty_positive",
			"ipo_list_min_amount_positive",
		},
		"ipo_gmp": {
			"ipo_gmp_ipo_name_not_empty",
			"ipo_gmp_company_code_not_empty",
			"ipo_gmp_ipo_price_positive",
		},
		"ipo_result_cache": {
			"ipo_result_cache_pan_hash_not_empty",
			"ipo_result_cache_status_not_empty",
			"ipo_result_cache_shares_allotted_non_negative",
			"ipo_result_cache_confidence_score_range",
			"ipo_result_cache_duplicate_count_non_negative",
			"ipo_result_cache_expires_after_timestamp",
		},
		"ipo_update_log": {
			"ipo_update_log_field_name_not_empty",
		},
	}

	for tableName, constraints := range requiredConstraints {
		existingConstraints, err := mv.getTableConstraints(tableName)
		if err != nil {
			return fmt.Errorf("failed to get constraints for table %s: %w", tableName, err)
		}

		for _, constraintName := range constraints {
			if !mv.constraintExists(existingConstraints, constraintName) {
				result.MissingConstraints = append(result.MissingConstraints, fmt.Sprintf("%s.%s", tableName, constraintName))
				result.IsValid = false
			}
		}
	}

	return nil
}

// validateConnectionPoolConfiguration validates database connection pool settings
func (mv *MigrationValidator) validateConnectionPoolConfiguration(result *MigrationValidationResult) error {
	// Get current connection pool statistics
	stats := mv.db.Stats()

	// Recommended configuration for enhanced service alignment
	recommendedConfig := shared.DatabaseConfig{
		MaxOpenConns:    25,              // Suitable for moderate concurrent load
		MaxIdleConns:    10,              // Keep some connections idle for quick reuse
		ConnMaxLifetime: 5 * time.Minute, // Prevent stale connections
		ConnMaxIdleTime: 2 * time.Minute, // Close idle connections after 2 minutes
	}

	// Check current configuration against recommendations
	if stats.MaxOpenConnections == 0 {
		result.PerformanceIssues = append(result.PerformanceIssues, "Unlimited max open connections may cause resource exhaustion")
		result.Recommendations = append(result.Recommendations, fmt.Sprintf("Set MaxOpenConns to %d", recommendedConfig.MaxOpenConns))
	} else if stats.MaxOpenConnections > 50 {
		result.PerformanceIssues = append(result.PerformanceIssues, "Very high max open connections may cause resource contention")
		result.Recommendations = append(result.Recommendations, fmt.Sprintf("Consider reducing MaxOpenConns to %d", recommendedConfig.MaxOpenConns))
	}

	// Check for connection pool health
	if stats.OpenConnections > 0 {
		utilizationRate := float64(stats.InUse) / float64(stats.OpenConnections)
		if utilizationRate > 0.8 {
			result.PerformanceIssues = append(result.PerformanceIssues, "High connection pool utilization detected")
			result.Recommendations = append(result.Recommendations, "Consider increasing MaxOpenConns or optimizing query performance")
		}
	}

	mv.logger.WithFields(logrus.Fields{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
	}).Debug("Current connection pool statistics")

	return nil
}

// analyzeIndexPerformance analyzes the performance characteristics of existing indexes
func (mv *MigrationValidator) analyzeIndexPerformance(result *MigrationValidationResult) error {
	// Query to get index usage statistics
	query := `
		SELECT 
			schemaname,
			tablename,
			indexname,
			idx_scan,
			idx_tup_read,
			idx_tup_fetch
		FROM pg_stat_user_indexes 
		WHERE schemaname = 'public'
		ORDER BY idx_scan DESC
	`

	rows, err := mv.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query index statistics: %w", err)
	}
	defer rows.Close()

	unusedIndexes := make([]string, 0)
	lowUsageIndexes := make([]string, 0)

	for rows.Next() {
		var schemaName, tableName, indexName string
		var idxScan, idxTupRead, idxTupFetch int64

		if err := rows.Scan(&schemaName, &tableName, &indexName, &idxScan, &idxTupRead, &idxTupFetch); err != nil {
			return fmt.Errorf("failed to scan index statistics: %w", err)
		}

		// Skip primary key and unique constraints
		if strings.HasSuffix(indexName, "_pkey") || strings.Contains(indexName, "_key") {
			continue
		}

		// Identify unused indexes
		if idxScan == 0 {
			unusedIndexes = append(unusedIndexes, fmt.Sprintf("%s.%s", tableName, indexName))
		} else if idxScan < 10 {
			// Indexes with very low usage
			lowUsageIndexes = append(lowUsageIndexes, fmt.Sprintf("%s.%s (scans: %d)", tableName, indexName, idxScan))
		}
	}

	if len(unusedIndexes) > 0 {
		result.PerformanceIssues = append(result.PerformanceIssues, fmt.Sprintf("Found %d unused indexes", len(unusedIndexes)))
		result.Recommendations = append(result.Recommendations, "Consider dropping unused indexes to improve write performance")
	}

	if len(lowUsageIndexes) > 0 {
		result.PerformanceIssues = append(result.PerformanceIssues, fmt.Sprintf("Found %d low-usage indexes", len(lowUsageIndexes)))
		result.Recommendations = append(result.Recommendations, "Review low-usage indexes for potential optimization")
	}

	return rows.Err()
}

// generateOptimizationRecommendations generates specific recommendations based on validation results
func (mv *MigrationValidator) generateOptimizationRecommendations(result *MigrationValidationResult) {
	// Add general recommendations based on findings
	if len(result.MissingTables) > 0 {
		result.Recommendations = append(result.Recommendations, "Run database migration to create missing tables")
	}

	if len(result.MissingColumns) > 0 {
		result.Recommendations = append(result.Recommendations, "Update table schemas to add missing columns for enhanced service alignment")
	}

	if len(result.MissingIndexes) > 0 {
		result.Recommendations = append(result.Recommendations, "Create missing indexes to optimize query performance")
	}

	if len(result.MissingConstraints) > 0 {
		result.Recommendations = append(result.Recommendations, "Add missing constraints to ensure data integrity")
	}

	// Add performance-specific recommendations
	result.Recommendations = append(result.Recommendations, "Configure connection pooling with MaxOpenConns=25, MaxIdleConns=10")
	result.Recommendations = append(result.Recommendations, "Set connection lifetime limits to prevent stale connections")
	result.Recommendations = append(result.Recommendations, "Monitor query performance and optimize slow queries")
	result.Recommendations = append(result.Recommendations, "Implement batch processing for bulk operations")
	result.Recommendations = append(result.Recommendations, "Use prepared statements for frequently executed queries")
}

// ApplyOptimizations applies database optimizations based on validation results
func (mv *MigrationValidator) ApplyOptimizations(result *MigrationValidationResult) error {
	mv.logger.Info("Applying database optimizations based on validation results")

	// Apply connection pool optimizations
	if err := mv.applyConnectionPoolOptimizations(); err != nil {
		return fmt.Errorf("failed to apply connection pool optimizations: %w", err)
	}

	// Create missing indexes (if any)
	if len(result.MissingIndexes) > 0 {
		if err := mv.createMissingIndexes(result.MissingIndexes); err != nil {
			return fmt.Errorf("failed to create missing indexes: %w", err)
		}
	}

	// Update database statistics for better query planning
	if err := mv.updateDatabaseStatistics(); err != nil {
		return fmt.Errorf("failed to update database statistics: %w", err)
	}

	mv.logger.Info("Successfully applied database optimizations")
	return nil
}

// applyConnectionPoolOptimizations configures optimal connection pool settings
func (mv *MigrationValidator) applyConnectionPoolOptimizations() error {
	// Set optimal connection pool configuration
	mv.db.SetMaxOpenConns(25)
	mv.db.SetMaxIdleConns(10)
	mv.db.SetConnMaxLifetime(5 * time.Minute)
	mv.db.SetConnMaxIdleTime(2 * time.Minute)

	mv.logger.WithFields(logrus.Fields{
		"max_open_conns":     25,
		"max_idle_conns":     10,
		"conn_max_lifetime":  "5m",
		"conn_max_idle_time": "2m",
	}).Info("Applied connection pool optimizations")

	return nil
}

// createMissingIndexes creates any missing indexes identified during validation
func (mv *MigrationValidator) createMissingIndexes(missingIndexes []string) error {
	// Index creation statements
	indexStatements := map[string]string{
		"idx_ipo_stock_id":              "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_stock_id ON ipo_list(stock_id)",
		"idx_ipo_company_code":          "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_company_code ON ipo_list(company_code)",
		"idx_ipo_status":                "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_status ON ipo_list(status)",
		"idx_ipo_status_dates":          "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_status_dates ON ipo_list(status, open_date, close_date)",
		"idx_ipo_open_date":             "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_open_date ON ipo_list(open_date) WHERE open_date IS NOT NULL",
		"idx_ipo_close_date":            "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_close_date ON ipo_list(close_date) WHERE close_date IS NOT NULL",
		"idx_ipo_registrar":             "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_registrar ON ipo_list(registrar)",
		"idx_ipo_list_api":              "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_list_api ON ipo_list(status, created_at DESC)",
		"idx_ipo_gmp_company_code":      "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_gmp_company_code ON ipo_gmp(company_code)",
		"idx_ipo_gmp_ipo_name":          "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_gmp_ipo_name ON ipo_gmp(ipo_name)",
		"idx_ipo_result_cache_pan_hash": "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_result_cache_pan_hash ON ipo_result_cache(pan_hash)",
		"idx_ipo_result_cache_ipo_id":   "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_result_cache_ipo_id ON ipo_result_cache(ipo_id)",
		"idx_ipo_update_log_ipo_id":     "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_update_log_ipo_id ON ipo_update_log(ipo_id)",
		"idx_ipo_update_log_timestamp":  "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ipo_update_log_timestamp ON ipo_update_log(timestamp DESC)",
	}

	for _, indexName := range missingIndexes {
		if statement, exists := indexStatements[indexName]; exists {
			mv.logger.WithField("index_name", indexName).Info("Creating missing index")

			// Use a timeout for index creation
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			_, err := mv.db.ExecContext(ctx, statement)
			if err != nil {
				mv.logger.WithFields(logrus.Fields{
					"index_name": indexName,
					"error":      err,
				}).Error("Failed to create index")
				return fmt.Errorf("failed to create index %s: %w", indexName, err)
			}

			mv.logger.WithField("index_name", indexName).Info("Successfully created index")
		}
	}

	return nil
}

// updateDatabaseStatistics updates database statistics for better query planning
func (mv *MigrationValidator) updateDatabaseStatistics() error {
	mv.logger.Info("Updating database statistics for optimal query planning")

	// Update statistics for all tables
	tables := []string{"ipo_list", "ipo_gmp", "ipo_result_cache", "ipo_update_log"}

	for _, tableName := range tables {
		query := fmt.Sprintf("ANALYZE %s", tableName)
		_, err := mv.db.Exec(query)
		if err != nil {
			mv.logger.WithFields(logrus.Fields{
				"table": tableName,
				"error": err,
			}).Warn("Failed to analyze table")
			continue
		}

		mv.logger.WithField("table", tableName).Debug("Updated table statistics")
	}

	mv.logger.Info("Completed database statistics update")
	return nil
}

// Helper methods for migration validation

// tableExists checks if a table exists in the database
func (mv *MigrationValidator) tableExists(tableName string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = $1
		)
	`
	var exists bool
	err := mv.db.QueryRow(query, tableName).Scan(&exists)
	return exists, err
}

// getTableColumns returns a map of column names to their data types
func (mv *MigrationValidator) getTableColumns(tableName string) (map[string]string, error) {
	query := `
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_schema = 'public' AND table_name = $1
	`
	rows, err := mv.db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]string)
	for rows.Next() {
		var columnName, dataType string
		if err := rows.Scan(&columnName, &dataType); err != nil {
			return nil, err
		}
		columns[columnName] = dataType
	}

	return columns, rows.Err()
}

// getTableConstraints returns a list of constraint names for a table
func (mv *MigrationValidator) getTableConstraints(tableName string) ([]string, error) {
	query := `
		SELECT constraint_name 
		FROM information_schema.table_constraints 
		WHERE table_schema = 'public' AND table_name = $1
	`
	rows, err := mv.db.Query(query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints []string
	for rows.Next() {
		var constraintName string
		if err := rows.Scan(&constraintName); err != nil {
			return nil, err
		}
		constraints = append(constraints, constraintName)
	}

	return constraints, rows.Err()
}

// getAllIndexes returns a list of all index names in the database
func (mv *MigrationValidator) getAllIndexes() ([]string, error) {
	query := `
		SELECT indexname 
		FROM pg_indexes 
		WHERE schemaname = 'public'
	`
	rows, err := mv.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []string
	for rows.Next() {
		var indexName string
		if err := rows.Scan(&indexName); err != nil {
			return nil, err
		}
		indexes = append(indexes, indexName)
	}

	return indexes, rows.Err()
}

// constraintExists checks if a constraint exists in the list of constraints
func (mv *MigrationValidator) constraintExists(constraints []string, constraintName string) bool {
	for _, constraint := range constraints {
		if constraint == constraintName {
			return true
		}
	}
	return false
}

// indexExists checks if an index exists in the list of indexes
func (mv *MigrationValidator) indexExists(indexes []string, indexName string) bool {
	for _, index := range indexes {
		if index == indexName {
			return true
		}
	}
	return false
}

// GenerateMigrationReport generates a comprehensive report of the migration validation results
func (mv *MigrationValidator) GenerateMigrationReport(result *MigrationValidationResult) string {
	var reportBuilder strings.Builder

	reportBuilder.WriteString("=== Database Migration Validation Report ===\n\n")
	reportBuilder.WriteString(fmt.Sprintf("Overall Status: %s\n", map[bool]string{true: "VALID", false: "INVALID"}[result.IsValid]))
	reportBuilder.WriteString(fmt.Sprintf("Validation Errors: %d\n\n", len(result.ValidationErrors)))

	if len(result.MissingTables) > 0 {
		reportBuilder.WriteString("Missing Tables:\n")
		for _, table := range result.MissingTables {
			reportBuilder.WriteString(fmt.Sprintf("  - %s\n", table))
		}
		reportBuilder.WriteString("\n")
	}

	if len(result.MissingColumns) > 0 {
		reportBuilder.WriteString("Missing Columns:\n")
		for _, column := range result.MissingColumns {
			reportBuilder.WriteString(fmt.Sprintf("  - %s\n", column))
		}
		reportBuilder.WriteString("\n")
	}

	if len(result.MissingIndexes) > 0 {
		reportBuilder.WriteString("Missing Indexes:\n")
		for _, index := range result.MissingIndexes {
			reportBuilder.WriteString(fmt.Sprintf("  - %s\n", index))
		}
		reportBuilder.WriteString("\n")
	}

	if len(result.MissingConstraints) > 0 {
		reportBuilder.WriteString("Missing Constraints:\n")
		for _, constraint := range result.MissingConstraints {
			reportBuilder.WriteString(fmt.Sprintf("  - %s\n", constraint))
		}
		reportBuilder.WriteString("\n")
	}

	if len(result.PerformanceIssues) > 0 {
		reportBuilder.WriteString("Performance Issues:\n")
		for _, issue := range result.PerformanceIssues {
			reportBuilder.WriteString(fmt.Sprintf("  - %s\n", issue))
		}
		reportBuilder.WriteString("\n")
	}

	if len(result.Recommendations) > 0 {
		reportBuilder.WriteString("Recommendations:\n")
		for _, recommendation := range result.Recommendations {
			reportBuilder.WriteString(fmt.Sprintf("  - %s\n", recommendation))
		}
		reportBuilder.WriteString("\n")
	}

	if len(result.ValidationErrors) > 0 {
		reportBuilder.WriteString("Validation Errors:\n")
		for _, err := range result.ValidationErrors {
			reportBuilder.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
		}
	}

	return reportBuilder.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
