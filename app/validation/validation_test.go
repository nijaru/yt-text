package validation

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid http URL",
			url:     "http://example.com",
			wantErr: false,
		},
		{
			name:    "valid https URL",
			url:     "https://example.com",
			wantErr: false,
		},
		{
			name:    "invalid URL - no scheme",
			url:     "example.com",
			wantErr: true,
		},
		{
			name:    "invalid URL - empty",
			url:     "",
			wantErr: true,
		},
		{
			name:    "invalid URL - malformed",
			url:     "http://",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL_EdgeCases(t *testing.T) {
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

	SetHTTPClient(mockClient)

	tests := []struct {
		url     string
		wantErr bool
	}{
		{"http://example.com/path?query=1", false},
		{"https://example.com/path#fragment", false},
		{"http://", true},
		{"http://example.com:8080", false},
		{"http://user:pass@example.com", false},
	}

	for _, tt := range tests {
		err := ValidateURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateURL(%s) error = %v, wantErr %v", tt.url, err, tt.wantErr)
		}
	}
}

type roundTripperFunc func(req *http.Request) *http.Response

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
