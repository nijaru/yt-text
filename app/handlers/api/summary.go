package api

import (
	"net/http"

	"github.com/nijaru/yt-text/errors"
	"github.com/nijaru/yt-text/models"
	"github.com/nijaru/yt-text/services/summary"
	"github.com/nijaru/yt-text/validation"
	"github.com/sirupsen/logrus"
)

type SummaryHandler struct {
	service   summary.Service
	validator *validation.Validator
	logger    *logrus.Logger
}

type createSummaryRequest struct {
	URL       string `json:"url"`
	MaxLength int    `json:"max_length,omitempty"`
	MinLength int    `json:"min_length,omitempty"`
	ModelName string `json:"model_name,omitempty"`
}

func NewSummaryHandler(service summary.Service, validator *validation.Validator) *SummaryHandler {
	return &SummaryHandler{
		service:   service,
		validator: validator,
		logger:    logrus.StandardLogger(),
	}
}

// HandleCreateSummary handles POST /api/v1/summary
func (h *SummaryHandler) HandleCreateSummary(w http.ResponseWriter, r *http.Request) {
	const op = "SummaryHandler.HandleCreateSummary"
	logger := h.logger.WithContext(r.Context())

	// Validate request format
	if err := h.validator.ValidateRequest(r, validation.RequestValidationOpts{
		MaxContentLength: 1024 * 1024, // 1MB
		AllowedMethods:   []string{http.MethodPost},
		RequireJSON:      true,
	}); err != nil {
		respondError(w, r, err)
		return
	}

	var req createSummaryRequest
	if err := readJSON(r, &req); err != nil {
		respondError(w, r, err)
		return
	}

	// Validate parameters
	if err := h.validateSummaryParams(&req); err != nil {
		respondError(w, r, err)
		return
	}

	// Create summary options
	opts := summary.Options{
		ModelName: req.ModelName,
		MaxLength: req.MaxLength,
		MinLength: req.MinLength,
	}

	video, err := h.service.CreateSummary(r.Context(), req.URL, opts)
	if err != nil {
		logger.WithError(err).Error("Failed to create summary")
		respondError(w, r, err)
		return
	}

	respondJSON(w, r, http.StatusAccepted, models.NewVideoResponse(video))
}

// HandleGetSummary handles GET /api/v1/summary
func (h *SummaryHandler) HandleGetSummary(w http.ResponseWriter, r *http.Request) {
	const op = "SummaryHandler.HandleGetSummary"
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

	video, err := h.service.GetSummary(r.Context(), url)
	if err != nil {
		logger.WithError(err).Error("Failed to get summary")
		respondError(w, r, err)
		return
	}

	respondJSON(w, r, http.StatusOK, models.NewVideoResponse(video))
}

// HandleGetSummaryStatus handles GET /api/v1/summary/status/{id}
func (h *SummaryHandler) HandleGetSummaryStatus(w http.ResponseWriter, r *http.Request) {
	const op = "SummaryHandler.HandleGetSummaryStatus"
	logger := h.logger.WithContext(r.Context())

	if err := h.validator.ValidateRequest(r, validation.RequestValidationOpts{
		AllowedMethods: []string{http.MethodGet},
	}); err != nil {
		respondError(w, r, err)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		respondError(w, r, errors.InvalidInput(op, nil, "ID is required"))
		return
	}

	status, err := h.service.GetStatus(r.Context(), id)
	if err != nil {
		logger.WithError(err).Error("Failed to get summary status")
		respondError(w, r, err)
		return
	}

	respondJSON(w, r, http.StatusOK, models.NewVideoResponse(status))
}

// Helper method for consistent logging
func (h *SummaryHandler) logRequest(r *http.Request, fields logrus.Fields) {
	h.logger.WithFields(logrus.Fields{
		"method":     r.Method,
		"path":       r.URL.Path,
		"request_id": r.Context().Value("request_id"),
	}).WithFields(fields).Info("Processing request")
}

// Helper method for validating summary parameters
func (h *SummaryHandler) validateSummaryParams(req *createSummaryRequest) error {
	const op = "SummaryHandler.validateSummaryParams"

	if req.MaxLength < 0 {
		return errors.InvalidInput(op, nil, "max_length must be positive")
	}
	if req.MinLength < 0 {
		return errors.InvalidInput(op, nil, "min_length must be positive")
	}
	if req.MinLength > req.MaxLength && req.MaxLength != 0 {
		return errors.InvalidInput(op, nil, "min_length cannot be greater than max_length")
	}

	// Validate model name if provided
	if req.ModelName != "" {
		// Example of model validation - adjust based on your needs
		allowedModels := map[string]bool{
			"facebook/bart-large-cnn": true,
			"google/pegasus-large":    true,
			// Add other supported models
		}

		if !allowedModels[req.ModelName] {
			return errors.InvalidInput(op, nil, "unsupported model name")
		}
	}

	return nil
}

// Helper method for error response with logging
func (h *SummaryHandler) handleError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
	op string,
	message string,
) {
	h.logger.WithFields(logrus.Fields{
		"error":      err,
		"operation":  op,
		"request_id": r.Context().Value("request_id"),
	}).Error(message)

	respondError(w, r, err)
}
