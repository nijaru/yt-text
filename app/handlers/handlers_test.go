package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestHealthHandler(t *testing.T) {
	app := fiber.New()
	app.Get("/health", HealthHandler)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to test request: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status code %d, got %d", fiber.StatusOK, resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type %s, got %s", "application/json", resp.Header.Get("Content-Type"))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var response struct {
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
	}

	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		t.Fatalf("Failed to unmarshal response body: %v", err)
	}

	if response.Status != "ok" {
		t.Errorf("Expected status \"ok\", got %q", response.Status)
	}

	// Validate timestamp format (ISO8601)
	_, err = time.Parse(time.RFC3339, response.Timestamp)
	if err != nil {
		t.Errorf("Invalid timestamp format: %v", err)
	}
}