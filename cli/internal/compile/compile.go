// Package compile types the operational flow of context through one tick
// of the AgentOps loop. It is the sibling of cli/internal/posture/:
// posture is the structural-ownership axis, compile is the operational-
// flow axis. They model the same six-layer agent stack from two angles.
//
// Compile names the noun behind the product thesis: "AgentOps is the
// in-session agent operating loop and the context compiler that feeds
// it" (docs/3.0.md, skills/domain/references/context-compiler.md).
// The capability turns the .agents/ corpus into the working set a loop
// tick needs, and absorbs the tick's exhaust back into the corpus.
//
// MINIMAL by design — additions require citation in
// docs/architecture/operating-loop.md, skills/domain/references/loop.md,
// or skills/domain/references/context-compiler.md.
//
// Bounded context: BC1 Corpus. Persistence to .agents/ JSONL files is
// the job of a separate adapter package — these types describe shape,
// not IO (hexagonal: domain knows nothing about its persistence).
package compile

import "fmt"

// Phase names one of the five canonical beats in a loop tick. The
// five-beat shape (research → plan → implement → validate → ratchet)
// is the invariant loop body per skills/domain/references/loop.md;
// drivers (rpi, evolve, factory) differ in scale but not in shape.
type Phase uint8

const (
	PhaseResearch  Phase = 1
	PhasePlan      Phase = 2
	PhaseImplement Phase = 3
	PhaseValidate  Phase = 4
	PhaseRatchet   Phase = 5
)

var phaseNames = [...]string{"", "Research", "Plan", "Implement", "Validate", "Ratchet"}

func (p Phase) String() string {
	if int(p) < len(phaseNames) {
		return phaseNames[p]
	}
	return "UnknownPhase"
}

// PhaseSet is a compact bit-set over the five canonical phases.
// Mirrors posture.LayerSet for the operational axis.
type PhaseSet uint8

// NewPhaseSet builds a PhaseSet from zero or more phases.
func NewPhaseSet(phases ...Phase) PhaseSet {
	var s PhaseSet
	for _, p := range phases {
		s |= 1 << p
	}
	return s
}

// Has reports whether the set contains the given phase.
func (s PhaseSet) Has(p Phase) bool { return s&(1<<p) != 0 }

// Union returns the set-theoretic union of s and other.
func (s PhaseSet) Union(other PhaseSet) PhaseSet { return s | other }

// AllPhases is the universe of recognized phases. Invariant per loop.md:
// any in-session loop runs exactly these five beats.
var AllPhases = NewPhaseSet(PhaseResearch, PhasePlan, PhaseImplement, PhaseValidate, PhaseRatchet)

// DensityPayload names one of the six allowed payloads at a loop edge per
// the Context Density Rule (skills/domain/references/context-density-rule.md).
// Every high-value token crossing a loop edge MUST carry one of these.
type DensityPayload uint8

const (
	PayloadIntent     DensityPayload = 1
	PayloadBoundary   DensityPayload = 2
	PayloadEvidence   DensityPayload = 3
	PayloadDecision   DensityPayload = 4
	PayloadConstraint DensityPayload = 5
	PayloadNextAction DensityPayload = 6
)

var payloadNames = [...]string{"", "Intent", "Boundary", "Evidence", "Decision", "Constraint", "NextAction"}

func (d DensityPayload) String() string {
	if int(d) < len(payloadNames) {
		return payloadNames[d]
	}
	return "UnknownPayload"
}

// ContextBundle is the artifact handed off at every loop edge. Context
// is the engineering artifact, not a byproduct (context-compiler.md).
//
// Fields are typed by their Density Rule payload classification, so
// anything crossing a loop edge has a named role: intent, boundary,
// evidence, decision, constraint, or next action. Each field aligns
// 1:1 with a DensityPayload constant — that alignment is checked by
// the test TestContextBundleHasFieldPerPayload.
type ContextBundle struct {
	Phase       Phase    // which beat this bundle is for
	Intent      []string // PayloadIntent — what the next step is for
	Boundary    []string // PayloadBoundary — bounded-context / scope
	Evidence    []string // PayloadEvidence — citations, links, SHAs
	Decisions   []string // PayloadDecision — chosen tradeoffs, ADRs
	Constraints []string // PayloadConstraint — gates, rules, invariants
	NextAction  string   // PayloadNextAction — one concrete next step
}

