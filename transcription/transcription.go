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
	"github.com/nijaru/yt-text/utils"
	"github.com/nijaru/yt-text/validation"
	"github.com/sirupsen/logrus"
)

var transcriptionLocks sync.Map

type transcriptionLock struct {
	mu sync.Mutex
}

func getTranscriptionLock(url string) *transcriptionLock {
	lock, _ := transcriptionLocks.LoadOrStore(url, &transcriptionLock{})
	return lock.(*transcriptionLock)
}

type TranscriptionService struct {
	TranscriptionFunc func(ctx context.Context, url string) (string, string, error)
	ExecuteScriptFunc func(ctx context.Context, url string) ([]byte, error)
	ReadFileFunc      func(filename string) (string, error)
	SummaryFunc       func(ctx context.Context, text string) (string, string, error)
}

func NewTranscriptionService() *TranscriptionService {
	return &TranscriptionService{
		TranscriptionFunc: runTranscriptionScript,
		ExecuteScriptFunc: executeTranscriptionScript,
		ReadFileFunc:      readTranscriptionFile,
		SummaryFunc:       generateSummary,
	}
}

func (s *TranscriptionService) HandleTranscription(ctx context.Context, url string, cfg *config.Config) (string, string, error) {
	lock := getTranscriptionLock(url)
	lock.mu.Lock()
	defer lock.mu.Unlock()

	text, status, err := db.GetTranscription(ctx, url)
	if err != nil {
		logrus.WithError(err).WithField("url", url).Error("Failed to get transcription from DB")
		return "", "", err
	}

	if status == "completed" {
		// Check if the model name in the database is different from the current model name
		modelName, err := db.GetModelName(ctx, url)
		if err != nil {
			logrus.WithError(err).WithField("url", url).Error("Failed to get model name from DB")
			return "", "", err
		}

		if modelName == cfg.ModelName {
			logrus.WithField("url", url).Info("Transcription found in database with the same model name")
			return text, modelName, nil
		}

		logrus.WithField("url", url).Info("Model name mismatch, redoing transcription")
	}

	if err := db.SetTranscriptionStatus(ctx, url, "in_progress"); err != nil {
		logrus.WithError(err).WithField("url", url).Error("Failed to set transcription status to in_progress")
		return "", "", fmt.Errorf("error setting transcription status: %v", err)
	}

	if err := validation.ValidateURL(url); err != nil {
		return "", "", err
	}

	text, modelName, err := s.TranscriptionFunc(ctx, url)
	if err != nil {
		db.SetTranscriptionStatus(ctx, url, "failed")
		logrus.WithError(err).WithField("url", url).Error("Transcription script failed")
		return "", "", err
	}

	if err := saveTranscription(ctx, url, text, modelName); err != nil {
		return "", "", err
	}

	logrus.WithField("url", url).Info("Transcription saved successfully")
	return text, modelName, nil
}

func saveTranscription(ctx context.Context, url, text, modelName string) error {
	if err := db.SetTranscription(ctx, url, text, modelName); err != nil {
		logrus.WithError(err).WithField("url", url).Error("Failed to save transcription")
		return fmt.Errorf("error saving transcription: %v", err)
	}

	if err := db.SetTranscriptionStatus(ctx, url, "completed"); err != nil {
		logrus.WithError(err).WithField("url", url).Error("Failed to set transcription status to completed")
		return fmt.Errorf("error setting transcription status: %v", err)
	}

	return nil
}

func runTranscriptionScript(ctx context.Context, url string) (string, string, error) {
	logrus.WithField("url", url).Info("Starting transcription")

	output, err := executeTranscriptionWithRetry(ctx, url)
	if err != nil {
		return "", "", err
	}

	// Extract JSON part from the output
	jsonPart, err := extractJSON(output)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract JSON from output: %v", err)
	}

	// Parse the JSON
	var response struct {
		Transcription string `json:"transcription"`
		ModelName     string `json:"model_name"`
	}
	if err := json.Unmarshal([]byte(jsonPart), &response); err != nil {
		return "", "", fmt.Errorf("failed to parse JSON: %v", err)
	}

	logrus.WithField("url", url).Info("Transcription completed successfully")
	return response.Transcription, response.ModelName, nil
}

