package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/db"
	"github.com/nijaru/yt-text/transcription"
	"github.com/nijaru/yt-text/utils"
	"github.com/nijaru/yt-text/validation"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

var (
	cfg         *config.Config
	rateLimiter *rate.Limiter
	service     *transcription.TranscriptionService
)

func InitHandlers(config *config.Config) {
	cfg = config
	rateLimiter = rate.NewLimiter(rate.Every(cfg.RateLimitInterval), cfg.RateLimit)
	service = transcription.NewTranscriptionService()
}

func TranscribeHandler(w http.ResponseWriter, r *http.Request) {
	logrus.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}).Info("Received request")

	if r.Method != http.MethodPost {
		utils.HandleError(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	url := r.FormValue("url")

	if err := validateAndRateLimit(w, url); err != nil {
		logrus.WithError(err).Error("Validation or rate limit error")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), cfg.TranscribeTimeout)
	defer cancel()

	text, modelName, err := service.HandleTranscription(ctx, url, cfg)
	if err != nil {
		handleTranscriptionError(w, url, err)
		return
	}

	if ctx.Err() != nil {
		utils.HandleError(w, "Request timed out", http.StatusGatewayTimeout)
		logrus.WithError(ctx.Err()).Error("Context cancelled before sending response")
		return
	}

	logrus.WithField("url", url).Info("Sending JSON response")
	if err := sendJSONResponse(w, text, modelName); err != nil {
		logrus.WithError(err).Error("Failed to send JSON response")
		return
	}
	logrus.WithField("url", url).Info("Transcription successful")
}

func validateAndRateLimit(w http.ResponseWriter, url string) error {
	if err := validation.ValidateURL(url); err != nil {
		utils.HandleError(w, err.Error(), http.StatusBadRequest)
		return fmt.Errorf("URL validation failed: %v", err)
	}

	if !rateLimiter.Allow() {
		utils.HandleError(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return fmt.Errorf("Rate limit exceeded for URL: %s", url)
	}

	return nil
}

func handleTranscriptionError(w http.ResponseWriter, url string, err error) {
	if validationErr, ok := err.(*validation.ValidationError); ok {
		utils.HandleError(w, "Invalid URL", http.StatusBadRequest)
		logrus.WithError(validationErr).WithField("url", url).Error("URL validation failed")
	} else {
		utils.HandleError(w, "An error occurred while processing your request. Please try again later.", http.StatusInternalServerError)
		logrus.WithError(err).WithField("url", url).Error("Transcription failed")
	}
}

func SummarizeHandler(w http.ResponseWriter, r *http.Request) {
	logrus.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}).Info("Received request")

	if r.Method != http.MethodPost {
		utils.HandleError(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	url := r.FormValue("url")

	if err := validation.ValidateURL(url); err != nil {
		utils.HandleError(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), cfg.TranscribeTimeout)
	defer cancel()

	text, status, err := db.GetTranscription(ctx, url)
	if err != nil {
		utils.HandleError(w, "Failed to get transcription from DB", http.StatusInternalServerError)
		logrus.WithError(err).Error("Failed to get transcription from DB")
		return
	}

	if status != "completed" {
		utils.HandleError(w, "Transcription not completed", http.StatusBadRequest)
		logrus.WithField("url", url).Error("Transcription not completed")
		return
	}

	// Check if summary already exists in the database
	summary, summaryModelName, err := db.GetSummary(ctx, url)
	if err != nil {
		utils.HandleError(w, "Failed to get summary from DB", http.StatusInternalServerError)
		logrus.WithError(err).Error("Failed to get summary from DB")
		return
	}

	if summary != "" && summaryModelName == cfg.ModelName {
		logrus.WithField("url", url).Info("Summary found in database with the same model name")
		if err := sendJSONResponse(w, summary, summaryModelName); err != nil {
			logrus.WithError(err).Error("Failed to send JSON response")
		}
		return
	}

	// Generate a new summary if it doesn't exist or the model name has changed
	summary, summaryModelName, err = service.SummaryFunc(ctx, text)
	if err != nil {
		utils.HandleError(w, "Failed to generate summary", http.StatusInternalServerError)
		logrus.WithError(err).Error("Failed to generate summary")
		return
	}

	if ctx.Err() != nil {
		utils.HandleError(w, "Request timed out", http.StatusGatewayTimeout)
		logrus.WithError(ctx.Err()).Error("Context cancelled before sending response")
		return
	}

	// Save the summary and summary model name in the database
	logrus.WithFields(logrus.Fields{
		"url":              url,
		"summary":          summary,
		"summaryModelName": summaryModelName,
	}).Info("Saving summary to DB")

	if err := db.SetSummary(ctx, url, summary, summaryModelName); err != nil {
		utils.HandleError(w, "Failed to save summary to DB", http.StatusInternalServerError)
		logrus.WithError(err).Error("Failed to save summary to DB")
		return
	}

	logrus.WithField("url", url).Info("Sending JSON response")
	if err := sendJSONResponse(w, summary, summaryModelName); err != nil {
		logrus.WithError(err).Error("Failed to send JSON response")
		return
	}
	logrus.WithField("url", url).Info("Summary generation successful")
}

func generateSummary(ctx context.Context, text string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "python3", "summarize.py", text)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error":  err,
			"output": string(output),
		}).Error("Error executing summarization script")
		return "", "", fmt.Errorf("error executing summarization script: %v, output: %s", err, output)
	}

	var result struct {
		Summary   string `json:"summary"`
		Error     string `json:"error"`
		ModelName string `json:"model_name"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		logrus.WithFields(logrus.Fields{
			"error":  err,
			"output": string(output),
		}).Error("Error parsing JSON output")
		return "", "", fmt.Errorf("error parsing JSON output: %v, output: %s", err, output)
	}

	if result.Error != "" {
		logrus.WithField("error", result.Error).Error("Summarization error")
		return "", "", fmt.Errorf("summarization error: %s", result.Error)
	}

	return result.Summary, result.ModelName, nil
}

func sendJSONResponse(w http.ResponseWriter, text, modelName string) error {
	w.Header().Set("Content-Type", "application/json")
	response := struct {
		Transcription string `json:"transcription"`
		ModelName     string `json:"model_name"`
	}{
		Transcription: text,
		ModelName:     modelName,
	}
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		logrus.WithError(err).Error("Failed to encode JSON response")
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
		return err
	}
	logrus.Info("JSON response sent successfully")
	return nil
}
