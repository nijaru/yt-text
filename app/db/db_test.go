package db

import (
	"context"
	"os"
	"testing"
)

const testDBPath = "/tmp/test.db"

func TestMain(m *testing.M) {
	// Setup: Initialize the database
	err := InitializeDB(testDBPath)
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

func TestSetAndGetTranscription(t *testing.T) {
	ctx := context.Background()
	url := "http://example.com"
	text := "Example transcription text"
	modelName := "base.en"

	err := SetTranscription(ctx, url, text, modelName)
	if err != nil {
		t.Fatalf("Failed to set transcription: %v", err)
	}

	retrievedText, status, err := GetTranscription(ctx, url)
	if err != nil {
		t.Fatalf("Failed to get transcription: %v", err)
	}
	if status != "completed" {
		t.Errorf("expected status 'completed', got %s", status)
	}
	if retrievedText != text {
		t.Errorf("expected text '%s', got '%s'", text, retrievedText)
	}

	retrievedModelName, err := GetModelName(ctx, url)
	if err != nil {
		t.Fatalf("Failed to get model name: %v", err)
	}
	if retrievedModelName != modelName {
		t.Errorf("expected model name '%s', got '%s'", modelName, retrievedModelName)
	}
}

func TestSetTranscriptionStatus(t *testing.T) {
	ctx := context.Background()
	url := "http://example.com"
	status := "in_progress"

	err := SetTranscriptionStatus(ctx, url, status)
	if err != nil {
		t.Fatalf("Failed to set transcription status: %v", err)
	}

	_, retrievedStatus, err := GetTranscription(ctx, url)
	if err != nil {
		t.Fatalf("Failed to get transcription: %v", err)
	}
	if retrievedStatus != status {
		t.Errorf("expected status '%s', got '%s'", status, retrievedStatus)
	}
}

func TestDeleteTranscription(t *testing.T) {
	ctx := context.Background()
	url := "http://example.com"

	err := DeleteTranscription(ctx, url)
	if err != nil {
		t.Fatalf("Failed to delete transcription: %v", err)
	}

	_, status, err := GetTranscription(ctx, url)
	if err != nil {
		t.Fatalf("Failed to get transcription: %v", err)
	}
	if status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", status)
	}
}

func TestInitializeDB_Error(t *testing.T) {
	// Simulate an error by providing an invalid path
	err := InitializeDB("/invalid/path/to/db")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSetAndGetSummary(t *testing.T) {
	ctx := context.Background()
	url := "http://example.com"
	summary := "Example summary text"
	summaryModelName := "facebook/bart-large-cnn"

	err := SetSummary(ctx, url, summary, summaryModelName)
	if err != nil {
		t.Fatalf("Failed to set summary: %v", err)
	}

	retrievedSummary, retrievedSummaryModelName, err := GetSummary(ctx, url)
	if err != nil {
		t.Fatalf("Failed to get summary: %v", err)
	}
	if retrievedSummary != summary {
		t.Errorf("expected summary '%s', got '%s'", summary, retrievedSummary)
	}
	if retrievedSummaryModelName != summaryModelName {
		t.Errorf("expected summary model name '%s', got '%s'", summaryModelName, retrievedSummaryModelName)
	}
}
