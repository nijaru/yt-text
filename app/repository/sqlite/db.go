package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"yt-text/errors"

	_ "github.com/mattn/go-sqlite3"
)

// Removing the embed FS since we'll manage schema differently
const schema = `
CREATE TABLE IF NOT EXISTS videos (
    id TEXT PRIMARY KEY,
    url TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL,
    transcription TEXT,
    error TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_videos_url ON videos(url);
CREATE INDEX IF NOT EXISTS idx_videos_status ON videos(status);
`

type DBConfig struct {
	MaxRetries         int
	RetryDelay         time.Duration
	QueryTimeout       time.Duration
	MaxConnections     int
	MaxIdleConnections int
	ConnMaxLifetime    time.Duration
}

func DefaultDBConfig() DBConfig {
	return DBConfig{
		MaxRetries:         3,
		RetryDelay:         time.Second,
		QueryTimeout:       30 * time.Second,
		MaxConnections:     10,
		MaxIdleConnections: 5,
		ConnMaxLifetime:    time.Hour,
	}
}

// Configure database with the provided settings
func ConfigureDB(db *sql.DB, config DBConfig) error {
	db.SetMaxOpenConns(config.MaxConnections)
	db.SetMaxIdleConns(config.MaxIdleConnections)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	return nil
}

func InitDB(dbPath string) (*sql.DB, error) {
	const op = "sqlite.InitDB"

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, errors.Internal(op, err, "failed to create database directory")
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, errors.Internal(op, err, "failed to open database")
	}

	// Set pragmas for better performance
	if err := configurePragmas(db); err != nil {
		db.Close()
		return nil, err
	}

	// Execute schema
	if err := execSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func configurePragmas(db *sql.DB) error {
	const op = "sqlite.configurePragmas"

	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA temp_store = MEMORY",
		"PRAGMA cache_size = -2000", // Use up to 2MB of memory for cache
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return errors.Internal(op, err, fmt.Sprintf("failed to set pragma: %s", pragma))
		}
	}

	return nil
}

func execSchema(db *sql.DB) error {
	const op = "sqlite.execSchema"

	// Split into individual statements
	statements := strings.Split(schema, ";")

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return errors.Internal(op, err, "failed to begin transaction")
	}
	defer tx.Rollback()

	// Execute each statement
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		if _, err := tx.Exec(stmt); err != nil {
			return errors.Internal(
				op,
				err,
				fmt.Sprintf("failed to execute schema statement: %s", stmt),
			)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return errors.Internal(op, err, "failed to commit schema transaction")
	}

	return nil
}

// Retry helper function
func withRetry(ctx context.Context, config DBConfig, op string, fn func() error) error {
	var lastErr error
	for i := 0; i < config.MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return errors.Internal(op, ctx.Err(), "context cancelled")
		default:
			if err := fn(); err != nil {
				lastErr = err
				time.Sleep(config.RetryDelay)
				continue
			}
			return nil
		}
	}
	return errors.Internal(op, lastErr, "max retries exceeded")
}

type Executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

// TxFn is a function that will be called with a transaction
type TxFn func(tx Executor) error

// WithTransaction wraps a transaction with proper rollback/commit logic
func WithTransaction(ctx context.Context, db *sql.DB, fn TxFn) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic after rollback
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
