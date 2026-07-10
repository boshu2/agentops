// Tests for the VERIFIED FRONTIER + pending window in `ao yield report`
// (age-fdae, R1 of the async-membrane epic age-xnet).
//
// Executed-red TDD: a real temp git repo (origin/main published via
// update-ref — no network remote) + a seeded provenance ledger whose lines
// mirror the REAL persisted shape of docs/provenance/ledger.jsonl (verdict →
// commit edge, disposition inside evidence_ref — fixture-fidelity rule,
// .claude/rules/go.md). All shared state restored per the isolation rule:
// setYieldReportState registers the cleanups; git runs with cmd.Dir set and a
// hermetic per-command env, never against the real repo.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// frontierGit runs one git command against the fixture repo with a hermetic
// identity/config/date env (cmd.Dir scoped — never the real repo).
func frontierGit(t *testing.T, root string, when time.Time, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	stamp := when.UTC().Format(time.RFC3339)
	cmd.Env = append(gitDiscoveryEnv(),
		"GIT_AUTHOR_NAME=frontier-test", "GIT_AUTHOR_EMAIL=frontier@test",
		"GIT_COMMITTER_NAME=frontier-test", "GIT_COMMITTER_EMAIL=frontier@test",
		"GIT_AUTHOR_DATE="+stamp, "GIT_COMMITTER_DATE="+stamp,
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newFrontierRepo initializes a temp git repo the frontier walk can read.
func newFrontierRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	frontierGit(t, root, reportTestNow, "init", "-q", "-b", "main")
	return root
}

// frontierCommitFixture appends to relPath and commits it with the given
// subject/body at a fixed committer time, returning the full sha.
func frontierCommitFixture(t *testing.T, root, subject, body, relPath string, when time.Time) string {
	t.Helper()
	abs := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", relPath, err)
	}
	f, err := os.OpenFile(abs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s: %v", relPath, err)
	}
	if _, err := fmt.Fprintf(f, "// %s\n", subject); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", relPath, err)
	}
	msg := subject
	if body != "" {
		msg = subject + "\n\n" + body
	}
	frontierGit(t, root, when, "add", "-A")
	frontierGit(t, root, when, "commit", "-q", "-m", msg)
	return frontierGit(t, root, when, "rev-parse", "HEAD")
}

// publishOriginMain points refs/remotes/origin/main at HEAD — the ref the
// frontier walks — without any network remote.
func publishOriginMain(t *testing.T, root string) {
	t.Helper()
	frontierGit(t, root, reportTestNow, "update-ref", "refs/remotes/origin/main", "HEAD")
}

// commitLedgerAndPublish commits the seeded provenance ledger as a #trivial
// provenance-only bind commit (the exact production land shape) and publishes
// origin/main at it. Required since the age-fdae refute-fix: the frontier
// reads origin/main's COMMITTED ledger, never the worktree file — so fixture
// evidence must be committed to count. The bind commit itself resolves via
// the waiver arm (provenance-only diff + trailing #trivial tag).
func commitLedgerAndPublish(t *testing.T, root string) {
	t.Helper()
	frontierGit(t, root, reportTestNow, "add", "docs/provenance")
	frontierGit(t, root, reportTestNow, "commit", "-q", "-m", "chore(provenance): bind fixture verdicts #trivial")
	publishOriginMain(t, root)
}

// seedProvenanceVerdict appends one verdict→commit edge to the fixture repo's
// provenance ledger in the REAL persisted line shape (field set mirrored from
// docs/provenance/ledger.jsonl: from_id "<bead>@<sha7>", to_id full commit
// sha, disposition inside evidence_ref).
func seedProvenanceVerdict(t *testing.T, root, bead, sha, disposition string) {
	t.Helper()
	line, err := json.Marshal(map[string]any{
		"schema_version": "agentops-sdlc-provenance.v1",
		"from_id":        fmt.Sprintf("%s@%s", bead, sha[:7]),
		"from_type":      "verdict",
		"to_id":          sha,
		"to_type":        "commit",
		"relation":       "wasDerivedFrom",
		"evidence_ref":   fmt.Sprintf("pawl-verdict %s disposition=%s", bead, disposition),
		"bead_id":        bead,
		"trust_tier":     "inferred",
		"ts":             reportTestNow.Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("marshal ledger line: %v", err)
	}
	ledger := filepath.Join(root, provenanceLedgerRelPath)
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		t.Fatalf("mkdir provenance dir: %v", err)
	}
	f, err := os.OpenFile(ledger, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		t.Fatalf("append ledger: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close ledger: %v", err)
	}
}

