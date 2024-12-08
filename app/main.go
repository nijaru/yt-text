package main

import (
	"context"
	"log"
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
	fiberLogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/fiber/v2/middleware/timeout"
	"github.com/google/uuid"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logConfig, err := logger.NewLogger(cfg.LogDir)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Initialize database
	db, err := sqlite.InitDB(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	repo, err := sqlite.NewRepository(db)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %v", err)
	}

	// Initialize script runner
	scriptRunner, err := scripts.NewScriptRunner(scripts.Config{
		PythonPath:  cfg.Video.PythonPath,
		ScriptsPath: cfg.Video.ScriptsPath,
		Timeout:     cfg.Video.ProcessTimeout,
		TempDir:     cfg.TempDir,
	})
	if err != nil {
		log.Fatalf("Failed to initialize script runner: %v", err)
	}

	// Initialize validator
	validator := validation.NewValidator(cfg)

	// Initialize video service
	videoService := video.NewService(
		repo,
		scriptRunner,
		validator,
		video.Config{
			ProcessTimeout: cfg.Video.ProcessTimeout,
			MaxDuration:    cfg.Video.MaxDuration,
			DefaultModel:   cfg.Video.DefaultModel,
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
	setupMiddleware(app, cfg, logConfig)

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
		log.Println("Shutting down server...")

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := app.ShutdownWithContext(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		// Close any other resources
		if err := db.Close(); err != nil {
			log.Printf("Database shutdown error: %v", err)
		}
	}()

	// Start server
	serverAddr := ":" + cfg.ServerPort
	if cfg.Debug {
		log.Printf("Server starting on http://localhost%s", serverAddr)
	}

	if err := app.Listen(serverAddr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func setupMiddleware(app *fiber.App, cfg *config.Config, logConfig *fiberLogger.Config) {
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
		app.Use(fiberLogger.New(*logConfig))
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
	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Gracefully shutting down...")
		_ = app.Shutdown()
	}()

	// Start server
	if err := app.Listen(":" + cfg.ServerPort); err != nil {
		log.Fatal(err)
	}
}

func initializeVideoService(cfg *config.Config) (video.Service, error) {
	// Initialize repository
	db, err := sqlite.InitDB(cfg.Database.Path)
	if err != nil {
		return nil, err
	}

	repo, err := sqlite.NewRepository(db)
	if err != nil {
		return nil, err
	}

	// Initialize script runner
	scriptRunner, err := scripts.NewScriptRunner(scripts.Config{
		PythonPath:  cfg.Video.PythonPath,
		ScriptsPath: cfg.Video.ScriptsPath,
		Timeout:     cfg.Video.ProcessTimeout,
		TempDir:     cfg.TempDir,
	})
	if err != nil {
		return nil, err
	}

	// Initialize validator
	validator := validation.NewValidator(cfg)

	// Create and return video service
	return video.NewService(repo, scriptRunner, validator, video.Config{
		ProcessTimeout: cfg.Video.ProcessTimeout,
		MaxDuration:    cfg.Video.MaxDuration,
		DefaultModel:   cfg.Video.DefaultModel,
	}), nil
}
