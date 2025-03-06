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
	ID                  string    `json:"id"`
	URL                 string    `json:"url"`
	Title               string    `json:"title"`
	Transcription       string    `json:"transcription"`
	Status              Status    `json:"status"`
	Error               string    `json:"error,omitempty"`
	Language            string    `json:"language,omitempty"`
	LanguageProbability float64   `json:"language_probability,omitempty"`
	ModelName           string    `json:"model_name,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
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
	ID                  string `json:"id"`
	URL                 string `json:"url"`
	Status              Status `json:"status"`
	Transcription       string `json:"transcription,omitempty"`
	Title               string `json:"title,omitempty"`
	Error               string `json:"error,omitempty"`
	Language            string `json:"language,omitempty"`
	LanguageProbability float64 `json:"language_probability,omitempty"`
	ModelName           string `json:"model_name,omitempty"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

// NewVideoResponse creates a response from a video model
func NewVideoResponse(v *Video) *VideoResponse {
	return &VideoResponse{
		ID:                  v.ID,
		URL:                 v.URL,
		Status:              v.Status,
		Transcription:       v.Transcription,
		Title:               v.Title,
		Error:               v.Error,
		Language:            v.Language,
		LanguageProbability: v.LanguageProbability,
		ModelName:           v.ModelName,
		CreatedAt:           v.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           v.UpdatedAt.Format(time.RFC3339),
	}
}
