package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nijaru/yt-text/errors"
	"github.com/sirupsen/logrus"
)

var DB *sql.DB

func InitializeDB(dbPath string) error {
	const op = "db.InitializeDB"
	logrus.Info("Initializing database")

	// Ensure the directory for the database file exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return errors.Internal(op, err, "Failed to create database directory")
	}

	var err error
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return errors.Internal(op, err, "Failed to open database")
	}

	// Set connection pool settings
	DB.SetMaxOpenConns(5)
	DB.SetMaxIdleConns(2)
	DB.SetConnMaxLifetime(15 * time.Minute)

	_, err = DB.Exec(`CREATE TABLE IF NOT EXISTS urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL UNIQUE,
		text TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		model_name TEXT NOT NULL DEFAULT '',
		summary TEXT,
		summary_model_name TEXT
	)`)
	if err != nil {
		DB.Close() // Close the database connection in case of an error
		return errors.Internal(op, err, "Failed to create table")
	}

	_, err = DB.Exec(`CREATE INDEX IF NOT EXISTS idx_urls_url ON urls(url)`)
	if err != nil {
		DB.Close()
		return errors.Internal(op, err, "Failed to create index")
	}

	// Add context with timeout for initialization
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test connection with timeout
	if err := DB.PingContext(ctx); err != nil {
		return errors.Internal(op, err, "Failed to connect to database")
	}

	return nil
}

func GetTranscription(ctx context.Context, url string) (string, string, error) {
	const op = "db.GetTranscription"
	var text sql.NullString
	var status string

	err := DB.QueryRowContext(ctx, "SELECT text, status FROM urls WHERE url = ?", url).Scan(&text, &status)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "pending", nil
		}
		return "", "", errors.Internal(op, err, "Failed to query transcription")
	}

	if !text.Valid {
		return "", status, nil
	}

	return text.String, status, nil
}

func GetModelName(ctx context.Context, url string) (string, error) {
	const op = "db.GetModelName"
	var modelName string

	err := DB.QueryRowContext(ctx, "SELECT model_name FROM urls WHERE url = ?", url).Scan(&modelName)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", errors.Internal(op, err, "Failed to query model name")
	}

	return modelName, nil
}

func SetTranscription(ctx context.Context, url, text, modelName string) error {
	const op = "db.SetTranscription"

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return errors.Internal(op, err, "Failed to begin transaction")
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO urls (url, text, status, model_name)
        VALUES (?, ?, 'completed', ?)
        ON CONFLICT(url)
        DO UPDATE SET text=excluded.text, status='completed', model_name=excluded.model_name`)
	if err != nil {
		return errors.Internal(op, err, "Failed to prepare statement")
	}
	defer stmt.Close()

	if _, err = stmt.ExecContext(ctx, url, text, modelName); err != nil {
		return errors.Internal(op, err, "Failed to execute statement")
	}

	return tx.Commit()
}

func SetTranscriptionStatus(ctx context.Context, url, status string) error {
	const op = "db.SetTranscriptionStatus"

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return errors.Internal(op, err, "Failed to begin transaction")
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "UPDATE urls SET status = ? WHERE url = ?")
	if err != nil {
		return errors.Internal(op, err, "Failed to prepare statement")
	}
	defer stmt.Close()

	result, err := stmt.ExecContext(ctx, status, url)
	if err != nil {
		return errors.Internal(op, err, "Failed to execute statement")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Internal(op, err, "Failed to get rows affected")
	}

	if rowsAffected == 0 {
		return errors.Internal(op, nil, "No rows updated")
	}

	return tx.Commit()
}

func DeleteTranscription(ctx context.Context, url string) error {
	const op = "db.DeleteTranscription"

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return errors.Internal(op, err, "Failed to begin transaction")
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "DELETE FROM urls WHERE url = ?")
	if err != nil {
		return errors.Internal(op, err, "Failed to prepare statement")
	}
	defer stmt.Close()

	if _, err = stmt.ExecContext(ctx, url); err != nil {
		return errors.Internal(op, err, "Failed to execute statement")
	}

	return tx.Commit()
}

func GetSummary(ctx context.Context, url string) (string, string, error) {
	const op = "db.GetSummary"
	var summary sql.NullString
	var summaryModelName sql.NullString

	err := DB.QueryRowContext(ctx, "SELECT summary, summary_model_name FROM urls WHERE url = ?", url).Scan(&summary, &summaryModelName)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", nil
		}
		return "", "", errors.Internal(op, err, "Failed to query summary")
	}

	return summary.String, summaryModelName.String, nil
}

func SetSummary(ctx context.Context, url, summary, summaryModelName string) error {
	const op = "db.SetSummary"

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return errors.Internal(op, err, "Failed to begin transaction")
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO urls (url, summary, summary_model_name)
        VALUES (?, ?, ?)
        ON CONFLICT(url)
        DO UPDATE SET summary=excluded.summary, summary_model_name=excluded.summary_model_name`)
	if err != nil {
		return errors.Internal(op, err, "Failed to prepare statement")
	}
	defer stmt.Close()

	if _, err = stmt.ExecContext(ctx, url, summary, summaryModelName); err != nil {
		return errors.Internal(op, err, "Failed to execute statement")
	}

	return tx.Commit()
}
