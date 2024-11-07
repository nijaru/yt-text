package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type contextKey string

const RequestIDKey contextKey = "requestID"

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK, 0}
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := lrw.ResponseWriter.Write(b)
	lrw.responseSize += int64(size)
	return size, err
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := uuid.New().String()

		// Add request ID to context
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		r = r.WithContext(ctx)

		// Create request logger with common fields
		logger := logrus.WithFields(logrus.Fields{
			"request_id": requestID,
			"method":     r.Method,
			"path":       r.URL.Path,
			"remote_ip":  r.RemoteAddr,
			"user_agent": r.UserAgent(),
		})

		logger.Info("Request started")

		lrw := NewLoggingResponseWriter(w)
		next.ServeHTTP(lrw, r)

		duration := time.Since(start)

		logger = logger.WithFields(logrus.Fields{
			"status":   lrw.statusCode,
			"duration": duration,
			"size":     lrw.responseSize,
		})

		if lrw.statusCode >= 500 {
			logger.Error("Request completed with server error")
		} else if lrw.statusCode >= 400 {
			logger.Warn("Request completed with client error")
		} else {
			logger.Info("Request completed successfully")
		}
	})
}
