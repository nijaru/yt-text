package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/db"
	"github.com/nijaru/yt-text/validation"
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
func mockTranscriptionFunc(ctx context.Context, url string) (string, error) {
	return "Example transcription text", nil
}

func TestTranscribeHandler(t *testing.T) {
	cfg := &config.Config{
		TranscribeTimeout: 10 * time.Second,
		RateLimit:         5,
		RateLimitInterval: 1 * time.Second,
		ModelName:         "base.en",
	}
	InitHandlers(cfg)

	// Inject the mock transcription function
	service.TranscriptionFunc = mockTranscriptionFunc

	// Mock HTTP client for URL validation
	mockClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Body:       io.NopCloser(strings.NewReader("OK")),
				Request:    req,
			}
		}),
	}
	validation.SetHTTPClient(mockClient)

	req, err := http.NewRequest("POST", "/transcribe", strings.NewReader("url=http://fakeurl.com"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(TranscribeHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"transcription":"Example transcription text"}`
	if strings.TrimSpace(rr.Body.String()) != strings.TrimSpace(expected) {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestTranscribeHandler_InvalidURL(t *testing.T) {
	cfg := &config.Config{
		TranscribeTimeout: 10 * time.Second,
		RateLimit:         5,
		RateLimitInterval: 1 * time.Second,
		ModelName:         "base.en",
	}
	InitHandlers(cfg)

	req, err := http.NewRequest("POST", "/transcribe", strings.NewReader("url=invalid-url"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(TranscribeHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	expected := `{"error":"error: invalid URL format"}`
	if strings.TrimSpace(rr.Body.String()) != strings.TrimSpace(expected) {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestTranscribeHandler_RateLimit(t *testing.T) {
	cfg := &config.Config{
		TranscribeTimeout: 10 * time.Second,
		RateLimit:         1,
		RateLimitInterval: 1 * time.Second,
		ModelName:         "base.en",
	}
	InitHandlers(cfg)

	// Inject the mock transcription function
	service.TranscriptionFunc = mockTranscriptionFunc

	// Mock HTTP client for URL validation
	mockClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Body:       io.NopCloser(strings.NewReader("OK")),
				Request:    req,
			}
		}),
	}
	validation.SetHTTPClient(mockClient)

	req, err := http.NewRequest("POST", "/transcribe", strings.NewReader("url=http://fakeurl.com"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(TranscribeHandler)

	// First request should pass
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Second request should be rate limited
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusTooManyRequests {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusTooManyRequests)
	}

	expected := `{"error":"Rate limit exceeded"}`
	if strings.TrimSpace(rr.Body.String()) != strings.TrimSpace(expected) {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestConcurrentTranscriptions(t *testing.T) {
	cfg := &config.Config{
		TranscribeTimeout: 10 * time.Second,
		RateLimit:         10,
		RateLimitInterval: 1 * time.Second,
		ModelName:         "base.en",
	}
	InitHandlers(cfg)

	// Inject the mock transcription function
	service.TranscriptionFunc = mockTranscriptionFunc

	// Mock HTTP client for URL validation
	mockClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Body:       io.NopCloser(strings.NewReader("OK")),
				Request:    req,
			}
		}),
	}
	validation.SetHTTPClient(mockClient)

	url := "http://concurrenturl.com"

	var wg sync.WaitGroup
	errCh := make(chan error, 10) // Buffered channel to collect errors

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reqBody := strings.NewReader("url=" + url)
			req, err := http.NewRequest("POST", "/transcribe", reqBody)
			if err != nil {
				errCh <- err
				return
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(TranscribeHandler)
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusOK {
				errCh <- fmt.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
				return
			}

			expected := `{"transcription":"Example transcription text"}`
			if strings.TrimSpace(rr.Body.String()) != strings.TrimSpace(expected) {
				errCh <- fmt.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
				return
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Error(err)
		}
	}
}

type roundTripperFunc func(req *http.Request) *http.Response

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
