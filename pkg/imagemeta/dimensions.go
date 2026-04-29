package imagemeta

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"

	_ "golang.org/x/image/webp"
)

// ExtractDimensions reads the image header to extract width and height
// without decoding the full pixel data.
func ExtractDimensions(data []byte) (width, height int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		slog.Warn("Failed to extract image dimensions", "error", err)
		return 0, 0
	}
	return cfg.Width, cfg.Height
}
