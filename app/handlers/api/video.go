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

func NewVideoHandler(service video.Service, validator *validation.Validator) *VideoHandler {
	return &VideoHandler{
		service:   service,
		validator: validator,
		logger:    logrus.StandardLogger(),
	}
}

// HandleTranscribe handles POST /api/v1/transcribe
func (h *VideoHandler) HandleTranscribe(w http.ResponseWriter, r *http.Request) {
	const op = "VideoHandler.HandleTranscribe"
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

	video, err := h.service.Transcribe(r.Context(), url)
	if err != nil {
		logger.WithError(err).Error("Failed to process transcription")
		respondError(w, r, err)
		return
	}

	respondJSON(w, r, http.StatusAccepted, models.NewVideoResponse(video))
}

// HandleGetTranscription handles GET /api/v1/transcribe/{id}
func (h *VideoHandler) HandleGetTranscription(w http.ResponseWriter, r *http.Request) {
	const op = "VideoHandler.HandleGetTranscription"
	logger := h.logger.WithContext(r.Context())

	id := r.PathValue("id")
	if id == "" {
		respondError(w, r, errors.InvalidInput(op, nil, "ID is required"))
		return
	}

	video, err := h.service.GetTranscription(r.Context(), id)
	if err != nil {
		logger.WithError(err).Error("Failed to get transcription")
		respondError(w, r, err)
		return
	}

	respondJSON(w, r, http.StatusOK, models.NewVideoResponse(video))
}
