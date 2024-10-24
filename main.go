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
                        url TEXT NOT NULL,
                        text TEXT NOT NULL
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

func getTranscription(ctx context.Context, url string) (string, bool, error) {
	var body sql.NullString
	err := db.QueryRowContext(ctx, "SELECT text FROM urls WHERE url = ?", url).Scan(&body)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("error querying database: %v", err)
	}

	if !body.Valid {
		return "", true, nil
	}

	return body.String, true, nil
}

func setTranscription(ctx context.Context, url, text string) error {
	stmt, err := db.PrepareContext(ctx, "INSERT INTO urls (url, text) VALUES (?, ?) ON CONFLICT(url) DO UPDATE SET text=excluded.text")
	if err != nil {
		return fmt.Errorf("error preparing statement: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, url, text)
	if err != nil {
		return fmt.Errorf("error executing statement: %v", err)
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
	text, found, err := getTranscription(ctx, url)
	if err != nil {
		return "", err
	}

	if found && text != "" {
		log.Printf("Transcription for URL %s found in database.", url)
		return text, nil
	}

	if !found {
		log.Printf("Transcription for URL %s not found in database. Proceeding to transcribe...", url)
		if err := setTranscription(ctx, url, ""); err != nil {
			return "", fmt.Errorf("error inserting empty transcription: %v", err)
		}
	} else {
		log.Printf("Transcription for URL %s not found in database. Waiting for it to be processed...", url)
		text, err = waitForTranscription(ctx, url)
		if err != nil {
			return "", err
		}
		if text != "" {
			return text, nil
		}
	}

	text, err = runTranscriptionScript(ctx, url)
	if err != nil {
		return "", err
	}

	if err := setTranscription(ctx, url, text); err != nil {
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
			text, _, err := getTranscription(ctx, url)
			if err != nil {
				return "", err
			}
			if text != "" {
				log.Printf("Transcription for URL %s found in database.", url)
				return text, nil
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