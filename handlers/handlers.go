package handlers

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/nijaru/yt-text/config"
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

    text, err := service.HandleTranscription(ctx, url, cfg)
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
    if err := sendJSONResponse(w, text); err != nil {
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

func sendJSONResponse(w http.ResponseWriter, text string) error {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{"transcription": text}
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		logrus.WithError(err).Error("Failed to encode JSON response")
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
		return err
	}
	logrus.Info("JSON response sent successfully")
	return nil
}