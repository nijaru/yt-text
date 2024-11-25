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
	ServerPort      string        `json:"server_port"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`
	IdleTimeout     time.Duration `json:"idle_timeout"`
	RequestTimeout  time.Duration `json:"request_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	Debug           bool          `json:"debug"`
	Version         string        `json:"version"`

	// Application paths
	LogDir      string `json:"log_dir"`
	TempDir     string `json:"temp_dir"`
	DownloadDir string `json:"download_dir"`

	// Logging
	LogLevel string `json:"log_level"`
	LogFile  string `json:"log_file"`

	// CORS Configuration
	CORS CORSConfig `json:"cors"`

	// Rate Limiting
	RateLimit RateLimitConfig `json:"rate_limit"`

	// Monitoring
	Monitoring MonitoringConfig `json:"monitoring"`

	// Database settings
	Database DatabaseConfig `json:"database"`

	// Service configurations
	Video   VideoConfig   `json:"video"`
	Summary SummaryConfig `json:"summary"`
	Scripts ScriptsConfig `json:"scripts"`
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

type MonitoringConfig struct {
	MetricsEnabled bool   `json:"metrics_enabled"`
	MetricsPath    string `json:"metrics_path"`
	TracingEnabled bool   `json:"tracing_enabled"`
	TracingType    string `json:"tracing_type"` // e.g., "jaeger", "datadog"
}

type DatabaseConfig struct {
	Path               string        `json:"path"`
	MaxConnections     int           `json:"max_connections"`
	MaxIdleConnections int           `json:"max_idle_connections"`
	ConnMaxLifetime    time.Duration `json:"conn_max_lifetime"`
}

type VideoConfig struct {
	MaxConcurrentJobs int           `json:"max_concurrent_jobs"`
	ProcessTimeout    time.Duration `json:"process_timeout"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
	MaxDuration       time.Duration `json:"max_duration"`
	MaxFileSize       int64         `json:"max_file_size"`
	AllowedFormats    []string      `json:"allowed_formats"`
	DefaultModel      string        `json:"default_model"`
}

type SummaryConfig struct {
	ModelName string `json:"model_name"`
	MaxLength int    `json:"max_length"`
	MinLength int    `json:"min_length"`
	BatchSize int    `json:"batch_size"`
	ChunkSize int    `json:"chunk_size"`
}

type ScriptsConfig struct {
	PythonPath  string        `json:"python_path"`
	ScriptsPath string        `json:"scripts_path"`
	Timeout     time.Duration `json:"timeout"`
	MaxRetries  int           `json:"max_retries"`
	Environment []string      `json:"environment"`
	Models      ModelsConfig  `json:"models"`
}

