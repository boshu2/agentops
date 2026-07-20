// Package initapp owns the filesystem effects behind the `ao init` command. It
// creates the local AgentOps evidence and verdict directories, keeping the init
// command module a thin Cobra presentation seam that performs no direct
// filesystem effect. It never initializes Git, edits ignore files, installs
// hooks, selects work, or starts a runtime.
package initapp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/boshu2/agentops/cli/internal/storage"
)

// evidenceDirs is the fixed, ordered set of local directories `ao init` creates.
// It must cover both loop-evidence stores `ao status` reads (intents, verdicts)
// and the knowledge-store substructure `ao doctor` enforces (the storage
// package's sessions/index/provenance contract), so a fresh init is never
// flagged incomplete by the CLI's own diagnostics.
var evidenceDirs = []string{
	filepath.Join(storage.DefaultBaseDir, "intents", "sha256"),
	filepath.Join(storage.DefaultBaseDir, "verdicts", "sha256"),
	filepath.Join(storage.DefaultBaseDir, storage.SessionsDir),
	filepath.Join(storage.DefaultBaseDir, storage.IndexDir),
	filepath.Join(storage.DefaultBaseDir, storage.ProvenanceDir),
	filepath.Join(".agents", "handoff"),
}

// RunOptions carries the presentation choices resolved by the command module.
// The working directory is resolved inside Run so the module never performs a
// direct filesystem effect.
type RunOptions struct {
	// DryRun prints what would be created without creating anything.
	DryRun bool
	// Stdout receives the per-directory progress lines.
	Stdout io.Writer
}

// Run creates the local evidence and verdict directories, or reports what it
// would create under DryRun.
func Run(opts RunOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	for _, relative := range evidenceDirs {
		if opts.DryRun {
			fmt.Fprintf(opts.Stdout, "would create %s\n", relative)
			continue
		}
		if err := os.MkdirAll(filepath.Join(cwd, relative), 0o700); err != nil {
			return fmt.Errorf("create %s: %w", relative, err)
		}
		fmt.Fprintf(opts.Stdout, "created %s\n", relative)
	}
	return nil
}
