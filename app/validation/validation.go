package validation

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"yt-text/config"
	"yt-text/errors"
)

type Validator struct {
	config *config.Config
}

func NewValidator(cfg *config.Config) *Validator {
	return &Validator{config: cfg}
}

// ValidateURL performs basic URL validation and YouTube-specific checks
func (v *Validator) ValidateURL(urlStr string) error {
	const op = "Validator.ValidateURL"

	if urlStr == "" {
		return errors.InvalidInput(op, nil, "URL is required")
	}

	// Sanitize URL by trimming spaces
	urlStr = strings.TrimSpace(urlStr)
	
	// Parse URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return errors.InvalidInput(op, err, "Invalid URL format")
	}

	// Protocol validation
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.InvalidInput(op, nil, "URL must use HTTP or HTTPS")
	}

	// Basic domain security check
	if strings.Contains(parsedURL.Hostname(), "localhost") || 
	   strings.Contains(parsedURL.Hostname(), "127.0.0.1") || 
	   strings.Contains(parsedURL.Hostname(), "::1") {
		return errors.InvalidInput(op, nil, "Local addresses are not permitted")
	}

	// If it's a YouTube URL, perform additional validation
	if isYouTubeDomain(parsedURL.Hostname()) {
		if err := v.validateYouTubeURL(parsedURL); err != nil {
			return err
		}
	} else {
		// For non-YouTube URLs, validate that domain is allowed in config
		// This is a basic check - add more platform validations as needed
		if !v.config.Video.AllowNonYouTubeURLs {
			return errors.InvalidInput(op, nil, "Only YouTube URLs are currently supported")
		}
	}

	return nil
}

// isYouTubeDomain checks if the hostname is a valid YouTube domain
func isYouTubeDomain(host string) bool {
	validDomains := []string{
		"youtube.com",
		"www.youtube.com",
		"youtu.be",
		"m.youtube.com",
		"mobile.youtube.com",
		"music.youtube.com",
		"gaming.youtube.com",
	}

	for _, domain := range validDomains {
		if host == domain {
			return true
		}
	}
	return false
}

// validateYouTubeURL performs YouTube-specific URL validation
func (v *Validator) validateYouTubeURL(parsedURL *url.URL) error {
	const op = "Validator.validateYouTubeURL"

	// Handle youtu.be format (short URLs)
	if parsedURL.Host == "youtu.be" {
		// Extract video ID from path (remove leading slash)
		videoID := strings.TrimPrefix(parsedURL.Path, "/")
		if videoID == "" {
			return errors.InvalidInput(op, nil, "Invalid YouTube short URL format")
		}
		return nil
	}

	// Handle YouTube Shorts
	if strings.HasPrefix(parsedURL.Path, "/shorts/") {
		videoID := strings.TrimPrefix(parsedURL.Path, "/shorts/")
		if videoID == "" {
			return errors.InvalidInput(op, nil, "Invalid YouTube Shorts URL format")
		}
		return nil
	}

	// Handle embedded videos
	if strings.HasPrefix(parsedURL.Path, "/embed/") {
		videoID := strings.TrimPrefix(parsedURL.Path, "/embed/")
		if videoID == "" {
			return errors.InvalidInput(op, nil, "Invalid YouTube embed URL format")
		}
		return nil
	}

	// Handle standard youtube.com/watch format
	if parsedURL.Path == "/watch" {
		query := parsedURL.Query()
		videoID := query.Get("v")
		if videoID == "" {
			return errors.InvalidInput(op, nil, "Missing YouTube video ID")
		}
		return nil
	}

	// If not a recognized format
	return errors.InvalidInput(op, nil, "Unsupported YouTube URL format")
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
		contentType := r.Header.Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			return errors.InvalidInput(op, nil, "Content-Type must be application/json")
		}
	}

	// Content length validation
	if opts.MaxContentLength > 0 && r.ContentLength > opts.MaxContentLength {
		return errors.InvalidInput(op, nil, "Request body too large")
	}

	return nil
}
