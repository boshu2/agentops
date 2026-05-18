// practices: [tdd, pragmatic-programmer]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// writeWikiCorpus seeds a temp corpus under base/.agents/ with the given
// relative-path → content files and returns base. It is the fixture shared by
// the wiki command-level tests.
func writeWikiCorpus(t *testing.T, files map[string]string) string {
	t.Helper()
	base := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(base, ".agents", rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return base
}

func TestWikiIndexCommand_IndexesCorpusDocuments(t *testing.T) {
	base := writeWikiCorpus(t, map[string]string{
		"learnings/alpha.md": "# Alpha\n\nWidget calibration learning.\n",
		"research/beta.md":   "# Beta\n\nObservability research notes.\n",
	})

	out, err := captureStdout(t, func() error {
		wikiIndexCmd.Flags().Set("base", base) //nolint:errcheck // test setup
		defer wikiIndexCmd.Flags().Set("base", "")
		return runWikiIndex(wikiIndexCmd, nil)
	})
	if err != nil {
		t.Fatalf("runWikiIndex returned error: %v", err)
	}
	if !strings.Contains(out, "2 documents indexed") {
		t.Fatalf("expected 2 documents indexed, got: %q", out)
	}
	if !strings.Contains(out, "2 added") {
		t.Fatalf("expected 2 added, got: %q", out)
	}
}

func TestWikiSearchCommand_ReturnsRankedResults(t *testing.T) {
	base := writeWikiCorpus(t, map[string]string{
		"learnings/widget.md": "# Widget\n\nwidget widget widget calibration.\n",
		"learnings/gadget.md": "# Gadget\n\nA single widget mention.\n",
		"learnings/other.md":  "# Other\n\nUnrelated content.\n",
	})

	out, err := captureStdout(t, func() error {
		wikiSearchCmd.Flags().Set("base", base) //nolint:errcheck // test setup
		defer wikiSearchCmd.Flags().Set("base", "")
		return runWikiSearch(wikiSearchCmd, []string{"widget"})
	})
	if err != nil {
		t.Fatalf("runWikiSearch returned error: %v", err)
	}
	if !strings.Contains(out, "2 match(es)") {
		t.Fatalf("expected 2 matches, got: %q", out)
	}
	// widget.md mentions "widget" four times, gadget.md once — the higher
	// score must rank first.
	widgetIdx := strings.Index(out, "widget.md")
	gadgetIdx := strings.Index(out, "gadget.md")
	if widgetIdx < 0 || gadgetIdx < 0 {
		t.Fatalf("expected both widget.md and gadget.md in output, got: %q", out)
	}
	if widgetIdx > gadgetIdx {
		t.Fatalf("expected widget.md ranked above gadget.md, got: %q", out)
	}
}

func TestWikiSearchCommand_NoMatchExitsClean(t *testing.T) {
	base := writeWikiCorpus(t, map[string]string{
		"learnings/alpha.md": "# Alpha\n\nNothing relevant here.\n",
	})

	out, err := captureStdout(t, func() error {
		wikiSearchCmd.Flags().Set("base", base) //nolint:errcheck // test setup
		defer wikiSearchCmd.Flags().Set("base", "")
		return runWikiSearch(wikiSearchCmd, []string{"nonexistentterm"})
	})
	if err != nil {
		t.Fatalf("runWikiSearch returned error on no-match: %v", err)
	}
	if !strings.Contains(out, "no matches") {
		t.Fatalf("expected no-matches message, got: %q", out)
	}
}

func TestRankWikiRecords_OrdersByDescendingScore(t *testing.T) {
	dir := t.TempDir()
	high := filepath.Join(dir, "high.md")
	low := filepath.Join(dir, "low.md")
	if err := os.WriteFile(high, []byte("alpha alpha alpha"), 0o600); err != nil {
		t.Fatalf("write high: %v", err)
	}
	if err := os.WriteFile(low, []byte("alpha"), 0o600); err != nil {
		t.Fatalf("write low: %v", err)
	}

	records := []ports.WikiIndexRecord{
		{Path: high, Root: dir},
		{Path: low, Root: dir},
	}
	hits := rankWikiRecords(records, "alpha")
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
	if hits[0].Path != high {
		t.Fatalf("expected high.md first, got %s", hits[0].Path)
	}
	if hits[0].Score != 3 || hits[1].Score != 1 {
		t.Fatalf("expected scores 3 and 1, got %d and %d", hits[0].Score, hits[1].Score)
	}
}

func TestWikiQueryTerms_DeduplicatesAndLowercases(t *testing.T) {
	terms := wikiQueryTerms("Widget WIDGET  gadget")
	if len(terms) != 2 {
		t.Fatalf("expected 2 unique terms, got %d: %v", len(terms), terms)
	}
	if terms[0] != "widget" || terms[1] != "gadget" {
		t.Fatalf("expected [widget gadget], got %v", terms)
	}
}

func TestWikiPromoteCommand_ReportsNotConfiguredSkip(t *testing.T) {
	vault := t.TempDir()

	out, err := captureStdout(t, func() error {
		wikiPromoteCmd.Flags().Set("vault", vault) //nolint:errcheck // test setup
		defer wikiPromoteCmd.Flags().Set("vault", "")
		return runWikiPromote(wikiPromoteCmd, nil)
	})
	if err != nil {
		t.Fatalf("runWikiPromote returned error: %v", err)
	}
	if !strings.Contains(out, "promote-handler-not-configured") {
		t.Fatalf("expected promote not-configured skip, got: %q", out)
	}
}

func TestWikiLintCommand_WritesLintReport(t *testing.T) {
	vault := t.TempDir()

	out, err := captureStdout(t, func() error {
		wikiLintCmd.Flags().Set("vault", vault) //nolint:errcheck // test setup
		defer wikiLintCmd.Flags().Set("vault", "")
		return runWikiLint(wikiLintCmd, nil)
	})
	if err != nil {
		t.Fatalf("runWikiLint returned error: %v", err)
	}
	if !strings.Contains(out, "wiki lint: complete") {
		t.Fatalf("expected lint completion, got: %q", out)
	}
}

func TestWikiDoctorCommand_ReportsCorpusAndIndexState(t *testing.T) {
	base := writeWikiCorpus(t, map[string]string{
		"learnings/alpha.md": "# Alpha\n\nContent.\n",
	})

	out, err := captureStdout(t, func() error {
		wikiDoctorCmd.Flags().Set("base", base) //nolint:errcheck // test setup
		defer wikiDoctorCmd.Flags().Set("base", "")
		return runWikiDoctor(wikiDoctorCmd, nil)
	})
	if err != nil {
		t.Fatalf("runWikiDoctor returned error: %v", err)
	}
	if !strings.Contains(out, "corpus root") || !strings.Contains(out, "(present)") {
		t.Fatalf("expected corpus-root present line, got: %q", out)
	}
	if !strings.Contains(out, "indexed docs: 0") {
		t.Fatalf("expected zero indexed docs before indexing, got: %q", out)
	}
}

func TestWikiCommand_IsRegisteredAndExperimental(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"wiki"})
	if err != nil {
		t.Fatalf("wiki command not found: %v", err)
	}
	if cmd.Name() != "wiki" {
		t.Fatalf("expected wiki command, got %q", cmd.Name())
	}
	if !strings.Contains(strings.ToLower(cmd.Short), "experimental") {
		t.Fatalf("expected wiki short help to mark surface experimental, got: %q", cmd.Short)
	}
	want := []string{"index", "search", "inject", "lint", "promote", "query", "doctor"}
	for _, sub := range want {
		if _, _, err := rootCmd.Find([]string{"wiki", sub}); err != nil {
			t.Fatalf("expected wiki subcommand %q to be registered: %v", sub, err)
		}
	}
}

func TestLegacyInjectCommand_StillRegistered(t *testing.T) {
	// Strangler invariant: the wiki group is accretive — the legacy inject
	// command must remain registered and untouched.
	if _, _, err := rootCmd.Find([]string{"inject"}); err != nil {
		t.Fatalf("legacy inject command must remain registered: %v", err)
	}
}