// LoopTick models one cycle of work over one bead. The In bundle is
// what the tick was handed at start; the Out bundle is what the tick
// produces and hands forward. Compounding rule: a healthy tick's Out
// is a superset of its In plus newly compiled evidence/decisions/
// constraints — the corpus moat grows monotonically.
type LoopTick struct {
	BeadID string
	Phase  Phase
	In     ContextBundle
	Out    ContextBundle
}

// Learning is the raw form of an insight captured during a tick. Lives
// in .agents/learnings/ until the ratchet promotes (or evicts) it.
// Citation is REQUIRED — anonymous insights cannot ratchet.
type Learning struct {
	Summary  string
	Citation string // e.g. ".agents/learnings/2026-05-26-some-slug.md"
}

// EnforcedBy names the mechanism through which a Constraint holds.
// The ratchet rule "knowledge becomes constraints" requires every
// Constraint to declare its enforcement — prose advice decays, only
// gates/tests/lints/rules hold (operating-loop.md, 3.0.md).
type EnforcedBy uint8

const (
	EnforcedByGate EnforcedBy = 1 // CI gate
	EnforcedByTest EnforcedBy = 2 // integration test
	EnforcedByLint EnforcedBy = 3 // lint rule
	EnforcedByRule EnforcedBy = 4 // skill rule (canonical doctrine prose)
)

var enforcedByNames = [...]string{"", "Gate", "Test", "Lint", "Rule"}

func (e EnforcedBy) String() string {
	if int(e) < len(enforcedByNames) {
		return enforcedByNames[e]
	}
	return "UnknownEnforcement"
}

// Constraint is the durable form of a Learning, with its enforcement
// mechanism named. Constraints, not prose, are what compound the corpus.
type Constraint struct {
	Name       string
	From       Learning
	EnforcedBy EnforcedBy
	Citation   string // pointer to the gate/test/lint/rule that enforces this
}

// PromoteToConstraint expresses the ratchet rule as a typed function:
// a Learning becomes a Constraint only when (a) the source Learning
// carries a citation (anonymous insights cannot ratchet), and (b) the
// enforcement mechanism is named (a Constraint without EnforcedBy is
// not durable). Either failure returns an error and no Constraint is
// produced — the type-level expression of "knowledge becomes constraints,
// not prose".
func PromoteToConstraint(l Learning, by EnforcedBy, name, citation string) (Constraint, error) {
	if l.Citation == "" {
		return Constraint{}, fmt.Errorf("learning has no citation; ratchet rejects anonymous insights (operating-loop.md: knowledge becomes constraints requires provenance)")
	}
	if by < EnforcedByGate || by > EnforcedByRule {
		return Constraint{}, fmt.Errorf("unknown EnforcedBy %d; every Constraint must name its enforcement mechanism (Gate/Test/Lint/Rule)", by)
	}
	if name == "" {
		return Constraint{}, fmt.Errorf("Constraint requires a name; unnamed constraints cannot be cited")
	}
	if citation == "" {
		return Constraint{}, fmt.Errorf("Constraint requires a citation pointer to its enforcing artifact")
	}
	return Constraint{Name: name, From: l, EnforcedBy: by, Citation: citation}, nil
}

// LoopDeclaration is what a project claims about which Phases of the
// loop body it runs in-session. Sibling to posture.Stance: posture says
// WHICH LAYERS we own; LoopDeclaration says WHICH PHASES we run.
type LoopDeclaration struct {
	OwnedPhases PhaseSet
}

// AgentOpsLoop is what this distribution claims about its in-session
// loop. AgentOps owns all five beats — it IS the in-session operating
// loop (the 3.0 thesis). The Factory driver (out-of-session, substrate-
// owned) is a separate concern; its phases are still these five, but
// driven by an out-of-session substrate rather than the in-session agent. Source:
// skills/domain/references/loop.md ("two drivers, one inner tick").
var AgentOpsLoop = LoopDeclaration{
	OwnedPhases: NewPhaseSet(PhaseResearch, PhasePlan, PhaseImplement, PhaseValidate, PhaseRatchet),
}
