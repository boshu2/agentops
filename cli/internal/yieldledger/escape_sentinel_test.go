package yieldledger

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStampEscapeSentinels_FailSafeOnLoadError(t *testing.T) {
	// A non-nil loadErr means a CORRUPT/unreadable existing ledger (missing loads
	// as empty per LoadPath). We cannot rule out a prior CONFIRMED -> fail SAFE.
	in := GateVerdictInput{BeadID: "ag-x", Disposition: DispositionRefuted, Attempt: 2}
	out, sub := StampEscapeSentinels(nil, errors.New("corrupt ledger"), in)
	if !sub {
		t.Fatal("fail-safe must report substituted")
	}
	if out.Domain != DomainUnclassified || out.Reason != ReasonUnspecified {
		t.Fatalf("fail-safe must stamp BOTH: domain=%q reason=%q", out.Domain, out.Reason)
	}
}

func TestStampEscapeSentinels_FailSafeSkipsNonRefuted(t *testing.T) {
	// A CONFIRMED (or any non-REFUTED) is never an escape — fail-safe must not stamp.
	in := GateVerdictInput{BeadID: "ag-x", Disposition: DispositionConfirmed, Attempt: 2}
	out, sub := StampEscapeSentinels(nil, errors.New("corrupt"), in)
	if sub || out.Domain != "" || out.Reason != "" {
		t.Fatalf("non-REFUTED must not be stamped on fail-safe: sub=%v domain=%q reason=%q", sub, out.Domain, out.Reason)
	}
}

func TestAppendGateVerdict_CorruptLedger_FailSafeStamps(t *testing.T) {
	// L2: a CORRUPT existing ledger makes LoadPath error; an escape must STILL be
	// recorded with domain+reason (the guarantee survives a degraded ledger).
	root := t.TempDir()
	path := LedgerPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not valid json at all\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPath(path); err == nil {
		t.Fatal("precondition: corrupt ledger must make LoadPath error")
	}
	w := Writer{}
	headSHA := "ag-x-h2"
	// The append's returned error is EXPECTED: appendValidated re-reads the ledger
	// after writing and that reload fails on the pre-existing corrupt line. The new
	// line is still written (and stamped, since StampEscapeSentinels runs first) —
	// which is exactly what we assert. Producers guard this emit with `|| true`.
	_, _ = w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-x", RunID: "run-1", TS: time.Date(2026, 6, 22, 13, 0, 2, 0, time.UTC),
		Difficulty: 1, PawlVerdictRef: PawlVerdictRef{BeadID: "ag-x", HeadSHA: headSHA},
		Disposition: DispositionRefuted, HeadSHA: headSHA, Attempt: 2,
		AuthorContextID: "ctx", AuthorFamily: "claude", // no domain, no reason
	})
	// Reload is impossible (still corrupt), so parse the appended last line raw.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	var ev Event
	if err := ev.UnmarshalJSON([]byte(lines[len(lines)-1])); err != nil {
		t.Fatalf("parse appended line: %v", err)
	}
	if ev.GateVerdict == nil || ev.GateVerdict.Domain != DomainUnclassified || ev.GateVerdict.Reason != ReasonUnspecified {
		t.Fatalf("escape on a corrupt ledger must be fail-safe stamped: %+v", ev.GateVerdict)
	}
}

// gv appends one gate-verdict through the real Writer (production path) so the
// sentinel logic is exercised exactly as it runs in prod (go.md fixture fidelity).
func gvDom(t *testing.T, root, run, bead, disposition, domain string, attempt int) *Ledger {
	t.Helper()
	w := Writer{}
	headSHA := bead + "-h" + string(rune('0'+attempt))
	led, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID:          bead,
		RunID:           run,
		TS:              time.Date(2026, 6, 22, 13, 0, attempt, 0, time.UTC),
		Difficulty:      1,
		PawlVerdictRef:  PawlVerdictRef{BeadID: bead, HeadSHA: headSHA},
		Disposition:     disposition,
		HeadSHA:         headSHA,
		Attempt:         attempt,
		AuthorContextID: "ctx-" + bead,
		AuthorFamily:    "claude",
		Domain:          domain,
	})
	if err != nil {
		t.Fatalf("append %s %s@%d: %v", disposition, bead, attempt, err)
	}
	return led
}

