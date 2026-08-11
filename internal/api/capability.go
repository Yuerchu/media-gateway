package api

import (
	"fmt"

	"github.com/yuerchu/media-gateway/internal/config"
	"github.com/yuerchu/media-gateway/pkg/capability"
	"github.com/yuerchu/media-gateway/pkg/ffmpeg"
)

// ContractVersion is the semantic version of the published capability
// contract. It tracks the contract, not the service: bump the minor for
// additions and for a guarantee moving to a weaker grade — a consumer refetches
// and only becomes more careful — and the major for a rename, a removal, or a
// guarantee moving to a stronger one.
const ContractVersion = "1.0.0"

// contractServiceID identifies this service in the rendered $id.
const contractServiceID = "media-gateway"

// buildCapabilityDocument assembles the contract for the task API.
//
// The operations here are the task types from POST /api/v1/tasks, and their
// guarantees are those of the library functions the worker actually calls —
// which is not always the one the task name suggests. The thumbnail task runs
// ffmpeg.ExtractFrame, not pkg/thumbnail, and inherits ExtractFrame's promises
// rather than the ones the parameter names imply.
//
// Each operation's request schema covers its own params object rather than the
// whole envelope, so a pointer like /$defs/thumbnail/properties/format reads
// the way one would expect. The shared envelope fields — type, source, output,
// callback — are described once at the document level.
func buildCapabilityDocument(cfg *config.Config) (capability.Document, error) {
	extractFrame, err := ffmpeg.ExtractFrameCapability()
	if err != nil {
		return capability.Document{}, err
	}
	transcode, err := ffmpeg.TranscodeCapability()
	if err != nil {
		return capability.Document{}, err
	}

	thumbnailOp, err := buildThumbnailOperation(extractFrame)
	if err != nil {
		return capability.Document{}, err
	}
	transcodeOp, err := buildTranscodeOperation(transcode)
	if err != nil {
		return capability.Document{}, err
	}
	metadataOp, err := buildMetadataOperation()
	if err != nil {
		return capability.Document{}, err
	}

	return capability.Document{
		Version:   ContractVersion,
		ServiceID: contractServiceID,
		Title:     "media-gateway task capabilities",
		Source: capability.SourceLimits{
			MaxSizeBytes: cfg.FFmpeg.MaxSourceSize,
			// Deliberately no container allowlist: this service accepts
			// whatever ffprobe can read and fails during processing otherwise.
		},
		Operations: []capability.Operation{thumbnailOp, transcodeOp, metadataOp},
		Extensions: map[string]any{
			"x-queue": map[string]any{
				"size":        cfg.Task.QueueSize,
				"concurrency": cfg.Task.MaxConcurrency,
				"full_status": 429,
				"note":        "A full queue is refused with Retry-After rather than blocking the request.",
			},
			"x-envelope": map[string]any{
				"endpoint": "POST /api/v1/tasks",
				"note": "Each operation's schema below describes its own params object. " +
					"The surrounding request carries type, source, output and callback, " +
					"with the operation selected by type.",
				"operation_timeout_seconds": cfg.FFmpeg.ConvertTimeout,
			},
			"x-status-caveat": "Several rejections below report 500 for what is really a bad " +
				"request. The statuses are published as they behave today rather than as they " +
				"ought to be, so that correcting them is a visible contract change.",
		},
	}, nil
}

