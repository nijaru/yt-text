package middleware

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type loggingResponseWriter struct {
    http.ResponseWriter
    statusCode int
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
    return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
    lrw.statusCode = code
    lrw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        lrw := NewLoggingResponseWriter(w)
        next.ServeHTTP(lrw, r)
        duration := time.Since(start)

        logrus.WithFields(logrus.Fields{
            "method":     r.Method,
            "path":       r.URL.Path,
            "statusCode": lrw.statusCode,
            "duration":   duration,
        }).Info("Handled request")
    })
}