// TestYieldReportFrontier_StopsAtUnverdictedCommit is the GWT happy path: a
// CONFIRMED base, an unverdicted non-trivial middle, a CONFIRMED tip. The
// frontier stops at the base — the tip's OWN verdict cannot vouch for its
// unverified ancestor — and the pending window lists every commit above the
// frontier newest-first with (short sha, bead id, age).
func TestYieldReportFrontier_StopsAtUnverdictedCommit(t *testing.T) {
	root := newFrontierRepo(t)
	c1 := frontierCommitFixture(t, root, "feat(api): base slice (age-aaaa)", "", "code.txt", reportTestNow.Add(-6*time.Hour))
	c2 := frontierCommitFixture(t, root, "feat(api): unverified slice (age-bbbb)", "", "code.txt", reportTestNow.Add(-4*time.Hour))
	c3 := frontierCommitFixture(t, root, "feat(api): verified tip (age-cccc)", "", "code.txt", reportTestNow.Add(-2*time.Hour))
	seedProvenanceVerdict(t, root, "age-aaaa", c1, "CONFIRMED")
	seedProvenanceVerdict(t, root, "age-cccc", c3, "CONFIRMED")
	commitLedgerAndPublish(t, root)
	setYieldReportState(t, root)

	doc := decodeReport(t)
	if doc.FrontierError != "" {
		t.Fatalf("frontier error = %q, want none", doc.FrontierError)
	}
	if doc.FrontierSHA != c1 {
		t.Errorf("frontier_sha = %q, want %q (the highest commit whose ancestors ALL resolve)", doc.FrontierSHA, c1)
	}
	// The fixture bind commit (waiver-resolved but above the frontier) leads
	// the window — resolved-above-unresolved commits stay listed, matching the
	// production report.
	if len(doc.Pending) != 3 {
		t.Fatalf("pending = %d rows, want 3 (bind + every commit above the frontier): %+v", len(doc.Pending), doc.Pending)
	}
	if doc.Pending[1].SHA != c3 || doc.Pending[2].SHA != c2 {
		t.Errorf("pending order = [%s %s], want newest-first [%s %s]",
			doc.Pending[1].SHA, doc.Pending[2].SHA, c3, c2)
	}
	if doc.Pending[1].Bead != "age-cccc" || doc.Pending[2].Bead != "age-bbbb" {
		t.Errorf("pending beads = [%q %q], want [age-cccc age-bbbb]", doc.Pending[1].Bead, doc.Pending[2].Bead)
	}
	if doc.Pending[2].Age != "4h" {
		t.Errorf("pending age = %q, want 4h (frozen clock)", doc.Pending[2].Age)
	}
	if doc.Pending[1].Short != shortSHA(c3) {
		t.Errorf("pending short sha = %q, want %q", doc.Pending[1].Short, shortSHA(c3))
	}

	yieldReportJSON = false
	out := runReport(t)
	for _, want := range []string{"VERIFIED FRONTIER", shortSHA(c1), shortSHA(c2), "age-bbbb", "pending"} {
		if !strings.Contains(out, want) {
			t.Errorf("text report missing %q; output:\n%s", want, out)
		}
	}
}

