package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// fixedDigestClock is a stable timestamp so render output is byte-deterministic
// under test (the production command uses time.Now()).
var fixedDigestClock = time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)

// TestRankCatchDigest_TopNAndStableTies is the unit contract: rank by HitCount
// DESC, tie-break by ClassKey ASC, truncate to topN, and never mutate the input.
func TestRankCatchDigest_TopNAndStableTies(t *testing.T) {
	// Deliberately NOT pre-sorted, and two classes tie at HitCount 3 so the
	// ClassKey tie-break is exercised (b/docs sorts before c/go).
	in := []yieldledger.Catch{
		{ClassKey: "v1:go/one-off", HitCount: 1},
		{ClassKey: "v1:shell/top", HitCount: 5},
		{ClassKey: "v1:go/tie-second", HitCount: 3},
		{ClassKey: "v1:docs/tie-first", HitCount: 3},
	}

	tests := []struct {
		name     string
		topN     int
		wantKeys []string
	}{
		{
			name:     "top2 keeps highest then first tie",
			topN:     2,
			wantKeys: []string{"v1:shell/top", "v1:docs/tie-first"},
		},
		{
			name: "full ranking with stable tie-break",
			topN: 10,
			wantKeys: []string{
				"v1:shell/top",      // 5
				"v1:docs/tie-first", // 3, ClassKey < go/tie-second
				"v1:go/tie-second",  // 3
				"v1:go/one-off",     // 1
			},
		},
		{
			name: "topN<=0 keeps all",
			topN: 0,
			wantKeys: []string{
				"v1:shell/top", "v1:docs/tie-first", "v1:go/tie-second", "v1:go/one-off",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rankCatchDigest(in, tc.topN)
			if len(got) != len(tc.wantKeys) {
				t.Fatalf("top=%d: want %d classes, got %d (%+v)", tc.topN, len(tc.wantKeys), len(got), got)
			}
			for i, want := range tc.wantKeys {
				t.Logf("rank[%d]=%s (×%d)", i, got[i].ClassKey, got[i].HitCount)
				if got[i].ClassKey != want {
					t.Errorf("rank[%d]: want %q, got %q", i, want, got[i].ClassKey)
				}
			}
		})
	}

	// Purity: the caller's slice order must be untouched (DetectCatches' first-
	// appearance order is relied on elsewhere).
	if in[0].ClassKey != "v1:go/one-off" {
		t.Fatalf("rankCatchDigest mutated its input: in[0]=%q", in[0].ClassKey)
	}
}

// TestCatchWatchFor_DeterministicImperative asserts the "watch-for-this" transform
// is a genuine imperative scoped to the class's domain + files, with no LLM call.
func TestCatchWatchFor_DeterministicImperative(t *testing.T) {
	tests := []struct {
		name  string
		catch yieldledger.Catch
		want  string
	}{
		{
			name:  "domain only",
			catch: yieldledger.Catch{Domain: "docs"},
			want:  "watch for it when working in `docs`",
		},
		{
			name:  "domain plus paths",
			catch: yieldledger.Catch{Domain: "shell", AffectedPaths: []string{"scripts/a.sh", "scripts/b.sh"}},
			want:  "watch for it when working in `shell` (scripts/a.sh, scripts/b.sh)",
		},
		{
			name:  "paths truncated with more",
			catch: yieldledger.Catch{Domain: "go", AffectedPaths: []string{"a.go", "b.go", "c.go", "d.go", "e.go"}},
			want:  "watch for it when working in `go` (a.go, b.go, c.go, +2 more)",
		},
		{
			name:  "no domain no paths",
			catch: yieldledger.Catch{},
			want:  "watch for it",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := catchWatchFor(tc.catch)
			t.Logf("watch-for => %q", got)
			if got != tc.want {
				t.Errorf("want %q, got %q", tc.want, got)
			}
			if !strings.HasPrefix(got, "watch for") {
				t.Errorf("directive must be an imperative starting with 'watch for'; got %q", got)
			}
		})
	}
}

