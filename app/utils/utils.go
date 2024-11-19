package utils

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nijaru/yt-text/errors"
	"github.com/nijaru/yt-text/middleware"
	"github.com/sirupsen/logrus"
)

func HandleError(w http.ResponseWriter, r *http.Request, message string, statusCode int) {
	RespondWithError(w, r, errors.E("HandleError", nil, message, statusCode))
}

func RespondWithError(w http.ResponseWriter, r *http.Request, err error) {
	logger := middleware.GetLogger(r.Context())
	traceInfo := middleware.GetTraceInfo(r.Context())

	var appErr *errors.AppError
	if e, ok := err.(*errors.AppError); ok {
		appErr = e
	} else {
		appErr = errors.Internal("RespondWithError", err, "Internal server error")
	}

	logger.WithFields(logrus.Fields{
		"status_code": appErr.Code,
		"error":       appErr.Error(),
		"request_id":  traceInfo.RequestID,
	}).Error("Request failed")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", traceInfo.RequestID)
	w.WriteHeader(appErr.Code)

	if err := json.NewEncoder(w).Encode(map[string]string{"error": appErr.Message}); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err,
			"request_id": traceInfo.RequestID,
		}).Error("Failed to encode error response")
	}
}

func RespondWithJSON(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
	logger := middleware.GetLogger(r.Context())
	traceInfo := middleware.GetTraceInfo(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", traceInfo.RequestID)
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(code)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err,
			"request_id": traceInfo.RequestID,
		}).Error("Failed to encode JSON response")
		return
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func FormatText(text string) string {
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
