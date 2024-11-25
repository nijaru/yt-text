package models

import (
	"time"
)

type Status string

const (
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

type Video struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Status        Status    `json:"status"`
	Transcription string    `json:"transcription,omitempty"`
	Error         string    `json:"error,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Status check methods
func (v *Video) IsProcessing() bool { return v.Status == StatusProcessing }
func (v *Video) IsCompleted() bool  { return v.Status == StatusCompleted }
func (v *Video) IsFailed() bool     { return v.Status == StatusFailed }

// IsStale checks if the job has been stuck in processing for too long
func (v *Video) IsStale(timeout time.Duration) bool {
	if v.Status != StatusProcessing {
		return false
	}
	return time.Since(v.UpdatedAt) > timeout
}

// VideoResponse represents the API response
type VideoResponse struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	Status        Status `json:"status"`
	Transcription string `json:"transcription,omitempty"`
	Error         string `json:"error,omitempty"`
}

// NewVideoResponse creates a response from a video model
func NewVideoResponse(v *Video) *VideoResponse {
	return &VideoResponse{
		ID:            v.ID,
		URL:           v.URL,
		Status:        v.Status,
		Transcription: v.Transcription,
		Error:         v.Error,
	}
}
