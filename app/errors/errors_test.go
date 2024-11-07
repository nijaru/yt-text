package errors

import (
	"fmt"
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
    err := New(http.StatusBadRequest, "test message", nil)

    if err.Code != http.StatusBadRequest {
        t.Errorf("expected code %d, got %d", http.StatusBadRequest, err.Code)
    }

    if err.Message != "test message" {
        t.Errorf("expected message 'test message', got '%s'", err.Message)
    }

    if err.Error() != "test message" {
        t.Errorf("expected error string 'test message', got '%s'", err.Error())
    }
}

func TestErrorWithCause(t *testing.T) {
    cause := New(http.StatusInternalServerError, "cause error", nil)
    err := New(http.StatusBadRequest, "test message", cause)

    expected := "test message: cause error"
    if err.Error() != expected {
        t.Errorf("expected '%s', got '%s'", expected, err.Error())
    }
}

func TestIsNotFound(t *testing.T) {
    tests := []struct {
        name     string
        err      error
        expected bool
    }{
        {
            name:     "not found error",
            err:      New(http.StatusNotFound, "not found", nil),
            expected: true,
        },
        {
            name:     "other error",
            err:      New(http.StatusBadRequest, "bad request", nil),
            expected: false,
        },
        {
            name:     "non-custom error",
            err:      fmt.Errorf("standard error"),
            expected: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := IsNotFound(tt.err); got != tt.expected {
                t.Errorf("IsNotFound() = %v, want %v", got, tt.expected)
            }
        })
    }
}

func TestCommonErrors(t *testing.T) {
    tests := []struct {
        name     string
        err      *Error
        expected int
    }{
        {
            name:     "invalid URL error",
            err:      ErrInvalidURL(fmt.Errorf("test")),
            expected: http.StatusBadRequest,
        },
        {
            name:     "database operation error",
            err:      ErrDatabaseOperation(fmt.Errorf("test")),
            expected: http.StatusInternalServerError,
        },
        {
            name:     "transcription failed error",
            err:      ErrTranscriptionFailed(fmt.Errorf("test")),
            expected: http.StatusInternalServerError,
        },
        {
            name:     "rate limit exceeded error",
            err:      ErrRateLimitExceeded,
            expected: http.StatusTooManyRequests,
        },
        {
            name:     "invalid request error",
            err:      ErrInvalidRequest("test"),
            expected: http.StatusBadRequest,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if tt.err.Code != tt.expected {
                t.Errorf("expected code %d, got %d", tt.expected, tt.err.Code)
            }
        })
    }
}
