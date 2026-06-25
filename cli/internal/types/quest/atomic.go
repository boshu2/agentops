package quest

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/storage"
)

// AtomicWriteYAML marshals v to YAML and atomically writes the result to
// path. Uses os.CreateTemp in the target directory plus os.Rename so a
// concurrent reader either sees the previous content or the new content,
// never a partial write. The target directory is created with 0755 if
// needed.
func AtomicWriteYAML(path string, v interface{}) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}
	return AtomicWriteFile(path, data)
}

// AtomicWriteFile atomically writes data to path via a same-directory
// temp file plus os.Rename. fsync is called before rename so the bytes
// are durable. The target directory is created with 0755 if needed. The
// file lands with the restrictive temp-file mode (0o600 on Unix); callers
// that need a specific mode use AtomicWriteFileWithPerm.
//
// This is the documented fleet-lease writer (see atomic_fault_test.go); it
// delegates to the canonical storage.AtomicWriteFile so the temp+fsync+rename
// algorithm has a single implementation across the CLI.
func AtomicWriteFile(path string, data []byte) error {
	return storage.AtomicWriteFile(path, data, 0o600)
}

// AtomicWriteFileWithPerm atomically writes data to path with the given
// file permissions, using the same temp-file + fsync + rename algorithm as
// AtomicWriteFile so the final entry lands with exactly the requested mode
// and no wider perm is observable until the rename completes. The target
// directory is created with 0755 if needed.
//
// Delegates to the canonical storage.AtomicWriteFile.
func AtomicWriteFileWithPerm(path string, data []byte, perm os.FileMode) error {
	return storage.AtomicWriteFile(path, data, perm)
}