// storedDomain returns the Domain persisted on the bead's REFUTED gate-verdict at
// the given attempt — read back from disk so we assert the real stored shape.
func storedDomain(t *testing.T, root, bead string, attempt int) string {
	t.Helper()
	led, err := LoadPath(LedgerPath(root))
	if err != nil {
		t.Fatalf("reload ledger: %v", err)
	}
	for _, ev := range led.Events {
		if ev.Event == EventGateVerdict && ev.BeadID == bead && ev.GateVerdict != nil &&
			ev.GateVerdict.Disposition == DispositionRefuted && ev.GateVerdict.Attempt == attempt {
			return ev.GateVerdict.Domain
		}
	}
	t.Fatalf("no REFUTED gate-verdict for %s@%d found on disk", bead, attempt)
	return ""
}

// storedRefuted returns the bead's REFUTED gate-verdict body at the given attempt,
// read back from disk so we assert the real persisted shape (go.md fidelity).
func storedRefuted(t *testing.T, root, bead string, attempt int) *GateVerdictBody {
	t.Helper()
	led, err := LoadPath(LedgerPath(root))
	if err != nil {
		t.Fatalf("reload ledger: %v", err)
	}
	for _, ev := range led.Events {
		if ev.Event == EventGateVerdict && ev.BeadID == bead && ev.GateVerdict != nil &&
			ev.GateVerdict.Disposition == DispositionRefuted && ev.GateVerdict.Attempt == attempt {
			return ev.GateVerdict
		}
	}
	t.Fatalf("no REFUTED gate-verdict for %s@%d found on disk", bead, attempt)
	return nil
}

func TestAppendGateVerdict_OverturningRefuted_StampsBothSentinels(t *testing.T) {
	root := t.TempDir()
	gvDom(t, root, "run-1", "ag-x", DispositionConfirmed, "", 1)
	gvDom(t, root, "run-1", "ag-x", DispositionRefuted, "", 2) // overturn, no domain, no reason

	gv := storedRefuted(t, root, "ag-x", 2)
	if gv.Domain != DomainUnclassified {
		t.Fatalf("empty domain on escape: stored = %q, want %q (never recorded domain-less)", gv.Domain, DomainUnclassified)
	}
	if gv.Reason != ReasonUnspecified {
		t.Fatalf("empty reason on escape: stored = %q, want %q (never recorded reason-less)", gv.Reason, ReasonUnspecified)
	}
}

func TestAppendGateVerdict_OverturningRefuted_PreservesSuppliedReason(t *testing.T) {
	root := t.TempDir()
	gvDom(t, root, "run-1", "ag-x", DispositionConfirmed, "", 1)
	// Supply a real reason via a direct Writer append (gvDom leaves reason empty).
	w := Writer{}
	headSHA := "ag-x-h2"
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-x", RunID: "run-1", TS: time.Date(2026, 6, 22, 13, 0, 2, 0, time.UTC),
		Difficulty: 1, PawlVerdictRef: PawlVerdictRef{BeadID: "ag-x", HeadSHA: headSHA},
		Disposition: DispositionRefuted, HeadSHA: headSHA, Attempt: 2,
		AuthorContextID: "ctx", AuthorFamily: "claude", Domain: "auth", Reason: "missed nil-deref",
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	gv := storedRefuted(t, root, "ag-x", 2)
	if gv.Reason != "missed nil-deref" {
		t.Fatalf("a supplied reason must be preserved (no sentinel clobber): got %q", gv.Reason)
	}
	if gv.Domain != "auth" {
		t.Fatalf("a supplied domain must be preserved: got %q", gv.Domain)
	}
}

func TestAppendGateVerdict_OverturningRefuted_KeepsSuppliedDomain(t *testing.T) {
	root := t.TempDir()
	gvDom(t, root, "run-1", "ag-x", DispositionConfirmed, "", 1)
	gvDom(t, root, "run-1", "ag-x", DispositionRefuted, "auth", 2) // overturn WITH domain

	if got := storedDomain(t, root, "ag-x", 2); got != "auth" {
		t.Fatalf("a supplied domain must be preserved (no sentinel clobber): got %q, want %q", got, "auth")
	}
}

func TestAppendGateVerdict_FirstRefuted_NoSentinel(t *testing.T) {
	root := t.TempDir()
	// A REFUTED with NO prior CONFIRMED is normal rework, not an escape — must not
	// be stamped (stamping it would inflate the UNCLASSIFIED debt with non-escapes).
	gvDom(t, root, "run-1", "ag-y", DispositionRefuted, "", 1)

	if got := storedDomain(t, root, "ag-y", 1); got != "" {
		t.Fatalf("non-overturning REFUTED must keep empty domain (not an escape): got %q, want \"\"", got)
	}
}

