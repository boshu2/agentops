package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// contextArtifactDir returns the path for context-scoped artifacts.
// If runID is empty, generates an adhoc identifier from the current timestamp.
// Called automatically when --for is used; uses RPI_RUN_ID if set.
func contextArtifactDir(runID string, randReader io.Reader) string {
	if randReader == nil {
		randReader = rand.Reader
	}
	if runID == "" {
		runID = newAdhocContextRunID(time.Now(), randReader)
	}
	return filepath.Join(".agents", "context", runID)
}

// newAdhocContextRunID returns a context run ID of the form
// `adhoc-<unix-seconds>-<4-hex>`. The 4-hex suffix is drawn from the
// crypto/rand source and gives ~1/65 536 collision probability between
// two calls inside the same second. When the entropy read fails the
// fallback uses the lower 16 bits of `now.UnixNano()` so the suffix is
// still unique within sub-second windows even on entropy-starved hosts.
//
// Even with the suffix, the timestamp remains second-granular: callers
// MUST treat the directory created by ensureContextDir as idempotent
// (os.MkdirAll → no-op when present), because two adhoc IDs minted in
// the same second with the same random suffix WILL share a path. This
// has been safe since the directory is created with MkdirAll and the
// suffix-collision rate is low; documenting it here so future callers
// don't try to use the path itself as a session-uniqueness signal.
func newAdhocContextRunID(now time.Time, r io.Reader) string {
	suffix := make([]byte, 2)
	if _, err := io.ReadFull(r, suffix); err != nil {
		return fmt.Sprintf("adhoc-%d-%04x", now.Unix(), uint16(now.UnixNano()))
	}
	return fmt.Sprintf("adhoc-%d-%s", now.Unix(), hex.EncodeToString(suffix))
}

// ensureContextDir creates the context artifact directory on disk.
// The mkdir is idempotent (os.MkdirAll), so two adhoc IDs that collide
// (~1/65 536 within the same second; see newAdhocContextRunID) reuse the
// same directory rather than erroring.
func ensureContextDir(cwd, runID string, randReader io.Reader) (string, error) {
	dir := filepath.Join(cwd, contextArtifactDir(runID, randReader))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create context dir: %w", err)
	}
	return dir, nil
}
