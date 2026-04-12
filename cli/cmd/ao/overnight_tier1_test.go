package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectRecentSessionJSONL_FindsRecentFiles(t *testing.T) {
	// Create a fake projects dir structure mirroring ~/.claude/projects/
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projDir := filepath.Join(tmp, ".claude", "projects", "test-project")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write a recent .jsonl file (>1000 bytes so it passes the size filter).
	recent := filepath.Join(projDir, "recent-session.jsonl")
	data := make([]byte, 2000)
	for i := range data {
		data[i] = 'x'
	}
	if err := os.WriteFile(recent, data, 0644); err != nil {
		t.Fatalf("write recent: %v", err)
	}

	// Write a tiny .jsonl file that should be filtered out.
	tiny := filepath.Join(projDir, "tiny.jsonl")
	if err := os.WriteFile(tiny, []byte("small"), 0644); err != nil {
		t.Fatalf("write tiny: %v", err)
	}

	// Write a non-jsonl file that should be ignored.
	other := filepath.Join(projDir, "notes.md")
	if err := os.WriteFile(other, data, 0644); err != nil {
		t.Fatalf("write other: %v", err)
	}

	paths, err := collectRecentSessionJSONL("/unused-cwd")
	if err != nil {
		t.Fatalf("collectRecentSessionJSONL: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("want 1 path (recent-session.jsonl), got %d: %v", len(paths), paths)
	}
	if len(paths) == 1 && filepath.Base(paths[0]) != "recent-session.jsonl" {
		t.Errorf("wrong file: %s", paths[0])
	}
}

func TestCollectRecentSessionJSONL_NoClaude(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// No .claude dir at all — should return nil, nil.
	paths, err := collectRecentSessionJSONL("/unused")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("want 0 paths, got %d", len(paths))
	}
}

func TestRunPostLoopTier1Forge_SkipsWhenKillSwitchSet(t *testing.T) {
	t.Setenv("AGENTOPS_FORGE_TIER1_DISABLE", "1")
	summary := &overnightSummary{}
	runPostLoopTier1Forge(nil, t.TempDir(), summary, overnightSettings{})
	if len(summary.Degraded) != 0 {
		t.Errorf("kill switch should skip silently, got degraded: %v", summary.Degraded)
	}
}

func TestRunPostLoopTier1Forge_SkipsWhenNoModel(t *testing.T) {
	t.Setenv("AGENTOPS_FORGE_TIER1_DISABLE", "")
	t.Setenv("AGENTOPS_DREAM_CURATOR_MODEL", "")
	summary := &overnightSummary{}
	runPostLoopTier1Forge(nil, t.TempDir(), summary, overnightSettings{})
	// No model configured = skip silently (opt-in feature).
	if len(summary.Degraded) != 0 {
		t.Errorf("no model should skip silently, got degraded: %v", summary.Degraded)
	}
}
