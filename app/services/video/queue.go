package video

import (
	"context"
	"errors"
	"sync"
	"time"
	"yt-text/models"
	
	"github.com/rs/zerolog"
)

// Common errors
var (
	ErrQueueFull = errors.New("job queue is full")
)

type JobQueue struct {
	jobs         chan *TranscriptionJob
	activeJobs   map[string]*TranscriptionJob
	priorityJobs chan *TranscriptionJob
	workerCount  int
	maxJobs      int
	mu           sync.Mutex
	quit         chan struct{}
}

type TranscriptionJob struct {
	ID         string
	URL        string
	Video      *models.Video
	ctx        context.Context
	cancelFunc context.CancelFunc
	result     chan error
	priority   int
	startTime  time.Time
}

func NewJobQueue(workerCount, maxQueueSize int) *JobQueue {
	q := &JobQueue{
		jobs:         make(chan *TranscriptionJob, maxQueueSize),
		priorityJobs: make(chan *TranscriptionJob, 5), // Small buffer for priority jobs
		activeJobs:   make(map[string]*TranscriptionJob),
		workerCount:  workerCount,
		maxJobs:      maxQueueSize,
		quit:         make(chan struct{}),
	}

	return q
}

// Start begins processing jobs
func (q *JobQueue) Start(processFunc func(context.Context, *models.Video) error) {
	// Start workers
	for i := 0; i < q.workerCount; i++ {
		go q.worker(i, processFunc)
	}

	// Start monitoring for hung jobs
	go q.monitorHungJobs()
}

// Submit adds a job to the queue
func (q *JobQueue) Submit(ctx context.Context, video *models.Video, priority int) (string, <-chan error, error) {
	jobCtx, cancel := context.WithCancel(ctx)
	
	job := &TranscriptionJob{
		ID:         video.ID,
		URL:        video.URL,
		Video:      video,
		ctx:        jobCtx,
		cancelFunc: cancel,
		result:     make(chan error, 1),
		priority:   priority,
		startTime:  time.Now(),
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if we're at capacity
	if len(q.jobs) >= q.maxJobs {
		cancel() // Clean up context
		return "", nil, ErrQueueFull
	}

	// Add to active jobs map
	q.activeJobs[job.ID] = job

	// Send to appropriate queue
	if priority > 0 {
		select {
		case q.priorityJobs <- job:
			// Successfully queued
		default:
			// Priority queue full, use regular queue
			q.jobs <- job
		}
	} else {
		q.jobs <- job
	}

	return job.ID, job.result, nil
}

// Cancel attempts to cancel a job
func (q *JobQueue) Cancel(jobID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, exists := q.activeJobs[jobID]
	if !exists {
		return false
	}

	// Call the cancel function to cancel the context
	job.cancelFunc()
	return true
}

// GetJobStatus returns the status of a job
func (q *JobQueue) GetJobStatus(jobID string) (bool, time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, exists := q.activeJobs[jobID]
	if !exists {
		return false, time.Time{}
	}

	return true, job.startTime
}

// worker processes jobs from the queue
func (q *JobQueue) worker(id int, processFunc func(context.Context, *models.Video) error) {
	log := zerolog.New(zerolog.ConsoleWriter{Out: zerolog.NewConsoleWriter()}).
		With().
		Timestamp().
		Int("worker_id", id).
		Logger()
		
	log.Info().Msg("Starting worker")

	for {
		var job *TranscriptionJob
		// First check priority queue, then regular queue
		select {
		case <-q.quit:
			log.Info().Msg("Worker shutting down")
			return
		case job = <-q.priorityJobs:
			// Got a priority job
			log.Info().Str("job_id", job.ID).Msg("Processing priority job")
		default:
			// No priority job, try regular queue
			select {
			case <-q.quit:
				log.Info().Msg("Worker shutting down")
				return
			case job = <-q.jobs:
				// Got a regular job
				log.Info().Str("job_id", job.ID).Msg("Processing regular job")
			}
		}

		// Process the job
		log.Info().Str("job_id", job.ID).Msg("Started processing job")
		startTime := time.Now()
		err := processFunc(job.ctx, job.Video)
		duration := time.Since(startTime)
		
		if err != nil {
			log.Error().Err(err).Str("job_id", job.ID).Int64("duration_ms", duration.Milliseconds()).Msg("Job processing failed")
		} else {
			log.Info().Str("job_id", job.ID).Int64("duration_ms", duration.Milliseconds()).Msg("Job processing succeeded")
		}

		// Send result
		select {
		case job.result <- err:
			// Result sent
			log.Debug().Str("job_id", job.ID).Msg("Job result sent to client")
		default:
			// No one listening for result
			log.Warn().Str("job_id", job.ID).Msg("No listener for job result")
		}

		// Remove from active jobs
		q.mu.Lock()
		delete(q.activeJobs, job.ID)
		q.mu.Unlock()
		
		log.Info().Str("job_id", job.ID).Msg("Job completed and removed from active jobs")
	}
}

// Close shuts down the queue
func (q *JobQueue) Close() {
	close(q.quit)
	
	// Cancel all active jobs
	q.mu.Lock()
	defer q.mu.Unlock()
	
	for _, job := range q.activeJobs {
		job.cancelFunc()
	}
}

// monitorHungJobs periodically checks for hung jobs
func (q *JobQueue) monitorHungJobs() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-q.quit:
			return
		case <-ticker.C:
			q.checkHungJobs()
		}
	}
}

// checkHungJobs looks for jobs that have been running too long
func (q *JobQueue) checkHungJobs() {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	hungTimeout := 30 * time.Minute
	
	log := zerolog.New(zerolog.ConsoleWriter{Out: zerolog.NewConsoleWriter()}).
		With().
		Timestamp().
		Logger()

	for id, job := range q.activeJobs {
		if now.Sub(job.startTime) > hungTimeout {
			log.Warn().
				Str("job_id", id).
				Dur("duration", now.Sub(job.startTime)).
				Msg("Found hung job")
			// Log but don't automatically cancel - that should be a separate policy decision
		}
	}
}