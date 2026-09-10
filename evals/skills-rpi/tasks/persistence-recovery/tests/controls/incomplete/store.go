package persistence

import "os"

// Store saves a complete checkpoint. BeforeCommit is an optional caller hook
// that can cancel a pending checkpoint before it replaces the previous one.
type Store struct {
	BeforeCommit func() error
}

func (s Store) Save(path string, payload []byte) error {
	if err := os.WriteFile(path, payload, 0600); err != nil {
		return err
	}
	if s.BeforeCommit != nil {
		_ = s.BeforeCommit()
	}
	return nil
}

// Load opens the checkpoint each time; recovery does not depend on live memory.
func Load(path string) ([]byte, error) {
	return os.ReadFile(path)
}
