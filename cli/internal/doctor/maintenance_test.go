package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGCDryRunReportsCandidatesWithoutDeleting(t *testing.T) {
	root := t.TempDir()
	oldDir := writeGCTestRun(t, root, "old", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	newDir := writeGCTestRun(t, root, "new", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	result, err := GC(root, cutoff, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Matched != 1 || result.Pruned != 0 || !result.DryRun {
		t.Fatalf("dry-run result=%+v", result)
	}
	for _, dir := range []string{oldDir, newDir} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("dry-run removed %s: %v", dir, err)
		}
	}

	result, err = GC(root, cutoff, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Matched != 1 || result.Pruned != 1 || result.DryRun {
		t.Fatalf("real result=%+v", result)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("old run still exists: %v", err)
	}
	if info, err := os.Stat(newDir); err != nil || !info.IsDir() {
		t.Fatalf("new run missing: %v", err)
	}
}

func writeGCTestRun(t *testing.T, root, name string, started time.Time) string {
	t.Helper()
	dir := filepath.Join(root, ".doctor", "runs", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(Report{StartedAt: started.Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "report.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
