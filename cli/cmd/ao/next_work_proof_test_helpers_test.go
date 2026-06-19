package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

func writeCompletedLoopRegistryRun(t *testing.T, rootDir, runID, epicID, goal string) {
	t.Helper()
	runDir := filepath.Join(rootDir, ".agents", "rpi", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir registry run dir: %v", err)
	}

	state := map[string]any{
		"schema_version": 1,
		"run_id":         runID,
		"epic_id":        epicID,
		"goal":           goal,
		"phase":          3,
		"started_at":     time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal registry state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, cliRPI.PhasedStateFile), data, 0o644); err != nil {
		t.Fatalf("write registry state: %v", err)
	}
}

func writeEvidenceOnlyClosurePacket(t *testing.T, rootDir, targetID string) string {
	t.Helper()
	packetDir := filepath.Join(rootDir, ".agents", "releases", "evidence-only-closures")
	if err := os.MkdirAll(packetDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence-only closure dir: %v", err)
	}

	packetPath := filepath.Join(packetDir, strings.ReplaceAll(targetID, "/", "_")+".json")
	packet := map[string]any{
		"schema_version": 1,
		"artifact_id":    "evidence-only-closure-" + targetID,
		"target_id":      targetID,
		"target_type":    "task",
		"created_at":     time.Now().UTC().Format(time.RFC3339),
		"producer":       "next-work-proof-test",
		"evidence_mode":  "staged",
		"validation_commands": []string{
			"bash scripts/validate-go-fast.sh",
		},
		"repo_state": map[string]any{
			"repo_root":       ".",
			"git_branch":      "main",
			"git_dirty":       false,
			"head_sha":        "deadbeef",
			"modified_files":  []string{},
			"staged_files":    []string{},
			"unstaged_files":  []string{},
			"untracked_files": []string{},
		},
		"evidence": map[string]any{
			"artifacts": []string{"proof.md"},
			"status":    "complete",
			"summary":   "completion proof fixture",
		},
	}
	data, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		t.Fatalf("marshal evidence-only closure packet: %v", err)
	}
	if err := os.WriteFile(packetPath, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write evidence-only closure packet: %v", err)
	}
	return packetPath
}

func findRepoFileForTest(t *testing.T, parts ...string) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(append([]string{dir}, parts...)...)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo file %s", filepath.Join(parts...))
		}
		dir = parent
	}
}

func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}
