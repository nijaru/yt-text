package utils

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nijaru/yt-text/errors"
	"github.com/sirupsen/logrus"
)

func HandleError(w http.ResponseWriter, message string, statusCode int) {
	RespondWithError(w, errors.New(statusCode, message, nil))
}

func RespondWithError(w http.ResponseWriter, err error) {
	var appErr *errors.Error
	if e, ok := err.(*errors.Error); ok {
		appErr = e
	} else {
		appErr = errors.New(http.StatusInternalServerError, "Internal server error", err)
	}

	logrus.WithFields(logrus.Fields{
		"status_code": appErr.Code,
		"error":       appErr.Error(),
	}).Error("Request failed")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Code)
	json.NewEncoder(w).Encode(map[string]string{"error": appErr.Message})
}

func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(code)

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(payload); err != nil {
		logrus.WithError(err).Error("Failed to encode JSON response")
		// Can't send error response at this point since headers already sent
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