// TestRenderCatchDigest_ByteIdempotent proves a fixed digest renders identically
// twice (no nondeterministic map iteration), and carries the loader's frontmatter.
func TestRenderCatchDigest_ByteIdempotent(t *testing.T) {
	d, err := buildCatchDigest([]yieldledger.Catch{
		{ClassKey: "v1:shell/top", Domain: "shell", Reason: "unguarded cmdsub aborts under set -e", HitCount: 5, Beads: []string{"age-a", "age-b"}, AffectedPaths: []string{"scripts/x.sh"}},
		{ClassKey: "v1:docs/stale", Domain: "docs", Reason: "stale retired surface referenced in shipped docs", HitCount: 2, Beads: []string{"age-c"}},
	}, 10, false, fixedDigestClock)
	if err != nil {
		t.Fatal(err)
	}

	first := renderCatchDigest(d)
	second := renderCatchDigest(d)
	if !bytes.Equal(first, second) {
		t.Fatalf("renderCatchDigest not byte-idempotent:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	body := string(first)
	t.Logf("rendered digest:\n%s", body)
	for _, want := range []string{
		`type: "pre-mortem-check"`,
		`status: "active"`,
		`applicable_when: ["recurring-catch"]`,
		"# Pre-Mortem Check: Recurring catch classes to watch for",
		"unguarded cmdsub aborts under set -e → watch for it when working in `shell`",
		"stale retired surface referenced in shipped docs → watch for it when working in `docs`",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered digest missing %q", want)
		}
	}
	// Honest scope (ADR-0004/0011): no compounding-moat marketing.
	if strings.Contains(strings.ToLower(body), "compounding moat") && !strings.Contains(body, "NOT a compounding moat") {
		t.Errorf("digest must not claim a compounding moat")
	}
}

// TestBuildCatchDigest_ExcludesPlaceholdersByDefault is Scenario 1 (age-7758): a
// reason-less placeholder class must NOT out-rank a real-reason class even when it has
// a HIGHER raw HitCount. By default placeholders are excluded entirely, so the real
// class leads and the placeholders are gone.
func TestBuildCatchDigest_ExcludesPlaceholdersByDefault(t *testing.T) {
	in := []yieldledger.Catch{
		// A placeholder with the HIGHEST hit count — the noise that dominates today.
		{ClassKey: "v1:docs/pawl", Domain: "docs", Reason: "pawl-review REFUTED (see evidence)", HitCount: 25},
		// A bare-token placeholder.
		{ClassKey: "v1:cli/bare", Domain: "cli", Reason: "r", HitCount: 4},
		// The one real, actionable class — buried at HitCount 1 under the placeholders.
		{ClassKey: "v1:gates/real", Domain: "gates", Reason: "gate-routing gap: a .agents edit skips its own contract gate", HitCount: 1},
	}

	d, err := buildCatchDigest(in, 10, false, fixedDigestClock)
	if err != nil {
		t.Fatal(err)
	}

	if d.TotalClasses != 3 {
		t.Errorf("TotalClasses should report the full corpus (3), got %d", d.TotalClasses)
	}
	if d.PlaceholderClasses != 2 {
		t.Errorf("want 2 placeholder classes reported, got %d", d.PlaceholderClasses)
	}
	if len(d.Entries) != 1 {
		t.Fatalf("default must exclude both placeholders, keeping only the 1 real class; got %d entries: %+v", len(d.Entries), d.Entries)
	}
	if d.Entries[0].Reason != "gate-routing gap: a .agents edit skips its own contract gate" {
		t.Errorf("the lone actionable class must lead; got %q", d.Entries[0].Reason)
	}
	// The placeholders (higher HitCount) must NOT appear at all.
	for _, e := range d.Entries {
		if yieldledger.IsPlaceholderReason(e.Reason) {
			t.Errorf("placeholder reason leaked into default digest: %q", e.Reason)
		}
	}
}

// TestBuildCatchDigest_IncludePlaceholders is Scenario 2 (age-7758): the escape hatch
// shows EVERYTHING for corpus auditing, but real-reason classes still lead — a
// placeholder with a higher HitCount ranks BELOW every actionable class.
func TestBuildCatchDigest_IncludePlaceholders(t *testing.T) {
	in := []yieldledger.Catch{
		{ClassKey: "v1:docs/pawl", Domain: "docs", Reason: "pawl-review REFUTED (see evidence)", HitCount: 25},
		{ClassKey: "v1:gates/real", Domain: "gates", Reason: "gate-routing gap: a .agents edit skips its own contract gate", HitCount: 1},
	}

	d, err := buildCatchDigest(in, 10, true, fixedDigestClock)
	if err != nil {
		t.Fatal(err)
	}

	if len(d.Entries) != 2 {
		t.Fatalf("--include-placeholders must show all classes; got %d", len(d.Entries))
	}
	// Real class first despite lower HitCount; placeholder trails.
	if yieldledger.IsPlaceholderReason(d.Entries[0].Reason) {
		t.Errorf("real-reason class must lead even in audit mode; entry[0]=%q", d.Entries[0].Reason)
	}
	if !d.Entries[1].Placeholder {
		t.Errorf("the trailing entry must be flagged Placeholder=true; got %+v", d.Entries[1])
	}
	if d.Entries[0].Rank != 1 || d.Entries[1].Rank != 2 {
		t.Errorf("ranks must be contiguous 1,2; got %d,%d", d.Entries[0].Rank, d.Entries[1].Rank)
	}
}

func TestBuildCatchDigest_FindingRecurrenceCreatesAdvisoryProducerCandidateByObjective(t *testing.T) {
	d, err := buildCatchDigest([]yieldledger.Catch{
		{
			ClassKey: "v1:docs/stale-surface",
			Domain:   "docs",
			Reason:   "A retired surface remained in active documentation.",
			// Five review occurrences, but only two objectives.
			HitCount: 5,
			Beads:    []string{"objective-a", "objective-b"},
		},
		{
			ClassKey: "v1:shell/one-off",
			Domain:   "shell",
			Reason:   "A one-off typo was caught.",
			HitCount: 1,
			Beads:    []string{"objective-c"},
		},
	}, 10, false, fixedDigestClock)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.ProducerCandidates) != 1 {
		t.Fatalf("producer_candidates = %d, want 1", len(d.ProducerCandidates))
	}
	candidate := d.ProducerCandidates[0]
	if candidate.RecurrenceCount != 2 || !candidate.Advisory {
		t.Fatalf("candidate = %+v, want advisory recurrence_count=2", candidate)
	}
	if len(candidate.Evidence) != 2 || candidate.Evidence[0].ObjectiveID != "objective-a" || candidate.Evidence[1].ObjectiveID != "objective-b" {
		t.Fatalf("candidate must cite both distinct objectives: %+v", candidate.Evidence)
	}
	if body := string(renderCatchDigest(d)); !strings.Contains(body, "Advisory producer-rule candidates") || !strings.Contains(body, "recurrence=2") {
		t.Fatalf("rendered digest omitted advisory candidate:\n%s", body)
	}
}

