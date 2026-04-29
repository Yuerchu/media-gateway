package task

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/yuerchu/media-gateway/internal/callback"
	"github.com/yuerchu/media-gateway/internal/config"
	"github.com/yuerchu/media-gateway/pkg/ffmpeg"
	mgmime "github.com/yuerchu/media-gateway/pkg/mime"
	"github.com/yuerchu/media-gateway/pkg/model"
	"github.com/yuerchu/media-gateway/pkg/s3util"
	"github.com/yuerchu/media-gateway/pkg/signing"
)

// Worker processes media tasks from a channel.
type Worker struct {
	manager  *Manager
	cfg      *config.Config
	caller   *callback.Caller
	taskChan chan string
	sem      chan struct{}
}

// NewWorker creates a worker pool.
func NewWorker(manager *Manager, cfg *config.Config, caller *callback.Caller) *Worker {
	w := &Worker{
		manager:  manager,
		cfg:      cfg,
		caller:   caller,
		taskChan: make(chan string, cfg.Task.QueueSize),
		sem:      make(chan struct{}, cfg.Task.MaxConcurrency),
	}
	return w
}

// Submit adds a task ID to the processing queue.
// Returns false if the queue is full.
func (w *Worker) Submit(taskID string) bool {
	select {
	case w.taskChan <- taskID:
		return true
	default:
		return false
	}
}

// Run starts consuming tasks. Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	slog.Info("Worker pool started", "concurrency", w.cfg.Task.MaxConcurrency, "queue_size", w.cfg.Task.QueueSize)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Worker pool shutting down, draining queue")
			w.drain()
			return
		case taskID := <-w.taskChan:
			w.sem <- struct{}{}
			go func() {
				defer func() { <-w.sem }()
				w.process(ctx, taskID)
			}()
		}
	}
}

// drain processes remaining tasks in the queue before shutdown.
func (w *Worker) drain() {
	for {
		select {
		case taskID := <-w.taskChan:
			w.process(context.Background(), taskID)
		default:
			return
		}
	}
}

func (w *Worker) process(ctx context.Context, taskID string) {
	t, err := w.manager.Get(taskID)
	if err != nil {
		slog.Error("Task not found", "task_id", taskID, "error", err)
		return
	}

	w.manager.SetProcessing(taskID)
	slog.Info("Processing task", "task_id", taskID, "type", t.Type)

	var result *TaskResult
	var processErr error

	switch t.Type {
	case TaskTypeThumbnail:
		result, processErr = w.processThumbnail(ctx, t)
	case TaskTypeMetadata:
		result, processErr = w.processMetadata(ctx, t)
	case TaskTypeTranscode:
		result, processErr = w.processTranscode(ctx, t)
	default:
		processErr = fmt.Errorf("unknown task type: %s", t.Type)
	}

	if processErr != nil {
		slog.Error("Task failed", "task_id", taskID, "error", processErr)
		w.manager.SetFailed(taskID, processErr.Error())
	} else {
		slog.Info("Task completed", "task_id", taskID)
		w.manager.SetCompleted(taskID, result)
	}

	// Send callback if configured
	if t.Request.Callback != nil {
		w.sendCallback(taskID, t)
	}
}

func (w *Worker) sendCallback(taskID string, t *Task) {
	// Re-read to get updated status
	updated, err := w.manager.Get(taskID)
	if err != nil {
		return
	}

	resp := TaskResponse{
		TaskID:    updated.ID,
		Status:    updated.Status,
		Result:    updated.Result,
		Error:     updated.Error,
		CreatedAt: updated.CreatedAt,
	}

	cb := t.Request.Callback
	if err := w.caller.Send(cb.URL, cb.Method, cb.Headers, resp); err != nil {
		slog.Error("Callback failed", "task_id", taskID, "url", cb.URL, "error", err)
	}
}

func (w *Worker) makeS3Client(src S3Source) *s3.Client {
	return s3.New(s3.Options{
		BaseEndpoint: aws.String(src.Endpoint),
		Region:       src.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(src.AccessKeyID, src.SecretKey, ""),
		UsePathStyle: true,
	})
}

