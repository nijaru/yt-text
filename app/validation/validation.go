package validation

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/errors"
)

type Validator struct {
	config *config.Config
}

func NewValidator(cfg *config.Config) *Validator {
	return &Validator{
		config: cfg,
	}
}

// BasicURLValidation performs quick validation without network calls
func (v *Validator) BasicURLValidation(urlStr string) error {
	const op = "Validator.BasicURLValidation"

	if urlStr == "" {
		return errors.InvalidInput(op, nil, "URL is required")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return errors.InvalidInput(op, err, "Invalid URL format")
	}

	// Protocol validation
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.InvalidInput(op, nil, "URL must use HTTP or HTTPS")
	}

	// Domain validation
	host := parsedURL.Hostname()
	if !strings.Contains(host, "youtube.com") && !strings.Contains(host, "youtu.be") {
		return errors.InvalidInput(op, nil, "Only YouTube URLs are supported")
	}

	// Video ID validation
	if strings.Contains(host, "youtube.com") {
		if v := parsedURL.Query().Get("v"); v == "" {
			return errors.InvalidInput(op, nil, "Missing video ID in YouTube URL")
		}
	} else if strings.Contains(host, "youtu.be") {
		if path := strings.TrimPrefix(parsedURL.Path, "/"); path == "" {
			return errors.InvalidInput(op, nil, "Missing video ID in YouTube URL")
		}
	}

	return nil
}

// Request validation
func (v *Validator) ValidateRequest(r *http.Request, opts RequestValidationOpts) error {
	const op = "Validator.ValidateRequest"

	// Method validation
	if len(opts.AllowedMethods) > 0 {
		methodAllowed := false
		for _, method := range opts.AllowedMethods {
			if r.Method == method {
				methodAllowed = true
				break
			}
		}
		if !methodAllowed {
			return errors.InvalidInput(op, nil, fmt.Sprintf("Method %s not allowed", r.Method))
		}
	}

	// Content type validation
	if opts.RequireJSON && (r.Method == http.MethodPost || r.Method == http.MethodPut) {
		if err := v.ValidateContentType(r, "application/json"); err != nil {
			return err
		}
	}

	// Content length validation
	if opts.MaxContentLength > 0 {
		if r.ContentLength > opts.MaxContentLength {
			return errors.InvalidInput(op, nil, "Request body too large")
		}
		r.Body = http.MaxBytesReader(nil, r.Body, opts.MaxContentLength)
	}

	return nil
}

type RequestValidationOpts struct {
	MaxContentLength int64
	AllowedMethods   []string
	RequireJSON      bool
}

// VideoMetadata validation
func (v *Validator) ValidateVideoMetadata(duration float64, fileSize int64) error {
	const op = "Validator.ValidateVideoMetadata"

	if duration > v.config.Video.MaxDuration.Seconds() {
		return errors.InvalidInput(op, nil, fmt.Sprintf(
			"Video duration (%.1f seconds) exceeds maximum allowed (%v seconds)",
			duration,
			v.config.Video.MaxDuration.Seconds(),
		))
	}

	if fileSize > v.config.Video.MaxFileSize {
		return errors.InvalidInput(op, nil, fmt.Sprintf(
			"Video file size (%d bytes) exceeds maximum allowed (%d bytes)",
			fileSize,
			v.config.Video.MaxFileSize,
		))
	}

	return nil
}

func (v *Validator) ValidateContentType(r *http.Request, expectedType string) error {
	const op = "Validator.ValidateContentType"

	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, expectedType) {
		return errors.InvalidInput(op, nil, fmt.Sprintf("Content-Type must be %s", expectedType))
	}

	return nil
}
