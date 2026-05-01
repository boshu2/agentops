package evalsubstrate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAtomic_WritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := WriteAtomic(path, []byte(`{"status":"pending"}`)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"status":"pending"}` {
		t.Fatalf("unexpected content: %s", got)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("tmp not cleaned up: %v", err)
	}
}

func TestWriteAtomic_OverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.json")
	if err := WriteAtomic(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("second")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "second" {
		t.Fatalf("got %s", got)
	}
}

func TestWriteAtomic_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested/deep/manifest.json")
	if err := WriteAtomic(path, []byte("ok")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestWriteAtomic_EmptyPathErrors(t *testing.T) {
	if err := WriteAtomic("", []byte("x")); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestSweepTempFiles_RemovesOrphans(t *testing.T) {
	dir := t.TempDir()
	orphan := filepath.Join(dir, "run-x", "manifest.json.tmp")
	if err := os.MkdirAll(filepath.Dir(orphan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphan, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute)
	_ = os.Chtimes(orphan, old, old)

	removed, err := SweepTempFiles(dir, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != orphan {
		t.Fatalf("expected removal of %q, got %v", orphan, removed)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("orphan still present")
	}
}

func TestSweepTempFiles_RespectsMinAge(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "manifest.json.tmp")
	if err := os.WriteFile(fresh, []byte("in-flight"), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := SweepTempFiles(dir, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Fatalf("fresh tmp should be preserved, got %v", removed)
	}
}

func TestSweepTempFiles_IgnoresNonTmp(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(keep, []byte(`{"x":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute)
	_ = os.Chtimes(keep, old, old)
	removed, err := SweepTempFiles(dir, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Fatalf("non-tmp files should not be swept: %v", removed)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal(err)
	}
}
