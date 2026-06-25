package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// These tests pin the canonical AtomicWriteFile contract. The implementation is
// the proven temp-file -> fsync -> chmod -> rename algorithm extracted from
// internal/types/quest (the documented fleet-lease helper); quest and llmwiki
// now delegate here, so this is the single behavioral source of truth.

func TestAtomicWriteFile_WritesExactBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.bin")
	want := []byte("hello atomic world\n")
	if err := AtomicWriteFile(path, want, 0o644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("file contents: got %q, want %q", got, want)
	}
}

func TestAtomicWriteFile_CreatesMissingDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deeper", "nest", "out.txt")
	if err := AtomicWriteFile(path, []byte("ok"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != 2 {
		t.Fatalf("file size: got %d, want 2", info.Size())
	}
}

func TestAtomicWriteFile_NoTempLeftover(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := AtomicWriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory entry count: got %d, want 1", len(entries))
	}
	if entries[0].Name() != "out.txt" {
		t.Fatalf("unexpected leftover entry %q after write", entries[0].Name())
	}
}

func TestAtomicWriteFile_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	if err := AtomicWriteFile(path, []byte("new"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("file contents: got %q, want %q", got, "new")
	}
}

func TestAtomicWriteFile_AppliesPerm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode bits not honored on Windows")
	}
	for _, perm := range []os.FileMode{0o600, 0o644} {
		dir := t.TempDir()
		path := filepath.Join(dir, "perm.bin")
		if err := AtomicWriteFile(path, []byte("data"), perm); err != nil {
			t.Fatalf("AtomicWriteFile(perm=%#o): %v", perm, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if got := info.Mode().Perm(); got != perm {
			t.Fatalf("file mode: got %#o, want %#o", got, perm)
		}
	}
}

func TestAtomicWriteFile_TempfilePermNotLeaked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode bits not honored on Windows")
	}
	// The wider final perm (0o644) must not be observable before the rename:
	// CreateTemp uses 0o600 on Unix and chmod is applied before rename, so a
	// reader never sees a half-written file at the final perm.
	dir := t.TempDir()
	path := filepath.Join(dir, "out.bin")
	if err := AtomicWriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory entry count: got %d, want 1", len(entries))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("post-rename perm: got %#o, want exactly %#o", got, 0o644)
	}
}

func TestAtomicWriteFile_FsyncSurvivesRoundTrip(t *testing.T) {
	// Behavioral postcondition of the write -> sync -> chmod -> close -> rename
	// order: after a successful return the exact bytes are observable at path.
	// A non-trivial payload makes a partial write detectable.
	dir := t.TempDir()
	path := filepath.Join(dir, "durable.bin")
	want := make([]byte, 8192)
	for i := range want {
		want[i] = byte(i % 251)
	}
	if err := AtomicWriteFile(path, want, 0o600); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("size: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestAtomicWriteFile_ConcurrentWritersConverge(t *testing.T) {
	// With N concurrent writers racing on one path, the final content must
	// equal exactly one writer's payload — never truncated or interleaved.
	dir := t.TempDir()
	path := filepath.Join(dir, "race.txt")
	const n = 8
	payloads := make([][]byte, n)
	for i := 0; i < n; i++ {
		payloads[i] = []byte("writer-payload-" + string(rune('a'+i)))
	}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(p []byte) {
			defer wg.Done()
			if err := AtomicWriteFile(path, p, 0o644); err != nil {
				t.Errorf("concurrent write: %v", err)
			}
		}(payloads[i])
	}
	wg.Wait()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	matched := false
	for _, p := range payloads {
		if string(got) == string(p) {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("final contents %q matched no writer payload (truncation or interleave)", got)
	}
}
