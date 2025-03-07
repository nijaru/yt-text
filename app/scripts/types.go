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
	YouTubeAPIKey string      // YouTube Data API v3 key for caption fetching
}

// GetDefaultModel returns the default model from the configuration or a fallback value.
// If no model is specified in the configuration, it returns "large-v3-turbo".
func (cfg *Config) GetDefaultModel() string {
	if cfg.Model != "" {
		return cfg.Model
	}
	return "large-v3-turbo"
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
	Text                string  `json:"text"`                      // The transcribed text
	ModelName           string  `json:"model_name"`                // Name of the Whisper model used
	Duration            float64 `json:"duration"`                  // Time taken to transcribe in seconds
	Error               string  `json:"error,omitempty"`           // Error message if transcription failed
	Title               *string `json:"title,omitempty"`           // Title of the video if available
	URL                 *string `json:"url,omitempty"`             // Original URL that was transcribed
	Language            *string `json:"language,omitempty"`        // Detected language code
	LanguageProbability float64 `json:"language_probability,omitempty"` // Confidence of language detection
	Source              string  `json:"source,omitempty"`          // Source of transcription (whisper or youtube_api)
}

// YouTubeCaptionResult represents the captions fetched from YouTube API
type YouTubeCaptionResult struct {
	Transcription string  `json:"transcription"`    // The caption text
	Title         string  `json:"title,omitempty"`  // Title of the video
	Language      string  `json:"language,omitempty"` // Language of the captions
	Error         string  `json:"error,omitempty"`  // Error message if fetching failed
}
