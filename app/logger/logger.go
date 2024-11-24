package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Common field keys
const (
	FieldRequestID = "request_id"
	FieldURL       = "url"
	FieldMethod    = "method"
	FieldPath      = "path"
	FieldDuration  = "duration"
	FieldStatus    = "status"
	FieldError     = "error"
)

// InitLogger initializes the logger with the given configuration
func InitLogger(logDir string) error {
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return err
	}

	logFile := filepath.Join(logDir, "app.log")
	fileLogger := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   // days
		Compress:   true, // disabled by default
	}

	// Create a multi-writer for both stdout and file
	multiWriter := io.MultiWriter(os.Stdout, fileLogger)

	// Configure logrus
	logrus.SetOutput(multiWriter)
	logrus.SetFormatter(&logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})

	// Set log level based on environment
	if os.Getenv("ENV") == "production" {
		logrus.SetLevel(logrus.InfoLevel)
	} else {
		logrus.SetLevel(logrus.DebugLevel)
	}

	return nil
}

// WithRequestID returns a logrus entry with request ID field
func WithRequestID(requestID string) *logrus.Entry {
	return logrus.WithField(FieldRequestID, requestID)
}

// SanitizeURL removes sensitive information from URLs before logging
func SanitizeURL(url string) string {
	// Implement URL sanitization logic here
	// For example, remove API keys, tokens, or other sensitive data
	return url
}
