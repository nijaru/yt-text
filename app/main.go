package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"yt-text/config"
	"yt-text/handlers"
	"yt-text/logger"
	"yt-text/repository/sqlite"
	"yt-text/scripts"
	"yt-text/services/video"
	"yt-text/validation"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/fiber/v2/middleware/timeout"
	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Initialize logger
	appLogger, err := logger.NewLogger(cfg.LogDir)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize logger")
	}
	log.Logger = appLogger.Logger // Set global logger

	// Initialize database
	db, err := sqlite.NewDB(cfg.Database.Path)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()

	// Initialize repository
	repo, err := sqlite.NewRepository(db)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize repository")
	}

	// Initialize transcription client (either gRPC or script runner)
	var transcriptionClient scripts.TranscriptionClient
	var clientErr error
	
	if cfg.Video.UseGRPC {
		// Use gRPC client
		log.Info().Str("server", cfg.Video.GRPCServerAddress).Msg("Using gRPC for transcription services")
		transcriptionClient, clientErr = scripts.NewTranscriptionClient(scripts.FactoryConfig{
			UseGRPC: true,
			GRPCConfig: scripts.GRPCConfig{
				ServerAddress: cfg.Video.GRPCServerAddress,
				Timeout:       cfg.Video.ProcessTimeout,
			},
		})
	} else {
		// Use script runner
		log.Info().Msg("Using script runner for transcription services")
		transcriptionClient, clientErr = scripts.NewTranscriptionClient(scripts.FactoryConfig{
			UseGRPC: false,
			ScriptRunnerConfig: scripts.Config{
				PythonPath:  cfg.Video.PythonPath,
				ScriptsPath: cfg.Video.ScriptsPath,
				Timeout:     cfg.Video.ProcessTimeout,
				TempDir:     cfg.TempDir,
				YouTubeAPIKey: cfg.Video.YouTubeAPIKey,
			},
		})
	}
	
	if clientErr != nil {
		log.Fatal().Err(clientErr).Msg("Failed to initialize transcription client")
	}
	defer transcriptionClient.Close()

	// Initialize validator
	validator := validation.NewValidator(cfg)

	// Initialize video service
	videoService := video.NewService(
		repo,
		transcriptionClient,
		validator,
		video.Config{
			ProcessTimeout:       cfg.Video.ProcessTimeout,
			MaxDuration:          cfg.Video.MaxDuration,
			DefaultModel:         cfg.Video.DefaultModel,
			YouTubeAPIKey:        cfg.Video.YouTubeAPIKey,
			TranscriptionPath:    cfg.Video.TranscriptionPath,
			StorageSizeThreshold: cfg.Video.StorageSizeThreshold,
			CleanupAfterDays:     cfg.Video.CleanupAfterDays,
			TempDir:              cfg.TempDir,
		},
	)

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
		ErrorHandler: handlers.ErrorHandler,
		// Optional additional configurations
		DisableStartupMessage: !cfg.Debug,
		StrictRouting:         true,
		CaseSensitive:         true,
		AppName:               "yt-text " + cfg.Version,
	})

	// Setup middleware
	setupMiddleware(app, cfg, appLogger)
	
	// Setup WebSocket
	app.Use("/ws", func(c *fiber.Ctx) error {
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// Setup routes
	videoHandler := handlers.NewVideoHandler(videoService)
	videoHandler.RegisterRoutes(app)

	// Health check
	app.Get("/health", handlers.HealthCheck)

	// Static files
	app.Static("/static", "/app/static")
	app.Static("/", "/app/static")

	// Start scheduled cleanup job for expired transcriptions
	startCleanupJob(videoService, cfg.Video.CleanupAfterDays)
	
	// Graceful shutdown setup
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-shutdownChan
		log.Info().Msg("Shutting down server...")

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := app.ShutdownWithContext(ctx); err != nil {
			log.Error().Err(err).Msg("Server shutdown error")
		}

		// Close any other resources
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("Database shutdown error")
		}
	}()

	// Start server
	serverAddr := ":" + cfg.ServerPort
	if cfg.Debug {
		log.Info().Str("addr", "http://localhost"+serverAddr).Msg("Server starting")
	}

	if err := app.Listen(serverAddr); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Server error")
	}
}

