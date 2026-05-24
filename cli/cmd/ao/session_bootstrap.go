// `ao session bootstrap` — the universal init prompt (soc-vuu6.25).
//
// Every agent spawned into an AgentOps repo runs this command first. The
// output is the same shape regardless of model — that's the fungibility
// guarantee: no two agents in a swarm start with different orientation.
//
// Substeps (all fail-open):
//   1. Confirm AGENTS.md (and post-vuu6.3 siblings AGENTS-WORKFLOW.md,
//      AGENTS-CI.md, AGENTS-CODEX.md, AGENTS-RUNTIME.md) exist and are
//      readable. Read by the agent itself, not pre-loaded into the report.
//   2. Run `ao onboard --auto` if it exists (soc-vuu6.9 — currently a P3
//      stub). Falls back to phase="skipped:not-implemented" if absent.
//   3. Call `bd ready --json` and count claimable items. Falls back to
//      ready_beads_count=null if bd is missing or errors.
//   4. mcp-agent-mail register/check — fully optional. Phase reports
//      `present` / `absent`; never fails the bootstrap.
//
// The output validates against schemas/session-bootstrap.v1.schema.json.
// Default human output is a 1-line summary plus a path-to-orientation hint;
// `--json` emits the structured shape for headless callers (SessionStart
// hooks, evolve runs, swarm spawn scripts).
//
// practices: [agile-manifesto, dora-metrics, fungibility-charter]

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/adapters/tracker_bd"

	"github.com/spf13/cobra"
)

var (
	sessionBootstrapJSON   bool
	sessionBootstrapNoMail bool
	sessionBootstrapRobot  bool
)

var sessionBootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Universal init prompt — every agent runs this first",
	Long: `Universal init prompt for any agent spawned into an AgentOps repo.

Reports orientation status (AGENTS.md tier-split presence), invokes
optional follow-on commands (ao onboard, mcp-agent-mail), counts ready
work, and emits a machine-readable summary so swarm coordination layers
can rely on identical agent starting frames regardless of model.

The bootstrap is intentionally fail-open: missing subcommands or
optional integrations are reported but never abort the run.

Flags:
  --json       Emit the full status object as JSON (default: 1-line summary).
  --no-mail    Skip the mcp-agent-mail probe even if the MCP server is reachable.
  --robot      Same as --json but tighter exit-code contract for hooks.

Examples:
  ao session bootstrap            # human summary
  ao session bootstrap --json     # full JSON status
  ao session bootstrap --robot    # for SessionStart hooks`,
	Args: cobra.NoArgs,
	RunE: runSessionBootstrap,
}

func init() {
	sessionCmd.AddCommand(sessionBootstrapCmd)
	sessionBootstrapCmd.Flags().BoolVar(&sessionBootstrapJSON, "json", false,
		"Emit machine-readable status as JSON")
	sessionBootstrapCmd.Flags().BoolVar(&sessionBootstrapNoMail, "no-mail", false,
		"Skip the mcp-agent-mail probe")
	sessionBootstrapCmd.Flags().BoolVar(&sessionBootstrapRobot, "robot", false,
		"Robot mode: JSON output with tight exit-code contract for SessionStart hooks")
}

// SessionBootstrapStatus is the canonical bootstrap report shape. Field
// names and types must match schemas/session-bootstrap.v1.schema.json.
type SessionBootstrapStatus struct {
	AgentsMDRead       bool     `json:"agents_md_read"`
	AgentsSiblingsRead []string `json:"agents_siblings_read"`
	OnboardPhase       string   `json:"onboard_phase"`
	ReadyBeadsCount    *int     `json:"ready_beads_count"`
	MailUnreadCount    *int     `json:"mail_unread_count"`
	Runtime            string   `json:"runtime"`
	StartedAt          string   `json:"started_at"`
	BootstrapVersion   string   `json:"bootstrap_version"`
}

// agentsMDSiblings are the post-vuu6.3 split files. Reported individually so
// callers can detect partial splits or operator-customized tier layouts.
var agentsMDSiblings = []string{
	"AGENTS-WORKFLOW.md",
	"AGENTS-CI.md",
	"AGENTS-CODEX.md",
	"AGENTS-RUNTIME.md",
}

