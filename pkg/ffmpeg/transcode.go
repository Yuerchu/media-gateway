package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"

	"github.com/yuerchu/media-gateway/pkg/model"
)

// Bitrate budget constants. CalculateTargetBitrate and BuildTranscodeArgs must
// both derive from these: the audio allowance subtracted from the size budget
// and the audio bitrate actually encoded are the same number, and keeping them
// as two independent literals let them drift silently.
const (
	// AudioBitrateBps is the bitrate audio is encoded at, and the allowance
	// reserved for audio when computing the video bitrate budget.
	AudioBitrateBps int64 = 128000

	// MinVideoBitrateBps is the floor applied to the computed video bitrate.
	// Reaching it means the size budget cannot be honoured — the output will
	// exceed maxSizeBytes. See TranscodeCapability for how this is declared.
	MinVideoBitrateBps int64 = 100000

	// ContainerOverheadRatio is the fraction of the size budget available to
	// media data; the remainder covers container overhead.
	ContainerOverheadRatio = 0.95
)

// CalculateTargetDimensions computes target width/height preserving aspect ratio.
// Result dimensions are always even (H.264 requirement).
func CalculateTargetDimensions(srcW, srcH, maxW, maxH int) (int, int) {
	if srcW <= maxW && srcH <= maxH {
		return ensureEven(srcW), ensureEven(srcH)
	}

	ratioW := float64(maxW) / float64(srcW)
	ratioH := float64(maxH) / float64(srcH)
	ratio := ratioW
	if ratioH < ratioW {
		ratio = ratioH
	}

	targetW := int(float64(srcW) * ratio)
	targetH := int(float64(srcH) * ratio)
	return ensureEven(targetW), ensureEven(targetH)
}

// ensureEven rounds down to nearest even number.
func ensureEven(n int) int {
	if n%2 != 0 {
		return n - 1
	}
	return n
}

// CalculateTargetBitrate computes video bitrate to fit within maxSizeBytes.
// Reserves AudioBitrateBps for audio and ContainerOverheadRatio for container
// overhead.
//
// The result is clamped to MinVideoBitrateBps, so fitting maxSizeBytes is a
// best effort, not a guarantee: a long source with a small size budget yields a
// computed bitrate below the floor, and the encode then overshoots the budget.
// Callers that must not overshoot have to verify the output themselves.
func CalculateTargetBitrate(maxSizeBytes int64, durationSec float64) int64 {
	if durationSec <= 0 {
		return MinVideoBitrateBps
	}
	totalBitrate := int64(float64(maxSizeBytes) * 8 * ContainerOverheadRatio / durationSec)
	videoBitrate := totalBitrate - AudioBitrateBps
	if videoBitrate < MinVideoBitrateBps {
		videoBitrate = MinVideoBitrateBps
	}
	return videoBitrate
}

// BuildTranscodeArgs constructs ffmpeg command line arguments from TranscodeParams.
func BuildTranscodeArgs(params *model.TranscodeParams) []string {
	args := []string{
		"-y",
		"-i", params.InputPath,
		"-c:v", params.VideoCodec,
		"-preset", params.Preset,
		"-b:v", strconv.FormatInt(params.TargetBitrate, 10),
		"-maxrate", strconv.FormatInt(params.TargetBitrate*2, 10),
		"-bufsize", strconv.FormatInt(params.TargetBitrate*4, 10),
		"-c:a", params.AudioCodec,
		// Derived from AudioBitrateBps so the encoded audio bitrate and the
		// allowance CalculateTargetBitrate subtracts can never disagree.
		// ffmpeg's "k" suffix is 1000, matching the bps constant exactly.
		"-b:a", strconv.FormatInt(AudioBitrateBps/1000, 10) + "k",
		"-vf", fmt.Sprintf("scale=%d:%d", params.TargetWidth, params.TargetHeight),
		"-movflags", "+faststart",
	}
	if params.MaxDuration > 0 {
		args = append(args, "-t", strconv.FormatFloat(params.MaxDuration, 'f', 3, 64))
	}
	args = append(args, params.OutputPath)
	return args
}

// BuildExtractFrameArgs constructs ffmpeg arguments to extract one frame as WebP.
func BuildExtractFrameArgs(params *model.ExtractFrameParams) []string {
	return []string{
		"-y",
		"-ss", "0",
		"-i", params.InputPath,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale='min(%d,iw)':'-1'", params.MaxWidth),
		"-f", "webp",
		params.OutputPath,
	}
}

// ExtractFrame extracts the first video frame as a WebP image.
func ExtractFrame(ctx context.Context, ffmpegPath string, params *model.ExtractFrameParams) error {
	args := BuildExtractFrameArgs(params)
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	slog.Info("Extracting frame",
		"input", params.InputPath,
		"output", params.OutputPath,
		"max_width", params.MaxWidth,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg extract_frame failed: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// Transcode executes ffmpeg with the given parameters.
func Transcode(ctx context.Context, ffmpegPath string, params *model.TranscodeParams) error {
	args := BuildTranscodeArgs(params)
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	slog.Info("Starting ffmpeg transcode",
		"input", params.InputPath,
		"output", params.OutputPath,
		"resolution", fmt.Sprintf("%dx%d", params.TargetWidth, params.TargetHeight),
		"bitrate", params.TargetBitrate,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w, stderr: %s", err, stderr.String())
	}

	return nil
}
