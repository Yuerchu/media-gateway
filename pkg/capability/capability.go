// Package capability describes what a media processing operation accepts and
// what it promises about its output, in a form that can be published to
// consumers as a JSON Schema 2020-12 document.
//
// The idea it is built around: a constraint a caller sends in ("width at most
// 1920") and a promise about the result ("the output really will be at most
// 1920 wide") are different kinds of statement. JSON Schema can validate the
// first; its vocabulary has nothing at all to say about the second. Rather than
// dress promises up as validation keywords — which reads as a guarantee while
// checking nothing — this package keeps them as annotations and forces every
// one of them to be graded. See Guarantee.
//
// The same split runs through the whole package. Anything expressible as a
// schema keyword is emitted as one; anything that is not (cross-field
// relations, which property is the authoritative gate, why a field is accepted
// but ignored) is emitted as an x- annotation and labelled as such. A contract
// that quietly overstates what it checks is worse than no contract.
package capability

import (
	"fmt"
	"sort"
)

// Guarantee grades how faithfully an operation honours a requested constraint.
//
// Read as sets of admissible outputs the three grades nest:
//
//	exact ⊂ best_effort ⊂ none
//
// That ordering is the reason a deployment policy may not restate a guarantee
// in either direction. See Operation.Narrow.
type Guarantee string

const (
	// GuaranteeExact means the output always satisfies the constraint.
	GuaranteeExact Guarantee = "exact"

	// GuaranteeBestEffort means the operation aims at the constraint and fails
	// loudly when it cannot reach it, rather than silently returning a result
	// that violates it. Pair it with a BestEffortModel so callers can work out
	// in advance whether their particular request is reachable.
	GuaranteeBestEffort Guarantee = "best_effort"

	// GuaranteeNone means the operation does not act on this dimension at all:
	// a source violating the constraint yields output that still violates it.
	// Declaring none is not an admission of a missing feature — it is the only
	// honest answer when the underlying tool has no such parameter.
	GuaranteeNone Guarantee = "none"
)

// AllGuarantees returns every grade, ordered from strongest to weakest.
func AllGuarantees() []Guarantee {
	return []Guarantee{GuaranteeExact, GuaranteeBestEffort, GuaranteeNone}
}

// Valid reports whether g is a known grade.
func (g Guarantee) Valid() bool {
	for _, known := range AllGuarantees() {
		if g == known {
			return true
		}
	}
	return false
}

// Dimension names a measurable property of a media file. These strings are
// shared vocabulary with consumers written in other languages and appear
// verbatim in published documents; they must not be renamed once released.
type Dimension string

const (
	DimContentType Dimension = "content_type"
	DimWidth       Dimension = "width"
	DimHeight      Dimension = "height"
	DimDuration    Dimension = "duration_seconds"
	DimSize        Dimension = "size_bytes"
	DimFrameRate   Dimension = "frame_rate"
)

// NamedValue is a name/value pair. It is a slice element rather than a map
// entry because it appears in rendered documents where ordering is part of the
// output and must not depend on iteration order.
type NamedValue struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// BestEffortModel explains, for a dimension graded GuaranteeBestEffort, how the
// operation approaches the bound and when it is expected to fall short.
//
// Publishing the mechanism rather than just the grade is what makes
// best_effort actionable: a caller holding Formula and Constants can decide
// whether its particular request is reachable before paying for a round trip
// that was always going to fail.
type BestEffortModel struct {
	// Mechanism names the approach, e.g. "bitrate_budget".
	Mechanism string `json:"mechanism"`

	// Formula expresses the computation in terms of the names in Constants
	// plus properties of the request and source.
	Formula string `json:"formula,omitempty"`

	// Constants are the values Formula refers to.
	Constants []NamedValue `json:"constants,omitempty"`

	// FailsWhen states the condition under which the mechanism cannot reach
	// the requested bound.
	FailsWhen string `json:"fails_when,omitempty"`
}

// DimensionGuarantee is the frozen, ordered form of one graded dimension.
type DimensionGuarantee struct {
	Dimension Dimension `json:"dimension"`
	Guarantee Guarantee `json:"guarantee"`

	// Model is present only when Guarantee is GuaranteeBestEffort.
	Model *BestEffortModel `json:"model,omitempty"`

	// Note records anything a consumer would otherwise have to read the source
	// to learn, such as an output being rounded to an even number.
	Note string `json:"note,omitempty"`
}

