package ffmpeg

import (
	"strings"
	"testing"

	"github.com/yuerchu/media-gateway/pkg/capability"
	"github.com/yuerchu/media-gateway/pkg/model"
)

func mustCapability(t *testing.T, build func() (capability.Operation, error)) capability.Operation {
	t.Helper()
	op, err := build()
	if err != nil {
		t.Fatalf("building capability = %v, want no error", err)
	}
	return op
}

func TestCapabilities_AllDeclarationsAreWellFormed(t *testing.T) {
	// NewOperation validates on construction, so this is the boot check a
	// service performs, run here where a failure blocks the merge instead.
	operations, err := Capabilities()
	if err != nil {
		t.Fatalf("Capabilities() = %v, want no error", err)
	}
	if len(operations) != 2 {
		t.Fatalf("Capabilities() returned %d operations, want 2", len(operations))
	}
}

func TestTranscodeCapability_ReturnsIndependentValues(t *testing.T) {
	// The declarations are built fresh per call rather than kept in a package
	// var. A shared value would hand every caller slices over the same backing
	// array, which under -race is a live data race the moment two services
	// narrow it concurrently.
	first := mustCapability(t, TranscodeCapability)
	second := mustCapability(t, TranscodeCapability)

	first.Produces()[0] = "video/tampered"
	if got := second.Produces()[0]; got != "video/mp4" {
		t.Errorf("second call Produces()[0] = %q after mutating the first, want %q", got, "video/mp4")
	}
}

func TestTranscodeCapability_Guarantees(t *testing.T) {
	op := mustCapability(t, TranscodeCapability)
	tests := []struct {
		dimension capability.Dimension
		want      capability.Guarantee
		why       string
	}{
		{capability.DimContentType, capability.GuaranteeExact, "the output container is hard-wired"},
		{capability.DimWidth, capability.GuaranteeExact, "scale=W:H is passed verbatim to the encoder"},
		{capability.DimHeight, capability.GuaranteeExact, "scale=W:H is passed verbatim to the encoder"},
		{capability.DimDuration, capability.GuaranteeExact, "-t truncates a long source"},
		{capability.DimSize, capability.GuaranteeBestEffort, "the bitrate floor can overshoot the budget"},
		{capability.DimFrameRate, capability.GuaranteeNone, "no frame rate argument is ever passed"},
	}
	for _, tt := range tests {
		if got := op.Guarantee(tt.dimension); got != tt.want {
			t.Errorf("Guarantee(%q) = %q, want %q — %s", tt.dimension, got, tt.want, tt.why)
		}
	}
}

func TestTranscodeCapability_SizeModelMatchesImplementation(t *testing.T) {
	// The published formula is what a caller uses to decide whether its own
	// request is reachable. If it drifts from CalculateTargetBitrate, callers
	// start pre-approving requests that are certain to fail.
	op := mustCapability(t, TranscodeCapability)

	var constants map[string]float64
	for _, g := range op.Guarantees() {
		if g.Dimension != capability.DimSize {
			continue
		}
		if g.Model == nil {
			t.Fatal("size is graded best_effort but carries no model")
		}
		constants = make(map[string]float64, len(g.Model.Constants))
		for _, c := range g.Model.Constants {
			constants[c.Name] = c.Value
		}
	}
	if constants == nil {
		t.Fatal("no guarantee published for the size dimension")
	}

	want := map[string]float64{
		"audio_bitrate_bps":        float64(AudioBitrateBps),
		"min_video_bitrate_bps":    float64(MinVideoBitrateBps),
		"container_overhead_ratio": ContainerOverheadRatio,
	}
	for name, wantValue := range want {
		if got, ok := constants[name]; !ok {
			t.Errorf("published model has no constant %q", name)
		} else if got != wantValue {
			t.Errorf("published %s = %v, implementation uses %v", name, got, wantValue)
		}
	}

	// And the declared failure mode really is reachable: a long source with a
	// small budget bottoms out at the floor, which is exactly when the encode
	// overshoots.
	if got := CalculateTargetBitrate(1024, 3600); got != MinVideoBitrateBps {
		t.Errorf("CalculateTargetBitrate(1024, 3600) = %d, want the floor %d", got, MinVideoBitrateBps)
	}
}