type ModelsConfig struct {
	TranscriptionModel string `json:"transcription_model"`
	SummaryModel       string `json:"summary_model"`
	ModelPath          string `json:"model_path"`
	CacheDir           string `json:"cache_dir"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		// Server settings
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		ReadTimeout:     getEnvAsDuration("READ_TIMEOUT", 15*time.Second),
		WriteTimeout:    getEnvAsDuration("WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:     getEnvAsDuration("IDLE_TIMEOUT", 60*time.Second),
		RequestTimeout:  getEnvAsDuration("REQUEST_TIMEOUT", 30*time.Second),
		ShutdownTimeout: getEnvAsDuration("SHUTDOWN_TIMEOUT", 30*time.Second),
		Debug:           getEnvAsBool("DEBUG", false),
		Version:         getEnv("VERSION", "1.0.0"),

		// Application paths
		LogDir:      getEnv("LOG_DIR", "/var/log/yt-text"),
		TempDir:     getEnv("TEMP_DIR", "/tmp/yt-text"),
		DownloadDir: getEnv("DOWNLOAD_DIR", "/tmp/yt-text/downloads"),

		// Logging
		LogLevel: getEnv("LOG_LEVEL", "info"),
		LogFile:  getEnv("LOG_FILE", ""),

		// CORS Configuration
		CORS: CORSConfig{
			Enabled:          getEnvAsBool("CORS_ENABLED", true),
			AllowedOrigins:   getEnvAsStringSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
			AllowedMethods:   getEnvAsStringSlice("CORS_ALLOWED_METHODS", []string{"GET", "POST", "OPTIONS"}),
			AllowedHeaders:   getEnvAsStringSlice("CORS_ALLOWED_HEADERS", []string{"Content-Type", "Authorization"}),
			ExposedHeaders:   getEnvAsStringSlice("CORS_EXPOSED_HEADERS", []string{"X-Request-ID"}),
			AllowCredentials: getEnvAsBool("CORS_ALLOW_CREDENTIALS", false),
			MaxAge:           getEnvAsInt("CORS_MAX_AGE", 86400),
		},

		// Rate Limiting
		RateLimit: RateLimitConfig{
			Enabled:           getEnvAsBool("RATE_LIMIT_ENABLED", true),
			RequestsPerMinute: getEnvAsInt("RATE_LIMIT_RPM", 60),
			BurstSize:         getEnvAsInt("RATE_LIMIT_BURST", 10),
		},

		// Monitoring
		Monitoring: MonitoringConfig{
			MetricsEnabled: getEnvAsBool("METRICS_ENABLED", true),
			MetricsPath:    getEnv("METRICS_PATH", "/metrics"),
			TracingEnabled: getEnvAsBool("TRACING_ENABLED", false),
			TracingType:    getEnv("TRACING_TYPE", "jaeger"),
		},

		// Database
		Database: DatabaseConfig{
			Path:               getEnv("DB_PATH", "/var/lib/yt-text/data.db"),
			MaxConnections:     getEnvAsInt("DB_MAX_CONNECTIONS", 10),
			MaxIdleConnections: getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime:    getEnvAsDuration("DB_CONN_MAX_LIFETIME", time.Hour),
		},

		// Video Service
		Video: VideoConfig{
			MaxConcurrentJobs: getEnvAsInt("VIDEO_MAX_CONCURRENT_JOBS", 5),
			ProcessTimeout:    getEnvAsDuration("VIDEO_PROCESS_TIMEOUT", 30*time.Minute),
			CleanupInterval:   getEnvAsDuration("VIDEO_CLEANUP_INTERVAL", 15*time.Minute),
			MaxRetries:        getEnvAsInt("VIDEO_MAX_RETRIES", 3),
			RetryDelay:        getEnvAsDuration("VIDEO_RETRY_DELAY", 5*time.Second),
			MaxDuration:       getEnvAsDuration("VIDEO_MAX_DURATION", 4*time.Hour),
			MaxFileSize:       getEnvAsInt64("VIDEO_MAX_FILE_SIZE", 100*1024*1024), // 100MB
			AllowedFormats:    getEnvAsStringSlice("VIDEO_ALLOWED_FORMATS", []string{"mp4", "webm"}),
			DefaultModel:      getEnv("WHISPER_MODEL", "base.en"),
		},

		// Summary Service
		Summary: SummaryConfig{
			ModelName: getEnv("SUMMARY_MODEL", "facebook/bart-large-cnn"),
			MaxLength: getEnvAsInt("SUMMARY_MAX_LENGTH", 150),
			MinLength: getEnvAsInt("SUMMARY_MIN_LENGTH", 30),
			BatchSize: getEnvAsInt("SUMMARY_BATCH_SIZE", 512),
			ChunkSize: getEnvAsInt("SUMMARY_CHUNK_SIZE", 1000),
		},

		// Scripts
		Scripts: ScriptsConfig{
			PythonPath:  getEnv("PYTHON_PATH", "python3"),
			ScriptsPath: getEnv("SCRIPTS_PATH", "./scripts"),
			Timeout:     getEnvAsDuration("SCRIPTS_TIMEOUT", 5*time.Minute),
			MaxRetries:  getEnvAsInt("SCRIPTS_MAX_RETRIES", 3),
			Environment: getEnvAsStringSlice("PYTHON_ENV", []string{
				"PYTHONUNBUFFERED=1",
				"PYTHONDONTWRITEBYTECODE=1",
			}),
			Models: ModelsConfig{
				TranscriptionModel: getEnv("TRANSCRIPTION_MODEL", "base.en"),
				SummaryModel:       getEnv("SUMMARY_MODEL", "facebook/bart-large-cnn"),
				ModelPath:          getEnv("MODEL_PATH", "/opt/models"),
				CacheDir:           getEnv("MODEL_CACHE_DIR", "/tmp/model-cache"),
			},
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

	// Validate service configurations
	if err := validateServices(c); err != nil {
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
		{c.DownloadDir, "download directory"},
		{c.Scripts.Models.ModelPath, "model path"},
		{c.Scripts.Models.CacheDir, "model cache directory"},
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
	if c.ShutdownTimeout <= 0 {
		return errors.New("shutdown timeout must be positive")
	}
	return nil
}

func validateServices(c *Config) error {
	if c.Video.MaxConcurrentJobs <= 0 {
		return errors.New("max concurrent jobs must be positive")
	}
	if c.Video.MaxDuration <= 0 {
		return errors.New("max video duration must be positive")
	}
	if c.Video.MaxFileSize <= 0 {
		return errors.New("max file size must be positive")
	}
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
