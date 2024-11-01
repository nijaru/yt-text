package validation

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ValidationError struct {
    Message string
}

func (e *ValidationError) Error() string {
    return e.Message
}

func ValidateURL(rawURL string) error {
    if rawURL == "" {
        return fmt.Errorf("error: URL is required")
    }

    rawURL = strings.TrimSpace(rawURL)

    parsedURL, err := url.ParseRequestURI(rawURL)
    if err != nil {
        return fmt.Errorf("error: invalid URL format")
    }

    if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
        return fmt.Errorf("error: URL must start with http or https")
    }

    if parsedURL.Host == "" {
        return fmt.Errorf("error: URL must have a host")
    }

    // Check for YouTube-specific parameters
    if strings.Contains(parsedURL.Host, "youtube.com") {
        queryParams := parsedURL.Query()
        if _, ok := queryParams["v"]; !ok || queryParams.Get("v") == "" {
            return fmt.Errorf("error: YouTube URL must contain a valid video ID")
        }
    }

    // Make an HTTP request to the URL to check if it's valid
    client := &http.Client{
        Timeout: 10 * time.Second,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            // Allow up to 10 redirects
            if len(via) >= 10 {
                return http.ErrUseLastResponse
            }
            return nil
        },
    }
    resp, err := client.Get(rawURL)
    if err != nil {
        return fmt.Errorf("error: failed to reach the URL")
    }
    defer resp.Body.Close()

    // Check if the final URL is valid and serves a page
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("error: URL returned status code %d", resp.StatusCode)
    }

    // Ensure the final URL is not a redirect
    finalURL := resp.Request.URL.String()
    if finalURL != rawURL {
        return fmt.Errorf("error: URL redirects to %s", finalURL)
    }

    // Check for specific content types (e.g., HTML)
    contentType := resp.Header.Get("Content-Type")
    if !strings.Contains(contentType, "text/html") {
        return fmt.Errorf("error: URL does not point to an HTML page")
    }

    return nil
}