func extractJSON(output []byte) (string, error) {
	re := regexp.MustCompile(`\{.*\}`)
	matches := re.Find(output)
	if matches == nil {
		return "", fmt.Errorf("no JSON found in output")
	}
	return string(matches), nil
}

func executeTranscriptionWithRetry(ctx context.Context, url string) ([]byte, error) {
	const (
		maxRetries     = 3
		initialBackoff = 2 * time.Second
		maxBackoff     = 30 * time.Second
		backoffFactor  = 2.0
	)

	var (
		output []byte
		err    error
	)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		output, err = executeTranscriptionScript(ctx, url)
		if err == nil {
			break
		}

		logrus.WithFields(logrus.Fields{
			"attempt":    attempt,
			"maxRetries": maxRetries,
			"url":        url,
			"error":      err,
			"output":     string(output),
		}).Error("Transcription script failed")

		backoff := time.Duration(float64(initialBackoff) * math.Pow(backoffFactor, float64(attempt-1)))
		if backoff > maxBackoff {
			backoff = maxBackoff
		}

		select {
		case <-time.After(backoff + time.Duration(rand.Int63n(int64(backoff/2)))):
			// Continue to the next retry attempt
		case <-ctx.Done():
			logrus.WithError(ctx.Err()).WithField("url", url).Error("Context cancelled during transcription")
			return nil, ctx.Err()
		}
	}

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"maxRetries": maxRetries,
			"url":        url,
			"error":      err,
			"output":     string(output),
		}).Error("Transcription failed after max retries")
		return nil, fmt.Errorf("error transcribing after %d attempts: %v, output: %s", maxRetries, err, output)
	}

	return output, nil
}

func executeTranscriptionScript(ctx context.Context, url string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "uv", "run", "transcribe.py", url, "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error executing transcription script: %v, output: %s", err, output)
	}
	return output, nil
}

func readTranscriptionFile(filename string) (string, error) {
	fileContent, err := os.ReadFile(filename)
	if err != nil {
		logrus.WithError(err).WithField("filename", filename).Error("Failed to read file")
		return "", fmt.Errorf("error reading file: %v", err)
	}
	text := string(fileContent)
	if text == "" {
		logrus.WithField("filename", filename).Error("Transcription resulted in empty text")
		return "", fmt.Errorf("error transcribing")
	}

	return utils.FormatText(text), nil
}

func extractFilename(output []byte) (string, error) {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	filename := lines[len(lines)-1]

	if filename == "" {
		logrus.Error("Transcription script returned an empty filename")
		return "", fmt.Errorf("error: transcription script returned an empty filename")
	}

	return filename, nil
}

func validateTranscriptionFile(filename string) error {
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.WithField("filename", filename).Error("Transcription file does not exist")
			return fmt.Errorf("error: transcription file does not exist: %s", filename)
		}
		logrus.WithError(err).WithField("filename", filename).Error("Failed to stat file")
		return fmt.Errorf("error: failed to stat file: %v", err)
	}
	return nil
}

func generateSummary(ctx context.Context, text string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "python3", "summarize.py", text)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error":  err,
			"output": string(output),
		}).Error("Error executing summarization script")
		return "", "", fmt.Errorf("error executing summarization script: %v, output: %s", err, output)
	}

	var result struct {
		Summary   string `json:"summary"`
		Error     string `json:"error"`
		ModelName string `json:"model_name"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", "", fmt.Errorf("error parsing JSON output: %v, output: %s", err, output)
	}

	if result.Error != "" {
		return "", "", fmt.Errorf("summarization error: %s", result.Error)
	}

	return result.Summary, result.ModelName, nil
}
