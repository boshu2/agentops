package wiki_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/wiki"
)

// TestFixWiki_StripsBrokenInlineLink asserts --fix removes a dangling inline
// [[target]] reference, leaving the surrounding prose intact, and reports the
// repair. The page itself is preserved (no page is ever deleted).
func TestFixWiki_StripsBrokenInlineLink(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "Alpha source about widgets.\n",
	})
	summary := filepath.Join(ws, "wiki", "summaries", "alpha.md")
	appendToFile(t, summary, "\nSee also [[concepts/ghost-deadbeefdeadbeef]] for more.\n")

	// Pre-condition: the broken link is a real defect.
	pre, err := wiki.CheckWiki(ws)
	if err != nil {
		t.Fatalf("pre CheckWiki: %v", err)
	}
	if len(defectsOfKind(pre, wiki.DefectBrokenLink)) != 1 {
		t.Fatalf("expected 1 broken link before fix, got %+v", pre.Defects)
	}

	result, err := wiki.FixWiki(ws)
	if err != nil {
		t.Fatalf("FixWiki: %v", err)
	}
	if result.LinksStripped != 1 {
		t.Fatalf("expected 1 link stripped, got %d", result.LinksStripped)
	}

	// Post-condition: the dangling token is gone, the prose around it remains,
	// and the page still exists.
	got := readFile(t, summary)
	if strings.Contains(got, "[[concepts/ghost-deadbeefdeadbeef]]") {
		t.Fatalf("broken link not stripped:\n%s", got)
	}
	if !strings.Contains(got, "See also") || !strings.Contains(got, "for more.") {
		t.Fatalf("surrounding prose was damaged:\n%s", got)
	}
	if !strings.Contains(got, "# Summary: alpha") {
		t.Fatalf("page content was lost (no page is ever deleted):\n%s", got)
	}

	// And the wiki is now clean of broken links.
	post, err := wiki.CheckWiki(ws)
	if err != nil {
		t.Fatalf("post CheckWiki: %v", err)
	}
	if len(defectsOfKind(post, wiki.DefectBrokenLink)) != 0 {
		t.Fatalf("expected 0 broken links after fix, got %+v", post.Defects)
	}
}

// TestFixWiki_PreservesUnrelatedWhitespace pins the data-safety property a
// global whitespace-collapse violated: stripping a dangling inline link must
// close ONLY the gap it occupied and leave all other spacing (intentional
// alignment, multi-space runs elsewhere on the line) byte-for-byte intact.
func TestFixWiki_PreservesUnrelatedWhitespace(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "Alpha source.\n",
	})
	summary := filepath.Join(ws, "wiki", "summaries", "alpha.md")
	// Intentional whitespace far from the link: interior double-spaces AND
	// trailing spaces — both must survive untouched.
	appendToFile(t, summary, "\nTable  cell    spacing before [[concepts/missing-deadbeefdeadbeef]] after.   \n")

	if _, err := wiki.FixWiki(ws); err != nil {
		t.Fatalf("FixWiki: %v", err)
	}
	got := readFile(t, summary)
	// Only the link plus its own single gap are removed; the interior
	// "Table  cell    spacing" double-spaces and the trailing "   " stay intact.
	want := "Table  cell    spacing before after.   "
	if !strings.Contains(got, want) {
		t.Fatalf("unrelated whitespace was mangled; want line %q in:\n%s", want, got)
	}
	if strings.Contains(got, "[[concepts/missing-deadbeefdeadbeef]]") {
		t.Fatalf("broken link not stripped:\n%s", got)
	}
}

