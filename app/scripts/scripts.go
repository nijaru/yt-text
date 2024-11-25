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

type Config struct {
	PythonPath  string
	ScriptsPath string
	Timeout     time.Duration
	TempDir     string
}

type VideoInfo struct {
	Valid    bool    `json:"valid"`
	Duration float64 `json:"duration"`
	FileSize int64   `json:"file_size"`
	Format   string  `json:"format"`
	Error    string  `json:"error,omitempty"`
}

type TranscriptionResult struct {
	Text      string  `json:"text"`
	ModelName string  `json:"model_name"`
	Duration  float64 `json:"duration"`
	Error     string  `json:"error,omitempty"`
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
	requiredScripts := []string{"validate.py", "transcribe.py"}
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

	logger := r.logger.WithFields(logrus.Fields{
		"url":  url,
		"opts": opts,
	})
	logger.Debug("Starting transcription")

	args := map[string]string{
		"url": url,
	}
	// Add model options
	for k, v := range opts {
		args[k] = v
	}

	output, err := r.runScript(ctx, "transcribe.py", args)
	if err != nil {
		logger.WithError(err).Error("Transcription script execution failed")
		return result, fmt.Errorf("transcription failed: %w", err)
	}

	if err := json.Unmarshal(output, &result); err != nil {
		logger.WithError(err).Error("Failed to parse transcription result")
		return result, fmt.Errorf("failed to parse transcription result: %w", err)
	}

	return result, nil
}

func (r *ScriptRunner) runScript(ctx context.Context, scriptName string, args map[string]string) ([]byte, error) {
	scriptPath := filepath.Join(r.config.ScriptsPath, scriptName)
	logger := r.logger.WithFields(logrus.Fields{
		"script": scriptName,
		"args":   args,
	})

	logger.Info("Executing script")

	// Build command with args
	cmdArgs := []string{"run", scriptPath}

	// Special handling for URL argument in transcribe.py
	if url, hasURL := args["url"]; hasURL && scriptName == "transcribe.py" {
		cmdArgs = append(cmdArgs, url)
		delete(args, "url")
	}

	// Add remaining arguments as named flags
	for k, v := range args {
		if v != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, v))
		}
	}
	cmdArgs = append(cmdArgs, "--json")

	cmd := exec.CommandContext(ctx, "uv", cmdArgs...)
	cmd.Dir = r.config.ScriptsPath
	cmd.Env = append(os.Environ(),
		"PYTORCH_CUDA_ALLOC_CONF=max_split_size_mb:512",
		"CUDA_LAUNCH_BLOCKING=1",
	)

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