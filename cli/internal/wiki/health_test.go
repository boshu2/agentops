package wiki_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/llmwiki"
	"github.com/boshu2/agentops/cli/internal/wiki"
)

// compileFixtureWorkspace stands up a real OpenKB workspace, seeds the given
// distilled sources, and runs the real CompileStage so the health checks operate
// on the EXACT persisted artifact shape (frontmatter + [[wikilinks]]) production
// emits — never a hand-built string a writer would not produce. Returns the
// workspace root.
func compileFixtureWorkspace(t *testing.T, rawSources map[string]string) string {
	t.Helper()
	ws := t.TempDir()
	if _, err := wiki.Scaffold(ws, wiki.DefaultScaffoldConfig("test-model", "en")); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	for slug, body := range rawSources {
		dest := filepath.Join(ws, "wiki", "sources", slug+".md")
		if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			t.Fatalf("mkdir sources: %v", err)
		}
		if err := os.WriteFile(dest, []byte(body), 0o600); err != nil {
			t.Fatalf("write source %s: %v", slug, err)
		}
	}
	stage := &llmwiki.CompileStage{
		Now:      func() time.Time { return time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC) },
		Compiler: llmwiki.NewDeterministicCompiler(),
	}
	if _, err := stage.Run(context.Background(), ws, 1); err != nil {
		t.Fatalf("compile: %v", err)
	}
	return ws
}

// TestCheckWiki_CleanWorkspace asserts a freshly-compiled, internally-consistent
// workspace lints CLEAN — no defects of any class. This is the negative control:
// a real persisted wiki must not trip a false positive.
func TestCheckWiki_CleanWorkspace(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "Alpha source about widgets and gears.\n",
		"beta":  "Beta source about gears and sprockets.\n",
	})

	report, err := wiki.CheckWiki(ws)
	if err != nil {
		t.Fatalf("CheckWiki: %v", err)
	}
	if report.PageCount == 0 {
		t.Fatalf("expected pages to be discovered, got 0")
	}
	if got := len(report.Defects); got != 0 {
		t.Fatalf("expected 0 defects on a clean workspace, got %d: %+v", got, report.Defects)
	}
	if report.HasBlocking() {
		t.Fatalf("clean workspace must not report blocking defects")
	}
}

// TestCheckWiki_BrokenWikilink asserts a [[target]] whose page does not exist
// FIRES a broken-link defect — and only that defect.
func TestCheckWiki_BrokenWikilink(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "Alpha source about widgets.\n",
	})
	summary := filepath.Join(ws, "wiki", "summaries", "alpha.md")
	appendToFile(t, summary, "\nSee also [[concepts/nonexistent-deadbeefdeadbeef]].\n")

	report, err := wiki.CheckWiki(ws)
	if err != nil {
		t.Fatalf("CheckWiki: %v", err)
	}
	got := defectsOfKind(report, wiki.DefectBrokenLink)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 broken-link defect, got %d: %+v", len(got), report.Defects)
	}
	if got[0].Detail != "concepts/nonexistent-deadbeefdeadbeef" {
		t.Fatalf("expected detail to name the dangling target, got %q", got[0].Detail)
	}
	if !got[0].Blocking {
		t.Fatalf("a broken wikilink is a blocking defect")
	}
}

// TestCheckWiki_MissingFrontmatter asserts a page with NO frontmatter block
// fires an invalid-frontmatter defect.
func TestCheckWiki_MissingFrontmatter(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "# Widgets\n\nAlpha source about widgets.\n",
	})
	page := firstPageIn(t, filepath.Join(ws, "wiki", "concepts"))
	if err := os.WriteFile(page, []byte("# Orphaned concept\n\nNo frontmatter here.\n"), 0o600); err != nil {
		t.Fatalf("rewrite page: %v", err)
	}

	report, err := wiki.CheckWiki(ws)
	if err != nil {
		t.Fatalf("CheckWiki: %v", err)
	}
	got := defectsOfKind(report, wiki.DefectInvalidFrontmatter)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 invalid-frontmatter defect, got %d: %+v", len(got), report.Defects)
	}
	if !got[0].Blocking {
		t.Fatalf("invalid frontmatter is a blocking defect")
	}
}

