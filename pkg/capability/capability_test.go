package capability

import (
	"errors"
	"strings"
	"testing"
)

// validSpec is a minimal well-formed spec that individual tests bend in one
// direction each, so a failure names exactly the rule that broke.
func validSpec() OperationSpec {
	return OperationSpec{
		Name:     "transcode",
		Produces: []string{"video/mp4"},
		Guarantees: map[Dimension]Guarantee{
			DimContentType: GuaranteeExact,
			DimSize:        GuaranteeBestEffort,
			DimFrameRate:   GuaranteeNone,
		},
		BestEffortModels: map[Dimension]BestEffortModel{
			DimSize: {Mechanism: "bitrate_budget"},
		},
		Rejections: []Rejection{
			{
				Code:       "source_too_large",
				Stage:      RejectStageSource,
				SchemaPath: "/$defs/source/properties/size_bytes/maximum",
				HTTPStatus: 422,
				Summary:    "source exceeds the ingest limit",
			},
		},
	}
}

func mustOperation(t *testing.T, spec OperationSpec) Operation {
	t.Helper()
	op, err := NewOperation(spec)
	if err != nil {
		t.Fatalf("NewOperation(%q) = %v, want no error", spec.Name, err)
	}
	return op
}

func TestGuarantee_Valid(t *testing.T) {
	for _, g := range AllGuarantees() {
		if !g.Valid() {
			t.Errorf("Guarantee(%q).Valid() = false, want true", g)
		}
	}
	for _, g := range []Guarantee{"", "exact ", "EXACT", "probably"} {
		if Guarantee(g).Valid() {
			t.Errorf("Guarantee(%q).Valid() = true, want false", g)
		}
	}
}

