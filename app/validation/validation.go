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
	
	// Basic sanity check to prevent XSS
	if strings.Contains(strings.ToLower(urlStr), "javascript:") {
		return errors.InvalidInput(op, nil, "Invalid URL format")
	}
	
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
	   strings.HasPrefix(parsedURL.Hostname(), "192.168.") ||
	   strings.HasPrefix(parsedURL.Hostname(), "10.") ||
	   strings.HasPrefix(parsedURL.Hostname(), "172.") ||
	   strings.Contains(parsedURL.Hostname(), "::1") {
		return errors.InvalidInput(op, nil, "Local and private network addresses are not permitted")
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
	
	// Origin validation for cross-origin requests
	origin := r.Header.Get("Origin")
	if origin != "" && r.Host != "" {
		originURL, err := url.Parse(origin)
		if err != nil {
			return errors.InvalidInput(op, err, "Invalid Origin header")
		}
		
		// Simple check to detect potential CSRF attempts
		// In production, this should be enhanced with CSRF tokens
		if originURL.Host != r.Host && r.Method != "GET" && r.Method != "HEAD" {
			// Log the incident but don't expose details in the error
			// This is a basic protection - an actual CSRF token would be better
			return errors.InvalidInput(op, nil, "Request validation failed")
		}
	}

	return nil
}
