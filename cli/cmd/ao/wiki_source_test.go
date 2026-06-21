package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/wiki"
)

func testNow() time.Time { return time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC) }

// newSourceWorkspace scaffolds a workspace under a temp cwd and selects it active,
// returning the workspace path. Tests then drive add/remove/recompile against it.
func newSourceWorkspace(t *testing.T) string {
	t.Helper()
	t.Chdir(t.TempDir())
	ws := "ws"
	if _, err := wiki.Scaffold(ws, wiki.DefaultScaffoldConfig("test", "en")); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if _, err := wiki.SetActiveWorkspace(".", ws); err != nil {
		t.Fatalf("set active: %v", err)
	}
	return ws
}

func writeSourceFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	return p
}

// runSourceCmd runs a source subcommand's RunE with flags reset afterward so the
// shared cobra flag state never leaks into other (-shuffle) tests.
func runSourceCmd(t *testing.T, cmd *cobra.Command, args []string, flags map[string]string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	t.Cleanup(func() { cmd.SetOut(nil) })
	for k, v := range flags {
		orig := cmd.Flags().Lookup(k).Value.String()
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatalf("set --%s: %v", k, err)
		}
		k := k
		orig2 := orig
		t.Cleanup(func() { _ = cmd.Flags().Set(k, orig2) })
	}
	err := cmd.RunE(cmd, args)
	return buf.String(), err
}

// TestWikiSourceAdd drives `ao wiki add`: supported files land in raw/ + registry;
// unsupported types are skipped/reported.
func TestWikiSourceAdd(t *testing.T) {
	ws := newSourceWorkspace(t)
	srcDir := t.TempDir()
	writeSourceFile(t, srcDir, "alpha.md", "# Alpha\nbody")
	writeSourceFile(t, srcDir, "notes.txt", "plain notes")
	writeSourceFile(t, srcDir, "image.png", "binary-ish")
	// .markdown is NOT what the ingest processes (.md/.txt only) — must be skipped
	// so add/dry-run/real-ingest stay consistent (cross-family REFUTE).
	writeSourceFile(t, srcDir, "doc.markdown", "# Doc")

	out, err := runSourceCmd(t, wikiAddCmd, []string{srcDir}, nil)
	if err != nil {
		t.Fatalf("runWikiAdd: %v", err)
	}
	// Supported files copied into raw/.
	for _, name := range []string{"alpha.md", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(ws, "raw", name)); err != nil {
			t.Errorf("supported source not copied to raw/: %s (%v)", name, err)
		}
	}
	// Unsupported skipped (not copied) and reported.
	for _, name := range []string{"image.png", "doc.markdown"} {
		if _, err := os.Stat(filepath.Join(ws, "raw", name)); err == nil {
			t.Errorf("unsupported %s was copied into raw/", name)
		}
	}
	if !bytes.Contains([]byte(out), []byte("skipped")) {
		t.Errorf("add did not report the skipped unsupported file:\n%s", out)
	}
	// Registry records the supported sources.
	sources, err := wiki.RegisteredSources(ws)
	if err != nil {
		t.Fatalf("RegisteredSources: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("registry has %d sources, want 2", len(sources))
	}
}

