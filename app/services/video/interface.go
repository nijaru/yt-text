package video

import (
	"context"
	"time"

	"github.com/nijaru/yt-text/models"
)

type Service interface {
	// Create initiates a new video transcription
	Create(ctx context.Context, url string) (*models.Video, error)

	// Get retrieves a video by ID
	Get(ctx context.Context, id string) (*models.Video, error)

	// GetByURL retrieves a video by URL
	GetByURL(ctx context.Context, url string) (*models.Video, error)

	// Process handles the video transcription
	Process(ctx context.Context, id string) error

	// Cancel stops an ongoing transcription
	Cancel(ctx context.Context, id string) error

	// Shutdown gracefully shuts down the service
	Shutdown(ctx context.Context) error
}

type Repository interface {
	Create(ctx context.Context, video *models.Video) error
	Get(ctx context.Context, id string) (*models.Video, error)
	GetByURL(ctx context.Context, url string) (*models.Video, error)
	Update(ctx context.Context, video *models.Video) error
	Delete(ctx context.Context, id string) error
}

type Config struct {
	MaxConcurrentJobs int           `json:"max_concurrent_jobs"`
	ProcessTimeout    time.Duration `json:"process_timeout"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
}