func TestTranscodeCapability_MinDimensionCoversTheDegenerateCase(t *testing.T) {
	// The precondition exists because rounding down to even turns a 1px edge
	// into 0px, and "scale=0:0" fails inside the encoder with a message about
	// something else entirely. If CalculateTargetDimensions ever stops
	// degenerating, this precondition should be reconsidered rather than left
	// as folklore.
	op := mustCapability(t, TranscodeCapability)
	minDim := op.SourceRequires().MinDimension
	if minDim < 2 {
		t.Fatalf("MinDimension = %d, want at least 2", minDim)
	}

	if w, h := CalculateTargetDimensions(minDim-1, minDim-1, 1920, 1080); w != 0 || h != 0 {
		t.Errorf("CalculateTargetDimensions(%d, %d, ...) = (%d, %d), want (0, 0): "+
			"the degenerate case the MinDimension precondition guards against has changed",
			minDim-1, minDim-1, w, h)
	}
	if w, h := CalculateTargetDimensions(minDim, minDim, 1920, 1080); w == 0 || h == 0 {
		t.Errorf("CalculateTargetDimensions(%d, %d, ...) = (%d, %d), want non-zero: "+
			"MinDimension is set too low", minDim, minDim, w, h)
	}
}

func TestTranscodeCapability_RequiresVideoStream(t *testing.T) {
	op := mustCapability(t, TranscodeCapability)
	streams := op.SourceRequires().Streams
	if len(streams) != 1 || streams[0] != "video" {
		t.Fatalf("SourceRequires().Streams = %v, want [video]", streams)
	}

	// The precondition is only meaningful if callers have something to test it
	// with, and only necessary because probing accepts audio-only input.
	audioOnly := &model.ProbeResult{FormatName: "ogg", AudioCodec: "vorbis", DurationSec: 120}
	if audioOnly.HasVideoStream() {
		t.Error("HasVideoStream() = true for an audio-only probe result")
	}
	if !(&model.ProbeResult{VideoCodec: "h264"}).HasVideoStream() {
		t.Error("HasVideoStream() = false for a probe result carrying video")
	}
}

func TestExtractFrameCapability_HeightIsUnbounded(t *testing.T) {
	// The single most useful fact in this contract: max_height is a parameter
	// a service will happily accept and this operation cannot honour, because
	// the scale filter derives height from the aspect ratio. Anyone adding a
	// height field to ExtractFrameParams should land here.
	op := mustCapability(t, ExtractFrameCapability)
	if got := op.Guarantee(capability.DimHeight); got != capability.GuaranteeNone {
		t.Errorf("Guarantee(height) = %q, want %q", got, capability.GuaranteeNone)
	}

	args := BuildExtractFrameArgs(&model.ExtractFrameParams{
		InputPath:  "/tmp/in.mp4",
		OutputPath: "/tmp/out.webp",
		MaxWidth:   400,
	})
	filter := argValue(t, args, "-vf")
	if !strings.Contains(filter, ":'-1'") {
		t.Errorf("scale filter %q no longer derives height from the aspect ratio; "+
			"the height guarantee may now be stronger than none", filter)
	}
	if !strings.Contains(filter, "min(400,iw)") {
		t.Errorf("scale filter %q no longer bounds width by the requested maximum", filter)
	}
}

func TestExtractFrameCapability_ProducesWebPOnly(t *testing.T) {
	op := mustCapability(t, ExtractFrameCapability)
	produces := op.Produces()
	if len(produces) != 1 || produces[0] != "image/webp" {
		t.Fatalf("Produces() = %v, want [image/webp]", produces)
	}
	if got := argValue(t, BuildExtractFrameArgs(&model.ExtractFrameParams{MaxWidth: 400}), "-f"); got != "webp" {
		t.Errorf("-f = %q, want %q: the declared output format no longer matches the encoder", got, "webp")
	}
}

// argValue returns the argument following flag.
func argValue(t *testing.T, args []string, flag string) string {
	t.Helper()
	for i, a := range args {
		if a == flag {
			if i+1 >= len(args) {
				t.Fatalf("flag %q has no value in %v", flag, args)
			}
			return args[i+1]
		}
	}
	t.Fatalf("flag %q not found in %v", flag, args)
	return ""
}
