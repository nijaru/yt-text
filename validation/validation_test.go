package validation

import (
    "testing"
)

func TestValidateURL_EdgeCases(t *testing.T) {
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