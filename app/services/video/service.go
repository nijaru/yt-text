package video

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nijaru/yt-text/errors"
	"github.com/nijaru/yt-text/models"
	"github.com/nijaru/yt-text/scripts"
	"github.com/nijaru/yt-text/validation"
	"github.com/sirupsen/logrus"
)

type service struct {
    repo         Repository
    scripts      *scripts.ScriptRunner
    validator    *validation.Validator
    config       Config
    activeJobs   sync.Map
    jobSemaphore chan struct{}
    shutdown     chan struct{}
    logger       *logrus.Logger
}

func NewService(
    repo Repository,
    scriptRunner *scripts.ScriptRunner,
    validator *validation.Validator,
    config Config,
) Service {
    s := &service{
        repo:         repo,
        scripts:      scriptRunner,
        validator:    validator,
        config:       config,
        jobSemaphore: make(chan struct{}, config.MaxConcurrentJobs),
        shutdown:     make(chan struct{}),
        logger:       logrus.StandardLogger(),
    }

    // Start cleanup routine
    go s.cleanupRoutine()

    return s
}

func (s *service) Create(ctx context.Context, url string) (*models.Video, error) {
    const op = "VideoService.Create"
    logger := s.logger.WithContext(ctx).WithField("url", url)

    // Basic URL validation
    if err := s.validator.BasicURLValidation(url); err != nil {
        logger.WithError(err).Warn("Invalid URL format")
        return nil, err
    }

    // Check for existing video
    existing, err := s.repo.GetByURL(ctx, url)
    if err != nil && !errors.IsNotFound(err) {
        logger.WithError(err).Error("Failed to check existing video")
        return nil, errors.Internal(op, err, "Failed to check existing video")
    }
    if existing != nil {
        return existing, nil
    }

    // Deep validation with Python script
    info, err := s.scripts.Validate(ctx, url)
    if err != nil {
        logger.WithError(err).Error("Failed to validate video")
        return nil, errors.InvalidInput(op, err, "Invalid video URL")
    }

    if !info.Valid {
        return nil, errors.InvalidInput(op, nil, info.Error)
    }

    // Create new video record
    video := &models.Video{
        ID:        uuid.New().String(),
        URL:       url,
        Status:    models.StatusPending,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
        ModelInfo: models.ModelInfo{
            Duration: info.Duration,
            FileSize: info.FileSize,
            Format:   info.Format,
        },
    }

    if err := s.repo.Create(ctx, video); err != nil {
        logger.WithError(err).Error("Failed to create video record")
        return nil, errors.Internal(op, err, "Failed to create video record")
    }

    // Start processing in background
    go func() {
        processCtx, cancel := context.WithTimeout(context.Background(), s.config.ProcessTimeout)
        defer cancel()

        if err := s.Process(processCtx, video.ID); err != nil {
            logger.WithError(err).Error("Failed to process video")
        }
    }()

    return video, nil
}

