package s3

import (
	"testing"
)

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"index.html", "text/html; charset=utf-8"},
		{"style.css", "text/css; charset=utf-8"},
		{"app.js", "application/javascript"},
		{"data.json", "application/json"},
		{"logo.svg", "image/svg+xml"},
		{"photo.png", "image/png"},
		{"photo.jpg", "image/jpeg"},
		{"photo.webp", "image/webp"},
		{"favicon.ico", "image/x-icon"},
		{"font.woff2", "font/woff2"},
		{"readme.txt", "text/plain; charset=utf-8"},
		{"page.html.enc", "application/octet-stream"},
		{"noext", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := detectContentType(tt.filename)
			if got != tt.expected {
				t.Errorf("detectContentType(%q) = %q, want %q", tt.filename, got, tt.expected)
			}
		})
	}
}
