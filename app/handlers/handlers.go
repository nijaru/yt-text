package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/db"
	"github.com/nijaru/yt-text/errors"
	"github.com/nijaru/yt-text/middleware"
	"github.com/nijaru/yt-text/transcription"
	"github.com/nijaru/yt-text/utils"
	"github.com/nijaru/yt-text/validation"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

const (
	methodGET  = "GET"
	methodPOST = "POST"
)

var (
	cfg         *config.Config
	rateLimiter *rate.Limiter
	service     *transcription.TranscriptionService
)

func InitHandlers(config *config.Config) {
	cfg = config
	rateLimiter = rate.NewLimiter(rate.Every(cfg.RateLimitInterval), cfg.RateLimit)
	service = transcription.NewTranscriptionService(cfg)
}

func TranscribeHandler(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())
	traceInfo := middleware.GetTraceInfo(r.Context())
	start := time.Now()

	logger.WithFields(logrus.Fields{
		"method":     r.Method,
		"path":       r.URL.Path,
		"request_id": traceInfo.RequestID,
	}).Info("Received transcription request")

	if r.Method != http.MethodPost {
		logger.WithFields(logrus.Fields{
			"method":     r.Method,
			"request_id": traceInfo.RequestID,
		}).Warn("Invalid HTTP method")
		utils.RespondWithError(w, r, errors.E("TranscribeHandler", nil, "Method not allowed", http.StatusMethodNotAllowed))
		return
	}

	url := r.FormValue("url")
	if err := validateAndRateLimit(r, url); err != nil {
		logger.WithFields(logrus.Fields{
			"url":        url,
			"error":      err,
			"request_id": traceInfo.RequestID,
		}).Warn("Validation or rate limit check failed")
		utils.RespondWithError(w, r, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), cfg.TranscribeTimeout)
	defer cancel()

	// Check if transcription already exists
	dbStart := time.Now()
	existingText, status, err := db.GetTranscription(ctx, url)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"url":        url,
			"error":      err,
			"duration":   time.Since(dbStart),
			"request_id": traceInfo.RequestID,
		}).Error("Database error checking existing transcription")
		utils.RespondWithError(w, r, err)
		return
	}

	// If transcription exists and is completed, return it
	if status == "completed" && existingText != "" {
		logger.WithFields(logrus.Fields{
			"url":        url,
			"status":     status,
			"duration":   time.Since(dbStart),
			"request_id": traceInfo.RequestID,
		}).Info("Using existing transcription")

		modelName, err := db.GetModelName(ctx, url)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"url":        url,
				"error":      err,
				"request_id": traceInfo.RequestID,
			}).Error("Failed to get model name")
			utils.RespondWithError(w, r, err)
			return
		}

		if err := sendJSONResponse(w, existingText, modelName); err != nil {
			logger.WithFields(logrus.Fields{
				"url":        url,
				"error":      err,
				"request_id": traceInfo.RequestID,
			}).Error("Failed to send JSON response")
		}
		return
	}

	// Generate new transcription
	logger.WithFields(logrus.Fields{
		"url":        url,
		"request_id": traceInfo.RequestID,
	}).Info("Generating new transcription")

	text, modelName, err := service.HandleTranscription(ctx, url, cfg)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"url":        url,
			"error":      err,
			"request_id": traceInfo.RequestID,
		}).Error("Transcription generation failed")
		utils.RespondWithError(w, r, errors.Internal("TranscribeHandler", err, "Transcription failed"))
		return
	}

	// Save new transcription
	dbStart = time.Now()
	if err := db.SetTranscription(ctx, url, text, modelName); err != nil {
		logger.WithFields(logrus.Fields{
			"url":        url,
			"error":      err,
			"duration":   time.Since(dbStart),
			"request_id": traceInfo.RequestID,
		}).Error("Failed to save transcription to database")
		utils.RespondWithError(w, r, err)
		return
	}

	logger.WithFields(logrus.Fields{
		"url":        url,
		"model_name": modelName,
		"duration":   time.Since(start),
		"request_id": traceInfo.RequestID,
	}).Info("Transcription process completed successfully")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Request-ID", traceInfo.RequestID)

	response := struct {
		Text      string `json:"text"`
		ModelName string `json:"model_name"`
	}{
		Text:      text,
		ModelName: modelName,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err,
			"request_id": traceInfo.RequestID,
		}).Error("Failed to encode response")
		return
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func validateAndRateLimit(r *http.Request, url string) error {
	const op = "handlers.validateAndRateLimit"

	if err := validation.ValidateURL(url); err != nil {
		return errors.InvalidInput(op, err, "Invalid URL format")
	}

	if !rateLimiter.Allow() {
		traceInfo := middleware.GetTraceInfo(r.Context())
		return errors.RateLimitExceeded(fmt.Sprintf("%s: request_id=%s", op, traceInfo.RequestID))
	}

	return nil
}

