// Package evidencedturn computes the legible Definition-of-Done predicate for
// an "Evidenced Turn" (ag-lmdx.5) — the parent epic's unit of work.
//
// An Evidenced Turn is the assurance contract that lets a bead legally
// transition validated->closed. Per ag-lmdx.5, turn-done is a 5-piece puzzle
// (bead close + Evidence trailer + ledger + next-work flag + CI) and the live
// AP#7 gate only checks Evidence *presence*, never *sufficiency*. This package
// rolls those pieces into ONE predicate — "can this bead legally transition
// validated->closed?" — and, crucially, makes the verdict LEGIBLE: a clear
// pass/fail per predicate with a human-readable reason for every gap.
//
// It builds directly on the ag-lmdx substrate that already landed:
//
//   - cli/internal/turnstate (#654): the append-only, hash-chained
//     state_transition log whose Fold is the artifact's lifecycle state. We
//     reuse FoldVerified (hash-chain integrity + ordered replay) so the
//     terminal-state predicate is true by construction, not by trusting a
//     mutable column.
//   - cli/internal/provenancegraph (ag-x31t): the provenance edge ledger and
//     the orphan finder. We reuse those to assert a provenance event exists
//     for the bead and that nothing the bead produced is an orphan.
//
// This is a pure, in-memory predicate core (no Dolt, no file I/O, no bd shell
// out): it is the testable kernel that defines what "done" MEANS. The cmd
// layer (cli/cmd/ao/turn_verify.go) wires real inputs into it.
//
// Extension point: ag-lmdx.4 (the author!=validator guard) is a SEPARATE
// concern and is deliberately NOT implemented here. A follow-up can add a
// PredicateAuthorNeqValidator row without reshaping the verdict; see the TODO
// on Evaluate.
package evidencedturn

import (
	"fmt"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
	"github.com/boshu2/agentops/cli/internal/turnstate"
)

// SchemaVersion stamps the verdict so a serialized DoD report is self-describing.
const SchemaVersion = "agentops-evidenced-turn.v1"

// terminalStates are the lifecycle states from which validated->closed is the
// legal next move, i.e. the states an Evidenced Turn must have folded into.
// "validated" is the pre-close state; "closed" is the post-close state. A bead
// folded into either is "at or past the done boundary"; anything earlier
// (e.g. "in_progress") is not done.
var terminalStates = map[string]bool{
	"validated": true,
	"closed":    true,
}

// Predicate names — the legible DoD checklist. Each appears in every verdict so
// the report shape is stable (a missing predicate never silently drops a row).
const (
	// PredicateChainIntact: the bead's state_transition log hash-verifies and
	// folds without a gap (turnstate.FoldVerified). Tamper or out-of-order
	// replay fails this first.
	PredicateChainIntact = "chain_intact"
	// PredicateTerminalState: the folded lifecycle state is at/past the
	// validated->closed boundary.
	PredicateTerminalState = "terminal_state"
	// PredicateScenariosCovered: every Closes-scenario the turn claims has a
	// passing test (the sufficiency half AP#7 omits).
	PredicateScenariosCovered = "scenarios_covered"
	// PredicateEvidenceResolves: every Evidence line resolves to a gate log
	// that exercised THAT scenario (not merely "an Evidence trailer exists").
	PredicateEvidenceResolves = "evidence_resolves"
	// PredicateProvenanceEvent: at least one provenance edge in the ledger
	// references the bead (a witnessed event exists).
	PredicateProvenanceEvent = "provenance_event"
	// PredicateNoOrphan: no artifact the turn produced is a provenance orphan.
	PredicateNoOrphan = "no_orphan"
	// PredicateAuthorNeqValidator: the no-self-grading invariant (ag-lmdx.4).
	// The acceptance verdict was produced by a judge context distinct from the
	// author context (author_id != judge_id). A verdict graded by its own
	// author is autocorrelated; the independent-trust-domain check is the guard
	// on the evidenced->validated transition. The documented, default-OFF
	// --allow-self escape permits a self-graded verdict for the inline fallback.
	PredicateAuthorNeqValidator = "author_neq_validator"
)

