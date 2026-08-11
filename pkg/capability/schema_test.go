package capability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func testDocument(t *testing.T) Document {
	t.Helper()
	spec := multiProduceSpec()
	spec.IgnoredFields = []IgnoredField{{Name: "fit_mode", Reason: "accepted, never read"}}
	spec.Rejections = []Rejection{
		{
			Code:       "source_too_large",
			Stage:      RejectStageSource,
			SchemaPath: "/$defs/source/properties/size_bytes/maximum",
			HTTPStatus: 422,
			Summary:    "source exceeds the ingest limit",
		},
		{
			Code:       "unsupported_target_mime",
			Stage:      RejectStageRequest,
			SchemaPath: "/$defs/transcode/properties/target_mime/enum",
			HTTPStatus: 400,
			Summary:    "requested output format is not produced",
		},
	}
	op := mustOperation(t, spec).WithRequestSchema(&Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"target_mime": {Type: "string", Enum: []string{"video/mp4"}},
			"max_width":   {Type: "integer", Minimum: Float(1)},
		},
		Required:              []string{"target_mime", "max_width"},
		UnevaluatedProperties: Bool(false),
	})

	return Document{
		Version:   "1.0.0",
		ServiceID: "media.example",
		Title:     "Test contract",
		Source: SourceLimits{
			MaxSizeBytes:     524288000,
			ContainerFormats: []string{"matroska,webm", "mov,mp4,m4a,3gp,3g2,mj2"},
			ContentTypes:     []string{"video/mp4", "video/webm"},
		},
		Operations: []Operation{op},
	}
}

func TestRender_RequiresIdentity(t *testing.T) {
	tests := []struct {
		name string
		bend func(*Document)
	}{
		{name: "no version", bend: func(d *Document) { d.Version = "" }},
		{name: "no service id", bend: func(d *Document) { d.ServiceID = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := testDocument(t)
			tt.bend(&doc)
			if _, err := doc.Render(); err == nil {
				t.Error("Render() = nil error, want a rejection")
			}
		})
	}
}

func TestRender_RejectsDuplicateOperation(t *testing.T) {
	doc := testDocument(t)
	doc.Operations = append(doc.Operations, doc.Operations[0])
	if _, err := doc.Render(); err == nil {
		t.Error("Render() = nil error, want a rejection for the duplicate operation")
	}
}

func TestRenderJSON_Deterministic(t *testing.T) {
	// Consumers cache and ETag these bytes. Rendering through anything with
	// unordered output — an encoder that does not sort map keys, a slice built
	// by ranging a map — would make both meaningless, and the symptom would be
	// a cache that thrashes rather than an obvious failure.
	doc := testDocument(t)
	first, err := doc.RenderJSON()
	if err != nil {
		t.Fatalf("RenderJSON() = %v, want no error", err)
	}
	for i := 0; i < 8; i++ {
		again, err := doc.RenderJSON()
		if err != nil {
			t.Fatalf("RenderJSON() = %v, want no error", err)
		}
		if !bytes.Equal(first, again) {
			t.Fatalf("RenderJSON() is not deterministic:\nfirst: %s\nagain: %s", first, again)
		}
	}
}

func TestRenderJSON_DeclaresDialect(t *testing.T) {
	doc := decodeDocument(t, testDocument(t))
	if got := doc["$schema"]; got != Dialect {
		t.Errorf("$schema = %v, want %q", got, Dialect)
	}
	if got, ok := doc["$id"].(string); !ok || !strings.Contains(got, "1.0.0") {
		t.Errorf("$id = %v, want it to carry the contract version", doc["$id"])
	}
}

func TestRenderJSON_OperationsKeyedByName(t *testing.T) {
	// Operations live under $defs keyed by name rather than in an array so that
	// a rejection's schema_path is a stable pointer. Positional pointers would
	// retarget the moment an operation is added.
	doc := decodeDocument(t, testDocument(t))
	if got := navigate(t, doc, "/$defs/transcode/title"); got != "transcode" {
		t.Errorf("$defs/transcode/title = %v, want %q", got, "transcode")
	}

	// x-operations lets a consumer enumerate them without having to know that
	// "source" is not one.
	listed, ok := navigate(t, doc, "/x-operations").([]any)
	if !ok || len(listed) != 1 || listed[0] != "transcode" {
		t.Errorf("x-operations = %v, want [transcode]", listed)
	}
}

func TestRender_RejectsOperationNamedSource(t *testing.T) {
	// Operations sit alongside the source definition in $defs, so that name is
	// taken. Letting it through would silently overwrite the preconditions.
	spec := multiProduceSpec()
	spec.Name = sourceDefName
	doc := testDocument(t)
	doc.Operations = []Operation{mustOperation(t, spec)}
	if _, err := doc.Render(); err == nil {
		t.Error("Render() = nil error, want a rejection for the reserved operation name")
	}
}

