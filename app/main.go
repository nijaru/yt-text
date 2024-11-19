package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/db"
	"github.com/nijaru/yt-text/handlers"
	"github.com/nijaru/yt-text/middleware"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"gopkg.in/natefinch/lumberjack.v2"
)

type ClientRateLimiter struct {
	clients map[string]*rate.Limiter
	mu      sync.RWMutex
	r       rate.Limit
	b       int
}

func NewClientRateLimiter(r rate.Limit, b int) *ClientRateLimiter {
	return &ClientRateLimiter{
		clients: make(map[string]*rate.Limiter),
		r:       r,
		b:       b,
	}
}

func (c *ClientRateLimiter) GetLimiter(clientID string) *rate.Limiter {
	c.mu.Lock()
	defer c.mu.Unlock()

	limiter, exists := c.clients[clientID]
	if !exists {
		limiter = rate.NewLimiter(c.r, c.b)
		c.clients[clientID] = limiter
	}

	return limiter
}

func clientRateLimitMiddleware(limiter *ClientRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := r.RemoteAddr // This can be replaced with a more sophisticated client identification method
			logger := middleware.GetLogger(r.Context())
			if limiter.GetLimiter(clientID).Allow() {
				next.ServeHTTP(w, r)
			} else {
				logger.WithField("client_id", clientID).Warn("Rate limit exceeded")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			}
		})
	}
}

func maxBytesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := middleware.GetLogger(r.Context())
		r.Body = http.MaxBytesReader(w, r.Body, 1024*1024) // 1MB limit
		logger.WithField("max_bytes", 1024*1024).Info("Max bytes middleware applied")
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://yt-text.com")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func init() {
	// Ensure the log directory exists
	logDir := config.GetEnv("LOG_DIR", "/app/logs")
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		logrus.Fatalf("Failed to create log directory: %v", err)
	}

	// Configure Logrus to write to both stdout and a file with rotation
	logFile := filepath.Join(logDir, "app.log")
	fileLogger := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   // days
		Compress:   true, // enabled compression
	}

	// Create a multi-writer to write to both stdout and the log file
	multiWriter := io.MultiWriter(os.Stdout, fileLogger)

	logrus.SetOutput(multiWriter)
	logrus.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "@timestamp",
			logrus.FieldKeyLevel: "@level",
			logrus.FieldKeyMsg:   "@message",
		},
	})
	logrus.SetLevel(logrus.InfoLevel)

	debug.SetGCPercent(50)                         // More aggressive garbage collection
	debug.SetMemoryLimit(1.5 * 1024 * 1024 * 1024) // 1.5GB memory limit
}
func main() {
	// Load and validate configuration
	cfg := config.LoadConfig()
	if err := config.ValidateConfig(cfg); err != nil {
		logrus.WithError(err).Fatal("Invalid configuration")
	}

	// Initialize database
	if err := db.InitializeDB(cfg.DBPath); err != nil {
		logrus.WithError(err).Fatal("Failed to initialize database")
	}
	defer func() {
		if err := db.DB.Close(); err != nil {
			logrus.WithError(err).Error("Failed to close database")
		}
	}()

	// Initialize handlers
	handlers.InitHandlers(cfg)

	// Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("/static/", serveStaticFiles)
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/transcribe", handlers.TranscribeHandler)
	mux.HandleFunc("/health", handlers.HealthCheckHandler)

	// Create client rate limiter
	clientLimiter := NewClientRateLimiter(rate.Every(cfg.RateLimitInterval), cfg.RateLimit)

	// Chain middleware in the correct order
	handler := middleware.LoggingMiddleware(
		clientRateLimitMiddleware(clientLimiter)(
			corsMiddleware(
				maxBytesMiddleware(mux),
			),
		),
	)

	// Configure server
	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Start server
	go func() {
		logrus.WithField("port", cfg.ServerPort).Info("Starting server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("Server failed to start")
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	logrus.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logrus.WithError(err).Fatal("Server shutdown failed")
	}

	logrus.Info("Server stopped gracefully")
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	logger := middleware.GetLogger(r.Context())
	logger.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}).Info("Serving index.html")
	http.ServeFile(w, r, "/app/static/index.html")
}

func serveStaticFiles(w http.ResponseWriter, r *http.Request) {
	filePath := "/app" + r.URL.Path
	http.ServeFile(w, r, filePath)
}
