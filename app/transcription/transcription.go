package transcription

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

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
	ReadFileFunc      func(filename string) (string, error)
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
	logger := logrus.WithFields(logrus.Fields{
		"url":        url,
		"request_id": ctx.Value(middleware.RequestIDKey),
		"model_name": cfg.ModelName,
	})

	logger.Info("Starting transcription process")

	lock := getTranscriptionLock(url)
	lock.mu.Lock()
	defer lock.mu.Unlock()

	text, status, err := db.GetTranscription(ctx, url)
	if err != nil {
		logger.WithError(err).Error("Failed to get transcription from DB")
		return "", "", errors.ErrDatabaseOperation(err)
	}

	if status == "completed" {
		modelName, err := db.GetModelName(ctx, url)
		if err != nil {
			logger.WithError(err).Error("Failed to get model name from DB")
			return "", "", errors.ErrDatabaseOperation(err)
		}

		if modelName == cfg.ModelName {
			logger.Info("Using existing transcription from database")
			return text, modelName, nil
		}

		logger.WithFields(logrus.Fields{
			"current_model": cfg.ModelName,
			"stored_model":  modelName,
		}).Info("Model mismatch, initiating new transcription")
	}

	if err := db.SetTranscriptionStatus(ctx, url, "in_progress"); err != nil {
		logger.WithError(err).Error("Failed to set transcription status to in_progress")
		return "", "", errors.ErrDatabaseOperation(err)
	}

	if err := validation.ValidateURL(url); err != nil {
		return "", "", errors.ErrInvalidURL(err)
	}

	text, modelName, err := s.TranscriptionFunc(ctx, url)
	if err != nil {
		db.SetTranscriptionStatus(ctx, url, "failed")
		logger.WithError(err).Error("Transcription script failed")
		return "", "", errors.ErrTranscriptionFailed(err)
	}

	if err := saveTranscription(ctx, url, text, modelName); err != nil {
		return "", "", errors.ErrDatabaseOperation(err)
	}

	logger.Info("Transcription saved successfully")
	return text, modelName, nil
}

func saveTranscription(ctx context.Context, url, text, modelName string) error {
	if url == "" {
		return errors.ErrInvalidRequest("URL cannot be empty")
	}

	if err := db.SetTranscription(ctx, url, text, modelName); err != nil {
		return errors.ErrDatabaseOperation(err)
	}

	if err := db.SetTranscriptionStatus(ctx, url, "completed"); err != nil {
		return errors.ErrDatabaseOperation(err)
	}

	return nil
}

func (s *TranscriptionService) runTranscriptionScript(ctx context.Context, url string) (string, string, error) {
	logrus.WithField("url", url).Info("Starting transcription")

	output, err := s.executeTranscriptionWithRetry(ctx, url)
	if err != nil {
		return "", "", err
	}

	// Extract JSON part from the output
	jsonPart, err := extractJSON(output)
	if err != nil {
		return "", "", errors.ErrTranscriptionFailed(fmt.Errorf("failed to extract JSON from output: %v", err))
	}

	// Parse the JSON
	var response struct {
		Transcription string `json:"transcription"`
		ModelName     string `json:"model_name"`
	}
	if err := json.Unmarshal([]byte(jsonPart), &response); err != nil {
		return "", "", errors.ErrTranscriptionFailed(fmt.Errorf("failed to parse JSON: %v", err))
	}

	logrus.WithField("url", url).Info("Transcription completed successfully")
	return response.Transcription, response.ModelName, nil
}

func extractJSON(output []byte) (string, error) {
	re := regexp.MustCompile(`\{.*\}`)
	matches := re.Find(output)
	if matches == nil {
		return "", errors.ErrTranscriptionFailed(fmt.Errorf("no JSON found in output"))
	}
	return string(matches), nil
}