// IgnoredField records a request field the operation accepts but never reads.
//
// Recording it is the difference between a caller learning about it from the
// contract and learning about it from a support ticket. An ignored field is a
// statement of fact about the implementation, so a deployment policy cannot
// add or remove one.
type IgnoredField struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// ResultGuarantee grades a field of the operation's result payload, addressed
// by JSON Pointer.
//
// It exists because the result is often not the HTTP response body: it may
// arrive in a callback, or be fetched later from a task record. Grading those
// fields is the only way to say things like "this key is always present, that
// one only when probing succeeded".
type ResultGuarantee struct {
	Pointer   string    `json:"pointer"`
	Guarantee Guarantee `json:"guarantee"`
	Note      string    `json:"note,omitempty"`
}

// RejectStage says at what point an operation refuses a request.
type RejectStage string

const (
	// RejectStageRequest is a refusal decided from the request alone, before
	// any source has been fetched.
	RejectStageRequest RejectStage = "request"

	// RejectStageSource is a refusal decided after the source was fetched and
	// inspected. A caller cannot avoid the fetch cost for these.
	RejectStageSource RejectStage = "source"
)

// Rejection is an input the operation refuses.
//
// Publishing the full set lets a consumer tell "this input is unacceptable"
// apart from "this service is broken" without matching on error strings. Code
// is echoed in error responses so the classification survives translation and
// rewording.
type Rejection struct {
	// Code is a stable machine-readable identifier. Never reuse or repurpose
	// one; consumers branch on it.
	Code string `json:"code"`

	// Stage says whether the refusal costs a source fetch.
	Stage RejectStage `json:"stage"`

	// SchemaPath is a JSON Pointer into the rendered document, locating the
	// constraint that was violated. It may point at an x- annotation when the
	// precondition is one JSON Schema cannot express.
	SchemaPath string `json:"schema_path"`

	// HTTPStatus is the status an HTTP transport returns for this rejection.
	HTTPStatus int `json:"http_status"`

	// Summary is a short human-readable description.
	Summary string `json:"summary"`
}

// SourceRequirements are preconditions an operation places on the source,
// on top of the document-wide ingest limits in SourceLimits.
type SourceRequirements struct {
	// Streams lists stream kinds the source must contain, e.g. "video".
	// A source lacking one is rejected rather than handled degenerately.
	Streams []string `json:"streams,omitempty"`

	// MinDimension is the smallest source width or height the operation
	// handles. Below it the geometry maths degenerates — a pipeline that
	// rounds down to even numbers turns a 1px edge into 0px — so the floor is
	// a precondition rather than something the operation quietly absorbs.
	MinDimension int `json:"min_dimension,omitempty"`
}

// OperationSpec is the input to NewOperation.
//
// It exists so the immutable parts of an Operation can be written plainly at
// the declaration site, next to the code they describe, and then be
// unreachable afterwards. Maps are accepted here for readability; NewOperation
// freezes them into sorted slices.
type OperationSpec struct {
	// Name identifies the operation, e.g. "transcode".
	Name string

	// Produces lists the output MIME types. List only what the implementation
	// actually exercises: a wider list is itself a false claim, and it would
	// be a false claim made by the very mechanism built to remove them.
	Produces []string

	// Guarantees grades the requested *upper bound* on each dimension.
	//
	// Lower bounds never appear here. A scale-down pipeline cannot enlarge
	// anything, so a requested minimum is never a promise about the output —
	// it is a precondition on the source, and belongs in Rejections or
	// SourceRequirements.
	Guarantees map[Dimension]Guarantee

	// BestEffortModels supplies the mechanism for dimensions graded
	// GuaranteeBestEffort. A model for any other grade is a construction error.
	BestEffortModels map[Dimension]BestEffortModel

	// Notes annotates individual dimensions.
	Notes map[Dimension]string

	// IgnoredFields lists request fields accepted but never read.
	IgnoredFields []IgnoredField

	// ResultGuarantees grades fields of the result payload.
	ResultGuarantees []ResultGuarantee

	// Rejections lists the inputs this operation refuses.
	Rejections []Rejection

	// SourceRequires are preconditions on the source.
	SourceRequires SourceRequirements
}

