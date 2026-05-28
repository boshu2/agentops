package packet

import (
	"testing"

	"pgregory.net/rapid"
)

// validBase returns an ExecutionPacket that passes Validate().
// Tests mutate one field at a time to exercise a single invariant.
func validBase() ExecutionPacket {
	return ExecutionPacket{
		PlanPath:   ".agents/plans/x.md",
		Complexity: ComplexityStandard,
		TestLevels: []TestLevel{L1, L2},
		Provenance: Provenance{CreatedAt: "2026-05-12T00:00:00Z", Source: "discovery"},
	}
}

// I1 — plan_path non-empty (negative).
func TestExecutionPacket_ValidateRejectsEmptyPlanPath(t *testing.T) {
	p := validBase()
	p.PlanPath = ""
	if err := p.Validate(); err != ErrPlanPathEmpty {
		t.Fatalf("expected ErrPlanPathEmpty, got %v", err)
	}
}

// I1 — plan_path non-empty (positive, property).
func TestExecutionPacket_ValidateAcceptsNonEmptyPlanPath(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		path := rapid.StringMatching(`[a-zA-Z0-9_./-]{1,40}`).Draw(t, "plan-path")
		p := validBase()
		p.PlanPath = path
		if err := p.Validate(); err != nil {
			t.Fatalf("expected nil, got %v (packet=%+v)", err, p)
		}
	})
}

// I2 — complexity ∈ {fast, standard, full} (negative, property).
func TestExecutionPacket_ValidateRejectsInvalidComplexity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bad := rapid.StringMatching(`[a-z]{1,10}`).
			Filter(func(s string) bool {
				return s != "fast" && s != "standard" && s != "full"
			}).Draw(t, "bad-complexity")
		p := validBase()
		p.Complexity = Complexity(bad)
		if err := p.Validate(); err != ErrInvalidComplexity {
			t.Fatalf("expected ErrInvalidComplexity, got %v", err)
		}
	})
}

// I3 — test_levels every entry ∈ {L0,L1,L2,L3} (negative, property).
func TestExecutionPacket_ValidateRejectsInvalidTestLevel(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bad := rapid.StringMatching(`L[4-9]|X[0-9]`).Draw(t, "bad-level")
		p := validBase()
		p.TestLevels = []TestLevel{TestLevel(bad)}
		if err := p.Validate(); err != ErrInvalidTestLevel {
			t.Fatalf("expected ErrInvalidTestLevel, got %v", err)
		}
	})
}

// I3 — test_levels non-empty (negative).
func TestExecutionPacket_ValidateRejectsEmptyTestLevels(t *testing.T) {
	p := validBase()
	p.TestLevels = nil
	if err := p.Validate(); err != ErrEmptyTestLevels {
		t.Fatalf("expected ErrEmptyTestLevels, got %v", err)
	}
}

// I4 — provenance.created_at non-empty (negative).
func TestExecutionPacket_ValidateRejectsEmptyProvenanceCreatedAt(t *testing.T) {
	p := validBase()
	p.Provenance.CreatedAt = ""
	if err := p.Validate(); err != ErrEmptyProvenance {
		t.Fatalf("expected ErrEmptyProvenance, got %v", err)
	}
}

// I4 — provenance.source non-empty (negative).
func TestExecutionPacket_ValidateRejectsEmptyProvenanceSource(t *testing.T) {
	p := validBase()
	p.Provenance.Source = ""
	if err := p.Validate(); err != ErrEmptyProvenance {
		t.Fatalf("expected ErrEmptyProvenance, got %v", err)
	}
}

// Composite positive — exercises I2 and I3 positive case across
// all valid Complexity × TestLevels combinations.
// Naming carries "Property" so `go test -run Property` selects at least
// one rapid-based test (acceptance gate #4).
func TestExecutionPacket_ValidateAcceptsAllValidCombinations_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		complexities := []Complexity{ComplexityFast, ComplexityStandard, ComplexityFull}
		levels := []TestLevel{L0, L1, L2, L3}
		p := validBase()
		p.Complexity = complexities[rapid.IntRange(0, 2).Draw(t, "ci")]
		n := rapid.IntRange(1, 4).Draw(t, "nlevels")
		p.TestLevels = nil
		for i := 0; i < n; i++ {
			p.TestLevels = append(p.TestLevels, levels[rapid.IntRange(0, 3).Draw(t, "li")])
		}
		if err := p.Validate(); err != nil {
			t.Fatalf("expected nil, got %v (packet=%+v)", err, p)
		}
	})
}
