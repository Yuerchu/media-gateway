package callback

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	jsoniter "github.com/json-iterator/go"
)

// Config describes how to notify the caller when a task completes.
type Config struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

// Caller sends HTTP callbacks to notify callers of task completion.
type Caller struct {
	client *http.Client
}

// NewCaller creates a new callback caller.
func NewCaller() *Caller {
	return &Caller{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Send posts the result to the callback URL.
func (c *Caller) Send(url, method string, headers map[string]string, payload any) error {
	if url == "" {
		return nil
	}

	if method == "" {
		method = http.MethodPost
	}

	body, err := jsoniter.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling callback payload: %w", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating callback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending callback to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("callback to %s returned HTTP %d", url, resp.StatusCode)
	}

	slog.Info("Callback sent", "url", url, "status", resp.StatusCode)
	return nil
}
