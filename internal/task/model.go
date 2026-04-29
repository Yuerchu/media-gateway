package task

import (
	"time"
)

// TaskType specifies the kind of media processing task.
type TaskType string

const (
	TaskTypeThumbnail TaskType = "thumbnail"
	TaskTypeMetadata  TaskType = "metadata"
	TaskTypeTranscode TaskType = "transcode"
)

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
	TaskStatusQueued     TaskStatus = "queued"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

// S3Source describes where to read the source file.
type S3Source struct {
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	Endpoint    string `json:"endpoint"`
	AccessKeyID string `json:"access_key_id"`
	SecretKey   string `json:"secret_access_key"`
	Region      string `json:"region"`
}

// S3Output describes where to write processed output.
type S3Output struct {
	Bucket      string `json:"bucket"`
	KeyPrefix   string `json:"key_prefix"`
	Endpoint    string `json:"endpoint"`
	AccessKeyID string `json:"access_key_id"`
	SecretKey   string `json:"secret_access_key"`
	Region      string `json:"region"`
}

// CallbackConfig describes how to notify the caller when a task completes.
type CallbackConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

// ThumbnailParams holds thumbnail generation parameters.
type ThumbnailParams struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"` // "webp", "jpeg", "png"
}

// MetadataParams holds metadata extraction parameters.
type MetadataParams struct {
	Namespaces []string `json:"namespaces"` // "exif", "stream", "music"
}

// TranscodeParams holds video transcoding parameters.
type TranscodeParams struct {
	MaxWidth   int   `json:"max_width"`
	MaxHeight  int   `json:"max_height"`
	MaxSizeMB  int64 `json:"max_size_mb"`
	VideoCodec string `json:"video_codec"`
	Preset     string `json:"preset"`
}

// TaskRequest is the POST /api/v1/tasks request body.
type TaskRequest struct {
	Type     TaskType        `json:"type" binding:"required"`
	Source   S3Source        `json:"source" binding:"required"`
	Output   *S3Output      `json:"output"`
	Callback *CallbackConfig `json:"callback"`

	// Type-specific params (only one should be set)
	Thumbnail *ThumbnailParams `json:"thumbnail_params"`
	Metadata  *MetadataParams  `json:"metadata_params"`
	Transcode *TranscodeParams `json:"transcode_params"`
}

// TaskResult holds the result of a completed task.
type TaskResult struct {
	MIMEType string            `json:"mime_type,omitempty"`
	Metadata map[string]any    `json:"metadata,omitempty"`
	Thumb    *ThumbnailResult  `json:"thumbnail,omitempty"`
	Output   *TranscodeResult  `json:"transcode,omitempty"`
}

// ThumbnailResult holds thumbnail generation output.
type ThumbnailResult struct {
	Generated bool   `json:"generated"`
	Key       string `json:"key"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Format    string `json:"format"`
	Size      int64  `json:"size"`
}

// TranscodeResult holds transcoding output.
type TranscodeResult struct {
	Key         string `json:"key"`
	MIMEType    string `json:"mime_type"`
	Size        int64  `json:"size"`
	ContentHash string `json:"content_hash"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

// Task represents a media processing task.
type Task struct {
	ID        string      `json:"task_id"`
	Type      TaskType    `json:"type"`
	Status    TaskStatus  `json:"status"`
	Request   TaskRequest `json:"-"`
	Result    *TaskResult `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// TaskResponse is the API response for a task.
type TaskResponse struct {
	TaskID    string      `json:"task_id"`
	Status    TaskStatus  `json:"status"`
	Result    *TaskResult `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}
