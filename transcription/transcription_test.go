package transcription

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRunTranscriptionScript(t *testing.T) {
	service := &TranscriptionService{
		TranscriptionFunc: func(ctx context.Context, url string) (string, error) {
			return "Example transcription text", nil
		},
		ExecuteScriptFunc: func(ctx context.Context, url string) ([]byte, error) {
			response := map[string]string{"transcription": "Example transcription text"}
			output, _ := json.Marshal(response)
			return output, nil
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

	var response map[string]string
	if err := json.Unmarshal(scriptOutput, &response); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}

	expectedScriptOutput := "Example transcription text"
	if response["transcription"] != expectedScriptOutput {
		t.Errorf("expected '%s', got '%s'", expectedScriptOutput, response["transcription"])
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
