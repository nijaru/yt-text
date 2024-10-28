package validation

import (
    "fmt"
    "net/url"
    "strings"
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

    return nil
}