package yieldledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// appendCatch records a REFUTED catch via the PRODUCTION writer — fixture-fidelity:
// round-trip through AppendGateVerdict + Load, never a hand-built GateVerdictBody{}
// (a hand-built body could set a shape the on-disk format never produces).
func appendCatch(t *testing.T, root, bead, headSHA, domain, reason, detector string, attempt int, paths []string) {
	t.Helper()
	w := Writer{}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID:          bead,
		RunID:           "run-1",
		TS:              time.Date(2026, 6, 27, 12, attempt, 0, 0, time.UTC),
		Difficulty:      1,
		PawlVerdictRef:  PawlVerdictRef{BeadID: bead, HeadSHA: headSHA},
		Disposition:     DispositionRefuted,
		AuthorContextID: "ctx",
		AuthorFamily:    "claude",
		RefuterFamilies: []string{"codex"},
		HeadSHA:         headSHA,
		Attempt:         attempt,
		Domain:          domain,
		Reason:          reason,
		DetectorPattern: detector,
		AffectedPaths:   paths,
	}); err != nil {
		t.Fatalf("append catch %s a%d: %v", bead, attempt, err)
	}
}

func TestDetectCatches_JudgmentClassIncludedAndPathKeyed(t *testing.T) {
	root := t.TempDir()
	// A JUDGMENT-class catch: no detector, but carries affected_paths.
	appendCatch(t, root, "age-aaa", "head0001", "pawl",
		"content-pattern key-injection into a possibly-reviewing pane is a fail-open", "", 2,
		[]string{"scripts/pawl.sh"})

	l, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	catches := DetectCatches(l)
	if len(catches) != 1 {
		t.Fatalf("want 1 catch class, got %d", len(catches))
	}
	c := catches[0]
	if c.DetectorPattern != "" {
		t.Fatalf("judgment-class catch should have no detector, got %q", c.DetectorPattern)
	}
	if len(c.AffectedPaths) != 1 || c.AffectedPaths[0] != "scripts/pawl.sh" {
		t.Fatalf("judgment-class catch must be path-keyed, got %v", c.AffectedPaths)
	}
	if c.HitCount != 1 {
		t.Fatalf("want HitCount 1, got %d", c.HitCount)
	}
	if cc := CompileCandidates(catches); len(cc) != 0 {
		t.Fatalf("CompileCandidates must EXCLUDE the judgment-class catch, got %d", len(cc))
	}
}

func TestCompileCandidates_DetectorSubsetOnly(t *testing.T) {
	root := t.TempDir()
	appendCatch(t, root, "age-bbb", "head0002", "shell",
		"unguarded command substitution aborts the route under set -e", "assign-cmdsub-no-guard", 2,
		[]string{"scripts/pawl.sh"})
	appendCatch(t, root, "age-ccc", "head0003", "pawl-review",
		"read-files review dropped deleted lines", "", 2,
		[]string{"scripts/pawl-review.sh"})

	l, _ := Load(root)
	catches := DetectCatches(l)
	if len(catches) != 2 {
		t.Fatalf("want 2 classes, got %d", len(catches))
	}
	cc := CompileCandidates(catches)
	if len(cc) != 1 {
		t.Fatalf("want 1 compile candidate (detector-bearing only), got %d", len(cc))
	}
	if cc[0].Domain != "shell" {
		t.Fatalf("wrong compile candidate, got domain %q", cc[0].Domain)
	}
}

func TestClassKeyFor_DeterministicAndVersioned(t *testing.T) {
	// Case/punctuation/stopword differences normalize to the SAME class.
	a := ClassKeyFor("Pawl", "A *_ready predicate called tmux send-keys!", "", "")
	b := ClassKeyFor("pawl", "ready predicate called tmux send keys", "", "")
	if a != b {
		t.Fatalf("same reason should yield same key:\n a=%q\n b=%q", a, b)
	}
	if !strings.HasPrefix(a, "v1:") {
		t.Fatalf("class key must carry the version prefix, got %q", a)
	}
	// A detector specializes the class (distinct key).
	if ClassKeyFor("pawl", "x", "rx", "") == ClassKeyFor("pawl", "x", "", "") {
		t.Fatalf("a detector pattern must specialize the class key")
	}
	// The stored ClassKey (computed at emit) equals the read-time key.
	root := t.TempDir()
	appendCatch(t, root, "age-kkk", "head0006", "pawl", "ready predicate called tmux send keys", "", 2, nil)
	l, _ := Load(root)
	if got := DetectCatches(l); len(got) != 1 || got[0].ClassKey != b {
		t.Fatalf("read-time key must match ClassKeyFor; got %#v", got)
	}
}

