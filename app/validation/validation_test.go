package validation

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

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
