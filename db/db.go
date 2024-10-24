package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitializeDB(dbPath string) error {
	log.Println("Initializing database")

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
                        request_id TEXT NOT NULL UNIQUE,
                        url TEXT NOT NULL UNIQUE,
                        text TEXT,
                        status TEXT NOT NULL DEFAULT 'pending'
    )`)
	if err != nil {
		DB.Close() // Close the database connection in case of an error
		return fmt.Errorf("error creating table: %v", err)
	}

	return nil
}

func GetTranscription(ctx context.Context, requestID string) (string, string, error) {
	var text sql.NullString
	var status string
	err := DB.QueryRowContext(ctx, "SELECT text, status FROM urls WHERE request_id = ?", requestID).Scan(&text, &status)
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

func SetTranscription(ctx context.Context, requestID, url, text string) error {
	stmt, err := DB.PrepareContext(ctx, "INSERT INTO urls (request_id, url, text, status) VALUES (?, ?, ?, 'completed') ON CONFLICT(request_id) DO UPDATE SET text=excluded.text, status='completed'")
	if err != nil {
		return fmt.Errorf("error preparing statement: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, requestID, url, text)
	if err != nil {
		return fmt.Errorf("error executing statement: %v", err)
	}

	return nil
}

func SetTranscriptionStatus(ctx context.Context, requestID, status string) error {
	stmt, err := DB.PrepareContext(ctx, "UPDATE urls SET status = ? WHERE request_id = ?")
	if err != nil {
		return fmt.Errorf("error preparing statement: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, status, requestID)
	if err != nil {
		return fmt.Errorf("error executing statement: %v", err)
	}

	return nil
}

func DeleteTranscription(ctx context.Context, requestID string) error {
	stmt, err := DB.PrepareContext(ctx, "DELETE FROM urls WHERE request_id = ?")
	if err != nil {
		return fmt.Errorf("error preparing delete statement: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, requestID)
	if err != nil {
		return fmt.Errorf("error executing delete statement: %v", err)
	}

	return nil
}
