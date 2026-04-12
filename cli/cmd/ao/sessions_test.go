package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionsIndexPruneOrphansDeletesMissingSourcePage(t *testing.T) {
	repo := t.TempDir()
	sessionsDir := filepath.Join(repo, ".agents", "ao", "sessions")
	sourcePath := filepath.Join(repo, "transcripts", "live.jsonl")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	livePage := writeSessionPrunePage(t, sessionsDir, "live.md", sourcePath)
	orphanPage := writeSessionPrunePage(t, sessionsDir, "orphan.md", filepath.Join(repo, "transcripts", "missing.jsonl"))

	origProjectDir := testProjectDir
	testProjectDir = repo
	t.Cleanup(func() { testProjectDir = origProjectDir })

	out, err := executeCommand("sessions", "index", "--prune-orphans", "--sessions-dir", sessionsDir)
	if err != nil {
		t.Fatalf("sessions index --prune-orphans: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "Deleted 1 orphan session page") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if _, err := os.Stat(livePage); err != nil {
		t.Fatalf("live page should remain: %v", err)
	}
	if _, err := os.Stat(orphanPage); !os.IsNotExist(err) {
		t.Fatalf("orphan page should be deleted, stat err=%v", err)
	}
}

func TestSessionsIndexPruneOrphansDryRunDoesNotDelete(t *testing.T) {
	repo := t.TempDir()
	sessionsDir := filepath.Join(repo, ".agents", "ao", "sessions")
	orphanPage := writeSessionPrunePage(t, sessionsDir, "orphan.md", filepath.Join(repo, "missing.jsonl"))

	origProjectDir := testProjectDir
	testProjectDir = repo
	t.Cleanup(func() { testProjectDir = origProjectDir })

	out, err := executeCommand("sessions", "index", "--prune-orphans", "--sessions-dir", sessionsDir, "--dry-run")
	if err != nil {
		t.Fatalf("sessions index --prune-orphans --dry-run: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "Would delete 1 orphan session page") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	if _, err := os.Stat(orphanPage); err != nil {
		t.Fatalf("dry-run should leave orphan page: %v", err)
	}
}

func TestSessionsIndexPruneOrphansJSON(t *testing.T) {
	repo := t.TempDir()
	sessionsDir := filepath.Join(repo, ".agents", "ao", "sessions")
	writeSessionPrunePage(t, sessionsDir, "orphan.md", filepath.Join(repo, "missing.jsonl"))
	writeSessionPrunePage(t, sessionsDir, "unknown.md", "")

	origProjectDir := testProjectDir
	testProjectDir = repo
	t.Cleanup(func() { testProjectDir = origProjectDir })

	out, err := executeCommand("sessions", "index", "--prune-orphans", "--sessions-dir", sessionsDir, "--dry-run", "--json")
	if err != nil {
		t.Fatalf("sessions index --prune-orphans --json: %v\noutput:\n%s", err, out)
	}
	var result sessionsPruneOrphansResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput:\n%s", err, out)
	}
	if result.Scanned != 2 || result.WouldDelete != 1 || result.SkippedNoSource != 1 || len(result.Orphans) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSessionsIndexRequiresPruneOrphans(t *testing.T) {
	out, err := executeCommand("sessions", "index")
	if err == nil {
		t.Fatalf("sessions index succeeded, want explicit --prune-orphans error; output:\n%s", out)
	}
	if !strings.Contains(err.Error(), "only supports --prune-orphans") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeSessionPrunePage(t *testing.T, dir, name, source string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	sourceLine := ""
	if source != "" {
		sourceLine = "source_jsonl: " + source + "\n"
	}
	body := "---\ntype: session\n" + sourceLine + "status: reviewed\n---\n\n# Session\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
