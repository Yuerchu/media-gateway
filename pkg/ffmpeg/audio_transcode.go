package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strconv"

	"github.com/yuerchu/media-gateway/pkg/model"
)

// BuildAudioTranscodeArgs constructs ffmpeg arguments for audio-only transcoding.
// When OutputPath is empty, output goes to stdout (pipe:1) with an appropriate
// streaming container format (ADTS for AAC, raw FLAC for FLAC).
// When OutputPath is set, output goes to a file with faststart for AAC (M4A).
func BuildAudioTranscodeArgs(params *model.AudioTranscodeParams) []string {
	args := []string{"-y", "-i", params.InputPath, "-vn"}

	args = append(args, "-c:a", params.Codec)

	if params.Bitrate != "" {
		args = append(args, "-b:a", params.Bitrate)
	}

	if params.SampleRate > 0 {
		args = append(args, "-ar", strconv.Itoa(params.SampleRate))
	}

	if params.Channels > 0 {
		args = append(args, "-ac", strconv.Itoa(params.Channels))
	}

	if params.OutputPath != "" {
		// File output
		switch params.Codec {
		case "aac":
			args = append(args, "-f", "mp4", "-movflags", "+faststart")
		case "flac":
			args = append(args, "-f", "flac")
		}
		args = append(args, params.OutputPath)
	} else {
		// Pipe output (stdout)
		switch params.Codec {
		case "aac":
			args = append(args, "-f", "adts")
		case "flac":
			args = append(args, "-f", "flac")
		}
		args = append(args, "pipe:1")
	}

	return args
}

// AudioTranscode runs ffmpeg to transcode audio to a file. Blocks until complete.
func AudioTranscode(ctx context.Context, ffmpegPath string, params *model.AudioTranscodeParams) error {
	args := BuildAudioTranscodeArgs(params)
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	slog.Info("Starting audio transcode",
		"input", params.InputPath,
		"output", params.OutputPath,
		"codec", params.Codec,
		"bitrate", params.Bitrate,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg audio transcode failed: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// AudioTranscodeStream starts ffmpeg and returns a reader connected to its stdout.
// The caller must read until EOF, then call cmd.Wait() to release resources.
// Cancel the context to abort the transcode.
func AudioTranscodeStream(ctx context.Context, ffmpegPath string, params *model.AudioTranscodeParams) (io.ReadCloser, *exec.Cmd, error) {
	p := *params
	p.OutputPath = ""
	args := BuildAudioTranscodeArgs(&p)

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	slog.Info("Starting audio transcode stream",
		"input", params.InputPath,
		"codec", params.Codec,
		"bitrate", params.Bitrate,
	)

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("starting ffmpeg: %w, stderr: %s", err, stderr.String())
	}

	return stdout, cmd, nil
}
