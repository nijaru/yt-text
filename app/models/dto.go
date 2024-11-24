package models

// VideoRequest represents the incoming request for video transcription
type VideoRequest struct {
	URL         string    `json:"url"`
	ModelConfig ModelInfo `json:"model_config,omitempty"`
}

// VideoResponse represents the API response
type VideoResponse struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Status        Status    `json:"status"`
	Transcription string    `json:"transcription,omitempty"`
	Summary       string    `json:"summary,omitempty"`
	ModelInfo     ModelInfo `json:"model_info"`
	Progress      Progress  `json:"progress,omitempty"`
	Error         string    `json:"error,omitempty"`
}

// NewVideoResponse creates a response from a video model
func NewVideoResponse(v *Video) *VideoResponse {
	return &VideoResponse{
		ID:            v.ID,
		URL:           v.URL,
		Status:        v.Status,
		Transcription: v.Transcription,
		Summary:       v.Summary,
		ModelInfo:     v.ModelInfo,
		Progress:      v.Progress,
		Error:         v.Error,
	}
}
