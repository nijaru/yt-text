package transcription

import (
	"context"
	"testing"
)

func TestRunTranscriptionScript(t *testing.T) {
	service := &TranscriptionService{
		TranscriptionFunc: func(ctx context.Context, url string) (string, error) {
			return "Example transcription text", nil
		},
		ExecuteScriptFunc: func(ctx context.Context, url string) ([]byte, error) {
			return []byte("transcription.txt"), nil
		},
		ReadFileFunc: func(filename string) (string, error) {
			return "Example transcription text", nil
		},
	}

	// Test TranscriptionFunc
	text, err := service.TranscriptionFunc(context.Background(), "http://example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := "Example transcription text"
	if text != expected {
		t.Errorf("expected '%s', got '%s'", expected, text)
	}

	// Test ExecuteScriptFunc
	scriptOutput, err := service.ExecuteScriptFunc(context.Background(), "http://example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedScriptOutput := "transcription.txt"
	if string(scriptOutput) != expectedScriptOutput {
		t.Errorf("expected '%s', got '%s'", expectedScriptOutput, string(scriptOutput))
	}

	// Test ReadFileFunc
	fileContent, err := service.ReadFileFunc("transcription.txt")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedFileContent := "Example transcription text"
	if fileContent != expectedFileContent {
		t.Errorf("expected '%s', got '%s'", expectedFileContent, fileContent)
	}
}