func runSessionBootstrap(cmd *cobra.Command, _ []string) error {
	robot := sessionBootstrapRobot || sessionBootstrapJSON
	status := computeBootstrapStatus(cmd.Context(), os.Getenv("PWD"), sessionBootstrapNoMail)

	if robot {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	return printBootstrapSummary(cmd, status)
}

// computeBootstrapStatus performs the four substeps and returns a populated
// status. Errors are absorbed into nil/skipped-marker fields — the function
// never errors out.
func computeBootstrapStatus(ctx context.Context, cwd string, noMail bool) SessionBootstrapStatus {
	if ctx == nil {
		ctx = context.Background()
	}
	if cwd == "" {
		if pwd, err := os.Getwd(); err == nil {
			cwd = pwd
		}
	}

	status := SessionBootstrapStatus{
		AgentsSiblingsRead: []string{},
		Runtime:            detectRuntime(),
		StartedAt:          time.Now().UTC().Format(time.RFC3339),
		BootstrapVersion:   "v1",
		OnboardPhase:       sessionBootstrapOnboard(ctx, cwd),
	}

	status.AgentsMDRead = fileExists(filepath.Join(cwd, "AGENTS.md"))
	for _, sib := range agentsMDSiblings {
		if fileExists(filepath.Join(cwd, sib)) {
			status.AgentsSiblingsRead = append(status.AgentsSiblingsRead, sib)
		}
	}

	if n, ok := sessionBootstrapReadyBeads(ctx, cwd); ok {
		status.ReadyBeadsCount = &n
	}

	if !noMail {
		if n, ok := sessionBootstrapMailUnread(ctx); ok {
			status.MailUnreadCount = &n
		}
	}

	return status
}

func detectRuntime() string {
	if v := os.Getenv("AGENTOPS_RPI_RUNTIME"); v != "" {
		return v
	}
	if os.Getenv("CLAUDE_CODE_SESSION_ID") != "" || os.Getenv("CLAUDECODE") != "" {
		return "claude-code"
	}
	if os.Getenv("CODEX_CLI") != "" {
		return "codex"
	}
	return runtime.GOOS
}

// sessionBootstrapOnboard returns the onboard phase. Falls back to
// "skipped:not-implemented" when `ao onboard` is not built (soc-vuu6.9 stub).
func sessionBootstrapOnboard(ctx context.Context, cwd string) string {
	// Check if `ao onboard` exists as a subcommand of this binary. We can detect
	// it by introspecting the root command tree. If absent, return the
	// skipped-with-reason marker so consumers can distinguish "ran and yielded
	// no work" from "command does not exist yet."
	for _, c := range rootCmd.Commands() {
		if c.Name() == "onboard" {
			return runOnboardSubprocess(ctx, cwd)
		}
	}
	return "skipped:not-implemented"
}

// runOnboardSubprocess executes `ao onboard --auto` via a child process when
// available. Returns the phase reported by onboard, or a skipped marker on
// error. Kept tiny so a swallowed error never crashes bootstrap.
func runOnboardSubprocess(ctx context.Context, cwd string) string {
	cmd := exec.CommandContext(ctx, "ao", "onboard", "--auto", "--json")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "skipped:error"
	}
	var payload struct {
		Phase string `json:"phase"`
	}
	if err := json.Unmarshal(out, &payload); err != nil || payload.Phase == "" {
		return "ran:no-phase"
	}
	return payload.Phase
}

// sessionBootstrapReadyBeads queries ready beads via the IssueTracker port
// (tracker_bd, `bd ready --json`) and returns the count of returned items.
// Returns (0, false) when bd is missing or errors.
func sessionBootstrapReadyBeads(ctx context.Context, cwd string) (int, bool) {
	issues, err := tracker_bd.New(cwd).Ready(ctx)
	if err != nil {
		return 0, false
	}
	return len(issues), true
}

// sessionBootstrapMailUnread is a soft probe for mcp-agent-mail. The current
// implementation returns (0, false) — wiring to the MCP transport lives in a
// follow-up bead so this primitive can ship now. The schema field stays
// nullable so callers handle both states.
func sessionBootstrapMailUnread(_ context.Context) (int, bool) {
	if os.Getenv("MCP_AGENT_MAIL_DISABLED") == "1" {
		return 0, false
	}
	return 0, false
}

// printBootstrapSummary writes the 1-line human form. Used when neither
// --json nor --robot is set.
func printBootstrapSummary(cmd *cobra.Command, s SessionBootstrapStatus) error {
	mdMark := "missing"
	if s.AgentsMDRead {
		mdMark = "ok"
	}
	mail := "n/a"
	if s.MailUnreadCount != nil {
		mail = fmt.Sprintf("%d", *s.MailUnreadCount)
	}
	ready := "n/a"
	if s.ReadyBeadsCount != nil {
		ready = fmt.Sprintf("%d", *s.ReadyBeadsCount)
	}

	parts := []string{
		fmt.Sprintf("agents_md=%s", mdMark),
		fmt.Sprintf("siblings=%d/%d", len(s.AgentsSiblingsRead), len(agentsMDSiblings)),
		fmt.Sprintf("onboard=%s", s.OnboardPhase),
		fmt.Sprintf("ready=%s", ready),
		fmt.Sprintf("mail=%s", mail),
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "session bootstrap: %s\n", strings.Join(parts, " ")); err != nil {
		return err
	}
	if !s.AgentsMDRead {
		_, err := fmt.Fprintln(cmd.ErrOrStderr(),
			"warn: AGENTS.md missing — start with `cat AGENTS.md` if it exists, or `bd onboard` for repo orientation")
		return err
	}
	return nil
}

// sessionBootstrapMakeReady is a test seam for the ready-beads path. Kept here
// so the test file can override without touching exec.Command.
var sessionBootstrapMakeReady = sessionBootstrapReadyBeads

// errNotImplemented is reserved for future use when one of the bootstrap steps
// hard-fails. Currently every step is fail-open and returns a marker instead.
var errNotImplemented = errors.New("bootstrap substep not yet implemented")

var _ = errNotImplemented // suppress unused-var until follow-up bead wires it in
