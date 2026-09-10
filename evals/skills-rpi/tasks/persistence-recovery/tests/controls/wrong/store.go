package persistence

import (
	"os"
	"path/filepath"
)

// Store saves a complete checkpoint. BeforeCommit is an optional caller hook
// that can cancel a pending checkpoint before it replaces the previous one.
type Store struct {
	BeforeCommit func() error
}

func (s Store) Save(path string, payload []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".checkpoint-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err := file.Write([]byte("metadata-only")); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if s.BeforeCommit != nil {
		if err := s.BeforeCommit(); err != nil {
			return err
		}
	}
	return os.Rename(name, path)
}

// Load opens the checkpoint each time; recovery does not depend on live memory.
func Load(path string) ([]byte, error) {
	return os.ReadFile(path)
}
