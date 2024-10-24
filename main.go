package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/db"
	"golang.org/x/time/rate"
)

var (
	cfg               *config.Config
	transcriptionLocks sync.Map
	rateLimiter       *rate.Limiter
)

type transcriptionLock struct {
	mu sync.Mutex
}

func getTranscriptionLock(url string) *transcriptionLock {
	lock, _ := transcriptionLocks.LoadOrStore(url, &transcriptionLock{})
	return lock.(*transcriptionLock)
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

func transcribeHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("INFO: Received request: %s %s", r.Method, r.URL.Path)
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		log.Printf("ERROR: Invalid request method: %s", r.Method)
		return
	}

	url := r.FormValue("url")

	if err := validateURL(url); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Printf("ERROR: URL validation failed: %v", err)
		return
	}

	if !rateLimiter.Allow() {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		log.Printf("ERROR: Rate limit exceeded for URL: %s", url)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), cfg.TranscribeTimeout)
	defer cancel()

	text, err := handleTranscription(ctx, url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("ERROR: Transcription failed for URL %s: %v", url, err)
		return
	}

	sendJSONResponse(w, text)
	log.Printf("INFO: Transcription successful for URL: %s", url)
}

func handleTranscription(ctx context.Context, url string) (string, error) {
	lock := getTranscriptionLock(url)
	lock.mu.Lock()
	defer lock.mu.Unlock()

	text, status, err := db.GetTranscription(ctx, url)
	if err != nil {
		log.Printf("ERROR: Failed to get transcription from DB for URL %s: %v", url, err)
		return "", err
	}

	if status == "completed" {
		log.Printf("INFO: Transcription for URL %s found in database.", url)
		return text, nil
	}

	err = db.SetTranscriptionStatus(ctx, url, "in_progress")
	if err != nil {
		log.Printf("ERROR: Failed to set transcription status to in_progress for URL %s: %v", url, err)
		return "", fmt.Errorf("error setting transcription status: %v", err)
	}

	text, err = runTranscriptionScript(ctx, url)
	if err != nil {
		db.SetTranscriptionStatus(ctx, url, "failed")
		log.Printf("ERROR: Transcription script failed for URL %s: %v", url, err)
		return "", err
	}

	err = db.SetTranscription(ctx, url, text)
	if err != nil {
		log.Printf("ERROR: Failed to save transcription for URL %s: %v", url, err)
		return "", fmt.Errorf("error saving transcription: %v", err)
	}

	db.SetTranscriptionStatus(ctx, url, "completed")
	log.Printf("INFO: Transcription for URL %s saved successfully.", url)
	return text, nil
}

func sendJSONResponse(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"transcription": text})
	log.Printf("INFO: JSON response sent successfully.")
}

func runTranscriptionScript(ctx context.Context, url string) (string, error) {
	log.Printf("INFO: Starting transcription for URL: %s", url)

	const (
		maxRetries    = 3
		initialBackoff = 2 * time.Second
		maxBackoff    = 30 * time.Second
		backoffFactor = 2.0
	)

	var (
		output []byte
		err    error
	)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		cmd := exec.CommandContext(ctx, "uv", "run", "transcribe.py", url)
		output, err = cmd.CombinedOutput()
		if err == nil {
			break
		}

		log.Printf("ERROR: Transcription script failed (attempt %d/%d) for URL %s: %v, output: %s", attempt, maxRetries, url, err, output)

		backoff := time.Duration(float64(initialBackoff) * math.Pow(backoffFactor, float64(attempt-1)))
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		select {
		case <-time.After(backoff + time.Duration(rand.Int63n(int64(backoff/2)))):
			// Continue to the next retry attempt
		case <-ctx.Done():
			log.Printf("ERROR: Context cancelled during transcription for URL %s: %v", url, ctx.Err())
			return "", ctx.Err()
		}
	}

	if err != nil {
		log.Printf("ERROR: Transcription failed after %d attempts for URL %s: %v, output: %s", maxRetries, url, err, output)
		return "", fmt.Errorf("error transcribing after %d attempts: %v, output: %s", maxRetries, err, output)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	filename := lines[len(lines)-1]
	defer func() {
		if err := os.Remove(filename); err != nil {
			log.Printf("ERROR: Failed to remove file %s: %v", filename, err)
		}
	}()

	fileContent, err := os.ReadFile(filename)
	if err != nil {
		log.Printf("ERROR: Failed to read file %s: %v", filename, err)
		return "", fmt.Errorf("error reading file: %v", err)
	}
	text := string(fileContent)
	if text == "" {
		log.Printf("ERROR: Transcription resulted in empty text for URL: %s", url)
		return "", fmt.Errorf("error transcribing")
	}

	log.Printf("INFO: Transcription for URL %s completed successfully.", url)
	return text, nil
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	log.Printf("INFO: Serving index.html for request: %s %s", r.Method, r.URL.Path)
	http.ServeFile(w, r, "./static/index.html")
}

func main() {
	cfg = config.LoadConfig()

	if err := validateConfig(cfg); err != nil {
		log.Fatalf("FATAL: Invalid configuration: %v", err)
	}

	err := db.InitializeDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.DB.Close(); err != nil {
			log.Printf("ERROR: Failed to close database: %v", err)
		}
	}()

	// Initialize rate limiter
	rateLimiter = rate.NewLimiter(rate.Every(1*time.Second), 5) // 5 requests per second

	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/transcribe", transcribeHandler)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		log.Printf("INFO: Listening on port %s", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("FATAL: Could not listen on :%s: %v", cfg.ServerPort, err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	log.Println("INFO: Shutting down the server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("FATAL: Server Shutdown: %v", err)
	}
}

func validateConfig(cfg *config.Config) error {
	if cfg.ServerPort == "" {
		return fmt.Errorf("server port is required")
	}
	if cfg.DBPath == "" {
		return fmt.Errorf("database path is required")
	}
	if cfg.TranscribeTimeout <= 0 {
		return fmt.Errorf("transcribe timeout must be greater than 0")
	}
	if cfg.ReadTimeout <= 0 {
		return fmt.Errorf("read timeout must be greater than 0")
	}
	if cfg.WriteTimeout <= 0 {
		return fmt.Errorf("write timeout must be greater than 0")
	}
	if cfg.IdleTimeout <= 0 {
		return fmt.Errorf("idle timeout must be greater than 0")
	}
	return nil
}