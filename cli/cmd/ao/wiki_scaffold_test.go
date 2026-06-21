package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/wiki"
)

// setWikiInitFlag sets a wikiInitCmd flag and restores it via t.Cleanup so the
// shared cobra command state does not leak into other (-shuffle) tests.
func setWikiInitFlag(t *testing.T, name, val string) {
	t.Helper()
	orig := wikiInitCmd.Flags().Lookup(name).Value.String()
	if err := wikiInitCmd.Flags().Set(name, val); err != nil {
		t.Fatalf("set --%s: %v", name, err)
	}
	t.Cleanup(func() { _ = wikiInitCmd.Flags().Set(name, orig) })
}

// TestRunWikiInit drives the `ao wiki init` command and asserts it scaffolds the
// workspace (layout + config) at the given path.
func TestRunWikiInit(t *testing.T) {
	dir := t.TempDir()
	setWikiInitFlag(t, "model", "cmd-test-model")
	setWikiInitFlag(t, "language", "en")

	buf := &bytes.Buffer{}
	wikiInitCmd.SetOut(buf)
	t.Cleanup(func() { wikiInitCmd.SetOut(nil) })

	if err := runWikiInit(wikiInitCmd, []string{dir}); err != nil {
		t.Fatalf("runWikiInit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "wiki", "config.yaml")); err != nil {
		t.Fatalf("config not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "raw")); err != nil {
		t.Errorf("raw/ layout dir not created: %v", err)
	}
	cfg, err := wiki.ReadScaffoldConfig(dir)
	if err != nil {
		t.Fatalf("ReadScaffoldConfig: %v", err)
	}
	if cfg.Model != "cmd-test-model" {
		t.Errorf("config model = %q, want cmd-test-model", cfg.Model)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Initialized")) {
		t.Errorf("init output missing confirmation: %q", buf.String())
	}
}

// TestRunWikiUse drives `ao wiki use` and asserts it records the active workspace
// repo-locally, and rejects a non-existent path.
func TestRunWikiUse(t *testing.T) {
	stateDir := t.TempDir()
	ws := t.TempDir()
	t.Chdir(stateDir)

	buf := &bytes.Buffer{}
	wikiUseCmd.SetOut(buf)
	t.Cleanup(func() { wikiUseCmd.SetOut(nil) })

	if err := runWikiUse(wikiUseCmd, []string{ws}); err != nil {
		t.Fatalf("runWikiUse: %v", err)
	}
	got, err := wiki.ActiveWorkspace(stateDir)
	if err != nil {
		t.Fatalf("ActiveWorkspace: %v", err)
	}
	if got == "" {
		t.Fatal("active workspace not recorded")
	}

	if err := runWikiUse(wikiUseCmd, []string{filepath.Join(ws, "nope")}); err == nil {
		t.Error("runWikiUse accepted a non-existent path")
	}

	// No-arg `use` reads the active workspace back (proves resolution is consumed).
	buf.Reset()
	if err := runWikiUse(wikiUseCmd, nil); err != nil {
		t.Fatalf("runWikiUse (read): %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(got)) {
		t.Errorf("no-arg use did not report the active workspace %q: %q", got, buf.String())
	}

	// The resolver honors the active selection (no path arg) and an explicit arg.
	resolved, err := wikiResolveWorkspace(stateDir, "")
	if err != nil || resolved != got {
		t.Errorf("wikiResolveWorkspace(active) = %q, %v; want %q", resolved, err, got)
	}
	if r, err := wikiResolveWorkspace(stateDir, ws); err != nil || r == "" {
		t.Errorf("wikiResolveWorkspace(explicit) = %q, %v", r, err)
	}
}

// TestRunWikiInit_SelectsWorkspace verifies init records the new workspace as
// active (init -> use round-trip), so subsequent commands resolve it.
func TestRunWikiInit_SelectsWorkspace(t *testing.T) {
	t.Chdir(t.TempDir())
	setWikiInitFlag(t, "model", "m")
	setWikiInitFlag(t, "language", "en")
	wikiInitCmd.SetOut(&bytes.Buffer{})
	t.Cleanup(func() { wikiInitCmd.SetOut(nil) })

	if err := runWikiInit(wikiInitCmd, []string{"ws"}); err != nil {
		t.Fatalf("runWikiInit: %v", err)
	}
	resolved, err := wikiResolveWorkspace(".", "")
	if err != nil {
		t.Fatalf("init did not select a workspace: %v", err)
	}
	if filepath.Base(resolved) != "ws" {
		t.Errorf("active workspace = %q, want .../ws", resolved)
	}
}
