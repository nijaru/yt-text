package scripts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// Config holds the configuration for the ScriptRunner
type Config struct {
	PythonPath  string
	ScriptsPath string
	Timeout     time.Duration
	TempDir     string
	DownloadDir string
	Environment []string
}

// VideoInfo represents the response from video validation
type VideoInfo struct {
	Valid    bool    `json:"valid"`
	Duration float64 `json:"duration"`
	FileSize int64   `json:"file_size"`
	Format   string  `json:"format"`
	Error    string  `json:"error,omitempty"`
}

// TranscriptionResult represents the response from transcription
type TranscriptionResult struct {
	Text      string            `json:"text"`
	ModelName string            `json:"model_name"`
	Duration  float64           `json:"duration"`
	Segments  []TextSegment     `json:"segments,omitempty"`
	Metadata  TranscriptionMeta `json:"metadata"`
	Error     string            `json:"error,omitempty"`
}

type TextSegment struct {
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Text    string  `json:"text"`
	Speaker string  `json:"speaker,omitempty"`
}

type TranscriptionMeta struct {
	Model       string  `json:"model"`
	Language    string  `json:"language"`
	Confidence  float64 `json:"confidence"`
	ProcessTime float64 `json:"process_time"`
}

// SummaryResult represents the response from summarization
type SummaryResult struct {
	Summary   string      `json:"summary"`
	ModelName string      `json:"model_name"`
	Metadata  SummaryMeta `json:"metadata"`
	Error     string      `json:"error,omitempty"`
}

type SummaryMeta struct {
	OriginalLength int     `json:"original_length"`
	SummaryLength  int     `json:"summary_length"`
	Ratio          float64 `json:"ratio"`
}

type ScriptRunner struct {
	config Config
	logger *logrus.Logger
}

func NewScriptRunner(cfg Config) (*ScriptRunner, error) {
	// Verify scripts directory exists
	if _, err := os.Stat(cfg.ScriptsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("scripts directory does not exist: %s", cfg.ScriptsPath)
	}

	// Verify required scripts exist
	requiredScripts := []string{"validate.py", "transcribe.py", "summarize.py"}
	for _, script := range requiredScripts {
		scriptPath := filepath.Join(cfg.ScriptsPath, script)
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("required script not found: %s", scriptPath)
		}
	}

	return &ScriptRunner{
		config: cfg,
		logger: logrus.StandardLogger(),
	}, nil
}

func (r *ScriptRunner) Validate(ctx context.Context, url string) (VideoInfo, error) {
	var result VideoInfo
	output, err := r.runScript(ctx, "validate.py", map[string]string{
		"url": url,
	})
	if err != nil {
		return result, fmt.Errorf("validation failed: %w", err)
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return result, fmt.Errorf("failed to parse validation result: %w", err)
	}

	return result, nil
}

func (r *ScriptRunner) Transcribe(ctx context.Context, url string, opts map[string]string) (TranscriptionResult, error) {
	var result TranscriptionResult

	// Add debug logging for options
	r.logger.WithFields(logrus.Fields{
		"url":  url,
		"opts": opts,
	}).Debug("Starting transcription with options")

	args := map[string]string{
		"url": url,
	}
	// Add model and language options
	for k, v := range opts {
		args[k] = v
		r.logger.WithFields(logrus.Fields{
			"key":   k,
			"value": v,
		}).Debug("Adding transcription option")
	}

	output, err := r.runScript(ctx, "transcribe.py", args)
	if err != nil {
		r.logger.WithError(err).Error("Transcription script execution failed")
		return result, fmt.Errorf("transcription failed: %w", err)
	}

	// Log raw output for debugging
	r.logger.WithField("output", string(output)).Debug("Raw transcription output")

	if err := json.Unmarshal(output, &result); err != nil {
		r.logger.WithError(err).Error("Failed to parse transcription result")
		return result, fmt.Errorf("failed to parse transcription result: %w", err)
	}

	return result, nil
}

func (r *ScriptRunner) Summarize(ctx context.Context, text string, opts map[string]string) (SummaryResult, error) {
	var result SummaryResult

	args := map[string]string{
		"text": text,
	}
	for k, v := range opts {
		args[k] = v
	}

	output, err := r.runScript(ctx, "summarize.py", args)
	if err != nil {
		return result, fmt.Errorf("summarization failed: %w", err)
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return result, fmt.Errorf("failed to parse summary result: %w", err)
	}

	return result, nil
}

func (r *ScriptRunner) runScript(ctx context.Context, scriptName string, args map[string]string) ([]byte, error) {
	scriptPath := filepath.Join(r.config.ScriptsPath, scriptName)
	logger := r.logger.WithFields(logrus.Fields{
		"scriptPath": scriptPath,
		"scriptName": scriptName,
		"args":       args,
	})

	logger.Info("Preparing to execute script")

	// Build command with args
	cmdArgs := []string{"run", scriptPath}

	// Special handling for URL argument in transcribe.py
	if url, hasURL := args["url"]; hasURL && scriptName == "transcribe.py" {
		// Add URL as positional argument
		cmdArgs = append(cmdArgs, url)
		delete(args, "url") // Remove from named args
	}

	// Add remaining arguments as named flags
	for k, v := range args {
		if v != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, v))
		}
	}
	cmdArgs = append(cmdArgs, "--json")

	logger.WithFields(logrus.Fields{
		"command": "uv",
		"args":    cmdArgs,
		"dir":     r.config.ScriptsPath,
		"env":     r.config.Environment,
	}).Debug("Executing command")

	cmd := exec.CommandContext(ctx, "uv", cmdArgs...)
	cmd.Env = append(os.Environ(), r.config.Environment...)
	cmd.Dir = r.config.ScriptsPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		stderrOutput := stderr.String()
		logger.WithFields(logrus.Fields{
			"error":  err,
			"stderr": stderrOutput,
			"stdout": stdout.String(),
		}).Error("Script execution failed")
		return nil, fmt.Errorf("script execution failed: %v (stderr: %s)", err, stderrOutput)
	}

	return stdout.Bytes(), nil
}

func validateConfig(cfg Config) error {
	if cfg.PythonPath == "" {
		return fmt.Errorf("python path is required")
	}
	if cfg.ScriptsPath == "" {
		return fmt.Errorf("scripts path is required")
	}
	if cfg.Timeout == 0 {
		return fmt.Errorf("timeout must be set")
	}

	// Validate paths exist
	paths := []string{cfg.ScriptsPath, cfg.TempDir, cfg.DownloadDir}
	for _, path := range paths {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}

	// Validate required scripts exist
	scripts := []string{"validate.py", "transcribe.py", "summarize.py"}
	for _, script := range scripts {
		path := filepath.Join(cfg.ScriptsPath, script)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("required script not found: %s", script)
		}
	}

	return nil
}
