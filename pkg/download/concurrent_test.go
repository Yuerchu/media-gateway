package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRewriteURL(t *testing.T) {
	rewrites := []URLRewrite{
		{From: "https://cdn.example.com/", To: "http://internal.local:9000/"},
		{From: "https://old.host/", To: "https://new.host/"},
	}
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "matching rewrite",
			uri:  "https://cdn.example.com/files/photo.jpg",
			want: "http://internal.local:9000/files/photo.jpg",
		},
		{
			name: "second rule",
			uri:  "https://old.host/path?q=1",
			want: "https://new.host/path?q=1",
		},
		{
			name: "no matching rule",
			uri:  "https://other.host/file",
			want: "https://other.host/file",
		},
		{
			name: "first match wins",
			uri:  "https://cdn.example.com/test",
			want: "http://internal.local:9000/test",
		},
		{
			name: "empty rewrites",
			uri:  "https://any.url/path",
			want: "https://any.url/path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := rewrites
			if tt.name == "empty rewrites" {
				rw = nil
			}
			got := RewriteURL(tt.uri, rw)
			if got != tt.want {
				t.Errorf("RewriteURL(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}

func TestTruncateURL(t *testing.T) {
	short := "https://example.com/short"
	if got := TruncateURL(short); got != short {
		t.Errorf("short URL should not be truncated")
	}

	long := "https://example.com/" + string(make([]byte, 200))
	got := TruncateURL(long)
	if len(got) != 123 {
		t.Errorf("truncated URL length = %d, want 123 (120 + ...)", len(got))
	}
	if got[len(got)-3:] != "..." {
		t.Error("truncated URL should end with ...")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0.0KB"},
		{1024, "1.0KB"},
		{512, "0.5KB"},
		{1024 * 1024, "1.0MB"},
		{5 * 1024 * 1024, "5.0MB"},
		{1536 * 1024, "1.5MB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestPayloadTooLargeError(t *testing.T) {
	err := &PayloadTooLargeError{
		ActualSize: 10 * 1024 * 1024,
		MaxSize:    5 * 1024 * 1024,
		Detail:     "test file",
	}
	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

func TestDownloadOne_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("hello world"))
	}))
	defer server.Close()

	result := downloadOne(context.Background(), server.Client(), server.URL+"/file.txt", 1024*1024, nil)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if string(result.Data) != "hello world" {
		t.Errorf("data = %q, want %q", result.Data, "hello world")
	}
	if result.URL != server.URL+"/file.txt" {
		t.Error("URL should be preserved as original")
	}
	if result.MIMEType != "text/plain" {
		t.Errorf("MIMEType = %q, want text/plain", result.MIMEType)
	}
}

func TestDownloadOne_TooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("this is more than 5 bytes"))
	}))
	defer server.Close()

	result := downloadOne(context.Background(), server.Client(), server.URL+"/big", 5, nil)
	if result.Err == nil {
		t.Fatal("expected error for oversized file")
	}
	if _, ok := result.Err.(*PayloadTooLargeError); !ok {
		t.Errorf("error type = %T, want *PayloadTooLargeError", result.Err)
	}
}

func TestDownloadOne_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	result := downloadOne(context.Background(), server.Client(), server.URL+"/missing", 1024, nil)
	if result.Err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestDownloadOne_WithRewrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("rewritten"))
	}))
	defer server.Close()

	rewrites := []URLRewrite{{From: "https://cdn.example.com/", To: server.URL + "/"}}
	result := downloadOne(context.Background(), server.Client(), "https://cdn.example.com/file.txt", 1024*1024, rewrites)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.URL != "https://cdn.example.com/file.txt" {
		t.Error("original URL should be preserved")
	}
	if string(result.Data) != "rewritten" {
		t.Errorf("data = %q, want %q", result.Data, "rewritten")
	}
}

func TestFiles_Concurrent(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	urls := []string{
		server.URL + "/a",
		server.URL + "/b",
		server.URL + "/c",
	}
	opts := &Options{MaxConcurrency: 2, MaxFileSize: 1024}
	results := Files(context.Background(), urls, server.Client(), opts, nil)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("result[%d] error: %v", i, r.Err)
		}
	}
}

func TestFiles_WithGlobalSemaphore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	urls := []string{server.URL + "/a", server.URL + "/b"}
	opts := &Options{MaxConcurrency: 5, MaxFileSize: 1024}
	globalSem := make(chan struct{}, 1)
	results := Files(context.Background(), urls, server.Client(), opts, globalSem)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("result[%d] error: %v", i, r.Err)
		}
	}
}

func TestFiles_CancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	urls := []string{server.URL + "/a"}
	opts := &Options{MaxConcurrency: 1, MaxFileSize: 1024}
	results := Files(ctx, urls, server.Client(), opts, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Error("expected error for cancelled context")
	}
}