// TestWikiSourceRemove drives `ao wiki remove`: --dry-run reports without
// deleting; the real remove deletes raw + registry entry (and reports the same).
func TestWikiSourceRemove(t *testing.T) {
	ws := newSourceWorkspace(t)
	srcDir := t.TempDir()
	writeSourceFile(t, srcDir, "alpha.md", "# Alpha")
	if _, err := wiki.AddSources(ws, []string{filepath.Join(srcDir, "alpha.md")}, testNow()); err != nil {
		t.Fatalf("add: %v", err)
	}
	rawPath := filepath.Join(ws, "raw", "alpha.md")

	// Dry-run: reports the raw artifact, deletes nothing.
	out, err := runSourceCmd(t, wikiRemoveCmd, []string{"alpha"}, map[string]string{"dry-run": "true"})
	if err != nil {
		t.Fatalf("remove --dry-run: %v", err)
	}
	if !bytes.Contains([]byte(out), []byte("raw/alpha.md")) {
		t.Errorf("dry-run did not report the raw artifact:\n%s", out)
	}
	if _, err := os.Stat(rawPath); err != nil {
		t.Error("dry-run deleted the raw file (must not)")
	}
	if s, _ := wiki.RegisteredSources(ws); len(s) != 1 {
		t.Error("dry-run mutated the registry (must not)")
	}

	// Real remove: deletes raw + registry entry.
	if _, err := runSourceCmd(t, wikiRemoveCmd, []string{"alpha"}, map[string]string{"dry-run": "false"}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(rawPath); err == nil {
		t.Error("remove did not delete the raw file")
	}
	if s, _ := wiki.RegisteredSources(ws); len(s) != 0 {
		t.Errorf("remove did not drop the registry entry: %d remain", len(s))
	}

	// Removing an unregistered doc errors (no silent success).
	if _, err := runSourceCmd(t, wikiRemoveCmd, []string{"nope"}, map[string]string{"dry-run": "false"}); err == nil {
		t.Error("remove of an unregistered doc should error")
	}
}

// TestWikiSourceRecompile drives `ao wiki recompile`: --dry-run lists registered
// sources; the real run ingests raw/ into wiki/sources/.
func TestWikiSourceRecompile(t *testing.T) {
	ws := newSourceWorkspace(t)
	srcDir := t.TempDir()
	writeSourceFile(t, srcDir, "alpha.md", "# Alpha\nbody")
	if _, err := wiki.AddSources(ws, []string{filepath.Join(srcDir, "alpha.md")}, testNow()); err != nil {
		t.Fatalf("add: %v", err)
	}

	out, err := runSourceCmd(t, wikiRecompileCmd, nil, map[string]string{"dry-run": "true"})
	if err != nil {
		t.Fatalf("recompile --dry-run: %v", err)
	}
	if !bytes.Contains([]byte(out), []byte("alpha")) {
		t.Errorf("recompile --dry-run did not list the source:\n%s", out)
	}

	if _, err := runSourceCmd(t, wikiRecompileCmd, nil, map[string]string{"dry-run": "false"}); err != nil {
		t.Fatalf("recompile: %v", err)
	}
	// Ingest distilled raw/alpha.md into wiki/sources/alpha.md.
	if _, err := os.Stat(filepath.Join(ws, "wiki", "sources", "alpha.md")); err != nil {
		t.Errorf("recompile did not produce wiki/sources/alpha.md: %v", err)
	}
}

// TestWikiSourceRecompile_DryRunMatchesRealInput is the cross-family REFUTE fix:
// recompile --dry-run reports the raw/ files the REAL run ingests (not the
// registry), so after `remove --keep-raw` orphans a raw file, dry-run still
// lists it (it would be recompiled), instead of lying with the registry count.
func TestWikiSourceRecompile_DryRunMatchesRealInput(t *testing.T) {
	ws := newSourceWorkspace(t)
	srcDir := t.TempDir()
	writeSourceFile(t, srcDir, "alpha.md", "# Alpha")
	if _, err := wiki.AddSources(ws, []string{filepath.Join(srcDir, "alpha.md")}, testNow()); err != nil {
		t.Fatalf("add: %v", err)
	}
	// remove --keep-raw: registry entry gone, raw/alpha.md orphaned.
	if _, err := wiki.RemoveSource(ws, "alpha", wiki.RemoveOptions{KeepRaw: true}); err != nil {
		t.Fatalf("remove --keep-raw: %v", err)
	}
	if s, _ := wiki.RegisteredSources(ws); len(s) != 0 {
		t.Fatalf("precondition: registry not empty (%d)", len(s))
	}
	// dry-run must still report the orphaned raw file (matches the real run).
	out, err := runSourceCmd(t, wikiRecompileCmd, nil, map[string]string{"dry-run": "true"})
	if err != nil {
		t.Fatalf("recompile --dry-run: %v", err)
	}
	if !bytes.Contains([]byte(out), []byte("alpha.md")) {
		t.Errorf("dry-run did not report the orphaned raw file (registry-based lie):\n%s", out)
	}
}

// TestWikiSourceRecompile_RejectsPerDoc: a doc argument is rejected explicitly
// (not silently ignored) since the ingest is whole-workspace (cross-family REFUTE).
func TestWikiSourceRecompile_RejectsPerDoc(t *testing.T) {
	newSourceWorkspace(t)
	if _, err := runSourceCmd(t, wikiRecompileCmd, []string{"alpha"}, map[string]string{"dry-run": "false"}); err == nil {
		t.Error("recompile <doc> should be rejected (per-doc not supported), not silently ignored")
	}
	if _, err := runSourceCmd(t, wikiRecompileCmd, []string{"alpha"}, map[string]string{"dry-run": "true"}); err == nil {
		t.Error("recompile <doc> --dry-run should also be rejected")
	}
}
