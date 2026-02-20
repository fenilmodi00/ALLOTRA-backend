package database

import (
	"database/sql"
	"fmt"
	"runtime/debug"

	"github.com/pressly/goose/v3"
)

func RunMigrations(db *sql.DB, migrationsDir string) (err error) {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("goose migrations panicked: %v\n%s", recovered, debug.Stack())
		}
	}()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("run goose migrations: %w", err)
	}

	return nil
}
