package scripts

import (
	"time"
)

// Config holds the configuration for the ScriptRunner
type Config struct {
	PythonPath  string        // Path to Python executable
	ScriptsPath string        // Path to Python scripts directory
	Timeout     time.Duration // Script execution timeout
	TempDir     string        // Temporary directory for downloads
	Environment []string      // Additional environment variables
	Model       string        // Default Whisper model to use
}

// GetDefaultModel returns the default model from the configuration or a fallback value.
// If no model is specified in the configuration, it returns "base.en".
func (cfg *Config) GetDefaultModel() string {
	if cfg.Model != "" {
		return cfg.Model
	}
	return "base.en"
}

// VideoInfo represents the validation result from the Python validation script
type VideoInfo struct {
	Valid    bool    `json:"valid"`           // Whether the video is valid and can be processed
	Duration float64 `json:"duration"`        // Duration of the video in seconds
	Format   string  `json:"format"`          // Format of the video
	Error    string  `json:"error,omitempty"` // Error message if validation failed
	URL      string  `json:"url"`             // Original URL that was validated
}

// TranscriptionResult represents the transcription output from the Python API script
type TranscriptionResult struct {
	Text      string  `json:"text"`            // The transcribed text
	ModelName string  `json:"model_name"`      // Name of the Whisper model used
	Duration  float64 `json:"duration"`        // Time taken to transcribe in seconds
	Error     string  `json:"error,omitempty"` // Error message if transcription failed
	Title     *string `json:"title,omitempty"` // Title of the video if available
	URL       *string `json:"url,omitempty"`   // Original URL that was transcribed
}
