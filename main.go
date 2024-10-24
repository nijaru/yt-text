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
	var err error
	db, err = sql.Open("sqlite3", "./urls.db")
	if err != nil {
		return fmt.Errorf("Error opening database: %v", err)
	}

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

func getTranscription(url string) (string, error) {
	var body string
	err := db.QueryRow("SELECT text FROM urls WHERE url = ?", url).Scan(&body)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("Error querying database: %v", err)
	}

	return body, nil
}

func setTranscription(url, text string) error {
	_, err := db.Exec("INSERT INTO urls (url, text) VALUES (?, ?)", url, text)
	if err != nil {
		return fmt.Errorf("Error inserting into database: %v", err)
	}

	return nil
}

func transcribeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	url := r.FormValue("url")

	if err := validateURL(url); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	text, err := getTranscription(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
	if text != "" {
		sendJSONResponse(w, text)
		return
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
	cmd := exec.Command("python3", "transcribe.py", url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Error transcribing: %v, Output: %s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	filename := lines[len(lines)-1]

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

func main() {
	err := initializeDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/transcribe", transcribeHandler)

	server := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
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