// Operation is one callable capability.
//
// Every field is unexported. A deployment policy is entitled to restrict what
// an operation accepts and produces, but not to restate what the
// implementation promises, and putting the promises out of reach is what turns
// that rule from a comment into something the compiler enforces. Build one
// with NewOperation, next to the code it describes.
type Operation struct {
	name             string
	produces         []string
	guarantees       []DimensionGuarantee
	ignoredFields    []IgnoredField
	resultGuarantees []ResultGuarantee
	rejections       []Rejection
	sourceRequires   SourceRequirements

	// requestSchema is policy-supplied rather than declared alongside the
	// implementation, so unlike the rest it has a setter. See
	// Operation.WithRequestSchema.
	requestSchema *Schema
}

// NewOperation validates spec and freezes it into an Operation.
//
// Validation is deliberately strict: a capability document is read by
// programs, and a malformed one is worse than a missing one. Callers are
// expected to build operations at process start so a mistake is a boot
// failure rather than a bad response to some later request.
func NewOperation(spec OperationSpec) (Operation, error) {
	if spec.Name == "" {
		return Operation{}, fmt.Errorf("operation name is required")
	}
	produces, err := normalizeProduces(spec.Name, spec.Produces)
	if err != nil {
		return Operation{}, err
	}
	guarantees, err := freezeGuarantees(spec)
	if err != nil {
		return Operation{}, err
	}
	if err := validateRejections(spec.Name, spec.Rejections); err != nil {
		return Operation{}, err
	}

	return Operation{
		name:             spec.Name,
		produces:         produces,
		guarantees:       guarantees,
		ignoredFields:    append([]IgnoredField(nil), spec.IgnoredFields...),
		resultGuarantees: append([]ResultGuarantee(nil), spec.ResultGuarantees...),
		rejections:       append([]Rejection(nil), spec.Rejections...),
		sourceRequires:   cloneSourceRequirements(spec.SourceRequires),
	}, nil
}

// normalizeProduces validates the output MIME list and returns a sorted copy.
func normalizeProduces(opName string, produces []string) ([]string, error) {
	if len(produces) == 0 {
		return nil, fmt.Errorf("operation %q produces nothing: express that by not registering it", opName)
	}
	seen := make(map[string]bool, len(produces))
	out := make([]string, 0, len(produces))
	for _, mime := range produces {
		if mime == "" {
			return nil, fmt.Errorf("operation %q has an empty produces entry", opName)
		}
		if seen[mime] {
			return nil, fmt.Errorf("operation %q lists produces %q twice", opName, mime)
		}
		seen[mime] = true
		out = append(out, mime)
	}
	sort.Strings(out)
	return out, nil
}

// freezeGuarantees merges the grade, model and note maps into one slice sorted
// by dimension, so rendered output does not depend on map iteration order.
func freezeGuarantees(spec OperationSpec) ([]DimensionGuarantee, error) {
	for dim, model := range spec.BestEffortModels {
		if spec.Guarantees[dim] != GuaranteeBestEffort {
			return nil, fmt.Errorf(
				"operation %q supplies a %q model for dimension %q graded %q: a model only means something for %q",
				spec.Name, model.Mechanism, dim, spec.Guarantees[dim], GuaranteeBestEffort,
			)
		}
	}

	out := make([]DimensionGuarantee, 0, len(spec.Guarantees))
	for dim, grade := range spec.Guarantees {
		if !grade.Valid() {
			return nil, fmt.Errorf("operation %q grades dimension %q as unknown guarantee %q", spec.Name, dim, grade)
		}
		entry := DimensionGuarantee{Dimension: dim, Guarantee: grade, Note: spec.Notes[dim]}
		if model, ok := spec.BestEffortModels[dim]; ok {
			cloned := model
			cloned.Constants = append([]NamedValue(nil), model.Constants...)
			entry.Model = &cloned
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dimension < out[j].Dimension })
	return out, nil
}

