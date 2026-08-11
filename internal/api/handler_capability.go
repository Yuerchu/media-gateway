package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yuerchu/media-gateway/internal/config"
)

// CapabilityHandler serves the published capability contract.
type CapabilityHandler struct {
	document []byte
	etag     string
}

// NewCapabilityHandler renders the contract once and holds the bytes.
//
// It returns an error rather than rendering per request so that a contract
// which cannot be built stops the process at startup. The alternative is a
// service that comes up fine and then answers every capability request with a
// 500 — the failure would be found by a consumer rather than by a deploy.
//
// Configuration is read only here, which is safe because nothing reloads it at
// runtime.
func NewCapabilityHandler(cfg *config.Config) (*CapabilityHandler, error) {
	document, err := renderCapabilityDocument(cfg)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(document)
	return &CapabilityHandler{
		document: document,
		etag:     `"` + hex.EncodeToString(digest[:]) + `"`,
	}, nil
}

// Get returns the capability contract.
//
// The bytes are served with an ETag because they only change on redeploy, and
// a consumer that refetches on a schedule should be able to find out cheaply
// that nothing moved. That works only because rendering is deterministic — see
// Document.RenderJSON.
func (h *CapabilityHandler) Get(c *gin.Context) {
	c.Header("ETag", h.etag)
	c.Header("Cache-Control", "no-cache")
	if c.GetHeader("If-None-Match") == h.etag {
		c.Status(http.StatusNotModified)
		return
	}
	// application/schema+json is the media type registered for JSON Schema
	// documents (RFC draft), and it tells a consumer this is a schema rather
	// than an arbitrary JSON payload.
	c.Data(http.StatusOK, "application/schema+json; charset=utf-8", h.document)
}
