package video

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nijaru/yt-text/errors"
	"github.com/nijaru/yt-text/models"
	"github.com/nijaru/yt-text/repository"
	"github.com/nijaru/yt-text/scripts"
	"github.com/nijaru/yt-text/validation"
	"github.com/sirupsen/logrus"
)

type Repository = repository.VideoRepository

type service struct {
	repo      Repository
	scripts   *scripts.ScriptRunner
	validator *validation.Validator
	config    Config
	logger    *logrus.Logger
}

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

func (s *service) Transcribe(ctx context.Context, url string) (*models.Video, error) {
	const op = "VideoService.Transcribe"
	logger := s.logger.WithFields(logrus.Fields{
		"operation": op,
		"url":       url,
	})
	logger.Info("Starting transcription request")

	// Check for existing transcription
	existing, err := s.repo.FindByURL(ctx, url)
	if err == nil {
		if existing.IsCompleted() {
			return existing, nil
		}
		if existing.IsProcessing() && !existing.IsStale(s.config.ProcessTimeout) {
			return existing, nil
		}
		// If it's stale or failed, we'll reprocess it
	}

	// Validate URL and video
	if err := s.validator.ValidateURL(url); err != nil {
		return nil, err
	}

	info, err := s.scripts.Validate(ctx, url)
	if err != nil {
		return nil, errors.InvalidInput(op, err, "Video validation failed")
	}

	if !info.Valid {
		return nil, errors.InvalidInput(op, nil, info.Error)
	}

	// Create or update video record
	video := &models.Video{
		ID:        uuid.New().String(),
		URL:       url,
		Status:    models.StatusProcessing,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Save(ctx, video); err != nil {
		return nil, errors.Internal(op, err, "Failed to save video")
	}

	// Start processing in background
	go s.processVideo(video)

	return video, nil
}

func (s *service) GetTranscription(ctx context.Context, id string) (*models.Video, error) {
	const op = "VideoService.GetTranscription"

	if id == "" {
		return nil, errors.InvalidInput(op, nil, "ID is required")
	}

	video, err := s.repo.Find(ctx, id)
	if err != nil {
		return nil, errors.NotFound(op, err, "Transcription not found")
	}

	return video, nil
}

func (s *service) processVideo(video *models.Video) {
	logger := s.logger.WithField("video_id", video.ID)
	ctx, cancel := context.WithTimeout(context.Background(), s.config.ProcessTimeout)
	defer cancel()

	logger.Info("Starting transcription process")

	// Set up transcription options
	opts := map[string]string{
		"model": s.config.DefaultModel,
	}

	// Perform transcription
	result, err := s.scripts.Transcribe(ctx, video.URL, opts)
	if err != nil {
		logger.WithError(err).Error("Transcription failed")
		video.Status = models.StatusFailed
		video.Error = err.Error()
	} else {
		logger.Info("Transcription completed successfully")
		video.Status = models.StatusCompleted
		video.Transcription = result.Text
	}

	video.UpdatedAt = time.Now()

	// Update video record
	if err := s.repo.Save(ctx, video); err != nil {
		logger.WithError(err).Error("Failed to save transcription result")
	}
}
