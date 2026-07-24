package evalsubstrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteAtomic implements the §4 Run-manifest atomic-write contract:
//  1. Write data to a unique sibling temporary
//  2. fsync(temp_fd) — durability of file body BEFORE rename
//  3. Atomic rename temp → <path>
//  4. fsync(parent_dir_fd) — durability of the rename itself
func WriteAtomic(path string, data []byte) error {
	if path == "" {
		return errors.New("WriteAtomic: empty path")
	}
	parent := filepath.Dir(path)
	store, err := CreateRootStore(parent, 0o755)
	if err != nil {
		return fmt.Errorf("WriteAtomic: %w", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.WriteAtomic(filepath.Base(path), data, 0o644); err != nil {
		return fmt.Errorf("WriteAtomic: %w", err)
	}
	return nil
}

// SweepTempFiles removes orphan legacy `*.tmp` and unique `*.tmp-*` files
// older than maxAgeSeconds.
// Used by `ao eval cleanup --tmp-files` to recover from rename-step crashes.
func SweepTempFiles(root string, maxAgeSeconds int64) ([]string, error) {
	var removed []string
	store, err := OpenRootStore(root)
	if err != nil {
		return removed, fmt.Errorf("SweepTempFiles: open root %q: %w", root, err)
	}
	defer func() { _ = store.Close() }()

	err = store.WalkDir(func(relative string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !isEvalTempName(entry.Name()) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if maxAgeSeconds > 0 {
			ageSec := timeNowUnix() - info.ModTime().Unix()
			if ageSec < maxAgeSeconds {
				return nil
			}
		}
		if relative == "." {
			return nil
		}
		if rerr := store.Remove(filepath.ToSlash(relative)); rerr == nil {
			removed = append(removed, filepath.Join(root, filepath.FromSlash(relative)))
		}
		return nil
	})
	if err != nil {
		return removed, fmt.Errorf("SweepTempFiles: walk %q: %w", root, err)
	}
	return removed, nil
}

func isEvalTempName(name string) bool {
	return strings.HasSuffix(name, ".tmp") || strings.Contains(name, ".tmp-")
}
