package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func initializeDB() error {
	db, err := sql.Open("sqlite3", "./urls.db")
	if err != nil {
		return fmt.Errorf("Error opening database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS urls (
                        id INTEGER PRIMARY KEY AUTOINCREMENT,
                        url TEXT NOT NULL,
                        text TEXT NOT NULL,
    )`)
	if err != nil {
		return fmt.Errorf("Error creating table: %v", err)
	}

	return nil
}

func validateURL(url string) error {
	// check if URL is empty
	if url == "" {
		return fmt.Errorf("Error: URL is required")
	}

	// check for sql injection
	if strings.Contains(url, "'") || strings.Contains(url, ";") {
		return fmt.Errorf("Error: Invalid URL format")
	}

	_, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("Error: Invalid URL format")
	}
	return nil
}

func getTranscription(url string) (string, error) {
	db, err := sql.Open("sqlite3", "./urls.db")
	if err != nil {
		return "", fmt.Errorf("Error opening database: %v", err)
	}
	defer db.Close()

	var body string
	err = db.QueryRow("SELECT text FROM urls WHERE url = ?", url).Scan(&body)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("Error querying database: %v", err)
	}

	return body, nil
}

func transcribeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	url := r.FormValue("url")

	// validate URL
	err := validateURL(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// check for transcription in database
	text, err := getTranscription(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if text != "" {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"transcription": "%s"}`, text)
		return
	}

	// run python script to transcribe
	cmd := exec.Command("python3", "transcribe.py", url)
	output, err := cmd.CombinedOutput() // Use CombinedOutput to capture stderr as well
	if err != nil {
		http.Error(w, fmt.Sprintf("Error transcribing: %v, Output: %s", err, output), http.StatusInternalServerError)
		return
	}

	// get transcription from database
	text, err = getTranscription(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if text != "" {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"transcription": "%s"}`, text)
		return
	}

	http.Error(w, "Error transcribing", http.StatusInternalServerError)
}

func main() {
	err := initializeDB()
	if err != nil {
		fmt.Println(err)
		return
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/transcribe", transcribeHandler)

	http.ListenAndServe(":8080", nil)
}
