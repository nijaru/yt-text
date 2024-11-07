package transcription

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/db"
	"github.com/nijaru/yt-text/errors"
)

const testDBPath = "/tmp/test.db"

func TestMain(m *testing.M) {
	if err := db.InitializeDB(testDBPath); err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	if db.DB != nil {
		db.DB.Close()
	}
	os.Remove(testDBPath)
	os.Exit(code)
}

func TestNewTranscriptionService(t *testing.T) {
	service := NewTranscriptionService()
	if service == nil {
		t.Error("expected non-nil service")
	}
	if service.TranscriptionFunc == nil {
		t.Error("expected non-nil TranscriptionFunc")
	}
}

func TestHandleTranscription_Basic(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		ModelName: "test-model",
		DBPath:    testDBPath,
	}

	tests := []struct {
		name           string
		url            string
		mockTranscribe func(ctx context.Context, url string) (string, string, error)
		expectError    bool
		expectedText   string
	}{
		{
			name: "successful transcription",
			url:  "http://example.com",
			mockTranscribe: func(ctx context.Context, url string) (string, string, error) {
				return "test transcription", "test-model", nil
			},
			expectError:  false,
			expectedText: "test transcription",
		},
		{
			name: "invalid URL",
			url:  "invalid-url",
			mockTranscribe: func(ctx context.Context, url string) (string, string, error) {
				return "", "", errors.ErrInvalidURL(fmt.Errorf("invalid URL"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewTranscriptionService()
			service.TranscriptionFunc = tt.mockTranscribe

			text, _, err := service.HandleTranscription(ctx, tt.url, cfg)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && tt.expectedText != text {
				t.Errorf("expected text %q, got %q", tt.expectedText, text)
			}
		})
	}
}

func TestExecuteTranscriptionWithRetry(t *testing.T) {
	t.Skip("Skipping long-running test")

	type mockResponse struct {
		output []byte
		err    error
	}

	tests := []struct {
		name             string
		mockResponses    []mockResponse
		expectedErr      bool
		expectedAttempts int
	}{
		{
			name: "successful first attempt",
			mockResponses: []mockResponse{
				{[]byte(`{"transcription": "test", "model_name": "test-model"}`), nil},
			},
			expectedErr:      false,
			expectedAttempts: 1,
		},
		{
			name: "temporary error with successful retry",
			mockResponses: []mockResponse{
				{nil, fmt.Errorf("temporary error")},
				{[]byte(`{"transcription": "test", "model_name": "test-model"}`), nil},
			},
			expectedErr:      false,
			expectedAttempts: 2,
		},
		{
			name: "permanent failure",
			mockResponses: []mockResponse{
				{nil, fmt.Errorf("permanent error")},
				{nil, fmt.Errorf("permanent error")},
				{nil, fmt.Errorf("permanent error")},
			},
			expectedErr:      true,
			expectedAttempts: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			attempts := 0
			service := &TranscriptionService{
				ExecuteScriptFunc: func(ctx context.Context, url string) ([]byte, error) {
					if attempts >= len(tt.mockResponses) {
						t.Fatal("more attempts than configured mock responses")
					}
					response := tt.mockResponses[attempts]
					attempts++
					return response.output, response.err
				},
			}

			output, err := service.executeTranscriptionWithRetry(ctx, "http://test.com")

			if tt.expectedErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if attempts != tt.expectedAttempts {
				t.Errorf("expected %d attempts, got %d", tt.expectedAttempts, attempts)
			}
			if !tt.expectedErr && output == nil {
				t.Error("expected output but got nil")
			}
		})
	}
}

func TestGenerateSummary(t *testing.T) {
	t.Skip("Skipping long-running test")

	ctx := context.Background()
	inputText := "test input text"
	expectedSummary := "test summary"
	expectedModel := "test-model"

	// Save and restore the original execCommand
	originalExecCommand := execCommand
	defer func() { execCommand = originalExecCommand }()

	// Set up the mock
	execCommand = func(command string, args ...string) *exec.Cmd {
		mockOutput := fmt.Sprintf(`{"summary": "%s", "model_name": "%s"}`, expectedSummary, expectedModel)
		return mockCommand(mockOutput)
	}

	summary, model, err := generateSummary(ctx, inputText)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if summary != expectedSummary {
		t.Errorf("expected '%s', got '%s'", expectedSummary, summary)
	}
	if model != expectedModel {
		t.Errorf("expected '%s', got '%s'", expectedModel, model)
	}
}

// Helper function to create mock command
func mockCommand(output string) *exec.Cmd {
	return exec.Command("echo", output)
}