// TestRunMembraneDigest_FiltersPlaceholdersE2E is the e2e acceptance: seed a real
// ledger mixing placeholder classes (higher hit counts) and one real-reason class,
// then assert the WRITTEN checklist leads with the actionable reason and drops the
// placeholders by default — and that --include-placeholders restores them below it.
func TestRunMembraneDigest_FiltersPlaceholdersE2E(t *testing.T) {
	root := t.TempDir()
	setDigestProjectDir(t, root)

	// Two placeholder classes recur heavily; the one real class hits once.
	seedCatch(t, root, "age-1", "docs", "pawl-review REFUTED (see evidence)", []string{"README.md"})
	seedCatch(t, root, "age-2", "docs", "pawl-review REFUTED (see evidence)", []string{"docs/x.md"})
	seedCatch(t, root, "age-3", "docs", "pawl-review REFUTED (see evidence)", []string{"docs/y.md"})
	seedCatch(t, root, "age-4", "cli", "r", []string{"cli/a.go"})
	seedCatch(t, root, "age-5", "gates", "gate-routing gap: a .agents edit skips its own contract gate", []string{"scripts/gate.sh"})

	readDigest := func(includePlaceholders bool) string {
		var buf bytes.Buffer
		membraneDigestCmd.SetOut(&buf)
		membraneDigestTopN = catchDigestDefaultTopN
		membraneDigestIncludePlaceholders = includePlaceholders
		if err := runMembraneDigest(membraneDigestCmd, nil); err != nil {
			t.Fatalf("runMembraneDigest(include=%v): %v", includePlaceholders, err)
		}
		raw, err := os.ReadFile(filepath.Join(root, ".agents", "pre-mortem-checks", "catch-digest.md"))
		if err != nil {
			t.Fatalf("digest not written: %v", err)
		}
		return string(raw)
	}

	// Default: the actionable class leads; the placeholders are gone.
	def := readDigest(false)
	t.Logf("DEFAULT digest:\n%s", def)
	if !strings.Contains(def, "gate-routing gap") {
		t.Errorf("default digest must surface the actionable class; body:\n%s", def)
	}
	if strings.Contains(def, "pawl-review REFUTED") {
		t.Errorf("default digest must exclude the pawl-review placeholder; body:\n%s", def)
	}

	// Audit: --include-placeholders restores them, but the real class still leads.
	all := readDigest(true)
	t.Logf("INCLUDE-PLACEHOLDERS digest:\n%s", all)
	if !strings.Contains(all, "pawl-review REFUTED") {
		t.Errorf("--include-placeholders must restore placeholder classes; body:\n%s", all)
	}
	posReal := strings.Index(all, "gate-routing gap")
	posPlaceholder := strings.Index(all, "pawl-review REFUTED")
	if posReal < 0 || posReal >= posPlaceholder {
		t.Errorf("real class must rank above placeholder even in audit mode: real=%d placeholder=%d", posReal, posPlaceholder)
	}
}