func TestRenderJSON_SchemaPathsResolve(t *testing.T) {
	// Without this the schema_path field is decoration: it reads like a
	// pointer, nothing checks that it points anywhere, and it rots the first
	// time the document layout moves.
	source := testDocument(t)
	doc := decodeDocument(t, source)
	for _, op := range source.Operations {
		for _, rejection := range op.Rejections() {
			t.Run(rejection.Code, func(t *testing.T) {
				if got := navigate(t, doc, rejection.SchemaPath); got == nil {
					t.Errorf("schema_path %q resolves to null", rejection.SchemaPath)
				}
			})
		}
	}
}

func TestRenderJSON_SourceGateIsAnnotated(t *testing.T) {
	// The MIME list is a pre-filter and the container format is the gate.
	// Publishing the MIME list without saying so would invite a consumer to
	// treat passing it as acceptance.
	doc := decodeDocument(t, testDocument(t))
	gate := navigate(t, doc, "/$defs/source/x-authoritative-gate")
	if got := gate.(map[string]any)["property"]; got != "container_format" {
		t.Errorf("authoritative gate = %v, want %q", got, "container_format")
	}
	advisory := navigate(t, doc, "/$defs/source/properties/content_type/x-advisory")
	if advisory != true {
		t.Errorf("content_type x-advisory = %v, want true", advisory)
	}
}

func TestRenderJSON_GuaranteesCarrySemantics(t *testing.T) {
	// A bare grade name is guessable in three different ways. Shipping the
	// definitions next to the values costs nothing and removes the guessing.
	doc := decodeDocument(t, testDocument(t))
	semantics := navigate(t, doc, "/$defs/transcode/x-guarantee-semantics")
	for _, g := range AllGuarantees() {
		if _, ok := semantics.(map[string]any)[string(g)]; !ok {
			t.Errorf("x-guarantee-semantics has no entry for %q", g)
		}
	}
}

func TestSchemaMarshalJSON_InlinesExtensions(t *testing.T) {
	raw, err := json.Marshal(Schema{
		Type:       "string",
		Extensions: map[string]any{"x-advisory": true},
	})
	if err != nil {
		t.Fatalf("Marshal() = %v, want no error", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal(%s) = %v", raw, err)
	}
	if decoded["type"] != "string" {
		t.Errorf("type = %v, want %q", decoded["type"], "string")
	}
	if decoded["x-advisory"] != true {
		t.Errorf("x-advisory = %v, want true", decoded["x-advisory"])
	}
}

func TestSchemaMarshalJSON_RejectsBadExtensions(t *testing.T) {
	tests := []struct {
		name   string
		schema Schema
	}{
		{
			name: "unprefixed name",
			// Indistinguishable from a misspelled keyword; a validator would
			// ignore both, so refuse it at the source.
			schema: Schema{Extensions: map[string]any{"advisory": true}},
		},
		{
			name:   "collides with a keyword",
			schema: Schema{Type: "string", Extensions: map[string]any{"type": "integer"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := json.Marshal(tt.schema); err == nil {
				t.Error("Marshal() = nil error, want a rejection")
			}
		})
	}
}

func TestRenderJSON_NumericBoundsAreNotExponential(t *testing.T) {
	// MaxSizeBytes travels as a float64 bound. Encoded in exponent form it is
	// still valid JSON but stops being greppable and reads as a rounding.
	raw, err := testDocument(t).RenderJSON()
	if err != nil {
		t.Fatalf("RenderJSON() = %v, want no error", err)
	}
	if !bytes.Contains(raw, []byte("524288000")) {
		t.Errorf("rendered document does not contain the plain source size bound:\n%s", raw)
	}
}

// decodeDocument renders doc and decodes it into generic maps, which is how a
// consumer in another language sees it.
func decodeDocument(t *testing.T, doc Document) map[string]any {
	t.Helper()
	raw, err := doc.RenderJSON()
	if err != nil {
		t.Fatalf("RenderJSON() = %v, want no error", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal(%s) = %v", raw, err)
	}
	return decoded
}

// navigate resolves a JSON Pointer (RFC 6901) against a decoded document and
// fails the test if any segment is missing.
func navigate(t *testing.T, root any, pointer string) any {
	t.Helper()
	if pointer == "" {
		return root
	}
	if !strings.HasPrefix(pointer, "/") {
		t.Fatalf("json pointer %q must start with \"/\"", pointer)
	}

	current := root
	for _, segment := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		// RFC 6901 escaping: ~1 is "/" and ~0 is "~", decoded in that order.
		key := strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("json pointer %q: segment %q applied to a %T, not an object", pointer, key, current)
		}
		value, ok := object[key]
		if !ok {
			t.Fatalf("json pointer %q: segment %q not found", pointer, key)
		}
		current = value
	}
	return current
}
