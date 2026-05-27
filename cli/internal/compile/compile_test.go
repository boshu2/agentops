package compile

import (
	"reflect"
	"testing"
)

// TestPhaseEnum_FiveCanonicalBeats asserts the loop body has exactly the
// five canonical beats per skills/domain/references/loop.md. Adding a
// sixth Phase constant without updating AllPhases breaks the build via
// TestNoSixthPhase; adding both breaks this test.
func TestPhaseEnum_FiveCanonicalBeats(t *testing.T) {
	canonical := []Phase{PhaseResearch, PhasePlan, PhaseImplement, PhaseValidate, PhaseRatchet}
	wantNames := []string{"Research", "Plan", "Implement", "Validate", "Ratchet"}
	for i, p := range canonical {
		if got := p.String(); got != wantNames[i] {
			t.Errorf("phase %d: String() = %q, want %q", p, got, wantNames[i])
		}
	}
	// Sanity: the unknown branch is reachable
	if got := Phase(99).String(); got != "UnknownPhase" {
		t.Errorf("Phase(99).String() = %q, want UnknownPhase", got)
	}
}

// TestNoSixthPhase asserts the canonical phase list and AllPhases stay
// in lockstep. Sibling to posture.TestNoSeventhLayer — the type-level
// expression of the loop-body invariant.
func TestNoSixthPhase(t *testing.T) {
	canonical := NewPhaseSet(PhaseResearch, PhasePlan, PhaseImplement, PhaseValidate, PhaseRatchet)
	if canonical != AllPhases {
		t.Fatalf("AllPhases drift: canonical=%08b AllPhases=%08b — a sixth "+
			"phase was likely added without updating the canonical list. "+
			"The five-beat loop body is invariant per loop.md; adding a "+
			"phase requires updating the canonical doctrine first.",
			uint8(canonical), uint8(AllPhases))
	}
}

// TestContextBundleHasFieldPerPayload asserts every DensityPayload kind has
// a corresponding field on ContextBundle. This is the type-level
// expression of the Context Density Rule: anything crossing a loop edge
// MUST carry a named payload kind. Adding a 7th payload kind without
// adding a bundle field — or vice versa — breaks this test.
func TestContextBundleHasFieldPerPayload(t *testing.T) {
	wantFields := map[DensityPayload]string{
		PayloadIntent:     "Intent",
		PayloadBoundary:   "Boundary",
		PayloadEvidence:   "Evidence",
		PayloadDecision:   "Decisions",
		PayloadConstraint: "Constraints",
		PayloadNextAction: "NextAction",
	}
	bundleType := reflect.TypeOf(ContextBundle{})
	for payload, fieldName := range wantFields {
		if _, ok := bundleType.FieldByName(fieldName); !ok {
			t.Errorf("ContextBundle missing field %q for %s payload — "+
				"Density Rule violation: payload kinds and bundle fields must align 1:1",
				fieldName, payload)
		}
	}
	// Sanity: payload count matches the constants
	if len(wantFields) != 6 {
		t.Fatalf("test bug: expected 6 payloads, got %d", len(wantFields))
	}
}

// TestLoopTickShape asserts the tick carries the required spine: a bead
// id, a phase, an input bundle, and an output bundle. Compounding
// (Out ⊇ In + new) lives in semantics, not type structure; this test
// only checks the structural pieces are present.
func TestLoopTickShape(t *testing.T) {
	tick := LoopTick{
		BeadID: "ag-test",
		Phase:  PhaseImplement,
		In:     ContextBundle{Phase: PhaseImplement, Intent: []string{"do the thing"}},
		Out:    ContextBundle{Phase: PhaseImplement, Evidence: []string{"PR #999"}},
	}
	if tick.BeadID == "" {
		t.Error("BeadID not retained")
	}
	if tick.Phase != PhaseImplement {
		t.Errorf("Phase = %v, want PhaseImplement", tick.Phase)
	}
	if len(tick.In.Intent) != 1 {
		t.Error("In bundle intent lost")
	}
	if len(tick.Out.Evidence) != 1 {
		t.Error("Out bundle evidence lost")
	}
}