// seedCatch emits one REFUTED catch verdict into root's yield ledger via the
// production Writer — the same path recall/triage read, so the fixture is the real
// persisted shape (go.md: guard-test fixtures use the production writer).
func seedCatch(t *testing.T, root, bead, domain, reason string, paths []string) {
	t.Helper()
	in := buildCatchInput(bead, domain, reason, paths, "", "", "", "", "abcdef0", "", fixedDigestClock)
	w := yieldledger.Writer{}
	if _, err := w.AppendGateVerdict(root, in); err != nil {
		t.Fatalf("seedCatch(%s/%s): %v", domain, bead, err)
	}
}

// setDigestProjectDir points the command handlers at root and restores the shared
// package globals on cleanup (shared rootCmd/flag vars — .claude/rules/go.md).
func setDigestProjectDir(t *testing.T, root string) {
	t.Helper()
	origProjectDir := testProjectDir
	testProjectDir = root
	origTop, origJSON, origIncl := membraneDigestTopN, membraneDigestJSON, membraneDigestIncludePlaceholders
	origDeltas, origSince := membraneDigestDeltas, membraneDigestSince
	membraneDigestTopN, membraneDigestJSON, membraneDigestIncludePlaceholders = catchDigestDefaultTopN, false, false
	membraneDigestDeltas, membraneDigestSince = false, ""
	t.Cleanup(func() {
		testProjectDir = origProjectDir
		membraneDigestTopN, membraneDigestJSON, membraneDigestIncludePlaceholders = origTop, origJSON, origIncl
		membraneDigestDeltas, membraneDigestSince = origDeltas, origSince
		membraneDigestCmd.SetOut(nil)
	})
}

