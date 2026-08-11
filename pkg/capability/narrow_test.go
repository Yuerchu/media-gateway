package capability

import (
	"errors"
	"testing"
)

// multiProduceSpec has more than one output so narrowing has something to do.
func multiProduceSpec() OperationSpec {
	spec := validSpec()
	spec.Produces = []string{"video/mp4", "video/webm", "video/ogg"}
	return spec
}

func TestNarrow_AcceptsSubset(t *testing.T) {
	tests := []struct {
		name     string
		produces []string
		want     []string
	}{
		{name: "proper subset", produces: []string{"video/webm"}, want: []string{"video/webm"}},
		{name: "equal set", produces: []string{"video/mp4", "video/ogg", "video/webm"},
			want: []string{"video/mp4", "video/ogg", "video/webm"}},
		{name: "argument order does not leak", produces: []string{"video/webm", "video/mp4"},
			want: []string{"video/mp4", "video/webm"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := mustOperation(t, multiProduceSpec())
			narrowed, err := op.Narrow(tt.produces)
			if err != nil {
				t.Fatalf("Narrow(%v) = %v, want no error", tt.produces, err)
			}
			got := narrowed.Produces()
			if len(got) != len(tt.want) {
				t.Fatalf("Produces() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("Produces()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNarrow_Rejects(t *testing.T) {
	tests := []struct {
		name     string
		produces []string
		want     error
	}{
		{name: "empty", produces: nil, want: ErrProducesEmpty},
		{name: "superset", produces: []string{"video/mp4", "video/quicktime"}, want: ErrProducesSuperset},
		{name: "entirely unsupported", produces: []string{"image/gif"}, want: ErrProducesSuperset},
		{name: "duplicate", produces: []string{"video/mp4", "video/mp4"}, want: ErrProducesDuplicate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := mustOperation(t, multiProduceSpec())
			_, err := op.Narrow(tt.produces)
			if !errors.Is(err, tt.want) {
				t.Errorf("Narrow(%v) = %v, want %v", tt.produces, err, tt.want)
			}
		})
	}
}

func TestNarrow_PreservesGuarantees(t *testing.T) {
	// The executable form of the rule in Narrow's doc comment: a deployment
	// policy restricts what an operation accepts and produces, never what the
	// implementation promises. Weakening exact to best_effort would enlarge the
	// set of admissible outputs — a widening dressed up as a narrowing.
	op := mustOperation(t, multiProduceSpec())
	narrowed, err := op.Narrow([]string{"video/mp4"})
	if err != nil {
		t.Fatalf("Narrow() = %v, want no error", err)
	}

	before, after := op.Guarantees(), narrowed.Guarantees()
	if len(before) != len(after) {
		t.Fatalf("Guarantees() has %d entries after narrowing, want %d", len(after), len(before))
	}
	for i := range before {
		if before[i].Dimension != after[i].Dimension || before[i].Guarantee != after[i].Guarantee {
			t.Errorf("dimension %q graded %q before narrowing and %q after",
				before[i].Dimension, before[i].Guarantee, after[i].Guarantee)
		}
	}
}

func TestNarrow_PreservesImplementationFacts(t *testing.T) {
	// Ignored fields, rejections and source requirements describe what the
	// implementation does, not what the deployment allows, so narrowing leaves
	// them alone too.
	spec := multiProduceSpec()
	spec.IgnoredFields = []IgnoredField{{Name: "fit_mode", Reason: "never read"}}
	spec.SourceRequires = SourceRequirements{Streams: []string{"video"}, MinDimension: 2}
	op := mustOperation(t, spec)

	narrowed, err := op.Narrow([]string{"video/mp4"})
	if err != nil {
		t.Fatalf("Narrow() = %v, want no error", err)
	}
	if got := len(narrowed.IgnoredFields()); got != 1 {
		t.Errorf("IgnoredFields() has %d entries after narrowing, want 1", got)
	}
	if got := len(narrowed.Rejections()); got != len(op.Rejections()) {
		t.Errorf("Rejections() has %d entries after narrowing, want %d", got, len(op.Rejections()))
	}
	if got := narrowed.SourceRequires().MinDimension; got != 2 {
		t.Errorf("SourceRequires().MinDimension = %d after narrowing, want 2", got)
	}
}

func TestNarrow_DoesNotMutateReceiver(t *testing.T) {
	// Narrow returns a value, and the shared slice headers underneath make an
	// accidental in-place write easy to introduce and hard to spot.
	op := mustOperation(t, multiProduceSpec())
	if _, err := op.Narrow([]string{"video/mp4"}); err != nil {
		t.Fatalf("Narrow() = %v, want no error", err)
	}
	if got := len(op.Produces()); got != 3 {
		t.Errorf("receiver Produces() has %d entries after narrowing, want 3", got)
	}
}

func TestNarrow_Composes(t *testing.T) {
	op := mustOperation(t, multiProduceSpec())
	wide := []string{"video/mp4", "video/webm"}
	narrow := []string{"video/mp4"}

	viaWide, err := op.Narrow(wide)
	if err != nil {
		t.Fatalf("Narrow(%v) = %v, want no error", wide, err)
	}
	twoStep, err := viaWide.Narrow(narrow)
	if err != nil {
		t.Fatalf("Narrow(%v) = %v, want no error", narrow, err)
	}
	oneStep, err := op.Narrow(narrow)
	if err != nil {
		t.Fatalf("Narrow(%v) = %v, want no error", narrow, err)
	}

	got, want := twoStep.Produces(), oneStep.Produces()
	if len(got) != len(want) {
		t.Fatalf("two-step Produces() = %v, one-step = %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("two-step Produces()[%d] = %q, one-step = %q", i, got[i], want[i])
		}
	}

	// And a second narrowing cannot climb back out of the first.
	if _, err := viaWide.Narrow([]string{"video/ogg"}); !errors.Is(err, ErrProducesSuperset) {
		t.Errorf("re-widening after narrowing = %v, want %v", err, ErrProducesSuperset)
	}
}