// orderedPredicates is the canonical report order so JSON/text output is stable
// across runs regardless of map iteration.
var orderedPredicates = []string{
	PredicateChainIntact,
	PredicateTerminalState,
	PredicateScenariosCovered,
	PredicateEvidenceResolves,
	PredicateProvenanceEvent,
	PredicateNoOrphan,
	PredicateAuthorNeqValidator,
}

// Scenario is one Closes-scenario claim a turn makes, with the two sufficiency
// facts AP#7 does not check: whether a passing test covers it, and whether its
// Evidence line resolves to a gate log that exercised it.
type Scenario struct {
	// Slug is the scenario token (e.g. "ao-turn-verify" in
	// "Closes-scenario: ag-lmdx.5#ao-turn-verify").
	Slug string `json:"slug"`
	// HasPassingTest is true when a test exercising this scenario passes.
	HasPassingTest bool `json:"has_passing_test"`
	// EvidenceResolved is true when this scenario's Evidence line resolves to a
	// gate log that exercised THIS scenario (presence->sufficiency).
	EvidenceResolved bool `json:"evidence_resolved"`
}

// Input is the complete set of facts the DoD predicate folds over. Everything
// is supplied by the caller; this package shells out to nothing.
type Input struct {
	// BeadID is the turn's bead (e.g. "ag-lmdx.5"). Required.
	BeadID string
	// Transitions is the bead's append-only state_transition log (turnstate).
	// May contain multiple artifacts; only BeadID's transitions are folded.
	Transitions []turnstate.Transition
	// Scenarios are the turn's Closes-scenario claims with coverage facts.
	Scenarios []Scenario
	// ProvenanceEdges is the provenance ledger (provenancegraph.Edge). A
	// provenance event "references the bead" when from_id or to_id == BeadID.
	ProvenanceEdges []provenancegraph.Edge
	// OrphanFindings are provenance orphans detected for the bead's produced
	// artifacts (provenancegraph.FindOrphans output). Empty == no orphan.
	OrphanFindings []provenancegraph.OrphanFinding
	// OrphanChecked records whether an orphan audit actually ran. When false
	// (e.g. no trace-graph was supplied), no_orphan fails as "not-yet-checked"
	// rather than passing vacuously — a turn is not provably done if its
	// orphan status was never audited.
	OrphanChecked bool
	// AuthorID is the identity of the session/context that AUTHORED the
	// artifact under verdict (the "who built it"). Empty means unknown.
	AuthorID string
	// JudgeID is the identity of the session/context that PRODUCED the
	// acceptance verdict (the "who graded it"). For the no-self-grading
	// invariant the judge must be a context distinct from the author — a
	// blind sub-agent context, not the same session. Empty means no
	// independent judge identity was recorded.
	JudgeID string
	// AllowSelf is the documented, default-OFF escape hatch that permits a
	// self-graded verdict (JudgeID == AuthorID). It exists for the inline
	// fallback path only; the default requires an independent judge.
	AllowSelf bool
}

// PredicateResult is one row of the legible DoD checklist.
type PredicateResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	// Reason is human-readable: WHY it passed, or precisely WHAT gap fails it.
	Reason string `json:"reason"`
}

// Verdict is the rolled-up DoD result: the single done/not-done answer plus the
// per-predicate breakdown that makes it legible.
type Verdict struct {
	SchemaVersion string            `json:"schema_version"`
	BeadID        string            `json:"bead_id"`
	Done          bool              `json:"done"`
	Predicates    []PredicateResult `json:"predicates"`
	// Gaps is the ordered list of failing predicate reasons — the actionable
	// "what's missing" summary. Empty when Done.
	Gaps []string `json:"gaps"`
}

