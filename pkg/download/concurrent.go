package download

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"golang.org/x/sync/errgroup"

	mgmime "github.com/yuerchu/media-gateway/pkg/mime"
)

// Result holds the result of a single file download.
type Result struct {
	URL      string
	Data     []byte
	MIMEType string
	Err      error
}

// URLRewrite maps a URL prefix to a replacement prefix.
type URLRewrite struct {
	From string
	To   string
}

// Options configures concurrent download behavior.
type Options struct {
	// MaxConcurrency is the per-batch concurrency limit.
	MaxConcurrency int
	// MaxFileSize is the maximum allowed file size in bytes.
	MaxFileSize int64
	// URLRewrites maps URL prefixes for internal network access.
	URLRewrites []URLRewrite
}

// PayloadTooLargeError is returned when a file exceeds the size limit.
type PayloadTooLargeError struct {
	ActualSize int64
	MaxSize    int64
	Detail     string
}

func (e *PayloadTooLargeError) Error() string {
	return fmt.Sprintf("payload too large: %s (%s, limit %s)",
		e.Detail, formatBytes(e.ActualSize), formatBytes(e.MaxSize),
	)
}

func formatBytes(b int64) string {
	const mb = 1024 * 1024
	if b >= mb {
		return fmt.Sprintf("%.1fMB", float64(b)/float64(mb))
	}
	return fmt.Sprintf("%.1fKB", float64(b)/1024)
}

// Files concurrently downloads multiple URLs with a concurrency limit.
// client is the shared HTTP client (with connection pooling).
// globalSem is the global download goroutine limiter (nil = no global limit).
func Files(ctx context.Context, urls []string, client *http.Client, opts *Options, globalSem chan struct{}) []Result {
	results := make([]Result, len(urls))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.MaxConcurrency)

	for i, rawURL := range urls {
		g.Go(func() error {
			if globalSem != nil {
				select {
				case globalSem <- struct{}{}:
					defer func() { <-globalSem }()
				case <-ctx.Done():
					results[i] = Result{URL: rawURL, Err: ctx.Err()}
					return nil
				}
			}

			results[i] = downloadOne(ctx, client, rawURL, opts.MaxFileSize, opts.URLRewrites)
			return nil
		})
	}

	_ = g.Wait()
	return results
}

func downloadOne(ctx context.Context, client *http.Client, originalURL string, maxSize int64, rewrites []URLRewrite) Result {
	downloadURL := RewriteURL(originalURL, rewrites)

	if downloadURL != originalURL {
		slog.Debug("URL rewritten for download", "original", TruncateURL(originalURL), "rewritten", TruncateURL(downloadURL))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, http.NoBody)
	if err != nil {
		return Result{URL: originalURL, Err: fmt.Errorf("creating request: %w", err)}
	}

	resp, err := client.Do(req)
	if err != nil {
		return Result{URL: originalURL, Err: fmt.Errorf("downloading: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Result{URL: originalURL, Err: fmt.Errorf("HTTP %d from %s", resp.StatusCode, TruncateURL(downloadURL))}
	}

	limitedReader := io.LimitReader(resp.Body, maxSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return Result{URL: originalURL, Err: fmt.Errorf("reading body: %w", err)}
	}
	if int64(len(data)) > maxSize {
		return Result{URL: originalURL, Err: &PayloadTooLargeError{
			ActualSize: int64(len(data)),
			MaxSize:    maxSize,
			Detail:     fmt.Sprintf("file %s exceeds per-file limit", TruncateURL(originalURL)),
		}}
	}

	contentType := resp.Header.Get("Content-Type")
	mimeType := mgmime.DetectMIMEType(data, contentType)

	slog.Info("Downloaded file", "url", TruncateURL(originalURL), "size", len(data), "mime", mimeType)

	return Result{
		URL:      originalURL,
		Data:     data,
		MIMEType: mimeType,
	}
}

// RewriteURL applies URL rewrite rules, replacing scheme+host while preserving
// path and query parameters.
func RewriteURL(uri string, rewrites []URLRewrite) string {
	for _, rw := range rewrites {
		if strings.HasPrefix(uri, rw.From) {
			return rw.To + uri[len(rw.From):]
		}
	}
	return uri
}

// TruncateURL shortens a URL for logging purposes.
func TruncateURL(u string) string {
	if len(u) > 120 {
		return u[:120] + "..."
	}
	return u
}
