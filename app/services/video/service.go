package video

import (
	"context"
	"time"
	"yt-text/errors"
	"yt-text/models"
	"yt-text/repository"
	"yt-text/scripts"
	"yt-text/validation"

	"github.com/google/uuid"
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

	// Check for existing transcription first
	existing, err := s.repo.FindByURL(ctx, url)
	if err == nil {
		if existing.IsCompleted() {
			return existing, nil
		}
		if existing.IsProcessing() && !existing.IsStale(s.config.ProcessTimeout) {
			return existing, nil
		}
		// If it's stale or failed, we'll continue with reprocessing
	}

	// Basic URL validation
	if err := s.validator.ValidateURL(url); err != nil {
		logger.WithError(err).Info("URL validation failed")
		return nil, err
	}

	// Validate video metadata using yt-dlp
	info, err := s.scripts.Validate(ctx, url)
	if err != nil {
		logger.WithError(err).Error("Video validation script failed")
		return nil, errors.InvalidInput(op, err, "Failed to validate video")
	}

	if !info.Valid {
		logger.WithField("error", info.Error).Info("Video validation failed")
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
	result, err := s.scripts.Transcribe(ctx, video.URL, opts, true)
	if err != nil {
		logger.WithError(err).Error("Transcription failed")
		video.Status = models.StatusFailed
		video.Error = err.Error()
	} else {
		logger.Info("Transcription completed successfully")
		video.Status = models.StatusCompleted
		video.Transcription = result.Text
		video.Title = *result.Title
	}

	video.UpdatedAt = time.Now()

	// Update video record
	if err := s.repo.Save(ctx, video); err != nil {
		logger.WithError(err).Error("Failed to save transcription result")
	}
}