// TestFixWiki_StripsBrokenBulletLine asserts --fix removes the WHOLE list-item
// line when a bullet's only content is a dangling wikilink (a ghost entry in a
// generated "## Entities" / "## Concepts" list), not just the token — leaving a
// dangling empty bullet would be a different defect.
func TestFixWiki_StripsBrokenBulletLine(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "Alpha source about widgets.\n",
	})
	summary := filepath.Join(ws, "wiki", "summaries", "alpha.md")
	appendToFile(t, summary, "- [[entities/ghost-cafecafecafecafe]]\n")

	result, err := wiki.FixWiki(ws)
	if err != nil {
		t.Fatalf("FixWiki: %v", err)
	}
	if result.LinksStripped != 1 {
		t.Fatalf("expected 1 link stripped, got %d", result.LinksStripped)
	}
	got := readFile(t, summary)
	if strings.Contains(got, "ghost-cafecafecafecafe") {
		t.Fatalf("ghost bullet not removed:\n%s", got)
	}
	// No empty "- " bullet left behind.
	for _, line := range strings.Split(got, "\n") {
		if strings.TrimSpace(line) == "-" || strings.TrimSpace(line) == "- " {
			t.Fatalf("an empty bullet was left behind:\n%s", got)
		}
	}
}

// TestFixWiki_PreservesValidLinks asserts --fix is surgical: a VALID wikilink in
// the same page is untouched. Only dangling targets are stripped.
func TestFixWiki_PreservesValidLinks(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "Alpha source about widgets.\n",
	})
	summary := filepath.Join(ws, "wiki", "summaries", "alpha.md")
	before := readFile(t, summary)
	// summaries/alpha already links a real [[entities/alpha-...]] and
	// [[sources/alpha]]; add one broken link alongside.
	appendToFile(t, summary, "\nAlso [[concepts/ghost-deadbeefdeadbeef]].\n")

	if _, err := wiki.FixWiki(ws); err != nil {
		t.Fatalf("FixWiki: %v", err)
	}
	got := readFile(t, summary)
	// Every valid link present before the broken-link injection must survive.
	for _, link := range wiki.ExtractWikilinks(before) {
		if !strings.Contains(got, "[["+link+"]]") {
			t.Fatalf("a valid link %q was stripped:\n%s", link, got)
		}
	}
}

// TestFixWiki_Idempotent asserts running --fix twice produces the same bytes:
// the second run finds nothing to repair and changes nothing.
func TestFixWiki_Idempotent(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "Alpha source about widgets.\n",
	})
	summary := filepath.Join(ws, "wiki", "summaries", "alpha.md")
	appendToFile(t, summary, "\nSee [[concepts/ghost-deadbeefdeadbeef]].\n")

	r1, err := wiki.FixWiki(ws)
	if err != nil {
		t.Fatalf("FixWiki #1: %v", err)
	}
	if r1.LinksStripped != 1 {
		t.Fatalf("first run should strip 1, got %d", r1.LinksStripped)
	}
	after1 := readFile(t, summary)

	r2, err := wiki.FixWiki(ws)
	if err != nil {
		t.Fatalf("FixWiki #2: %v", err)
	}
	if r2.LinksStripped != 0 {
		t.Fatalf("second run should be a no-op, stripped %d", r2.LinksStripped)
	}
	after2 := readFile(t, summary)
	if after1 != after2 {
		t.Fatalf("fix is not idempotent:\nrun1:\n%s\nrun2:\n%s", after1, after2)
	}
}

// TestFixWiki_NeverDeletesPages asserts no page file is removed by --fix, even a
// fully-orphaned one — orphans are reported by lint but never auto-repaired
// (deletion is lossy).
func TestFixWiki_NeverDeletesPages(t *testing.T) {
	ws := compileFixtureWorkspace(t, map[string]string{
		"alpha": "Alpha source about widgets.\n",
	})
	orphan := filepath.Join(ws, "wiki", "concepts", "lonely-bbbbbbbbbbbbbbbb.md")
	body := "---\ntype: concept\nstage: compile\nattempt: 1\n---\n\n# Lonely\n\nNobody links here.\n"
	if err := os.WriteFile(orphan, []byte(body), 0o600); err != nil {
		t.Fatalf("write orphan: %v", err)
	}

	if _, err := wiki.FixWiki(ws); err != nil {
		t.Fatalf("FixWiki: %v", err)
	}
	if _, err := os.Stat(orphan); err != nil {
		t.Fatalf("orphan page must NOT be deleted by --fix: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