// Evaluate folds the Input into the Evidenced-Turn Verdict. The turn is Done
// iff EVERY predicate passes (validated->closed is legal). A missing input is a
// failing predicate with a legible reason, never a silent pass.
//
// The author!=validator guard (ag-lmdx.4) is one of those predicates: a verdict
// graded by its own author is autocorrelated, so an Evidenced Turn is not done
// unless an independent judge context produced the verdict (or --allow-self
// explicitly waives it for the inline fallback).
func Evaluate(in Input) (Verdict, error) {
	if strings.TrimSpace(in.BeadID) == "" {
		return Verdict{}, fmt.Errorf("bead_id is required")
	}

	results := map[string]PredicateResult{
		PredicateChainIntact:      evalChainIntact(in),
		PredicateTerminalState:    evalTerminalState(in),
		PredicateScenariosCovered: evalScenariosCovered(in),
		PredicateEvidenceResolves: evalEvidenceResolves(in),
		PredicateProvenanceEvent:    evalProvenanceEvent(in),
		PredicateNoOrphan:           evalNoOrphan(in),
		PredicateAuthorNeqValidator: evalAuthorNeqValidator(in),
	}

	v := Verdict{
		SchemaVersion: SchemaVersion,
		BeadID:        in.BeadID,
		Done:          true,
		Predicates:    make([]PredicateResult, 0, len(orderedPredicates)),
	}
	for _, name := range orderedPredicates {
		r := results[name]
		v.Predicates = append(v.Predicates, r)
		if !r.Passed {
			v.Done = false
			v.Gaps = append(v.Gaps, fmt.Sprintf("%s: %s", r.Name, r.Reason))
		}
	}
	return v, nil
}

// evalChainIntact verifies the bead's transition log hash-chains and folds.
func evalChainIntact(in Input) PredicateResult {
	r := PredicateResult{Name: PredicateChainIntact}
	if _, err := turnstate.FoldVerified(in.Transitions); err != nil {
		r.Reason = fmt.Sprintf("state_transition log does not verify/fold: %v", err)
		return r
	}
	r.Passed = true
	r.Reason = "state_transition log hash-verifies and folds cleanly"
	return r
}

// evalTerminalState requires the bead's folded state to be at/past the
// validated->closed boundary. An absent bead (no transitions) is not done.
func evalTerminalState(in Input) PredicateResult {
	r := PredicateResult{Name: PredicateTerminalState}
	state, found, err := turnstate.StateOf(in.Transitions, in.BeadID)
	if err != nil {
		r.Reason = fmt.Sprintf("cannot fold state for %q: %v", in.BeadID, err)
		return r
	}
	if !found {
		r.Reason = fmt.Sprintf("bead %q has no state_transition log (never transitioned)", in.BeadID)
		return r
	}
	if !terminalStates[state] {
		r.Reason = fmt.Sprintf("folded state %q is not at the validated->closed boundary", state)
		return r
	}
	r.Passed = true
	r.Reason = fmt.Sprintf("folded state is %q (validated->closed is legal)", state)
	return r
}

// evalScenariosCovered requires at least one Closes-scenario, each with a
// passing test. It names the uncovered scenarios so the gap is actionable.
func evalScenariosCovered(in Input) PredicateResult {
	r := PredicateResult{Name: PredicateScenariosCovered}
	if len(in.Scenarios) == 0 {
		r.Reason = "no Closes-scenario claims (a turn must close at least one scenario)"
		return r
	}
	var uncovered []string
	for _, s := range in.Scenarios {
		if !s.HasPassingTest {
			uncovered = append(uncovered, s.Slug)
		}
	}
	if len(uncovered) > 0 {
		sort.Strings(uncovered)
		r.Reason = fmt.Sprintf("scenario(s) with no passing test: %s", strings.Join(uncovered, ", "))
		return r
	}
	r.Passed = true
	r.Reason = fmt.Sprintf("all %d scenario(s) have a passing test", len(in.Scenarios))
	return r
}

