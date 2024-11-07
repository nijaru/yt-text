package utils

import (
	"encoding/json"
	"net/http"
	"strings"
)

func HandleError(w http.ResponseWriter, message string, statusCode int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func FormatText(text string) string {
    text = strings.TrimSpace(text)
    var builder strings.Builder
    for _, char := range text {
        builder.WriteRune(char)
        if char == '.' || char == '!' || char == '?' {
            builder.WriteRune('\n')
        }
    }
    return builder.String()
}
