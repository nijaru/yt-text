package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/nijaru/yt-text/errors"
	"github.com/sirupsen/logrus"
)

type contextKey string

const (
	TraceKey  contextKey = "trace"
	LoggerKey contextKey = "logger"
)

type TraceInfo struct {
	RequestID string
	StartTime time.Time
	UserAgent string
	RemoteIP  string
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
	wroteHeader  bool
	err          error
	stack        []byte
	startTime    time.Time
	traceInfo    *TraceInfo
	metrics      struct {
		dbDuration    time.Duration
		cacheDuration time.Duration
	}
}

func NewLoggingResponseWriter(w http.ResponseWriter, traceInfo *TraceInfo) *loggingResponseWriter {
	if w == nil {
		return nil
	}
	return &loggingResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		wroteHeader:    false,
		stack:          debug.Stack(),
		startTime:      time.Now(),
		traceInfo:      traceInfo,
	}
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if lrw == nil {
		err := errors.Internal("Write", http.ErrAbortHandler, "nil response writer")
		return 0, fmt.Errorf("%w: %s", err, string(debug.Stack()))
	}
	if !lrw.wroteHeader {
		lrw.WriteHeader(http.StatusOK)
	}
	size, err := lrw.ResponseWriter.Write(b)
	if err != nil {
		lrw.err = errors.Internal("Write", err, "failed to write response")
		lrw.stack = debug.Stack()
	}
	lrw.responseSize += int64(size)
	return size, err
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	if lrw == nil || lrw.wroteHeader {
		return
	}
	if code >= 400 {
		lrw.err = errors.E("WriteHeader", nil, fmt.Sprintf("HTTP %d", code), code)
		lrw.stack = debug.Stack()
	}
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
	lrw.wroteHeader = true
}

func (lrw *loggingResponseWriter) Flush() {
	if lrw == nil {
		return
	}
	if f, ok := lrw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (lrw *loggingResponseWriter) RecordDBDuration(d time.Duration) {
	lrw.metrics.dbDuration += d
}

func (lrw *loggingResponseWriter) RecordCacheDuration(d time.Duration) {
	lrw.metrics.cacheDuration += d
}

func LoggingMiddleware(next http.Handler) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := errors.Internal("LoggingMiddleware", nil, "Handler not configured")
			logrus.WithError(err).WithField("stack", string(debug.Stack())).Error("Nil handler")
			http.Error(w, err.Error(), err.Code)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil {
			err := errors.Internal("LoggingMiddleware", nil, "Nil request")
			logrus.WithError(err).WithField("stack", string(debug.Stack())).Error("Nil request")
			return
		}

		traceInfo := &TraceInfo{
			RequestID: uuid.New().String(),
			StartTime: time.Now(),
			UserAgent: r.UserAgent(),
			RemoteIP:  r.RemoteAddr,
		}

		// Add trace headers to response
		w.Header().Set("X-Request-ID", traceInfo.RequestID)

		// Create logger with trace info
		logger := logrus.WithFields(logrus.Fields{
			"request_id": traceInfo.RequestID,
			"method":     r.Method,
			"path":       r.URL.Path,
			"remote_ip":  traceInfo.RemoteIP,
			"user_agent": traceInfo.UserAgent,
		})

		// Add trace info and logger to context
		ctx := context.WithValue(r.Context(), TraceKey, traceInfo)
		ctx = context.WithValue(ctx, LoggerKey, logger)
		r = r.WithContext(ctx)

		logger.Info("Request started")

		defer func() {
			if rec := recover(); rec != nil {
				err := errors.Internal("LoggingMiddleware", fmt.Errorf("%v", rec), "Panic recovered")
				logger.WithError(err).WithField("stack", string(debug.Stack())).Error("Panic in handler")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		lrw := NewLoggingResponseWriter(w, traceInfo)
		if lrw == nil {
			err := errors.Internal("LoggingMiddleware", nil, "Invalid response writer")
			logger.WithError(err).WithField("stack", string(debug.Stack())).Error("Invalid response writer")
			http.Error(w, err.Error(), err.Code)
			return
		}

		next.ServeHTTP(lrw, r)

		duration := time.Since(traceInfo.StartTime)

		logFields := logrus.Fields{
			"status":         lrw.statusCode,
			"duration":       duration,
			"size":           lrw.responseSize,
			"db_duration":    lrw.metrics.dbDuration,
			"cache_duration": lrw.metrics.cacheDuration,
		}

		if lrw.err != nil {
			logFields["error"] = lrw.err.Error()
			logFields["error_stack"] = string(lrw.stack)
		}

		logger = logger.WithFields(logFields)

		switch {
		case lrw.statusCode >= 500:
			logger.Error("Request completed with server error")
		case lrw.statusCode >= 400:
			logger.Warn("Request completed with client error")
		default:
			logger.Info("Request completed successfully")
		}

		if ctx.Err() != nil {
			logger.WithError(ctx.Err()).Error("Request timeout")
		}
	})
}

func GetTraceInfo(ctx context.Context) *TraceInfo {
	if trace, ok := ctx.Value(TraceKey).(*TraceInfo); ok {
		return trace
	}
	return nil
}

func GetLogger(ctx context.Context) *logrus.Entry {
	if logger, ok := ctx.Value(LoggerKey).(*logrus.Entry); ok {
		return logger
	}
	return logrus.NewEntry(logrus.StandardLogger())
}
