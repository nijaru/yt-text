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
	cfg                *config.Config
	transcriptionLocks sync.Map
	rateLimiter        *rate.Limiter
)

type transcriptionLock struct {
	mu sync.Mutex
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := newLoggingResponseWriter(w)
		next.ServeHTTP(lrw, r)
		duration := time.Since(start)

		log.Printf("INFO: %s %s %d %s", r.Method, r.URL.Path, lrw.statusCode, duration)
	})
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
		handleError(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	url := r.FormValue("url")

	if err := validateAndRateLimit(w, url); err != nil {
		log.Printf("ERROR: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), cfg.TranscribeTimeout)
	defer cancel()

	text, err := handleTranscription(ctx, url)
	if err != nil {
		handleTranscriptionError(w, url, err)
		return
	}

	if ctx.Err() != nil {
		handleError(w, "Request timed out", http.StatusGatewayTimeout)
		log.Printf("ERROR: Context cancelled before sending response for URL %s: %v", url, ctx.Err())
		return
	}

	log.Printf("INFO: Sending JSON response for URL: %s", url)
	if err := sendJSONResponse(w, text); err != nil {
		log.Printf("ERROR: Failed to send JSON response for URL %s: %v", url, err)
		return
	}
	log.Printf("INFO: Transcription successful for URL: %s", url)
}

func validateAndRateLimit(w http.ResponseWriter, url string) error {
	if err := validateURL(url); err != nil {
		handleError(w, err.Error(), http.StatusBadRequest)
		return fmt.Errorf("URL validation failed: %v", err)
	}

	if !rateLimiter.Allow() {
		handleError(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return fmt.Errorf("Rate limit exceeded for URL: %s", url)
	}

	return nil
}

func handleTranscriptionError(w http.ResponseWriter, url string, err error) {
	if validationErr, ok := err.(*ValidationError); ok {
		handleError(w, "Invalid URL", http.StatusBadRequest)
		log.Printf("ERROR: URL validation failed for URL %s: %v", url, validationErr)
	} else {
		handleError(w, "An error occurred while processing your request. Please try again later.", http.StatusInternalServerError)
		log.Printf("ERROR: Transcription failed for URL %s: %v", url, err)
	}
}

func handleError(w http.ResponseWriter, message string, statusCode int) {
	http.Error(w, message, statusCode)
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

	if err := db.SetTranscriptionStatus(ctx, url, "in_progress"); err != nil {
		log.Printf("ERROR: Failed to set transcription status to in_progress for URL %s: %v", url, err)
		return "", fmt.Errorf("error setting transcription status: %v", err)
	}

	if err := validateURLWithScript(url); err != nil {
		return "", err
	}

	text, err = runTranscriptionScript(ctx, url)
	if err != nil {
		db.SetTranscriptionStatus(ctx, url, "failed")
		log.Printf("ERROR: Transcription script failed for URL %s: %v", url, err)
		return "", err
	}

	if err := saveTranscription(ctx, url, text); err != nil {
		return "", err
	}

	log.Printf("INFO: Transcription for URL %s saved successfully.", url)
	return text, nil
}

func validateURLWithScript(url string) error {
	if err := executeValidationScript(url); err != nil {
		if validationErr, ok := err.(*ValidationError); ok {
			log.Printf("ERROR: URL validation script failed for URL %s: %v", url, validationErr)
			return validationErr
		}
		log.Printf("ERROR: URL validation script failed for URL %s: %v", url, err)
		return fmt.Errorf("error validating URL: %v", err)
	}
	return nil
}

func saveTranscription(ctx context.Context, url, text string) error {
	if err := db.SetTranscription(ctx, url, text); err != nil {
		log.Printf("ERROR: Failed to save transcription for URL %s: %v", url, err)
		return fmt.Errorf("error saving transcription: %v", err)
	}

	if err := db.SetTranscriptionStatus(ctx, url, "completed"); err != nil {
		log.Printf("ERROR: Failed to set transcription status to completed for URL %s: %v", url, err)
		return fmt.Errorf("error setting transcription status: %v", err)
	}

	return nil
}

func sendJSONResponse(w http.ResponseWriter, text string) error {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"transcription": text}
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("ERROR: Failed to encode JSON response: %v", err)
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
		return err
	}
	log.Printf("INFO: JSON response sent successfully")
	return nil
}

func runTranscriptionScript(ctx context.Context, url string) (string, error) {
	log.Printf("INFO: Starting transcription for URL: %s", url)

	output, err := executeTranscriptionWithRetry(ctx, url)
	if err != nil {
		return "", err
	}

	filename, err := extractFilename(output)
	if err != nil {
		return "", err
	}

	if err := validateTranscriptionFile(filename); err != nil {
		return "", err
	}

	defer func() {
		if err := os.Remove(filename); err != nil {
			log.Printf("ERROR: Failed to remove file %s: %v", filename, err)
		}
	}()

	text, err := readTranscriptionFile(filename)
	if err != nil {
		return "", err
	}

	log.Printf("INFO: Transcription for URL %s completed successfully.", url)
	return text, nil
}

func executeTranscriptionWithRetry(ctx context.Context, url string) ([]byte, error) {
	const (
		maxRetries     = 3
		initialBackoff = 2 * time.Second
		maxBackoff     = 30 * time.Second
		backoffFactor  = 2.0
	)

	var (
		output []byte
		err    error
	)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		output, err = executeTranscriptionScript(ctx, url, maxRetries, initialBackoff, maxBackoff, backoffFactor)
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
			return nil, ctx.Err()
		}
	}

	if err != nil {
		log.Printf("ERROR: Transcription failed after %d attempts for URL %s: %v, output: %s", maxRetries, url, err, output)
		return nil, fmt.Errorf("error transcribing after %d attempts: %v, output: %s", maxRetries, err, output)
	}

	return output, nil
}

