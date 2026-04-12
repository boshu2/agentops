package main

import (
	"encoding/json"
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
	t.Setenv("AGENTOPS_CONFIG", filepath.Join(t.TempDir(), "missing-config.yaml"))
	t.Setenv("AGENTOPS_DREAM_CURATOR_WORKER_DIR", "")
	t.Setenv("AGENTOPS_DREAM_CURATOR_MODEL", "")
	summary := &overnightSummary{}
	runPostLoopTier1Forge(nil, t.TempDir(), summary, overnightSettings{})
	// No model configured = skip silently (opt-in feature).
	if len(summary.Degraded) != 0 {
		t.Errorf("no model should skip silently, got degraded: %v", summary.Degraded)
	}
}

func TestRunPostLoopTier1Forge_QueuesWhenWorkerConfigured(t *testing.T) {
	tmp := t.TempDir()
	workerDir := filepath.Join(tmp, "dream-worker")
	projDir := filepath.Join(tmp, ".claude", "projects", "test-project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sourcePath := filepath.Join(projDir, "recent-session.jsonl")
	data := make([]byte, 2000)
	for i := range data {
		data[i] = 'x'
	}
	if err := os.WriteFile(sourcePath, data, 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	t.Setenv("HOME", tmp)
	t.Setenv("AGENTOPS_FORGE_TIER1_DISABLE", "")
	t.Setenv("AGENTOPS_CONFIG", filepath.Join(tmp, "missing-config.yaml"))
	t.Setenv("AGENTOPS_DREAM_CURATOR_MODEL", "")
	t.Setenv("AGENTOPS_DREAM_CURATOR_WORKER_DIR", workerDir)

	summary := &overnightSummary{}
	runPostLoopTier1Forge(nil, filepath.Join(tmp, "repo"), summary, overnightSettings{})
	if len(summary.Degraded) != 0 {
		t.Fatalf("expected no degradation, got %v", summary.Degraded)
	}

	queueDir := filepath.Join(workerDir, "queue")
	entries, err := os.ReadDir(queueDir)
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("queue entries = %d, want 1", len(entries))
	}

	tier1, ok := summary.CloseLoop["tier1_forge"].(map[string]any)
	if !ok {
		t.Fatalf("tier1_forge summary = %#v, want map", summary.CloseLoop["tier1_forge"])
	}
	if tier1["mode"] != "dream-worker-queue" {
		t.Fatalf("mode = %#v, want dream-worker-queue", tier1["mode"])
	}
	if tier1["queued"] != 1 {
		t.Fatalf("queued = %#v, want 1", tier1["queued"])
	}
	if tier1["queue_dir"] != queueDir {
		t.Fatalf("queue_dir = %#v, want %q", tier1["queue_dir"], queueDir)
	}

	jobData, err := os.ReadFile(filepath.Join(queueDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read job: %v", err)
	}
	var job curatorJob
	if err := json.Unmarshal(jobData, &job); err != nil {
		t.Fatalf("parse job: %v\n%s", err, string(jobData))
	}
	if job.Kind != "ingest-claude-session" {
		t.Fatalf("job kind = %q, want ingest-claude-session", job.Kind)
	}
	if job.Source == nil || job.Source.Path != sourcePath {
		t.Fatalf("job source = %+v, want path %q", job.Source, sourcePath)
	}
}
