package s3util

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ErrSourceTooLarge is returned when the object exceeds maxSize.
//
// It is a sentinel because the distinction matters to callers: an oversized
// source is the requester's problem and should be reported back as such, while
// every other failure here is ours. Without something to match on, a caller
// has only the error string, and the two get reported identically.
var ErrSourceTooLarge = errors.New("source file too large")

// DownloadToFile downloads an S3 object to destPath on disk.
//
// The size limit is enforced twice: once against the advertised ContentLength
// and once against what was actually written, because the first is only a
// claim and a source that understates it would otherwise fill the disk.
// Both paths return an error wrapping ErrSourceTooLarge.
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
		return fmt.Errorf("%w: %d bytes exceeds limit %d", ErrSourceTooLarge, *out.ContentLength, maxSize)
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
		return fmt.Errorf("%w: %d bytes exceeds limit %d", ErrSourceTooLarge, size, maxSize)
	}

	return nil
}
