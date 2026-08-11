package capability

import (
	"errors"
	"fmt"
	"sort"
)

// Errors returned by Operation.Narrow.
var (
	// ErrProducesEmpty is returned when a policy narrows an operation down to
	// producing nothing.
	ErrProducesEmpty = errors.New("narrowed produces list is empty")

	// ErrProducesSuperset is returned when a policy tries to add an output the
	// implementation does not support.
	ErrProducesSuperset = errors.New("narrowed produces list is not a subset")

	// ErrProducesDuplicate is returned when a policy lists the same output
	// twice. A duplicate in a policy list is a typo; swallowing it silently
	// would hide a configuration bug.
	ErrProducesDuplicate = errors.New("narrowed produces list has duplicates")
)

// Narrow restricts an operation to a subset of its output MIME types,
// returning the restricted copy. The receiver is not modified.
//
// # What narrowing is allowed to touch, and why
//
// Narrowing has to preserve safe substitutability: a program written against
// the narrowed contract must still be correct against the broad one.
//
// Produces may only shrink. A client of the narrow contract never asks for a
// removed output, and had it done so the broad contract would have handled it
// anyway. Growing the list is unsound in the other direction — the
// implementation genuinely cannot deliver what was added.
//
// Guarantees may not move at all, in either direction, and this method offers
// no way to touch them. Read as sets of admissible outputs the grades nest —
// exact ⊂ best_effort ⊂ none — so weakening exact to best_effort *enlarges*
// the admissible set. That is a widening, and it breaks every client that
// relied on the stronger promise. It reads like narrowing only because "narrow"
// gets heard as "make less capable". Strengthening is closed off for the
// opposite reason: a deployment policy is never in a position to promise
// something the implementation cannot do. The same applies to BestEffortModel
// and SourceRequires, which are statements of fact about the implementation
// rather than policy knobs.
//
// The request surface — schema, ignored fields, rejections — is not covered by
// any of this. It belongs to the service rather than the implementation and is
// supplied outright through the With* methods.
//
// None of this needs a runtime check. Guarantees are unexported and the result
// is built from the receiver, so the type system enforces it.
//
// Ordering follows the receiver, not the argument, so a policy file's ordering
// cannot leak into the rendered document and make it non-deterministic.
//
// Narrow composes: for B ⊆ A ⊆ o.Produces(), o.Narrow(A).Narrow(B) equals
// o.Narrow(B).
func (o Operation) Narrow(produces []string) (Operation, error) {
	if len(produces) == 0 {
		return Operation{}, fmt.Errorf("%w for operation %q", ErrProducesEmpty, o.name)
	}

	supported := make(map[string]bool, len(o.produces))
	for _, mime := range o.produces {
		supported[mime] = true
	}

	wanted := make(map[string]bool, len(produces))
	for _, mime := range produces {
		if wanted[mime] {
			return Operation{}, fmt.Errorf("%w: operation %q, %q", ErrProducesDuplicate, o.name, mime)
		}
		wanted[mime] = true
		if !supported[mime] {
			return Operation{}, fmt.Errorf(
				"%w: operation %q cannot produce %q, it supports %v",
				ErrProducesSuperset, o.name, mime, o.produces,
			)
		}
	}

	kept := make([]string, 0, len(wanted))
	for _, mime := range o.produces {
		if wanted[mime] {
			kept = append(kept, mime)
		}
	}
	sort.Strings(kept)

	// The remaining slice fields are shared with the receiver rather than
	// copied. That is safe precisely because they are unexported and every
	// accessor hands out a copy, so neither the receiver nor the result can be
	// mutated through the other.
	narrowed := o
	narrowed.produces = kept
	return narrowed, nil
}
