package validation

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nijaru/yt-text/errors"
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// Allow up to 10 redirects
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

func SetHTTPClient(client *http.Client) {
	httpClient = client
}

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func ValidateURL(rawURL string) error {
	const op = "validation.ValidateURL"

	if rawURL == "" {
		return errors.InvalidInput(op, nil, "URL is required")
	}

	rawURL = strings.TrimSpace(rawURL)
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return errors.InvalidInput(op, err, "Invalid URL format")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.InvalidInput(op, nil, "URL must start with http or https")
	}

	if parsedURL.Host == "" {
		return errors.InvalidInput(op, nil, "URL must have a host")
	}

	if strings.Contains(parsedURL.Host, "youtube.com") {
		queryParams := parsedURL.Query()
		if _, ok := queryParams["v"]; !ok || queryParams.Get("v") == "" {
			return errors.InvalidInput(op, nil, "YouTube URL must contain a valid video ID")
		}
	}

	return nil
}

var (
	allowedDomains = []string{"youtube.com", "youtu.be"}
	blockedDomains = []string{"example.com", "malicious.com"}
)

func validateDomain(urlStr string) error {
	const op = "validation.validateDomain"

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return errors.InvalidInput(op, err, "Invalid URL format")
	}

	// Check if domain is blocked
	for _, blocked := range blockedDomains {
		if strings.Contains(parsedURL.Host, blocked) {
			return errors.InvalidInput(op, nil, "Domain not allowed")
		}
	}

	// Check if domain is allowed
	allowed := false
	for _, domain := range allowedDomains {
		if strings.Contains(parsedURL.Host, domain) {
			allowed = true
			break
		}
	}
	if !allowed {
		return errors.InvalidInput(op, nil, "Domain not supported")
	}

	return nil
}
