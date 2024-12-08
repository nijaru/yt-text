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
	video, err := s.repo.FindByURL(ctx, url)
	if err == nil {
		// Handle existing video
		if shouldProcessExisting(video, s.config.ProcessTimeout) {
			return s.startProcessing(ctx, video)
		}
		return video, nil
	}

	// For new videos, validate and create
	if err := s.validateNewVideo(ctx, url); err != nil {
		return nil, err
	}

	// Create new video record
	video = &models.Video{
		ID:        uuid.New().String(),
		URL:       url,
		CreatedAt: time.Now(),
	}

	return s.startProcessing(ctx, video)
}

func shouldProcessExisting(video *models.Video, timeout time.Duration) bool {
	switch video.Status {
	case models.StatusCompleted:
		return false
	case models.StatusProcessing:
		return video.IsStale(timeout)
	case models.StatusFailed:
		return true
	default:
		return true
	}
}

func (s *service) validateNewVideo(ctx context.Context, url string) error {
	const op = "VideoService.validateNewVideo"

	// Basic URL validation
	if err := s.validator.ValidateURL(url); err != nil {
		s.logger.WithError(err).Info("URL validation failed")
		return err
	}

	// Validate video metadata
	info, err := s.scripts.Validate(ctx, url)
	if err != nil {
		s.logger.WithError(err).Error("Video validation script failed")
		return errors.InvalidInput(op, err, "Failed to validate video")
	}

	if !info.Valid {
		s.logger.WithField("error", info.Error).Info("Video validation failed")
		return errors.InvalidInput(op, nil, info.Error)
	}

	return nil
}

func (s *service) startProcessing(ctx context.Context, video *models.Video) (*models.Video, error) {
	const op = "VideoService.startProcessing"

	// Update status and timestamp
	video.Status = models.StatusProcessing
	video.UpdatedAt = time.Now()
	video.Error = "" // Clear any previous error

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
		if result.Title != nil {
			video.Title = *result.Title
		} else {
			video.Title = video.URL
		}

		// Add debug logging
		logger.WithFields(logrus.Fields{
			"transcription_length": len(video.Transcription),
			"title":                video.Title,
			"status":               video.Status,
		}).Info("Updated video with transcription")
	}

	video.UpdatedAt = time.Now()

	// Update video record
	if err := s.repo.Save(ctx, video); err != nil {
		logger.WithError(err).Error("Failed to save transcription result")
	} else {
		// Add debug logging after save
		logger.WithFields(logrus.Fields{
			"transcription_length": len(video.Transcription),
			"title":                video.Title,
			"status":               video.Status,
			"updated_at":           video.UpdatedAt,
		}).Info("Saved video with transcription")
	}
}
