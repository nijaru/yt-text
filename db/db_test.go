package db

import (
	"context"
	"testing"
)

func TestMain(m *testing.M) {
	// Setup: Initialize the database
	err := InitializeDB("/tmp/test.db")
	if err != nil {
		panic("Failed to initialize database: " + err.Error())
	}
	defer DB.Close()

	// Run tests
	m.Run()
}

func TestSetAndGetTranscription(t *testing.T) {
	ctx := context.Background()
	url := "http://example.com"
	text := "Example transcription text"

	err := SetTranscription(ctx, url, text)
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
