// Package postgres holds Postgres connection setup, returning a *gorm.DB.
// It only connects — schema migrations are applied separately, by the
// docker-compose "migrate" one-shot service; this assumes the database is
// already migrated.
package postgres

import (
	"context"
	"fmt"
	"time"

	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"smart-grid-asset-management/backend/internal/base/logs"
)

// Config configures the Postgres connection.
type Config struct {
	DataSourceName string        // e.g. postgres://user:pass@host:port/db?sslmode=disable
	MaxRetries     int           // connection retry attempts (default 10)
	RetryDelay     time.Duration // delay between retries (default 3s)
}

// Open makes one attempt at a GORM handle over dsn, and pings it.
//
// The GORM configuration is deliberate:
//   - SkipDefaultTransaction: every write is already either a single
//     statement or inside an explicit repository.InTx, so GORM's implicit
//     per-write transaction would only add round trips (and, inside InTx,
//     nested savepoints).
//   - TranslateError: unique/foreign-key violations surface as
//     gorm.ErrDuplicatedKey / gorm.ErrForeignKeyViolated, which the
//     repository maps to errx.ErrConflict.
//   - a silent logger: failures are returned and logged once, with the
//     trace ID, by the layers above; GORM's own log would be a second,
//     uncorrelated copy (and would print row data on slow queries).
func Open(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(pgdriver.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
		TranslateError:         true,
	})
	if err != nil {
		if db != nil {
			Close(db) // gorm returns the handle alongside a failed ping
		}
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}

// New opens a Postgres connection, retrying until it succeeds or
// MaxRetries is exhausted. This is a deliberately small stand-in for the
// retry+mTLS setup this pattern usually has (see docs/ASSUMPTIONS.md) —
// this project has one Postgres and no TLS.
func New(ctx context.Context, cfg Config) (*gorm.DB, error) {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 10
	}
	retryDelay := cfg.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 3 * time.Second
	}

	var (
		db  *gorm.DB
		err error
	)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if db, err = Open(cfg.DataSourceName); err == nil {
			return db, nil
		}
		logs.WithCtx(ctx).Warn("postgres connect failed", "attempt", attempt, "max_retries", maxRetries, "err", err)
		if attempt == maxRetries {
			break
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("postgres connect cancelled: %w", ctx.Err())
		case <-time.After(retryDelay):
		}
	}
	return nil, fmt.Errorf("postgres unreachable after %d attempts: %w", maxRetries, err)
}

// Close releases the handle's connection pool.
func Close(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
