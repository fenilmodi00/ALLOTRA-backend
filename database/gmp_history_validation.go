package database

import (
	"fmt"
)

// validateGMPPriceHistoryTableStructure validates the GMP price history table structure
func (v *SchemaValidator) validateGMPPriceHistoryTableStructure() (*ValidationResult, error) {
	result := &ValidationResult{
		TableName:          "gmp_price_history",
		IsValid:            true,
		MissingColumns:     make([]string, 0),
		MissingIndexes:     make([]string, 0),
		InvalidConstraints: make([]string, 0),
		Recommendations:    make([]string, 0),
	}

	// Check if table exists
	exists, err := v.tableExists("gmp_price_history")
	if err != nil {
		return nil, fmt.Errorf("failed to check if gmp_price_history table exists: %w", err)
	}
	if !exists {
		result.IsValid = false
		result.MissingColumns = append(result.MissingColumns, "entire table missing")
		result.Recommendations = append(result.Recommendations, "Create gmp_price_history table with complete schema")
		return result, nil
	}

	// Required columns for GMP price history
	requiredColumns := map[string]string{
		"id":                  "uuid",
		"ipo_id":              "uuid",
		"company_code":        "varchar(50)",
		"record_date":         "date",
		"ipo_price":           "decimal(10,2)",
		"gmp_value":           "decimal(10,2)",
		"estimated_listing":   "decimal(10,2)",
		"listing_percent":     "decimal(10,2)",
		"estimated_profit":    "decimal(10,2)",
		"subscription_status": "varchar(100)",
		"sub2_sauda":          "decimal(10,2)",
		"last_updated":        "varchar(200)",
		"data_source":         "varchar(100)",
		"created_at":          "timestamp",
		"updated_at":          "timestamp",
	}

	// Check for missing columns
	existingColumns, err := v.getTableColumns("gmp_price_history")
	if err != nil {
		return nil, fmt.Errorf("failed to get gmp_price_history columns: %w", err)
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
	constraints, err := v.getTableConstraints("gmp_price_history")
	if err != nil {
		return nil, fmt.Errorf("failed to get gmp_price_history constraints: %w", err)
	}

	requiredConstraints := []string{
		"fk_gmp_history_ipo_id",
		"uk_gmp_history_ipo_date",
		"chk_gmp_history_prices_positive",
		"chk_gmp_history_listing_calculation",
		"gmp_price_history_company_code_not_empty",
		"gmp_price_history_data_source_not_empty",
	}

	for _, constraintName := range requiredConstraints {
		if !v.constraintExists(constraints, constraintName) {
			result.InvalidConstraints = append(result.InvalidConstraints, constraintName)
			result.IsValid = false
		}
	}

	// Check for required indexes
	existingIndexes, err := v.getAllIndexes()
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}

	requiredIndexes := []string{
		"idx_gmp_history_ipo_id",
		"idx_gmp_history_company_code",
		"idx_gmp_history_record_date",
		"idx_gmp_history_ipo_date_range",
		"idx_gmp_history_created_at",
	}

	for _, indexName := range requiredIndexes {
		if !v.indexExists(existingIndexes, indexName) {
			result.MissingIndexes = append(result.MissingIndexes, indexName)
			result.IsValid = false
		}
	}

	// Add recommendations
	if len(result.MissingColumns) > 0 || len(result.InvalidConstraints) > 0 {
		result.Recommendations = append(result.Recommendations,
			"Update gmp_price_history table schema to support historical GMP data tracking")
	}

	return result, nil
}

