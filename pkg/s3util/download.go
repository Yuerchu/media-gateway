package s3util

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// DownloadToFile downloads an S3 object to destPath on disk.
func DownloadToFile(ctx context.Context, client *s3.Client, bucket, key, destPath string, maxSize int64) error {
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		slog.Error("S3 download failed", "bucket", bucket, "key", key, "error", err)
		return fmt.Errorf("s3 get object bucket=%s key=%s: %w", bucket, key, err)
	}
	defer out.Body.Close()

	if out.ContentLength != nil && *out.ContentLength > maxSize {
		return fmt.Errorf("source file too large: %d bytes exceeds limit %d", *out.ContentLength, maxSize)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}

	limitedReader := io.LimitReader(out.Body, maxSize+1)
	size, copyErr := io.Copy(file, limitedReader)
	file.Close()

	if copyErr != nil {
		os.Remove(destPath)
		return fmt.Errorf("writing source to disk: %w", copyErr)
	}

	if size > maxSize {
		os.Remove(destPath)
		return fmt.Errorf("source file too large: %d bytes exceeds limit %d", size, maxSize)
	}

	return nil
}
