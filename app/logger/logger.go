package logger

import (
	"io"
	"os"
	"path/filepath"

	fiberLogger "github.com/gofiber/fiber/v2/middleware/logger"
	"gopkg.in/natefinch/lumberjack.v2"
)

func NewLogger(logDir string) (*fiberLogger.Config, error) {
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return nil, err
	}

	logFile := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "app.log"),
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)

	config := &fiberLogger.Config{
		Output:     multiWriter,
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path} | ${error}\n",
		TimeFormat: "2006-01-02 15:04:05",
		TimeZone:   "Local",
	}

	return config, nil
}
