package scripts

import (
	"context"
	"fmt"
	"time"
)

// TranscriptionClient defines the interface for transcription services
type TranscriptionClient interface {
	// Validate checks if a video URL is valid and can be processed
	Validate(ctx context.Context, url string) (VideoInfo, error)
	
	// Transcribe transcribes a video URL
	Transcribe(ctx context.Context, url string, opts map[string]string, enableConstraints bool) (*TranscriptionResult, error)
	
	// FetchYouTubeCaptions fetches captions from YouTube API
	FetchYouTubeCaptions(ctx context.Context, videoID string, apiKey string) (*TranscriptionResult, error)
	
	// Close any resources
	Close() error
}

// FactoryConfig holds configuration for creating a transcription client
type FactoryConfig struct {
	// UseGRPC determines whether to use gRPC client or script runner
	UseGRPC bool
	
	// ScriptRunnerConfig is used if UseGRPC is false
	ScriptRunnerConfig Config
	
	// GRPCConfig is used if UseGRPC is true
	GRPCConfig GRPCConfig
}

// NewTranscriptionClient creates a new transcription client based on configuration
func NewTranscriptionClient(config FactoryConfig) (TranscriptionClient, error) {
	if config.UseGRPC {
		return NewGRPCClient(config.GRPCConfig)
	}
	
	return NewScriptRunner(config.ScriptRunnerConfig)
}

// Ensure both implementations satisfy the interface
var _ TranscriptionClient = (*ScriptRunner)(nil)
var _ TranscriptionClient = (*GRPCClient)(nil)