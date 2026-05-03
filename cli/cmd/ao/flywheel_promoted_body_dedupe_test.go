package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const stalePromotedBody = "When corpus cleanup archives a learning body, close-loop startup maintenance must compare pending knowledge against the promoted body hash before adding a pool candidate. This keeps stale pending files from recreating already-promoted artifacts after cleanup."

func writePendingLearningWithBody(t *testing.T, tmp, name, body string) string {
	t.Helper()
	pendingDir := filepath.Join(tmp, ".agents", "knowledge", "pending")
	if err := os.MkdirAll(pendingDir, 0o755); err != nil {
		t.Fatalf("mkdir pending: %v", err)
	}
	pendingFile := filepath.Join(pendingDir, name+".md")
	content := strings.Join([]string{
		"# Learnings: ag-" + name,
		"",
		"**Date:** 2026-01-01",
		"",
		"# Learning: " + name + " title",
		"",
		"**ID**: L-" + name,
		"**Category**: process",
		"**Confidence**: high",
		"",
		"## What We Learned",
		"",
		body,
		"",
		"## Source",
		"",
		"Session: ag-" + name,
		"",
	}, "\n")
	if err := os.WriteFile(pendingFile, []byte(content), 0o600); err != nil {
		t.Fatalf("write pending file: %v", err)
	}
	return pendingFile
}

func setupStalePendingAlreadyPromotedFixture(t *testing.T) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	writeArtifactFile(t, filepath.Join(tmp, ".agents", "learnings"), "2026-04-30-existing.md", stalePromotedBody)
	pendingFile := writePendingLearningWithBody(t, tmp, "2026-04-30-stale", stalePromotedBody)
	return tmp, pendingFile
}

func setupStalePendingArchivedBodyFixture(t *testing.T) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	archiveDir := filepath.Join(tmp, ".agents", "defrag", "2026-05-01-artifact-dedup", "files", ".agents", "learnings")
	writeArtifactFile(t, archiveDir, "2026-04-30-archived.md", stalePromotedBody)
	pendingFile := writePendingLearningWithBody(t, tmp, "2026-04-30-archived-stale", stalePromotedBody)
	return tmp, pendingFile
}

