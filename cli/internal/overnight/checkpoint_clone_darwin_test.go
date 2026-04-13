//go:build darwin

package overnight

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloneFileForCheckpoint_Darwin(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	mustWrite(t, src, "darwin clonefile")

	n, cloned, err := cloneFileForCheckpoint(src, dst, 0o600)
	if err != nil {
		t.Fatalf("cloneFileForCheckpoint: %v", err)
	}
	if !cloned {
		t.Skip("clonefile unavailable on temp filesystem")
	}
	if n != int64(len("darwin clonefile")) {
		t.Fatalf("bytes = %d, want %d", n, len("darwin clonefile"))
	}
	if got := mustRead(t, dst); got != "darwin clonefile" {
		t.Fatalf("dst content = %q", got)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("dst mode = %v, want 0600", info.Mode().Perm())
	}
}
