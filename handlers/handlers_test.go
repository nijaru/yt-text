package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nijaru/yt-text/config"
	"github.com/nijaru/yt-text/db"
)

func TestMain(m *testing.M) {
    // Setup: Initialize the database
    err := db.InitializeDB("/tmp/test.db")
    if err != nil {
        panic("Failed to initialize database: " + err.Error())
    }
    defer db.DB.Close()

    // Run tests
    m.Run()
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
    }
    InitHandlers(cfg)

    // Inject the mock transcription function
    service.TranscriptionFunc = mockTranscriptionFunc

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
    }
    InitHandlers(cfg)

    // Inject the mock transcription function
    service.TranscriptionFunc = mockTranscriptionFunc

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
    }
    InitHandlers(cfg)

    // Inject the mock transcription function
    service.TranscriptionFunc = mockTranscriptionFunc

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