// TestYieldReportFrontier_TrivialWaiverAndEmptyWindow: a #trivial
// provenance-only bind commit at the tip resolves via the waiver arm (no
// verdict edge of its own), so the frontier IS origin/main and the report says
// so with the ✓ line (the GWT empty-window case).
func TestYieldReportFrontier_TrivialWaiverAndEmptyWindow(t *testing.T) {
	root := newFrontierRepo(t)
	c1 := frontierCommitFixture(t, root, "feat(core): real work (age-dddd)", "", "code.txt", reportTestNow.Add(-3*time.Hour))
	// Seed BEFORE the bind commit so c2 IS the production-shape bind: the
	// commit that carries the ledger edge for c1 and itself waives (#trivial,
	// provenance-only). The frontier reads the committed ledger at origin/main.
	seedProvenanceVerdict(t, root, "age-dddd", c1, "CONFIRMED")
	c2 := frontierCommitFixture(t, root,
		"chore(provenance): bind pawl CONFIRMED verdict for age-dddd #trivial", "",
		"docs/provenance/ledger.jsonl", reportTestNow.Add(-1*time.Hour))
	publishOriginMain(t, root)
	setYieldReportState(t, root)

	doc := decodeReport(t)
	if doc.FrontierError != "" {
		t.Fatalf("frontier error = %q, want none", doc.FrontierError)
	}
	if doc.FrontierSHA != c2 {
		t.Errorf("frontier_sha = %q, want tip %q (#trivial provenance-only waives)", doc.FrontierSHA, c2)
	}
	if len(doc.Pending) != 0 {
		t.Errorf("pending = %+v, want empty window", doc.Pending)
	}

	yieldReportJSON = false
	out := runReport(t)
	if !strings.Contains(out, "frontier == origin/main ✓") {
		t.Errorf("empty window must render %q; output:\n%s", "frontier == origin/main ✓", out)
	}
}

// TestYieldReportFrontier_RefutedDominatesWaiver pins the precedence edge: a
// commit that WOULD waive (#trivial marker, provenance-only diff) but carries
// a REFUTED verdict does NOT resolve — refuted evidence beats the author's
// triviality assertion.
func TestYieldReportFrontier_RefutedDominatesWaiver(t *testing.T) {
	root := newFrontierRepo(t)
	c1 := frontierCommitFixture(t, root, "feat(core): base (age-eeee)", "", "code.txt", reportTestNow.Add(-5*time.Hour))
	seedProvenanceVerdict(t, root, "age-eeee", c1, "CONFIRMED")
	c2 := frontierCommitFixture(t, root,
		"chore(provenance): bind pawl verdict for age-ffff #trivial", "",
		"docs/provenance/note.txt", reportTestNow.Add(-1*time.Hour))
	seedProvenanceVerdict(t, root, "age-ffff", c2, "REFUTED")
	commitLedgerAndPublish(t, root)
	setYieldReportState(t, root)

	doc := decodeReport(t)
	if doc.FrontierSHA != c1 {
		t.Errorf("frontier_sha = %q, want %q (REFUTED must dominate the waiver arm)", doc.FrontierSHA, c1)
	}
	if len(doc.Pending) != 2 || doc.Pending[1].SHA != c2 {
		t.Errorf("pending = %+v, want [fixture bind, refuted #trivial commit %s]", doc.Pending, c2)
	}
}

// TestYieldReportFrontier_WorktreeOnlyEvidenceDoesNotCount pins the age-fdae
// refute-fix: a CONFIRMED edge that exists only in the WORKTREE ledger (not
// committed on origin/main) must not certify a published commit — the
// frontier holds and the commit stays pending.
func TestYieldReportFrontier_WorktreeOnlyEvidenceDoesNotCount(t *testing.T) {
	root := newFrontierRepo(t)
	c1 := frontierCommitFixture(t, root, "feat(core): slice (age-gggg)", "", "code.txt", reportTestNow.Add(-2*time.Hour))
	publishOriginMain(t, root) // published WITHOUT the ledger edge
	seedProvenanceVerdict(t, root, "age-gggg", c1, "CONFIRMED") // worktree-only
	setYieldReportState(t, root)

	doc := decodeReport(t)
	if doc.FrontierSHA == c1 {
		t.Errorf("frontier_sha = %q — worktree-only evidence certified a published commit (the exact refuted fail-open)", doc.FrontierSHA)
	}
	if len(doc.Pending) != 1 || doc.Pending[0].SHA != c1 {
		t.Errorf("pending = %+v, want exactly the uncertified commit %s", doc.Pending, c1)
	}
}

