package llmwiki

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// errAfterNReader is an io.Reader that yields N bytes then returns an error.
// Used to simulate a stream failure mid-write so we can verify the original
// file is not corrupted and no temp files remain.
type errAfterNReader struct {
	data []byte
	n    int
	pos  int
	err  error
}

func (r *errAfterNReader) Read(p []byte) (int, error) {
	if r.pos >= r.n {
		return 0, r.err
	}
	remaining := r.n - r.pos
	end := len(p)
	if end > remaining {
		end = remaining
	}
	if end > len(r.data)-r.pos {
		end = len(r.data) - r.pos
	}
	copy(p, r.data[r.pos:r.pos+end])
	r.pos += end
	return end, nil
}

func TestAtomicWriteFile_BasicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	want := []byte("hello world")
	if err := AtomicWriteFile(path, want, 0o644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("content mismatch: got %q want %q", got, want)
	}
}

func TestAtomicWriteFile_OverwriteDoesNotCorruptOnPartialWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	originalA := []byte("CONTENT-A-original-and-stable")
	if err := os.WriteFile(path, originalA, 0o644); err != nil {
		t.Fatalf("seed WriteFile: %v", err)
	}

	contentB := []byte("CONTENT-B-replacement-that-fails-mid-stream")
	failingReader := &errAfterNReader{
		data: contentB,
		n:    8, // emit 8 bytes then fail
		err:  errors.New("simulated mid-stream failure"),
	}

	err := AtomicWriteFromReader(path, failingReader, 0o644)
	if err == nil {
		t.Fatal("AtomicWriteFromReader: expected error, got nil")
	}

	// Original file must remain content A.
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	if string(got) != string(originalA) {
		t.Fatalf("destination corrupted: got %q want %q", got, originalA)
	}

	// No temp files left in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("temp file leaked: %s", e.Name())
		}
	}
}

func TestAtomicWriteFile_PreservesMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode bits not honored on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "perm.md")
	if err := AtomicWriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	mode := info.Mode().Perm()
	if mode != 0o600 {
		t.Fatalf("mode mismatch: got %o want %o", mode, 0o600)
	}
}

func TestAtomicWriteFile_CleansUpTempOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")

	failingReader := &errAfterNReader{
		data: []byte("payload-bytes"),
		n:    4,
		err:  errors.New("boom"),
	}

	err := AtomicWriteFromReader(path, failingReader, 0o644)
	if err == nil {
		t.Fatal("expected error from failing reader, got nil")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("temp file leaked: %s", e.Name())
		}
	}
}

func TestAtomicWriteFile_RejectsEmptyPath(t *testing.T) {
	if err := AtomicWriteFile("", []byte("x"), 0o644); err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

func TestAtomicWriteFromReader_RejectsNilReader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	if err := AtomicWriteFromReader(path, nil, 0o644); err == nil {
		t.Fatal("expected error for nil reader, got nil")
	}
}

func TestAtomicWriteFromReader_BasicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stream.md")
	want := []byte("streamed content")
	if err := AtomicWriteFromReader(path, bytes.NewReader(want), 0o644); err != nil {
		t.Fatalf("AtomicWriteFromReader: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("content mismatch: got %q want %q", got, want)
	}
}

// Verify io.EOF specifically does NOT abort the streaming path — it's the
// normal end-of-stream signal and the file should be written.
func TestAtomicWriteFromReader_EOFIsTerminator(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "eof.md")
	r := &errAfterNReader{data: []byte("ok"), n: 2, err: io.EOF}
	if err := AtomicWriteFromReader(path, r, 0o644); err != nil {
		t.Fatalf("AtomicWriteFromReader: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "ok" {
		t.Fatalf("got %q want %q", got, "ok")
	}
}
