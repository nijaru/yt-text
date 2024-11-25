package api

import (
	"net/http"

	"github.com/nijaru/yt-text/errors"
	"github.com/nijaru/yt-text/models"
	"github.com/nijaru/yt-text/services/video"
	"github.com/nijaru/yt-text/validation"
	"github.com/sirupsen/logrus"
)

type VideoHandler struct {
	service   video.Service
	validator *validation.Validator
	logger    *logrus.Logger
}

type createTranscriptionRequest struct {
	URL       string            `json:"url"`
	ModelInfo *models.ModelInfo `json:"model_info,omitempty"`
}

func NewVideoHandler(service video.Service, validator *validation.Validator) *VideoHandler {
	return &VideoHandler{
		service:   service,
		validator: validator,
		logger:    logrus.StandardLogger(),
	}
}

// HandleCreateTranscription handles POST /api/v1/transcribe
func (h *VideoHandler) HandleCreateTranscription(w http.ResponseWriter, r *http.Request) {
	const op = "VideoHandler.HandleCreateTranscription"
	logger := h.logger.WithContext(r.Context())

	// Validate request format
	if err := h.validator.ValidateRequest(r, validation.RequestValidationOpts{
		MaxContentLength: 1024 * 1024, // 1MB
		AllowedMethods:   []string{http.MethodPost},
		RequireJSON:      false,
	}); err != nil {
		respondError(w, r, err)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		logger.WithError(err).Error("Failed to parse form data")
		respondError(w, r, errors.InvalidInput(op, err, "Failed to parse form data"))
		return
	}

	url := r.FormValue("url")
	logger.WithField("url", url).Info("Received transcription request")

	if url == "" {
		respondError(w, r, errors.InvalidInput(op, nil, "URL is required"))
		return
	}

	video, err := h.service.Create(r.Context(), url)
	if err != nil {
		logger.WithError(err).Error("Failed to create transcription")
		respondError(w, r, err)
		return
	}

	logger.WithFields(logrus.Fields{
		"video_id": video.ID,
		"status":   video.Status,
	}).Info("Transcription job created")

	// Ensure we're returning a proper response
	response := map[string]interface{}{
		"id":     video.ID,
		"status": string(video.Status),
		"url":    video.URL,
	}

	respondJSON(w, r, http.StatusAccepted, response)
}

// HandleGetTranscription handles GET /api/v1/transcription
func (h *VideoHandler) HandleGetTranscription(w http.ResponseWriter, r *http.Request) {
	const op = "VideoHandler.HandleGetTranscription"
	logger := h.logger.WithContext(r.Context())

	if err := h.validator.ValidateRequest(r, validation.RequestValidationOpts{
		AllowedMethods: []string{http.MethodGet},
	}); err != nil {
		respondError(w, r, err)
		return
	}

	url := r.URL.Query().Get("url")
	if url == "" {
		respondError(w, r, errors.InvalidInput(op, nil, "URL parameter is required"))
		return
	}

	if err := h.validator.BasicURLValidation(url); err != nil {
		respondError(w, r, err)
		return
	}

	video, err := h.service.GetByURL(r.Context(), url)
	if err != nil {
		logger.WithError(err).Error("Failed to get transcription")
		respondError(w, r, err)
		return
	}

	respondJSON(w, r, http.StatusOK, models.NewVideoResponse(video))
}

// HandleCancelTranscription handles POST /api/v1/transcribe/cancel
func (h *VideoHandler) HandleCancelTranscription(w http.ResponseWriter, r *http.Request) {
	const op = "VideoHandler.HandleCancelTranscription"
	logger := h.logger.WithContext(r.Context())

	if err := h.validator.ValidateRequest(r, validation.RequestValidationOpts{
		MaxContentLength: 1024 * 1024,
		AllowedMethods:   []string{http.MethodPost},
		RequireJSON:      true,
	}); err != nil {
		respondError(w, r, err)
		return
	}

	var req struct {
		ID string `json:"id"`
	}

	if err := readJSON(r, &req); err != nil {
		respondError(w, r, err)
		return
	}

	if req.ID == "" {
		respondError(w, r, errors.InvalidInput(op, nil, "ID is required"))
		return
	}

	if err := h.service.Cancel(r.Context(), req.ID); err != nil {
		logger.WithError(err).Error("Failed to cancel transcription")
		respondError(w, r, err)
		return
	}

	respondJSON(w, r, http.StatusOK, map[string]string{"status": "cancelled"})
}

// HandleGetStatus handles GET /api/v1/transcribe/status/{id}
func (h *VideoHandler) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	const op = "VideoHandler.HandleGetStatus"
	logger := h.logger.WithContext(r.Context())

	id := r.PathValue("id")
	if id == "" {
		respondError(w, r, errors.InvalidInput(op, nil, "ID is required"))
		return
	}

	video, err := h.service.Get(r.Context(), id)
	if err != nil {
		logger.WithError(err).Error("Failed to get transcription status")
		respondError(w, r, err)
		return
	}

	respondJSON(w, r, http.StatusOK, models.NewVideoResponse(video))
}
