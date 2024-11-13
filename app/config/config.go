package config

import (
	"os"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/sirupsen/logrus"
)

type Config struct {
	DBPath            string
	ServerPort        string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	TranscribeTimeout time.Duration
	RateLimit         int
	RateLimitInterval time.Duration
	ModelName         string
	SummaryModelName  string

	// Spaces Configuration
	SpacesEnabled  bool
	SpacesKey      string
	SpacesSecret   string
	SpacesRegion   string
	SpacesEndpoint string
	SpacesBucket   string
}

func LoadConfig() *Config {
	return &Config{
		DBPath:            GetEnv("DB_PATH", "/tmp/urls.db"),
		ServerPort:        GetEnv("SERVER_PORT", "8080"),
		ReadTimeout:       getEnvAsDuration("READ_TIMEOUT", 120*time.Second),
		WriteTimeout:      getEnvAsDuration("WRITE_TIMEOUT", 300*time.Second),
		IdleTimeout:       getEnvAsDuration("IDLE_TIMEOUT", 120*time.Second),
		TranscribeTimeout: getEnvAsDuration("TRANSCRIBE_TIMEOUT", 30*time.Minute),
		RateLimit:         getEnvAsInt("RATE_LIMIT", 5),
		RateLimitInterval: getEnvAsDuration("RATE_LIMIT_INTERVAL", 5*time.Second),
		ModelName:         GetEnv("MODEL_NAME", "base.en"),
		SummaryModelName:  GetEnv("SUMMARY_MODEL_NAME", "facebook/bart-large-cnn"),

		// Spaces Configuration
		SpacesEnabled:  getEnvAsBool("SPACES_ENABLED", false),
        SpacesKey:     GetEnv("SPACES_KEY", ""),
        SpacesSecret:  GetEnv("SPACES_SECRET", ""),
        SpacesRegion:  GetEnv("SPACES_REGION", "nyc3"),
        SpacesEndpoint: GetEnv("SPACES_ENDPOINT", "https://nyc3.digitaloceanspaces.com"),
        SpacesBucket:  GetEnv("SPACES_BUCKET", "yt-text"),
    }
	}
}

func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
		logrus.WithFields(logrus.Fields{
			"key":          key,
			"value":        value,
			"defaultValue": defaultValue,
		}).Warn("Invalid duration, using default")
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		logrus.WithFields(logrus.Fields{
			"key":          key,
			"value":        value,
			"defaultValue": defaultValue,
		}).Warn("Invalid integer, using default")
	}
	return defaultValue
}

func ValidateConfig(cfg *Config) error {
	if cfg.ServerPort == "" {
		return errors.New("server port is required")
	}
	if cfg.DBPath == "" {
		return errors.New("database path is required")
	}
	if cfg.RateLimit <= 0 {
		return errors.New("rate limit must be greater than 0")
	}
	if cfg.RateLimitInterval <= 0 {
		return errors.New("rate limit interval must be greater than 0")
	}
	if cfg.ModelName == "" {
		return errors.New("model name is required")
	}
	if cfg.TranscribeTimeout <= 0 {
		return errors.New("transcribe timeout must be greater than 0")
	}
	if cfg.ReadTimeout <= 0 {
		return errors.New("read timeout must be greater than 0")
	}
	if cfg.WriteTimeout <= 0 {
		return errors.New("write timeout must be greater than 0")
	}
	if cfg.IdleTimeout <= 0 {
		return errors.New("idle timeout must be greater than 0")
	}
	return nil
}
