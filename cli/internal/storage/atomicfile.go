package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWriteFile is the canonical atomic file writer for the repo. It writes
// data to path durably and atomically: a same-directory temp file is written,
// fsynced, chmod'd to perm, closed, and renamed over the destination, so a
// concurrent reader sees either the previous content or the complete new
// content — never a partial write — and the bytes are on disk before the path
// becomes observable. The parent directory is created with 0755 if missing.
//
// The intermediate temp file is created via os.CreateTemp (mode 0o600 on Unix),
// so a wider requested perm is never observable on the filesystem until the
// rename completes. The temp file is removed on every error path.
//
// This is the single implementation behind the package-level atomic-write
// helpers elsewhere in the CLI (internal/types/quest, internal/llmwiki); new
// call sites should prefer this directly. It is the proven shape the
// fleet-lease fsync contract depends on (see internal/types/quest fault tests).
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := f.Chmod(perm); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
