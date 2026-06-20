package main

import (
	"fmt"
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

// TestWikiPublish_RealPublishRequiresBead: real publish (no --dry-run) without
// --bead is refused (the bead's CONFIRMED verdict for HEAD is the gate).
func TestWikiPublish_RealPublishRequiresBead(t *testing.T) {
	base := writeWikiPublishFixture(t, "Gates must fail closed when a condition cannot be proven true.")
	t.Chdir(base)
	wikiPublishDryRun = false
	wikiPublishBead = ""
	t.Cleanup(func() { wikiPublishDryRun = false; wikiPublishBead = "" })

	_, err := captureStdout(t, func() error { return runWikiPublish(wikiPublishCmd, nil) })
	if err == nil {
		t.Fatal("expected real publish to be refused without --bead")
	}
	if !strings.Contains(err.Error(), "--bead") {
		t.Errorf("error should require --bead, got: %v", err)
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

// TestWikiPublish_RefusesUnsafeOut: --out resolving to a root/repo-base path is
// refused before any destructive clear.
func TestWikiPublish_RefusesUnsafeOut(t *testing.T) {
	base := writeWikiPublishFixture(t, "Gates must fail closed when a condition cannot be proven true.")
	t.Chdir(base)
	stubVerdict(t, true)
	wikiPublishDryRun = false
	wikiPublishBead = "ag-test"
	wikiPublishOut = "/"
	t.Cleanup(func() { wikiPublishDryRun = false; wikiPublishBead = ""; wikiPublishOut = ".ao/wiki" })

	_, err := captureStdout(t, func() error { return runWikiPublish(wikiPublishCmd, nil) })
	if err == nil {
		t.Fatal("expected refusal of unsafe --out /")
	}
	if !strings.Contains(err.Error(), "unsafe --out") {
		t.Errorf("error should name the unsafe out, got: %v", err)
	}
}

// stubVerdict overrides the verdict gate + HEAD resolver for a test and restores
// them after. confirmed=true => the gate passes.
func stubVerdict(t *testing.T, confirmed bool) {
	t.Helper()
	origCheck, origHead := checkPawlVerdict, resolveHeadSHA
	resolveHeadSHA = func(string) (string, error) { return "deadbeefcafe1234", nil }
	checkPawlVerdict = func(_, _, _ string) error {
		if confirmed {
			return nil
		}
		return fmt.Errorf("no confirmed verdict")
	}
	t.Cleanup(func() { checkPawlVerdict, resolveHeadSHA = origCheck, origHead })
}

// TestWikiPublish_RealPublishConfirmedWrites: with a CONFIRMED verdict for HEAD
// and a clean candidate, real publish writes the gold tree to --out.
func TestWikiPublish_RealPublishConfirmedWrites(t *testing.T) {
	base := writeWikiPublishFixture(t, "Gates must fail closed when a condition cannot be proven true.")
	t.Chdir(base)
	stubVerdict(t, true)
	wikiPublishDryRun = false
	wikiPublishBead = "ag-test"
	wikiPublishOut = ".ao/wiki"
	t.Cleanup(func() { wikiPublishDryRun = false; wikiPublishBead = ""; wikiPublishOut = ".ao/wiki" })

	out, err := captureStdout(t, func() error { return runWikiPublish(wikiPublishCmd, nil) })
	if err != nil {
		t.Fatalf("confirmed publish should succeed: %v", err)
	}
	if !strings.Contains(out, "PUBLISHED") {
		t.Errorf("expected PUBLISHED line, got: %q", out)
	}
	// The gold tree must now exist on disk.
	if _, err := os.Stat(filepath.Join(base, ".ao", "wiki", "index.md")); err != nil {
		t.Errorf("expected published gold wiki index.md, got: %v", err)
	}
}

// TestWikiPublish_RealPublishNoVerdictFailsClosed: a clean candidate WITHOUT a
// CONFIRMED verdict for HEAD is refused, and nothing is written to --out.
func TestWikiPublish_RealPublishNoVerdictFailsClosed(t *testing.T) {
	base := writeWikiPublishFixture(t, "Gates must fail closed when a condition cannot be proven true.")
	t.Chdir(base)
	stubVerdict(t, false)
	wikiPublishDryRun = false
	wikiPublishBead = "ag-test"
	wikiPublishOut = ".ao/wiki"
	t.Cleanup(func() { wikiPublishDryRun = false; wikiPublishBead = ""; wikiPublishOut = ".ao/wiki" })

	_, err := captureStdout(t, func() error { return runWikiPublish(wikiPublishCmd, nil) })
	if err == nil {
		t.Fatal("expected publish to FAIL CLOSED without a confirmed verdict")
	}
	if !strings.Contains(err.Error(), "no CONFIRMED pawl verdict") {
		t.Errorf("error should name the missing verdict, got: %v", err)
	}
	// Fail-closed must NOT have written the gold tree.
	if _, err := os.Stat(filepath.Join(base, ".ao", "wiki")); !os.IsNotExist(err) {
		t.Errorf("gold dir must not exist after a refused publish, stat err=%v", err)
	}
}

// TestWikiPublish_RealPublishLeakBeatsVerdict: a leaky candidate fails closed on
// the leak scan BEFORE the verdict gate is even consulted (verdict stub would
// pass, but the leak must still refuse).
func TestWikiPublish_RealPublishLeakBeatsVerdict(t *testing.T) {
	base := writeWikiPublishFixture(t, "The shield cluster topology must never reach published gold.")
	t.Chdir(base)
	stubVerdict(t, true)
	wikiPublishDryRun = false
	wikiPublishBead = "ag-test"
	t.Cleanup(func() { wikiPublishDryRun = false; wikiPublishBead = "" })

	_, err := captureStdout(t, func() error { return runWikiPublish(wikiPublishCmd, nil) })
	if err == nil {
		t.Fatal("a leaky candidate must fail closed even with a confirmed verdict")
	}
	if !strings.Contains(err.Error(), "leak scan") {
		t.Errorf("leak must be refused before the verdict gate, got: %v", err)
	}
}

// TestWikiPublish_ExpectDigestMismatchFailsClosed: --expect-digest that doesn't
// match the recomputed digest refuses publish (publish exactly what dry-run reviewed).
func TestWikiPublish_ExpectDigestMismatchFailsClosed(t *testing.T) {
	base := writeWikiPublishFixture(t, "Gates must fail closed when a condition cannot be proven true.")
	t.Chdir(base)
	stubVerdict(t, true)
	wikiPublishDryRun = false
	wikiPublishBead = "ag-test"
	wikiPublishExpect = "0000000000000000000000000000000000000000000000000000000000000000"
	t.Cleanup(func() { wikiPublishDryRun = false; wikiPublishBead = ""; wikiPublishExpect = "" })

	_, err := captureStdout(t, func() error { return runWikiPublish(wikiPublishCmd, nil) })
	if err == nil {
		t.Fatal("expected digest mismatch to fail closed")
	}
	if !strings.Contains(err.Error(), "digest mismatch") {
		t.Errorf("error should name the digest mismatch, got: %v", err)
	}
}
