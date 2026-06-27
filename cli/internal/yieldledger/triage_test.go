package yieldledger

import (
	"testing"
	"time"
)

// (a) below the power floor -> INSUFFICIENT-DATA; (b) above the floor + full coverage,
// the axes straddle the thresholds to the exact decision branch. (S4)
func TestTriageDecide_FloorAndStraddle(t *testing.T) {
	// (a) classesWithStoredReason = 14 (< 15) -> INSUFFICIENT-DATA regardless of axes.
	if got := TriageDecide(20, 6, 14, 3, 3).Decision; got != DecisionInsufficientData {
		t.Errorf("below power floor must be INSUFFICIENT-DATA, got %s", got)
	}
	// (b) axis1 = 2/20 = 0.10 < 0.20 -> MEMORY-ONLY (coverage 2/2 = 1.0; floor met).
	if got := TriageDecide(20, 2, 20, 2, 0).Decision; got != DecisionMemoryOnly {
		t.Errorf("axis1=0.10 must be MEMORY-ONLY, got %s", got)
	}
	// (b) axis1 = 6/20 = 0.30, axis2 = 1/6 = 0.167 < 0.33 -> CURATED (coverage 6/6 = 1.0).
	if got := TriageDecide(20, 6, 20, 1, 5).Decision; got != DecisionCurated {
		t.Errorf("axis1=0.30 axis2<0.33 must be CURATED, got %s", got)
	}
	// (b) axis1 = 6/20 = 0.30, axis2 = 3/6 = 0.50 >= 0.33 -> GO (coverage 6/6 = 1.0).
	if got := TriageDecide(20, 6, 20, 3, 3).Decision; got != DecisionGo {
		t.Errorf("axis1=0.30 axis2>=0.33 must be GO, got %s", got)
	}
}

// (c) unassessed recurring classes are in the denominator AND block GO/CURATED: a small
// assessed subset cannot inflate the decision. (S4)
func TestTriageDecide_UnassessedCannotInflate(t *testing.T) {
	// 4 recurring, 1 compilable, 1 not_compilable, 2 unassessed -> coverage (1+1)/4 = 0.5.
	r := TriageDecide(20, 4, 20, 1, 1)
	if r.Unassessed != 2 {
		t.Fatalf("want 2 unassessed, got %d", r.Unassessed)
	}
	if r.Axis2Coverage >= 1.0 {
		t.Fatalf("coverage must be < 1.0 with unassessed classes, got %v", r.Axis2Coverage)
	}
	if r.Decision != DecisionInsufficientData {
		t.Errorf("an unassessed recurring class -> INSUFFICIENT-DATA, got %s", r.Decision)
	}
}

// (e) no-op detector (no TP) and (f) overfit detector (misses an instance) are both
// not_compilable; a detector that hits ALL instances + zero-FP is compilable. (S4)
func TestAssessCompilability_TPReplayAllInstances(t *testing.T) {
	// (e) no-op detector matches NOTHING -> fails TP on the first instance.
	if got := AssessCompilability("zzz-never", []string{"a bad line", "another bad line"}, "clean"); got != AssessNotCompilable {
		t.Errorf("no-op detector (no TP) must be not_compilable, got %s", got)
	}
	// (f) overfit detector hits instance 1 but MISSES instance 2.
	if got := AssessCompilability("foo", []string{"has foo here", "no match here"}, "clean"); got != AssessNotCompilable {
		t.Errorf("overfit detector (misses an instance) must be not_compilable, got %s", got)
	}
	// Sound: hits BOTH instances, NOT clean HEAD.
	if got := AssessCompilability("race", []string{"a race bug", "another race here"}, "clean code"); got != AssessCompilable {
		t.Errorf("detector hitting all instances + zero-FP must be compilable, got %s", got)
	}
	// Zero-FP violation: hits both instances AND clean HEAD.
	if got := AssessCompilability("bug", []string{"the bug one", "the bug two"}, "no bug here is clean... bug"); got != AssessNotCompilable {
		t.Errorf("detector matching clean HEAD (false positive) must be not_compilable, got %s", got)
	}
	// Degenerate: empty detector or no instances -> not_compilable.
	if got := AssessCompilability("", []string{"x"}, "y"); got != AssessNotCompilable {
		t.Errorf("empty detector must be not_compilable, got %s", got)
	}
	if got := AssessCompilability("x", nil, "y"); got != AssessNotCompilable {
		t.Errorf("no instances must be not_compilable, got %s", got)
	}
}

