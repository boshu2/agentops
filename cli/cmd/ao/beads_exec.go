// `ao beads exec [args...]` — the ONE tracker-agnostic CRUD passthrough
// (age-3mdu, dual-support increment 2, building on age-fvr8's resolveTracker).
//
// AgentOps is a product. Most end users track their beads with bd (beads, Go);
// this repo tracks with br (beads_rust). Skills and callers should not hardcode
// `br` or `bd` — they call `ao beads exec <verb> ...` and this command resolves
// the right tracker, sets its ledger correctly, forwards the args verbatim, and
// propagates the child's exit code unchanged. So `ao beads exec ready`,
// `ao beads exec close <id> -r "..."`, `ao beads exec update <id> --status
// in_progress`, `ao beads exec create ...`, and `ao beads exec list --json` all
// work against EITHER tracker.
//
// Ledger wiring per tracker:
//   - br: export BEADS_DIR=<resolution.LedgerDir> for the child (br's explicit
//     ledger override; worktree-aware via resolveTracker/resolveBeadsDir).
//   - bd: set the child's working directory to the repo root so its .beads/
//     auto-discovery resolves; no BEADS_DIR (any inherited value is stripped so
//     it cannot mislead bd).
//
// Hard divergence handled here: `bd children <epic>` exists; br has NO children
// subcommand. `ao beads exec children <epic>` works for BOTH — bd's native
// `children` is forwarded verbatim, and for br the child-id list is synthesized
// from `br show <epic> --json` (its dependents[] with a parent-child edge).
//
// practices: [tdd]

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var beadsExecCmd = &cobra.Command{
	Use:   "exec [args...]",
	Short: "Run a bead CRUD command against whichever tracker (bd or br) this environment uses",
	Long: `Forward a bead command verbatim to the resolved beads tracker (bd or br),
setting that tracker's ledger correctly and propagating its exit code unchanged.

This is the ONE tracker-agnostic entry point: skills and callers use
'ao beads exec <verb> ...' instead of hardcoding 'br' or 'bd'. The tracker is
resolved exactly as 'ao beads tracker' reports it (AGENTOPS_TRACKER > config >
ledger > binary). Examples (all work against either tracker):

  ao beads exec ready
  ao beads exec close <id> -r "Done"
  ao beads exec update <id> --status in_progress
  ao beads exec create "title" --type task
  ao beads exec list --json

Ledger wiring:
  br — BEADS_DIR is set to the resolved ledger dir (worktree-aware).
  bd — the child runs from the repo root so .beads/ auto-discovery resolves;
       no BEADS_DIR is set.

Children (hard divergence — bd has 'children', br does not):
  ao beads exec children <epic>
       bd: forwarded to 'bd children <epic>' (bd's native list output).
       br: synthesized from 'br show <epic> --json' — the id of every dependent
           with a parent-child edge, one per line.

All flags are forwarded to the tracker verbatim (flag parsing is disabled), so
tracker flags never collide with ao's. Only -h/--help is intercepted here.`,
	// DisableFlagParsing so every flag reaches the tracker unchanged. The trap:
	// cobra then forwards -h/--help into RunE, and if we resolved the tracker (or
	// exec'd) before intercepting it, the command-surface doc generator would bake
	// a path-dependent runtime string into cli/docs/COMMANDS.md and fail
	// derived.changed-scope non-deterministically. Intercept it FIRST. (Mirrors
	// membrane.go runMembraneCalibrate.)
	DisableFlagParsing: true,
	RunE:               runBeadsExec,
}

func init() {
	beadsCmd.AddCommand(beadsExecCmd)
}

func runBeadsExec(cmd *cobra.Command, args []string) error {
	// --help/-h is for THIS command, intercepted BEFORE resolving the tracker or
	// exec-ing so help stays a static, path-independent string (see the command
	// doc comment above).
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return cmd.Help()
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	res, err := resolveTracker(cwd, os.Environ())
	if err != nil {
		return err
	}

	// Hard divergence: br has no `children` subcommand. Synthesize it from
	// `br show <epic> --json`. bd's `children` is native — fall through to the
	// verbatim passthrough, which forwards `children <epic>` to bd unchanged.
	if res.Tracker == trackerBR && len(args) >= 1 && args[0] == "children" {
		return runBeadsExecChildrenBR(cmd, res, cwd, args[1:])
	}

	return execTracker(cmd, res, cwd, args)
}

