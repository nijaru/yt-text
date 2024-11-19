package transcription

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/smithy-go/rand"
	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/db"
	"github.com/nijaru/yt-text/errors"
	"github.com/nijaru/yt-text/middleware"
	"github.com/nijaru/yt-text/utils"
	"github.com/nijaru/yt-text/validation"
	"github.com/sirupsen/logrus"
)

const (
	maxRetries     = 3
	initialBackoff = 2 * time.Second
	maxBackoff     = 30 * time.Second
	backoffFactor  = 2.0
)

var (
	transcriptionLocks sync.Map
	execCommand        = exec.Command
)

type transcriptionLock struct {
	mu sync.Mutex
}

func getTranscriptionLock(url string) *transcriptionLock {
	lock, _ := transcriptionLocks.LoadOrStore(url, &transcriptionLock{})
	return lock.(*transcriptionLock)
}

type TranscriptionService struct {
	TranscriptionFunc func(ctx context.Context, url string) (string, string, error)
	ExecuteScriptFunc func(ctx context.Context, url string, cfg *config.Config) ([]byte, error)
	ReadFileFunc      func(ctx context.Context, filename string) (string, error)
	SummaryFunc       func(ctx context.Context, text string) (string, string, error)
	config            *config.Config
}

func NewTranscriptionService(cfg *config.Config) *TranscriptionService {
	s := &TranscriptionService{
		config: cfg,
	}
	s.TranscriptionFunc = s.runTranscriptionScript
	s.ExecuteScriptFunc = executeTranscriptionScript
	s.ReadFileFunc = readTranscriptionFile
	s.SummaryFunc = generateSummary
	return s
}

func (s *TranscriptionService) HandleTranscription(ctx context.Context, url string, cfg *config.Config) (string, string, error) {
	const op = "transcription.HandleTranscription"
	logger := middleware.GetLogger(ctx)
	start := time.Now()

	logger.WithFields(logrus.Fields{
		"url":        url,
		"model_name": cfg.ModelName,
	}).Info("Starting transcription process")

	lock := getTranscriptionLock(url)
	lock.mu.Lock()
	defer lock.mu.Unlock()

	var text string
	var modelName string
	var err error

	// Check existing transcription
	text, status, err := db.GetTranscription(ctx, url)
	if err != nil {
		logger.WithError(err).WithField("url", url).Error("Failed to get transcription from database")
		return "", "", errors.Internal(op, err, "Failed to get transcription from database")
	}

	if status == "completed" {
		modelName, err = db.GetModelName(ctx, url)
		if err != nil {
			logger.WithError(err).WithField("url", url).Error("Failed to get model name from database")
			return "", "", errors.Internal(op, err, "Failed to get model name from database")
		}

		if modelName == cfg.ModelName {
			logger.WithFields(logrus.Fields{
				"url":        url,
				"model_name": modelName,
				"duration":   time.Since(start),
			}).Info("Using existing transcription")
			return text, modelName, nil
		}

		logger.WithField("url", url).Info("Model mismatch, initiating new transcription")
	}

	if err := db.SetTranscriptionStatus(ctx, url, "in_progress"); err != nil {
		logger.WithError(err).WithField("url", url).Error("Failed to set transcription status to in_progress")
		return "", "", errors.Internal("HandleTranscription", err, "Failed to update transcription status")
	}

	if err := validation.ValidateURL(url); err != nil {
		logger.WithError(err).WithField("url", url).Warn("Invalid URL format")
		return "", "", errors.InvalidInput("HandleTranscription", err, "Invalid URL format")
	}

	text, modelName, err = s.TranscriptionFunc(ctx, url)
	if err != nil {
		if err := db.SetTranscriptionStatus(ctx, url, "failed"); err != nil {
			logger.WithError(err).WithField("url", url).Error("Failed to set transcription status to failed")
		}
		logger.WithError(err).WithField("url", url).Error("Transcription script failed")
		return "", "", errors.Internal("HandleTranscription", err, "Transcription process failed")
	}

	if err := saveTranscription(ctx, url, text, modelName); err != nil {
		logger.WithError(err).WithFields(logrus.Fields{
			"url":        url,
			"model_name": modelName,
		}).Error("Failed to save transcription")
		return "", "", errors.Internal("HandleTranscription", err, "Failed to save transcription")
	}

	logger.WithFields(logrus.Fields{
		"url":        url,
		"model_name": modelName,
		"duration":   time.Since(start),
	}).Info("Transcription saved successfully")
	return text, modelName, nil
}

