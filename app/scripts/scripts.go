package scripts

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// Config holds the configuration for the ScriptRunner
type Config struct {
    PythonPath   string
    ScriptsPath  string
    Timeout      time.Duration
    TempDir      string
    DownloadDir  string
    Environment  []string
}

// Progress represents a progress update from a Python script
type Progress struct {
    Percent float64 `json:"percent"`
    Stage   string  `json:"stage"`
    Message string  `json:"message,omitempty"`
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
    Duration  float64          `json:"duration"`
    Segments  []TextSegment    `json:"segments,omitempty"`
    Metadata  TranscriptionMeta `json:"metadata"`
    Error     string           `json:"error,omitempty"`
}

type TextSegment struct {
    Start    float64 `json:"start"`
    End      float64 `json:"end"`
    Text     string  `json:"text"`
    Speaker  string  `json:"speaker,omitempty"`
}

type TranscriptionMeta struct {
    Model       string  `json:"model"`
    Language    string  `json:"language"`
    Confidence  float64 `json:"confidence"`
    ProcessTime float64 `json:"process_time"`
}

// SummaryResult represents the response from summarization
type SummaryResult struct {
    Summary   string       `json:"summary"`
    ModelName string       `json:"model_name"`
    Metadata  SummaryMeta `json:"metadata"`
    Error     string      `json:"error,omitempty"`
}

type SummaryMeta struct {
    OriginalLength int     `json:"original_length"`
    SummaryLength  int     `json:"summary_length"`
    Ratio         float64 `json:"ratio"`
}

type ScriptRunner struct {
    config Config
    logger *logrus.Logger
}

func NewScriptRunner(cfg Config) (*ScriptRunner, error) {
    if err := validateConfig(cfg); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
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

func (r *ScriptRunner) Transcribe(ctx context.Context, url string, opts map[string]string) (TranscriptionResult, chan Progress, error) {
    progress := make(chan Progress)
    result := TranscriptionResult{}

    // Prepare script arguments
    args := map[string]string{
        "url": url,
    }
    for k, v := range opts {
        args[k] = v
    }

    // Run script with progress monitoring
    go func() {
        defer close(progress)
        output, err := r.runScriptWithProgress(ctx, "transcribe.py", args, progress)
        if err != nil {
            r.logger.WithError(err).Error("Transcription failed")
            return
        }

        if err := json.Unmarshal(output, &result); err != nil {
            r.logger.WithError(err).Error("Failed to parse transcription result")
            return
        }
    }()

    return result, progress, nil
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
    cmdArgs := []string{filepath.Join(r.config.ScriptsPath, scriptName)}
    for k, v := range args {
        cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", k), v)
    }
    cmdArgs = append(cmdArgs, "--json")

    cmd := exec.CommandContext(ctx, r.config.PythonPath, cmdArgs...)
    cmd.Env = append(os.Environ(), r.config.Environment...)
    cmd.Dir = r.config.ScriptsPath

    return cmd.Output()
}

func (r *ScriptRunner) runScriptWithProgress(ctx context.Context, scriptName string, args map[string]string, progress chan<- Progress) ([]byte, error) {
    cmdArgs := []string{filepath.Join(r.config.ScriptsPath, scriptName)}
    for k, v := range args {
        cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", k), v)
    }
    cmdArgs = append(cmdArgs, "--json", "--progress")

    cmd := exec.CommandContext(ctx, r.config.PythonPath, cmdArgs...)
    cmd.Env = append(os.Environ(), r.config.Environment...)
    cmd.Dir = r.config.ScriptsPath

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
    }

    stderr, err := cmd.StderrPipe()
    if err != nil {
        return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
    }

    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("failed to start command: %w", err)
    }

    // Monitor progress from stderr
    go func() {
        scanner := bufio.NewScanner(stderr)
        for scanner.Scan() {
            var update Progress
            if err := json.Unmarshal([]byte(scanner.Text()), &update); err == nil {
                progress <- update
            }
        }
    }()

    // Read result from stdout
    output, err := io.ReadAll(stdout)
    if err != nil {
        return nil, fmt.Errorf("failed to read stdout: %w", err)
    }

    if err := cmd.Wait(); err != nil {
        return nil, fmt.Errorf("command failed: %w", err)
    }

    return output, nil
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
