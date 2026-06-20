package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeWikiPublishFixture lays out a minimal .agents/ corpus under a fresh temp
// dir and returns the base (so a test can t.Chdir into it; `ao wiki publish`
// resolves the corpus relative to the working directory).
func writeWikiPublishFixture(t *testing.T, findingBody string) string {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, ".agents", "findings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "---\ntype: finding\nid: f\nmaturity: established\nconfidence: 0.9\n---\n\n" + findingBody + "\n"
	if err := os.WriteFile(filepath.Join(dir, "f.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return base
}

// TestWikiPublish_RealPublishRefusedWithoutDryRun: `ao wiki publish` without
// --dry-run is refused (real publish is gated on the verdict design, age-xf9r).
func TestWikiPublish_RealPublishRefusedWithoutDryRun(t *testing.T) {
	wikiPublishDryRun = false
	err := runWikiPublish(wikiPublishCmd, nil)
	if err == nil {
		t.Fatal("expected real publish to be refused without --dry-run")
	}
	if !strings.Contains(err.Error(), "age-xf9r") {
		t.Errorf("error should point at the verdict-gate design bead, got: %v", err)
	}
}

// TestWikiPublish_DryRunCleanFixture: a leak-clean candidate prints a digest and
// exits 0.
func TestWikiPublish_DryRunCleanFixture(t *testing.T) {
	base := writeWikiPublishFixture(t, "Gates must fail closed when a condition cannot be proven true.")
	t.Chdir(base)
	wikiPublishDryRun = true
	t.Cleanup(func() { wikiPublishDryRun = false })

	out, err := captureStdout(t, func() error { return runWikiPublish(wikiPublishCmd, nil) })
	if err != nil {
		t.Fatalf("dry-run on a clean fixture should not error: %v", err)
	}
	if !strings.Contains(out, "candidate digest:") {
		t.Errorf("expected a candidate digest line, got: %q", out)
	}
	if !strings.Contains(out, "CLEAN") {
		t.Errorf("expected leak-scan CLEAN, got: %q", out)
	}
}

// TestWikiPublish_DryRunLeakFailsClosed: a candidate carrying a fleet marker
// (a real private span sanitize does not scrub) fails closed.
func TestWikiPublish_DryRunLeakFailsClosed(t *testing.T) {
	base := writeWikiPublishFixture(t, "The shield cluster topology must never reach published gold.")
	t.Chdir(base)
	wikiPublishDryRun = true
	t.Cleanup(func() { wikiPublishDryRun = false })

	_, err := captureStdout(t, func() error { return runWikiPublish(wikiPublishCmd, nil) })
	if err == nil {
		t.Fatal("expected dry-run to FAIL CLOSED on a leaky candidate")
	}
	if !strings.Contains(err.Error(), "leak scan") {
		t.Errorf("error should name the leak scan, got: %v", err)
	}
}
