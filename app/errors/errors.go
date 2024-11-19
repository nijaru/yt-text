package errors

import (
	"fmt"
	"net/http"
)

// AppError represents an application-specific error
type AppError struct {
	Code    int    `json:"-"`
	Message string `json:"error"`
	Op      string `json:"-"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// E creates a new AppError
func E(op string, err error, message string, code int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

// Common error constructors
func InvalidInput(op string, err error, message string) *AppError {
	return E(op, err, message, http.StatusBadRequest)
}

func NotFound(op string, err error, message string) *AppError {
	return E(op, err, message, http.StatusNotFound)
}

func Internal(op string, err error, message string) *AppError {
	return E(op, err, message, http.StatusInternalServerError)
}

func RateLimitExceeded(op string) *AppError {
	return E(op, nil, "Rate limit exceeded", http.StatusTooManyRequests)
}

// Error checking functions
func Is(err error, target *AppError) bool {
	appErr, ok := err.(*AppError)
	if !ok {
		return false
	}
	return appErr.Code == target.Code
}

func IsNotFound(err error) bool {
	appErr, ok := err.(*AppError)
	if !ok {
		return false
	}
	return appErr.Code == http.StatusNotFound
}

func IsInvalidInput(err error) bool {
	appErr, ok := err.(*AppError)
	if !ok {
		return false
	}
	return appErr.Code == http.StatusBadRequest
}
