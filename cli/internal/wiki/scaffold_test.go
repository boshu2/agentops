package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestWikiScaffoldInit is age-port-openkb ac.1: `ao wiki init`'s core creates the
// OpenKB-style layout, seed files, and config under a clean root.
func TestWikiScaffoldInit(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultScaffoldConfig("test-model", "en")
	cfgPath, err := Scaffold(root, cfg)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	// Every layout directory exists.
	for _, dir := range ScaffoldLayout {
		abs := filepath.Join(root, filepath.FromSlash(dir))
		if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
			t.Errorf("layout dir not created: %s (err=%v)", dir, err)
		}
	}

	// Seed files exist and the AGENTS.md schema records the model.
	for _, rel := range []string{"wiki/index.md", "wiki/log.md", "wiki/AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("seed file not created: %s (%v)", rel, err)
		}
	}
	agents, err := os.ReadFile(filepath.Join(root, "wiki", "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agents), "test-model") {
		t.Errorf("AGENTS.md schema missing model reference:\n%s", agents)
	}

	// Config was written and round-trips back with the requested model/language.
	if cfgPath == "" {
		t.Fatal("Scaffold returned empty config path")
	}
	got, err := ReadScaffoldConfig(root)
	if err != nil {
		t.Fatalf("ReadScaffoldConfig: %v", err)
	}
	if got.Model != "test-model" || got.Language != "en" {
		t.Errorf("config = %+v, want model=test-model language=en", got)
	}
	if len(got.EntityTypes) == 0 {
		t.Error("config has no entity types")
	}
	if got.Thresholds.SummaryMinChars <= 0 || got.Thresholds.ConceptMinMentions <= 0 {
		t.Errorf("config thresholds not seeded: %+v", got.Thresholds)
	}
}

// TestWikiScaffoldInit_Idempotent verifies a second init preserves existing seed
// content (never clobbers) while still rewriting the config.
func TestWikiScaffoldInit_Idempotent(t *testing.T) {
	root := t.TempDir()
	if _, err := Scaffold(root, DefaultScaffoldConfig("m1", "en")); err != nil {
		t.Fatalf("first Scaffold: %v", err)
	}
	indexPath := filepath.Join(root, "wiki", "index.md")
	if err := os.WriteFile(indexPath, []byte("# Edited by operator\n"), 0o600); err != nil {
		t.Fatalf("edit index: %v", err)
	}
	if _, err := Scaffold(root, DefaultScaffoldConfig("m2", "fr")); err != nil {
		t.Fatalf("second Scaffold: %v", err)
	}
	// Authored seed content is preserved.
	got, _ := os.ReadFile(indexPath)
	if string(got) != "# Edited by operator\n" {
		t.Errorf("idempotent init clobbered authored index.md: %q", got)
	}
	// Config reflects the latest model/language.
	cfg, err := ReadScaffoldConfig(root)
	if err != nil {
		t.Fatalf("ReadScaffoldConfig: %v", err)
	}
	if cfg.Model != "m2" || cfg.Language != "fr" {
		t.Errorf("config not updated on re-init: %+v", cfg)
	}
}

// TestWikiScaffoldInit_RejectsReservedRoot is the bead's boundary risk: init must
// refuse a root that names a reserved corpus path (.agents/.ao), never writing
// into the private corpus or gold wiki (cross-family REFUTE).
func TestWikiScaffoldInit_RejectsReservedRoot(t *testing.T) {
	base := t.TempDir()
	for _, seg := range []string{".agents", ".ao"} {
		reservedRoot := filepath.Join(base, seg, "wiki")
		if _, err := Scaffold(reservedRoot, DefaultScaffoldConfig("m", "en")); err == nil {
			t.Errorf("Scaffold accepted reserved root containing %q", seg)
		}
		// Nothing should have been created under the reserved segment.
		if _, err := os.Stat(filepath.Join(base, seg)); err == nil {
			t.Errorf("Scaffold created files under reserved segment %q", seg)
		}
	}
	// Case variants must also be rejected (case-insensitive volumes: macOS/Windows).
	for _, seg := range []string{".AGENTS", ".AO", ".Ao"} {
		if _, err := Scaffold(filepath.Join(base, seg, "wiki"), DefaultScaffoldConfig("m", "en")); err == nil {
			t.Errorf("Scaffold accepted case-variant reserved root %q", seg)
		}
	}
}

// TestWikiScaffoldInit_RejectsSymlinkEscape: a layout dir that is a pre-existing
// symlink into a reserved corpus path must be rejected (the escape vector), not
// silently followed.
func TestWikiScaffoldInit_RejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	reserved := filepath.Join(t.TempDir(), ".ao", "wiki")
	if err := os.MkdirAll(reserved, 0o750); err != nil {
		t.Fatalf("mk reserved: %v", err)
	}
	// root/wiki -> <other>/.ao/wiki  (so wiki/sources would land in the gold tree)
	if err := os.Symlink(reserved, filepath.Join(root, "wiki")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := Scaffold(root, DefaultScaffoldConfig("m", "en")); err == nil {
		t.Fatal("Scaffold followed a symlink into a reserved corpus path")
	}
	// The reserved target must not have gained scaffold subdirs.
	if _, err := os.Stat(filepath.Join(reserved, "sources")); err == nil {
		t.Error("Scaffold wrote into the reserved symlink target")
	}
}

