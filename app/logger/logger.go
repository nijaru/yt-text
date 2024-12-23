package logger

import (
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger struct {
	zerolog.Logger
}

func NewLogger(logDir string) (*Logger, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return nil, err
	}

	// Set up log rotation
	logFile := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "app.log"),
		MaxSize:    10, // MB
		MaxBackups: 3,
		MaxAge:     28, // days
		Compress:   true,
	}

	// Create multi-writer for console and file
	multiWriter := zerolog.MultiLevelWriter(
		zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		},
		logFile,
	)

	// Configure zerolog
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	logger := zerolog.New(multiWriter).
		With().
		Timestamp().
		Caller().
		Logger()

	return &Logger{logger}, nil
}

// Middleware creates a Fiber-compatible logging middleware
func (l *Logger) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		// Log request details
		l.Info().
			Str("request_id", c.Get("X-Request-ID")).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Str("ip", c.IP()).
			Str("user_agent", c.Get("User-Agent")).
			Int("status", c.Response().StatusCode()).
			Dur("latency", time.Since(start)).
			Err(err).
			Msg("Request processed")

		return err
	}
}