func assertSinglePromotedArtifact(t *testing.T, tmp string) {
	t.Helper()
	files, err := collectReindexFiles(tmp)
	if err != nil {
		t.Fatalf("collect artifacts: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("promoted artifacts = %d, want 1: %v", len(files), files)
	}
	if filepath.Base(files[0]) != "2026-04-30-existing.md" {
		t.Fatalf("promoted artifact = %s, want existing artifact only", files[0])
	}
}

func assertPendingMovedToProcessed(t *testing.T, tmp, pendingFile string) {
	t.Helper()
	if _, err := os.Stat(pendingFile); !os.IsNotExist(err) {
		t.Fatalf("pending file still exists at %s (err=%v)", pendingFile, err)
	}
	processed := filepath.Join(tmp, ".agents", "knowledge", "processed", filepath.Base(pendingFile))
	if _, err := os.Stat(processed); err != nil {
		t.Fatalf("processed pending file missing at %s: %v", processed, err)
	}
}

func assertDuplicateSkipAudited(t *testing.T, tmp string) {
	t.Helper()
	chainPath := filepath.Join(tmp, ".agents", "pool", "chain.jsonl")
	data, err := os.ReadFile(chainPath)
	if err != nil {
		t.Fatalf("read pool chain audit: %v", err)
	}
	chain := string(data)
	if !strings.Contains(chain, "already-promoted content") {
		t.Fatalf("pool chain missing duplicate skip reason:\n%s", chain)
	}
}

func TestCloseLoopSkipsStalePendingAlreadyArchivedBody(t *testing.T) {
	tmp, pendingFile := setupStalePendingArchivedBodyFixture(t)

	result, err := performFlywheelCloseLoop(tmp, filepath.Join(".agents", "knowledge", "pending"), 0, true)
	if err != nil {
		t.Fatalf("performFlywheelCloseLoop: %v", err)
	}
	if result.Ingest.Added != 0 {
		t.Fatalf("ingest Added=%d, want 0 for already-archived body (res=%+v)", result.Ingest.Added, result.Ingest)
	}
	if result.AutoPromote.Promoted != 0 {
		t.Fatalf("AutoPromote.Promoted=%d, want 0", result.AutoPromote.Promoted)
	}
	assertPendingMovedToProcessed(t, tmp, pendingFile)
	assertDuplicateSkipAudited(t, tmp)
	assertSinglePromotedArtifactCount(t, tmp, 0)
}

func assertSinglePromotedArtifactCount(t *testing.T, tmp string, want int) {
	t.Helper()
	files, err := collectReindexFiles(tmp)
	if err != nil {
		t.Fatalf("collect artifacts: %v", err)
	}
	if len(files) != want {
		t.Fatalf("promoted artifacts = %d, want %d: %v", len(files), want, files)
	}
}

func TestCloseLoopDryRunSkipsStalePendingAlreadyPromotedBody(t *testing.T) {
	tmp, pendingFile := setupStalePendingAlreadyPromotedFixture(t)

	oldDryRun := dryRun
	dryRun = true
	t.Cleanup(func() { dryRun = oldDryRun })

	result, err := performFlywheelCloseLoop(tmp, filepath.Join(".agents", "knowledge", "pending"), 0, true)
	if err != nil {
		t.Fatalf("performFlywheelCloseLoop dry-run: %v", err)
	}
	if result.Ingest.Added != 0 {
		t.Fatalf("dry-run ingest Added=%d, want 0 for already-promoted body (res=%+v)", result.Ingest.Added, result.Ingest)
	}
	if result.AutoPromote.Promoted != 0 {
		t.Fatalf("dry-run AutoPromote.Promoted=%d, want 0", result.AutoPromote.Promoted)
	}
	assertSinglePromotedArtifact(t, tmp)
	if _, err := os.Stat(pendingFile); err != nil {
		t.Fatalf("dry-run should leave pending file in place: %v", err)
	}
}

func TestCloseLoopSkipsAndAuditsStalePendingAlreadyPromotedBody(t *testing.T) {
	tmp, pendingFile := setupStalePendingAlreadyPromotedFixture(t)

	result, err := performFlywheelCloseLoop(tmp, filepath.Join(".agents", "knowledge", "pending"), 0, true)
	if err != nil {
		t.Fatalf("performFlywheelCloseLoop: %v", err)
	}
	if result.Ingest.Added != 0 {
		t.Fatalf("ingest Added=%d, want 0 for already-promoted body (res=%+v)", result.Ingest.Added, result.Ingest)
	}
	if result.AutoPromote.Promoted != 0 {
		t.Fatalf("AutoPromote.Promoted=%d, want 0", result.AutoPromote.Promoted)
	}
	assertSinglePromotedArtifact(t, tmp)
	assertPendingMovedToProcessed(t, tmp, pendingFile)
	assertDuplicateSkipAudited(t, tmp)

	second, err := performFlywheelCloseLoop(tmp, filepath.Join(".agents", "knowledge", "pending"), 0, true)
	if err != nil {
		t.Fatalf("second performFlywheelCloseLoop: %v", err)
	}
	if second.Ingest.Added != 0 || second.AutoPromote.Promoted != 0 {
		t.Fatalf("second close-loop not idempotent: %+v", second)
	}
	assertSinglePromotedArtifact(t, tmp)
}

func TestCodexEnsureStartMaintenanceSkipsStalePendingAlreadyPromotedBody(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_THREAD_ID", "codex-stale-pending-promoted-body")
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "Codex Desktop")

	tmp, pendingFile := setupStalePendingAlreadyPromotedFixture(t)
	if err := os.WriteFile(filepath.Join(tmp, "AGENTS.md"), []byte("# Test repo\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	t.Chdir(tmp)
	out, err := executeCommand("codex", "ensure-start", "--json", "--query", "stale pending promoted body")
	if err != nil {
		t.Fatalf("codex ensure-start: %v\noutput: %s", err, out)
	}
	var first codexEnsureStartResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &first); err != nil {
		t.Fatalf("parse ensure-start json: %v\noutput: %s", err, out)
	}
	if !first.Performed {
		t.Fatalf("ensure-start Performed=false, want true: %+v", first)
	}

	assertSinglePromotedArtifact(t, tmp)
	assertPendingMovedToProcessed(t, tmp, pendingFile)
	assertDuplicateSkipAudited(t, tmp)

	before, err := os.ReadFile(filepath.Join(tmp, ".agents", "ao", "codex", "state.json"))
	if err != nil {
		t.Fatalf("read codex state: %v", err)
	}
	time.Sleep(time.Millisecond)
	secondOut, err := executeCommand("codex", "ensure-start", "--json", "--query", "stale pending promoted body")
	if err != nil {
		t.Fatalf("second codex ensure-start: %v\noutput: %s", err, secondOut)
	}
	var second codexEnsureStartResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(secondOut)), &second); err != nil {
		t.Fatalf("parse second ensure-start json: %v\noutput: %s", err, secondOut)
	}
	if second.Performed {
		t.Fatalf("second ensure-start Performed=true, want false: %+v", second)
	}
	after, err := os.ReadFile(filepath.Join(tmp, ".agents", "ao", "codex", "state.json"))
	if err != nil {
		t.Fatalf("read codex state after second ensure-start: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("ensure-start idempotency changed state\nbefore:\n%s\nafter:\n%s", string(before), string(after))
	}
	assertSinglePromotedArtifact(t, tmp)
}
