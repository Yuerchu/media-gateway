package ffmpeg

import (
	"testing"

	"github.com/yuerchu/media-gateway/pkg/model"
)

func TestBuildAudioTranscodeArgs_FileOutput_AAC(t *testing.T) {
	params := &model.AudioTranscodeParams{
		InputPath:  "/tmp/input.flac",
		OutputPath: "/tmp/output.m4a",
		Codec:      "aac",
		Bitrate:    "320k",
		SampleRate: 48000,
	}
	args := BuildAudioTranscodeArgs(params)

	assertArg(t, args, "-y")
	assertArgPair(t, args, "-i", "/tmp/input.flac")
	assertArg(t, args, "-vn")
	assertArgPair(t, args, "-c:a", "aac")
	assertArgPair(t, args, "-b:a", "320k")
	assertArgPair(t, args, "-ar", "48000")
	assertArgPair(t, args, "-f", "mp4")
	assertArgPair(t, args, "-movflags", "+faststart")

	if args[len(args)-1] != "/tmp/output.m4a" {
		t.Error("last arg should be output path")
	}
}

func TestBuildAudioTranscodeArgs_FileOutput_FLAC(t *testing.T) {
	params := &model.AudioTranscodeParams{
		InputPath:  "/tmp/input.wav",
		OutputPath: "/tmp/output.flac",
		Codec:      "flac",
		SampleRate: 44100,
	}
	args := BuildAudioTranscodeArgs(params)

	assertArgPair(t, args, "-c:a", "flac")
	assertArgPair(t, args, "-f", "flac")
	assertArgPair(t, args, "-ar", "44100")

	for _, a := range args {
		if a == "-movflags" {
			t.Error("FLAC should not have -movflags")
		}
	}

	if args[len(args)-1] != "/tmp/output.flac" {
		t.Error("last arg should be output path")
	}
}

func TestBuildAudioTranscodeArgs_PipeOutput_AAC(t *testing.T) {
	params := &model.AudioTranscodeParams{
		InputPath: "/tmp/input.flac",
		Codec:     "aac",
		Bitrate:   "128k",
	}
	args := BuildAudioTranscodeArgs(params)

	assertArgPair(t, args, "-f", "adts")

	if args[len(args)-1] != "pipe:1" {
		t.Errorf("last arg = %s, want pipe:1", args[len(args)-1])
	}

	for _, a := range args {
		if a == "-movflags" {
			t.Error("pipe mode should not have -movflags")
		}
	}
}

func TestBuildAudioTranscodeArgs_PipeOutput_FLAC(t *testing.T) {
	params := &model.AudioTranscodeParams{
		InputPath: "/tmp/input.wav",
		Codec:     "flac",
	}
	args := BuildAudioTranscodeArgs(params)

	assertArgPair(t, args, "-f", "flac")
	if args[len(args)-1] != "pipe:1" {
		t.Errorf("last arg = %s, want pipe:1", args[len(args)-1])
	}
}

func TestBuildAudioTranscodeArgs_NoBitrate(t *testing.T) {
	params := &model.AudioTranscodeParams{
		InputPath:  "/tmp/input.wav",
		OutputPath: "/tmp/output.flac",
		Codec:      "flac",
	}
	args := BuildAudioTranscodeArgs(params)

	for _, a := range args {
		if a == "-b:a" {
			t.Error("-b:a should not be present when Bitrate is empty")
		}
	}
}

func TestBuildAudioTranscodeArgs_WithChannels(t *testing.T) {
	params := &model.AudioTranscodeParams{
		InputPath:  "/tmp/input.wav",
		OutputPath: "/tmp/output.m4a",
		Codec:      "aac",
		Bitrate:    "128k",
		Channels:   2,
	}
	args := BuildAudioTranscodeArgs(params)
	assertArgPair(t, args, "-ac", "2")
}

func TestBuildAudioTranscodeArgs_NoChannels(t *testing.T) {
	params := &model.AudioTranscodeParams{
		InputPath:  "/tmp/input.wav",
		OutputPath: "/tmp/output.m4a",
		Codec:      "aac",
		Bitrate:    "128k",
	}
	args := BuildAudioTranscodeArgs(params)
	for _, a := range args {
		if a == "-ac" {
			t.Error("-ac should not be present when Channels is 0")
		}
	}
}

func TestBuildAudioTranscodeArgs_NoSampleRate(t *testing.T) {
	params := &model.AudioTranscodeParams{
		InputPath:  "/tmp/input.flac",
		OutputPath: "/tmp/output.m4a",
		Codec:      "aac",
		Bitrate:    "320k",
	}
	args := BuildAudioTranscodeArgs(params)
	for _, a := range args {
		if a == "-ar" {
			t.Error("-ar should not be present when SampleRate is 0")
		}
	}
}

func assertArg(t *testing.T, args []string, val string) {
	t.Helper()
	for _, a := range args {
		if a == val {
			return
		}
	}
	t.Errorf("args missing %s", val)
}

func assertArgPair(t *testing.T, args []string, key, val string) {
	t.Helper()
	for i, a := range args {
		if a == key && i+1 < len(args) && args[i+1] == val {
			return
		}
	}
	t.Errorf("args missing %s %s", key, val)
}