// TestRunMembraneDigest_WritesChecklistToPreMortemChecks is the e2e acceptance:
// seed a real ledger, run the command, assert the checklist lands in the
// pre-mortem-checks sink ranked correctly and in the loader's expected format.
func TestRunMembraneDigest_WritesChecklistToPreMortemChecks(t *testing.T) {
	root := t.TempDir()
	setDigestProjectDir(t, root)

	// Class "top" recurs 3× (distinct beads); "mid" 2×; "low" 1×. HitCount counts
	// distinct (bead, head), so three beads on one class => HitCount 3.
	seedCatch(t, root, "age-1", "shell", "unguarded cmdsub aborts under set -e", []string{"scripts/x.sh"})
	seedCatch(t, root, "age-2", "shell", "unguarded cmdsub aborts under set -e", []string{"scripts/y.sh"})
	seedCatch(t, root, "age-3", "shell", "unguarded cmdsub aborts under set -e", []string{"scripts/z.sh"})
	seedCatch(t, root, "age-4", "docs", "stale retired surface referenced in shipped docs", []string{"README.md"})
	seedCatch(t, root, "age-5", "docs", "stale retired surface referenced in shipped docs", []string{"docs/x.md"})
	seedCatch(t, root, "age-6", "go", "missing t.Cleanup restore of shared global", []string{"cli/cmd/ao/a.go"})

	var buf bytes.Buffer
	membraneDigestCmd.SetOut(&buf)
	membraneDigestTopN = catchDigestDefaultTopN
	if err := runMembraneDigest(membraneDigestCmd, nil); err != nil {
		t.Fatalf("runMembraneDigest: %v", err)
	}
	t.Logf("stdout:\n%s", buf.String())

	// The auto-mined sink exists at the loader's canonical location.
	path := filepath.Join(root, ".agents", "pre-mortem-checks", "catch-digest.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("digest file not written to pre-mortem-checks sink: %v", err)
	}
	body := string(raw)
	t.Logf("catch-digest.md:\n%s", body)

	// Loader-shape assertions: a subsequent /pre-mortem load globs *.md here and
	// reads YAML frontmatter (type/status/applicable_when) + the check heading.
	for _, want := range []string{
		`type: "pre-mortem-check"`,
		`status: "active"`,
		`applicable_when: ["recurring-catch"]`,
		"# Pre-Mortem Check:",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("digest missing loader-expected token %q", want)
		}
	}

	// Watch-for lines: reason -> imperative, one per class.
	for _, want := range []string{
		"unguarded cmdsub aborts under set -e → watch for it when working in `shell`",
		"stale retired surface referenced in shipped docs → watch for it when working in `docs`",
		"missing t.Cleanup restore of shared global → watch for it when working in `go`",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("digest missing watch-for line %q", want)
		}
	}

	// Ranking: the 3× shell class must appear BEFORE the 2× docs class, which must
	// appear before the 1× go class.
	posTop := strings.Index(body, "unguarded cmdsub")
	posMid := strings.Index(body, "stale retired surface")
	posLow := strings.Index(body, "missing t.Cleanup")
	if posTop < 0 || posTop >= posMid || posMid >= posLow {
		t.Errorf("digest not ranked by HitCount desc: top=%d mid=%d low=%d", posTop, posMid, posLow)
	}
	if !strings.Contains(body, "×3") {
		t.Errorf("digest must show the ×3 hit count for the top class")
	}
}

