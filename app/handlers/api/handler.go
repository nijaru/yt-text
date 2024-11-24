package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nijaru/yt-text/errors"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	logger *logrus.Logger
}

// Response represents a standardized API response
type Response struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// MetaResponse includes pagination and other metadata
type MetaResponse struct {
	Response
	Meta struct {
		Total      int `json:"total"`
		Page       int `json:"page"`
		PageSize   int `json:"page_size"`
		TotalPages int `json:"total_pages"`
	} `json:"meta,omitempty"`
}

func respondJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
	response := Response{
		Success:   code >= 200 && code < 300,
		Data:      payload,
		RequestID: r.Context().Value("request_id").(string),
		Timestamp: time.Now().UTC(),
	}

	if !response.Success && payload != nil {
		if err, ok := payload.(string); ok {
			response.Error = err
			response.Data = nil
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		logrus.WithError(err).Error("Failed to encode response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func respondError(w http.ResponseWriter, r *http.Request, err error) {
	code := http.StatusInternalServerError
	msg := "Internal server error"

	if appErr, ok := err.(*errors.AppError); ok {
		code = appErr.Code
		msg = appErr.Message
	}

	logrus.WithFields(logrus.Fields{
		"error":      err,
		"status":     code,
		"request_id": r.Context().Value("request_id"),
		"path":       r.URL.Path,
		"method":     r.Method,
	}).Error("Request error")

	respondJSON(w, r, code, msg)
}

func readJSON(r *http.Request, v interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return errors.InvalidInput("readJSON", err, "Invalid JSON format")
	}
	return nil
}
