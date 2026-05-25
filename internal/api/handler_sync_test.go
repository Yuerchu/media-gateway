package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupSyncRouter() *gin.Engine {
	r := gin.New()
	h := NewSyncHandler()
	r.POST("/detect-mime", h.DetectMIME)
	r.POST("/image-meta", h.ImageMeta)
	return r
}

func createMultipartFile(fieldName, fileName string, data []byte) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile(fieldName, fileName)
	part.Write(data)
	writer.Close()
	return body, writer.FormDataContentType()
}

func TestDetectMIME_PNG(t *testing.T) {
	pngData := createTestPNG(1, 1)
	body, ct := createMultipartFile("file", "test.png", pngData)

	r := setupSyncRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/detect-mime", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["mime_type"] != "image/png" {
		t.Errorf("mime_type = %q, want image/png", resp["mime_type"])
	}
}

func TestDetectMIME_NoFile(t *testing.T) {
	r := setupSyncRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/detect-mime", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestImageMeta_PNG(t *testing.T) {
	pngData := createTestPNG(320, 240)
	body, ct := createMultipartFile("file", "test.png", pngData)

	r := setupSyncRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/image-meta", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if int(resp["width"].(float64)) != 320 {
		t.Errorf("width = %v, want 320", resp["width"])
	}
	if int(resp["height"].(float64)) != 240 {
		t.Errorf("height = %v, want 240", resp["height"])
	}
	if resp["mime_type"] != "image/png" {
		t.Errorf("mime_type = %v, want image/png", resp["mime_type"])
	}
}

func TestImageMeta_NoFile(t *testing.T) {
	r := setupSyncRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/image-meta", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestDetectMIME_UnknownBytes(t *testing.T) {
	body, ct := createMultipartFile("file", "unknown.bin", []byte{0x00, 0x01, 0x02})

	r := setupSyncRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/detect-mime", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["mime_type"] != "application/octet-stream" {
		t.Errorf("mime_type = %q, want application/octet-stream", resp["mime_type"])
	}
}

func TestDetectMIME_LargeFile_Only8KB(t *testing.T) {
	big := make([]byte, 16384)
	copy(big, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	body, ct := createMultipartFile("file", "big.png", big)

	r := setupSyncRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/detect-mime", body)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func createTestPNG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	png.Encode(&buf, img)
	data, _ := io.ReadAll(&buf)
	return data
}
