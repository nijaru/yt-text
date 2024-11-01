package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitializeDB(dbPath string) error {
	logrus.Info("Initializing database")

	// Ensure the directory for the database file exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("error creating directory for database: %v", err)
	}

	var err error
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("error opening database: %v", err)
	}

	// Set connection pool settings
	DB.SetMaxOpenConns(10)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(30 * time.Minute)

	_, err = DB.Exec(`CREATE TABLE IF NOT EXISTS urls (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    url TEXT NOT NULL UNIQUE,
                    text TEXT,
                    status TEXT NOT NULL DEFAULT 'pending',
                    model_version TEXT NOT NULL DEFAULT ''
)`)
	if err != nil {
		DB.Close() // Close the database connection in case of an error
		return fmt.Errorf("error creating table: %v", err)
	}

	return nil
}

func GetTranscription(ctx context.Context, url string) (string, string, error) {
	var text sql.NullString
	var status string
	err := DB.QueryRowContext(ctx, "SELECT text, status FROM urls WHERE url = ?", url).Scan(&text, &status)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "pending", nil
		}
		return "", "", fmt.Errorf("error querying database: %v", err)
	}

	if !text.Valid {
		return "", status, nil
	}

	return text.String, status, nil
}

func GetModelName(ctx context.Context, url string) (string, error) {
    var modelName string
    err := DB.QueryRowContext(ctx, "SELECT model_name FROM urls WHERE url = ?", url).Scan(&modelName)
    if err != nil {
        if err == sql.ErrNoRows {
            return "", nil
        }
        return "", fmt.Errorf("error querying database: %v", err)
    }
    return modelName, nil
}

func SetTranscription(ctx context.Context, url, text, modelName string) error {
    tx, err := DB.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("error beginning transaction: %v", err)
    }

    stmt, err := tx.PrepareContext(ctx, "INSERT INTO urls (url, text, status, model_name) VALUES (?, ?, 'completed', ?) ON CONFLICT(url) DO UPDATE SET text=excluded.text, status='completed', model_name=excluded.model_name")
    if err != nil {
        tx.Rollback()
        return fmt.Errorf("error preparing statement: %v", err)
    }
    defer stmt.Close()

    _, err = stmt.ExecContext(ctx, url, text, modelName)
    if err != nil {
        tx.Rollback()
        return fmt.Errorf("error executing statement: %v", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("error committing transaction: %v", err)
    }

    return nil
}

func SetTranscriptionStatus(ctx context.Context, url, status string) error {
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error beginning transaction: %v", err)
	}

	stmt, err := tx.PrepareContext(ctx, "UPDATE urls SET status = ? WHERE url = ?")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error preparing statement: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, status, url)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error executing statement: %v", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	return nil
}

func DeleteTranscription(ctx context.Context, url string) error {
	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error beginning transaction: %v", err)
	}

	stmt, err := tx.PrepareContext(ctx, "DELETE FROM urls WHERE url = ?")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error preparing delete statement: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, url)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("error executing delete statement: %v", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	return nil
}