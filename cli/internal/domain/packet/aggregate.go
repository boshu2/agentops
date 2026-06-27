// Package packet defines the ExecutionPacket aggregate root.
// This is the linked-intent object that carries discovery output
// through the RPI pipeline. See docs/cdlc.md and
// docs/architecture/operating-loop.md.
package packet

// ExecutionPacket is the Aggregate Root for an RPI discovery output.
// It is the type alias migration of cli/internal/rpi.ExecutionPacket,
// re-exposed here as the domain-canonical type. The rpi package keeps
// the old name as an alias for back-compat.
type ExecutionPacket struct {
	PlanPath         string      `json:"plan_path"`
	EpicID           string      `json:"epic_id,omitempty"`
	Complexity       Complexity  `json:"complexity"`
	TestLevels       []TestLevel `json:"test_levels"`
	DefaultVerdict   Verdict     `json:"default_verdict,omitempty"`
	RankedPacketPath string      `json:"ranked_packet_path,omitempty"`
	Provenance       Provenance  `json:"provenance"`
}

type Complexity string

const (
	ComplexityFast     Complexity = "fast"
	ComplexityStandard Complexity = "standard"
	ComplexityFull     Complexity = "full"
)

type TestLevel string

const (
	L0 TestLevel = "L0"
	L1 TestLevel = "L1"
	L2 TestLevel = "L2"
	L3 TestLevel = "L3"
)

type Verdict string

const (
	VerdictPass Verdict = "PASS"
	VerdictWarn Verdict = "WARN"
	VerdictFail Verdict = "FAIL"

	DefaultVerdict = VerdictFail
)

// EffectiveVerdict is the canonical verdict read for loaded packets. Missing
// default_verdict resolves fail-closed instead of relying on schema defaults.
func (p ExecutionPacket) EffectiveVerdict() Verdict {
	switch p.DefaultVerdict {
	case VerdictPass, VerdictWarn, VerdictFail:
		return p.DefaultVerdict
	default:
		// Absent OR unrecognized/malformed -> fail-closed to FAIL. A loaded packet may
		// carry a junk default_verdict; never treat an unvalidated value as authoritative.
		return DefaultVerdict
	}
}

type Provenance struct {
	CreatedAt string `json:"created_at"` // RFC3339
	Source    string `json:"source"`     // e.g. "discovery"
	RunID     string `json:"run_id,omitempty"`
}
