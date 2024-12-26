package scripts

import (
	"time"
)

// Config holds the configuration for the ScriptRunner.
type Config struct {
	PythonPath  string
	ScriptsPath string
	Timeout     time.Duration
	TempDir     string
	Environment []string
	DefaultModel string
}

// DefaultModel returns the default model from the configuration or a fallback value.
func (cfg *Config) DefaultModel() string {
	if cfg.DefaultModel != "" {
		return cfg.DefaultModel
	}
	return "base.en"
}

// VideoInfo represents the validation result.
type VideoInfo struct {
	Valid    bool    `json:"valid"`
	Duration float64 `json:"duration"`
	Format   string  `json:"format"`
	Error    string  `json:"error,omitempty"`
}

// TranscriptionResult represents the transcription output.
type TranscriptionResult struct {
	Text      string  `json:"text"`
	ModelName string  `json:"model_name"`
	Duration  float64 `json:"duration"`
	Error     string  `json:"error,omitempty"`
	Title     *string `json:"title,omitempty"`
	URL       *string `json:"url,omitempty"`
}

// ResourceStats tracks script execution resources.
type ResourceStats struct {
	MaxMemory int64         // in bytes
	CPUTime   time.Duration // in nanoseconds
}

// ExecutionResult holds the complete script execution result.
type ExecutionResult struct {
	Output    []byte
	ExecTime  time.Duration
	Resources ResourceStats
	ExitCode  int
	Error     error
}

// ScriptErrorType defines the type of script error.
type ScriptErrorType int

const (
	ErrorExecution ScriptErrorType = iota
	ErrorValidation
	ErrorTimeout
	ErrorIO
)

// ScriptError encapsulates errors related to script execution.
type ScriptError struct {
	Type    ScriptErrorType
	Message string
	Err     error
}

// Error implements the error interface for ScriptError.
func (e *ScriptError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}