func readTranscriptionFile(filename string) (string, error) {
	fileContent, err := os.ReadFile(filename)
	if err != nil {
		log.Printf("ERROR: Failed to read file %s: %v", filename, err)
		return "", fmt.Errorf("error reading file: %v", err)
	}
	text := string(fileContent)
	if text == "" {
		log.Printf("ERROR: Transcription resulted in empty text for file: %s", filename)
		return "", fmt.Errorf("error transcribing")
	}

	return formatText(text), nil
}

func formatText(text string) string {
	text = strings.TrimSpace(text)
	var builder strings.Builder
	for _, char := range text {
		builder.WriteRune(char)
		if char == '.' || char == '!' || char == '?' {
			builder.WriteRune('\n')
		}
	}
	return builder.String()
}

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func executeValidationScript(url string) error {
	cmd := exec.Command("uv", "run", "validate.py", url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error executing validation script: %v, output: %s", err, output)
	}

	outputLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(outputLines) == 0 {
		return &ValidationError{Message: "Validation script returned no output"}
	}
	lastLine := outputLines[len(outputLines)-1]
	if lastLine != "True" {
		return &ValidationError{Message: lastLine}
	}
	return nil
}

func executeTranscriptionScript(ctx context.Context, url string, maxRetries int, initialBackoff, maxBackoff time.Duration, backoffFactor float64) ([]byte, error) {
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
			return nil, ctx.Err()
		}
	}

	if err != nil {
		log.Printf("ERROR: Transcription failed after %d attempts for URL %s: %v, output: %s", maxRetries, url, err, output)
		return nil, fmt.Errorf("error transcribing after %d attempts: %v, output: %s", maxRetries, err, output)
	}

	return output, nil
}

func extractFilename(output []byte) (string, error) {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	filename := lines[len(lines)-1]

	if filename == "" {
		log.Printf("ERROR: Transcription script returned an empty filename")
		return "", fmt.Errorf("error: transcription script returned an empty filename")
	}

	return filename, nil
}

func validateTranscriptionFile(filename string) error {
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("ERROR: Transcription file does not exist: %s", filename)
			return fmt.Errorf("error: transcription file does not exist: %s", filename)
		}
		log.Printf("ERROR: Failed to stat file %s: %v", filename, err)
		return fmt.Errorf("error: failed to stat file: %v", err)
	}
	return nil
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	log.Printf("INFO: Serving index.html for request: %s %s", r.Method, r.URL.Path)
	http.ServeFile(w, r, "./static/index.html")
}

func serveStaticFiles(w http.ResponseWriter, r *http.Request) {
	filePath := "." + r.URL.Path
	http.ServeFile(w, r, filePath)
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

	mux := http.NewServeMux()
	mux.HandleFunc("/static/", serveStaticFiles)
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/transcribe", transcribeHandler)

	loggedMux := loggingMiddleware(mux)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      loggedMux,
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