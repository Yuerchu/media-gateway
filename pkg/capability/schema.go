package capability

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Dialect is the JSON Schema dialect every rendered document declares.
const Dialect = "https://json-schema.org/draft/2020-12/schema"

// Schema is the subset of JSON Schema 2020-12 this package emits.
//
// It is deliberately small. Every keyword here is one a consumer's validator
// has to implement, and emitting a keyword we do not also exercise in our own
// tests is how a contract quietly turns decorative — it keeps reading like a
// constraint while nothing checks it any more. Add a keyword only together
// with the code that honours it.
//
// Anything JSON Schema cannot express — cross-field relations, which property
// is the authoritative gate, why a field is accepted and then ignored — goes in
// Extensions under an x- name, so a reader can tell a checked constraint from
// a documented one at a glance.
type Schema struct {
	Dialect     string `json:"$schema,omitempty"`
	ID          string `json:"$id,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`

	Type string `json:"type,omitempty"`

	// Properties is a map because JSON Schema models it as an object. Go's
	// encoding/json sorts map keys, so this stays deterministic — see
	// Document.RenderJSON.
	Properties map[string]*Schema `json:"properties,omitempty"`

	Required []string `json:"required,omitempty"`
	Enum     []string `json:"enum,omitempty"`

	Minimum          *float64 `json:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	MinLength        *int     `json:"minLength,omitempty"`

	Deprecated bool `json:"deprecated,omitempty"`

	// UnevaluatedProperties set to false makes a consumer's stray field an
	// error instead of something silently dropped, which is how drift between
	// the two sides gets noticed.
	UnevaluatedProperties *bool `json:"unevaluatedProperties,omitempty"`

	AllOf []*Schema `json:"allOf,omitempty"`
	If    *Schema   `json:"if,omitempty"`
	Then  *Schema   `json:"then,omitempty"`
	Const string    `json:"const,omitempty"`

	Defs map[string]*Schema `json:"$defs,omitempty"`

	// Extensions holds x- annotations, merged into the object by MarshalJSON.
	Extensions map[string]any `json:"-"`
}

// MarshalJSON renders the schema with its x- extensions inlined as siblings of
// the standard keywords, which is where JSON Schema expects vocabulary
// extensions to live.
//
// Names not starting with "x-" are refused: an unprefixed unknown key is
// indistinguishable from a misspelled keyword, and a validator would ignore
// both.
func (s Schema) MarshalJSON() ([]byte, error) {
	type plain Schema // shed the method set to avoid recursing
	encoded, err := json.Marshal(plain(s))
	if err != nil {
		return nil, err
	}
	if len(s.Extensions) == 0 {
		return encoded, nil
	}

	merged := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &merged); err != nil {
		return nil, fmt.Errorf("re-reading encoded schema: %w", err)
	}
	for name, value := range s.Extensions {
		if !strings.HasPrefix(name, "x-") {
			return nil, fmt.Errorf("schema extension %q must start with \"x-\"", name)
		}
		if _, taken := merged[name]; taken {
			return nil, fmt.Errorf("schema extension %q collides with a rendered keyword", name)
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encoding schema extension %q: %w", name, err)
		}
		merged[name] = raw
	}
	return json.Marshal(merged)
}

// Float returns a pointer to v, for the numeric bound fields.
func Float(v float64) *float64 { return &v }

// Int returns a pointer to v, for MinLength.
func Int(v int) *int { return &v }

// Bool returns a pointer to v, for UnevaluatedProperties.
func Bool(v bool) *bool { return &v }

// WithRequestSchema attaches the request-body schema for this operation,
// returning a copy. The receiver is not modified.
//
// Unlike the guarantees, the request shape belongs to the service exposing the
// operation rather than to the implementation behind it — the same library
// function is reached through different request bodies in different services —
// so the policy layer supplies it.
func (o Operation) WithRequestSchema(schema *Schema) Operation {
	updated := o
	updated.requestSchema = schema
	return updated
}

// RequestSchema returns the attached request-body schema, or nil.
func (o Operation) RequestSchema() *Schema { return o.requestSchema }

// sourceDefName is the $defs key holding the ingest-wide preconditions.
// Operations sit alongside it under their own names, so no operation may claim
// this one.
const sourceDefName = "source"

// Render builds the JSON Schema document for the contract.
//
// Layout, and why: each operation is a $defs entry under its own name, so a
// Rejection's SchemaPath is a short stable JSON Pointer such as
// /$defs/transcode/properties/target_mime/enum. Collecting operations in an
// array instead would make every pointer positional, and they would all
// silently retarget the moment an operation was added or reordered.
func (d Document) Render() (*Schema, error) {
	if d.Version == "" {
		return nil, fmt.Errorf("document version is required")
	}
	if d.ServiceID == "" {
		return nil, fmt.Errorf("document service id is required")
	}

	defs := map[string]*Schema{sourceDefName: renderSourceLimits(d.Source)}
	for _, op := range d.Operations {
		if op.Name() == sourceDefName {
			return nil, fmt.Errorf("operation may not be named %q: it collides with the source definition", sourceDefName)
		}
		if _, taken := defs[op.Name()]; taken {
			return nil, fmt.Errorf("document declares operation %q twice", op.Name())
		}
		rendered, err := renderOperation(op)
		if err != nil {
			return nil, err
		}
		defs[op.Name()] = rendered
	}

	return &Schema{
		Dialect: Dialect,
		ID:      fmt.Sprintf("https://%s/schemas/capabilities/%s", d.ServiceID, d.Version),
		Title:   d.Title,
		Defs:    defs,
		Extensions: map[string]any{
			"x-contract-version": d.Version,
			"x-operations":       operationNames(d.Operations),
		},
	}, nil
}

// operationNames lists the operations in declaration order, so a consumer can
// enumerate them without having to filter the source definition out of $defs.
func operationNames(operations []Operation) []string {
	names := make([]string, 0, len(operations))
	for _, op := range operations {
		names = append(names, op.Name())
	}
	return names
}

// RenderJSON renders the contract and encodes it.
//
// It uses encoding/json rather than a faster encoder on purpose: encoding/json
// sorts map keys, so the same Document always produces byte-identical output.
// Callers cache and ETag these bytes, and a non-deterministic encoder would
// make both meaningless.
func (d Document) RenderJSON() ([]byte, error) {
	schema, err := d.Render()
	if err != nil {
		return nil, err
	}
	return json.Marshal(schema)
}

// renderSourceLimits renders the ingest-wide preconditions.
func renderSourceLimits(limits SourceLimits) *Schema {
	properties := map[string]*Schema{
		"container_format": {
			Type:        "string",
			Description: "Container format as the probe reports it.",
			Enum:        append([]string(nil), limits.ContainerFormats...),
		},
		"size_bytes": {
			Type:    "integer",
			Minimum: Float(1),
			Maximum: Float(float64(limits.MaxSizeBytes)),
		},
	}
	required := []string{"container_format", "size_bytes"}

	if len(limits.ContentTypes) > 0 {
		properties["content_type"] = &Schema{
			Type: "string",
			Enum: append([]string(nil), limits.ContentTypes...),
			Extensions: map[string]any{
				"x-advisory": true,
			},
			Description: "Advisory pre-filter for callers holding a declared MIME type " +
				"and no probe output. Passing it does not mean the source is accepted.",
		}
	}

	return &Schema{
		Title:      "Source preconditions",
		Type:       "object",
		Properties: properties,
		Required:   required,
		Extensions: map[string]any{
			"x-authoritative-gate": map[string]any{
				"property": "container_format",
				"reason": "A declared content type is whatever the uploader claimed; " +
					"the container format is what the bytes turned out to be. They disagree " +
					"often enough that only the latter can be the gate.",
			},
		},
	}
}

// renderOperation renders one operation's contract.
//
// The operation's $defs entry *is* its request schema, carrying the guarantees
// and rejections as x- annotations on the same object. Nesting the request
// schema one level deeper would be tidier to look at but would push every
// SchemaPath a segment further from the natural
// /$defs/operations/<name>/properties/<field>.
func renderOperation(op Operation) (*Schema, error) {
	rendered := Schema{Title: op.Name()}
	if schema := op.RequestSchema(); schema != nil {
		rendered = *schema
		rendered.Title = op.Name()
	}

	extensions := map[string]any{
		"x-produces":          op.Produces(),
		"x-output-guarantees": op.Guarantees(),
		"x-rejections":        op.Rejections(),
		"x-guarantee-semantics": map[string]any{
			"exact":       "the output always satisfies the requested bound",
			"best_effort": "the operation aims at the bound and fails loudly when it cannot reach it",
			"none":        "the operation does not act on this dimension at all",
			"scope": "Guarantees describe the requested upper bound on a dimension. " +
				"A requested lower bound is never a guarantee — a scale-down pipeline " +
				"cannot enlarge anything — it appears as a rejection instead.",
		},
	}
	if ignored := op.IgnoredFields(); len(ignored) > 0 {
		extensions["x-ignored-properties"] = ignored
	}
	if results := op.ResultGuarantees(); len(results) > 0 {
		extensions["x-result-guarantees"] = results
	}
	if requires := op.SourceRequires(); len(requires.Streams) > 0 || requires.MinDimension > 0 {
		extensions["x-source-requirements"] = requires
	}

	merged := make(map[string]any, len(rendered.Extensions)+len(extensions))
	for name, value := range rendered.Extensions {
		merged[name] = value
	}
	for name, value := range extensions {
		if _, taken := merged[name]; taken {
			return nil, fmt.Errorf(
				"operation %q request schema already sets %q, which the renderer owns",
				op.Name(), name,
			)
		}
		merged[name] = value
	}
	rendered.Extensions = merged
	return &rendered, nil
}
