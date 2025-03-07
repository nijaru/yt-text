package video

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"yt-text/errors"
	"yt-text/models"
	"yt-text/repository"
	"yt-text/scripts"
	"yt-text/validation"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Repository = repository.VideoRepository

type service struct {
	repo      Repository
	scripts   scripts.TranscriptionClient
	validator *validation.Validator
	config    Config
	logger    zerolog.Logger
	jobQueue  *JobQueue
}

func NewService(
	repo Repository,
	transcriptionClient scripts.TranscriptionClient,
	validator *validation.Validator,
	config Config,
) Service {
	s := &service{
		repo:      repo,
		scripts:   transcriptionClient,
		validator: validator,
		config:    config,
		logger:    zerolog.New(zerolog.NewConsoleWriter()),
	}
	
	// Initialize job queue with worker pool
	workerCount := config.WorkerPoolSize
	if workerCount <= 0 {
		workerCount = 3 // Default to 3 workers if not specified
	}
	
	s.jobQueue = NewJobQueue(workerCount, 50) // Allow up to 50 queued jobs
	s.jobQueue.Start(s.doProcessVideo)
	
	return s
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

	// Submit job to the queue with priority 0 (normal)
	jobID, resultChan, err := s.jobQueue.Submit(ctx, video, 0)
	if err != nil {
		s.logger.Error().Err(err).Str("video_id", video.ID).Msg("Failed to submit job to queue")
		
		// Fall back to direct processing if queue is full
		if errors.Is(err, ErrQueueFull) {
			s.logger.Warn().Str("video_id", video.ID).Msg("Queue full, falling back to direct processing")
			go s.processVideo(video)
		} else {
			// For other errors, update status
			video.Status = models.StatusFailed
			video.Error = "Failed to queue job: " + err.Error()
			video.UpdatedAt = time.Now()
			
			if saveErr := s.repo.Save(ctx, video); saveErr != nil {
				s.logger.Error().Err(saveErr).Msg("Failed to save error status")
			}
			
			return nil, errors.Internal(op, err, "Failed to queue transcription job")
		}
	} else {
		// Start a goroutine to handle the job result
		go func() {
			jobErr := <-resultChan
			if jobErr != nil {
				s.logger.Error().Err(jobErr).Str("job_id", jobID).Msg("Job completed with error")
				
				// We don't need to update the video status here as the worker will do that
			} else {
				s.logger.Info().Str("job_id", jobID).Msg("Job completed successfully")
			}
		}()
	}

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

// extractYouTubeID attempts to extract a video ID from various YouTube URL formats
// This is a simple implementation that handles the most common formats
func extractYouTubeID(url string) (string, error) {
	// Handle youtu.be URLs
	if strings.Contains(url, "youtu.be/") {
		parts := strings.Split(url, "youtu.be/")
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid youtu.be URL format")
		}
		
		id := strings.Split(parts[1], "?")[0]
		return id, nil
	}
	
	// Handle youtube.com URLs
	if strings.Contains(url, "youtube.com/watch") {
		// Extract v parameter
		startIdx := strings.Index(url, "v=")
		if startIdx == -1 {
			return "", fmt.Errorf("missing video ID in youtube.com URL")
		}
		
		// Start after v=
		startIdx += 2
		
		// Find end of ID (either & or end of string)
		endIdx := strings.Index(url[startIdx:], "&")
		if endIdx == -1 {
			// No &, take the rest of the string
			return url[startIdx:], nil
		}
		
		// Include startIdx in endIdx calculation
		return url[startIdx : startIdx+endIdx], nil
	}
	
	// Handle other formats like youtube.com/embed/ID
	if strings.Contains(url, "youtube.com/embed/") {
		parts := strings.Split(url, "youtube.com/embed/")
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid youtube embed URL format")
		}
		
		id := strings.Split(parts[1], "?")[0]
		return id, nil
	}
	
	return "", fmt.Errorf("unsupported YouTube URL format")
}

func (s *service) processVideo(video *models.Video) {
	logger := s.logger.With().Str("video_id", video.ID).Logger()
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), s.config.ProcessTimeout)
	defer cancel()
	
	// Execute the processing directly
	err := s.doProcessVideo(ctx, video)
	
	if err != nil {
		logger.Error().Err(err).Msg("Transcription processing failed")
		
		// Handle specific error cases here if needed
		// Note: The doProcessVideo function already updates the video status in the DB
	} else {
		logger.Info().Msg("Transcription processing completed successfully")
	}
}

// storeTranscriptionInFile stores large transcription text in a file
func (s *service) storeTranscriptionInFile(videoID string, text string) error {
	filePath := s.getTranscriptionFilePath(videoID)
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create transcription directory: %w", err)
	}
	
	// Write transcription to file
	if err := os.WriteFile(filePath, []byte(text), 0640); err != nil {
		return fmt.Errorf("failed to write transcription file: %w", err)
	}
	
	return nil
}

