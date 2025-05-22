// internal/storage/database.go
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/mbaxamb3/nusli/scraper-service/db/sqlc"
	"go.uber.org/zap"
)

// Database represents the database connection and operations
type Database struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *zap.Logger
}

// NewDatabase creates a new database connection
func NewDatabase(databaseURL string, logger *zap.Logger) (*Database, error) {
	// Configure connection pool
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Set pool configuration
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 5 * time.Minute
	config.MaxConnIdleTime = 1 * time.Minute

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create SQLC queries
	queries := db.New(pool)

	database := &Database{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}

	logger.Info("Database connection established successfully")

	return database, nil
}

// Close closes the database connection pool
func (d *Database) Close() {
	if d.pool != nil {
		d.logger.Info("Closing database connection pool")
		d.pool.Close()
	}
}

// GetQueries returns the SQLC queries instance
func (d *Database) GetQueries() *db.Queries {
	return d.queries
}

// GetPool returns the underlying pgxpool.Pool instance
func (d *Database) GetPool() *pgxpool.Pool {
	return d.pool
}

// Health checks the database connection health
func (d *Database) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := d.pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Test a simple query
	var count int
	err := d.pool.QueryRow(ctx, "SELECT COUNT(*) FROM scraping_jobs").Scan(&count)
	if err != nil {
		return fmt.Errorf("test query failed: %w", err)
	}

	return nil
}

// Transaction executes a function within a database transaction
func (d *Database) Transaction(ctx context.Context, fn func(*db.Queries) error) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	qtx := d.queries.WithTx(tx)

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				d.logger.Error("Failed to rollback transaction",
					zap.Error(rbErr),
					zap.Error(err))
			}
		} else {
			err = tx.Commit(ctx)
		}
	}()

	err = fn(qtx)
	return err
}

// Cleanup removes old completed/failed jobs
func (d *Database) Cleanup(ctx context.Context, retentionPeriod time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retentionPeriod)

	result, err := d.pool.Exec(ctx,
		"DELETE FROM scraping_jobs WHERE created_at < $1 AND status IN ('completed', 'failed', 'canceled')",
		cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup failed: %w", err)
	}

	rowsAffected := result.RowsAffected()

	d.logger.Info("Database cleanup completed",
		zap.Int64("rows_deleted", rowsAffected),
		zap.Duration("retention_period", retentionPeriod))

	return rowsAffected, nil
}

// GetStats returns database connection pool statistics
func (d *Database) GetStats() *pgxpool.Stat {
	return d.pool.Stat()
}