func TestAllGuarantees_OrderedStrongestFirst(t *testing.T) {
	// Rendered documents and prose both lean on this order; a reshuffle would
	// silently invert how the grades read.
	want := []Guarantee{GuaranteeExact, GuaranteeBestEffort, GuaranteeNone}
	got := AllGuarantees()
	if len(got) != len(want) {
		t.Fatalf("AllGuarantees() has %d grades, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllGuarantees()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNewOperation_Rejects(t *testing.T) {
	tests := []struct {
		name string
		bend func(*OperationSpec)
	}{
		{
			name: "empty name",
			bend: func(s *OperationSpec) { s.Name = "" },
		},
		{
			name: "produces nothing",
			bend: func(s *OperationSpec) { s.Produces = nil },
		},
		{
			name: "duplicate produces",
			bend: func(s *OperationSpec) { s.Produces = []string{"video/mp4", "video/mp4"} },
		},
		{
			name: "empty produces entry",
			bend: func(s *OperationSpec) { s.Produces = []string{""} },
		},
		{
			name: "unknown guarantee",
			bend: func(s *OperationSpec) { s.Guarantees[DimWidth] = "probably" },
		},
		{
			name: "model on an exact dimension",
			bend: func(s *OperationSpec) {
				s.BestEffortModels[DimContentType] = BestEffortModel{Mechanism: "wishful"}
			},
		},
		{
			name: "model on an ungraded dimension",
			bend: func(s *OperationSpec) {
				s.BestEffortModels[DimDuration] = BestEffortModel{Mechanism: "wishful"}
			},
		},
		{
			name: "rejection without code",
			bend: func(s *OperationSpec) { s.Rejections[0].Code = "" },
		},
		{
			name: "rejection without schema path",
			bend: func(s *OperationSpec) { s.Rejections[0].SchemaPath = "" },
		},
		{
			name: "rejection with unknown stage",
			bend: func(s *OperationSpec) { s.Rejections[0].Stage = "whenever" },
		},
		{
			name: "duplicate rejection code",
			bend: func(s *OperationSpec) {
				dup := s.Rejections[0]
				s.Rejections = append(s.Rejections, dup)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validSpec()
			tt.bend(&spec)
			if _, err := NewOperation(spec); err == nil {
				t.Error("NewOperation() = nil error, want a rejection")
			}
		})
	}
}

func TestNewOperation_AcceptsValidSpec(t *testing.T) {
	op := mustOperation(t, validSpec())
	if op.Name() != "transcode" {
		t.Errorf("Name() = %q, want %q", op.Name(), "transcode")
	}
}

func TestOperation_GuaranteesSortedByDimension(t *testing.T) {
	// Grades arrive in a map. Freezing them into a sorted slice is what keeps
	// rendered bytes from depending on map iteration order.
	op := mustOperation(t, validSpec())
	got := op.Guarantees()
	for i := 1; i < len(got); i++ {
		if got[i-1].Dimension >= got[i].Dimension {
			t.Errorf("Guarantees() not sorted: %q before %q", got[i-1].Dimension, got[i].Dimension)
		}
	}
}

func TestOperation_GuaranteeDefaultsToNone(t *testing.T) {
	// Saying nothing about a dimension and saying "I do not act on it" are the
	// same claim, so callers should not have to distinguish absence.
	op := mustOperation(t, validSpec())
	if got := op.Guarantee(DimDuration); got != GuaranteeNone {
		t.Errorf("Guarantee(%q) = %q, want %q for an ungraded dimension", DimDuration, got, GuaranteeNone)
	}
}

func TestOperation_AccessorsReturnCopies(t *testing.T) {
	spec := validSpec()
	spec.IgnoredFields = []IgnoredField{{Name: "fit_mode", Reason: "never read"}}
	spec.ResultGuarantees = []ResultGuarantee{{Pointer: "/size", Guarantee: GuaranteeExact}}
	spec.BestEffortModels[DimSize] = BestEffortModel{
		Mechanism: "bitrate_budget",
		Constants: []NamedValue{{Name: "audio_bitrate_bps", Value: 128000}},
	}
	spec.SourceRequires = SourceRequirements{Streams: []string{"video"}, MinDimension: 2}
	op := mustOperation(t, spec)

	op.Produces()[0] = "video/tampered"
	op.IgnoredFields()[0].Name = "tampered"
	op.ResultGuarantees()[0].Pointer = "/tampered"
	op.Rejections()[0].Code = "tampered"
	op.SourceRequires().Streams[0] = "tampered"
	guarantees := op.Guarantees()
	for i := range guarantees {
		guarantees[i].Guarantee = "tampered"
		if guarantees[i].Model != nil {
			guarantees[i].Model.Constants[0].Value = -1
		}
	}

	if got := op.Produces()[0]; got != "video/mp4" {
		t.Errorf("Produces()[0] = %q after caller mutation, want %q", got, "video/mp4")
	}
	if got := op.IgnoredFields()[0].Name; got != "fit_mode" {
		t.Errorf("IgnoredFields()[0].Name = %q after caller mutation, want %q", got, "fit_mode")
	}
	if got := op.ResultGuarantees()[0].Pointer; got != "/size" {
		t.Errorf("ResultGuarantees()[0].Pointer = %q after caller mutation, want %q", got, "/size")
	}
	if got := op.Rejections()[0].Code; got != "source_too_large" {
		t.Errorf("Rejections()[0].Code = %q after caller mutation, want %q", got, "source_too_large")
	}
	if got := op.SourceRequires().Streams[0]; got != "video" {
		t.Errorf("SourceRequires().Streams[0] = %q after caller mutation, want %q", got, "video")
	}
	if got := op.Guarantee(DimContentType); got != GuaranteeExact {
		t.Errorf("Guarantee(%q) = %q after caller mutation, want %q", DimContentType, got, GuaranteeExact)
	}
	for _, g := range op.Guarantees() {
		if g.Dimension == DimSize && g.Model.Constants[0].Value != 128000 {
			t.Errorf("model constant = %v after caller mutation, want 128000", g.Model.Constants[0].Value)
		}
	}
}

func TestNewOperation_ErrorNamesTheOperation(t *testing.T) {
	// These errors surface at process start. One that does not say which
	// operation is at fault costs a bisect through every declaration.
	spec := validSpec()
	spec.Produces = nil
	_, err := NewOperation(spec)
	if err == nil {
		t.Fatal("NewOperation() = nil error, want a rejection")
	}
	if !errorMentions(err, "transcode") {
		t.Errorf("error %q does not name the operation", err)
	}
}

func errorMentions(err error, want string) bool {
	for err != nil {
		if strings.Contains(err.Error(), want) {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}
