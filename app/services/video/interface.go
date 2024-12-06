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
}

type Config struct {
	// ProcessTimeout is the maximum time allowed for a single transcription
	ProcessTimeout time.Duration `json:"process_timeout"`

	// Max file size and duration limits
	MaxDuration time.Duration `json:"max_duration"`
	// MaxFileSize int64         `json:"max_file_size"`

	// Model configuration
	DefaultModel string `json:"default_model"`
}
