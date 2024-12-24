package video

import "time"

type Config struct {
	// ProcessTimeout is the maximum time allowed for a single transcription
	ProcessTimeout time.Duration `json:"process_timeout"`

	// Max file size and duration limits
	MaxDuration time.Duration `json:"max_duration"`

	// Model configuration
	DefaultModel string `json:"default_model"`
}
