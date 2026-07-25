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
	"github.com/boshu2/agentops/cli/internal/subprocess"
)

// Native ports of the bash gate's INLINE checks (no backing script). ag-3n71 PB1.

type subprocessRunner func(context.Context, subprocess.Command) (subprocess.Result, error)

func init() {
	gates.Register(gates.Check{ID: "go.vet", Tiers: gates.Fast | gates.Full, Match: []string{"cli/**", "go.mod", "go.sum"}, Blocking: true, Run: runGoVet})
	gates.Register(gates.Check{ID: "changelog.sync", Tiers: gates.Fast | gates.Full, Match: []string{"CHANGELOG.md", "docs/CHANGELOG.md"}, Blocking: true, Run: runChangelogSync})
	gates.Register(gates.Check{ID: "shell.shellcheck-changed", Tiers: gates.Fast | gates.Full, Match: []string{"**/*.sh"}, Blocking: true, Run: runShellcheckChanged})
	gates.Register(gates.Check{ID: "learning.coherence", Tiers: gates.Fast | gates.Full, Match: learningRootGlobs, Blocking: true, Run: runLearningCoherence})
}

// changedFilesFor returns the routed change set, falling back to the diff vs
// origin/main when the orchestrator didn't route (Full mode). This best-effort
// routing hint intentionally collapses any subprocess or cleanup failure to an
// empty set; authoritative scoped routing supplies rc.ChangedFiles.
func changedFilesFor(ctx context.Context, rc gates.RunContext) []string {
	return changedFilesForWithRunner(ctx, rc, subprocess.Run)
}

func changedFilesForWithRunner(ctx context.Context, rc gates.RunContext, run subprocessRunner) []string {
	if len(rc.ChangedFiles) > 0 {
		return rc.ChangedFiles
	}
	result, err := run(ctx, subprocess.Command{
		Name:        "git",
		Args:        []string{"diff", "--name-only", "origin/main...HEAD"},
		Dir:         rc.RepoRoot,
		Env:         gates.ScrubbedGitEnv(),
		StdoutLimit: subprocess.CaptureLimit{HeadBytes: 4 * 1024 * 1024},
		StderrLimit: subprocess.CaptureLimit{TailBytes: 4096},
	})
	if err != nil || result.Stdout.Truncated {
		return nil
	}
	var files []string
	for _, l := range strings.Split(string(result.Stdout.Bytes()), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files
}

func runGoVet(ctx context.Context, rc gates.RunContext) (ports.GateVerdict, error) {
	result, err := subprocess.Run(ctx, subprocess.Command{
		Name:           "go",
		Args:           []string{"vet", "./..."},
		Dir:            filepath.Join(rc.RepoRoot, "cli"),
		CombinedOutput: true,
		OutputLimit:    subprocess.CaptureLimit{TailBytes: 4096},
	})
	if ctxErr := ctx.Err(); ctxErr != nil {
		if err != nil {
			return ports.GateVerdict{}, err
		}
		return ports.GateVerdict{}, ctxErr
	}
	if result.Cleanup.Failed() {
		return ports.GateVerdict{}, err
	}
	if err != nil {
		return ports.GateVerdict{Status: ports.GateStatusFail, Reason: "go vet ./... failed", LogTail: result.Combined.String()}, nil
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
	var logs string
	logsTruncated := false
	for _, f := range changedFilesFor(ctx, rc) {
		if !strings.HasSuffix(f, ".sh") {
			continue
		}
		if _, err := os.Stat(filepath.Join(rc.RepoRoot, f)); err != nil {
			continue // deleted
		}
		result, err := subprocess.Run(ctx, subprocess.Command{
			Name:           "shellcheck",
			Args:           []string{"-S", "warning", f},
			Dir:            rc.RepoRoot,
			CombinedOutput: true,
			OutputLimit:    subprocess.CaptureLimit{TailBytes: 4096},
		})
		if ctxErr := ctx.Err(); ctxErr != nil {
			if err != nil {
				return ports.GateVerdict{}, err
			}
			return ports.GateVerdict{}, ctxErr
		}
		if result.Cleanup.Failed() {
			return ports.GateVerdict{}, err
		}
		if err != nil {
			failed = append(failed, f)
			next := result.Combined.String()
			if result.Combined.Truncated || len(logs)+len(next) > 4096 {
				logsTruncated = true
			}
			logs = tail(logs+next, 4096)
		}
	}
	if len(failed) > 0 {
		if logsTruncated {
			logs = "…[shellcheck output truncated]…\n" + logs
		}
		return ports.GateVerdict{Status: ports.GateStatusFail, Reason: "shellcheck failed: " + strings.Join(failed, ", "), LogTail: logs}, nil
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