// validateGMPHistoryJobLogTableStructure validates the job log table structure
func (v *SchemaValidator) validateGMPHistoryJobLogTableStructure() (*ValidationResult, error) {
	result := &ValidationResult{
		TableName:          "gmp_history_job_log",
		IsValid:            true,
		MissingColumns:     make([]string, 0),
		MissingIndexes:     make([]string, 0),
		InvalidConstraints: make([]string, 0),
		Recommendations:    make([]string, 0),
	}

	// Check if table exists
	exists, err := v.tableExists("gmp_history_job_log")
	if err != nil {
		return nil, fmt.Errorf("failed to check if gmp_history_job_log table exists: %w", err)
	}
	if !exists {
		result.IsValid = false
		result.MissingColumns = append(result.MissingColumns, "entire table missing")
		result.Recommendations = append(result.Recommendations, "Create gmp_history_job_log table for job execution tracking")
		return result, nil
	}

	// Required columns for job log
	requiredColumns := map[string]string{
		"id":                  "uuid",
		"job_start_time":      "timestamp",
		"job_end_time":        "timestamp",
		"ipos_processed":      "integer",
		"successful_scrapes":  "integer",
		"failed_scrapes":      "integer",
		"total_records_added": "integer",
		"execution_status":    "varchar(50)",
		"error_summary":       "text",
		"created_at":          "timestamp",
	}

	// Check for missing columns
	existingColumns, err := v.getTableColumns("gmp_history_job_log")
	if err != nil {
		return nil, fmt.Errorf("failed to get gmp_history_job_log columns: %w", err)
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
	constraints, err := v.getTableConstraints("gmp_history_job_log")
	if err != nil {
		return nil, fmt.Errorf("failed to get gmp_history_job_log constraints: %w", err)
	}

	requiredConstraints := []string{
		"gmp_history_job_log_execution_status_not_empty",
		"gmp_history_job_log_ipos_processed_non_negative",
		"gmp_history_job_log_successful_scrapes_non_negative",
		"gmp_history_job_log_failed_scrapes_non_negative",
		"gmp_history_job_log_total_records_non_negative",
	}

	for _, constraintName := range requiredConstraints {
		if !v.constraintExists(constraints, constraintName) {
			result.InvalidConstraints = append(result.InvalidConstraints, constraintName)
			result.IsValid = false
		}
	}

	// Check for required indexes
	existingIndexes, err := v.getAllIndexes()
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}

	requiredIndexes := []string{
		"idx_gmp_history_job_log_job_start_time",
		"idx_gmp_history_job_log_execution_status",
		"idx_gmp_history_job_log_created_at",
	}

	for _, indexName := range requiredIndexes {
		if !v.indexExists(existingIndexes, indexName) {
			result.MissingIndexes = append(result.MissingIndexes, indexName)
			result.IsValid = false
		}
	}

	// Add recommendations
	if len(result.MissingColumns) > 0 || len(result.InvalidConstraints) > 0 {
		result.Recommendations = append(result.Recommendations,
			"Update gmp_history_job_log table schema for proper job execution tracking")
	}

	return result, nil
}

// ValidateGMPHistorySchema validates both GMP history tables
func ValidateGMPHistorySchema() error {
	if DB == nil {
		return fmt.Errorf("database connection not established")
	}

	validator := NewSchemaValidator(DB)

	// Validate GMP price history table
	historyResult, err := validator.validateGMPPriceHistoryTableStructure()
	if err != nil {
		return fmt.Errorf("failed to validate gmp_price_history table: %w", err)
	}

	if !historyResult.IsValid {
		return fmt.Errorf("gmp_price_history table validation failed: missing columns=%v, missing indexes=%v, invalid constraints=%v",
			historyResult.MissingColumns, historyResult.MissingIndexes, historyResult.InvalidConstraints)
	}

	// Validate job log table
	jobLogResult, err := validator.validateGMPHistoryJobLogTableStructure()
	if err != nil {
		return fmt.Errorf("failed to validate gmp_history_job_log table: %w", err)
	}

	if !jobLogResult.IsValid {
		return fmt.Errorf("gmp_history_job_log table validation failed: missing columns=%v, missing indexes=%v, invalid constraints=%v",
			jobLogResult.MissingColumns, jobLogResult.MissingIndexes, jobLogResult.InvalidConstraints)
	}

	return nil
}
