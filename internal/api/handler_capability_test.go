package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/yuerchu/media-gateway/internal/config"
)

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.FFmpeg.MaxSourceSize = 524288000
	cfg.FFmpeg.ConvertTimeout = 300
	cfg.Task.QueueSize = 100
	cfg.Task.MaxConcurrency = 4
	return cfg
}

func setupCapabilityRouter(t *testing.T, cfg *config.Config) *gin.Engine {
	t.Helper()
	h, err := NewCapabilityHandler(cfg)
	if err != nil {
		t.Fatalf("NewCapabilityHandler() = %v, want no error", err)
	}
	r := gin.New()
	r.GET("/capabilities", h.Get)
	return r
}

func fetchCapabilities(t *testing.T, cfg *config.Config) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	setupCapabilityRouter(t, cfg).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/capabilities", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /capabilities = %d, want %d", w.Code, http.StatusOK)
	}
	var decoded map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding response: %v\n%s", err, w.Body.String())
	}
	return decoded
}

func TestNewCapabilityHandler_ContractIsWellFormed(t *testing.T) {
	// The contract is rendered at startup, and rendering verifies that every
	// rejection's schema_path resolves. So this is the boot check, run where a
	// failure blocks the merge rather than the deploy.
	if _, err := NewCapabilityHandler(testConfig()); err != nil {
		t.Fatalf("NewCapabilityHandler() = %v, want no error", err)
	}
}

func TestCapabilities_ServesASchemaDocument(t *testing.T) {
	w := httptest.NewRecorder()
	setupCapabilityRouter(t, testConfig()).
		ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/capabilities", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); got != "application/schema+json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want the schema media type", got)
	}
	if w.Header().Get("ETag") == "" {
		t.Error("no ETag header; consumers polling for changes have nothing to compare")
	}
}

func TestCapabilities_ETagMatchSkipsTheBody(t *testing.T) {
	router := setupCapabilityRouter(t, testConfig())

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/capabilities", nil))
	etag := first.Header().Get("ETag")

	second := httptest.NewRecorder()
	conditional := httptest.NewRequest(http.MethodGet, "/capabilities", nil)
	conditional.Header.Set("If-None-Match", etag)
	router.ServeHTTP(second, conditional)

	if second.Code != http.StatusNotModified {
		t.Errorf("conditional GET = %d, want %d", second.Code, http.StatusNotModified)
	}
	if second.Body.Len() != 0 {
		t.Errorf("304 carried a %d byte body, want none", second.Body.Len())
	}
}

func TestCapabilities_ReflectRuntimeConfiguration(t *testing.T) {
	// The ingest limit is an environment variable, which is the whole reason
	// the contract is rendered at runtime rather than checked in as a file. If
	// this stops tracking the config, the published document becomes a claim
	// about some other deployment.
	small := testConfig()
	small.FFmpeg.MaxSourceSize = 1048576

	doc := fetchCapabilities(t, small)
	got := navigate(t, doc, "/$defs/source/properties/size_bytes/maximum")
	if got != float64(1048576) {
		t.Errorf("published source size limit = %v, want the configured 1048576", got)
	}

	queue := navigate(t, doc, "/x-queue/size")
	if queue != float64(small.Task.QueueSize) {
		t.Errorf("published queue size = %v, want %d", queue, small.Task.QueueSize)
	}
}

func TestCapabilities_DeclaresEveryTaskType(t *testing.T) {
	doc := fetchCapabilities(t, testConfig())
	listed, ok := navigate(t, doc, "/x-operations").([]any)
	if !ok {
		t.Fatalf("x-operations = %v, want a list", navigate(t, doc, "/x-operations"))
	}
	present := map[string]bool{}
	for _, name := range listed {
		present[name.(string)] = true
	}
	// The task type enum accepted by POST /api/v1/tasks. A type the contract
	// does not describe is one a consumer has to reverse-engineer.
	for _, want := range []string{"thumbnail", "transcode", "metadata"} {
		if !present[want] {
			t.Errorf("x-operations does not list the %q task type, got %v", want, listed)
		}
	}
}

func TestCapabilities_ThumbnailAdmitsWhatItIgnores(t *testing.T) {
	// The thumbnail task accepts a height and a format and honours neither: the
	// scale filter derives height from the aspect ratio, and the encoder is
	// invoked with a hard-wired -f webp. Saying so is the single most useful
	// thing this document does, so it gets a test of its own.
	doc := fetchCapabilities(t, testConfig())

	ignored, ok := navigate(t, doc, "/$defs/thumbnail/x-ignored-properties").([]any)
	if !ok {
		t.Fatalf("thumbnail x-ignored-properties = %v, want a list",
			navigate(t, doc, "/$defs/thumbnail/x-ignored-properties"))
	}
	named := map[string]bool{}
	for _, entry := range ignored {
		named[entry.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"height", "format"} {
		if !named[want] {
			t.Errorf("thumbnail does not declare %q as ignored, got %v", want, ignored)
		}
	}

	if got := navigate(t, doc, "/$defs/thumbnail/properties/height/deprecated"); got != true {
		t.Errorf("thumbnail height deprecated = %v, want true", got)
	}
	if got := navigate(t, doc, "/$defs/thumbnail/properties/format/deprecated"); got != true {
		t.Errorf("thumbnail format deprecated = %v, want true", got)
	}
}

func TestCapabilities_SourceGateIsUnrestricted(t *testing.T) {
	// This service keeps no container allowlist. Publishing an empty enum would
	// say the opposite — an empty enum accepts nothing — so the property is
	// omitted and the annotation says so instead.
	doc := fetchCapabilities(t, testConfig())

	properties := navigate(t, doc, "/$defs/source/properties").(map[string]any)
	if _, present := properties["container_format"]; present {
		t.Error("source declares a container_format enum, but this service has no allowlist")
	}
	if got := navigate(t, doc, "/$defs/source/x-authoritative-gate/property"); got != nil {
		t.Errorf("authoritative gate = %v, want null for an unrestricted service", got)
	}
}

// navigate resolves a JSON Pointer (RFC 6901) and fails the test if any segment
// is missing.
func navigate(t *testing.T, root any, pointer string) any {
	t.Helper()
	current := root
	for _, segment := range splitPointer(t, pointer) {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("json pointer %q: segment %q applied to a %T", pointer, segment, current)
		}
		value, ok := object[segment]
		if !ok {
			t.Fatalf("json pointer %q: segment %q not found", pointer, segment)
		}
		current = value
	}
	return current
}

func splitPointer(t *testing.T, pointer string) []string {
	t.Helper()
	if pointer == "" {
		return nil
	}
	if pointer[0] != '/' {
		t.Fatalf("json pointer %q must start with %q", pointer, "/")
	}
	var segments []string
	for _, raw := range strings.Split(pointer[1:], "/") {
		segments = append(segments, strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~"))
	}
	return segments
}