// TestCheckWiki_MissingRequiredField asserts a page whose frontmatter parses but
// lacks the required `type` field fires a missing-field defect (not an
// invalid-frontmatter defect — the block is well-formed, just incomplete).
func TestCheckWiki_MissingRequiredField(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "# Widgets\n\nAlpha source about widgets.\n",
	})
	page := firstPageIn(t, filepath.Join(ws, "wiki", "concepts"))
	body := "---\nstage: compile\nattempt: 1\n---\n\n# Concept\n\nBody.\n"
	if err := os.WriteFile(page, []byte(body), 0o600); err != nil {
		t.Fatalf("rewrite page: %v", err)
	}

	report, err := wiki.CheckWiki(ws)
	if err != nil {
		t.Fatalf("CheckWiki: %v", err)
	}
	got := defectsOfKind(report, wiki.DefectMissingField)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 missing-field defect, got %d: %+v", len(got), report.Defects)
	}
	if got[0].Detail != "type" {
		t.Fatalf("expected the missing field to be 'type', got %q", got[0].Detail)
	}
	if defectsOfKind(report, wiki.DefectInvalidFrontmatter) != nil {
		t.Fatalf("a well-formed but incomplete block is NOT invalid frontmatter")
	}
}

// TestCheckWiki_OrphanPage asserts a valid page that no other page links to
// (and which is not the index) fires an orphan defect — non-blocking.
func TestCheckWiki_OrphanPage(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "Alpha source about widgets.\n",
	})
	orphan := filepath.Join(ws, "wiki", "concepts", "lonely-aaaaaaaaaaaaaaaa.md")
	body := "---\ntype: concept\nstage: compile\nattempt: 1\n---\n\n# Lonely\n\nNobody links here.\n"
	if err := os.WriteFile(orphan, []byte(body), 0o600); err != nil {
		t.Fatalf("write orphan: %v", err)
	}

	report, err := wiki.CheckWiki(ws)
	if err != nil {
		t.Fatalf("CheckWiki: %v", err)
	}
	got := defectsOfKind(report, wiki.DefectOrphan)
	found := false
	for _, d := range got {
		if d.Page == "concepts/lonely-aaaaaaaaaaaaaaaa" {
			found = true
			if d.Blocking {
				t.Fatalf("an orphan page is a non-blocking defect")
			}
		}
	}
	if !found {
		t.Fatalf("expected the unlinked page to be reported as an orphan, got %+v", got)
	}
}

// TestCheckWiki_IndexNeverOrphan asserts index.md and log.md are excluded from
// the orphan check (nothing links to the index by design).
func TestCheckWiki_IndexNeverOrphan(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "Alpha source about widgets.\n",
	})
	report, err := wiki.CheckWiki(ws)
	if err != nil {
		t.Fatalf("CheckWiki: %v", err)
	}
	for _, d := range report.Defects {
		if d.Kind == wiki.DefectOrphan && (d.Page == "index" || d.Page == "log") {
			t.Fatalf("index/log must be excluded from the orphan check, got %+v", d)
		}
	}
}

// TestListPages_EnumeratesByType asserts ListPages returns every page with its
// type and supports a type filter.
func TestListPages_EnumeratesByType(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "# Gears\n\nAlpha about gears and widgets.\n",
	})
	all, err := wiki.ListPages(ws, "")
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	if len(all) == 0 {
		t.Fatalf("expected pages, got 0")
	}
	concepts, err := wiki.ListPages(ws, "concept")
	if err != nil {
		t.Fatalf("ListPages(concept): %v", err)
	}
	if len(concepts) == 0 {
		t.Fatalf("expected at least one concept page")
	}
	for _, p := range concepts {
		if p.Type != "concept" {
			t.Fatalf("type filter leaked a %q page: %+v", p.Type, p)
		}
	}
	if len(concepts) >= len(all) {
		t.Fatalf("filtered list (%d) should be a strict subset of all (%d)", len(concepts), len(all))
	}
}

// TestStatus_CountsAndHealth asserts Status reports per-type counts and a defect
// summary consistent with CheckWiki.
func TestStatus_CountsAndHealth(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "Alpha about gears.\n",
		"beta":  "Beta about sprockets.\n",
	})
	st, err := wiki.Status(ws)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.TotalPages == 0 {
		t.Fatalf("expected non-zero total pages")
	}
	if st.ByType["summary"] != 2 {
		t.Fatalf("expected 2 summary pages, got %d", st.ByType["summary"])
	}
	if st.DefectCount != 0 {
		t.Fatalf("a clean workspace should report 0 defects, got %d", st.DefectCount)
	}
}

// --- helpers ---

func appendToFile(t *testing.T, path, extra string) {
	t.Helper()
	existing, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(existing, []byte(extra)...), 0o600); err != nil {
		t.Fatalf("append %s: %v", path, err)
	}
}

func firstPageIn(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		t.Fatalf("no .md pages in %s", dir)
	}
	sort.Strings(names)
	return filepath.Join(dir, names[0])
}

func defectsOfKind(r wiki.WikiHealthReport, kind wiki.DefectKind) []wiki.WikiDefect {
	var out []wiki.WikiDefect
	for _, d := range r.Defects {
		if d.Kind == kind {
			out = append(out, d)
		}
	}
	return out
}
