package models

import (
	"time"

	"github.com/sirupsen/logrus"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

type Video struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Status        Status    `json:"status"`
	Transcription string    `json:"transcription,omitempty"`
	Summary       string    `json:"summary,omitempty"`
	ModelInfo     ModelInfo `json:"model_info"`
	Error         string    `json:"error,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ModelInfo struct {
	Name         string  `json:"name"`
	SummaryModel string  `json:"summary_model,omitempty"`
	Temperature  float64 `json:"temperature"`
	BeamSize     int     `json:"beam_size"`
	BestOf       int     `json:"best_of"`
	Duration     float64 `json:"duration,omitempty"`
	FileSize     int64   `json:"file_size,omitempty"`
	Format       string  `json:"format,omitempty"`
}

// NewVideo creates a new video with default values
func NewVideo(url string) *Video {
	now := time.Now()
	return &Video{
		URL:       url,
		Status:    StatusPending,
		ModelInfo: DefaultModelInfo(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// DefaultModelInfo returns default model settings
func DefaultModelInfo() ModelInfo {
	return ModelInfo{
		Name:        "base.en",
		Temperature: 0.2,
		BeamSize:    2,
		BestOf:      1,
	}
}

// Status check methods
func (v *Video) IsPending() bool    { return v.Status == StatusPending }
func (v *Video) IsProcessing() bool { return v.Status == StatusProcessing }
func (v *Video) IsCompleted() bool  { return v.Status == StatusCompleted }
func (v *Video) IsFailed() bool     { return v.Status == StatusFailed }
func (v *Video) IsCancelled() bool  { return v.Status == StatusCancelled }

// IsStale checks if the job has been stuck in processing for too long
func (v *Video) IsStale(timeout time.Duration) bool {
	if v.Status != StatusProcessing {
		return false
	}
	timeSinceUpdate := time.Since(v.UpdatedAt)
	isStale := timeSinceUpdate > timeout
	if isStale {
		log := logrus.WithFields(logrus.Fields{
			"video_id":          v.ID,
			"status":            v.Status,
			"updated_at":        v.UpdatedAt,
			"time_since_update": timeSinceUpdate,
			"timeout":           timeout,
		})
		log.Info("Video is considered stale")
	}
	return isStale
}

// Update methods
func (v *Video) UpdateStatus(status Status) {
	v.Status = status
	v.UpdatedAt = time.Now()
}

func (v *Video) SetError(msg string) {
	v.Status = StatusFailed
	v.Error = msg
	v.UpdatedAt = time.Now()
}

func (v *Video) SetTranscription(text string) {
	v.Transcription = text
	v.Status = StatusCompleted
	v.UpdatedAt = time.Now()
}

func (v *Video) SetSummary(summary string, modelName string) {
	v.Summary = summary
	v.ModelInfo.SummaryModel = modelName
	v.UpdatedAt = time.Now()
}
