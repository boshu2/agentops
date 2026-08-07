package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanJSONLFile_MissingFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "absent.jsonl")
	called := false
	if err := ScanJSONLFile(path, func(line []byte) { called = true }); err != nil {
		t.Fatalf("ScanJSONLFile() error = %v, want nil", err)
	}
	if called {
		t.Fatal("fn called for a missing file")
	}
}

func TestScanJSONLFile_OpenError(t *testing.T) {
	t.Parallel()
	parentFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(parentFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := ScanJSONLFile(filepath.Join(parentFile, "child.jsonl"), func(line []byte) {})
	if err == nil {
		t.Fatal("expected open error for a path under a regular file")
	}
}

func TestScanJSONLFile_ReadsEveryLine(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "records.jsonl")
	if err := os.WriteFile(path, []byte("{\"a\":1}\n{\"a\":2}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var lines []string
	if err := ScanJSONLFile(path, func(line []byte) { lines = append(lines, string(line)) }); err != nil {
		t.Fatalf("ScanJSONLFile() error = %v, want nil", err)
	}
	want := []string{"{\"a\":1}", "{\"a\":2}"}
	if len(lines) != 2 || lines[0] != want[0] || lines[1] != want[1] {
		t.Fatalf("lines = %q, want %q", lines, want)
	}
}

func TestScanJSONL_LineTooLongIsLoud(t *testing.T) {
	t.Parallel()
	oversized := strings.NewReader("{\"pad\":\"" + strings.Repeat("x", scanJSONLMaxLine+2) + "\"}\n")
	err := ScanJSONL(oversized, func(line []byte) {})
	if !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("expected ErrLineTooLong, got %v", err)
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("error should name the offending line: %v", err)
	}
}
