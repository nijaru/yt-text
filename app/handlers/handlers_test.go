package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/db"
)

const testDBPath = "/tmp/test.db"

func TestMain(m *testing.M) {
	// Setup: Initialize the database
	err := db.InitializeDB(testDBPath)
	if err != nil {
		panic("Failed to initialize database: " + err.Error())
	}
	// Run tests
	code := m.Run()
	// Cleanup: Remove the test database file
	os.Remove(testDBPath)
	// Exit with the test result code
	os.Exit(code)
}

// Mock transcription function
func mockTranscriptionFunc(ctx context.Context, url string) (string, string, error) {
	return "Example transcription text", "base.en", nil
}

type mockRecaptchaServer struct {
	returnSuccess bool
	returnError   bool
}

func (m *mockRecaptchaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.returnError {
		// Return a proper server error instead of empty response
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
		return
	}
	response := struct {
		Success bool     `json:"success"`
		Errors  []string `json:"error-codes,omitempty"`
	}{
		Success: m.returnSuccess,
	}
	if !m.returnSuccess {
		response.Errors = []string{"invalid-input-response"}
	}
	json.NewEncoder(w).Encode(response)
}

func TestTranscribeHandler_InvalidCaptcha(t *testing.T) {
	// Setup mock server
	mock := &mockRecaptchaServer{returnSuccess: false}
	server := httptest.NewServer(mock)
	defer server.Close()
	// Override verification URL for testing
	originalURL := recaptchaVerifyURL
	recaptchaVerifyURL = server.URL
	defer func() { recaptchaVerifyURL = originalURL }()
	cfg := &config.Config{
		TranscribeTimeout: 10 * time.Second,
		RateLimit:         5,
		RateLimitInterval: 1 * time.Second,
		ModelName:         "base.en",
		RecaptchaSecret:   "test_secret",
	}
	InitHandlers(cfg)
	tests := []struct {
		name         string
		captchaToken string
		mockSuccess  bool
		mockError    bool
		expectedCode int
		expectedBody string
	}{
		{
			name:         "missing token",
			captchaToken: "",
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"error":"Captcha token required"}`,
		},
		{
			name:         "invalid token",
			captchaToken: "invalid",
			mockSuccess:  false,
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"error":"Captcha verification failed"}`,
		},
		{
			name:         "server error",
			captchaToken: "valid",
			mockError:    true,
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"error":"Failed to verify captcha"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.returnSuccess = tt.mockSuccess
			mock.returnError = tt.mockError
			formData := url.Values{
				"url":                  {"http://example.com"},
				"g-recaptcha-response": {tt.captchaToken},
			}
			req := httptest.NewRequest("POST", "/transcribe", strings.NewReader(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(TranscribeHandler)
			handler.ServeHTTP(rr, req)
			if status := rr.Code; status != tt.expectedCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedCode)
			}
			if body := strings.TrimSpace(rr.Body.String()); body != tt.expectedBody {
				t.Errorf("handler returned unexpected body: got %v want %v", body, tt.expectedBody)
			}
		})
	}
}