// appendCatchWithClass records a REFUTED catch carrying a SEMANTIC class through the
// PRODUCTION writer — fixture fidelity: the on-disk shape is whatever AppendGateVerdict
// emits, never a hand-built body. (age-jjt8)
func appendCatchWithClass(t *testing.T, root, bead, headSHA, domain, reason, class string, attempt int) {
	t.Helper()
	w := Writer{}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID:          bead,
		RunID:           "run-1",
		TS:              time.Date(2026, 6, 27, 12, attempt, 0, 0, time.UTC),
		Difficulty:      1,
		PawlVerdictRef:  PawlVerdictRef{BeadID: bead, HeadSHA: headSHA},
		Disposition:     DispositionRefuted,
		AuthorContextID: "ctx",
		AuthorFamily:    "claude",
		RefuterFamilies: []string{"codex"},
		HeadSHA:         headSHA,
		Attempt:         attempt,
		Domain:          domain,
		Reason:          reason,
		Class:           class,
	}); err != nil {
		t.Fatalf("append catch %s: %v", bead, err)
	}
}

// A SEMANTIC class overrides the reason as the class-identity component: the same
// --class on two DIFFERENT reasons yields the SAME key, and an empty class falls back
// to the reason (so historical rows key exactly as before). (age-jjt8, defect 1)
func TestClassKeyFor_SemanticClassOverridesReason(t *testing.T) {
	// Same class, different reason wording -> SAME key.
	a := ClassKeyFor("pawl", "some bead-specific verdict text A", "", "stale-retired-surface")
	b := ClassKeyFor("pawl", "totally different verdict text B", "", "stale-retired-surface")
	if a != b {
		t.Fatalf("same --class must yield the same key regardless of reason:\n a=%q\n b=%q", a, b)
	}
	if want := "v1:pawl/stale-retired-surface"; a != want {
		t.Fatalf("class key should be domain/class-slug; got %q want %q", a, want)
	}
	// Empty class -> reason path (backward compatible with pre-class rows).
	noClass := ClassKeyFor("pawl", "ready predicate called tmux send keys", "", "")
	if strings.Contains(noClass, "stale-retired-surface") {
		t.Fatalf("empty class must fall back to the reason, got %q", noClass)
	}
	// A detector still specializes a class-keyed catch.
	if ClassKeyFor("pawl", "x", "rx", "cls") == ClassKeyFor("pawl", "x", "", "cls") {
		t.Fatalf("detector must specialize even a semantic-class key")
	}
}

// TWO catches on DIFFERENT beads with DIFFERENT reasons but the SAME --class collapse
// to ONE cross-bead class (HitCount 2, both beads listed) — the recurrence that a
// bead-drifting reason-derived key made invisible. Round-tripped through the production
// writer+reader (fixture fidelity). (age-jjt8, scenario 1)
func TestDetectCatches_SemanticClassIsCrossBead(t *testing.T) {
	root := t.TempDir()
	appendCatchWithClass(t, root, "age-b1", "head0001", "scripts", "REFUTED: X on bead one", "stale-retired-surface", 1)
	appendCatchWithClass(t, root, "age-b2", "head0002", "scripts", "REFUTED: totally other wording on bead two", "stale-retired-surface", 2)

	l, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	catches := DetectCatches(l)
	if len(catches) != 1 {
		t.Fatalf("same --class on two beads must be ONE class, got %d: %#v", len(catches), catches)
	}
	c := catches[0]
	if c.ClassKey != "v1:scripts/stale-retired-surface" {
		t.Fatalf("cross-bead class key wrong: %q", c.ClassKey)
	}
	if c.HitCount != 2 {
		t.Fatalf("two distinct beads must count as 2 hits (recurring), got %d", c.HitCount)
	}
	if len(c.Beads) != 2 {
		t.Fatalf("class must list both beads, got %v", c.Beads)
	}
}