func saveTranscription(ctx context.Context, url, text, modelName string) error {
	if url == "" {
		return errors.InvalidInput("saveTranscription", nil, "URL cannot be empty")
	}

	if err := db.SetTranscription(ctx, url, text, modelName); err != nil {
		return errors.Internal("saveTranscription", err, "Failed to set transcription in database")
	}

	if err := db.SetTranscriptionStatus(ctx, url, "completed"); err != nil {
		return errors.Internal("saveTranscription", err, "Failed to update transcription status")
	}

	return nil
}

func (s *TranscriptionService) runTranscriptionScript(ctx context.Context, url string) (string, string, error) {
	const op = "transcription.runTranscriptionScript"
	logger := middleware.GetLogger(ctx)

	logger.WithField("url", url).Info("Starting transcription")

	output, err := s.executeTranscriptionWithRetry(ctx, url)
	if err != nil {
		return "", "", errors.Internal(op, err, "Transcription retry failed")
	}

	// Extract JSON part from the output
	jsonPart, err := extractJSON(ctx, output)
	if err != nil {
		return "", "", errors.Internal("runTranscriptionScript", err, "Failed to extract JSON from output")
	}

	// Parse the JSON
	var response struct {
		Transcription string `json:"transcription"`
		ModelName     string `json:"model_name"`
	}
	if err := json.Unmarshal([]byte(jsonPart), &response); err != nil {
		return "", "", errors.Internal("runTranscriptionScript", err, "Failed to parse JSON response")
	}

	logger.WithField("url", url).Info("Transcription completed successfully")
	return response.Transcription, response.ModelName, nil
}

func extractJSON(ctx context.Context, output []byte) (string, error) {
	logger := middleware.GetLogger(ctx)
	re := regexp.MustCompile(`\{.*\}`)
	matches := re.Find(output)
	if matches == nil {
		logger.Error("No JSON found in output")
		return "", errors.Internal("extractJSON", nil, "no JSON found in output")
	}
	return string(matches), nil
}

func (s *TranscriptionService) executeTranscriptionWithRetry(ctx context.Context, url string) ([]byte, error) {
	const op = "transcription.executeTranscriptionWithRetry"
	logger := middleware.GetLogger(ctx)
	start := time.Now()

	for attempt := 1; attempt <= maxRetries; attempt++ {
		logger.WithFields(logrus.Fields{
			"attempt": attempt,
			"url":     url,
		}).Debug("Attempting transcription")

		output, err := s.ExecuteScriptFunc(ctx, url, s.config)
		if err == nil {
			logger.WithField("duration", time.Since(start)).Info("Transcription successful")
			return output, nil
		}

		if attempt == maxRetries {
			return nil, errors.Internal(op, err, "Transcription failed after max retries")
		}

		// Calculate backoff duration with jitter
		backoff := time.Duration(float64(initialBackoff) * math.Pow(backoffFactor, float64(attempt-1)))
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
		jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
		totalBackoff := backoff + jitter

		logger.WithFields(logrus.Fields{
			"attempt": attempt,
			"backoff": totalBackoff,
			"error":   err,
		}).Warn("Transcription attempt failed, retrying")

		select {
		case <-time.After(totalBackoff):
			continue
		case <-ctx.Done():
			return nil, errors.Internal(op, ctx.Err(), "Context cancelled during retry")
		}
	}
	return nil, errors.Internal(op, fmt.Errorf("unexpected exit from retry loop"), "Transcription failed")
}