func (w *Worker) processThumbnail(ctx context.Context, t *Task) (*TaskResult, error) {
	params := t.Request.Thumbnail
	if params == nil {
		params = &ThumbnailParams{Width: 400, Height: 300, Format: "webp"}
	}
	if params.Format == "" {
		params.Format = "webp"
	}

	// Create temp dir
	tmpDir := filepath.Join(w.cfg.FFmpeg.TempDir, t.ID)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download source
	srcPath := filepath.Join(tmpDir, "source")
	client := w.makeS3Client(t.Request.Source)
	if err := s3util.DownloadToFile(ctx, client, t.Request.Source.Bucket, t.Request.Source.Key, srcPath, w.cfg.FFmpeg.MaxSourceSize); err != nil {
		return nil, fmt.Errorf("downloading source: %w", err)
	}

	// Detect MIME type
	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("reading source: %w", err)
	}
	mimeType := mgmime.DetectFromBytes(srcData[:min(len(srcData), 8192)])

	// Generate thumbnail
	outPath := filepath.Join(tmpDir, "thumb."+params.Format)
	timeout := time.Duration(w.cfg.FFmpeg.ConvertTimeout) * time.Second
	convertCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := ffmpeg.ExtractFrame(convertCtx, w.cfg.FFmpeg.FFmpegPath, &model.ExtractFrameParams{
		InputPath:  srcPath,
		OutputPath: outPath,
		MaxWidth:   params.Width,
	}); err != nil {
		return nil, fmt.Errorf("extracting frame: %w", err)
	}

	// Read output and upload
	thumbData, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("reading thumbnail: %w", err)
	}

	// Upload thumbnail if output config is provided
	var thumbKey string
	if t.Request.Output != nil {
		thumbKey = t.Request.Output.KeyPrefix + t.ID + "." + params.Format
		uploader := s3util.NewUploader(&s3util.UploaderConfig{
			Endpoint:       t.Request.Output.Endpoint,
			AccessKey:      t.Request.Output.AccessKeyID,
			SecretKey:      t.Request.Output.SecretKey,
			Bucket:         t.Request.Output.Bucket,
			Region:         t.Request.Output.Region,
			ForcePathStyle: true,
		})
		_, uploadErr := uploader.UploadWithKey(ctx, thumbData, "image/"+params.Format, thumbKey)
		if uploadErr != nil {
			return nil, fmt.Errorf("uploading thumbnail: %w", uploadErr)
		}
	}

	return &TaskResult{
		MIMEType: mimeType,
		Thumb: &ThumbnailResult{
			Generated: true,
			Key:       thumbKey,
			Width:     params.Width,
			Height:    params.Height,
			Format:    params.Format,
			Size:      int64(len(thumbData)),
		},
	}, nil
}

func (w *Worker) processMetadata(ctx context.Context, t *Task) (*TaskResult, error) {
	// Create temp dir
	tmpDir := filepath.Join(w.cfg.FFmpeg.TempDir, t.ID)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download source
	srcPath := filepath.Join(tmpDir, "source")
	client := w.makeS3Client(t.Request.Source)
	if err := s3util.DownloadToFile(ctx, client, t.Request.Source.Bucket, t.Request.Source.Key, srcPath, w.cfg.FFmpeg.MaxSourceSize); err != nil {
		return nil, fmt.Errorf("downloading source: %w", err)
	}

	// Detect MIME type
	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("reading source: %w", err)
	}
	mimeType := mgmime.DetectFromBytes(srcData[:min(len(srcData), 8192)])

	metadata := make(map[string]any)

	// Extract stream metadata via ffprobe
	timeout := time.Duration(w.cfg.FFmpeg.ConvertTimeout) * time.Second
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	probeResult, err := ffmpeg.Probe(probeCtx, w.cfg.FFmpeg.FFprobePath, srcPath)
	if err != nil {
		slog.Warn("ffprobe failed, may not be a media file", "error", err)
	} else {
		stream := map[string]any{
			"width":       probeResult.Width,
			"height":      probeResult.Height,
			"duration":    probeResult.DurationSec,
			"video_codec": probeResult.VideoCodec,
			"audio_codec": probeResult.AudioCodec,
			"format":      probeResult.FormatName,
			"bitrate":     probeResult.Bitrate,
		}
		metadata["stream"] = stream
	}

	// Content hash
	metadata["sys"] = map[string]any{
		"content_hash": signing.SHA256Hex(srcData),
		"size":         len(srcData),
	}

	return &TaskResult{
		MIMEType: mimeType,
		Metadata: metadata,
	}, nil
}

