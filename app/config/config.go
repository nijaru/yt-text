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

	// Transcription Settings
	TranscriptionTemperature float64
	TranscriptionBeamSize    int
	TranscriptionBestOf      int

	// Whisper Configuration
	WhisperDownloadRoot string
	CudaVisibleDevices  string
	ModelsDir           string
	CacheDir            string
	TempDir             string

	// Resource Management
	MemoryLimit         int64
	GCPercent           int
	MallocTrimThreshold int
	MallocArenaMax      int
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
		ModelName:         GetEnv("MODEL_NAME", "tiny.en"),
		SummaryModelName:  GetEnv("SUMMARY_MODEL_NAME", "facebook/bart-large-cnn"),

		// Spaces Configuration
		SpacesEnabled:  getEnvAsBool("SPACES_ENABLED", false),
		SpacesKey:      GetEnv("SPACES_KEY", ""),
		SpacesSecret:   GetEnv("SPACES_SECRET", ""),
		SpacesRegion:   GetEnv("SPACES_REGION", "nyc3"),
		SpacesEndpoint: GetEnv("SPACES_ENDPOINT", "https://nyc3.digitaloceanspaces.com"),
		SpacesBucket:   GetEnv("SPACES_BUCKET", "yt-text"),

		// Transcription Settings with reasonable defaults
		TranscriptionTemperature: getEnvAsFloat("TRANSCRIPTION_TEMPERATURE", 0.2),
		TranscriptionBeamSize:    getEnvAsInt("TRANSCRIPTION_BEAM_SIZE", 2),
		TranscriptionBestOf:      getEnvAsInt("TRANSCRIPTION_BEST_OF", 1),

		// Whisper Configuration
		WhisperDownloadRoot: GetEnv("WHISPER_DOWNLOAD_ROOT", "/tmp/models"),
		CudaVisibleDevices:  GetEnv("CUDA_VISIBLE_DEVICES", ""),
		ModelsDir:           GetEnv("MODELS_DIR", "/tmp/models"),
		CacheDir:            GetEnv("CACHE_DIR", "/tmp/cache"),
		TempDir:             GetEnv("TEMP_DIR", "/tmp"),

		// Resource Management
		MemoryLimit:         getEnvAsInt64("MEMORY_LIMIT", 1536*1024*1024), // 1.5GB default
		GCPercent:           getEnvAsInt("GOGC", 10),
		MallocTrimThreshold: getEnvAsInt("MALLOC_TRIM_THRESHOLD_", 100000),
		MallocArenaMax:      getEnvAsInt("MALLOC_ARENA_MAX", 1),
	}
}

func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
		logrus.WithFields(logrus.Fields{
			"key":          key,
			"value":        value,
			"defaultValue": defaultValue,
		}).Warn("Invalid boolean, using default")
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

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
		logrus.WithFields(logrus.Fields{
			"key":          key,
			"value":        value,
			"defaultValue": defaultValue,
		}).Warn("Invalid int64, using default")
	}
	return defaultValue
}

func getEnvAsFloat(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
		logrus.WithFields(logrus.Fields{
			"key":          key,
			"value":        value,
			"defaultValue": defaultValue,
		}).Warn("Invalid float, using default")
	}
	return defaultValue
}

func (c *Config) GetTranscriptionEnv() []string {
	return []string{
		"PYTHONUNBUFFERED=1",
		"PYTHONDONTWRITEBYTECODE=1",
		"PYTHONPATH=/app",
		"MALLOC_TRIM_THRESHOLD_=" + strconv.Itoa(c.MallocTrimThreshold),
		"MALLOC_ARENA_MAX=" + strconv.Itoa(c.MallocArenaMax),
		"PYTHONMALLOC=malloc",
		"WHISPER_DOWNLOAD_ROOT=" + c.WhisperDownloadRoot,
		"MODELS_DIR=" + c.ModelsDir,
		"CACHE_DIR=" + c.CacheDir,
		"TEMP_DIR=" + c.TempDir,
		"CUDA_VISIBLE_DEVICES=" + c.CudaVisibleDevices,
	}
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

	// Validate Whisper configuration
	if cfg.WhisperDownloadRoot == "" {
		return errors.New("whisper download root is required")
	}
	if cfg.ModelsDir == "" {
		return errors.New("models directory is required")
	}
	if cfg.CacheDir == "" {
		return errors.New("cache directory is required")
	}
	if cfg.TempDir == "" {
		return errors.New("temp directory is required")
	}

	// Validate resource limits
	if cfg.MemoryLimit <= 0 {
		return errors.New("memory limit must be greater than 0")
	}
	if cfg.MallocTrimThreshold <= 0 {
		return errors.New("malloc trim threshold must be greater than 0")
	}

	return nil
}