// TestYieldReportFrontier_DegradesWithoutOriginMain: a root that is not a git
// repo (or has no origin/main) reports frontier_error and still renders the
// rest of the report — degraded, never fatal, matching BeadsError semantics.
func TestYieldReportFrontier_DegradesWithoutOriginMain(t *testing.T) {
	root := t.TempDir() // deliberately not a git repo
	setYieldReportState(t, root)

	doc := decodeReport(t)
	if doc.FrontierError == "" {
		t.Errorf("frontier_error must report the degraded walk, got empty")
	}
	if doc.FrontierSHA != "" {
		t.Errorf("frontier_sha = %q, want empty on degraded", doc.FrontierSHA)
	}
	if doc.Pending == nil {
		t.Errorf("pending must be [] not null on degraded")
	}

	yieldReportJSON = false
	out := runReport(t)
	if !strings.Contains(out, "frontier unavailable") {
		t.Errorf("text report must carry the degraded frontier notice; output:\n%s", out)
	}
	if !strings.Contains(out, "ANDON QUEUE") {
		t.Errorf("degraded frontier must not suppress the rest of the report; output:\n%s", out)
	}
}

// TestHasTrivialMarker ports the age-w2ny marker grammar cases from
// scripts/lib/trivial-waiver.sh: #trivial waives only as a TRAILING subject
// tag or a standalone body line — never a prose mention (the historical
// fail-open where naming #trivial mid-subject bypassed the pawl).
func TestHasTrivialMarker(t *testing.T) {
	cases := []struct {
		name    string
		subject string
		body    string
		want    bool
	}{
		{name: "trailing subject tag", subject: "chore(provenance): bind verdict #trivial", want: true},
		{name: "case-insensitive", subject: "chore(provenance): bind verdict #TRIVIAL", want: true},
		{name: "mid-subject prose mention", subject: "fix(pawl): prevent #trivial from bypassing pawl", want: false},
		{name: "standalone body line", subject: "feat: thing", body: "resolved.\n\n#trivial\n", want: true},
		{name: "indented body line", subject: "feat: thing", body: "  #trivial  ", want: true},
		{name: "body prose mention", subject: "feat: thing", body: "this mentions #trivial in prose", want: false},
		{name: "no marker", subject: "feat: thing", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasTrivialMarker(tc.subject, tc.body); got != tc.want {
				t.Errorf("hasTrivialMarker(%q, %q) = %v, want %v", tc.subject, tc.body, got, tc.want)
			}
		})
	}
}

// TestResolveCommitToday pins the arm precedence of the RESOLVED-today seam:
// CONFIRMED resolves outright (even alongside an earlier REFUTED — re-review
// supersedes); the waiver arm resolves only absent ANY refute; otherwise
// unresolved.
func TestResolveCommitToday(t *testing.T) {
	cases := []struct {
		name      string
		confirmed bool
		refuted   bool
		waivable  bool
		want      frontierArm
	}{
		{name: "confirmed", confirmed: true, want: frontierArmConfirmed},
		{name: "confirmed after refute (re-review)", confirmed: true, refuted: true, want: frontierArmConfirmed},
		{name: "waiver", waivable: true, want: frontierArmWaiver},
		{name: "refuted dominates waiver", refuted: true, waivable: true, want: frontierArmNone},
		{name: "nothing", want: frontierArmNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCommitToday(frontierCommit{SHA: "abc"},
				commitVerdicts{Confirmed: tc.confirmed, Refuted: tc.refuted},
				func(frontierCommit) bool { return tc.waivable })
			if got != tc.want {
				t.Errorf("resolveCommitToday(confirmed=%v refuted=%v waivable=%v) = %q, want %q",
					tc.confirmed, tc.refuted, tc.waivable, got, tc.want)
			}
		})
	}
}

// TestBeadIDFromSubject pins the pending-window bead extraction: the trailing
// parenthesized id convention, the bind-commit "for <bead>" form, and honest
// blanks — a conventional-commit scope like "(yield)" is never an id.
func TestBeadIDFromSubject(t *testing.T) {
	cases := []struct {
		subject string
		want    string
	}{
		{subject: "feat(yield): verified frontier (age-fdae)", want: "age-fdae"},
		{subject: "feat(x): long epic slice (age-verification-economics-ebec.1)", want: "age-verification-economics-ebec.1"},
		{subject: "chore(provenance): bind pawl CONFIRMED verdict for age-mv67 #trivial", want: "age-mv67"},
		{subject: "fix: dotted child close (soc-y8b.8.2)", want: "soc-y8b.8.2"},
		{subject: "docs: no bead here", want: ""},
		{subject: "feat(scope): scope is not a bead", want: ""},
	}
	for _, tc := range cases {
		if got := beadIDFromSubject(tc.subject); got != tc.want {
			t.Errorf("beadIDFromSubject(%q) = %q, want %q", tc.subject, got, tc.want)
		}
	}
}

