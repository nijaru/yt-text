package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func initializeDB() error {
	log.Println("Initializing database")

	var err error
	db, err = sql.Open("sqlite3", "./urls.db")
	if err != nil {
		return fmt.Errorf("error opening database: %v", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS urls (
                        id INTEGER PRIMARY KEY AUTOINCREMENT,
                        url TEXT NOT NULL UNIQUE,
                        text TEXT,
                        status TEXT NOT NULL DEFAULT 'pending'
    )`)
	if err != nil {
		return fmt.Errorf("error creating table: %v", err)
	}

	return nil
}

func validateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("error: URL is required")
	}

	rawURL = strings.TrimSpace(rawURL)

	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("error: invalid URL format")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("error: URL must start with http or https")
	}

	return nil
}

func getTranscription(ctx context.Context, url string) (string, string, error) {
	var text sql.NullString
	var status string
	err := db.QueryRowContext(ctx, "SELECT text, status FROM urls WHERE url = ?", url).Scan(&text, &status)
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

func setTranscriptionStatus(ctx context.Context, url, status string) error {
	log.Printf("Setting transcription status for URL %s to %s", url, status)
	stmt, err := db.PrepareContext(ctx, "UPDATE urls SET status = ? WHERE url = ?")
	if err != nil {
		return fmt.Errorf("error preparing statement: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, status, url)
	if err != nil {
		return fmt.Errorf("error executing statement: %v", err)
	}

	return nil
}

func deleteTranscription(ctx context.Context, url string) error {
	stmt, err := db.PrepareContext(ctx, "DELETE FROM urls WHERE url = ?")
	if err != nil {
		return fmt.Errorf("error preparing delete statement: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, url)
	if err != nil {
		return fmt.Errorf("error executing delete statement: %v", err)
	}

	return nil
}

func transcribeHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request: %s %s", r.Method, r.URL.Path)
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	url := r.FormValue("url")

	if err := validateURL(url); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	text, err := handleTranscription(ctx, url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	sendJSONResponse(w, text)
}

func handleTranscription(ctx context.Context, url string) (string, error) {
	text, status, err := getTranscription(ctx, url)
	if err != nil {
		return "", err
	}

	if status == "completed" {
		log.Printf("Transcription for URL %s found in database.", url)
		return text, nil
	}

	if status == "in_progress" {
		log.Printf("Transcription for URL %s is in progress. Waiting for it to be processed...", url)
		text, err = waitForTranscription(ctx, url)
		if err != nil {
			return "", err
		}
		if text != "" {
			return text, nil
		}
	}

	if status == "pending" {
		log.Printf("Transcription for URL %s not found in database. Proceeding to transcribe...", url)
		if err := setTranscriptionStatus(ctx, url, "in_progress"); err != nil {
			return "", fmt.Errorf("error setting transcription status: %v", err)
		}
	}

	text, err = runTranscriptionScript(ctx, url)
	if err != nil {
		setTranscriptionStatus(ctx, url, "failed")
		return "", err
	}

	err = setTranscription(ctx, url, text)
	if err != nil {
		return "", fmt.Errorf("error saving transcription: %v", err)
	}

	setTranscriptionStatus(ctx, url, "completed")
	return text, nil
}

	text, err = runTranscriptionScript(ctx, url)
	if err != nil {
		// Use a new context for the delete operation to avoid context cancellation issues
		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer deleteCancel()
		if deleteErr := deleteTranscription(deleteCtx, url); deleteErr != nil {
			log.Printf("Error removing transcription: %v", deleteErr)
		}
		return "", err
	}

	err = setTranscription(ctx, url, text)
	if err != nil {
		return "", fmt.Errorf("error saving transcription: %v", err)
	}

	return text, nil
}

func waitForTranscription(ctx context.Context, url string) (string, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			text, status, err := getTranscription(ctx, url)
			if err != nil {
				return "", err
			}
			if status == "completed" {
				log.Printf("Transcription for URL %s found in database.", url)
				return text, nil
			}
			if status == "failed" {
				return "", fmt.Errorf("transcription failed for URL %s", url)
			}
		}
	}
}

func sendJSONResponse(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"transcription": text})
}

func runTranscriptionScript(ctx context.Context, url string) (string, error) {
	log.Printf("Transcribing URL: %s", url)

	cmd := exec.CommandContext(ctx, "uv", "run", "transcribe.py", url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error transcribing: %v, output: %s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	filename := lines[len(lines)-1]
	defer func() {
		if err := os.Remove(filename); err != nil {
			log.Printf("Error removing file: %v", err)
		}
	}()

	fileContent, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("error reading file: %v", err)
	}
	text := string(fileContent)
	if text == "" {
		return "", fmt.Errorf("error transcribing")
	}

	log.Printf("Transcription for URL %s completed.", url)
	return text, nil
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving index.html for request: %s %s", r.Method, r.URL.Path)
	http.ServeFile(w, r, "./static/index.html")
}

func main() {
	err := initializeDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/transcribe", transcribeHandler)

	server := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		log.Println("Listening on port 8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on :8080: %v\n", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	log.Println("Shutting down the server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown: %v", err)
	}
}
