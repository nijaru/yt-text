package utils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleError(t *testing.T) {
	rr := httptest.NewRecorder()
	HandleError(rr, "Test error", http.StatusBadRequest)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	expected := `{"error":"Test error"}`
	if strings.TrimSpace(rr.Body.String()) != strings.TrimSpace(expected) {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestFormatText(t *testing.T) {
	input := "This is a test. This is only a test!"
	expected := "This is a test.\n This is only a test!\n"
	output := FormatText(input)

	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}