// buildThumbnailOperation exposes ExtractFrame as the "thumbnail" task.
//
// Three of the parameters this task accepts do not survive the trip, and this
// is the only place that says so.
func buildThumbnailOperation(base capability.Operation) (capability.Operation, error) {
	op := base.WithName("thumbnail").
		WithRequestSchema(&capability.Schema{
			Type:        "object",
			Description: "thumbnail_params. Omitted entirely, the defaults are 400x300 webp.",
			Properties: map[string]*capability.Schema{
				"width": {
					Type:        "integer",
					Minimum:     capability.Float(1),
					Description: "Upper bound on output width. A narrower source is not enlarged.",
				},
				"height": {
					Type:       "integer",
					Minimum:    capability.Float(1),
					Deprecated: true,
					Description: "Accepted and never applied. Output height follows the " +
						"aspect ratio and is unbounded.",
				},
				"format": {
					Type:       "string",
					Enum:       []string{"webp", "jpeg", "png"},
					Deprecated: true,
					Description: "Accepted and never applied. The output is always WebP; " +
						"this value only picks the object key suffix and the stored content type.",
				},
			},
		}).
		WithIgnoredFields([]capability.IgnoredField{
			{
				Name: "height",
				Reason: "ExtractFrameParams has no height field. The scale filter derives " +
					"height from the aspect ratio, so any value here affects nothing.",
			},
			{
				Name: "format",
				Reason: "The encoder is invoked with a hard-wired -f webp. A request for jpeg " +
					"or png still returns WebP bytes, stored under that suffix and labelled " +
					"with that content type — so the stored object is mislabelled. Request webp.",
			},
		}).
		WithResultGuarantees([]capability.ResultGuarantee{
			{
				Pointer:   "/thumbnail/size",
				Guarantee: capability.GuaranteeExact,
				Note:      "Measured from the generated bytes.",
			},
			{
				Pointer:   "/thumbnail/width",
				Guarantee: capability.GuaranteeNone,
				Note: "Echoes the requested width. The real output width is " +
					"min(requested, source width) and is not measured.",
			},
			{
				Pointer:   "/thumbnail/height",
				Guarantee: capability.GuaranteeNone,
				Note:      "Echoes the requested height, which was never applied.",
			},
			{
				Pointer:   "/thumbnail/format",
				Guarantee: capability.GuaranteeNone,
				Note:      "Echoes the requested format. The bytes are always WebP.",
			},
			{
				Pointer:   "/mime_type",
				Guarantee: capability.GuaranteeExact,
				Note:      "Detected from the source, not from the thumbnail.",
			},
		})

	return op.WithRejections([]capability.Rejection{
		{
			Code:       "source_too_large",
			Stage:      capability.RejectStageSource,
			SchemaPath: "/$defs/source/properties/size_bytes/maximum",
			HTTPStatus: 500,
			Summary: "Source exceeds the ingest limit. Reported as 500 today even though " +
				"it is the request's problem.",
		},
		{
			Code:       "queue_full",
			Stage:      capability.RejectStageRequest,
			SchemaPath: "/x-queue",
			HTTPStatus: 429,
			Summary:    "Task queue is at capacity; retry after the Retry-After interval.",
		},
	})
}

// buildTranscodeOperation exposes the transcode pipeline as the "transcode"
// task.
func buildTranscodeOperation(base capability.Operation) (capability.Operation, error) {
	op := base.
		WithRequestSchema(&capability.Schema{
			Type:        "object",
			Description: "transcode_params. Required; the task fails without it.",
			Properties: map[string]*capability.Schema{
				"max_width": {
					Type:        "integer",
					Minimum:     capability.Float(1),
					Description: "Upper bound on output width; defaults to 1920.",
				},
				"max_height": {
					Type:        "integer",
					Minimum:     capability.Float(1),
					Description: "Upper bound on output height; defaults to 1080.",
				},
				"max_size_mb": {
					Type:    "integer",
					Minimum: capability.Float(1),
					Description: "Size budget in MiB, defaulting to 50. Used only to derive a " +
						"bitrate — see the best-effort model on the size dimension. The output " +
						"is not checked against it afterwards.",
				},
				"video_codec": {
					Type:        "string",
					Description: "Passed through to the encoder; defaults to libx264.",
				},
				"preset": {
					Type:        "string",
					Description: "Passed through to the encoder; defaults to fast.",
				},
			},
		}).
		WithResultGuarantees([]capability.ResultGuarantee{
			{
				Pointer:   "/transcode/size",
				Guarantee: capability.GuaranteeExact,
				Note:      "Measured from the encoded bytes.",
			},
			{
				Pointer:   "/transcode/content_hash",
				Guarantee: capability.GuaranteeExact,
				Note:      "SHA-256 of the encoded bytes.",
			},
			{
				Pointer:   "/transcode/width",
				Guarantee: capability.GuaranteeExact,
				Note:      "The dimensions handed to the scale filter, so they describe the output.",
			},
			{
				Pointer:   "/transcode/height",
				Guarantee: capability.GuaranteeExact,
				Note:      "The dimensions handed to the scale filter, so they describe the output.",
			},
			{
				Pointer:   "/transcode/mime_type",
				Guarantee: capability.GuaranteeExact,
				Note: "Always video/mp4, describing the container. A video_codec other than " +
					"libx264 still lands in an MP4 container and is still reported this way.",
			},
		})

	return op.WithRejections([]capability.Rejection{
		{
			Code:       "missing_transcode_params",
			Stage:      capability.RejectStageRequest,
			SchemaPath: "/$defs/transcode",
			HTTPStatus: 500,
			Summary:    "transcode_params was omitted. Surfaces as a failed task, not a rejected request.",
		},
		{
			Code:       "source_too_large",
			Stage:      capability.RejectStageSource,
			SchemaPath: "/$defs/source/properties/size_bytes/maximum",
			HTTPStatus: 500,
			Summary: "Source exceeds the ingest limit. Reported as 500 today even though " +
				"it is the request's problem.",
		},
		{
			Code:       "unprobeable_source",
			Stage:      capability.RejectStageSource,
			SchemaPath: "/$defs/transcode/x-source-requirements",
			HTTPStatus: 500,
			Summary:    "The source could not be probed, so no target geometry could be computed.",
		},
		{
			Code:       "queue_full",
			Stage:      capability.RejectStageRequest,
			SchemaPath: "/x-queue",
			HTTPStatus: 429,
			Summary:    "Task queue is at capacity; retry after the Retry-After interval.",
		},
	})
}

