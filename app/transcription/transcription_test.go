package transcription

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRunTranscriptionScript(t *testing.T) {
	service := &TranscriptionService{
		TranscriptionFunc: func(ctx context.Context, url string) (string, string, error) {
			return "Example transcription text", "base.en", nil
		},
		ExecuteScriptFunc: func(ctx context.Context, url string) ([]byte, error) {
			response := map[string]string{"transcription": "Example transcription text", "model_name": "base.en"}
			output, _ := json.Marshal(response)
			return output, nil
		},
		ReadFileFunc: func(filename string) (string, error) {
			return "Example transcription text", nil
		},
	}

	// Test TranscriptionFunc
	text, modelName, err := service.TranscriptionFunc(context.Background(), "http://example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedText := "Example transcription text"
	expectedModelName := "base.en"
	if text != expectedText {
		t.Errorf("expected '%s', got '%s'", expectedText, text)
	}
	if modelName != expectedModelName {
		t.Errorf("expected '%s', got '%s'", expectedModelName, modelName)
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

	if response["transcription"] != expectedText {
		t.Errorf("expected '%s', got '%s'", expectedText, response["transcription"])
	}
	if response["model_name"] != expectedModelName {
		t.Errorf("expected '%s', got '%s'", expectedModelName, response["model_name"])
	}

	// Test ReadFileFunc
	fileContent, err := service.ReadFileFunc("transcription.txt")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if fileContent != expectedText {
		t.Errorf("expected '%s', got '%s'", expectedText, fileContent)
	}
}
