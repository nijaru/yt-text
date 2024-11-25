package video

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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

// processState represents what action to take with a video
type processState int

const (
	stateCreate processState = iota
	stateReturnExisting
	stateRetry
)

func (s *service) Create(ctx context.Context, url string) (*models.Video, error) {
	const op = "VideoService.Create"
	logger := s.logger.WithFields(logrus.Fields{
		"operation": op,
		"url":       url,
	})

	logger.Info("Starting video creation process")

	// Determine what to do with this URL
	video, state, err := s.determineVideoState(ctx, url)
	if err != nil {
		return nil, err
	}

	switch state {
	case stateReturnExisting:
		logger.WithField("video_id", video.ID).Info("Returning existing video")
		return video, nil

	case stateRetry:
		logger.WithField("video_id", video.ID).Info("Retrying failed video")
		return s.retryVideo(ctx, video)

	case stateCreate:
		logger.Info("Creating new video")
		return s.createNewVideo(ctx, url)

	default:
		return nil, errors.Internal(op, nil, "Invalid process state")
	}
}

func (s *service) determineVideoState(ctx context.Context, url string) (*models.Video, processState, error) {
	const op = "VideoService.determineVideoState"

	// Check for existing video
	existing, err := s.repo.GetByURL(ctx, url)
	if err != nil && !errors.IsNotFound(err) {
		return nil, stateCreate, errors.Internal(op, err, "Failed to check existing video")
	}

	if existing == nil {
		return nil, stateCreate, nil
	}

	// Determine state based on existing video status
	switch {
	case existing.IsCompleted():
		return existing, stateReturnExisting, nil

	case existing.IsProcessing():
		// If it's stuck in processing for too long, retry it
		if existing.IsStale(s.config.ProcessTimeout) {
			s.logger.WithFields(logrus.Fields{
				"video_id":   existing.ID,
				"status":     existing.Status,
				"updated_at": existing.UpdatedAt,
			}).Info("Found stale processing video, will retry")
			return existing, stateRetry, nil
		}
		// If it's already processing and not stale, just return it
		return existing, stateReturnExisting, nil

	case existing.IsPending():
		// If it's pending, we should retry it
		s.logger.WithField("video_id", existing.ID).Info("Found pending video, will retry")
		return existing, stateRetry, nil

	case existing.IsFailed() || existing.IsCancelled():
		s.logger.WithFields(logrus.Fields{
			"video_id": existing.ID,
			"status":   existing.Status,
		}).Info("Found failed or cancelled video, will retry")
		return existing, stateRetry, nil

	default:
		s.logger.WithFields(logrus.Fields{
			"video_id": existing.ID,
			"status":   existing.Status,
		}).Warn("Found video with unknown status, will retry")
		return existing, stateRetry, nil
	}
}

func (s *service) createNewVideo(ctx context.Context, url string) (*models.Video, error) {
	const op = "VideoService.createNewVideo"
	logger := s.logger.WithField("url", url)

	// Validate URL
	if err := s.validator.BasicURLValidation(url); err != nil {
		logger.WithError(err).Warn("Invalid URL format")
		return nil, err
	}

	// Deep validation
	info, err := s.scripts.Validate(ctx, url)
	if err != nil {
		logger.WithError(err).Error("Video validation failed")
		return nil, errors.InvalidInput(op, err, fmt.Sprintf("Video validation failed: %v", err))
	}

	if !info.Valid {
		errMsg := info.Error
		if errMsg == "" {
			errMsg = "Invalid video URL (no specific error provided)"
		}
		return nil, errors.InvalidInput(op, nil, errMsg)
	}

	// Create new video record
	video := &models.Video{
		ID:        uuid.New().String(),
		URL:       url,
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ModelInfo: models.ModelInfo{
			Name:     s.config.DefaultModel,
			Duration: info.Duration,
			FileSize: info.FileSize,
			Format:   info.Format,
		},
	}

	if err := s.repo.Create(ctx, video); err != nil {
		logger.WithError(err).Error("Failed to create video record")
		return nil, errors.Internal(op, err, "Failed to create video record")
	}

	// Start processing
	s.startProcessing(video)
	return video, nil
}