// Defect-1 contrast: the OLD bead-embedded reason fallback ("...REFUTED for <bead>...")
// keys PER-BEAD (the recurrence-blind behavior), while the de-bead-ided fallback
// ("...REFUTED (see evidence)") collapses to ONE per-domain class across beads. Both are
// exercised WITHOUT --class, so this pins the pawl-review.sh fallback-wording fix. (age-jjt8)
func TestClassKeyFor_DeBeadIdedFallbackCollapsesAcrossBeads(t *testing.T) {
	// OLD (bad) fallback: bead id in the reason -> distinct keys per bead.
	oldA := ClassKeyFor("scripts", "pawl-review REFUTED for age-aaa (see evidence)", "", "")
	oldB := ClassKeyFor("scripts", "pawl-review REFUTED for age-bbb (see evidence)", "", "")
	if oldA == oldB {
		t.Fatalf("sanity: bead-embedded reasons SHOULD differ per bead (that is the defect)")
	}
	// NEW (fixed) fallback: no bead id -> same key across beads.
	newA := ClassKeyFor("scripts", "pawl-review REFUTED (see evidence)", "", "")
	newB := ClassKeyFor("scripts", "pawl-review REFUTED (see evidence)", "", "")
	if newA != newB {
		t.Fatalf("de-bead-ided fallback must collapse across beads:\n a=%q\n b=%q", newA, newB)
	}
}

// A REAL line copied verbatim from the live ledger (a LEGACY bead-keyed catch that
// predates the class field) must still parse AND classify unchanged — the read path is
// fully backward compatible; a legacy row keeps its per-bead legacy class, never an
// error. (age-jjt8, scenario 2 — fixture fidelity against a real on-disk sample)
func TestDetectCatches_RealLegacyLedgerLine(t *testing.T) {
	// Verbatim from .agents/yield/yield-ledger.jsonl — the exact defect this bead fixes:
	// the bead id ("age-landq-self") baked into the stored class_key.
	const real = `{"event":"gate-verdict","bead_id":"age-landq-self","run_id":"membrane-catch","ts":"2026-06-27T17:20:48Z","body":{"difficulty":1,"pawl_verdict_ref":{"bead_id":"age-landq-self","head_sha":"291b2c0a9ff0de6caa0b8517bb66cf1a83c0cbec"},"disposition":"REFUTED","head_sha":"291b2c0a9ff0de6caa0b8517bb66cf1a83c0cbec","attempt":1,"mode":"fresh-context","author_context_id":"ao-membrane-catch","refuter_families":[],"author_family":"manual","cross_family":false,"author_ne_reviewer":true,"evidence_present":true,"domain":"scripts","reason":"pawl-review REFUTED for age-landq-self (see evidence)","class_key":"v1:scripts/pawl-review-refuted-age-landq-self-see-evidence","affected_paths":["scripts/land-queue-test.sh","tests/land-queue/PAINS.md","tests/land-queue/e2e-acceptance.bats"]}}`
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	if err := os.WriteFile(path, []byte(real+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := LoadPath(path)
	if err != nil {
		t.Fatalf("real legacy line must still parse: %v", err)
	}
	catches := DetectCatches(l)
	if len(catches) != 1 {
		t.Fatalf("want 1 catch class from the real line, got %d", len(catches))
	}
	if got, want := catches[0].ClassKey, "v1:scripts/pawl-review-refuted-age-landq-self-see-evidence"; got != want {
		t.Fatalf("legacy bead-keyed row must classify UNCHANGED\n got  %q\n want %q", got, want)
	}
}

// A nil RefuterFamilies must serialize as [] (not JSON null): the closed schema
// requires refuter_families to be an array, and the deterministic-catch tier can
// emit with no refuters. (schema↔Go consistency regression, S1)
func TestAppendGateVerdict_NilRefuterFamiliesSerializesAsArray(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID:          "age-z",
		RunID:           "r",
		TS:              time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
		PawlVerdictRef:  PawlVerdictRef{BeadID: "age-z", HeadSHA: "abcdef0"},
		Disposition:     DispositionRefuted,
		HeadSHA:         "abcdef0",
		Attempt:         1,
		AuthorContextID: "c",
		AuthorFamily:    "claude",
		Domain:          "d",
		Reason:          "a deterministic catch with no refuters",
		// RefuterFamilies intentionally nil.
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	raw, err := os.ReadFile(LedgerPath(root))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, `"refuter_families":[]`) {
		t.Fatalf("nil RefuterFamilies must serialize as [] (schema requires array); got:\n%s", s)
	}
	if strings.Contains(s, `"refuter_families":null`) {
		t.Fatalf("nil RefuterFamilies must NOT serialize as null; got:\n%s", s)
	}
}

// Empty-string items in refuter_families / affected_paths must be dropped at emit:
// validateGateVerdictEvent does not validate slice items, but the closed schema
// requires items.minLength:1, so the Writer sanitizes. (schema↔Go consistency, S1)
func TestAppendGateVerdict_EmptyStringItemsDropped(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID:          "age-q",
		RunID:           "r",
		TS:              time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
		PawlVerdictRef:  PawlVerdictRef{BeadID: "age-q", HeadSHA: "abcdef0"},
		Disposition:     DispositionRefuted,
		HeadSHA:         "abcdef0",
		Attempt:         1,
		AuthorContextID: "c",
		AuthorFamily:    "claude",
		Domain:          "d",
		Reason:          "empty items must drop",
		RefuterFamilies: []string{"", "codex", ""},
		AffectedPaths:   []string{"scripts/x.sh", ""},
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	raw, err := os.ReadFile(LedgerPath(root))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, `"refuter_families":["codex"]`) {
		t.Fatalf("empty refuter_families items must be dropped -> [\"codex\"]; got:\n%s", s)
	}
	if !strings.Contains(s, `"affected_paths":["scripts/x.sh"]`) {
		t.Fatalf("empty affected_paths items must be dropped -> [\"scripts/x.sh\"]; got:\n%s", s)
	}
}