// execTracker forwards args verbatim to the resolved tracker binary, streaming
// stdin/stdout/stderr, and maps a non-zero child exit to a beadsExitError so
// Execute() propagates the code unchanged (no verdict = not done).
func execTracker(cmd *cobra.Command, res trackerResolution, cwd string, args []string) error {
	c := exec.Command(res.Binary, args...) // #nosec G204 -- res.Binary is resolved by resolveTracker (bd|br); args are operator-supplied bead CRUD flags forwarded verbatim.
	c.Env = beadsExecChildEnv(res, cwd)
	c.Dir = beadsExecChildDir(res, cwd)
	c.Stdin = cmd.InOrStdin()
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	runErr := c.Run()
	if runErr == nil {
		return nil
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return &beadsExitError{code: exitErr.ExitCode()}
	}
	return runErr
}

// beadsExecChildEnv builds the child environment. For br, BEADS_DIR is the
// explicit ledger override (set to the resolved ledger dir), replacing any
// inherited value. For bd, no BEADS_DIR is set — bd auto-discovers .beads/ from
// its working directory — and any inherited BEADS_DIR is stripped so it cannot
// mislead bd.
func beadsExecChildEnv(res trackerResolution, _ string) []string {
	base := os.Environ()
	out := make([]string, 0, len(base)+1)
	for _, e := range base {
		if strings.HasPrefix(e, "BEADS_DIR=") {
			continue
		}
		out = append(out, e)
	}
	if res.Tracker == trackerBR {
		out = append(out, "BEADS_DIR="+res.LedgerDir)
	}
	return out
}

// beadsExecChildDir returns the child's working directory. bd resolves its
// .beads/ ledger relative to cwd, so it runs from the repo root (the parent of
// the resolved .beads dir, worktree-aware). br takes its ledger from BEADS_DIR,
// so it runs from the caller's cwd unchanged.
func beadsExecChildDir(res trackerResolution, cwd string) string {
	if res.Tracker == trackerBD {
		return filepath.Dir(res.LedgerDir)
	}
	return cwd
}

// brShowDependent is the subset of a dependent entry in `br show <id> --json`.
type brShowDependent struct {
	ID             string `json:"id"`
	DependencyType string `json:"dependency_type"`
}

// brShowIssue is the subset of a `br show <id> --json` array element we read.
// `br show` emits a JSON ARRAY of matched issues, each carrying a `dependents`
// array. (The full-output SHAPE divergence between this br envelope and bd's
// flatter list is a LATER increment; this command only makes the child-id SET
// available for both trackers.)
type brShowIssue struct {
	ID         string            `json:"id"`
	Dependents []brShowDependent `json:"dependents"`
}

// runBeadsExecChildrenBR synthesizes `children` for br from
// `br show <epic> --json`: it emits the id of every dependent with a
// parent-child edge, one per line, preserving br's order. bd's native
// `bd children` (the bd branch of runBeadsExec) prints bd's own list format;
// normalizing the two OUTPUT shapes into one machine-readable form is a
// follow-up increment.
func runBeadsExecChildrenBR(cmd *cobra.Command, res trackerResolution, cwd string, rest []string) error {
	if len(rest) < 1 || strings.TrimSpace(rest[0]) == "" {
		return fmt.Errorf("ao beads exec children: an epic id is required")
	}
	epic := rest[0]
	c := exec.Command(res.Binary, "show", epic, "--json") // #nosec G204 -- res.Binary is the resolved br binary; fixed subcommand + operator-supplied epic id.
	c.Env = beadsExecChildEnv(res, cwd)
	c.Dir = cwd
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		// Surface br's own stderr for diagnostics, then propagate its exit code
		// UNCHANGED — matching the exec passthrough's contract (line ~131) rather than
		// collapsing every br failure into the generic ao error path (age-3mdu refute-fix).
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), msg)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return &beadsExitError{code: exitErr.ExitCode()}
		}
		return fmt.Errorf("br show %s --json: %w", epic, err)
	}
	var issues []brShowIssue
	if err := json.Unmarshal(stdout.Bytes(), &issues); err != nil {
		return fmt.Errorf("parse br show %s --json: %w", epic, err)
	}
	out := cmd.OutOrStdout()
	for _, iss := range issues {
		for _, d := range iss.Dependents {
			if d.DependencyType == "parent-child" {
				fmt.Fprintln(out, d.ID)
			}
		}
	}
	return nil
}
