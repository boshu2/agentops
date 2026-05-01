// Package llmwiki implements the Karpathy LLM-Wiki pattern as a daemon job
// executor. The Karpathy loop has four stages — Ingest, Query, Lint, Promote —
// applied conditionally based on wiki state on each tick.
//
// Per-stage idempotency contracts (pre-mortem amendment A3):
//
//   - INGEST: atomic write (tmp + fsync + rename). Re-claim after crash inspects
//     existing wiki/sources/<slug>.md and skips if already present.
//   - QUERY: same atomic-write + deterministic slug derived from query hash.
//   - LINT: overwrite IS the contract (date-keyed snapshots), but the writer
//     uses atomic-write to avoid partially-written lint reports being read by
//     downstream consumers; attempt is recorded in frontmatter.
//   - PROMOTE: git mv is atomic; INDEX/backlinks reconciled by the next tick of
//     wiki-promote-reconcile.
//
// The atomic-write helper in this file is non-negotiable for INGEST/QUERY/LINT;
// PROMOTE uses git mv directly (handled in stage handlers, soc-8inr.8).
package llmwiki

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// AtomicWriteFile writes contents to path atomically by creating a temporary
// file in the same directory, fsyncing it, and renaming over the destination.
// A partial write never leaves a corrupted file visible to readers.
//
// Per pre-mortem amendment A3.
func AtomicWriteFile(path string, contents []byte, mode os.FileMode) error {
	if path == "" {
		return fmt.Errorf("llmwiki: atomic write requires non-empty path")
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("llmwiki: create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	// Cleanup on error path; if rename succeeds, this Remove is a no-op
	// because the path no longer exists under tmpName.
	defer os.Remove(tmpName)

	if _, err := tmp.Write(contents); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("llmwiki: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("llmwiki: fsync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("llmwiki: close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("llmwiki: chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("llmwiki: rename temp file to %s: %w", path, err)
	}
	return nil
}

// AtomicWriteFromReader is the streaming variant of AtomicWriteFile. It copies
// from r into a temp file, fsyncs, then renames over path. If the reader
// returns an error mid-stream, the destination remains untouched and no temp
// file is left behind.
func AtomicWriteFromReader(path string, r io.Reader, mode os.FileMode) error {
	if path == "" {
		return fmt.Errorf("llmwiki: atomic write requires non-empty path")
	}
	if r == nil {
		return fmt.Errorf("llmwiki: atomic write requires non-nil reader")
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("llmwiki: create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("llmwiki: copy to temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("llmwiki: fsync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("llmwiki: close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("llmwiki: chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("llmwiki: rename temp file to %s: %w", path, err)
	}
	return nil
}
