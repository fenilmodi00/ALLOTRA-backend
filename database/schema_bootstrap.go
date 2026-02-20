package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// EnsureBaseSchema applies schema.sql only when core tables are missing.
// This prevents fresh environments from failing with "relation does not exist"
// while keeping existing databases untouched.
func EnsureBaseSchema(db *sql.DB, schemaFilePath string) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	var tableName sql.NullString
	if err := db.QueryRow(`SELECT to_regclass('public.ipo_list')`).Scan(&tableName); err != nil {
		return fmt.Errorf("check base schema existence: %w", err)
	}

	if tableName.Valid {
		return nil
	}

	schemaSQL, resolvedPath, err := loadSchemaSQL(schemaFilePath)
	if err != nil {
		return fmt.Errorf("load schema file: %w", err)
	}

	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("apply base schema from %s: %w", resolvedPath, err)
	}

	if err := db.QueryRow(`SELECT to_regclass('public.ipo_list')`).Scan(&tableName); err != nil {
		return fmt.Errorf("verify base schema creation: %w", err)
	}
	if !tableName.Valid {
		return fmt.Errorf("base schema apply completed but public.ipo_list is still missing")
	}

	logrus.WithField("schema_file", resolvedPath).Info("Applied bootstrap schema (public.ipo_list was missing)")
	return nil
}

func loadSchemaSQL(schemaFilePath string) (string, string, error) {
	if content, err := os.ReadFile(schemaFilePath); err == nil {
		return string(content), schemaFilePath, nil
	}

	execPath, execErr := os.Executable()
	if execErr == nil {
		execDir := filepath.Dir(execPath)
		candidate := filepath.Join(execDir, schemaFilePath)
		if content, err := os.ReadFile(candidate); err == nil {
			return string(content), candidate, nil
		}
	}

	return "", "", fmt.Errorf("schema file not found at %s", schemaFilePath)
}