func SummarizeHandler(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())
	traceInfo := middleware.GetTraceInfo(r.Context())
	start := time.Now()

	logger.WithFields(logrus.Fields{
		"method":     r.Method,
		"path":       r.URL.Path,
		"request_id": traceInfo.RequestID,
	}).Info("Received summarize request")

	if r.Method != http.MethodPost {
		utils.RespondWithError(w, r, errors.E("SummarizeHandler", nil, "Method not allowed", http.StatusMethodNotAllowed))
		return
	}

	url := r.FormValue("url")
	if err := validation.ValidateURL(url); err != nil {
		logger.WithFields(logrus.Fields{
			"url":        url,
			"error":      err,
			"request_id": traceInfo.RequestID,
		}).Warn("Invalid URL format") // WARN for client error
		utils.RespondWithError(w, r, errors.InvalidInput("SummarizeHandler", err, "Invalid URL format"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), cfg.TranscribeTimeout)
	defer cancel()

	// Log DB operations with timing
	dbStart := time.Now()
	text, status, err := db.GetTranscription(ctx, url)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"url":        url,
			"error":      err,
			"duration":   time.Since(dbStart),
			"request_id": traceInfo.RequestID,
		}).Error("Database error fetching transcription")
		utils.RespondWithError(w, r, err)
		return
	}

	logger.WithFields(logrus.Fields{
		"url":        url,
		"status":     status,
		"duration":   time.Since(dbStart),
		"request_id": traceInfo.RequestID,
	}).Debug("Retrieved transcription status") // DEBUG for successful DB operation

	if status != "completed" {
		logger.WithFields(logrus.Fields{
			"url":        url,
			"status":     status,
			"request_id": traceInfo.RequestID,
		}).Info("Transcription not yet complete") // INFO for normal business state
		utils.RespondWithError(w, r, errors.E("SummarizeHandler", nil, "Transcription not completed", http.StatusBadRequest))
		return
	}

	dbStart = time.Now()
	summary, summaryModelName, err := db.GetSummary(ctx, url)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"url":        url,
			"error":      err,
			"duration":   time.Since(dbStart),
			"request_id": traceInfo.RequestID,
		}).Error("Database error fetching summary")
		utils.RespondWithError(w, r, err)
		return
	}

	// Only log if we found an existing summary
	if summary != "" && summaryModelName == cfg.SummaryModelName {
		logger.WithFields(logrus.Fields{
			"url":        url,
			"model_name": summaryModelName,
			"duration":   time.Since(dbStart),
			"request_id": traceInfo.RequestID,
		}).Info("Using existing summary") // INFO for cache hit
		if err := sendJSONResponse(w, summary, summaryModelName); err != nil {
			logger.WithError(err).Error("Failed to send JSON response")
		}
		return
	}

	// Generate new summary
	logger.WithFields(logrus.Fields{
		"url":        url,
		"request_id": traceInfo.RequestID,
	}).Info("Generating new summary") // INFO for significant operation

	summary, summaryModelName, err = service.SummaryFunc(ctx, text)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"url":        url,
			"error":      err,
			"request_id": traceInfo.RequestID,
		}).Error("Failed to generate summary")
		utils.RespondWithError(w, r, err)
		return
	}

	// Save new summary
	dbStart = time.Now()
	if err := db.SetSummary(ctx, url, summary, summaryModelName); err != nil {
		logger.WithFields(logrus.Fields{
			"url":        url,
			"error":      err,
			"duration":   time.Since(dbStart),
			"request_id": traceInfo.RequestID,
		}).Error("Failed to save summary to database")
		utils.RespondWithError(w, r, err)
		return
	}

	logger.WithFields(logrus.Fields{
		"url":        url,
		"model_name": summaryModelName,
		"duration":   time.Since(start),
		"request_id": traceInfo.RequestID,
	}).Info("Summary process completed successfully") // INFO for overall success

	if err := sendJSONResponse(w, summary, summaryModelName); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err,
			"request_id": traceInfo.RequestID,
		}).Error("Failed to send JSON response")
	}
}

func sendJSONResponse(w http.ResponseWriter, text, modelName string) error {
	w.Header().Set("Content-Type", "application/json")
	response := struct {
		Text      string `json:"text"`
		ModelName string `json:"model_name"`
	}{
		Text:      text,
		ModelName: modelName,
	}
	return json.NewEncoder(w).Encode(response)
}

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())
	traceInfo := middleware.GetTraceInfo(r.Context())
	start := time.Now()

	logger.WithFields(logrus.Fields{
		"method":     r.Method,
		"path":       r.URL.Path,
		"request_id": traceInfo.RequestID,
	}).Debug("Health check requested") // DEBUG since health checks are high-volume

	if r.Method != http.MethodGet {
		logger.WithFields(logrus.Fields{
			"method":     r.Method,
			"request_id": traceInfo.RequestID,
		}).Warn("Invalid HTTP method for health check")
		utils.RespondWithError(w, r, errors.E("HealthCheckHandler", nil, "Method not allowed", http.StatusMethodNotAllowed))
		return
	}

	response := struct {
		Status   string        `json:"status"`
		Duration time.Duration `json:"duration"`
	}{
		Status:   "ok",
		Duration: time.Since(start),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", traceInfo.RequestID)
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err,
			"request_id": traceInfo.RequestID,
		}).Error("Failed to encode health check response")
		return
	}

	logger.WithFields(logrus.Fields{
		"duration":   time.Since(start),
		"request_id": traceInfo.RequestID,
	}).Debug("Health check completed")
}
