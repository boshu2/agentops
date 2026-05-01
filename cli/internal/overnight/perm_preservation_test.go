package overnight

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Mirrors openclaw/snapshot_test.go's TestSnapshot_PermsPreservedAt0o600 by
// asserting that the four production callers in this package land their
// outputs at 0o644 — not whatever the runtime umask happens to produce. The
// underlying writer (quest.AtomicWriteFileWithPerm) has its own perm test;
// these are integration covers so a future caller swapping writer or perm
// arg is caught at the call site.
//
// Windows does not preserve POSIX read bits exactly; Go maps Chmod to the
// read-only attribute, so a requested 0o644 writable file is observed as 0o666.

func TestWriteCheckpointManifest_PermsPreservedAt0o644(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint", "manifest.json")
	manifest := CheckpointManifest{
		SchemaVersion: CheckpointManifestSchemaVersion,
		IterationID:   "iter-1",
		StagingDir:    filepath.Join(dir, "staging"),
		PrevDir:       filepath.Join(dir, "prev"),
		LiveDir:       filepath.Join(dir, "live"),
		MarkerPath:    filepath.Join(dir, "marker"),
		CreatedAt:     "2026-05-01T03:30:00Z",
	}
	if err := WriteCheckpointManifest(path, manifest); err != nil {
		t.Fatalf("WriteCheckpointManifest: %v", err)
	}
	assertPerm(t, path, 0o644)
}

func TestWriteReduceStageJobResult_PermsPreservedAt0o644(t *testing.T) {
	outputDir := t.TempDir()
	path, err := WriteReduceStageJobResult(outputDir, ReduceStageJobResult{
		SchemaVersion: 1,
		DreamRunID:    "dream-test",
		Stage:         "reduce",
		Status:        "success",
	})
	if err != nil {
		t.Fatalf("WriteReduceStageJobResult: %v", err)
	}
	assertPerm(t, path, 0o644)
}

func TestWriteMeasureStageJobResult_PermsPreservedAt0o644(t *testing.T) {
	outputDir := t.TempDir()
	path, err := WriteMeasureStageJobResult(outputDir, MeasureStageJobResult{
		SchemaVersion: 1,
		DreamRunID:    "dream-test",
		Stage:         "measure",
		Status:        "success",
	})
	if err != nil {
		t.Fatalf("WriteMeasureStageJobResult: %v", err)
	}
	assertPerm(t, path, 0o644)
}

func TestWriteCommitStageJobResult_PermsPreservedAt0o644(t *testing.T) {
	outputDir := t.TempDir()
	path, err := WriteCommitStageJobResult(outputDir, CommitStageJobResult{
		SchemaVersion: 1,
		DreamRunID:    "dream-test",
		Stage:         "commit",
		Status:        "success",
	})
	if err != nil {
		t.Fatalf("WriteCommitStageJobResult: %v", err)
	}
	assertPerm(t, path, 0o644)
}

func assertPerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	visibleWant := want
	if runtime.GOOS == "windows" && want == 0o644 {
		visibleWant = 0o666
	}
	if got := info.Mode().Perm(); got != visibleWant {
		t.Fatalf("perm %s = %o, want observable %o for requested %o", path, got, visibleWant, want)
	}
}
