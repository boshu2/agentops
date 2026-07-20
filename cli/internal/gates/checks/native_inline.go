package checks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// Native ports of the bash gate's INLINE checks (no backing script). ag-3n71 PB1.

func init() {
	gates.Register(gates.Check{ID: "go.vet", Tiers: gates.Fast | gates.Full, Match: []string{"cli/**", "go.mod", "go.sum"}, Blocking: true, Run: runGoVet})
	gates.Register(gates.Check{ID: "changelog.sync", Tiers: gates.Fast | gates.Full, Match: []string{"CHANGELOG.md", "docs/CHANGELOG.md"}, Blocking: true, Run: runChangelogSync})
	gates.Register(gates.Check{ID: "shell.shellcheck-changed", Tiers: gates.Fast | gates.Full, Match: []string{"**/*.sh"}, Blocking: true, Run: runShellcheckChanged})
	gates.Register(gates.Check{ID: "learning.coherence", Tiers: gates.Fast | gates.Full, Match: learningRootGlobs, Blocking: true, Run: runLearningCoherence})
}

// changedFilesFor returns the routed change set, falling back to the diff vs
// origin/main when the orchestrator didn't route (Full mode).
func changedFilesFor(ctx context.Context, rc gates.RunContext) []string {
	if len(rc.ChangedFiles) > 0 {
		return rc.ChangedFiles
	}
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "origin/main...HEAD")
	cmd.Dir = rc.RepoRoot
	// Scrub git's hook-injected discovery env (GIT_DIR, ...) so a leaked GIT_DIR
	// cannot route this fallback change set at the wrong repo (SECURITY-MED).
	cmd.Env = gates.ScrubbedGitEnv()
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files
}

func runGoVet(ctx context.Context, rc gates.RunContext) (ports.GateVerdict, error) {
	cmd := exec.CommandContext(ctx, "go", "vet", "./...")
	cmd.Dir = filepath.Join(rc.RepoRoot, "cli")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return ports.GateVerdict{Status: ports.GateStatusFail, Reason: "go vet ./... failed", LogTail: tail(out.String(), 4096)}, nil
	}
	return ports.GateVerdict{Status: ports.GateStatusPass, Reason: "go vet ./... ok"}, nil
}

func runChangelogSync(_ context.Context, rc gates.RunContext) (ports.GateVerdict, error) {
	root := filepath.Join(rc.RepoRoot, "CHANGELOG.md")
	docs := filepath.Join(rc.RepoRoot, "docs", "CHANGELOG.md")
	rb, rerr := os.ReadFile(root)
	db, derr := os.ReadFile(docs)
	if rerr != nil {
		return ports.GateVerdict{Status: ports.GateStatusFail, Reason: fmt.Sprintf("read CHANGELOG.md: %v", rerr)}, nil
	}
	if derr != nil {
		return ports.GateVerdict{Status: ports.GateStatusFail, Reason: fmt.Sprintf("read docs/CHANGELOG.md: %v", derr)}, nil
	}
	if !bytes.Equal(rb, db) {
		return ports.GateVerdict{Status: ports.GateStatusFail, Reason: "CHANGELOG.md != docs/CHANGELOG.md (run: cp CHANGELOG.md docs/CHANGELOG.md)"}, nil
	}
	return ports.GateVerdict{Status: ports.GateStatusPass, Reason: "CHANGELOG in sync"}, nil
}

func runShellcheckChanged(ctx context.Context, rc gates.RunContext) (ports.GateVerdict, error) {
	if _, err := exec.LookPath("shellcheck"); err != nil {
		// Fail closed: this is a BLOCKING check over shell files. A SKIP clears a
		// blocking check (orchestrator.isBlockingFail), so returning SKIP here
		// would let a missing shellcheck silently pass changed .sh files. Treat
		// absence like ScriptRunner treats UNKNOWN — a FAIL (SECURITY-MED).
		return ports.GateVerdict{Status: ports.GateStatusFail, Reason: "shellcheck not installed: cannot verify changed shell files (install shellcheck)"}, nil
	}
	var failed []string
	var logs bytes.Buffer
	for _, f := range changedFilesFor(ctx, rc) {
		if !strings.HasSuffix(f, ".sh") {
			continue
		}
		if _, err := os.Stat(filepath.Join(rc.RepoRoot, f)); err != nil {
			continue // deleted
		}
		cmd := exec.CommandContext(ctx, "shellcheck", "-S", "warning", f)
		cmd.Dir = rc.RepoRoot
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			failed = append(failed, f)
			logs.WriteString(out.String())
		}
	}
	if len(failed) > 0 {
		return ports.GateVerdict{Status: ports.GateStatusFail, Reason: "shellcheck failed: " + strings.Join(failed, ", "), LogTail: tail(logs.String(), 4096)}, nil
	}
	return ports.GateVerdict{Status: ports.GateStatusPass, Reason: "shellcheck clean (changed .sh)"}, nil
}

// Learnings live under BOTH roots: the canonical .agents/ao/learnings (where
// doctor's split fixer migrates files) and the legacy .agents/learnings. The
// routing globs and the Run func prefix filter must stay in lockstep — a root
// present in one but not the other either never routes or routes-then-skips.
var (
	learningRootPrefixes = []string{".agents/ao/learnings/", ".agents/learnings/"}
	learningRootGlobs    = []string{".agents/ao/learnings/**", ".agents/learnings/**"}
)

// underLearningRoot reports whether a repo-relative path sits under either
// learnings root.
func underLearningRoot(f string) bool {
	for _, p := range learningRootPrefixes {
		if strings.HasPrefix(f, p) {
			return true
		}
	}
	return false
}

func runLearningCoherence(ctx context.Context, rc gates.RunContext) (ports.GateVerdict, error) {
	var missing []string
	for _, f := range changedFilesFor(ctx, rc) {
		if !underLearningRoot(f) || !strings.HasSuffix(f, ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(rc.RepoRoot, f))
		if err != nil {
			continue // deleted
		}
		if !bytes.HasPrefix(data, []byte("---")) {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return ports.GateVerdict{Status: ports.GateStatusFail, Reason: "learnings missing frontmatter: " + strings.Join(missing, ", ")}, nil
	}
	return ports.GateVerdict{Status: ports.GateStatusPass, Reason: "learning frontmatter ok"}, nil
}
