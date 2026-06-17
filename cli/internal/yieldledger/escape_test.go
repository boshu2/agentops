package yieldledger

import (
	"testing"
	"time"
)

// gv appends a gate-verdict for bead at attempt with the given disposition,
// building the fixture through the PRODUCTION writer (round-trip fidelity per
// the test-pyramid Fixture Fidelity rule) rather than a hand-built event.
func gv(t *testing.T, root, run, bead, disposition, headSHA string, attempt int, families []string) {
	t.Helper()
	w := Writer{}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID:          bead,
		RunID:           run,
		TS:              time.Date(2026, 6, 17, 12, attempt, 0, 0, time.UTC),
		Difficulty:      1,
		PawlVerdictRef:  PawlVerdictRef{BeadID: bead, HeadSHA: headSHA},
		Disposition:     disposition,
		AuthorContextID: "ctx-" + bead,
		AuthorFamily:    "claude",
		HeadSHA:         headSHA,
		Attempt:         attempt,
		RefuterFamilies: families,
	}); err != nil {
		t.Fatalf("append %s %s/a%d: %v", disposition, bead, attempt, err)
	}
}

func TestDetectEscapes_ConfirmedThenRefuted(t *testing.T) {
	root := t.TempDir()
	const run = "r-escape-test"

	// age-zqc escape: CONFIRMED at attempt 1, then REFUTED at attempt 2 — the
	// membrane let a false-done through and a later, harder pass caught it.
	gv(t, root, run, "age-escapee", DispositionConfirmed, "aaaaaaa1", 1, nil)
	gv(t, root, run, "age-escapee", DispositionRefuted, "bbbbbbb2", 2, []string{"gemini", "codex"})

	// Control: a clean first-pass CONFIRMED that is never refuted — NOT an escape.
	gv(t, root, run, "age-clean", DispositionConfirmed, "ccccccc1", 1, nil)

	// Control: REFUTED-then-CONFIRMED (normal rework, the membrane working) — the
	// confirm comes AFTER the refute, so there is no confirmed-then-refuted miss.
	gv(t, root, run, "age-rework", DispositionRefuted, "ddddddd1", 1, nil)
	gv(t, root, run, "age-rework", DispositionConfirmed, "eeeeeee2", 2, nil)

	l, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := DetectEscapes(l, run)
	if len(got) != 1 {
		t.Fatalf("DetectEscapes returned %d escapes, want 1: %+v", len(got), got)
	}
	e := got[0]
	if e.BeadID != "age-escapee" {
		t.Errorf("escape bead = %q, want age-escapee", e.BeadID)
	}
	if e.ConfirmedAttempt != 1 || e.ConfirmedHeadSHA != "aaaaaaa1" {
		t.Errorf("confirmed = a%d/%s, want a1/aaaaaaa1", e.ConfirmedAttempt, e.ConfirmedHeadSHA)
	}
	if e.RefutedAttempt != 2 || e.RefutedHeadSHA != "bbbbbbb2" {
		t.Errorf("refuted = a%d/%s, want a2/bbbbbbb2", e.RefutedAttempt, e.RefutedHeadSHA)
	}
	if len(e.RefuterFamilies) != 2 || e.RefuterFamilies[0] != "gemini" {
		t.Errorf("refuter families = %v, want [gemini codex]", e.RefuterFamilies)
	}
}

func TestDetectEscapes_EmptyAndNil(t *testing.T) {
	if got := DetectEscapes(nil, "r"); got != nil {
		t.Errorf("DetectEscapes(nil) = %v, want nil", got)
	}
	if got := DetectEscapes(&Ledger{}, "r"); got != nil {
		t.Errorf("DetectEscapes(empty) = %v, want nil", got)
	}
}

func TestDetectEscapes_RunScoped(t *testing.T) {
	root := t.TempDir()
	// Escape lives in run A; querying run B must not surface it.
	gv(t, root, "run-a", "age-x", DispositionConfirmed, "1111111a", 1, nil)
	gv(t, root, "run-a", "age-x", DispositionRefuted, "2222222b", 2, nil)

	l, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := DetectEscapes(l, "run-b"); len(got) != 0 {
		t.Errorf("DetectEscapes(run-b) = %v, want none (escape is in run-a)", got)
	}
	if got := DetectEscapes(l, "run-a"); len(got) != 1 {
		t.Errorf("DetectEscapes(run-a) = %d escapes, want 1", len(got))
	}
}
