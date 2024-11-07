package errors

import (
	"fmt"
	"net/http"
)

type Error struct {
	Code    int    `json:"-"`
	Message string `json:"error"`
	Err     error  `json:"-"`
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func New(code int, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

func IsNotFound(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == http.StatusNotFound
	}
	return false
}

// Common errors
var (
	ErrInvalidURL = func(err error) *Error {
		return New(http.StatusBadRequest, "Invalid URL format", err)
	}

	ErrDatabaseOperation = func(err error) *Error {
		return New(http.StatusInternalServerError, "Database operation failed", err)
	}

	ErrTranscriptionFailed = func(err error) *Error {
		return New(http.StatusInternalServerError, "Transcription process failed", err)
	}

	ErrRateLimitExceeded = New(http.StatusTooManyRequests, "Rate limit exceeded", nil)

	ErrInvalidRequest = func(msg string) *Error {
		return New(http.StatusBadRequest, msg, nil)
	}
)
