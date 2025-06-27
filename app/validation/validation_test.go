package validation

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"yt-text/config"
)

func TestValidateURL(t *testing.T) {
	cfg := &config.Config{
		Video: config.VideoConfig{
			AllowNonYouTubeURLs: false,
		},
	}
	validator := NewValidator(cfg)

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "Empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "JavaScript URL",
			url:     "javascript:alert(1)",
			wantErr: true,
		},
		{
			name:    "Invalid URL format",
			url:     "not-a-url",
			wantErr: true,
		},
		{
			name:    "Non-HTTP scheme",
			url:     "ftp://example.com",
			wantErr: true,
		},
		{
			name:    "Localhost URL",
			url:     "http://localhost:8000",
			wantErr: true,
		},
		{
			name:    "Private IP URL",
			url:     "http://192.168.1.1",
			wantErr: true,
		},
		{
			name:    "Valid YouTube URL",
			url:     "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantErr: false,
		},
		{
			name:    "Valid YouTube shorts URL",
			url:     "https://www.youtube.com/shorts/dQw4w9WgXcQ",
			wantErr: false,
		},
		{
			name:    "Valid YouTube embed URL",
			url:     "https://www.youtube.com/embed/dQw4w9WgXcQ",
			wantErr: false,
		},
		{
			name:    "Valid YouTube short URL",
			url:     "https://youtu.be/dQw4w9WgXcQ",
			wantErr: false,
		},
		{
			name:    "Non-YouTube URL with AllowNonYouTubeURLs=false",
			url:     "https://example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Test with AllowNonYouTubeURLs=true
	cfg.Video.AllowNonYouTubeURLs = true
	validator = NewValidator(cfg)
	t.Run("Non-YouTube URL with AllowNonYouTubeURLs=true", func(t *testing.T) {
		err := validator.ValidateURL("https://example.com")
		if err != nil {
			t.Errorf("ValidateURL() error = %v, expected no error when AllowNonYouTubeURLs=true", err)
		}
	})
}

func TestIsYouTubeDomain(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     bool
	}{
		{
			name:     "youtube.com",
			hostname: "youtube.com",
			want:     true,
		},
		{
			name:     "www.youtube.com",
			hostname: "www.youtube.com",
			want:     true,
		},
		{
			name:     "m.youtube.com",
			hostname: "m.youtube.com",
			want:     true,
		},
		{
			name:     "youtu.be",
			hostname: "youtu.be",
			want:     true,
		},
		{
			name:     "example.com",
			hostname: "example.com",
			want:     false,
		},
		{
			name:     "youtube.example.com",
			hostname: "youtube.example.com",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isYouTubeDomain(tt.hostname)
			if got != tt.want {
				t.Errorf("isYouTubeDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateYouTubeURL(t *testing.T) {
	cfg := &config.Config{}
	validator := NewValidator(cfg)

	tests := []struct {
		name    string
		urlStr  string
		wantErr bool
	}{
		{
			name:    "Standard YouTube URL",
			urlStr:  "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantErr: false,
		},
		{
			name:    "YouTube URL with multiple parameters",
			urlStr:  "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=10s",
			wantErr: false,
		},
		{
			name:    "YouTube shorts URL",
			urlStr:  "https://www.youtube.com/shorts/dQw4w9WgXcQ",
			wantErr: false,
		},
		{
			name:    "YouTube embed URL",
			urlStr:  "https://www.youtube.com/embed/dQw4w9WgXcQ",
			wantErr: false,
		},
		{
			name:    "Short YouTube URL",
			urlStr:  "https://youtu.be/dQw4w9WgXcQ",
			wantErr: false,
		},
		{
			name:    "YouTube URL without video ID",
			urlStr:  "https://www.youtube.com/watch",
			wantErr: true,
		},
		{
			name:    "YouTube URL with empty video ID",
			urlStr:  "https://www.youtube.com/watch?v=",
			wantErr: true,
		},
		{
			name:    "Non-YouTube URL",
			urlStr:  "https://example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.urlStr)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}
			
			err = validator.validateYouTubeURL(parsedURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateYouTubeURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRequest(t *testing.T) {
	cfg := &config.Config{}
	validator := NewValidator(cfg)

	tests := []struct {
		name           string
		method         string
		contentType    string
		contentLength  int
		origin         string
		options        RequestValidationOpts
		wantErr        bool
		wantErrMessage string
	}{
		{
			name:          "GET request with default options",
			method:        "GET",
			contentType:   "",
			contentLength: 0,
			origin:        "",
			options:       RequestValidationOpts{},
			wantErr:       false,
		},
		{
			name:          "POST request with valid Content-Type",
			method:        "POST",
			contentType:   "application/json",
			contentLength: 100,
			origin:        "https://example.com",
			options: RequestValidationOpts{
				RequireJSON: true,
			},
			wantErr: false,
		},
		{
			name:          "PUT request with invalid Content-Type",
			method:        "PUT",
			contentType:   "text/plain",
			contentLength: 100,
			origin:        "https://example.com",
			options: RequestValidationOpts{
				RequireJSON: true,
			},
			wantErr:        true,
			wantErrMessage: "application/json",
		},
		{
			name:          "POST request with excessive content length",
			method:        "POST",
			contentType:   "application/json",
			contentLength: 2 * 1024 * 1024, // 2MB
			origin:        "https://example.com",
			options: RequestValidationOpts{
				MaxContentLength: 1024 * 1024, // 1MB
			},
			wantErr:        true,
			wantErrMessage: "body too large",
		},
		{
			name:          "Method not allowed",
			method:        "DELETE",
			contentType:   "application/json",
			contentLength: 100,
			origin:        "https://example.com",
			options: RequestValidationOpts{
				AllowedMethods: []string{"GET", "POST"},
			},
			wantErr:        true,
			wantErrMessage: "method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a regular HTTP request for testing
			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			req.ContentLength = int64(tt.contentLength)

			// Test the validation
			err := validator.ValidateRequest(req, tt.options)
			
			// Check error presence
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			// If we want error, check the error message contains expected substring
			if tt.wantErr && tt.wantErrMessage != "" && err != nil {
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.wantErrMessage)) {
					t.Errorf("ValidateRequest() error message = %v, wantErrMessage to contain %v", 
						err.Error(), tt.wantErrMessage)
				}
			}
		})
	}
}