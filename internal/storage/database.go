// internal/storage/database.go
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	db "github.com/mbaxamb3/nusli/scraper-service/db/sqlc"
	"go.uber.org/zap"
)

// Database represents the database connection and operations
type Database struct {
	db      *sql.DB
	queries *db.Queries
	logger  *zap.Logger
}

// NewDatabase creates a new database connection
func NewDatabase(databaseURL string, logger *zap.Logger) (*Database, error) {
	// Open database connection
	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(1 * time.Minute)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create SQLC queries
	queries := db.New(sqlDB)

	database := &Database{
		db:      sqlDB,
		queries: queries,
		logger:  logger,
	}

	logger.Info("Database connection established successfully")

	return database, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		d.logger.Info("Closing database connection")
		return d.db.Close()
	}
	return nil
}

// GetQueries returns the SQLC queries instance
func (d *Database) GetQueries() *db.Queries {
	return d.queries
}

// GetDB returns the underlying sql.DB instance
func (d *Database) GetDB() *sql.DB {
	return d.db
}

// Health checks the database connection health
func (d *Database) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := d.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Test a simple query
	var count int
	err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM scraping_jobs").Scan(&count)
	if err != nil {
		return fmt.Errorf("test query failed: %w", err)
	}

	return nil
}

// Transaction executes a function within a database transaction
func (d *Database) Transaction(ctx context.Context, fn func(*db.Queries) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	qtx := d.queries.WithTx(tx)

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				d.logger.Error("Failed to rollback transaction",
					zap.Error(rbErr),
					zap.Error(err))
			}
		} else {
			err = tx.Commit()
		}
	}()

	err = fn(qtx)
	return err
}

// Cleanup removes old completed/failed jobs
func (d *Database) Cleanup(ctx context.Context, retentionPeriod time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retentionPeriod)

	result, err := d.db.ExecContext(ctx,
		"DELETE FROM scraping_jobs WHERE created_at < $1 AND status IN ('completed', 'failed', 'canceled')",
		cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	d.logger.Info("Database cleanup completed",
		zap.Int64("rows_deleted", rowsAffected),
		zap.Duration("retention_period", retentionPeriod))

	return rowsAffected, nil
}

// GetStats returns database statistics
func (d *Database) GetStats() sql.DBStats {
	return d.db.Stats()
}
