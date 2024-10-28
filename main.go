package main

import (
    "context"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    "fmt"

    "github.com/nijaru/yt-text/config"
    "github.com/nijaru/yt-text/db"
    "github.com/nijaru/yt-text/handlers"
    "github.com/nijaru/yt-text/middleware"
    "github.com/sirupsen/logrus"
)

func init() {
    // Initialize the logger to write to stdout
    logrus.SetOutput(os.Stdout)
    logrus.SetFormatter(&logrus.JSONFormatter{})
    logrus.SetLevel(logrus.InfoLevel)
}

func main() {
    cfg := config.LoadConfig()

    // Validate configuration
    if err := validateConfig(cfg); err != nil {
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
}

func validateConfig(cfg *config.Config) error {
    if cfg.ServerPort == "" {
        return fmt.Errorf("server port is required")
    }
    if cfg.DBPath == "" {
        return fmt.Errorf("database path is required")
    }
    if cfg.TranscribeTimeout <= 0 {
        return fmt.Errorf("transcribe timeout must be greater than 0")
    }
    if cfg.ReadTimeout <= 0 {
        return fmt.Errorf("read timeout must be greater than 0")
    }
    if cfg.WriteTimeout <= 0 {
        return fmt.Errorf("write timeout must be greater than 0")
    }
    if cfg.IdleTimeout <= 0 {
        return fmt.Errorf("idle timeout must be greater than 0")
    }
    return nil
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
    logrus.WithFields(logrus.Fields{
        "method": r.Method,
        "path":   r.URL.Path,
    }).Info("Serving index.html")
    http.ServeFile(w, r, "./static/index.html")
}

func serveStaticFiles(w http.ResponseWriter, r *http.Request) {
    filePath := "." + r.URL.Path
    http.ServeFile(w, r, filePath)
}