func setupMiddleware(app *fiber.App, cfg *config.Config, logger *logger.Logger) {
	if cfg.Middleware.EnableRecover {
		app.Use(recover.New(recover.Config{
			EnableStackTrace: cfg.Debug,
		}))
	}

	if cfg.Middleware.EnableRequestID {
		app.Use(requestid.New(requestid.Config{
			Header: "X-Request-ID",
			Generator: func() string {
				return uuid.New().String()
			},
		}))
	}

	if cfg.Middleware.EnableLogger {
		app.Use(logger.Middleware())
	}

	if cfg.Middleware.EnableTimeout {
		app.Use(timeout.New(func(c *fiber.Ctx) error {
			return c.Next()
		}, cfg.RequestTimeout))
	}

	if cfg.Middleware.EnableCORS {
		app.Use(cors.New(cors.Config{
			AllowOrigins:     strings.Join(cfg.CORS.AllowedOrigins, ","),
			AllowMethods:     strings.Join(cfg.CORS.AllowedMethods, ","),
			AllowHeaders:     strings.Join(cfg.CORS.AllowedHeaders, ","),
			ExposeHeaders:    strings.Join(cfg.CORS.ExposedHeaders, ","),
			AllowCredentials: cfg.CORS.AllowCredentials,
			MaxAge:           cfg.CORS.MaxAge,
		}))
	}

	if cfg.Middleware.EnableRateLimit && cfg.RateLimit.Enabled {
		app.Use(limiter.New(limiter.Config{
			Max:        cfg.RateLimit.RequestsPerMinute,
			Expiration: time.Minute,
			KeyGenerator: func(c *fiber.Ctx) string {
				return c.IP()
			},
			LimitReached: func(c *fiber.Ctx) error {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"error": "Rate limit exceeded",
				})
			},
		}))
	}

	if cfg.Middleware.EnableCompress {
		app.Use(compress.New(compress.Config{
			Level: compress.LevelDefault,
		}))
	}

	if cfg.Middleware.EnableETag {
		app.Use(etag.New())
	}

	if cfg.Middleware.EnableDebugMode && cfg.Debug {
		app.Use(func(c *fiber.Ctx) error {
			c.Set("X-Debug-Mode", "true")
			return c.Next()
		})
	}
}

func setupRoutes(app *fiber.App, videoService video.Service) {
	// Static files
	app.Static("/", "./static")

	// Create handlers
	videoHandler := handlers.NewVideoHandler(videoService)

	// API routes
	app.Post("/api/transcribe", videoHandler.Transcribe)
	app.Get("/api/transcribe/:id", videoHandler.GetTranscription)

	// Health check
	app.Get("/health", handlers.HealthCheck)
}

func startServer(app *fiber.App, cfg *config.Config) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Info().Msg("Gracefully shutting down...")
		_ = app.Shutdown()
	}()

	if err := app.Listen(":" + cfg.ServerPort); err != nil {
		log.Fatal().Err(err).Msg("Server error")
	}
}

// startCleanupJob starts a periodic cleanup job for expired transcriptions
func startCleanupJob(videoService video.Service, cleanupDays int) {
	// Use reasonable defaults if not configured
	interval := 24 * time.Hour // Run once a day
	if cleanupDays <= 0 {
		cleanupDays = 90 // Default to 90 days if not specified
	}
	
	go func() {
		log.Info().Int("cleanup_days", cleanupDays).Msg("Starting transcription cleanup job")
		
		// Run immediately on startup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		err := videoService.CleanupExpiredTranscriptions(ctx)
		cancel()
		
		if err != nil {
			log.Error().Err(err).Msg("Initial cleanup job failed")
		}
		
		// Then run periodically
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				log.Info().Msg("Running scheduled transcription cleanup")
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				err := videoService.CleanupExpiredTranscriptions(ctx)
				cancel()
				
				if err != nil {
					log.Error().Err(err).Msg("Scheduled cleanup job failed")
				} else {
					log.Info().Msg("Scheduled cleanup completed successfully")
				}
			}
		}
	}()
}