// buildMetadataOperation describes the "metadata" task, which writes no
// artifact at all.
func buildMetadataOperation() (capability.Operation, error) {
	op, err := capability.NewOperation(capability.OperationSpec{
		Name: "metadata",
		// No Produces and no content-type promise: this task uploads nothing.
	})
	if err != nil {
		return capability.Operation{}, err
	}

	op = op.
		WithRequestSchema(&capability.Schema{
			Type:        "object",
			Description: "metadata_params. Entirely optional and entirely ignored.",
			Properties: map[string]*capability.Schema{
				"namespaces": {
					Type:       "array",
					Deprecated: true,
					Description: "Accepted and never read. The result always carries sys, and " +
						"stream whenever probing succeeded, regardless of what is requested here.",
				},
			},
		}).
		WithIgnoredFields([]capability.IgnoredField{
			{
				Name: "namespaces",
				Reason: "The worker never reads it. The exif and music namespaces it advertises " +
					"are not produced by this task at all.",
			},
		}).
		WithResultGuarantees([]capability.ResultGuarantee{
			{
				Pointer:   "/metadata/sys/content_hash",
				Guarantee: capability.GuaranteeExact,
				Note:      "SHA-256 of the source bytes; always present.",
			},
			{
				Pointer:   "/metadata/sys/size",
				Guarantee: capability.GuaranteeExact,
				Note:      "Source size in bytes; always present.",
			},
			{
				Pointer:   "/metadata/stream",
				Guarantee: capability.GuaranteeBestEffort,
				Note: "Present only when ffprobe succeeded. A probe failure is logged and " +
					"tolerated, so a non-media source yields a result with sys and no stream " +
					"rather than a failed task.",
			},
			{
				Pointer:   "/mime_type",
				Guarantee: capability.GuaranteeExact,
				Note:      "Detected from the leading bytes of the source.",
			},
		})

	return op.WithRejections([]capability.Rejection{
		{
			Code:       "source_too_large",
			Stage:      capability.RejectStageSource,
			SchemaPath: "/$defs/source/properties/size_bytes/maximum",
			HTTPStatus: 500,
			Summary: "Source exceeds the ingest limit. Reported as 500 today even though " +
				"it is the request's problem.",
		},
		{
			Code:       "queue_full",
			Stage:      capability.RejectStageRequest,
			SchemaPath: "/x-queue",
			HTTPStatus: 429,
			Summary:    "Task queue is at capacity; retry after the Retry-After interval.",
		},
	})
}

// renderCapabilityDocument builds the contract and encodes it once.
//
// Doing this at startup rather than per request means a malformed contract is
// a boot failure instead of a bad response discovered by whoever asked first.
func renderCapabilityDocument(cfg *config.Config) ([]byte, error) {
	document, err := buildCapabilityDocument(cfg)
	if err != nil {
		return nil, fmt.Errorf("building capability document: %w", err)
	}
	encoded, err := document.RenderJSON()
	if err != nil {
		return nil, fmt.Errorf("rendering capability document: %w", err)
	}
	return encoded, nil
}
