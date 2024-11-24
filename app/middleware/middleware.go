package middleware

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nijaru/yt-text/config"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

func Chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] != nil {
			handler = middlewares[i](handler)
		}
	}
	return handler
}

// RateLimiter interface for rate limiting middleware
type RateLimiter interface {
	Allow() bool
	Wait(context.Context) error
	Middleware(http.Handler) http.Handler
}

// rateLimiter implements token bucket algorithm
type rateLimiter struct {
	limiter *rate.Limiter
}

func NewRateLimiter(requestsPerMinute int, burst int) RateLimiter {
	return &rateLimiter{
		limiter: rate.NewLimiter(rate.Limit(requestsPerMinute)/60, burst),
	}
}

// Implement RateLimiter interface
func (rl *rateLimiter) Allow() bool {
	return rl.limiter.Allow()
}

func (rl *rateLimiter) Wait(ctx context.Context) error {
	return rl.limiter.Wait(ctx)
}

func (rl *rateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "Rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequestID middleware function (previously undefined)
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}

			ctx := context.WithValue(r.Context(), "request_id", requestID)
			w.Header().Set("X-Request-ID", requestID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Recovery middleware updated to accept logger
func Recovery(logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					stack := debug.Stack()
					logger.WithFields(logrus.Fields{
						"error":      err,
						"stack":      string(stack),
						"request_id": r.Context().Value("request_id"),
					}).Error("Panic recovered")

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error": "Internal server error"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// CORS middleware
func CORS(cfg config.CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.Enabled {
				w.Header().Set("Access-Control-Allow-Origin", strings.Join(cfg.AllowedOrigins, ","))
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ","))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ","))
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ","))

				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				if cfg.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
				}
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Timeout middleware
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			done := make(chan struct{})

			go func() {
				next.ServeHTTP(w, r.WithContext(ctx))
				close(done)
			}()

			select {
			case <-done:
				return
			case <-ctx.Done():
				w.WriteHeader(http.StatusGatewayTimeout)
				w.Write([]byte(`{"error": "Request timeout"}`))
			}
		})
	}
}

// Logging middleware with structured logging
func Logging(logger *logrus.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()

            // Create logging response writer to capture status code and size
            lrw := newLoggingResponseWriter(w)

            // Get request ID from context or header
            requestID := r.Context().Value("request_id")
            if requestID == nil {
                requestID = r.Header.Get("X-Request-ID")
            }

            // Create logger entry with request details
            entry := logger.WithFields(logrus.Fields{
                "request_id": requestID,
                "method":     r.Method,
                "path":       r.URL.Path,
                "remote_ip":  r.RemoteAddr,
                "user_agent": r.UserAgent(),
            })

            // Log request start
            entry.Info("Request started")

            // Process request
            next.ServeHTTP(lrw, r)

            // Calculate duration
            duration := time.Since(start)

            // Log request completion
            entry.WithFields(logrus.Fields{
                "status":     lrw.statusCode,
                "duration":   duration,
                "size":      lrw.size,
            }).Info("Request completed")
        })
    }
}

// Helper type for logging response details
type loggingResponseWriter struct {
    http.ResponseWriter
    statusCode int
    size      int64
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
    // Default status is 200 OK
    return &loggingResponseWriter{
        ResponseWriter: w,
        statusCode:    http.StatusOK,
    }
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
    lrw.statusCode = code
    lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
    size, err := lrw.ResponseWriter.Write(b)
    lrw.size += int64(size)
    return size, err
}

// Support for hijacking (needed for WebSocket/HTTP2)
func (lrw *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
    if hijacker, ok := lrw.ResponseWriter.(http.Hijacker); ok {
        return hijacker.Hijack()
    }
    return nil, nil, fmt.Errorf("hijacking not supported")
}
