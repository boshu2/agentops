package evalsubstrate

import (
	"fmt"
	"strings"
)

// Refusal is the structured §6 gate-refusal record.
//
// MUST format exactly per SCHEMA.md §6:
//
//	GATE FAILED: <gate-number> <gate-name>
//	  Why:      <one-sentence explanation>
//	  Evidence: <the offending diff / quota state / kappa-history excerpt>
//	  Fix:      <the exact command that would unblock the run>
type Refusal struct {
	GateNumber int
	GateName   string
	Why        string
	Evidence   string
	Fix        string
}

func (r Refusal) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "GATE FAILED: %d %s\n", r.GateNumber, r.GateName)
	fmt.Fprintf(&b, "  Why:      %s\n", r.Why)
	fmt.Fprintf(&b, "  Evidence: %s\n", r.Evidence)
	fmt.Fprintf(&b, "  Fix:      %s", r.Fix)
	return b.String()
}

func (r Refusal) Error() string { return r.Format() }

type Refusals []Refusal

func (rs Refusals) Format() string {
	parts := make([]string, len(rs))
	for i, r := range rs {
		parts[i] = r.Format()
	}
	return strings.Join(parts, "\n\n")
}

func (rs Refusals) Empty() bool { return len(rs) == 0 }