// evalEvidenceResolves requires every claimed scenario's Evidence line to
// resolve to a gate log that exercised THAT scenario (presence->sufficiency).
func evalEvidenceResolves(in Input) PredicateResult {
	r := PredicateResult{Name: PredicateEvidenceResolves}
	if len(in.Scenarios) == 0 {
		r.Reason = "no scenarios, so no Evidence to resolve"
		return r
	}
	var unresolved []string
	for _, s := range in.Scenarios {
		if !s.EvidenceResolved {
			unresolved = append(unresolved, s.Slug)
		}
	}
	if len(unresolved) > 0 {
		sort.Strings(unresolved)
		r.Reason = fmt.Sprintf("scenario(s) whose Evidence does not resolve to a gate log that exercised it: %s",
			strings.Join(unresolved, ", "))
		return r
	}
	r.Passed = true
	r.Reason = fmt.Sprintf("all %d scenario(s) have Evidence resolving to an exercising gate log", len(in.Scenarios))
	return r
}

// evalProvenanceEvent requires at least one provenance edge referencing the
// bead (as from_id or to_id) — a witnessed event exists.
func evalProvenanceEvent(in Input) PredicateResult {
	r := PredicateResult{Name: PredicateProvenanceEvent}
	for _, e := range in.ProvenanceEdges {
		if e.FromID == in.BeadID || e.ToID == in.BeadID {
			r.Passed = true
			r.Reason = fmt.Sprintf("provenance edge present (%s %s %s)", e.FromID, e.Relation, e.ToID)
			return r
		}
	}
	r.Reason = fmt.Sprintf("no provenance edge references bead %q", in.BeadID)
	return r
}

// evalNoOrphan requires no provenance orphan among the bead's produced
// artifacts. It names the orphans so the gap is actionable.
func evalNoOrphan(in Input) PredicateResult {
	r := PredicateResult{Name: PredicateNoOrphan}
	if !in.OrphanChecked {
		r.Reason = "orphan audit not run (supply a provenance trace-graph to check no_orphan)"
		return r
	}
	if len(in.OrphanFindings) > 0 {
		ids := make([]string, 0, len(in.OrphanFindings))
		for _, f := range in.OrphanFindings {
			ids = append(ids, f.OrphanArtifactID)
		}
		sort.Strings(ids)
		r.Reason = fmt.Sprintf("orphan artifact(s) with no inbound provenance edge: %s", strings.Join(ids, ", "))
		return r
	}
	r.Passed = true
	r.Reason = "no provenance orphans"
	return r
}

// evalAuthorNeqValidator enforces the no-self-grading invariant (ag-lmdx.4):
// the acceptance verdict must come from a judge context distinct from the
// author context. A self-graded verdict (author_id == judge_id) is
// autocorrelated and fails unless --allow-self explicitly waives it. A missing
// judge identity also fails: independence that was never recorded cannot be
// asserted.
func evalAuthorNeqValidator(in Input) PredicateResult {
	r := PredicateResult{Name: PredicateAuthorNeqValidator}
	author := strings.TrimSpace(in.AuthorID)
	judge := strings.TrimSpace(in.JudgeID)
	if in.AllowSelf {
		r.Passed = true
		r.Reason = "self-grading explicitly waived via --allow-self (inline fallback)"
		return r
	}
	if author == "" {
		r.Reason = "no author_id recorded (cannot prove the judge is independent of the author)"
		return r
	}
	if judge == "" {
		r.Reason = "no judge_id recorded (independent judge context required; use --allow-self to waive)"
		return r
	}
	if judge == author {
		r.Reason = fmt.Sprintf("verdict self-graded: judge_id == author_id (%q); an independent judge context is required (use --allow-self to waive)", author)
		return r
	}
	r.Passed = true
	r.Reason = fmt.Sprintf("independent judge: judge_id %q != author_id %q", judge, author)
	return r
}