// head_sha length is counted in CODE POINTS (matching the schema's code-point
// minLength:7), so a multi-byte head_sha that is 7 BYTES but <7 code points is
// REJECTED pre-write, never emitted. (schema↔Go consistency, S1)
func TestAppendGateVerdict_MultibyteHeadSHARejected(t *testing.T) {
	root := t.TempDir()
	w := Writer{}
	// "éééa" = 4 code points but 7 bytes (é = 2 UTF-8 bytes).
	if _, err := w.AppendGateVerdict(root, GateVerdictInput{
		BeadID:          "age-u",
		RunID:           "r",
		TS:              time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
		PawlVerdictRef:  PawlVerdictRef{BeadID: "age-u", HeadSHA: "éééa"},
		Disposition:     DispositionRefuted,
		HeadSHA:         "éééa",
		Attempt:         1,
		AuthorContextID: "c",
		AuthorFamily:    "claude",
		Domain:          "d",
		Reason:          "multibyte head_sha",
	}); err == nil {
		t.Fatalf("a 4-code-point (7-byte) head_sha must be REJECTED to match schema minLength:7 (code points)")
	}
}

func TestDetectCatches_RoundCollapse(t *testing.T) {
	root := t.TempDir()
	// TWO REFUTED rounds on the SAME (bead, head): review rounds, NOT recurrence.
	appendCatch(t, root, "age-ddd", "head0004", "pawl", "stall-clear fired after one poll", "", 2, []string{"scripts/pawl.sh"})
	appendCatch(t, root, "age-ddd", "head0004", "pawl", "stall-clear fired after one poll", "", 3, []string{"scripts/pawl.sh"})

	l, _ := Load(root)
	catches := DetectCatches(l)
	if len(catches) != 1 {
		t.Fatalf("two rounds on one (bead,head) -> 1 class, got %d", len(catches))
	}
	if catches[0].HitCount != 1 {
		t.Fatalf("round-collapse: 2 rounds on 1 (bead,head) must be HitCount 1, got %d", catches[0].HitCount)
	}

	// A DISTINCT bead in the SAME class is real recurrence -> HitCount 2.
	appendCatch(t, root, "age-eee", "head0005", "pawl", "stall-clear fired after one poll", "", 2, []string{"scripts/pawl.sh"})
	l2, _ := Load(root)
	c2 := DetectCatches(l2)
	if len(c2) != 1 || c2[0].HitCount != 2 {
		t.Fatalf("distinct bead same class -> HitCount 2; got classes=%d hit=%d", len(c2), c2[0].HitCount)
	}
	if len(c2[0].Beads) != 2 {
		t.Fatalf("want 2 distinct beads recorded, got %v", c2[0].Beads)
	}
}