// validateRejections checks that every rejection is addressable by consumers.
func validateRejections(opName string, rejections []Rejection) error {
	seen := make(map[string]bool, len(rejections))
	for _, r := range rejections {
		if r.Code == "" {
			return fmt.Errorf("operation %q has a rejection without a code", opName)
		}
		if seen[r.Code] {
			return fmt.Errorf("operation %q uses rejection code %q twice", opName, r.Code)
		}
		seen[r.Code] = true
		if r.Stage != RejectStageRequest && r.Stage != RejectStageSource {
			return fmt.Errorf("rejection %q has unknown stage %q", r.Code, r.Stage)
		}
		if r.SchemaPath == "" {
			return fmt.Errorf("rejection %q has no schema_path: consumers cannot locate what it refers to", r.Code)
		}
	}
	return nil
}

func cloneSourceRequirements(src SourceRequirements) SourceRequirements {
	return SourceRequirements{
		Streams:      append([]string(nil), src.Streams...),
		MinDimension: src.MinDimension,
	}
}

// Name returns the operation's identifier.
func (o Operation) Name() string { return o.name }

// Produces returns a copy of the output MIME list, sorted.
func (o Operation) Produces() []string { return append([]string(nil), o.produces...) }

// Guarantees returns a copy of the graded dimensions, sorted by dimension.
func (o Operation) Guarantees() []DimensionGuarantee {
	out := make([]DimensionGuarantee, len(o.guarantees))
	for i, g := range o.guarantees {
		out[i] = g
		if g.Model != nil {
			cloned := *g.Model
			cloned.Constants = append([]NamedValue(nil), g.Model.Constants...)
			out[i].Model = &cloned
		}
	}
	return out
}

// Guarantee returns the grade for dim, or GuaranteeNone when the operation says
// nothing about it. Silence and "does not act on this" are the same claim, so
// collapsing them is safe and keeps callers from special-casing absence.
func (o Operation) Guarantee(dim Dimension) Guarantee {
	for _, g := range o.guarantees {
		if g.Dimension == dim {
			return g.Guarantee
		}
	}
	return GuaranteeNone
}

// IgnoredFields returns a copy of the accepted-but-unread request fields.
func (o Operation) IgnoredFields() []IgnoredField {
	return append([]IgnoredField(nil), o.ignoredFields...)
}

// ResultGuarantees returns a copy of the graded result-payload fields.
func (o Operation) ResultGuarantees() []ResultGuarantee {
	return append([]ResultGuarantee(nil), o.resultGuarantees...)
}

// Rejections returns a copy of the refused inputs.
func (o Operation) Rejections() []Rejection {
	return append([]Rejection(nil), o.rejections...)
}

// SourceRequires returns a copy of the operation's preconditions on the source.
func (o Operation) SourceRequires() SourceRequirements {
	return cloneSourceRequirements(o.sourceRequires)
}

// SourceLimits are the ingest-wide preconditions every source must satisfy,
// whichever operation it is destined for. They live on the Document rather
// than on an Operation because they are enforced while fetching, before the
// operation gets a say.
type SourceLimits struct {
	// MaxSizeBytes is the largest source the service will fetch.
	MaxSizeBytes int64 `json:"max_size_bytes"`

	// ContainerFormats lists accepted container formats as the probe reports
	// them. This is the authoritative gate.
	ContainerFormats []string `json:"container_formats"`

	// ContentTypes is an advisory MIME view of ContainerFormats, for consumers
	// that hold a declared MIME type and no probe output. It is a pre-filter,
	// not the gate: a declared MIME type is whatever the uploader claimed,
	// while the container format is what the bytes turned out to be, and the
	// two disagree often enough to matter.
	ContentTypes []string `json:"content_types,omitempty"`
}

// Document is a complete capability contract for one service.
type Document struct {
	// Version is the contract's semantic version, not the service's.
	Version string

	// ServiceID identifies the service in the rendered $id.
	ServiceID string

	// Title is a human-readable name for the contract.
	Title string

	// Source holds the ingest-wide preconditions.
	Source SourceLimits

	// Operations are the callable capabilities, in the order they should be
	// presented.
	Operations []Operation
}