// TestRunMembraneDigest_Idempotent proves a re-run overwrites cleanly (no error,
// still exactly one digest file with the same ranked content).
func TestRunMembraneDigest_Idempotent(t *testing.T) {
	root := t.TempDir()
	setDigestProjectDir(t, root)
	seedCatch(t, root, "age-1", "shell", "unguarded cmdsub", []string{"scripts/x.sh"})

	run := func() string {
		var buf bytes.Buffer
		membraneDigestCmd.SetOut(&buf)
		membraneDigestTopN = catchDigestDefaultTopN
		if err := runMembraneDigest(membraneDigestCmd, nil); err != nil {
			t.Fatalf("runMembraneDigest: %v", err)
		}
		path := filepath.Join(root, ".agents", "pre-mortem-checks", "catch-digest.md")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read digest: %v", err)
		}
		return string(raw)
	}
	first := run()
	second := run()

	// Exactly one digest file — a re-run overwrites, never accumulates.
	matches, _ := filepath.Glob(filepath.Join(root, ".agents", "pre-mortem-checks", "catch-digest*.md"))
	if len(matches) != 1 {
		t.Fatalf("re-run must overwrite one file, found %d: %v", len(matches), matches)
	}
	// Content (modulo the informational timestamp) is stable across runs.
	stripTS := func(s string) string {
		var kept []string
		for _, line := range strings.Split(s, "\n") {
			if strings.HasPrefix(line, "generated_at:") {
				continue
			}
			kept = append(kept, line)
		}
		return strings.Join(kept, "\n")
	}
	if stripTS(first) != stripTS(second) {
		t.Errorf("digest content drifted across idempotent re-runs:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// TestRunMembraneDigest_JSON asserts --json prints the ranked list AND still writes
// the checklist file.
func TestRunMembraneDigest_JSON(t *testing.T) {
	root := t.TempDir()
	setDigestProjectDir(t, root)
	seedCatch(t, root, "age-1", "shell", "unguarded cmdsub", []string{"scripts/x.sh"})
	seedCatch(t, root, "age-2", "shell", "unguarded cmdsub", []string{"scripts/y.sh"})
	seedCatch(t, root, "age-3", "docs", "stale retired surface", []string{"README.md"})

	var buf bytes.Buffer
	membraneDigestCmd.SetOut(&buf)
	membraneDigestJSON = true
	membraneDigestTopN = catchDigestDefaultTopN
	if err := runMembraneDigest(membraneDigestCmd, nil); err != nil {
		t.Fatalf("runMembraneDigest --json: %v", err)
	}

	var got catchDigest
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("--json output is not valid JSON: %v\n%s", err, buf.String())
	}
	t.Logf("json: total_classes=%d total_hits=%d entries=%d", got.TotalClasses, got.TotalHits, len(got.Entries))
	if got.TotalClasses != 2 {
		t.Errorf("want 2 classes, got %d", got.TotalClasses)
	}
	if len(got.Entries) == 0 || got.Entries[0].HitCount != 2 || got.Entries[0].Domain != "shell" {
		t.Fatalf("top entry must be the 2× shell class, got %+v", got.Entries)
	}
	if got.Entries[0].Rank != 1 || !strings.HasPrefix(got.Entries[0].WatchFor, "watch for") {
		t.Errorf("entry[0] rank/watch_for malformed: %+v", got.Entries[0])
	}
	// --json still writes the file.
	if _, err := os.Stat(filepath.Join(root, ".agents", "pre-mortem-checks", "catch-digest.md")); err != nil {
		t.Errorf("--json must still write the checklist file: %v", err)
	}
}

// TestRunMembraneDigest_EmptyCorpus writes an empty-but-valid checklist rather than
// erroring, so the loader sink always exists.
func TestRunMembraneDigest_EmptyCorpus(t *testing.T) {
	root := t.TempDir()
	setDigestProjectDir(t, root)

	var buf bytes.Buffer
	membraneDigestCmd.SetOut(&buf)
	membraneDigestTopN = catchDigestDefaultTopN
	if err := runMembraneDigest(membraneDigestCmd, nil); err != nil {
		t.Fatalf("runMembraneDigest on empty corpus: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".agents", "pre-mortem-checks", "catch-digest.md"))
	if err != nil {
		t.Fatalf("empty-corpus digest not written: %v", err)
	}
	if !strings.Contains(string(raw), "No classifiable catch classes recorded yet") {
		t.Errorf("empty-corpus digest must say so, got:\n%s", raw)
	}
}

// TestRunMembraneDigest_RejectsNonPositiveTop guards the --top validation.
func TestRunMembraneDigest_RejectsNonPositiveTop(t *testing.T) {
	root := t.TempDir()
	setDigestProjectDir(t, root)
	membraneDigestCmd.SetOut(&bytes.Buffer{})
	membraneDigestTopN = 0
	if err := runMembraneDigest(membraneDigestCmd, nil); err == nil {
		t.Fatal("--top 0 must be rejected")
	}
}

// seedCatchAt emits one REFUTED catch verdict at an explicit timestamp via the
// production Writer (fixture fidelity, as seedCatch) — the deltas measurement
// splits per-class hits on the envelope ts, so tests control it exactly.
func seedCatchAt(t *testing.T, root, bead, domain, reason string, paths []string, ts time.Time) {
	t.Helper()
	in := buildCatchInput(bead, domain, reason, paths, "", "", "", "", "abcdef0", "", ts)
	w := yieldledger.Writer{}
	if _, err := w.AppendGateVerdict(root, in); err != nil {
		t.Fatalf("seedCatchAt(%s/%s): %v", domain, bead, err)
	}
}

// deltaCutoff is the fix date D the Gherkin pins: 3 catches of class X land before
// it, 1 after.
var deltaCutoff = time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

// seedDeltaLedger seeds the acceptance corpus: class X (shell) with 3 hits before
// deltaCutoff and 1 since; class Y (docs) with 2 before and 0 since (improved).
func seedDeltaLedger(t *testing.T, root string) {
	t.Helper()
	const reasonX = "unguarded cmdsub aborts under set -e"
	const reasonY = "stale retired surface referenced in shipped docs"
	seedCatchAt(t, root, "age-x1", "shell", reasonX, []string{"scripts/a.sh"}, deltaCutoff.AddDate(0, 0, -5))
	seedCatchAt(t, root, "age-x2", "shell", reasonX, []string{"scripts/b.sh"}, deltaCutoff.AddDate(0, 0, -3))
	seedCatchAt(t, root, "age-x3", "shell", reasonX, []string{"scripts/c.sh"}, deltaCutoff.AddDate(0, 0, -1))
	seedCatchAt(t, root, "age-x4", "shell", reasonX, []string{"scripts/d.sh"}, deltaCutoff.Add(10*time.Hour))
	seedCatchAt(t, root, "age-y1", "docs", reasonY, []string{"README.md"}, deltaCutoff.AddDate(0, 0, -4))
	seedCatchAt(t, root, "age-y2", "docs", reasonY, []string{"docs/x.md"}, deltaCutoff.AddDate(0, 0, -2))
}

// TestParseDeltaCutoff pins the accepted --since forms: a bare ISO date (UTC
// midnight) or a full RFC3339 timestamp; anything else is a hard error.
func TestParseDeltaCutoff(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    time.Time
		wantErr bool
	}{
		{name: "ISO date is UTC midnight", in: "2026-07-08", want: time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)},
		{name: "full RFC3339", in: "2026-07-08T15:30:00Z", want: time.Date(2026, 7, 8, 15, 30, 0, 0, time.UTC)},
		{name: "RFC3339 with offset normalizes", in: "2026-07-08T02:00:00+02:00", want: time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)},
		{name: "garbage rejected", in: "next tuesday", wantErr: true},
		{name: "empty rejected", in: "", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDeltaCutoff(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseDeltaCutoff(%q) must error, got %v", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDeltaCutoff(%q): %v", tc.in, err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("parseDeltaCutoff(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestRunMembraneDigest_DeltasBeforeAfter is the Gherkin happy path (age-de5t):
// GIVEN class X with 3 catches before D and 1 after, WHEN `--deltas --since D`
// runs, THEN X's row shows before=3 since=1; class Y (0 since) trails as improved.
// Deltas is a read-only measurement: it must NOT write the checklist sink.
func TestRunMembraneDigest_DeltasBeforeAfter(t *testing.T) {
	root := t.TempDir()
	setDigestProjectDir(t, root)
	seedDeltaLedger(t, root)

	var buf bytes.Buffer
	membraneDigestCmd.SetOut(&buf)
	membraneDigestDeltas, membraneDigestSince = true, "2026-07-08"
	if err := runMembraneDigest(membraneDigestCmd, nil); err != nil {
		t.Fatalf("runMembraneDigest --deltas: %v", err)
	}
	out := buf.String()
	t.Logf("deltas output:\n%s", out)

	if !strings.Contains(out, "before=3 since=1") {
		t.Errorf("class X row must show before=3 since=1; got:\n%s", out)
	}
	if !strings.Contains(out, "before=2 since=0") {
		t.Errorf("class Y row must show before=2 since=0; got:\n%s", out)
	}
	// Sort: since DESC — the still-recurring class X leads; the 0-since class Y
	// trails, marked improved.
	posX := strings.Index(out, "unguarded cmdsub")
	posY := strings.Index(out, "stale retired surface")
	if posX < 0 || posX >= posY {
		t.Errorf("still-recurring class must sort before the improved one: X=%d Y=%d", posX, posY)
	}
	improvedLine := out[posY:]
	if !strings.Contains(improvedLine, "improved") {
		t.Errorf("0-since class must be marked improved; got:\n%s", improvedLine)
	}
	// Read-only: no checklist file (that is the default mode's sink, not a measurement's).
	if _, err := os.Stat(filepath.Join(root, ".agents", "pre-mortem-checks", "catch-digest.md")); !os.IsNotExist(err) {
		t.Errorf("--deltas must not write the checklist sink (stat err=%v)", err)
	}
}

// TestRunMembraneDigest_DeltasJSON asserts the machine shape the post-mortem
// runner consumes: per-class before/since counts plus the normalized cutoff.
func TestRunMembraneDigest_DeltasJSON(t *testing.T) {
	root := t.TempDir()
	setDigestProjectDir(t, root)
	seedDeltaLedger(t, root)

	var buf bytes.Buffer
	membraneDigestCmd.SetOut(&buf)
	membraneDigestDeltas, membraneDigestSince, membraneDigestJSON = true, "2026-07-08", true
	if err := runMembraneDigest(membraneDigestCmd, nil); err != nil {
		t.Fatalf("runMembraneDigest --deltas --json: %v", err)
	}

	var got catchDeltas
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("--deltas --json output is not valid JSON: %v\n%s", err, buf.String())
	}
	t.Logf("json: since=%s classes=%d", got.Since, got.TotalClasses)
	if got.Since != "2026-07-08T00:00:00Z" {
		t.Errorf("cutoff must be normalized RFC3339 UTC, got %q", got.Since)
	}
	if got.TotalClasses != 2 || len(got.Entries) != 2 {
		t.Fatalf("want 2 classes, got total=%d entries=%d", got.TotalClasses, len(got.Entries))
	}
	x, y := got.Entries[0], got.Entries[1]
	if x.Domain != "shell" || x.Before != 3 || x.Since != 1 || x.Improved {
		t.Errorf("entry[0] must be shell before=3 since=1 improved=false, got %+v", x)
	}
	if y.Domain != "docs" || y.Before != 2 || y.Since != 0 || !y.Improved {
		t.Errorf("entry[1] must be docs before=2 since=0 improved=true, got %+v", y)
	}
	if x.ClassKey == "" || y.ClassKey == "" {
		t.Errorf("entries must carry class keys, got %+v / %+v", x, y)
	}
}

// TestRunMembraneDigest_DeltasEmptyLedger is the edge: an empty ledger yields a
// clean empty result, exit 0 — never an error.
func TestRunMembraneDigest_DeltasEmptyLedger(t *testing.T) {
	root := t.TempDir()
	setDigestProjectDir(t, root)

	var buf bytes.Buffer
	membraneDigestCmd.SetOut(&buf)
	membraneDigestDeltas, membraneDigestSince = true, "2026-07-08"
	if err := runMembraneDigest(membraneDigestCmd, nil); err != nil {
		t.Fatalf("--deltas on an empty ledger must exit 0, got: %v", err)
	}
	if !strings.Contains(buf.String(), "no catch classes") {
		t.Errorf("empty ledger must say so cleanly, got:\n%s", buf.String())
	}
}

// TestRunMembraneDigest_DeltasFlagValidation guards the flag pairing: --since
// requires --deltas, --deltas requires --since, and a bad cutoff is rejected.
func TestRunMembraneDigest_DeltasFlagValidation(t *testing.T) {
	root := t.TempDir()
	setDigestProjectDir(t, root)
	membraneDigestCmd.SetOut(&bytes.Buffer{})

	membraneDigestDeltas, membraneDigestSince = true, ""
	if err := runMembraneDigest(membraneDigestCmd, nil); err == nil {
		t.Error("--deltas without --since must be rejected")
	}
	membraneDigestDeltas, membraneDigestSince = false, "2026-07-08"
	if err := runMembraneDigest(membraneDigestCmd, nil); err == nil {
		t.Error("--since without --deltas must be rejected")
	}
	membraneDigestDeltas, membraneDigestSince = true, "not-a-date"
	if err := runMembraneDigest(membraneDigestCmd, nil); err == nil {
		t.Error("an unparsable --since must be rejected")
	}
}
