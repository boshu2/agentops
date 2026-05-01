package evalsubstrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteAtomic implements the §4 Run-manifest atomic-write contract:
//  1. Write data to <path>.tmp
//  2. fsync(temp_fd) — durability of file body BEFORE rename
//  3. Atomic rename <path>.tmp → <path>
//  4. fsync(parent_dir_fd) — durability of the rename itself
func WriteAtomic(path string, data []byte) error {
	if path == "" {
		return errors.New("WriteAtomic: empty path")
	}
	tmp := path + ".tmp"
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("WriteAtomic: mkdir parent: %w", err)
	}
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("WriteAtomic: open temp: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("WriteAtomic: write temp: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("WriteAtomic: fsync temp: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("WriteAtomic: close temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("WriteAtomic: rename: %w", err)
	}
	if err := fsyncDir(parent); err != nil {
		return fmt.Errorf("WriteAtomic: fsync parent: %w", err)
	}
	return nil
}

func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		// macOS APFS sometimes returns EINVAL on dir Sync — accept as no-op.
		return nil
	}
	return nil
}

// SweepTempFiles removes orphan `*.tmp` files older than maxAgeSeconds.
// Used by `ao eval cleanup --tmp-files` to recover from rename-step crashes.
func SweepTempFiles(root string, maxAgeSeconds int64) ([]string, error) {
	var removed []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".tmp" {
			return nil
		}
		if maxAgeSeconds > 0 {
			ageSec := timeNowUnix() - info.ModTime().Unix()
			if ageSec < maxAgeSeconds {
				return nil
			}
		}
		if rerr := os.Remove(path); rerr == nil {
			removed = append(removed, path)
		}
		return nil
	})
	if err != nil {
		return removed, fmt.Errorf("SweepTempFiles: walk %q: %w", root, err)
	}
	return removed, nil
}
