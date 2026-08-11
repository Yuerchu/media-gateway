package ffmpeg

import (
	"fmt"

	"github.com/yuerchu/media-gateway/pkg/capability"
)

// The declarations in this file state what the functions in this package
// promise about their output and demand of their input. They live here rather
// than next to whichever service publishes them so that changing
// BuildTranscodeArgs, CalculateTargetDimensions, CalculateTargetBitrate or
// BuildExtractFrameArgs puts the claim about that behaviour in the same diff.
//
// They cover output promises and source preconditions only. How a request
// reaches these functions — its schema, which of its fields are accepted and
// ignored, what status a refusal returns — differs per service and is supplied
// by the caller through capability.Operation's With* methods.

// TranscodeCapability describes the video transcoding pipeline: target
// dimensions from CalculateTargetDimensions, a video bitrate from
// CalculateTargetBitrate, and the encode itself from BuildTranscodeArgs. The
// three are described as one operation because no caller uses them apart, and
// the size guarantee in particular only means anything for the combination.
//
// Keep this in step with those three functions.
func TranscodeCapability() (capability.Operation, error) {
	return capability.NewOperation(capability.OperationSpec{
		Name: "transcode",

		// Only the container this package is actually exercised with. The
		// encode is hard-wired to H.264 video in an MP4-shaped stream — the
		// "+faststart" flag in BuildTranscodeArgs is an MP4/MOV construct — and
		// listing formats ffmpeg could theoretically emit would make this
		// contract's very first statement a false one.
		Produces: []string{"video/mp4"},

		Guarantees: map[capability.Dimension]capability.Guarantee{
			capability.DimContentType: capability.GuaranteeExact,
			capability.DimWidth:       capability.GuaranteeExact,
			capability.DimHeight:      capability.GuaranteeExact,
			capability.DimDuration:    capability.GuaranteeExact,
			capability.DimSize:        capability.GuaranteeBestEffort,
			capability.DimFrameRate:   capability.GuaranteeNone,
		},

		Notes: map[capability.Dimension]string{
			capability.DimWidth: "Scaled down to fit the requested box, never enlarged, " +
				"then rounded down to an even number as H.264 requires. Aspect ratio is preserved.",
			capability.DimHeight: "Scaled down to fit the requested box, never enlarged, " +
				"then rounded down to an even number as H.264 requires. Aspect ratio is preserved.",
			capability.DimDuration: "A source longer than the requested bound is truncated to it. " +
				"A requested lower bound is not honoured here — nothing can lengthen a source — " +
				"and is a precondition on the source instead.",
			capability.DimFrameRate: "No frame rate argument is passed, so the source rate is carried through unchanged.",
		},

		BestEffortModels: map[capability.Dimension]capability.BestEffortModel{
			capability.DimSize: {
				Mechanism: "bitrate_budget",
				Formula: "video_bitrate = max(min_video_bitrate_bps, " +
					"size_bytes * 8 * container_overhead_ratio / duration_seconds - audio_bitrate_bps)",
				Constants: []capability.NamedValue{
					{Name: "audio_bitrate_bps", Value: float64(AudioBitrateBps)},
					{Name: "min_video_bitrate_bps", Value: float64(MinVideoBitrateBps)},
					{Name: "container_overhead_ratio", Value: ContainerOverheadRatio},
				},
				FailsWhen: "The computed bitrate falls to min_video_bitrate_bps — a long source " +
					"with a small size budget — in which case the encode necessarily overshoots. " +
					"A caller can evaluate the formula up front to find out.",
			},
		},

		SourceRequires: capability.SourceRequirements{
			Streams: []string{"video"},

			// CalculateTargetDimensions rounds down to an even number, so a
			// 1px edge becomes 0px and the encode is handed "scale=0:0". Two
			// is the smallest edge that survives, and stating it as a
			// precondition is more honest than weakening the width and height
			// guarantees to cover a case that produces nothing usable anyway.
			MinDimension: 2,
		},
	})
}

// ExtractFrameCapability describes BuildExtractFrameArgs and ExtractFrame:
// the first frame of a video, written as WebP.
//
// Keep this in step with BuildExtractFrameArgs.
func ExtractFrameCapability() (capability.Operation, error) {
	return capability.NewOperation(capability.OperationSpec{
		Name: "extract_frame",

		// "-f webp" is hard-wired. There is no format parameter, so a caller
		// asking for anything else gets WebP regardless.
		Produces: []string{"image/webp"},

		Guarantees: map[capability.Dimension]capability.Guarantee{
			capability.DimContentType: capability.GuaranteeExact,
			capability.DimWidth:       capability.GuaranteeExact,

			// The scale filter is "min(width,iw):-1": the -1 tells ffmpeg to
			// derive the height from the aspect ratio, so nothing constrains
			// it. ExtractFrameParams has no height field at all. A caller that
			// needs a bounded height needs a different filter, not a bigger
			// number.
			capability.DimHeight: capability.GuaranteeNone,

			// Nothing targets a byte size; quality is left at the encoder
			// default.
			capability.DimSize: capability.GuaranteeNone,
		},

		Notes: map[capability.Dimension]string{
			capability.DimWidth: "Narrowed to the requested bound, never enlarged: a source " +
				"already below it comes back at its original width.",
			capability.DimHeight: "Derived from the aspect ratio and unbounded. Requesting a " +
				"maximum height has no effect whatsoever.",
		},

		SourceRequires: capability.SourceRequirements{
			Streams: []string{"video"},
		},
	})
}

// Capabilities returns every operation this package declares, in a stable
// order, so a service can publish them without having to know the list.
func Capabilities() ([]capability.Operation, error) {
	builders := []func() (capability.Operation, error){
		TranscodeCapability,
		ExtractFrameCapability,
	}
	operations := make([]capability.Operation, 0, len(builders))
	for _, build := range builders {
		op, err := build()
		if err != nil {
			return nil, fmt.Errorf("building ffmpeg capabilities: %w", err)
		}
		operations = append(operations, op)
	}
	return operations, nil
}
