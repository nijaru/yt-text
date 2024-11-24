package summary

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nijaru/yt-text/errors"
	"github.com/nijaru/yt-text/models"
	"github.com/nijaru/yt-text/scripts"
	"github.com/nijaru/yt-text/validation"
	"github.com/sirupsen/logrus"
)

type service struct {
	repo      Repository
	scripts   *scripts.ScriptRunner
	validator *validation.Validator
	config    Config
	logger    *logrus.Logger
}

// NewService creates a new summary service
func NewService(
	repo Repository,
	scriptRunner *scripts.ScriptRunner,
	validator *validation.Validator,
	config Config,
) Service {
	return &service{
		repo:      repo,
		scripts:   scriptRunner,
		validator: validator,
		config:    config,
		logger:    logrus.StandardLogger(),
	}
}

func (s *service) CreateSummary(ctx context.Context, url string, opts Options) (*models.Video, error) {
	const op = "SummaryService.CreateSummary"
	logger := s.logger.WithContext(ctx).WithField("url", url)

	// Basic URL validation
	if err := s.validator.BasicURLValidation(url); err != nil {
		logger.WithError(err).Warn("Invalid URL format")
		return nil, err
	}

	// Get video
	video, err := s.repo.GetByURL(ctx, url)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NotFound(op, err, "Video not found")
		}
		return nil, errors.Internal(op, err, "Failed to get video")
	}

	// Check if transcription exists and is completed
	if !video.IsCompleted() || video.Transcription == "" {
		return nil, errors.InvalidInput(op, nil, "Video must be transcribed before summarizing")
	}

	// Check if summary already exists with same model
	if video.Summary != "" && video.ModelInfo.SummaryModel == s.config.ModelName {
		return video, nil
	}

	// Process transcription in chunks
	chunks := s.splitText(video.Transcription)
	summaries := make([]string, 0, len(chunks))

	// Process each chunk
	for i, chunk := range chunks {
		select {
		case <-ctx.Done():
			return nil, errors.Internal(op, ctx.Err(), "Summary creation cancelled")
		default:
			logger.WithFields(logrus.Fields{
				"chunk": i + 1,
				"total": len(chunks),
			}).Debug("Processing chunk")

			summary, err := s.processChunk(ctx, chunk)
			if err != nil {
				return nil, errors.Internal(op, err, "Failed to summarize chunk")
			}
			summaries = append(summaries, summary)
		}
	}

	// Combine summaries
	finalSummary, err := s.combineSummaries(ctx, summaries)
	if err != nil {
		return nil, errors.Internal(op, err, "Failed to combine summaries")
	}

	// Update video
	video.Summary = finalSummary
	video.ModelInfo.SummaryModel = s.config.ModelName
	video.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, video); err != nil {
		return nil, errors.Internal(op, err, "Failed to save summary")
	}

	return video, nil
}

func (s *service) GetSummary(ctx context.Context, url string) (*models.Video, error) {
	const op = "SummaryService.GetSummary"

	// Basic URL validation
	if err := s.validator.BasicURLValidation(url); err != nil {
		return nil, err
	}

	video, err := s.repo.GetByURL(ctx, url)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NotFound(op, err, "Video not found")
		}
		return nil, errors.Internal(op, err, "Failed to get video")
	}

	if video.Summary == "" {
		return nil, errors.NotFound(op, nil, "Summary not found")
	}

	return video, nil
}

func (s *service) GetStatus(ctx context.Context, id string) (*models.Video, error) {
	const op = "SummaryService.GetStatus"

	if id == "" {
		return nil, errors.InvalidInput(op, nil, "ID is required")
	}

	video, err := s.repo.GetByURL(ctx, id)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NotFound(op, err, "Video not found")
		}
		return nil, errors.Internal(op, err, "Failed to get video status")
	}

	return video, nil
}

// Helper methods

func (s *service) processChunk(ctx context.Context, text string) (string, error) {
	opts := map[string]string{
		"model":      s.config.ModelName,
		"max_length": fmt.Sprintf("%d", s.config.MaxLength),
		"min_length": fmt.Sprintf("%d", s.config.MinLength),
	}

	result, err := s.scripts.Summarize(ctx, text, opts)
	if err != nil {
		return "", fmt.Errorf("failed to summarize chunk: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("summarization failed: %s", result.Error)
	}

	return result.Summary, nil
}

func (s *service) splitText(text string) []string {
	words := strings.Fields(text)
	if len(words) <= s.config.BatchSize {
		return []string{text}
	}

	var chunks []string
	for i := 0; i < len(words); i += s.config.BatchSize {
		end := i + s.config.BatchSize
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
	}

	return chunks
}

func (s *service) combineSummaries(ctx context.Context, summaries []string) (string, error) {
	if len(summaries) == 1 {
		return summaries[0], nil
	}

	combinedText := strings.Join(summaries, " ")
	if len(strings.Fields(combinedText)) <= s.config.BatchSize {
		return combinedText, nil
	}

	// Create final summary from combined text
	summary, err := s.processChunk(ctx, combinedText)
	if err != nil {
		return combinedText, fmt.Errorf("failed to create final summary: %w", err)
	}

	return summary, nil
}
