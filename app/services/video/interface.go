package video

import (
	"context"
	"time"
	"yt-text/models"
)

type Service interface {
	// Transcribe initiates a new transcription or returns existing one
	Transcribe(ctx context.Context, url string) (*models.Video, error)

	// GetTranscription retrieves a transcription by ID
	GetTranscription(ctx context.Context, id string) (*models.Video, error)
	
	// GetTranscriptionText retrieves the full text of a transcription (from DB or file)
	GetTranscriptionText(ctx context.Context, video *models.Video) (string, error)
	
	// CleanupExpiredTranscriptions removes transcriptions that haven't been accessed recently
	CleanupExpiredTranscriptions(ctx context.Context) error
	
	// CancelJob attempts to cancel a job in progress
	CancelJob(jobID string) bool
	
	// GetJobStatus returns information about a job's status
	GetJobStatus(jobID string) (bool, time.Time)
	
	// GetRepository returns the underlying repository
	GetRepository() Repository
	
	// Close shuts down the service
	Close()
}

type Config struct {
	// ProcessTimeout is the maximum time allowed for a single transcription
	ProcessTimeout time.Duration `json:"process_timeout"`

	// Max file size and duration limits
	MaxDuration time.Duration `json:"max_duration"`

	// Model configuration
	DefaultModel string `json:"default_model"`
	
	// Worker pool configuration
	WorkerPoolSize int `json:"worker_pool_size"`
	
	// Transcription storage configuration
	TranscriptionPath    string `json:"transcription_path"`
	StorageSizeThreshold int64  `json:"storage_size_threshold"`
	CleanupAfterDays     int    `json:"cleanup_after_days"`
	TempDir              string `json:"temp_dir"`
	
	// YouTube API integration
	YouTubeAPIKey string `json:"youtube_api_key"`
}
