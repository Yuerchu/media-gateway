package thumbnail

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Params holds thumbnail generation parameters.
type Params struct {
	InputPath  string
	OutputPath string
	MIMEType   string
	Width      int
	Height     int
}

// Generate creates a WebP thumbnail using ffmpeg.
// For images: resize to fit within Width×Height.
// For videos: pick the most representative frame via the thumbnail filter.
// For audio: extract the embedded cover art (fails if none exists).
func Generate(ctx context.Context, ffmpegPath string, p *Params) error {
	var args []string
	switch {
	case strings.HasPrefix(p.MIMEType, "image/"):
		args = buildImageArgs(p)
	case strings.HasPrefix(p.MIMEType, "video/"):
		args = buildVideoArgs(p)
	case strings.HasPrefix(p.MIMEType, "audio/"):
		args = buildAudioArgs(p)
	default:
		return fmt.Errorf("unsupported MIME type for thumbnail: %s", p.MIMEType)
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w, stderr: %s", err, stderr.String())
	}
	return nil
}

func scaleFilter(w, h int) string {
	return fmt.Sprintf("scale='min(%d,iw)':'min(%d,ih)':force_original_aspect_ratio=decrease", w, h)
}

func buildImageArgs(p *Params) []string {
	return []string{
		"-y",
		"-i", p.InputPath,
		"-vf", scaleFilter(p.Width, p.Height),
		"-frames:v", "1",
		"-f", "webp",
		"-quality", "80",
		p.OutputPath,
	}
}

func buildVideoArgs(p *Params) []string {
	return []string{
		"-y",
		"-i", p.InputPath,
		"-vf", fmt.Sprintf("thumbnail=300,%s", scaleFilter(p.Width, p.Height)),
		"-frames:v", "1",
		"-f", "webp",
		"-quality", "80",
		p.OutputPath,
	}
}

func buildAudioArgs(p *Params) []string {
	return []string{
		"-y",
		"-i", p.InputPath,
		"-an",
		"-vf", scaleFilter(p.Width, p.Height),
		"-frames:v", "1",
		"-f", "webp",
		"-quality", "80",
		p.OutputPath,
	}
}