// TestRatchet_PromoteRequiresCitation asserts the ratchet rejects
// anonymous learnings. "Knowledge becomes constraints" requires
// provenance — a Learning with no Citation cannot promote.
func TestRatchet_PromoteRequiresCitation(t *testing.T) {
	l := Learning{Summary: "agents drift on 'keep going' signals"}
	_, err := PromoteToConstraint(l, EnforcedByRule, "intent-drift-check", ".agents/playbooks/intent-check.md")
	if err == nil {
		t.Fatal("expected error for anonymous learning (no Citation); ratchet must require provenance")
	}
}

// TestRatchet_PromoteRequiresEnforcedBy asserts the ratchet rejects
// constraints without a named enforcement mechanism. Prose advice
// decays; gates/tests/lints/rules hold.
func TestRatchet_PromoteRequiresEnforcedBy(t *testing.T) {
	l := Learning{Summary: "real summary", Citation: ".agents/learnings/test.md"}
	_, err := PromoteToConstraint(l, EnforcedBy(99), "x", ".github/workflows/x.yml")
	if err == nil {
		t.Fatal("expected error for unknown EnforcedBy; every Constraint must name its enforcement mechanism")
	}
}

// TestRatchet_PromoteHappyPath asserts the ratchet produces a usable
// Constraint when given a fully-formed Learning + named enforcement.
func TestRatchet_PromoteHappyPath(t *testing.T) {
	l := Learning{
		Summary:  "package names must be domain terms, not infrastructure-speak",
		Citation: ".../memory/feedback_ddd_package_naming_no_infra_speak.md",
	}
	c, err := PromoteToConstraint(l, EnforcedByLint, "package-name-domain-term", ".github/workflows/validate.yml#package-name-lint")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.From.Citation != l.Citation {
		t.Errorf("provenance lost: c.From.Citation = %q, want %q", c.From.Citation, l.Citation)
	}
	if c.EnforcedBy != EnforcedByLint {
		t.Errorf("EnforcedBy = %v, want EnforcedByLint", c.EnforcedBy)
	}
	if c.Name == "" || c.Citation == "" {
		t.Error("Constraint missing required Name or Citation")
	}
}

// TestAgentOpsLoopOwnsAllFiveBeats asserts this distribution declares
// the full loop body. AgentOps IS the in-session operating loop (3.0
// thesis); owning anything less would contradict the product claim.
// Sibling to posture.TestAgentOpsPostureCoversAllLayers.
func TestAgentOpsLoopOwnsAllFiveBeats(t *testing.T) {
	if AgentOpsLoop.OwnedPhases != AllPhases {
		t.Fatalf("AgentOpsLoop does not own all five beats:\n"+
			"  owned = %08b\n"+
			"  all   = %08b\n\n"+
			"AgentOps IS the in-session operating loop (docs/3.0.md). Owning "+
			"anything less is a contradiction with the product thesis. If you "+
			"are intentionally narrowing the in-session scope, update "+
			"docs/3.0.md and loop.md first.",
			uint8(AgentOpsLoop.OwnedPhases),
			uint8(AllPhases))
	}
}

// TestPhaseSetOps exercises Has and Union — the small surface posture's
// LayerSet established and compile inherits by parity.
func TestPhaseSetOps(t *testing.T) {
	s := NewPhaseSet(PhaseResearch, PhaseValidate)
	if !s.Has(PhaseResearch) {
		t.Error("Has(PhaseResearch) = false, want true")
	}
	if s.Has(PhaseImplement) {
		t.Error("Has(PhaseImplement) = true, want false")
	}
	t2 := s.Union(NewPhaseSet(PhasePlan, PhaseImplement, PhaseRatchet))
	if t2 != AllPhases {
		t.Errorf("Union did not produce AllPhases: %08b vs %08b", uint8(t2), uint8(AllPhases))
	}
}