func (s *service) Process(ctx context.Context, id string) error {
    const op = "VideoService.Process"
    logger := s.logger.WithContext(ctx).WithField("video_id", id)

    // Acquire semaphore
    select {
    case s.jobSemaphore <- struct{}{}:
        defer func() { <-s.jobSemaphore }()
    case <-ctx.Done():
        return errors.Internal(op, ctx.Err(), "Context cancelled")
    case <-s.shutdown:
        return errors.Internal(op, nil, "Service is shutting down")
    }

    // Get video
    video, err := s.repo.Get(ctx, id)
    if err != nil {
        return errors.Internal(op, err, "Failed to get video")
    }

    // Store job in active jobs
    cancelCtx, cancel := context.WithCancel(ctx)
    s.activeJobs.Store(id, cancel)
    defer func() {
        cancel()
        s.activeJobs.Delete(id)
    }()

    // Update status to processing
    video.Status = models.StatusProcessing
    video.UpdatedAt = time.Now()
    if err := s.repo.Update(ctx, video); err != nil {
        return errors.Internal(op, err, "Failed to update video status")
    }

    // Start transcription with progress monitoring
    opts := map[string]string{
        "model":    video.ModelInfo.Name,
        "language": "en",
    }

    result, progress, err := s.scripts.Transcribe(cancelCtx, video.URL, opts)
    if err != nil {
        video.Status = models.StatusFailed
        video.Error = err.Error()
        video.UpdatedAt = time.Now()
        _ = s.repo.Update(ctx, video)
        return errors.Internal(op, err, "Transcription failed")
    }

    // Monitor progress
    go func() {
        for p := range progress {
            select {
            case <-cancelCtx.Done():
                return
            default:
                video.Progress = models.Progress{
                    Percent: p.Percent,
                    Stage:   p.Stage,
                    Message: p.Message,
                }
                video.UpdatedAt = time.Now()
                if err := s.repo.Update(ctx, video); err != nil {
                    logger.WithError(err).Error("Failed to update progress")
                }
            }
        }
    }()

    // Update video with transcription result
    video.Status = models.StatusCompleted
    video.Transcription = result.Text
    video.ModelInfo.Name = result.ModelName
    video.UpdatedAt = time.Now()

    if err := s.repo.Update(ctx, video); err != nil {
        return errors.Internal(op, err, "Failed to save transcription")
    }

    return nil
}

func (s *service) Get(ctx context.Context, id string) (*models.Video, error) {
    const op = "VideoService.Get"

    if id == "" {
        return nil, errors.InvalidInput(op, nil, "ID is required")
    }

    video, err := s.repo.Get(ctx, id)
    if err != nil {
        if errors.IsNotFound(err) {
            return nil, errors.NotFound(op, err, "Video not found")
        }
        return nil, errors.Internal(op, err, "Failed to get video")
    }

    return video, nil
}

func (s *service) GetByURL(ctx context.Context, url string) (*models.Video, error) {
    const op = "VideoService.GetByURL"

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

    return video, nil
}

func (s *service) Cancel(ctx context.Context, id string) error {
    const op = "VideoService.Cancel"

    if id == "" {
        return errors.InvalidInput(op, nil, "ID is required")
    }

    // Get cancel function from active jobs
    if cancelFunc, ok := s.activeJobs.Load(id); ok {
        cancel := cancelFunc.(context.CancelFunc)
        cancel()
        s.activeJobs.Delete(id)

        // Update video status
        video, err := s.repo.Get(ctx, id)
        if err != nil {
            return errors.Internal(op, err, "Failed to get video")
        }

        video.Status = models.StatusCancelled
        video.UpdatedAt = time.Now()

        if err := s.repo.Update(ctx, video); err != nil {
            return errors.Internal(op, err, "Failed to update video status")
        }

        return nil
    }

    return errors.NotFound(op, nil, "No active job found for video")
}

func (s *service) cleanupRoutine() {
    ticker := time.NewTicker(s.config.CleanupInterval)
    defer ticker.Stop()

    for {
        select {
        case <-s.shutdown:
            return
        case <-ticker.C:
            s.cleanup()
        }
    }
}

func (s *service) cleanup() {
    ctx := context.Background()

    s.activeJobs.Range(func(key, value interface{}) bool {
        id := key.(string)

        video, err := s.repo.Get(ctx, id)
        if err != nil || (video != nil && video.IsStale(s.config.ProcessTimeout)) {
            if cancel, ok := value.(context.CancelFunc); ok {
                cancel()
            }
            s.activeJobs.Delete(id)

            if video != nil {
                video.Status = models.StatusFailed
                video.Error = "Processing timed out"
                video.UpdatedAt = time.Now()
                _ = s.repo.Update(ctx, video)
            }
        }
        return true
    })
}

func (s *service) Shutdown(ctx context.Context) error {
    close(s.shutdown)

    // Cancel all active jobs
    s.activeJobs.Range(func(key, value interface{}) bool {
        if cancel, ok := value.(context.CancelFunc); ok {
            cancel()
        }
        return true
    })

    // Wait for jobs to complete or context to cancel
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        return nil
    }
}
