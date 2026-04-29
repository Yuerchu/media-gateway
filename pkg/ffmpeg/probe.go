package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"

	jsoniter "github.com/json-iterator/go"

	"github.com/yuerchu/media-gateway/pkg/model"
)

// ffprobeOutput is the top-level ffprobe JSON output.
type ffprobeOutput struct {
	Format  ffprobeFormat   `json:"format"`
	Streams []ffprobeStream `json:"streams"`
}

type ffprobeFormat struct {
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
	BitRate    string `json:"bit_rate"`
}

type ffprobeStream struct {
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// parseProbeOutput parses ffprobe JSON output into ProbeResult.
func parseProbeOutput(data []byte) (*model.ProbeResult, error) {
	var output ffprobeOutput
	if err := jsoniter.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("parsing ffprobe output: %w", err)
	}

	duration, err := strconv.ParseFloat(output.Format.Duration, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing duration %q: %w", output.Format.Duration, err)
	}

	bitrate, err := strconv.ParseInt(output.Format.BitRate, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing bitrate %q: %w", output.Format.BitRate, err)
	}

	result := &model.ProbeResult{
		DurationSec: duration,
		Bitrate:     bitrate,
		FormatName:  output.Format.FormatName,
	}

	for _, s := range output.Streams {
		switch s.CodecType {
		case "video":
			result.Width = s.Width
			result.Height = s.Height
			result.VideoCodec = s.CodecName
		case "audio":
			result.AudioCodec = s.CodecName
		}
	}

	if result.VideoCodec == "" {
		return nil, fmt.Errorf("no video stream found")
	}

	return result, nil
}

// Probe runs ffprobe on inputPath and extracts video metadata.
func Probe(ctx context.Context, ffprobePath string, inputPath string) (*model.ProbeResult, error) {
	cmd := exec.CommandContext(ctx, ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w, stderr: %s", err, stderr.String())
	}

	return parseProbeOutput(stdout.Bytes())
}
