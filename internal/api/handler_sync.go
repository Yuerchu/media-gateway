package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yuerchu/media-gateway/pkg/imagemeta"
	mgmime "github.com/yuerchu/media-gateway/pkg/mime"
)

// SyncHandler handles synchronous utility endpoints.
type SyncHandler struct{}

// NewSyncHandler creates a sync handler.
func NewSyncHandler() *SyncHandler {
	return &SyncHandler{}
}

// DetectMIME handles POST /api/v1/detect-mime.
// Accepts multipart file upload.
func (h *SyncHandler) DetectMIME(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file upload required"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, 8192))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	mimeType := mgmime.DetectFromBytes(data)

	c.JSON(http.StatusOK, gin.H{
		"mime_type": mimeType,
	})
}

// ImageMeta handles POST /api/v1/image-meta.
// Accepts multipart file upload.
func (h *SyncHandler) ImageMeta(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file upload required"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	width, height := imagemeta.ExtractDimensions(data)
	mimeType := mgmime.DetectFromBytes(data[:min(len(data), 8192)])

	c.JSON(http.StatusOK, gin.H{
		"width":     width,
		"height":    height,
		"mime_type": mimeType,
	})
}
