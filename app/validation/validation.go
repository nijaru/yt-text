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
	return &Validator{config: cfg}
}

// ValidateURL performs URL validation
func (v *Validator) ValidateURL(urlStr string) error {
	const op = "Validator.ValidateURL"

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

	return nil
}

// RequestValidationOpts holds options for request validation
type RequestValidationOpts struct {
	MaxContentLength int64
	AllowedMethods   []string
	RequireJSON      bool
}

// ValidateRequest validates HTTP requests
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
	if opts.RequireJSON {
		if contentType := r.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
			return errors.InvalidInput(op, nil, "Content-Type must be application/json")
		}
	}

	// Content length validation
	if opts.MaxContentLength > 0 && r.ContentLength > opts.MaxContentLength {
		return errors.InvalidInput(op, nil, "Request body too large")
	}

	return nil
}
