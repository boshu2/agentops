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

// Slice 1 (age-membrane-memory-j9c6.1): an escape must carry the DOMAIN it
// happened in (from the confirmed false-done) and what was MISSED (the reason on
// the refuting catch) — the two fields the slow loop needs to answer "what
// escaped in THIS domain, and why." Fixture built through the production writer.
func TestDetectEscapes_CarriesDomainAndMissed(t *testing.T) {
	root := t.TempDir()
	const run = "r-domain-test"
	w := Writer{}
	mustAppend := func(disposition, headSHA, domain, reason string, attempt int, fams []string) {
		t.Helper()
		if _, err := w.AppendGateVerdict(root, GateVerdictInput{
			BeadID:          "age-racy",
			RunID:           run,
			TS:              time.Date(2026, 6, 19, 12, attempt, 0, 0, time.UTC),
			Difficulty:      1,
			PawlVerdictRef:  PawlVerdictRef{BeadID: "age-racy", HeadSHA: headSHA},
			Disposition:     disposition,
			AuthorContextID: "ctx",
			AuthorFamily:    "claude",
			HeadSHA:         headSHA,
			Attempt:         attempt,
			RefuterFamilies: fams,
			Domain:          domain,
			Reason:          reason,
		}); err != nil {
			t.Fatalf("append %s a%d: %v", disposition, attempt, err)
		}
	}
	mustAppend(DispositionConfirmed, "aaaaaaa1", "concurrency", "", 1, nil)
	mustAppend(DispositionRefuted, "bbbbbbb2", "concurrency", "data race on a shared counter: read-modify-write without a lock", 2, []string{"codex"})

	l, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := DetectEscapes(l, run)
	if len(got) != 1 {
		t.Fatalf("want 1 escape, got %d: %+v", len(got), got)
	}
	if got[0].Domain != "concurrency" {
		t.Errorf("escape Domain = %q, want concurrency (from the confirmed verdict)", got[0].Domain)
	}
	if got[0].Missed != "data race on a shared counter: read-modify-write without a lock" {
		t.Errorf("escape Missed = %q, want the refuted reason", got[0].Missed)
	}
}

// Slice 2 (age-membrane-memory-j9c6.2): the slow loop's accumulated-memory
// query — every escape in a given domain, across ALL runs in the ledger. This
// is the "what has escaped here before" lookup that seeds a domain-scoped check.
func TestEscapesByDomain(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	emit := func(run, bead, disposition, headSHA, domain string, attempt int) {
		t.Helper()
		if _, err := w.AppendGateVerdict(root, GateVerdictInput{
			BeadID: bead, RunID: run,
			TS:              time.Date(2026, 6, 19, 13, attempt, 0, 0, time.UTC),
			Difficulty:      1,
			PawlVerdictRef:  PawlVerdictRef{BeadID: bead, HeadSHA: headSHA},
			Disposition:     disposition,
			AuthorContextID: "ctx", AuthorFamily: "claude",
			HeadSHA: headSHA, Attempt: attempt, Domain: domain,
		}); err != nil {
			t.Fatalf("emit %s %s/%s a%d: %v", run, bead, disposition, attempt, err)
		}
	}
	// run-a: a concurrency escape and a docs escape.
	emit("run-a", "age-x", DispositionConfirmed, "aaaaaa11", "concurrency", 1)
	emit("run-a", "age-x", DispositionRefuted, "aaaaaa22", "concurrency", 2)
	emit("run-a", "age-y", DispositionConfirmed, "yyyyyy11", "docs", 1)
	emit("run-a", "age-y", DispositionRefuted, "yyyyyy22", "docs", 2)
	// run-b: another concurrency escape.
	emit("run-b", "age-z", DispositionConfirmed, "zzzzzz11", "concurrency", 1)
	emit("run-b", "age-z", DispositionRefuted, "zzzzzz22", "concurrency", 2)

	l, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	conc := EscapesByDomain(l, "concurrency")
	if len(conc) != 2 {
		t.Fatalf("EscapesByDomain(concurrency) = %d, want 2 (age-x run-a, age-z run-b): %+v", len(conc), conc)
	}
	for _, e := range conc {
		if e.Domain != "concurrency" {
			t.Errorf("escape %s domain = %q, want concurrency", e.BeadID, e.Domain)
		}
		if e.BeadID == "age-y" {
			t.Errorf("docs escape age-y leaked into the concurrency query")
		}
	}
	if got := EscapesByDomain(l, "docs"); len(got) != 1 || got[0].BeadID != "age-y" {
		t.Errorf("EscapesByDomain(docs) = %+v, want [age-y]", got)
	}
	if got := EscapesByDomain(l, "nonexistent"); len(got) != 0 {
		t.Errorf("EscapesByDomain(nonexistent) = %+v, want empty", got)
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
