package packet

import "errors"

var (
	ErrPlanPathEmpty     = errors.New("packet: plan_path is empty")
	ErrInvalidComplexity = errors.New("packet: complexity must be fast|standard|full")
	ErrInvalidTestLevel  = errors.New("packet: test_levels contains invalid value")
	ErrEmptyTestLevels   = errors.New("packet: test_levels must be non-empty")
	ErrEmptyProvenance   = errors.New("packet: provenance.created_at and source required")
)

// Validate returns the first invariant violation, or nil.
// Invariants:
//
//	I1. plan_path is non-empty
//	I2. complexity ∈ {fast, standard, full}
//	I3. test_levels is non-empty AND every entry ∈ {L0,L1,L2,L3}
//	I4. provenance.created_at and provenance.source are non-empty
//
// (epic_id may be empty in tasklist mode — not an invariant)
func (p ExecutionPacket) Validate() error {
	if p.PlanPath == "" {
		return ErrPlanPathEmpty
	}
	switch p.Complexity {
	case ComplexityFast, ComplexityStandard, ComplexityFull:
	default:
		return ErrInvalidComplexity
	}
	if len(p.TestLevels) == 0 {
		return ErrEmptyTestLevels
	}
	for _, l := range p.TestLevels {
		switch l {
		case L0, L1, L2, L3:
		default:
			return ErrInvalidTestLevel
		}
	}
	if p.Provenance.CreatedAt == "" || p.Provenance.Source == "" {
		return ErrEmptyProvenance
	}
	return nil
}
