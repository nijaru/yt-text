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
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logrus.WithError(err).Error("Failed to encode JSON response")
		RespondWithError(w, errors.New(http.StatusInternalServerError, "Failed to encode response", err))
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
