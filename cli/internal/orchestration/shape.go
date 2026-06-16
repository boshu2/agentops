// practices: [hexagonal-architecture, design-by-contract]
package orchestration

import "strings"

// Orchestration-shape values — the four shapes the ATM/AM decision composes from
// two independent axes: contention (needs AM coordination) and durability (needs
// the ATM substrate). These string values are the contract: they match the
// chosen_shape enum in schemas/execution-packet.schema.json and the stamp in
// cli/cmd/ao/rpi_execution_packet.go. (A later slice unifies the two declarations
// behind this package.)
const (
	ShapeSingleAgent = "single-agent"
	ShapeAMOnly      = "am-only"
	ShapeATMOnly     = "atm-only"
	ShapeBoth        = "both"
)

// ShapeInputs are the observable, deterministic signals a shape proposal is
// validated against. The model proposes Proposed; the predicates check it against
// these inputs — the determinism inversion: trust the checkable terrain over the
// stochastic proposal. A confabulated "these lanes are disjoint" loses to the
// WriteSets overlap check.
type ShapeInputs struct {
	// Proposed is the model's proposed chosen_shape (may be empty / unrecognized).
	Proposed string
	// LiveWriters is the count of active writer lanes sharing the repo right now
	// (e.g. the row count from `am robot agents --active`).
	LiveWriters int
	// WriteSets is the per-lane set of write-path entries (globs or paths). Two
	// lanes contend only if their write-sets actually intersect.
	WriteSets [][]string
	// UnattendedNeed is true when the work must outlive the session or run
	// unattended (the durability axis → ATM).
	UnattendedNeed bool
}

// ShapeVerdict is the validated decision: the shape the predicates require given
// the observable inputs, which predicates fired, whether the proposal was
// overridden, and a one-line rationale. Shape/PredicatesFired/Rationale map
// directly onto the orchestration_decision record fields.
type ShapeVerdict struct {
	Shape           string
	PredicatesFired []string
	Overridden      bool
	Rationale       string
}

// ValidateShape resolves the orchestration shape from observable ground truth and
// records how it relates to the proposal. The validated Shape is the axis-required
// shape — the deterministic terrain wins over the proposal. Under-escalation
// (proposal missing a required axis) and over-escalation (proposal carrying an
// unneeded axis) are both classified into PredicatesFired so the record is honest
// about the disagreement. Over-escalation is downgraded too: stripping unneeded
// ceremony is the de-mandate; the dangerous direction (missing AM under real
// contention) is caught by the contention predicate.
func ValidateShape(in ShapeInputs) ShapeVerdict {
	needsAM := contention(in)
	needsATM := in.UnattendedNeed
	required := composeShape(needsAM, needsATM)

	fired := make([]string, 0, 4)
	if needsAM {
		fired = append(fired, "contention:live-writers-overlap")
	}
	if needsATM {
		fired = append(fired, "durability:unattended")
	}

	proposed := normalizeShape(in.Proposed)
	overridden := proposed != "" && proposed != required
	if overridden {
		switch {
		case shapeHasAM(required) && !shapeHasAM(proposed):
			fired = append(fired, "override:under-escalated-contention")
		case shapeHasAM(proposed) && !shapeHasAM(required):
			fired = append(fired, "override:over-escalated-contention")
		}
		switch {
		case shapeHasATM(required) && !shapeHasATM(proposed):
			fired = append(fired, "override:under-escalated-durability")
		case shapeHasATM(proposed) && !shapeHasATM(required):
			fired = append(fired, "override:over-escalated-durability")
		}
	}

	return ShapeVerdict{
		Shape:           required,
		PredicatesFired: fired,
		Overridden:      overridden,
		Rationale:       shapeRationale(required),
	}
}

// contention is true only when >=2 writer lanes are live AND their write-sets
// actually intersect. A sole writer, or disjoint write-sets, is not contention —
// this is the predicate that makes "partition before you lock" enforceable.
func contention(in ShapeInputs) bool {
	return in.LiveWriters >= 2 && writeSetsOverlap(in.WriteSets)
}

// writeSetsOverlap reports whether any two distinct lanes share an identical
// write-path entry. Exact-match is the conservative, deterministic floor;
// glob-intersection is a future refinement (a stricter check would flag more
// overlaps, never fewer, so exact-match never produces a false "disjoint").
func writeSetsOverlap(sets [][]string) bool {
	owner := make(map[string]int)
	for lane, set := range sets {
		for _, p := range set {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if prev, ok := owner[p]; ok && prev != lane {
				return true
			}
			owner[p] = lane
		}
	}
	return false
}

func composeShape(needsAM, needsATM bool) string {
	switch {
	case needsAM && needsATM:
		return ShapeBoth
	case needsAM:
		return ShapeAMOnly
	case needsATM:
		return ShapeATMOnly
	default:
		return ShapeSingleAgent
	}
}

func shapeHasAM(shape string) bool  { return shape == ShapeAMOnly || shape == ShapeBoth }
func shapeHasATM(shape string) bool { return shape == ShapeATMOnly || shape == ShapeBoth }

// normalizeShape lowercases/trims a proposal and returns it only if it is one of
// the four known shapes; an unrecognized proposal returns "" (no override
// classification, but the required shape is still resolved from ground truth).
func normalizeShape(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case ShapeSingleAgent:
		return ShapeSingleAgent
	case ShapeAMOnly:
		return ShapeAMOnly
	case ShapeATMOnly:
		return ShapeATMOnly
	case ShapeBoth:
		return ShapeBoth
	default:
		return ""
	}
}

func shapeRationale(shape string) string {
	switch shape {
	case ShapeBoth:
		return "contention (>=2 writers, overlapping write-sets) and unattended durability both fired"
	case ShapeAMOnly:
		return "contention fired (>=2 writers with overlapping write-sets); no unattended/durability need"
	case ShapeATMOnly:
		return "unattended/durability fired; no contention (sole writer or disjoint write-sets)"
	default:
		return "single-agent-first (operating-loop principle 8); no escalation predicate fired"
	}
}
