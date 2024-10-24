package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
)

var (
	cfg               *config.Config
	transcriptionLocks sync.Map
)

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
	if _, loaded := transcriptionLocks.LoadOrStore(url, struct{}{}); loaded {
		// Another transcription is already in progress for this URL
		return waitForTranscription(ctx, url)
	}
	defer transcriptionLocks.Delete(url) // Ensure the lock is released after transcription

	// Proceed with transcription logic
	text, status, err := db.GetTranscription(ctx, url)
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
		if err := db.SetTranscriptionStatus(ctx, url, "in_progress"); err != nil {
			return "", fmt.Errorf("error setting transcription status: %v", err)
		}
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

func waitForTranscription(ctx context.Context, url string) (string, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			text, status, err := db.GetTranscription(ctx, url)
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
		log.Printf("Error running transcription script: %v, output: %s", err, output)
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