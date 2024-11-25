package errors

import (
	"fmt"
	"net/http"
)

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

func InvalidInput(op string, err error, message string) *AppError {
	return &AppError{
		Code:    http.StatusBadRequest,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

func NotFound(op string, err error, message string) *AppError {
	return &AppError{
		Code:    http.StatusNotFound,
		Message: message,
		Op:      op,
		Err:     err,
	}
}

func Internal(op string, err error, message string) *AppError {
	return &AppError{
		Code:    http.StatusInternalServerError,
		Message: message,
		Op:      op,
		Err:     err,
	}
}
