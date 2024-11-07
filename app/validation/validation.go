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
	if rawURL == "" {
		return errors.New(http.StatusBadRequest, "URL is required", nil)
	}

	rawURL = strings.TrimSpace(rawURL)
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return errors.ErrInvalidURL(err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.ErrInvalidRequest("URL must start with http or https")
	}

	if parsedURL.Host == "" {
		return errors.ErrInvalidRequest("URL must have a host")
	}

	if strings.Contains(parsedURL.Host, "youtube.com") {
		queryParams := parsedURL.Query()
		if _, ok := queryParams["v"]; !ok || queryParams.Get("v") == "" {
			return errors.ErrInvalidRequest("YouTube URL must contain a valid video ID")
		}
	}

	return nil
}
