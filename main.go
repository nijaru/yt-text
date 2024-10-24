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

	// Additional validation to ensure it's a YouTube URL
	if !strings.Contains(parsedURL.Host, "youtube.com") && !strings.Contains(parsedURL.Host, "youtu.be") {
		return fmt.Errorf("error: URL must be a YouTube link")
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

	if !rateLimiter.Allow() {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), cfg.TranscribeTimeout)
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
	lock := getTranscriptionLock(url)
	lock.mu.Lock()
	defer lock.mu.Unlock()

	text, status, err := db.GetTranscription(ctx, url)
	if err != nil {
		return "", err
	}

	if status == "completed" {
		log.Printf("Transcription for URL %s found in database.", url)
		return text, nil
	}

	err = db.SetTranscriptionStatus(ctx, url, "in_progress")
	if err != nil {
		return "", fmt.Errorf("error setting transcription status: %v", err)
	}

	text, err = runTranscriptionScript(ctx, url)
	if err != nil {
		db.SetTranscriptionStatus(ctx, url, "failed")
		return "", err
	}

	err = db.SetTranscription(ctx, url, text)
	if err != nil {
		return "", fmt.Errorf("error saving transcription: %v", err)
	}

	db.SetTranscriptionStatus(ctx, url, "completed")
	return text, nil
}

func sendJSONResponse(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"transcription": text})
}

func runTranscriptionScript(ctx context.Context, url string) (string, error) {
	log.Printf("Transcribing URL: %s", url)

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

		log.Printf("Error running transcription script (attempt %d/%d): %v, output: %s", attempt, maxRetries, err, output)

		backoff := time.Duration(float64(initialBackoff) * math.Pow(backoffFactor, float64(attempt-1)))
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		select {
		case <-time.After(backoff + time.Duration(rand.Int63n(int64(backoff/2)))):
			// Continue to the next retry attempt
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	if err != nil {
		return "", fmt.Errorf("error transcribing after %d attempts: %v, output: %s", maxRetries, err, output)
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
		log.Printf("Error reading file: %v", err)
		return "", fmt.Errorf("error reading file: %v", err)
	}
	text := string(fileContent)
	if text == "" {
		log.Printf("Transcription resulted in empty text for URL: %s", url)
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
	cfg = config.LoadConfig()

	err := db.InitializeDB(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.DB.Close()

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
		log.Printf("Listening on port %s", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on :%s: %v\n", cfg.ServerPort, err)
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