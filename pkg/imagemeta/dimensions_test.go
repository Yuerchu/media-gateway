package imagemeta

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func createTestPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func createTestJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 50})
	return buf.Bytes()
}

func TestExtractDimensions_PNG(t *testing.T) {
	tests := []struct {
		name   string
		w, h   int
	}{
		{"1x1", 1, 1},
		{"100x200", 100, 200},
		{"1920x1080", 1920, 1080},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := createTestPNG(tt.w, tt.h)
			gotW, gotH := ExtractDimensions(data)
			if gotW != tt.w || gotH != tt.h {
				t.Errorf("ExtractDimensions() = (%d, %d), want (%d, %d)", gotW, gotH, tt.w, tt.h)
			}
		})
	}
}

func TestExtractDimensions_JPEG(t *testing.T) {
	data := createTestJPEG(640, 480)
	w, h := ExtractDimensions(data)
	if w != 640 || h != 480 {
		t.Errorf("ExtractDimensions(JPEG) = (%d, %d), want (640, 480)", w, h)
	}
}

func TestExtractDimensions_InvalidData(t *testing.T) {
	w, h := ExtractDimensions([]byte("not an image"))
	if w != 0 || h != 0 {
		t.Errorf("ExtractDimensions(invalid) = (%d, %d), want (0, 0)", w, h)
	}
}

func TestExtractDimensions_EmptyData(t *testing.T) {
	w, h := ExtractDimensions(nil)
	if w != 0 || h != 0 {
		t.Errorf("ExtractDimensions(nil) = (%d, %d), want (0, 0)", w, h)
	}
}
