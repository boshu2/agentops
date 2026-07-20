// practices: [hexagonal-architecture]
package main

import (
	"os"
	"path/filepath"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// resolveLedgerPath locates docs/provenance/ledger.jsonl relative to the
// repository root, walking up from cwd to find a directory containing a docs/
// dir or a .git entry. Falls back to a cwd-relative path so the error from the
// store is clear when no repo root is found.
//
// This host seam is shared: the provenance command module reads it via
// HostOptions.LedgerPath, and the doctor legacy-checks adapter reads it as its
// ledger-path resolver. It stays in package main (the composition root) so
// neither command module carries a direct filesystem effect.
func resolveLedgerPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return provenancegraph.LedgerRelativePath
	}
	dir := cwd
	for i := 0; i < 12; i++ {
		if isRepoDir(filepath.Join(dir, "docs")) && isRepoDir(filepath.Join(dir, "schemas")) {
			return filepath.Join(dir, provenancegraph.LedgerRelativePath)
		}
		if _, statErr := os.Stat(filepath.Join(dir, ".git")); statErr == nil {
			return filepath.Join(dir, provenancegraph.LedgerRelativePath)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(cwd, provenancegraph.LedgerRelativePath)
}

// isRepoDir reports whether p is an existing directory. It is a local helper so
// resolveLedgerPath stays self-contained and independent of the skills family's
// isDir (which is carved into internal/skillsapp).
func isRepoDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
