package video

import (
	"context"
	"errors"
	"sync"
	"time"
	"yt-text/logger"
	"yt-text/models"
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
	logger.Info("Starting worker", "worker_id", id)

	for {
		var job *TranscriptionJob
		// First check priority queue, then regular queue
		select {
		case <-q.quit:
			logger.Info("Worker shutting down", "worker_id", id)
			return
		case job = <-q.priorityJobs:
			// Got a priority job
			logger.Info("Processing priority job", "worker_id", id, "job_id", job.ID)
		default:
			// No priority job, try regular queue
			select {
			case <-q.quit:
				logger.Info("Worker shutting down", "worker_id", id)
				return
			case job = <-q.jobs:
				// Got a regular job
				logger.Info("Processing regular job", "worker_id", id, "job_id", job.ID)
			}
		}

		// Process the job
		logger.Info("Started processing job", "worker_id", id, "job_id", job.ID)
		startTime := time.Now()
		err := processFunc(job.ctx, job.Video)
		duration := time.Since(startTime)
		
		if err != nil {
			logger.Error("Job processing failed", "worker_id", id, "job_id", job.ID, "error", err, "duration_ms", duration.Milliseconds())
		} else {
			logger.Info("Job processing succeeded", "worker_id", id, "job_id", job.ID, "duration_ms", duration.Milliseconds())
		}

		// Send result
		select {
		case job.result <- err:
			// Result sent
			logger.Debug("Job result sent to client", "worker_id", id, "job_id", job.ID)
		default:
			// No one listening for result
			logger.Warn("No listener for job result", "worker_id", id, "job_id", job.ID)
		}

		// Remove from active jobs
		q.mu.Lock()
		delete(q.activeJobs, job.ID)
		q.mu.Unlock()
		
		logger.Info("Job completed and removed from active jobs", "worker_id", id, "job_id", job.ID)
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

	for id, job := range q.activeJobs {
		if now.Sub(job.startTime) > hungTimeout {
			logger.Warn("Found hung job", "job_id", id, "duration", now.Sub(job.startTime))
			// Log but don't automatically cancel - that should be a separate policy decision
		}
	}
}