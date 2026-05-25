package mime

import (
	"testing"
)

func TestDetectMIMEType(t *testing.T) {
	pngHeader := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	jpegHeader := []byte{0xff, 0xd8, 0xff, 0xe0}
	gifHeader := []byte("GIF89a")

	tests := []struct {
		name        string
		data        []byte
		contentType string
		want        string
	}{
		{
			name:        "explicit content type takes priority",
			data:        pngHeader,
			contentType: "image/jpeg",
			want:        "image/jpeg",
		},
		{
			name:        "content type with charset",
			data:        nil,
			contentType: "text/html; charset=utf-8",
			want:        "text/html",
		},
		{
			name:        "octet-stream falls through to magic bytes",
			data:        pngHeader,
			contentType: "application/octet-stream",
			want:        "image/png",
		},
		{
			name:        "binary/octet-stream falls through",
			data:        pngHeader,
			contentType: "binary/octet-stream",
			want:        "image/png",
		},
		{
			name:        "empty content type uses magic bytes",
			data:        pngHeader,
			contentType: "",
			want:        "image/png",
		},
		{
			name:        "JPEG magic bytes",
			data:        jpegHeader,
			contentType: "",
			want:        "image/jpeg",
		},
		{
			name:        "GIF magic bytes",
			data:        gifHeader,
			contentType: "",
			want:        "image/gif",
		},
		{
			name:        "empty data empty content type",
			data:        nil,
			contentType: "",
			want:        "application/octet-stream",
		},
		{
			name:        "unknown bytes",
			data:        []byte{0x00, 0x01, 0x02, 0x03},
			contentType: "",
			want:        "application/octet-stream",
		},
		{
			name:        "whitespace-only content type",
			data:        pngHeader,
			contentType: "   ",
			want:        "image/png",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectMIMEType(tt.data, tt.contentType)
			if got != tt.want {
				t.Errorf("DetectMIMEType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectFromBytes(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	got := DetectFromBytes(png)
	if got != "image/png" {
		t.Errorf("DetectFromBytes(PNG) = %q, want image/png", got)
	}

	got = DetectFromBytes(nil)
	if got != "application/octet-stream" {
		t.Errorf("DetectFromBytes(nil) = %q, want application/octet-stream", got)
	}

	got = DetectFromBytes([]byte{})
	if got != "application/octet-stream" {
		t.Errorf("DetectFromBytes(empty) = %q, want application/octet-stream", got)
	}
}
