// Package packet defines the ExecutionPacket aggregate root.
// This is the linked-intent object that carries discovery output
// through the RPI pipeline. See PRACTICE.md lines 169-172.
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

type Provenance struct {
	CreatedAt string `json:"created_at"` // RFC3339
	Source    string `json:"source"`     // e.g. "discovery"
	RunID     string `json:"run_id,omitempty"`
}
