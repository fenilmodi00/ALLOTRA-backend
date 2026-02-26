package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fenilmodi00/ipo-backend/shared"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

// DB is kept for backward compatibility with existing tests/tools.
// New code should pass *sql.DB explicitly via dependency injection.
var DB *sql.DB

func Connect(dbURL string) error {
	config := shared.NewDefaultUnifiedConfiguration().Database
	_, err := ConnectWithConfigAndReturn(dbURL, &config)
	return err
}

func ConnectDB(dbURL string) (*sql.DB, error) {
	config := shared.NewDefaultUnifiedConfiguration().Database
	return ConnectWithConfigAndReturn(dbURL, &config)
}

func ConnectWithConfigAndReturn(dbURL string, config *shared.DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), config.PingTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	DB = db

	logrus.WithFields(logrus.Fields{
		"max_open_conns":     config.MaxOpenConns,
		"max_idle_conns":     config.MaxIdleConns,
		"conn_max_lifetime":  config.ConnMaxLifetime,
		"conn_max_idle_time": config.ConnMaxIdleTime,
	}).Info("Connected to database")

	return db, nil
}

func Close() {
	if DB != nil {
		_ = DB.Close()
		DB = nil
		logrus.Info("Database connection closed")
	}
}

func HealthCheck() error {
	if DB == nil {
		return fmt.Errorf("database connection not established")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := DB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	stats := DB.Stats()
	if stats.OpenConnections == 0 {
		return fmt.Errorf("no open database connections")
	}

	return nil
}
