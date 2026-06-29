package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/storage"
)

// TestRecallProjectRoot_WalksUpToAgentsCorpus verifies recall resolves the project
// tier from the repo root, not cwd: from a subdirectory it climbs to the nearest
// ancestor holding a .agents/ corpus (so `ao recall` works from anywhere in a repo).
// When no ancestor has one, it falls back to the start dir (no false project root).
func TestRecallProjectRoot_WalksUpToAgentsCorpus(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agents", "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "cli", "cmd", "ao")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := recallProjectRoot(sub); got != repo {
		t.Errorf("recallProjectRoot(subdir) = %q, want repo root %q", got, repo)
	}

	// No .agents/ ancestor: falls back to the start dir (does not climb forever).
	bare := t.TempDir()
	if got := recallProjectRoot(bare); got != bare {
		t.Errorf("recallProjectRoot(no-corpus) = %q, want start %q", got, bare)
	}
}

// TestRecallProjectRoot_NeverClimbsIntoHome verifies the machine hub is never
// mis-resolved as a project root: a repo/subdir under $HOME without its own
// .agents/ falls back to the start dir, NOT $HOME (whose ~/.agents is the machine
// tier). Reproduces the cross-family pawl's home-climb defect.
func TestRecallProjectRoot_NeverClimbsIntoHome(t *testing.T) {
	home := t.TempDir()
	// home has the machine hub corpus...
	if err := os.MkdirAll(filepath.Join(home, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	// ...and a repo under it with NO .agents of its own.
	repoUnderHome := filepath.Join(home, "dev", "somerepo")
	if err := os.MkdirAll(repoUnderHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := recallProjectRootFrom(repoUnderHome, home); got != repoUnderHome {
		t.Errorf("recallProjectRootFrom(under-home-no-corpus) = %q, want start %q (must not climb to home)", got, repoUnderHome)
	}
	// A real repo under home WITH its own .agents is still found.
	realRepo := filepath.Join(home, "dev", "withcorpus")
	if err := os.MkdirAll(filepath.Join(realRepo, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := recallProjectRootFrom(filepath.Join(realRepo, "sub"), home); got != realRepo {
		t.Errorf("recallProjectRootFrom(real-repo-under-home) = %q, want %q", got, realRepo)
	}
}

// TestRecall_CommandContract asserts the recall command is wired to its contract:
// the unified memory front door takes exactly one query, lives in the knowledge
// group, and defaults to a 10-result limit.
func TestRecall_CommandContract(t *testing.T) {
	if recallCmd.Use != "recall <query>" {
		t.Errorf("Use = %q, want %q", recallCmd.Use, "recall <query>")
	}
	if recallCmd.GroupID != "knowledge" {
		t.Errorf("GroupID = %q, want %q", recallCmd.GroupID, "knowledge")
	}
	// Exactly one positional arg (the query).
	if err := recallCmd.Args(recallCmd, []string{"q"}); err != nil {
		t.Errorf("Args rejected a single query: %v", err)
	}
	if err := recallCmd.Args(recallCmd, []string{"a", "b"}); err == nil {
		t.Error("Args accepted two positional args; want ExactArgs(1)")
	}
	limit := recallCmd.Flags().Lookup("limit")
	if limit == nil {
		t.Fatal("--limit flag not registered")
	}
	if limit.DefValue != "10" {
		t.Errorf("--limit default = %q, want %q", limit.DefValue, "10")
	}
}

// TestSearchRepoCuratedKnowledge_RanksRelevantFirst is a behavioral retrieval test
// (not just wiring): over a real temp corpus, the curated search must return the
// query-relevant learning ranked above an irrelevant one, score-ordered and
// de-duped. Guards the round-10 regression where the curated path dropped its
// rankUniqueSearchResults and returned per-category append order.
func TestSearchRepoCuratedKnowledge_RanksRelevantFirst(t *testing.T) {
	root := t.TempDir()
	learnings := filepath.Join(root, ".agents", "learnings")
	if err := os.MkdirAll(learnings, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		fm := "---\nid: " + name + "\ntype: learning\nmaturity: provisional\n---\n\n# Learning: " + name + "\n\n" + body + "\n"
		if err := os.WriteFile(filepath.Join(learnings, name+".md"), []byte(fm), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Relevant doc mentions the query term repeatedly; the decoy never does.
	write("relevant", "widget widget widget calibration of the widget subsystem widget")
	write("decoy", "entirely unrelated content about gardening and weather")

	sessionsDir := filepath.Join(root, storage.DefaultBaseDir, storage.SessionsDir)
	results := searchRepoCuratedKnowledge("widget", sessionsDir, 10)
	if len(results) == 0 {
		t.Fatal("curated search returned no results for a present term")
	}
	if !strings.Contains(results[0].Path, "relevant") {
		t.Errorf("top result = %q, want the relevant memory ranked first", results[0].Path)
	}
	// De-dupe: no path appears twice.
	seen := map[string]bool{}
	for _, r := range results {
		if seen[r.Path] {
			t.Errorf("duplicate path in ranked results: %s", r.Path)
		}
		seen[r.Path] = true
	}
}

// TestCorpusRelativeKey_DedupesAcrossTiers verifies the de-dupe key collapses the
// SAME logical memory harvested into both tiers (repo/.agents/... vs ~/.agents/...)
// to one key, while keeping genuinely different relative paths distinct.
func TestCorpusRelativeKey_DedupesAcrossTiers(t *testing.T) {
	proj := filepath.Join("/Users/bo/dev/agentops", ".agents", "learnings", "x.md")
	mach := filepath.Join("/Users/bo", ".agents", "learnings", "x.md")
	if corpusRelativeKey(proj) != corpusRelativeKey(mach) {
		t.Errorf("same logical memory across tiers got different keys: %q vs %q",
			corpusRelativeKey(proj), corpusRelativeKey(mach))
	}
	other := filepath.Join("/Users/bo", ".agents", "council", "y.md")
	if corpusRelativeKey(proj) == corpusRelativeKey(other) {
		t.Error("distinct memories collapsed to the same key")
	}
	// No .agents/ segment: falls back to the full path (never panics/empties).
	if got := corpusRelativeKey("/tmp/loose.md"); got != "/tmp/loose.md" {
		t.Errorf("fallback key = %q, want full path", got)
	}
}

// TestRecall_RejectsNonPositiveLimit guards the slice bound: a non-positive
// --limit must return a CLI error, not panic on hits[:recallLimit] with a
// negative bound.
func TestRecall_RejectsNonPositiveLimit(t *testing.T) {
	for _, bad := range []int{0, -1, -10} {
		prev := recallLimit
		recallLimit = bad
		err := runRecall(&cobra.Command{}, []string{"q"})
		recallLimit = prev
		if err == nil {
			t.Errorf("--limit %d: want error, got nil", bad)
		}
	}
}

// TestRecall_DryRun verifies the dry-run path prints the intended query and makes
// no retrieval calls.
func TestRecall_DryRun(t *testing.T) {
	prev := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = prev })

	out, err := captureStdout(t, func() error {
		return runRecall(&cobra.Command{}, []string{"why we rejected graphs"})
	})
	if err != nil {
		t.Fatalf("runRecall dry-run returned error: %v", err)
	}

	if !strings.Contains(out, "[dry-run] Would recall: why we rejected graphs") {
		t.Errorf("dry-run output missing expected line; got: %q", out)
	}
}