// TestWikiScaffoldInit_RelativeRoot guards the relative-root fix: scaffolding
// with a RELATIVE root (resolved against cwd) must work, not false-reject on a
// relative-vs-absolute path comparison (cross-family REFUTE).
func TestWikiScaffoldInit_RelativeRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := Scaffold("mywiki", DefaultScaffoldConfig("m", "en")); err != nil {
		t.Fatalf("Scaffold with relative root: %v", err)
	}
	if fi, err := os.Stat(filepath.Join("mywiki", "wiki", "config.yaml")); err != nil || fi.IsDir() {
		t.Errorf("relative-root scaffold did not create config: %v", err)
	}
}

// TestWikiScaffoldInit_RejectsDanglingSymlink: a DANGLING symlink at a write
// target (link exists, target missing) that points into a reserved corpus path
// must be rejected — os.WriteFile would otherwise follow it and create the file
// under .agents/.ao (cross-family REFUTE).
func TestWikiScaffoldInit_RejectsDanglingSymlink(t *testing.T) {
	root := t.TempDir()
	reservedDir := filepath.Join(t.TempDir(), ".agents")
	if err := os.MkdirAll(reservedDir, 0o750); err != nil {
		t.Fatalf("mk reserved: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "wiki"), 0o750); err != nil {
		t.Fatalf("mk wiki: %v", err)
	}
	// Dangling: link exists, target file does not yet exist.
	danglingTarget := filepath.Join(reservedDir, "config.yaml")
	if err := os.Symlink(danglingTarget, filepath.Join(root, "wiki", "config.yaml")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := Scaffold(root, DefaultScaffoldConfig("m", "en")); err == nil {
		t.Fatal("Scaffold followed a dangling symlink into a reserved path")
	}
	if _, err := os.Stat(danglingTarget); err == nil {
		t.Error("Scaffold wrote through a dangling symlink into the reserved target")
	}
}

// TestWikiScaffoldInit_RejectsSymlinkChain: a MULTI-HOP dangling symlink chain
// (config.yaml -> link2 -> reserved) must be rejected — the no-symlink-component
// guard catches it at the first hop, regardless of chain depth (cross-family
// REFUTE: one-level resolution missed chains).
func TestWikiScaffoldInit_RejectsSymlinkChain(t *testing.T) {
	root := t.TempDir()
	reservedDir := filepath.Join(t.TempDir(), ".ao")
	if err := os.MkdirAll(reservedDir, 0o750); err != nil {
		t.Fatalf("mk reserved: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "wiki"), 0o750); err != nil {
		t.Fatalf("mk wiki: %v", err)
	}
	link2 := filepath.Join(root, "wiki", "link2")
	if err := os.Symlink(filepath.Join(reservedDir, "config.yaml"), link2); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := os.Symlink(link2, filepath.Join(root, "wiki", "config.yaml")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := Scaffold(root, DefaultScaffoldConfig("m", "en")); err == nil {
		t.Fatal("Scaffold followed a symlink chain")
	}
	if _, err := os.Stat(filepath.Join(reservedDir, "config.yaml")); err == nil {
		t.Error("Scaffold wrote through a symlink chain into the reserved target")
	}
}

// TestScaffoldConfigSchema is age-port-openkb ac.1 (Schema half): the config
// struct round-trips losslessly through its YAML schema, including nested
// thresholds and entity types.
func TestScaffoldConfigSchema(t *testing.T) {
	cfg := ScaffoldConfig{
		Model:       "claude-x",
		Language:    "en",
		EntityTypes: []string{"person", "tool"},
		Thresholds:  ScaffoldThresholds{SummaryMinChars: 150, ConceptMinMentions: 2},
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back ScaffoldConfig
	if err := yaml.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Model != cfg.Model || back.Language != cfg.Language {
		t.Errorf("model/language did not round-trip: %+v", back)
	}
	if len(back.EntityTypes) != 2 || back.EntityTypes[0] != "person" {
		t.Errorf("entity types did not round-trip: %+v", back.EntityTypes)
	}
	if back.Thresholds != cfg.Thresholds {
		t.Errorf("thresholds did not round-trip: %+v want %+v", back.Thresholds, cfg.Thresholds)
	}
}

// TestScaffoldActiveWorkspaceSchema verifies use/resolve persistence: an absent
// pointer is empty (no error), and a set pointer round-trips.
func TestScaffoldActiveWorkspaceSchema(t *testing.T) {
	stateDir := t.TempDir()
	if got, err := ActiveWorkspace(stateDir); err != nil || got != "" {
		t.Fatalf("absent active workspace: got %q err %v, want empty/no-error", got, err)
	}
	ws := t.TempDir()
	abs, err := SetActiveWorkspace(stateDir, ws)
	if err != nil {
		t.Fatalf("SetActiveWorkspace: %v", err)
	}
	got, err := ActiveWorkspace(stateDir)
	if err != nil {
		t.Fatalf("ActiveWorkspace: %v", err)
	}
	if got != abs {
		t.Errorf("active workspace = %q, want %q", got, abs)
	}
	// A non-existent path is rejected (no silent dangling pointer).
	if _, err := SetActiveWorkspace(stateDir, filepath.Join(ws, "does-not-exist")); err == nil {
		t.Error("SetActiveWorkspace accepted a non-existent path")
	}
}

