package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

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
	CodecType  string `json:"codec_type"`
	CodecName  string `json:"codec_name"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	RFrameRate string `json:"r_frame_rate"`
	BitRate    string `json:"bit_rate"`
	SampleRate string `json:"sample_rate"`
}

// parseFrameRate parses ffprobe r_frame_rate format "num/den" into float64.
func parseFrameRate(s string) float64 {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return 0
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}
	return num / den
}

// parseProbeOutput parses ffprobe JSON output into ProbeResult.
func parseProbeOutput(data []byte) (*model.ProbeResult, error) {
	var output ffprobeOutput
	if err := jsoniter.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("parsing ffprobe output: %w", err)
	}

	duration, _ := strconv.ParseFloat(output.Format.Duration, 64)
	bitrate, _ := strconv.ParseInt(output.Format.BitRate, 10, 64)

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
			result.VideoFrameRate = parseFrameRate(s.RFrameRate)
			result.VideoBitrate, _ = strconv.ParseInt(s.BitRate, 10, 64)
		case "audio":
			result.AudioCodec = s.CodecName
			result.AudioBitrate, _ = strconv.ParseInt(s.BitRate, 10, 64)
			result.AudioSampleRate, _ = strconv.Atoi(s.SampleRate)
		}
	}

	return result, nil
}

// Probe runs ffprobe on inputPath and extracts media metadata.
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
