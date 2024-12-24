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
	"yt-text/services/subtitles"
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

	// Initialize video service
	videoService, db, err := initializeVideoService(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize video service")
	}

	// Close database connection on exit
	defer db.Close()

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

	// Setup routes
	videoHandler := handlers.NewVideoHandler(videoService)

	// API routes
	app.Post("/api/transcribe", videoHandler.Transcribe)
	app.Get("/api/transcribe/:id", videoHandler.GetTranscription)

	// Health check
	app.Get("/health", handlers.HealthCheck)

	// Static files
	app.Static("/static", "/app/static")
	app.Static("/", "/app/static")

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

func initializeVideoService(cfg *config.Config) (video.Service, *sqlite.DB, error) {
	// Initialize database
	db, err := sqlite.NewDB(cfg.Database.Path)
	if err != nil {
		return nil, nil, err
	}

	// Initialize repository
	repo, err := sqlite.NewRepository(db)
	if err != nil {
		return nil, nil, err
	}

	// Initialize script runner
	scriptRunner, err := scripts.NewScriptRunner(scripts.Config{
		PythonPath:  cfg.Video.PythonPath,
		ScriptsPath: cfg.Video.ScriptsPath,
		Timeout:     cfg.Video.ProcessTimeout,
		TempDir:     cfg.TempDir,
	})
	if err != nil {
		return nil, nil, err
	}

	// Initialize validator
	validator := validation.NewValidator(cfg)

	// Initialize subtitle service
	subtitleService := subtitles.NewService(scriptRunner)

	// Create and return video service
	videoService := video.NewService(
		repo,
		scriptRunner,
		validator,
		subtitleService,
		video.Config{
			ProcessTimeout: cfg.Video.ProcessTimeout,
			MaxDuration:    cfg.Video.MaxDuration,
			DefaultModel:   cfg.Video.DefaultModel,
		},
	)

	return videoService, db, nil
}
