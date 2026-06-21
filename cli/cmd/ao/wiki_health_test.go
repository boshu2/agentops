// practices: [tdd, pragmatic-programmer]
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/llmwiki"
	"github.com/boshu2/agentops/cli/internal/wiki"
)

// buildWikiWorkspace scaffolds a real OpenKB workspace, seeds distilled sources,
// and runs the real compile stage so the command-level tests drive over the
// EXACT persisted artifact shape production emits. Returns the workspace root.
func buildWikiWorkspace(t *testing.T, sources map[string]string) string {
	t.Helper()
	ws := t.TempDir()
	if _, err := wiki.Scaffold(ws, wiki.DefaultScaffoldConfig("test-model", "en")); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	for slug, body := range sources {
		dest := filepath.Join(ws, "wiki", "sources", slug+".md")
		if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			t.Fatalf("mkdir sources: %v", err)
		}
		if err := os.WriteFile(dest, []byte(body), 0o600); err != nil {
			t.Fatalf("write source: %v", err)
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

func TestWikiListCommand_EnumeratesPagesByType(t *testing.T) {
	ws := buildWikiWorkspace(t, map[string]string{
		"alpha": "# Gears\n\nAlpha about gears and widgets.\n",
	})

	out, err := captureStdout(t, func() error {
		wikiListCmd.Flags().Set("path", ws)        //nolint:errcheck // test setup
		wikiListCmd.Flags().Set("type", "concept") //nolint:errcheck // test setup
		defer wikiListCmd.Flags().Set("path", "")  //nolint:errcheck // test setup
		defer wikiListCmd.Flags().Set("type", "")  //nolint:errcheck // test setup
		return runWikiList(wikiListCmd, nil)
	})
	if err != nil {
		t.Fatalf("runWikiList returned error: %v", err)
	}
	if !strings.Contains(out, "concept") {
		t.Fatalf("expected a concept page in the filtered list, got: %q", out)
	}
	if strings.Contains(out, "summaries/") {
		t.Fatalf("type=concept filter leaked a summary page: %q", out)
	}
}

func TestWikiStatusCommand_ReportsCountsAndCleanHealth(t *testing.T) {
	ws := buildWikiWorkspace(t, map[string]string{
		"alpha": "Alpha about gears.\n",
		"beta":  "Beta about sprockets.\n",
	})

	out, err := captureStdout(t, func() error {
		wikiStatusCmd.Flags().Set("path", ws)       //nolint:errcheck // test setup
		defer wikiStatusCmd.Flags().Set("path", "") //nolint:errcheck // test setup
		return runWikiStatus(wikiStatusCmd, nil)
	})
	if err != nil {
		t.Fatalf("runWikiStatus returned error: %v", err)
	}
	if !strings.Contains(out, "summary    2") {
		t.Fatalf("expected 2 summary pages in status, got: %q", out)
	}
	if !strings.Contains(out, "defects     : 0") {
		t.Fatalf("expected 0 defects on a clean workspace, got: %q", out)
	}
}

func TestWikiLintCommand_CleanWorkspaceExitsZero(t *testing.T) {
	ws := buildWikiWorkspace(t, map[string]string{
		"alpha": "Alpha about gears.\n",
	})

	out, err := captureStdout(t, func() error {
		wikiLintCmd.Flags().Set("path", ws)       //nolint:errcheck // test setup
		defer wikiLintCmd.Flags().Set("path", "") //nolint:errcheck // test setup
		return runWikiLint(wikiLintCmd, nil)
	})
	if err != nil {
		t.Fatalf("clean workspace must lint without error, got: %v", err)
	}
	if !strings.Contains(out, "0 defect") {
		t.Fatalf("expected zero defects, got: %q", out)
	}
}

func TestWikiLintCommand_BlockingDefectExitsNonZero(t *testing.T) {
	ws := buildWikiWorkspace(t, map[string]string{
		"alpha": "Alpha about gears.\n",
	})
	// Inject a dangling wikilink into a real page.
	summary := filepath.Join(ws, "wiki", "summaries", "alpha.md")
	appendFileForTest(t, summary, "\nSee [[concepts/ghost-deadbeefdeadbeef]].\n")

	out, err := captureStdout(t, func() error {
		wikiLintCmd.Flags().Set("path", ws)       //nolint:errcheck // test setup
		defer wikiLintCmd.Flags().Set("path", "") //nolint:errcheck // test setup
		return runWikiLint(wikiLintCmd, nil)
	})
	// A blocking defect returns a wikiHealthExitError with code 1.
	healthErr, ok := err.(*wikiHealthExitError)
	if !ok {
		t.Fatalf("expected *wikiHealthExitError, got %T (%v)", err, err)
	}
	if healthErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d", healthErr.ExitCode())
	}
	if !strings.Contains(out, "broken wikilink") {
		t.Fatalf("expected the broken-link defect in the report, got: %q", out)
	}
}

func TestWikiLintCommand_FixIsNotReadOnlyByDefault(t *testing.T) {
	ws := buildWikiWorkspace(t, map[string]string{
		"alpha": "Alpha about gears.\n",
	})
	summary := filepath.Join(ws, "wiki", "summaries", "alpha.md")
	appendFileForTest(t, summary, "\nSee [[concepts/ghost-deadbeefdeadbeef]].\n")
	before := readFileForTest(t, summary)

	// Default (no --fix) is READ-ONLY: the file is untouched even though a
	// blocking defect is present.
	_, _ = captureStdout(t, func() error {
		wikiLintCmd.Flags().Set("path", ws)       //nolint:errcheck // test setup
		defer wikiLintCmd.Flags().Set("path", "") //nolint:errcheck // test setup
		return runWikiLint(wikiLintCmd, nil)
	})
	if readFileForTest(t, summary) != before {
		t.Fatalf("default lint must be read-only; the page was modified")
	}

	// With --fix the dangling link is stripped and the run now exits clean.
	out, err := captureStdout(t, func() error {
		wikiLintCmd.Flags().Set("path", ws)           //nolint:errcheck // test setup
		wikiLintCmd.Flags().Set("fix", "true")        //nolint:errcheck // test setup
		defer wikiLintCmd.Flags().Set("path", "")     //nolint:errcheck // test setup
		defer wikiLintCmd.Flags().Set("fix", "false") //nolint:errcheck // test setup
		return runWikiLint(wikiLintCmd, nil)
	})
	if err != nil {
		t.Fatalf("after --fix the workspace should lint clean, got: %v", err)
	}
	if !strings.Contains(out, "stripped 1 dangling link") {
		t.Fatalf("expected the fix summary, got: %q", out)
	}
	if strings.Contains(readFileForTest(t, summary), "ghost-deadbeefdeadbeef") {
		t.Fatalf("the dangling link was not stripped by --fix")
	}
}

func TestWikiHealthCommands_Registered(t *testing.T) {
	for _, sub := range []string{"list", "status"} {
		if _, _, err := rootCmd.Find([]string{"wiki", sub}); err != nil {
			t.Fatalf("expected wiki subcommand %q to be registered: %v", sub, err)
		}
	}
}

func appendFileForTest(t *testing.T, path, extra string) {
	t.Helper()
	existing := readFileForTest(t, path)
	if err := os.WriteFile(path, []byte(existing+extra), 0o600); err != nil {
		t.Fatalf("append %s: %v", path, err)
	}
}

func readFileForTest(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