// getTranscriptionFilePath returns the file path for a transcription
func (s *service) getTranscriptionFilePath(videoID string) string {
	// Use configured path or fallback to default
	basePath := s.config.TranscriptionPath
	if basePath == "" {
		basePath = filepath.Join(s.config.TempDir, "transcriptions")
	}
	
	// Create a directory structure like: basePath/xx/xx/videoID.txt
	// (where xx/xx are the first two pairs of characters from the ID)
	// This helps prevent too many files in a single directory
	if len(videoID) >= 4 {
		return filepath.Join(basePath, videoID[:2], videoID[2:4], videoID+".txt")
	}
	
	// Fallback for short IDs
	return filepath.Join(basePath, videoID+".txt")
}

// GetTranscriptionText retrieves transcription content, either from DB or file
func (s *service) GetTranscriptionText(ctx context.Context, video *models.Video) (string, error) {
	const op = "VideoService.GetTranscriptionText"
	
	// If transcription is stored in DB, return it directly
	if video.Transcription != "" {
		return video.Transcription, nil
	}
	
	// If no path, error
	if video.TranscriptionPath == "" {
		return "", errors.Internal(op, nil, "No transcription available")
	}
	
	// Read from file
	data, err := os.ReadFile(video.TranscriptionPath)
	if err != nil {
		return "", errors.Internal(op, err, "Failed to read transcription file")
	}
	
	// Update the last_accessed time
	video.LastAccessed = time.Now()
	
	// Do this in a new context to avoid dependency on the parent
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer updateCancel()
	
	if saveErr := s.repo.Save(updateCtx, video); saveErr != nil {
		// Log but don't fail the request
		s.logger.Error().Err(saveErr).Str("video_id", video.ID).
			Msg("Failed to update last_accessed timestamp")
	}
	
	return string(data), nil
}

// CleanupExpiredTranscriptions removes transcriptions that haven't been accessed recently
func (s *service) CleanupExpiredTranscriptions(ctx context.Context) error {
	const op = "VideoService.CleanupExpiredTranscriptions"
	
	// Calculate the cutoff date based on configuration
	cutoffDays := s.config.CleanupAfterDays
	if cutoffDays <= 0 {
		cutoffDays = 90 // Default to 90 days if not specified
	}
	
	cutoff := time.Now().AddDate(0, 0, -cutoffDays)
	
	// Find expired videos
	expired, err := s.repo.FindExpiredVideos(ctx, cutoff)
	if err != nil {
		return errors.Internal(op, err, "Failed to find expired videos")
	}
	
	s.logger.Info().Int("count", len(expired)).Msg("Found expired transcriptions to clean up")
	
	// Delete each expired video
	for _, video := range expired {
		// Delete associated file if exists
		if video.TranscriptionPath != "" {
			if err := os.Remove(video.TranscriptionPath); err != nil {
				if !os.IsNotExist(err) {
					s.logger.Error().Err(err).Str("path", video.TranscriptionPath).
						Msg("Failed to delete transcription file")
				}
			}
		}
		
		// Delete from database
		if err := s.repo.DeleteExpiredVideo(ctx, video.ID); err != nil {
			s.logger.Error().Err(err).Str("id", video.ID).
				Msg("Failed to delete expired video from database")
			continue
		}
		
		s.logger.Info().Str("id", video.ID).Msg("Deleted expired transcription")
	}
	
	return nil
}

// CancelJob attempts to cancel a job in progress
func (s *service) CancelJob(jobID string) bool {
	return s.jobQueue.Cancel(jobID)
}

// GetJobStatus returns information about a job's status
func (s *service) GetJobStatus(jobID string) (bool, time.Time) {
	return s.jobQueue.GetJobStatus(jobID)
}

// GetRepository returns the underlying repository
func (s *service) GetRepository() Repository {
	return s.repo
}

// Close shuts down the service and its job queue
func (s *service) Close() {
	s.jobQueue.Close()
}