func executeTranscriptionScript(ctx context.Context, url string, cfg *config.Config) ([]byte, error) {
	logger := middleware.GetLogger(ctx)
	cmd := execCommand("uv", "run", "/app/scripts/transcribe.py",
		url,
		"--json",
		"--model", cfg.ModelName,
		"--temperature", fmt.Sprintf("%.2f", cfg.TranscriptionTemperature),
		"--beam-size", fmt.Sprintf("%d", cfg.TranscriptionBeamSize),
		"--best-of", fmt.Sprintf("%d", cfg.TranscriptionBestOf),
	)

	// Use the new environment configuration with memory limits
	env := append(os.Environ(), cfg.GetTranscriptionEnv()...)
	env = append(env, []string{
		fmt.Sprintf("MALLOC_ARENA_MAX=%d", 1),
		fmt.Sprintf("MALLOC_TRIM_THRESHOLD_=%d", cfg.MallocTrimThreshold),
		fmt.Sprintf("PYTHONMALLOC=malloc"),
		fmt.Sprintf("PYTORCH_CUDA_ALLOC_CONF=max_split_size_mb:%d", 32),
	}...)

	cmd.Env = env
	cmd.Dir = "/app"

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Log the error with context
		logger.WithFields(logrus.Fields{
			"url":    url,
			"error":  err,
			"output": string(output),
		}).Error("Error executing transcription script")

		// Handle memory-related errors...
		if strings.Contains(string(output), "MemoryError") || strings.Contains(string(output), "OutOfMemoryError") {
			return nil, errors.Internal("executeTranscriptionScript", err, "insufficient memory")
		}

		return nil, errors.Internal("executeTranscriptionScript", err, fmt.Sprintf("error executing transcription script: %v, output: %s", err, output))
	}

	logger.WithField("url", url).Info("Transcription script executed successfully")
	return output, nil
}

func readTranscriptionFile(ctx context.Context, filename string) (string, error) {
	logger := middleware.GetLogger(ctx)
	fileContent, err := os.ReadFile(filename)
	if err != nil {
		logger.WithError(err).WithField("filename", filename).Error("Failed to read file")
		return "", errors.Internal("readTranscriptionFile", err, fmt.Sprintf("error reading file: %v", err))
	}
	text := string(fileContent)
	if text == "" {
		logger.WithField("filename", filename).Error("Transcription resulted in empty text")
		return "", errors.Internal("readTranscriptionFile", nil, "empty transcription text")
	}

	return utils.FormatText(text), nil
}

func extractFilename(ctx context.Context, output []byte) (string, error) {
	logger := middleware.GetLogger(ctx)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	filename := lines[len(lines)-1]

	if filename == "" {
		logger.Error("Transcription script returned an empty filename")
		return "", errors.Internal("extractFilename", nil, "empty filename returned")
	}

	return filename, nil
}

func validateTranscriptionFile(ctx context.Context, filename string) error {
	logger := middleware.GetLogger(ctx)
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			logger.WithField("filename", filename).Error("Transcription file does not exist")
			return errors.Internal("validateTranscriptionFile", err, fmt.Sprintf("transcription file does not exist: %s", filename))
		}
		logger.WithError(err).WithField("filename", filename).Error("Failed to stat file")
		return errors.Internal("validateTranscriptionFile", err, fmt.Sprintf("failed to stat file: %v", err))
	}
	return nil
}

func generateSummary(ctx context.Context, text string) (string, string, error) {
	const op = "transcription.generateSummary"
	logger := middleware.GetLogger(ctx)

	cmd := execCommand("uv", "run", "/app/scripts/summarize.py", text)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error":  err,
			"output": string(output),
		}).Error("Error executing summarization script")
		return "", "", errors.Internal(op, err, fmt.Sprintf("error executing summarization script: %v, output: %s", err, output))
	}

	jsonPart, err := extractFinalJSON(ctx, output)
	if err != nil {
		logger.WithError(err).Error("Failed to extract JSON from output")
		return "", "", errors.Internal("generateSummary", err, fmt.Sprintf("failed to extract JSON from output: %v", err))
	}

	var result struct {
		Summary   string `json:"summary"`
		Error     string `json:"error"`
		ModelName string `json:"model_name"`
	}

	if err := json.Unmarshal([]byte(jsonPart), &result); err != nil {
		logrus.WithFields(logrus.Fields{
			"error":  err,
			"output": string(output),
		}).Error("Error parsing JSON output")
		return "", "", errors.Internal("generateSummary", err, fmt.Sprintf("error parsing JSON output: %v, output: %s", err, output))
	}

	if result.Error != "" {
		logrus.WithField("error", result.Error).Error("Summarization error")
		return "", "", errors.Internal("generateSummary", nil, fmt.Sprintf("summarization error: %s", result.Error))
	}

	return result.Summary, result.ModelName, nil
}

func extractFinalJSON(ctx context.Context, output []byte) (string, error) {
	logger := middleware.GetLogger(ctx)
	re := regexp.MustCompile(`\{.*\}`)
	matches := re.FindAll(output, -1)
	if len(matches) == 0 {
		logger.Error("No JSON found in output")
		return "", errors.Internal("extractFinalJSON", nil, "no JSON found in output")
	}
	return string(matches[len(matches)-1]), nil
}