func (s *service) retryVideo(ctx context.Context, video *models.Video) (*models.Video, error) {
	const op = "VideoService.retryVideo"
	logger := s.logger.WithField("video_id", video.ID)

	// Reset video state
	video.Status = models.StatusPending
	video.Error = ""
	video.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, video); err != nil {
		logger.WithError(err).Error("Failed to update video for retry")
		return nil, errors.Internal(op, err, "Failed to update video for retry")
	}

	// Start processing
	s.startProcessing(video)
	return video, nil
}

func (s *service) startProcessing(video *models.Video) {
	logger := s.logger.WithFields(logrus.Fields{
		"video_id": video.ID,
		"url":      video.URL,
	})

	go func() {
		processCtx, cancel := context.WithTimeout(context.Background(), s.config.ProcessTimeout)
		defer cancel()

		logger.Info("Starting background processing")

		if err := s.Process(processCtx, video.ID); err != nil {
			logger.WithError(err).Error("Background processing failed")

			video.Status = models.StatusFailed
			video.Error = err.Error()
			video.UpdatedAt = time.Now()

			if updateErr := s.repo.Update(context.Background(), video); updateErr != nil {
				logger.WithError(updateErr).Error("Failed to update video status after error")
			}
		}
	}()
}

func (s *service) Process(ctx context.Context, id string) error {
	const op = "VideoService.Process"
	logger := s.logger.WithFields(logrus.Fields{
		"operation": op,
		"video_id":  id,
	})

	logger.Info("Starting video processing")

	// Get video
	video, err := s.repo.Get(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get video")
		return errors.Internal(op, err, "Failed to get video")
	}

	// Update status to processing
	video.Status = models.StatusProcessing
	video.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, video); err != nil {
		logger.WithError(err).Error("Failed to update video status")
		return errors.Internal(op, err, "Failed to update video status")
	}

	// Start transcription
	logger.Info("Starting transcription process")
	opts := map[string]string{}

	if video.ModelInfo.Name != "" {
		opts["model"] = video.ModelInfo.Name
	}

	result, err := s.scripts.Transcribe(ctx, video.URL, opts)
	if err != nil {
		logger.WithError(err).Error("Transcription failed")
		video.Status = models.StatusFailed
		video.Error = err.Error()
		video.UpdatedAt = time.Now()
		_ = s.repo.Update(ctx, video)
		return errors.Internal(op, err, "Transcription failed")
	}

	// Update video with transcription result
	logger.Info("Transcription completed successfully")
	video.Status = models.StatusCompleted
	video.Transcription = result.Text
	video.ModelInfo.Name = result.ModelName
	video.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, video); err != nil {
		logger.WithError(err).Error("Failed to save transcription")
		return errors.Internal(op, err, "Failed to save transcription")
	}

	logger.Info("Video processing completed successfully")
	return nil
}

func (s *service) Get(ctx context.Context, id string) (*models.Video, error) {
	const op = "VideoService.Get"
	logger := s.logger.WithFields(logrus.Fields{
		"operation": op,
		"video_id":  id,
	})

	video, err := s.repo.Get(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get video")
		return nil, errors.Internal(op, err, "Failed to get video")
	}

	return video, nil
}

func (s *service) GetByURL(ctx context.Context, url string) (*models.Video, error) {
	const op = "VideoService.GetByURL"
	logger := s.logger.WithFields(logrus.Fields{
		"operation": op,
		"url":       url,
	})

	video, err := s.repo.GetByURL(ctx, url)
	if err != nil {
		logger.WithError(err).Error("Failed to get video by URL")
		return nil, errors.Internal(op, err, "Failed to get video by URL")
	}

	return video, nil
}

func (s *service) Cancel(ctx context.Context, id string) error {
	const op = "VideoService.Cancel"
	logger := s.logger.WithFields(logrus.Fields{
		"operation": op,
		"video_id":  id,
	})

	if id == "" {
		return errors.InvalidInput(op, nil, "ID is required")
	}

	// Get video
	video, err := s.repo.Get(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get video")
		return errors.Internal(op, err, "Failed to get video")
	}

	// Update video status
	video.Status = models.StatusCancelled
	video.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, video); err != nil {
		logger.WithError(err).Error("Failed to update video status")
		return errors.Internal(op, err, "Failed to update video status")
	}

	logger.Info("Video processing cancelled successfully")
	return nil
}

func (s *service) Shutdown(ctx context.Context) error {
	return nil
}
