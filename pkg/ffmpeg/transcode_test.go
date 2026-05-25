package ffmpeg

import (
	"testing"

	"github.com/yuerchu/media-gateway/pkg/model"
)

func TestEnsureEven(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 0},
		{1, 0},
		{2, 2},
		{3, 2},
		{4, 4},
		{99, 98},
		{100, 100},
		{1920, 1920},
		{1921, 1920},
	}
	for _, tt := range tests {
		if got := ensureEven(tt.input); got != tt.want {
			t.Errorf("ensureEven(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestCalculateTargetDimensions(t *testing.T) {
	tests := []struct {
		name         string
		srcW, srcH   int
		maxW, maxH   int
		wantW, wantH int
	}{
		{
			name: "fits within max",
			srcW: 640, srcH: 480, maxW: 1920, maxH: 1080,
			wantW: 640, wantH: 480,
		},
		{
			name: "exact match",
			srcW: 1920, srcH: 1080, maxW: 1920, maxH: 1080,
			wantW: 1920, wantH: 1080,
		},
		{
			name: "4K to 1080p",
			srcW: 3840, srcH: 2160, maxW: 1920, maxH: 1080,
			wantW: 1920, wantH: 1080,
		},
		{
			name:  "scale down limited by height",
			srcW:  1920, srcH: 2160, maxW: 1920, maxH: 1080,
			wantW: 960, wantH: 1080,
		},
		{
			name:  "ultra wide",
			srcW:  3840, srcH: 1080, maxW: 1920, maxH: 1080,
			wantW: 1920, wantH: 540,
		},
		{
			name: "odd source that fits",
			srcW: 641, srcH: 481, maxW: 1920, maxH: 1080,
			wantW: 640, wantH: 480,
		},
		{
			name: "1x1 source",
			srcW: 1, srcH: 1, maxW: 1920, maxH: 1080,
			wantW: 0, wantH: 0,
		},
		{
			name:  "tall portrait video",
			srcW:  1080, srcH: 1920, maxW: 1920, maxH: 1080,
			wantW: 606, wantH: 1080,
		},
		{
			name: "small even source",
			srcW: 2, srcH: 2, maxW: 1920, maxH: 1080,
			wantW: 2, wantH: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH := CalculateTargetDimensions(tt.srcW, tt.srcH, tt.maxW, tt.maxH)
			if gotW != tt.wantW || gotH != tt.wantH {
				t.Errorf("CalculateTargetDimensions(%d, %d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.srcW, tt.srcH, tt.maxW, tt.maxH, gotW, gotH, tt.wantW, tt.wantH)
			}
			if gotW%2 != 0 {
				t.Errorf("width %d is not even", gotW)
			}
			if gotH%2 != 0 {
				t.Errorf("height %d is not even", gotH)
			}
		})
	}
}

func TestCalculateTargetBitrate(t *testing.T) {
	tests := []struct {
		name        string
		maxSize     int64
		duration    float64
		wantMinimum int64
	}{
		{
			name:        "zero duration returns minimum",
			maxSize:     100 * 1024 * 1024,
			duration:    0,
			wantMinimum: 100000,
		},
		{
			name:        "negative duration returns minimum",
			maxSize:     100 * 1024 * 1024,
			duration:    -5,
			wantMinimum: 100000,
		},
		{
			name:     "100MB 60s video",
			maxSize:  100 * 1024 * 1024,
			duration: 60,
		},
		{
			name:        "tiny file yields minimum bitrate",
			maxSize:     1024,
			duration:    3600,
			wantMinimum: 100000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateTargetBitrate(tt.maxSize, tt.duration)
			if got < 100000 {
				t.Errorf("bitrate %d is below minimum 100000", got)
			}
			if tt.wantMinimum > 0 && got < tt.wantMinimum {
				t.Errorf("bitrate %d < expected minimum %d", got, tt.wantMinimum)
			}
			if tt.duration > 0 && tt.maxSize > 0 {
				totalBits := int64(float64(tt.maxSize) * 8 * 0.95 / tt.duration)
				maxExpected := totalBits - 128000
				if maxExpected > 100000 && got != maxExpected {
					t.Errorf("bitrate %d != expected %d", got, maxExpected)
				}
			}
		})
	}
}

func TestCalculateTargetBitrate_AudioReserve(t *testing.T) {
	maxSize := int64(50 * 1024 * 1024)
	dur := 60.0
	bitrate := CalculateTargetBitrate(maxSize, dur)
	totalBitrate := int64(float64(maxSize) * 8 * 0.95 / dur)
	expected := totalBitrate - 128000
	if bitrate != expected {
		t.Errorf("got %d, want %d (total %d minus 128000 audio)", bitrate, expected, totalBitrate)
	}
}

func TestBuildTranscodeArgs(t *testing.T) {
	params := &model.TranscodeParams{
		InputPath:     "/tmp/input.mp4",
		OutputPath:    "/tmp/output.mp4",
		TargetWidth:   1920,
		TargetHeight:  1080,
		TargetBitrate: 5000000,
		VideoCodec:    "libx264",
		AudioCodec:    "aac",
		Preset:        "fast",
	}
	args := BuildTranscodeArgs(params)

	contains := func(args []string, key, val string) bool {
		for i, a := range args {
			if a == key && i+1 < len(args) && args[i+1] == val {
				return true
			}
		}
		return false
	}

	if !contains(args, "-i", "/tmp/input.mp4") {
		t.Error("missing -i input")
	}
	if !contains(args, "-c:v", "libx264") {
		t.Error("missing -c:v")
	}
	if !contains(args, "-preset", "fast") {
		t.Error("missing -preset")
	}
	if !contains(args, "-c:a", "aac") {
		t.Error("missing -c:a")
	}
	if !contains(args, "-b:a", "128k") {
		t.Error("missing -b:a 128k")
	}
	if !contains(args, "-vf", "scale=1920:1080") {
		t.Error("missing -vf scale")
	}
	if !contains(args, "-movflags", "+faststart") {
		t.Error("missing -movflags")
	}
	if args[len(args)-1] != "/tmp/output.mp4" {
		t.Error("output path should be last arg")
	}
}

func TestBuildTranscodeArgs_WithMaxDuration(t *testing.T) {
	params := &model.TranscodeParams{
		InputPath:     "/tmp/in.mp4",
		OutputPath:    "/tmp/out.mp4",
		TargetWidth:   1280,
		TargetHeight:  720,
		TargetBitrate: 2000000,
		MaxDuration:   30.5,
		VideoCodec:    "libx264",
		AudioCodec:    "aac",
		Preset:        "medium",
	}
	args := BuildTranscodeArgs(params)

	found := false
	for i, a := range args {
		if a == "-t" {
			found = true
			if i+1 >= len(args) {
				t.Fatal("-t flag has no value")
			}
			if args[i+1] != "30.500" {
				t.Errorf("-t value = %s, want 30.500", args[i+1])
			}
		}
	}
	if !found {
		t.Error("-t flag not found for MaxDuration > 0")
	}
}

func TestBuildTranscodeArgs_NoMaxDuration(t *testing.T) {
	params := &model.TranscodeParams{
		InputPath:     "/tmp/in.mp4",
		OutputPath:    "/tmp/out.mp4",
		TargetWidth:   1280,
		TargetHeight:  720,
		TargetBitrate: 2000000,
		VideoCodec:    "libx264",
		AudioCodec:    "aac",
		Preset:        "medium",
	}
	args := BuildTranscodeArgs(params)
	for _, a := range args {
		if a == "-t" {
			t.Error("-t flag should not be present when MaxDuration is 0")
		}
	}
}

func TestBuildExtractFrameArgs(t *testing.T) {
	params := &model.ExtractFrameParams{
		InputPath:  "/tmp/video.mp4",
		OutputPath: "/tmp/frame.webp",
		MaxWidth:   400,
	}
	args := BuildExtractFrameArgs(params)

	if args[0] != "-y" {
		t.Error("first arg should be -y")
	}
	if args[len(args)-1] != "/tmp/frame.webp" {
		t.Error("output path should be last arg")
	}

	hasFrames := false
	hasFormat := false
	for i, a := range args {
		if a == "-frames:v" && i+1 < len(args) && args[i+1] == "1" {
			hasFrames = true
		}
		if a == "-f" && i+1 < len(args) && args[i+1] == "webp" {
			hasFormat = true
		}
	}
	if !hasFrames {
		t.Error("missing -frames:v 1")
	}
	if !hasFormat {
		t.Error("missing -f webp")
	}
}
