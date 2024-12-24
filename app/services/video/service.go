package video

import (
	"context"
	"time"
	"yt-text/errors"
	"yt-text/models"
	"yt-text/repository"
	"yt-text/scripts"
	"yt-text/services/subtitles"
	"yt-text/validation"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Repository = repository.VideoRepository

type service struct {
	repo            Repository
	scripts         *scripts.ScriptRunner
	validator       *validation.Validator
	config          Config
	logger          zerolog.Logger
	subtitleService subtitles.Service
}

func NewService(
	repo Repository,
	scriptRunner *scripts.ScriptRunner,
	validator *validation.Validator,
	subtitleService subtitles.Service,
	config Config,
) Service {
	return &service{
		repo:            repo,
		scripts:         scriptRunner,
		validator:       validator,
		subtitleService: subtitleService,
		config:          config,
		logger:          zerolog.New(zerolog.NewConsoleWriter()),
	}
}

func (s *service) Transcribe(ctx context.Context, url string) (*models.Video, error) {
	const op = "VideoService.Transcribe"
	logger := s.logger.With().
		Str("operation", op).
		Str("url", url).
		Logger()
	logger.Info().Msg("Starting transcription request")

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
		s.logger.Info().Err(err).Msg("URL validation failed")
		return err
	}

	// Validate video metadata
	info, err := s.scripts.Validate(ctx, url)
	if err != nil {
		s.logger.Error().Err(err).Msg("Video validation script failed")
		return errors.InvalidInput(op, err, "Failed to validate video")
	}

	if !info.Valid {
		s.logger.Info().Str("error", info.Error).Msg("Video validation failed")
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
	logger := s.logger.With().Str("video_id", video.ID).Logger()
	ctx, cancel := context.WithTimeout(context.Background(), s.config.ProcessTimeout)
	defer cancel()

	logger.Info().Msg("Starting transcription process")

	// First try to get subtitles
	subtitleInfo, err := s.subtitleService.GetAvailable(ctx, video.URL)
	if err == nil && subtitleInfo.Available {
		// Try auto-generated subtitles first since they're often available
		if tracks, ok := subtitleInfo.AutoSubtitles["en"]; ok && len(tracks) > 0 {
			text, err := s.subtitleService.Download(ctx, video.URL, "en", true)
			if err == nil && text != "" {
				s.updateVideoWithTranscription(video, text, subtitleInfo.Title)
				return
			}
			logger.Warn().Err(err).Msg("Failed to get auto-generated subtitles")
		}

		// Try manual subtitles as fallback
		if tracks, ok := subtitleInfo.Subtitles["en"]; ok && len(tracks) > 0 {
			text, err := s.subtitleService.Download(ctx, video.URL, "en", false)
			if err == nil && text != "" {
				s.updateVideoWithTranscription(video, text, subtitleInfo.Title)
				return
			}
			logger.Warn().Err(err).Msg("Failed to get manual subtitles")
		}
	}

	// Fallback to transcription if no subtitles available or failed
	logger.Info().Msg("No subtitles available or failed to get them, falling back to transcription")
	opts := map[string]string{
		"model": s.config.DefaultModel,
	}

	result, err := s.scripts.Transcribe(ctx, video.URL, opts, true)
	if err != nil {
		logger.Error().Err(err).Msg("Transcription failed")
		video.Status = models.StatusFailed
		video.Error = err.Error()
	} else {
		logger.Info().Msg("Transcription completed successfully")
		video.Status = models.StatusCompleted
		video.Transcription = result.Text
		if result.Title != nil {
			video.Title = *result.Title
		} else {
			video.Title = video.URL
		}

		logger.Info().
			Int("transcription_length", len(video.Transcription)).
			Str("title", video.Title).
			Str("status", string(video.Status)).
			Msg("Updated video with transcription")
	}

	video.UpdatedAt = time.Now()

	if err := s.repo.Save(ctx, video); err != nil {
		logger.Error().Err(err).Msg("Failed to save transcription result")
	} else {
		logger.Info().
			Int("transcription_length", len(video.Transcription)).
			Str("title", video.Title).
			Str("status", string(video.Status)).
			Time("updated_at", video.UpdatedAt).
			Msg("Saved video with transcription")
	}
}

func (s *service) updateVideoWithTranscription(video *models.Video, text string, title string) {
	video.Status = models.StatusCompleted
	video.Transcription = text
	video.Title = title
	video.UpdatedAt = time.Now()

	if err := s.repo.Save(context.Background(), video); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save video with subtitles")
	}
}
