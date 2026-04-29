package s3util

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	"github.com/yuerchu/media-gateway/pkg/imagemeta"
	"github.com/yuerchu/media-gateway/pkg/signing"
)

// Uploader uploads data to S3-compatible storage.
type Uploader struct {
	client *s3.Client
	bucket string
}

// UploadResult holds the result of an S3 upload.
type UploadResult struct {
	Key         string `json:"key"`
	MimeType    string `json:"mimeType"`
	Size        uint64 `json:"size"`
	ContentHash string `json:"contentHash"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

// UploaderConfig holds S3 uploader configuration.
type UploaderConfig struct {
	Endpoint       string
	AccessKey      string
	SecretKey      string
	Bucket         string
	Region         string
	ForcePathStyle bool
}

// NewUploader creates an S3 uploader from config.
func NewUploader(cfg *UploaderConfig) *Uploader {
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.Endpoint),
		Region:       cfg.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		UsePathStyle: cfg.ForcePathStyle,
	})

	slog.Info("S3 uploader initialized",
		"endpoint", cfg.Endpoint,
		"bucket", cfg.Bucket,
		"region", cfg.Region,
	)

	return &Uploader{
		client: client,
		bucket: cfg.Bucket,
	}
}

// Upload uploads data to S3 with a UUID key and returns the result with metadata.
func (u *Uploader) Upload(ctx context.Context, data []byte, mimeType string) (*UploadResult, error) {
	key := uuid.New().String()
	return u.UploadWithKey(ctx, data, mimeType, key)
}

// UploadWithKey uploads data to S3 with a specified key.
func (u *Uploader) UploadWithKey(ctx context.Context, data []byte, mimeType, key string) (*UploadResult, error) {
	width, height := imagemeta.ExtractDimensions(data)
	contentHash := signing.SHA256Hex(data)

	_, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(mimeType),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 upload failed (key=%s): %w", key, err)
	}

	slog.Info("Uploaded to S3",
		"key", key,
		"mime", mimeType,
		"size", len(data),
		"dimensions", fmt.Sprintf("%dx%d", width, height),
	)

	return &UploadResult{
		Key:         key,
		MimeType:    mimeType,
		Size:        uint64(len(data)),
		ContentHash: contentHash,
		Width:       width,
		Height:      height,
	}, nil
}

// Client returns the underlying S3 client for direct operations.
func (u *Uploader) Client() *s3.Client {
	return u.client
}

// Bucket returns the configured bucket name.
func (u *Uploader) Bucket() string {
	return u.bucket
}
