package mime

import (
	"net/http"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

// DetectMIMEType determines the MIME type of content.
// Priority: Content-Type header > magic bytes (mimetype lib) > http.DetectContentType fallback.
func DetectMIMEType(data []byte, contentType string) string {
	// 1. Use Content-Type header if present and meaningful
	if contentType != "" {
		ct := strings.SplitN(contentType, ";", 2)[0]
		ct = strings.TrimSpace(ct)
		if ct != "" && ct != "application/octet-stream" && ct != "binary/octet-stream" {
			return ct
		}
	}

	// 2. Magic bytes detection via mimetype library
	if len(data) > 0 {
		detected := mimetype.Detect(data)
		if detected.String() != "application/octet-stream" {
			return detected.String()
		}

		// 3. Fallback to standard library detection
		stdDetected := http.DetectContentType(data)
		if stdDetected != "application/octet-stream" {
			return stdDetected
		}
	}

	return "application/octet-stream"
}

// DetectFromBytes determines the MIME type from raw bytes only (no Content-Type hint).
func DetectFromBytes(data []byte) string {
	return DetectMIMEType(data, "")
}