// (d) the no-fabrication rule: reason-less / domain-less REFUTEDs are the UNCLASSIFIED
// FLOOR — counted, never synthesized into a class; the real-shape corpus is below the
// floor -> INSUFFICIENT-DATA. (S4)
func TestTriageCorpus_NoFabricationFloorAndInsufficient(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	refute := func(bead, head, domain, reason string, attempt int) {
		t.Helper()
		if _, err := w.AppendGateVerdict(root, GateVerdictInput{
			BeadID: bead, RunID: "r", TS: time.Date(2026, 6, 27, 12, attempt, 0, 0, time.UTC),
			PawlVerdictRef: PawlVerdictRef{BeadID: bead, HeadSHA: head},
			Disposition:    DispositionRefuted, HeadSHA: head, Attempt: attempt,
			AuthorContextID: "c", AuthorFamily: "claude", RefuterFamilies: []string{"codex"},
			Domain: domain, Reason: reason,
		}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	refute("age-x", "aaaaaaa1", "", "", 1)                                // reason-less -> floor
	refute("age-y", "bbbbbbb2", "", "", 1)                                // reason-less -> floor
	refute("age-s", "ddddddd4", DomainUnclassified, ReasonUnspecified, 1) // SENTINEL-stamped -> floor, NOT a class
	refute("age-z", "ccccccc3", "pawl", "a real classified reason", 2)    // 1 real classified class

	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	r := TriageCorpus(l, func(Catch) string { return AssessUnassessed })
	if r.UnclassifiedFloor != 3 {
		t.Errorf("want UnclassifiedFloor=3 (2 reason-less + 1 sentinel-stamped), got %d", r.UnclassifiedFloor)
	}
	if r.DistinctClasses != 1 || r.ClassesWithStoredReason != 1 {
		t.Errorf("the floor must NOT be fabricated into classes; want 1 classified, got distinct=%d withReason=%d",
			r.DistinctClasses, r.ClassesWithStoredReason)
	}
	if r.Decision != DecisionInsufficientData {
		t.Errorf("a 1-class corpus is below the floor -> INSUFFICIENT-DATA, got %s", r.Decision)
	}
}

// The emit side must NOT persist a fabricated class_key on a sentinel-stamped row — the
// no-fabrication rule holds at the LEDGER CONTRACT, not just in triage counts. (S4, codex
// round-2: classKeyIfCatch previously used the old empty-only predicate.)
func TestClassKeyIfCatch_NoFabricatedKeyOnSentinelRow(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "age-s", RunID: "r", TS: time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
		PawlVerdictRef: PawlVerdictRef{BeadID: "age-s", HeadSHA: "ddddddd4"},
		Disposition:    DispositionRefuted, HeadSHA: "ddddddd4", Attempt: 1,
		AuthorContextID: "c", AuthorFamily: "claude", RefuterFamilies: []string{"codex"},
		Domain: DomainUnclassified, Reason: ReasonUnspecified,
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	l, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	saw := false
	for _, ev := range l.Events {
		if ev.GateVerdict == nil || ev.BeadID != "age-s" {
			continue
		}
		saw = true
		if ev.GateVerdict.ClassKey != "" {
			t.Fatalf("sentinel-stamped row must NOT persist a class_key, got %q", ev.GateVerdict.ClassKey)
		}
	}
	if !saw {
		t.Fatal("sentinel row not found in the ledger — the test isn't exercising the emit path")
	}
}
