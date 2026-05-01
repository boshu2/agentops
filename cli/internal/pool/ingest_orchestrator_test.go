package pool

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestIterateIngestFiles_PerFileIndependence verifies that a read error on
// one file does not stop sibling files from being processed.
func TestIterateIngestFiles_PerFileIndependence(t *testing.T) {
	tmp := t.TempDir()
	good1 := filepath.Join(tmp, "a.md")
	good2 := filepath.Join(tmp, "b.md")
	missing := filepath.Join(tmp, "missing.md")

	if err := os.WriteFile(good1, []byte("body-a"), 0600); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(good2, []byte("body-b"), 0600); err != nil {
		t.Fatalf("write b: %v", err)
	}

	var ingested []string
	var readErrors []string

	IterateIngestFiles([]string{good1, missing, good2}, IngestOrchestratorOpts{
		IngestFile: func(path string, data []byte) bool {
			ingested = append(ingested, filepath.Base(path))
			return false
		},
		OnReadError: func(path string, err error) {
			readErrors = append(readErrors, filepath.Base(path))
		},
	})

	sort.Strings(ingested)
	if got, want := ingested, []string{"a.md", "b.md"}; !equalStrSlice(got, want) {
		t.Fatalf("ingested=%v, want %v", got, want)
	}
	if got, want := readErrors, []string{"missing.md"}; !equalStrSlice(got, want) {
		t.Fatalf("readErrors=%v, want %v", got, want)
	}
}

// TestIterateIngestFiles_MovesProcessedOnSuccess verifies that the move
// callback receives every file that ingested without errors when
// TrackProcessed is enabled.
func TestIterateIngestFiles_MovesProcessedOnSuccess(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.md")
	b := filepath.Join(tmp, "b.md")
	c := filepath.Join(tmp, "c.md")
	for _, p := range []string{a, b, c} {
		if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	var moved []string
	IterateIngestFiles([]string{a, b, c}, IngestOrchestratorOpts{
		IngestFile: func(path string, data []byte) bool {
			// Mark b as having an error.
			return filepath.Base(path) == "b.md"
		},
		TrackProcessed: true,
		MoveProcessed: func(processed []string) {
			for _, p := range processed {
				moved = append(moved, filepath.Base(p))
			}
		},
	})

	sort.Strings(moved)
	if got, want := moved, []string{"a.md", "c.md"}; !equalStrSlice(got, want) {
		t.Fatalf("moved=%v, want %v", got, want)
	}
}

// TestIterateIngestFiles_TrackProcessedDisabled verifies that no move is
// invoked when TrackProcessed=false even if IngestFile returns no errors.
// This preserves the historical close-loop behavior of leaving files in
// pending/ (the only caller that should move is `ao pool ingest`).
func TestIterateIngestFiles_TrackProcessedDisabled(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.md")
	if err := os.WriteFile(a, []byte("x"), 0600); err != nil {
		t.Fatalf("write a: %v", err)
	}

	var moveCalled bool
	IterateIngestFiles([]string{a}, IngestOrchestratorOpts{
		IngestFile:     func(path string, data []byte) bool { return false },
		TrackProcessed: false,
		MoveProcessed: func(processed []string) {
			moveCalled = true
		},
	})

	if moveCalled {
		t.Fatalf("MoveProcessed must not be called when TrackProcessed=false")
	}
}

// TestIterateIngestFiles_MoveSkippedWhenAllErrored verifies that
// MoveProcessed is not invoked when every file errored, even when
// TrackProcessed is true.
func TestIterateIngestFiles_MoveSkippedWhenAllErrored(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.md")
	if err := os.WriteFile(a, []byte("x"), 0600); err != nil {
		t.Fatalf("write a: %v", err)
	}

	var moveCalled bool
	IterateIngestFiles([]string{a}, IngestOrchestratorOpts{
		IngestFile:     func(path string, data []byte) bool { return true }, // always error
		TrackProcessed: true,
		MoveProcessed: func(processed []string) {
			moveCalled = true
		},
	})

	if moveCalled {
		t.Fatalf("MoveProcessed must not be called when all files errored")
	}
}

// TestIterateIngestFiles_PassesFileBytes verifies that IngestFile receives
// the actual file bytes (not a re-read or empty buffer).
func TestIterateIngestFiles_PassesFileBytes(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.md")
	want := []byte("hello world\n# Learning\nbody")
	if err := os.WriteFile(a, want, 0600); err != nil {
		t.Fatalf("write a: %v", err)
	}

	var got []byte
	IterateIngestFiles([]string{a}, IngestOrchestratorOpts{
		IngestFile: func(path string, data []byte) bool {
			got = append([]byte(nil), data...)
			return false
		},
	})

	if string(got) != string(want) {
		t.Fatalf("data=%q, want %q", string(got), string(want))
	}
}

// TestIterateIngestFiles_NoFiles verifies that an empty input list is a no-op
// — neither IngestFile nor MoveProcessed should be called.
func TestIterateIngestFiles_NoFiles(t *testing.T) {
	var ingested, moved bool
	IterateIngestFiles(nil, IngestOrchestratorOpts{
		IngestFile:     func(path string, data []byte) bool { ingested = true; return false },
		TrackProcessed: true,
		MoveProcessed:  func(processed []string) { moved = true },
	})
	if ingested || moved {
		t.Fatalf("empty input must be a no-op (ingested=%v moved=%v)", ingested, moved)
	}
}

// TestIterateIngestFiles_NilIngestFn verifies safety against a nil
// IngestFile callback (defensive guard — should not panic).
func TestIterateIngestFiles_NilIngestFn(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.md")
	if err := os.WriteFile(a, []byte("x"), 0600); err != nil {
		t.Fatalf("write a: %v", err)
	}
	// Should not panic.
	IterateIngestFiles([]string{a}, IngestOrchestratorOpts{IngestFile: nil})
}

func equalStrSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
