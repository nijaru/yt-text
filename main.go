package main

import (
	"database/sql"
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
		return fmt.Errorf("Error opening database: %v", err)
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
		return fmt.Errorf("Error creating table: %v", err)
	}

	return nil
}

func validateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("Error: URL is required")
	}

	rawURL = strings.TrimSpace(rawURL)

	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("Error: Invalid URL format")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("Error: URL must start with http or https")
	}

	return nil
}

func getTranscription(url string) (string, bool, error) {
	var body string
	err := db.QueryRow("SELECT text FROM urls WHERE url = ?", url).Scan(&body)
	if err != nil {
		if err == sql.ErrNoRows {
			if err := setTranscription(url, ""); err != nil {
				return "", false, fmt.Errorf("Error inserting empty transcription: %v", err)
			}
			return "", false, nil
		}
		return "", false, fmt.Errorf("Error querying database: %v", err)
	}

	return body, true, nil
}

func setTranscription(url, text string) error {
	stmt, err := db.Prepare("INSERT INTO urls (url, text) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("Error preparing statement: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(url, text)
	if err != nil {
		return fmt.Errorf("Error executing statement: %v", err)
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

	text, found, err := getTranscription(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
	if text != "" {
		log.Printf("Transcription for URL %s found in database.", url)
		sendJSONResponse(w, text)
		return
	}

	// if transcription already in progress
	if found {
		log.Printf("Transcription for URL %s not found in database. Waiting for it to be processed...", url)
		timeout := time.After(1 * time.Minute)
		tick := time.Tick(5 * time.Second)

		for {
			select {
			case <-timeout:
				log.Printf("Timeout waiting for transcription for URL %s. Proceeding to transcribe...", url)
				break
			case <-tick:
				text, found, err = getTranscription(url)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					log.Println(err)
					return
				}
				if text != "" {
					log.Printf("Transcription for URL %s found in database.", url)
					sendJSONResponse(w, text)
					return
				}
			}
		}
	}
	
	text, err = runTranscriptionScript(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	if err := setTranscription(url, text); err != nil {
		http.Error(w, fmt.Sprintf("Error saving transcription: %v", err), http.StatusInternalServerError)
		log.Println(err)
		return
	}

	sendJSONResponse(w, text)
}

func sendJSONResponse(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"transcription": "%s"}`, text)
}

func runTranscriptionScript(url string) (string, error) {
	log.Printf("Transcribing URL: %s", url)

	cmd := exec.Command("uv", "run", "transcribe.py", url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Error transcribing: %v, Output: %s", err, output)
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
		return "", fmt.Errorf("Error reading file: %v", err)
	}
	text := string(fileContent)
	if text == "" {
		return "", fmt.Errorf("Error transcribing")
	}

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
	if err := server.Close(); err != nil {
		log.Fatalf("Server Close: %v", err)
	}
}
