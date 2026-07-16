package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
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
	// fsync the parent directory so the rename itself survives power loss: the
	// file body is durable (f.Sync above), but the directory entry that makes
	// the new name observable is not guaranteed on disk until the dir is synced.
	if err := FsyncDir(dir); err != nil {
		return fmt.Errorf("syncing parent dir %s: %w", dir, err)
	}
	return nil
}

// FsyncDir flushes a directory's metadata to disk by opening it and calling
// fsync, making a prior rename into that directory durable across power loss.
// It is the single dir-fsync implementation for the CLI; the atomic writers
// that do their own rename (storage.FileStorage, internal/pool, internal/llmwiki,
// internal/evalsubstrate) call this after the rename completes.
//
// A directory fsync that reports the operation as unsupported is treated as a
// no-op: some filesystems (notably macOS APFS) reject fsync on a directory
// handle with EINVAL/ENOTSUP, and the rename is already durable there, so that
// error is not one the caller can act on. A genuine I/O error (e.g. EIO) is
// returned rather than swallowed, so a real durability failure is not hidden.
// An open failure (e.g. the directory does not exist) is also returned so
// callers surface a genuinely missing path.
func FsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	// Windows does not expose a usable directory fsync through os.File.Sync:
	// FlushFileBuffers on the directory handle returns ERROR_ACCESS_DENIED.
	// Opening the directory above still preserves the missing-path/error
	// contract; the metadata barrier itself is unavailable on this platform.
	if runtime.GOOS == "windows" {
		return nil
	}
	if err := d.Sync(); err != nil && !dirFsyncUnsupported(err) {
		return err
	}
	return nil
}

// dirFsyncUnsupported reports whether err indicates the filesystem does not
// support fsync on a directory handle (and the rename is therefore already
// durable). macOS APFS returns EINVAL; other platforms may report ENOTSUP or
// ENOTTY. Real I/O errors are deliberately excluded so they propagate.
func dirFsyncUnsupported(err error) bool {
	return errors.Is(err, syscall.EINVAL) ||
		errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.ENOTTY)
}