func (s *TranscriptionService) executeTranscriptionWithRetry(ctx context.Context, url string) ([]byte, error) {
	logger := logrus.WithFields(logrus.Fields{
		"url":        url,
		"request_id": ctx.Value(middleware.RequestIDKey),
		"model_name": s.config.ModelName,
	})

	var (
		output []byte
		err    error
	)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		logger = logger.WithField("attempt", attempt)
		logger.Info("Attempting transcription")

		output, err = s.ExecuteScriptFunc(ctx, url, s.config)
		if err == nil {
			logger.Info("Transcription attempt successful")
			break
		}

		logger.WithFields(logrus.Fields{
			"error":  err,
			"output": string(output),
		}).Warn("Transcription attempt failed")

		if attempt == maxRetries {
			logger.Error("All transcription attempts failed")
			return nil, errors.ErrTranscriptionFailed(fmt.Errorf("error transcribing after %d attempts: %v, output: %s", maxRetries, err, output))
		}

		// Calculate backoff duration
		backoff := time.Duration(float64(initialBackoff) * math.Pow(backoffFactor, float64(attempt-1)))
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		// Add jitter to prevent thundering herd
		jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
		totalBackoff := backoff + jitter

		logger.WithFields(logrus.Fields{
			"backoff_duration": totalBackoff,
			"next_attempt":     attempt + 1,
		}).Info("Waiting before next attempt")

		// Wait for backoff duration or context cancellation
		select {
		case <-time.After(totalBackoff):
			continue
		case <-ctx.Done():
			logger.WithError(ctx.Err()).Error("Context cancelled during retry backoff")
			return nil, ctx.Err()
		}
	}

	if err != nil {
		logger.WithFields(logrus.Fields{
			"maxRetries": maxRetries,
			"error":      err,
			"output":     string(output),
		}).Error("Transcription failed after max retries")
		return nil, errors.ErrTranscriptionFailed(fmt.Errorf("error transcribing after %d attempts: %v, output: %s", maxRetries, err, output))
	}

	return output, nil
}

func executeTranscriptionScript(ctx context.Context, url string, cfg *config.Config) ([]byte, error) {
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
		// Check for memory-related errors in the output
		if strings.Contains(string(output), "MemoryError") ||
			strings.Contains(string(output), "OutOfMemoryError") {
			return nil, errors.ErrTranscriptionFailed(fmt.Errorf("insufficient memory"))
		}

		if len(output) > 0 {
			return nil, errors.ErrTranscriptionFailed(fmt.Errorf("error executing transcription script: %v, output: %s", err, output))
		}
		return nil, errors.ErrTranscriptionFailed(fmt.Errorf("error executing transcription script: %v", err))
	}

	return output, nil
}
func readTranscriptionFile(filename string) (string, error) {
	fileContent, err := os.ReadFile(filename)
	if err != nil {
		logrus.WithError(err).WithField("filename", filename).Error("Failed to read file")
		return "", errors.ErrTranscriptionFailed(fmt.Errorf("error reading file: %v", err))
	}
	text := string(fileContent)
	if text == "" {
		logrus.WithField("filename", filename).Error("Transcription resulted in empty text")
		return "", errors.ErrTranscriptionFailed(fmt.Errorf("empty transcription text"))
	}

	return utils.FormatText(text), nil
}

func extractFilename(output []byte) (string, error) {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	filename := lines[len(lines)-1]

	if filename == "" {
		logrus.Error("Transcription script returned an empty filename")
		return "", errors.ErrTranscriptionFailed(fmt.Errorf("empty filename returned"))
	}

	return filename, nil
}
func validateTranscriptionFile(filename string) error {
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.WithField("filename", filename).Error("Transcription file does not exist")
			return errors.ErrTranscriptionFailed(fmt.Errorf("transcription file does not exist: %s", filename))
		}
		logrus.WithError(err).WithField("filename", filename).Error("Failed to stat file")
		return errors.ErrTranscriptionFailed(fmt.Errorf("failed to stat file: %v", err))
	}
	return nil
}

func generateSummary(ctx context.Context, text string) (string, string, error) {
	cmd := execCommand("uv", "run", "/app/scripts/summarize.py", text)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error":  err,
			"output": string(output),
		}).Error("Error executing summarization script")
		return "", "", errors.ErrTranscriptionFailed(fmt.Errorf("error executing summarization script: %v, output: %s", err, output))
	}

	jsonPart, err := extractFinalJSON(output)
	if err != nil {
		return "", "", errors.ErrTranscriptionFailed(fmt.Errorf("failed to extract JSON from output: %v", err))
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
		return "", "", errors.ErrTranscriptionFailed(fmt.Errorf("error parsing JSON output: %v, output: %s", err, output))
	}

	if result.Error != "" {
		logrus.WithField("error", result.Error).Error("Summarization error")
		return "", "", errors.ErrTranscriptionFailed(fmt.Errorf("summarization error: %s", result.Error))
	}

	return result.Summary, result.ModelName, nil
}

func extractFinalJSON(output []byte) (string, error) {
	re := regexp.MustCompile(`\{.*\}`)
	matches := re.FindAll(output, -1)
	if len(matches) == 0 {
		return "", errors.ErrTranscriptionFailed(fmt.Errorf("no JSON found in output"))
	}
	return string(matches[len(matches)-1]), nil
}
