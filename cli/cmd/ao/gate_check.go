// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/gates"
	// Blank import registers the seed checks into gates.Default via init().
	_ "github.com/boshu2/agentops/cli/internal/gates/checks"
)

// `ao gate check` is the orchestrated gate surface (ag-qidx GA4): it runs the
// declarative BC2 gate registry (cli/internal/gates) in fast (cockpit/pre-push,
// changed-file routed) or full (CI/refinery, all checks) mode, wiring the
// existing productionGateRunner (for Backing check-*.sh) and a git-backed
// ChangedFilesPort. Distinct from `ao gate run <name>`, which runs ONE check.
//
// Mode is the ONLY variable the three consumers pass — hook `--fast`, CI
// `--full`, refinery `--full --json` — so local and CI run the identical binary.

var (
	gateCheckFast     bool
	gateCheckFull     bool
	gateCheckJSON     bool
	gateCheckGitHub   bool
	gateCheckFailFast bool
	gateCheckScope    string
)

// gateCheckRegistry is the registry the command runs; a seam so tests can swap
// in an isolated registry instead of the init()-populated Default.
var gateCheckRegistry = gates.Default

var gateCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Run the gate registry (fast cockpit subset or full suite)",
	Long: `Run the declarative gate registry.

  ao gate check            # fast: only checks affected by changed files (cockpit)
  ao gate check --full     # full: every check, routing ignored (CI/refinery)
  ao gate check --json     # machine-readable report (refinery/CI annotations)

Exit code is the verdict: 0 if no blocking check FAILed; 1 if any did
(non-blocking FAIL and WARN/SKIP are advisory).`,
	Args: cobra.NoArgs,
	RunE: runGateCheck,
}

func init() {
	f := gateCheckCmd.Flags()
	f.BoolVar(&gateCheckFast, "fast", false, "fast cockpit subset routed to changed files (the default; explicit flag for clarity in hooks)")
	f.BoolVar(&gateCheckFull, "full", false, "run every check (routing ignored); default is the fast changed-file subset")
	f.BoolVar(&gateCheckJSON, "json", false, "emit the machine-readable JSON report")
	f.BoolVar(&gateCheckGitHub, "github-annotations", false, "emit GitHub Actions annotations for WARN/FAIL checks")
	f.BoolVar(&gateCheckFailFast, "fail-fast", false, "stop after the first blocking failure")
	f.StringVar(&gateCheckScope, "scope", "head", "fast-mode changed-file scope: head|staged|worktree|upstream")
	gateCmd.AddCommand(gateCheckCmd)
}

// gateCheckMode maps the --full flag to a run tier.
func gateCheckMode(full bool) gates.Tier {
	if full {
		return gates.Full
	}
	return gates.Fast
}

// gateRepoRoot resolves the repository root robustly via `git rev-parse
// --show-toplevel` (correct from any subdir and inside linked worktrees),
// falling back to the cwd-based resolver. The orchestrator joins scripts/ off
// this, so it must be the repo/worktree root, not the caller's cwd.
func gateRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		if root := strings.TrimSpace(string(out)); root != "" {
			return root, nil
		}
	}
	return resolveProjectDir()
}

func runGateCheck(cmd *cobra.Command, _ []string) error {
	root, err := gateRepoRoot()
	if err != nil {
		return &gateExitError{code: gateExitInternal, msg: fmt.Sprintf("resolve repo root: %v", err)}
	}

	orch := gates.NewOrchestrator(
		gateCheckRegistry,
		gates.NewScriptRunner(root),
		gates.NewGitChangedFiles(root),
		root,
	)
	report, err := orch.Run(cmd.Context(), gates.RunOptions{
		Mode:     gateCheckMode(gateCheckFull),
		Scope:    gates.Scope(gateCheckScope),
		FailFast: gateCheckFailFast,
	})
	if err != nil {
		return &gateExitError{code: gateExitInternal, msg: err.Error()}
	}

	out := cmd.OutOrStdout()
	if gateCheckJSON {
		raw, jerr := report.JSON()
		if jerr != nil {
			return &gateExitError{code: gateExitInternal, msg: jerr.Error()}
		}
		fmt.Fprintln(out, string(raw))
	} else {
		report.Human(out)
	}
	if gateCheckGitHub {
		report.GitHubAnnotations(cmd.ErrOrStderr())
	}

	if code := report.ExitCode(); code != 0 {
		return &gateExitError{code: code}
	}
	return nil
}