func (w *Worker) processTranscode(ctx context.Context, t *Task) (*TaskResult, error) {
	params := t.Request.Transcode
	if params == nil {
		return nil, fmt.Errorf("transcode_params is required")
	}
	if params.VideoCodec == "" {
		params.VideoCodec = "libx264"
	}
	if params.Preset == "" {
		params.Preset = "fast"
	}
	if params.MaxWidth == 0 {
		params.MaxWidth = 1920
	}
	if params.MaxHeight == 0 {
		params.MaxHeight = 1080
	}

	// Create temp dir
	tmpDir := filepath.Join(w.cfg.FFmpeg.TempDir, t.ID)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download source
	srcPath := filepath.Join(tmpDir, "source")
	client := w.makeS3Client(t.Request.Source)
	if err := s3util.DownloadToFile(ctx, client, t.Request.Source.Bucket, t.Request.Source.Key, srcPath, w.cfg.FFmpeg.MaxSourceSize); err != nil {
		return nil, fmt.Errorf("downloading source: %w", err)
	}

	// Probe source
	timeout := time.Duration(w.cfg.FFmpeg.ConvertTimeout) * time.Second
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	probeResult, err := ffmpeg.Probe(probeCtx, w.cfg.FFmpeg.FFprobePath, srcPath)
	if err != nil {
		return nil, fmt.Errorf("probing source: %w", err)
	}

	// Calculate target dimensions and bitrate
	targetW, targetH := ffmpeg.CalculateTargetDimensions(probeResult.Width, probeResult.Height, params.MaxWidth, params.MaxHeight)
	maxSizeBytes := params.MaxSizeMB * 1024 * 1024
	if maxSizeBytes == 0 {
		maxSizeBytes = 50 * 1024 * 1024 // 50MB default
	}
	targetBitrate := ffmpeg.CalculateTargetBitrate(maxSizeBytes, probeResult.DurationSec)

	// Transcode
	outPath := filepath.Join(tmpDir, "output.mp4")
	convertCtx, convertCancel := context.WithTimeout(ctx, timeout)
	defer convertCancel()

	if err := ffmpeg.Transcode(convertCtx, w.cfg.FFmpeg.FFmpegPath, &model.TranscodeParams{
		InputPath:     srcPath,
		OutputPath:    outPath,
		TargetWidth:   targetW,
		TargetHeight:  targetH,
		TargetBitrate: targetBitrate,
		VideoCodec:    params.VideoCodec,
		AudioCodec:    "aac",
		Preset:        params.Preset,
	}); err != nil {
		return nil, fmt.Errorf("transcoding: %w", err)
	}

	// Read output
	outData, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("reading output: %w", err)
	}

	// Upload if output config provided
	var outKey string
	if t.Request.Output != nil {
		outKey = t.Request.Output.KeyPrefix + t.ID + ".mp4"
		uploader := s3util.NewUploader(&s3util.UploaderConfig{
			Endpoint:       t.Request.Output.Endpoint,
			AccessKey:      t.Request.Output.AccessKeyID,
			SecretKey:      t.Request.Output.SecretKey,
			Bucket:         t.Request.Output.Bucket,
			Region:         t.Request.Output.Region,
			ForcePathStyle: true,
		})
		_, uploadErr := uploader.UploadWithKey(ctx, outData, "video/mp4", outKey)
		if uploadErr != nil {
			return nil, fmt.Errorf("uploading output: %w", uploadErr)
		}
	}

	return &TaskResult{
		Output: &TranscodeResult{
			Key:         outKey,
			MIMEType:    "video/mp4",
			Size:        int64(len(outData)),
			ContentHash: signing.SHA256Hex(outData),
			Width:       targetW,
			Height:      targetH,
		},
	}, nil
}
