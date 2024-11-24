package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/handlers/api"
	"github.com/nijaru/yt-text/logger"
	"github.com/nijaru/yt-text/repository"
	"github.com/nijaru/yt-text/repository/sqlite"
	"github.com/nijaru/yt-text/scripts"
	"github.com/nijaru/yt-text/services/summary"
	"github.com/nijaru/yt-text/services/video"
	"github.com/nijaru/yt-text/validation"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("Failed to load configuration: %v", err))
	}

	// Initialize logger
	if err := logger.InitLogger(cfg.LogDir); err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	log := logrus.StandardLogger()
	log.Info("Starting application")

	// Initialize components
	if err := setupApplication(cfg); err != nil {
		log.WithError(err).Fatal("Failed to setup application")
	}
}

func setupApplication(cfg *config.Config) error {
	log := logrus.StandardLogger()

	// Initialize database with context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	// Pass ctx to initializeDatabase
	db, err := initializeDatabase(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.WithError(err).Error("Failed to close database connection")
		}
	}()

	// Initialize repositories
	videoRepo, err := sqlite.NewRepository(db)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Initialize script runner
	scriptRunner, err := initializeScriptRunner(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize script runner: %w", err)
	}

	// Initialize validator
	validator := validation.NewValidator(cfg)

	// Initialize services
	services, err := initializeServices(cfg, videoRepo, scriptRunner, validator)
	if err != nil {
		return fmt.Errorf("failed to initialize services: %w", err)
	}

	// Initialize and start server
	server := api.NewServer(
		cfg,
		api.WithServices(services.video, services.summary),
		api.WithLogger(log),
	)

	return startServer(server, cfg)
}

func initializeDatabase(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	db, err := sqlite.InitDB(cfg.Database.Path)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.Database.MaxConnections)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConnections)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// Test connection with context
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	return db, nil
}

func initializeScriptRunner(cfg *config.Config) (*scripts.ScriptRunner, error) {
	scriptConfig := scripts.Config{
		PythonPath:  cfg.Scripts.PythonPath,
		ScriptsPath: cfg.Scripts.ScriptsPath,
		Timeout:     cfg.Scripts.Timeout,
		TempDir:     cfg.TempDir,
		DownloadDir: cfg.DownloadDir,
		Environment: cfg.Scripts.Environment,
	}

	return scripts.NewScriptRunner(scriptConfig)
}

type services struct {
	video   video.Service
	summary summary.Service
}

func initializeServices(
	cfg *config.Config,
	videoRepo repository.VideoRepository,
	scriptRunner *scripts.ScriptRunner,
	validator *validation.Validator,
) (*services, error) {
	// Initialize video service
	videoService := video.NewService(
		videoRepo,
		scriptRunner,
		validator,
		video.Config{
			MaxConcurrentJobs: cfg.Video.MaxConcurrentJobs,
			ProcessTimeout:    cfg.Video.ProcessTimeout,
			CleanupInterval:   cfg.Video.CleanupInterval,
			MaxRetries:        cfg.Video.MaxRetries,
			RetryDelay:        cfg.Video.RetryDelay,
		},
	)

	// Initialize summary service
	summaryService := summary.NewService(
		videoRepo,
		scriptRunner,
		validator,
		summary.Config{
			ModelName: cfg.Summary.ModelName,
			MaxLength: cfg.Summary.MaxLength,
			MinLength: cfg.Summary.MinLength,
			BatchSize: cfg.Summary.BatchSize,
			ChunkSize: cfg.Summary.ChunkSize,
		},
	)

	return &services{
		video:   videoService,
		summary: summaryService,
	}, nil
}

func startServer(server *api.Server, cfg *config.Config) error {
	log := logrus.StandardLogger()

	// Create shutdown context
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	// Start server
	go func() {
		log.WithField("port", cfg.ServerPort).Info("Starting server")
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Server failed")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.WithField("signal", sig.String()).Info("Shutdown signal received")

	// Graceful shutdown
	log.Info("Initiating graceful shutdown...")
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Error("Error during shutdown")
		return err
	}

	log.Info("Server stopped gracefully")
	return nil
}
