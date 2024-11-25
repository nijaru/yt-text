package video

import (
	"context"
	"time"

	"github.com/nijaru/yt-text/models"
	"github.com/nijaru/yt-text/repository"
)

type Repository = repository.VideoRepository // This is correct

type Service interface {
	// Create initiates a new video transcription or returns existing
	Create(ctx context.Context, url string) (*models.Video, error)

	// Process handles the video transcription
	Process(ctx context.Context, id string) error

	// Get retrieves a video by ID
	Get(ctx context.Context, id string) (*models.Video, error)

	// GetByURL retrieves a video by URL
	GetByURL(ctx context.Context, url string) (*models.Video, error)

	// Cancel stops an ongoing transcription
	Cancel(ctx context.Context, id string) error

	// Shutdown gracefully shuts down the service
	Shutdown(ctx context.Context) error
}

type Config struct {
	// ProcessTimeout is the maximum time allowed for a single transcription
	ProcessTimeout time.Duration `json:"process_timeout"`

	// CleanupInterval is how often to check for and clean up stale jobs
	CleanupInterval time.Duration `json:"cleanup_interval"`

	// MaxRetries is the maximum number of retry attempts for failed jobs
	MaxRetries int `json:"max_retries"`

	// RetryDelay is the time to wait between retry attempts
	RetryDelay time.Duration `json:"retry_delay"`

	// DefaultModel is the default model to use for transcription
	DefaultModel string `json:"default_model"`
}