func TestAppendGateVerdict_RefutedDifferentRun_NoSentinel(t *testing.T) {
	root := t.TempDir()
	// Escapes are run-scoped (mirrors DetectEscapes): a CONFIRMED in run-1 and a
	// REFUTED in run-2 are NOT one escape, so the run-2 REFUTED is not stamped.
	gvDom(t, root, "run-1", "ag-z", DispositionConfirmed, "", 1)
	gvDom(t, root, "run-2", "ag-z", DispositionRefuted, "", 2)

	if got := storedDomain(t, root, "ag-z", 2); got != "" {
		t.Fatalf("cross-run REFUTED is not an escape — must not be stamped: got %q, want \"\"", got)
	}
}

func TestIsOverturningRefuted_Predicate(t *testing.T) {
	led := &Ledger{Events: []Event{
		{Event: EventGateVerdict, BeadID: "ag-x", RunID: "r1", GateVerdict: &GateVerdictBody{Disposition: DispositionConfirmed, Attempt: 1}},
	}}
	cases := []struct {
		name string
		in   GateVerdictInput
		want bool
	}{
		{"refuted higher attempt same bead/run -> escape", GateVerdictInput{BeadID: "ag-x", RunID: "r1", Disposition: DispositionRefuted, Attempt: 2}, true},
		{"refuted equal attempt -> not strictly higher", GateVerdictInput{BeadID: "ag-x", RunID: "r1", Disposition: DispositionRefuted, Attempt: 1}, false},
		{"refuted other bead -> not an overturn", GateVerdictInput{BeadID: "ag-other", RunID: "r1", Disposition: DispositionRefuted, Attempt: 2}, false},
		{"refuted other run -> run-scoped, not an overturn", GateVerdictInput{BeadID: "ag-x", RunID: "r2", Disposition: DispositionRefuted, Attempt: 2}, false},
		{"confirmed -> never an overturn", GateVerdictInput{BeadID: "ag-x", RunID: "r1", Disposition: DispositionConfirmed, Attempt: 2}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsOverturningRefuted(led, tc.in); got != tc.want {
				t.Fatalf("IsOverturningRefuted = %v, want %v", got, tc.want)
			}
		})
	}
}

// EM.2.10: detector metadata on the overturning REFUTED (the catch) flows through
// DetectEscapes onto the Escape, so deriveFindingFromEscape can compile it into a
// mechanical constraint. Mirrors how Missed comes from the refuted verdict.
func TestDetectEscapes_CarriesDetectorMetadata(t *testing.T) {
	root := t.TempDir()
	gvDom(t, root, "run-1", "ag-x", DispositionConfirmed, "", 1)
	// REFUTED carrying a detector — appended via the real Writer.
	w := Writer{}
	headSHA := "ag-x-h2"
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID: "ag-x", RunID: "run-1", TS: time.Date(2026, 6, 22, 13, 0, 2, 0, time.UTC),
		Difficulty: 1, PawlVerdictRef: PawlVerdictRef{BeadID: "ag-x", HeadSHA: headSHA},
		Disposition: DispositionRefuted, HeadSHA: headSHA, Attempt: 2,
		AuthorContextID: "ctx", AuthorFamily: "claude", Domain: "validation", Reason: "caught",
		DetectorPattern: `eval\(`, ConstraintPathGlobs: "cli/**", DetectorKind: "regex",
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	led, err := LoadPath(LedgerPath(root))
	if err != nil {
		t.Fatal(err)
	}
	escapes := DetectEscapes(led, "run-1")
	if len(escapes) != 1 {
		t.Fatalf("want 1 escape, got %d", len(escapes))
	}
	e := escapes[0]
	if e.DetectorPattern != `eval\(` || e.ConstraintPathGlobs != "cli/**" || e.DetectorKind != "regex" {
		t.Fatalf("detector metadata not carried from the refuted catch: %+v", e)
	}
}

func TestDetectEscapes_DomainFallsBackToRefuted(t *testing.T) {
	root := t.TempDir()
	// CONFIRMED carries no domain; the overturning REFUTED tags domain="db".
	// The escape must classify on the REFUTED's domain (the conscious catch).
	gvDom(t, root, "run-1", "ag-x", DispositionConfirmed, "", 1)
	gvDom(t, root, "run-1", "ag-x", DispositionRefuted, "db", 2)

	led, err := LoadPath(LedgerPath(root))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	escapes := DetectEscapes(led, "run-1")
	if len(escapes) != 1 {
		t.Fatalf("want 1 escape, got %d", len(escapes))
	}
	if escapes[0].Domain != "db" {
		t.Fatalf("escape domain should fall back to the REFUTED's tag: got %q, want %q", escapes[0].Domain, "db")
	}
}
