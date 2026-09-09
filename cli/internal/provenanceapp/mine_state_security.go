package provenanceapp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// The canonical writer replaces the inode. Before giving it checkpoint data,
// compare the existing permissions with an empty replacement prepared the same
// way. This includes inherited ACLs and ownership, not just permission bits.
// Unsupported or different security metadata fails closed. Like the writer,
// this assumes the caller controls concurrent changes to the destination.
func checkCheckpointReplacement(path string, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	original, err := checkpointSecurity(path, info)
	if err != nil {
		return fmt.Errorf("inspect checkpoint permissions: %w", err)
	}
	probe, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("inspect replacement permissions: %w", err)
	}
	defer func() {
		_ = probe.Close()
		_ = os.Remove(probe.Name())
	}()
	if err := probe.Chmod(mode); err != nil {
		return err
	}
	probeInfo, err := probe.Stat()
	if err != nil {
		return err
	}
	replacement, err := checkpointSecurity(probe.Name(), probeInfo)
	if err != nil {
		return fmt.Errorf("inspect replacement permissions: %w", err)
	}
	if !bytes.Equal(original, replacement) {
		return fmt.Errorf("checkpoint permissions cannot be preserved by atomic replacement: %s", path)
	}
	return nil
}