// TestComputeFrontier is the pure L1 over the walk math: newest-first input,
// frontier = the commit below the OLDEST unresolved one, pending = everything
// above it (resolved commits above an unresolved ancestor are still pending —
// their LKG status is unreachable).
func TestComputeFrontier(t *testing.T) {
	mk := func(shas ...string) []frontierCommit {
		out := make([]frontierCommit, 0, len(shas))
		for _, s := range shas {
			out = append(out, frontierCommit{SHA: s})
		}
		return out
	}
	resolver := func(resolved map[string]bool) func(frontierCommit) frontierArm {
		return func(c frontierCommit) frontierArm {
			if resolved[c.SHA] {
				return frontierArmConfirmed
			}
			return frontierArmNone
		}
	}
	pendingSHAs := func(pending []frontierCommit) []string {
		out := make([]string, 0, len(pending))
		for _, c := range pending {
			out = append(out, c.SHA)
		}
		return out
	}

	cases := []struct {
		name         string
		commits      []frontierCommit
		resolved     map[string]bool
		wantFrontier string
		wantPending  []string
	}{
		{
			name: "all resolved — frontier is the tip", commits: mk("t", "m", "b"),
			resolved: map[string]bool{"t": true, "m": true, "b": true},
			wantFrontier: "t", wantPending: []string{},
		},
		{
			name: "unresolved middle — resolved tip still pending", commits: mk("t", "m", "b"),
			resolved: map[string]bool{"t": true, "b": true},
			wantFrontier: "b", wantPending: []string{"t", "m"},
		},
		{
			name: "oldest walked commit unresolved — no frontier in window", commits: mk("t", "m", "b"),
			resolved: map[string]bool{"t": true, "m": true},
			wantFrontier: "", wantPending: []string{"t", "m", "b"},
		},
		{
			name: "empty walk", commits: nil,
			wantFrontier: "", wantPending: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frontier, pending := computeFrontier(tc.commits, resolver(tc.resolved))
			if frontier != tc.wantFrontier {
				t.Errorf("frontier = %q, want %q", frontier, tc.wantFrontier)
			}
			got := pendingSHAs(pending)
			if len(got) != len(tc.wantPending) {
				t.Fatalf("pending = %v, want %v", got, tc.wantPending)
			}
			for i := range got {
				if got[i] != tc.wantPending[i] {
					t.Errorf("pending[%d] = %q, want %q", i, got[i], tc.wantPending[i])
				}
			}
		})
	}
}

// TestTrivialWaiverDiffOK pins the ported age-u43w diff arm against a real
// repo: provenance-only diffs waive; any non-provenance path refuses; a bogus
// sha fails closed.
func TestTrivialWaiverDiffOK(t *testing.T) {
	root := newFrontierRepo(t)
	// Base commit first: diff-tree (matching scripts/lib/trivial-waiver.sh, no
	// --root) yields an empty list for a ROOT commit and fail-closes — correct
	// waiver semantics, but a provenance commit as the repo root is a shape
	// production never produces (fixture-fidelity rule).
	frontierCommitFixture(t, root, "chore: base", "", "code.txt", reportTestNow.Add(-3*time.Hour))
	prov := frontierCommitFixture(t, root, "chore(provenance): bind #trivial", "",
		"docs/provenance/ledger.jsonl", reportTestNow.Add(-2*time.Hour))
	mixed := frontierCommitFixture(t, root, "chore(provenance): sneaky #trivial", "",
		"cli/code.go", reportTestNow.Add(-1*time.Hour))

	if !trivialWaiverDiffOK(root, prov) {
		t.Errorf("provenance-only diff at %s must waive", shortSHA(prov))
	}
	if trivialWaiverDiffOK(root, mixed) {
		t.Errorf("non-provenance diff at %s must NOT waive", shortSHA(mixed))
	}
	if trivialWaiverDiffOK(root, "0000000000000000000000000000000000000000") {
		t.Errorf("unresolvable sha must fail closed, not waive")
	}
}
