package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Config struct {
	// Server settings
	ServerPort   string        `json:"server_port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout"`
	Debug        bool          `json:"debug"`

	// Application paths
	LogDir  string `json:"log_dir"`
	TempDir string `json:"temp_dir"`

	// CORS Configuration
	CORS CORSConfig `json:"cors"`

	// Rate Limiting
	RateLimit RateLimitConfig `json:"rate_limit"`

	// Database settings
	Database DatabaseConfig `json:"database"`

	// Video configurations
	Video VideoConfig `json:"video"`

	// Application version
	Version string `json:"version"`

	// Request and shutdown timeouts
	RequestTimeout  time.Duration `json:"request_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

type DatabaseConfig struct {
	Path               string        `json:"path"`
	MaxConnections     int           `json:"max_connections"`
	MaxIdleConnections int           `json:"max_idle_connections"`
	ConnMaxLifetime    time.Duration `json:"conn_max_lifetime"`
}

type VideoConfig struct {
	ProcessTimeout time.Duration `json:"process_timeout"`
	MaxDuration    time.Duration `json:"max_duration"`
	// MaxFileSize    int64         `json:"max_file_size"`
	DefaultModel string   `json:"default_model"`
	PythonPath   string   `json:"python_path"`
	ScriptsPath  string   `json:"scripts_path"`
	Environment  []string `json:"environment"`
}

type CORSConfig struct {
	Enabled          bool     `json:"enabled"`
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	ExposedHeaders   []string `json:"exposed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           int      `json:"max_age"`
}

type RateLimitConfig struct {
	Enabled           bool `json:"enabled"`
	RequestsPerMinute int  `json:"requests_per_minute"`
	BurstSize         int  `json:"burst_size"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		// Server settings
		ServerPort:   getEnv("SERVER_PORT", "8080"),
		ReadTimeout:  getEnvAsDuration("READ_TIMEOUT", 15*time.Second),
		WriteTimeout: getEnvAsDuration("WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:  getEnvAsDuration("IDLE_TIMEOUT", 60*time.Second),
		Debug:        getEnvAsBool("DEBUG", false),

		// Application paths
		LogDir:  getEnv("LOG_DIR", "/var/log/yt-text"),
		TempDir: getEnv("TEMP_DIR", "/tmp/yt-text"),

		// Application version
		Version: getEnv("VERSION", "1.0.0"),

		// Request and shutdown timeouts
		RequestTimeout:  getEnvAsDuration("REQUEST_TIMEOUT", 60*time.Minute),
		ShutdownTimeout: getEnvAsDuration("SHUTDOWN_TIMEOUT", 30*time.Second),

		// CORS Configuration
		CORS: CORSConfig{
			Enabled:        getEnvAsBool("CORS_ENABLED", true),
			AllowedOrigins: getEnvAsStringSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
			AllowedMethods: getEnvAsStringSlice(
				"CORS_ALLOWED_METHODS",
				[]string{"GET", "POST", "OPTIONS"},
			),
			AllowedHeaders:   getEnvAsStringSlice("CORS_ALLOWED_HEADERS", []string{"Content-Type"}),
			ExposedHeaders:   getEnvAsStringSlice("CORS_EXPOSED_HEADERS", []string{}),
			AllowCredentials: getEnvAsBool("CORS_ALLOW_CREDENTIALS", false),
			MaxAge:           getEnvAsInt("CORS_MAX_AGE", 86400),
		},

		// Rate Limiting
		RateLimit: RateLimitConfig{
			Enabled:           getEnvAsBool("RATE_LIMIT_ENABLED", true),
			RequestsPerMinute: getEnvAsInt("RATE_LIMIT_RPM", 60),
			BurstSize:         getEnvAsInt("RATE_LIMIT_BURST", 10),
		},

		// Database
		Database: DatabaseConfig{
			Path:           getEnv("DB_PATH", "/var/lib/yt-text/data.db"),
			MaxConnections: getEnvAsInt("DB_MAX_CONNECTIONS", 10),
		},

		// Video Service
		Video: VideoConfig{
			ProcessTimeout: getEnvAsDuration("VIDEO_PROCESS_TIMEOUT", 30*time.Minute),
			MaxDuration:    getEnvAsDuration("VIDEO_MAX_DURATION", 4*time.Hour),
			// MaxFileSize:    getEnvAsInt64("VIDEO_MAX_FILE_SIZE", 100*1024*1024), // 100MB
			DefaultModel: getEnv("WHISPER_MODEL", "base.en"),
			PythonPath:   getEnv("PYTHON_PATH", "python3"),
			ScriptsPath:  getEnv("SCRIPTS_PATH", "./scripts"),
		},
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	// Validate paths
	if err := validatePaths(c); err != nil {
		return err
	}

	// Validate timeouts
	if err := validateTimeouts(c); err != nil {
		return err
	}

	return nil
}

func validatePaths(c *Config) error {
	paths := []struct {
		path string
		name string
	}{
		{c.LogDir, "log directory"},
		{c.TempDir, "temp directory"},
		{filepath.Dir(c.Database.Path), "database directory"},
	}

	for _, p := range paths {
		if err := os.MkdirAll(p.path, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", p.name, err)
		}
	}

	return nil
}

func validateTimeouts(c *Config) error {
	if c.ReadTimeout <= 0 {
		return errors.New("read timeout must be positive")
	}
	if c.WriteTimeout <= 0 {
		return errors.New("write timeout must be positive")
	}
	return nil
}

func validateServices(c *Config) error {
	if c.Video.MaxDuration <= 0 {
		return errors.New("max video duration must be positive")
	}
	// if c.Video.MaxFileSize <= 0 {
	// 	return errors.New("max file size must be positive")
	// }
	return nil
}

// Helper functions for reading environment variables
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvAsStringSlice(key string, defaultValue []string) []string {
	if value, exists := os.LookupEnv(key); exists {
		if value = strings.TrimSpace(value); value != "" {
			return strings.Split(value, ",")
		}
	}
	return defaultValue
}