func (s *service) doProcessVideo(ctx context.Context, video *models.Video) error {
	logger := s.logger.With().Str("video_id", video.ID).Logger()
	logger.Info().Msg("Starting transcription process")

	// Check if context is already canceled before starting
	if ctx.Err() != nil {
		logger.Warn().Err(ctx.Err()).Msg("Context already canceled before transcription started")
		video.Status = models.StatusFailed
		video.Error = "Processing timed out before it could start"
		video.UpdatedAt = time.Now()
		
		// Use a new background context for saving the error state
		saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer saveCancel()
		
		if err := s.repo.Save(saveCtx, video); err != nil {
			logger.Error().Err(err).Msg("Failed to save error state")
		}
		return fmt.Errorf("context canceled before transcription could start: %w", ctx.Err())
	}

	// Mark as processing
	video.Status = models.StatusProcessing
	video.UpdatedAt = time.Now()
	
	// Update processing status
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := s.repo.Save(saveCtx, video); err != nil {
		logger.Error().Err(err).Msg("Failed to save processing status")
	}
	saveCancel()

	// Extract YouTube ID
	videoID, err := extractYouTubeID(video.URL)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to extract YouTube ID, will skip caption API")
	}

	var result *scripts.TranscriptionResult
	var transcriptionErr error

	// Only try to fetch captions if we have a valid YouTube ID and API key
	if videoID != "" && s.config.YouTubeAPIKey != "" {
		// Try YouTube captions first
		logger.Info().Str("video_id", videoID).Msg("Attempting to fetch YouTube captions")
		result, transcriptionErr = s.scripts.FetchYouTubeCaptions(ctx, videoID, s.config.YouTubeAPIKey)
		
		if transcriptionErr == nil && result != nil && result.Text != "" {
			logger.Info().Msg("Successfully retrieved YouTube captions")
			// Set the source to YouTube API
			video.Source = models.SourceYouTubeAPI
		} else {
			// Log the error but continue to Whisper as fallback
			logger.Info().Err(transcriptionErr).Msg("Failed to get YouTube captions, falling back to Whisper")
		}
	}

	// If YouTube captions failed or were not available, use Whisper
	if result == nil || result.Text == "" {
		// Set up transcription options
		opts := map[string]string{
			"model": s.config.DefaultModel,
			"chunk_length": "120", // Default 2-minute chunks
		}

		// Perform transcription
		logger.Info().Msg("Starting Whisper transcription")
		result, transcriptionErr = s.scripts.Transcribe(ctx, video.URL, opts, true)
		
		// Set the source to Whisper
		video.Source = models.SourceWhisper
	}
	
	// Check if context was canceled during transcription
	if ctx.Err() != nil {
		logger.Warn().Err(ctx.Err()).Msg("Context canceled during transcription")
		video.Status = models.StatusFailed
		video.Error = "Processing timed out"
		video.UpdatedAt = time.Now()
		
		// Use a new background context for saving the error state
		saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer saveCancel()
		
		if err := s.repo.Save(saveCtx, video); err != nil {
			logger.Error().Err(err).Msg("Failed to save timeout state")
		}
		return fmt.Errorf("context canceled during transcription: %w", ctx.Err())
	}
	
	if transcriptionErr != nil {
		logger.Error().Err(transcriptionErr).Msg("Transcription failed")
		video.Status = models.StatusFailed
		video.Error = transcriptionErr.Error()
	} else if result.Text == "" {
		logger.Error().Msg("Empty transcription returned")
		video.Status = models.StatusFailed
		video.Error = "No transcription text was generated"
	} else {
		logger.Info().Msg("Transcription completed successfully")
		video.Status = models.StatusCompleted
		
		// Implement hybrid storage approach
		if len(result.Text) > int(s.config.StorageSizeThreshold) {
			// For large transcriptions, store in a file
			err := s.storeTranscriptionInFile(video.ID, result.Text)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to store transcription in file")
				video.Status = models.StatusFailed
				video.Error = "Failed to store large transcription: " + err.Error()
				
				// Return error about storage failure
				finalErr := fmt.Errorf("failed to store transcription: %w", err)
				
				// Update video record with error
				finalCtx, finalCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer finalCancel()
				
				if saveErr := s.repo.Save(finalCtx, video); saveErr != nil {
					logger.Error().Err(saveErr).Msg("Failed to save error state")
				}
				
				return finalErr
			} else {
				// Set the path but leave transcription empty in DB
				video.TranscriptionPath = s.getTranscriptionFilePath(video.ID)
				video.Transcription = "" // Clear DB field to save space
			}
		} else {
			// For small transcriptions, store directly in DB
			video.Transcription = result.Text
			video.TranscriptionPath = "" // Clear any existing path
		}
		
		video.ModelName = result.ModelName
		
		if result.Title != nil {
			video.Title = *result.Title
		} else {
			video.Title = video.URL
		}
		
		// Add language information if available
		if result.Language != nil {
			video.Language = *result.Language
			video.LanguageProbability = result.LanguageProbability
		}

		// Add debug logging
		logger.Info().
			Int("transcription_length", len(result.Text)).
			Str("title", video.Title).
			Str("model", video.ModelName).
			Str("language", video.Language).
			Float64("language_probability", video.LanguageProbability).
			Str("source", string(video.Source)).
			Str("storage", video.TranscriptionPath != "" ? "file" : "database").
			Str("status", string(video.Status)).
			Msg("Updated video with transcription")
	}

	video.UpdatedAt = time.Now()
	video.LastAccessed = time.Now()

	// Use a new context for saving to ensure it completes even if original context is canceled
	finalCtx, finalCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finalCancel()
	
	// Update video record
	if err := s.repo.Save(finalCtx, video); err != nil {
		logger.Error().Err(err).Msg("Failed to save transcription result")
		return fmt.Errorf("failed to save transcription result: %w", err)
	} else {
		// Add debug logging after save
		logger.Info().
			Str("title", video.Title).
			Str("model", video.ModelName).
			Str("language", video.Language).
			Str("status", string(video.Status)).
			Str("source", string(video.Source)).
			Time("updated_at", video.UpdatedAt).
			Msg("Saved video with transcription")
	}
	
	// Return any error from transcription process
	if transcriptionErr != nil {
		return transcriptionErr
	}
	
	return nil
}