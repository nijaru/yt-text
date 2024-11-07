package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/db"
	"github.com/nijaru/yt-text/handlers"
	"github.com/nijaru/yt-text/middleware"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

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
		Compress:   true, // disabled by default
	}

	// Create a multi-writer to write to both stdout and the log file
	multiWriter := io.MultiWriter(os.Stdout, fileLogger)

	logrus.SetOutput(multiWriter)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)
}

func main() {
	cfg := config.LoadConfig()

	// Validate configuration
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

	mux := http.NewServeMux()
	mux.HandleFunc("/static/", serveStaticFiles)
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/transcribe", handlers.TranscribeHandler)
	mux.HandleFunc("/summarize", handlers.SummarizeHandler)

	loggedMux := middleware.LoggingMiddleware(mux)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      loggedMux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		logrus.WithField("port", cfg.ServerPort).Info("Listening on port")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("Could not listen on port")
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	logrus.Info("Shutting down the server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logrus.WithError(err).Fatal("Server Shutdown")
	}

	// Close database connection
	if err := db.DB.Close(); err != nil {
		logrus.WithError(err).Error("Failed to close database")
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	logrus.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}).Info("Serving index.html")
	http.ServeFile(w, r, "/app/static/index.html")
}

func serveStaticFiles(w http.ResponseWriter, r *http.Request) {
	filePath := "/app" + r.URL.Path
	http.ServeFile(w, r, filePath)
}
