// Package initapp owns the filesystem effects behind the `ao init` command. It
// creates the local AgentOps evidence and verdict directories and writes one
// idempotent ignore block into the working directory's .gitignore, keeping the
// init command module a thin Cobra presentation seam that performs no direct
// filesystem effect. It never initializes Git, installs hooks, selects work, or
// starts a runtime.
package initapp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

// The ignore policy.
//
// Before this, `ao init` scaffolded .agents/ao/** and said nothing about git.
// One loop later the working tree was littered with untracked scratch — a
// derived search index, local session transcripts, a machine-local provenance
// ledger, Python bytecode caches — and every user had to invent the same
// .gitignore lines. So init now writes them once, as a marker-delimited block.
//
// What is ignored is only machine-local scratch: derived, private, or
// per-machine files that would conflict on merge and mean nothing in another
// checkout. What is deliberately NOT ignored is the loop's evidence —
// .agents/ao/intents/ and .agents/ao/verdicts/ — because whether an intent
// digest or a verdict.v2 belongs in version control is the consumer
// repository's policy, and AgentOps owns no policy there (product boundary).
// A repository that wants everything tracked deletes the block; a repository
// that wants evidence ignored adds its own lines outside it.
const (
	// GitignoreBeginMarker opens the managed block. It is the idempotency key:
	// its presence anywhere in .gitignore means the block is already installed,
	// and init leaves the file untouched — including when a user has edited the
	// lines inside, which is an intentional local override, not drift to repair.
	GitignoreBeginMarker = "# --- AgentOps (ao init): machine-local scratch ---"
	// GitignoreEndMarker closes the managed block so a later tool (or a human)
	// can find its bounds.
	GitignoreEndMarker = "# --- end AgentOps ---"
)

// gitignoreBlock is the exact text appended to .gitignore, marker to marker.
var gitignoreBlock = strings.Join([]string{
	GitignoreBeginMarker,
	"# Derived, private, or per-machine files the AgentOps loop writes.",
	"# NOT ignored, on purpose: .agents/ao/intents/ and .agents/ao/verdicts/ —",
	"# whether to commit loop evidence is your repository's policy, not the CLI's.",
	"# Delete this block to track everything.",
	".agents/ao/index/",
	".agents/ao/sessions/",
	".agents/ao/provenance/",
	"__pycache__/",
	GitignoreEndMarker,
	"",
}, "\n")

// RunOptions carries the presentation choices resolved by the command module.
// The working directory is resolved inside Run so the module never performs a
// direct filesystem effect.
type RunOptions struct {
	// DryRun prints what would be created without creating anything.
	DryRun bool
	// Stdout receives the per-directory progress lines.
	Stdout io.Writer
}

// Run creates the local evidence and verdict directories and installs the
// managed .gitignore block, or reports what it would do under DryRun.
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
	return ensureGitignoreBlock(cwd, opts)
}

// ensureGitignoreBlock appends the managed ignore block to <dir>/.gitignore,
// creating the file when absent. It is idempotent on GitignoreBeginMarker: a
// second `ao init` in the same directory appends nothing, so the block is
// present exactly once no matter how many times init runs.
//
// The file targeted is the working directory's, not the enclosing git root's —
// the block's patterns are relative to the same directory whose .agents/ao/**
// this run just created, so writing it anywhere else would produce patterns
// that match nothing.
func ensureGitignoreBlock(dir string, opts RunOptions) error {
	path := filepath.Join(dir, ".gitignore")
	existing, err := os.ReadFile(path) // #nosec G304 -- path is <cwd>/.gitignore, not caller-controlled.
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read .gitignore: %w", err)
	}
	if strings.Contains(string(existing), GitignoreBeginMarker) {
		fmt.Fprintln(opts.Stdout, ".gitignore already has the AgentOps block (left unchanged)")
		return nil
	}
	if opts.DryRun {
		fmt.Fprintln(opts.Stdout, "would append the AgentOps block to .gitignore")
		return nil
	}
	content := string(existing)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	content += gitignoreBlock
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { // #nosec G306 -- .gitignore is a tracked, world-readable repo file.
		return fmt.Errorf("write .gitignore: %w", err)
	}
	fmt.Fprintln(opts.Stdout, "appended the AgentOps block to .gitignore")
	return nil
}
