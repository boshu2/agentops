// practices: [hexagonal-architecture, design-by-contract]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// agentCmd is the `ao agent` noun: produce runtime-specific Agent/session
// definitions that make out-of-session background agents AgentOps-native.
// Distinct from `ao agents` (which manages AGENTS.md surfaces).
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Produce AgentOps-native session profiles for background agents",
	Long: `Emit a runtime-specific Agent/session profile that carries the
AgentOps skill set plus the ao tool surface, so an out-of-session background
agent runs under the same guardrails as interactive work.

Holdout/eval content is never inlined into a profile (the eval substrate is
LOCKED).`,
}

var (
	agentBundleRuntime string
	agentBundleSkills  string
	agentBundleSandbox string
	agentBundleOut     string
	agentBundleJSON    bool

	agentRosterJSON bool

	agentNTMSpawnClaude  int
	agentNTMSpawnCodex   int
	agentNTMSpawnDir     string
	agentNTMSpawnExecute bool
)

var agentBundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Emit a runtime-specific Agent/session profile",
	Long: `Stitch the selected AgentOps skills + the ao tool surface into an
AgentOps-native profile for the chosen runtime.

  ao agent bundle --runtime managed              # Managed Agents JSON payload
  ao agent bundle --runtime codex-ntm --json     # NTM-consumable bundle
  ao agent bundle --runtime claude-ntm --json    # Claude NTM session profile

Default skills: session-bootstrap, standards, validation, provenance.
Refuses (non-zero) if any selected skill would inline holdout/eval content.`,
	RunE: runAgentBundle,
}

var agentRosterCmd = &cobra.Command{
	Use:   "roster",
	Short: "Emit the default NTM background-agent roster",
	Long: `Emit the default AgentOps background-agent roster: one Claude NTM
session profile and one Codex NTM session profile. NTM owns pane lifecycle;
mcp-agent-mail owns assignment, reservations, check-ins, and handoff; workers
load skills and use ao/MCP tools. This command renders the roster only — it
does not start or stop live NTM sessions.`,
	Args: cobra.NoArgs,
	RunE: runAgentRoster,
}

var agentNTMSpawnCmd = &cobra.Command{
	Use:   "ntm-spawn <session>",
	Short: "Render or execute an NTM background-agent spawn command",
	Long: `Render the NTM command that starts an AgentOps background-agent
session. By default this is a dry run and prints the command. Pass --execute to
call ntm for real. NTM owns tmux pane lifecycle; mcp-agent-mail owns
assignment, reservations, check-ins, and handoff.`,
	Args: cobra.ExactArgs(1),
	RunE: runAgentNTMSpawn,
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentBundleCmd)
	agentCmd.AddCommand(agentRosterCmd)
	agentCmd.AddCommand(agentNTMSpawnCmd)
	agentBundleCmd.Flags().StringVar(&agentBundleRuntime, "runtime", "", "Target runtime: managed | codex-ntm | claude-ntm (required)")
	agentBundleCmd.Flags().StringVar(&agentBundleSkills, "skills", "", "Comma-separated skill names (default: session-bootstrap,standards,validation,provenance)")
	agentBundleCmd.Flags().StringVar(&agentBundleSandbox, "sandbox", "", "Sandbox placement: self-hosted | cloud")
	agentBundleCmd.Flags().StringVar(&agentBundleOut, "out", "", "Write the bundle to this path instead of stdout")
	agentBundleCmd.Flags().BoolVar(&agentBundleJSON, "json", false, "Emit machine-readable JSON (always JSON for now; reserved for parity)")

	agentRosterCmd.Flags().BoolVar(&agentRosterJSON, "json", false, "Emit machine-readable JSON")

	agentNTMSpawnCmd.Flags().IntVar(&agentNTMSpawnClaude, "claude", 1, "Number of Claude background agents")
	agentNTMSpawnCmd.Flags().IntVar(&agentNTMSpawnCodex, "codex", 1, "Number of Codex background agents")
	agentNTMSpawnCmd.Flags().StringVar(&agentNTMSpawnDir, "dir", ".", "Working directory for the NTM session")
	agentNTMSpawnCmd.Flags().BoolVar(&agentNTMSpawnExecute, "execute", false, "Execute ntm instead of printing a dry-run command")
}

func runAgentBundle(cmd *cobra.Command, _ []string) error {
	if agentBundleRuntime == "" {
		return fmt.Errorf("--runtime is required (managed | codex-ntm | claude-ntm)")
	}
	var skills []string
	if s := strings.TrimSpace(agentBundleSkills); s != "" {
		for _, part := range strings.Split(s, ",") {
			if p := strings.TrimSpace(part); p != "" {
				skills = append(skills, p)
			}
		}
	}
	bundle, err := buildAgentBundle(bundleOptions{
		Runtime: agentBundleRuntime,
		Skills:  skills,
		Sandbox: agentBundleSandbox,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling agent bundle: %w", err)
	}
	if agentBundleOut != "" {
		if err := os.WriteFile(agentBundleOut, append(raw, '\n'), 0o644); err != nil {
			return fmt.Errorf("writing bundle to %s: %w", agentBundleOut, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote %s agent bundle to %s\n", bundle.Runtime, agentBundleOut)
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(raw))
	return nil
}

func runAgentRoster(cmd *cobra.Command, _ []string) error {
	roster, err := buildAgentRoster(bundleOptions{})
	if err != nil {
		return err
	}
	if agentRosterJSON {
		raw, err := json.MarshalIndent(roster, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling agent roster: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(raw))
		return nil
	}
	for _, b := range roster.Agents {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\tmailbox=%s\tpolicy=%s\tskills=%s\n",
			b.Runtime, b.Mailbox, b.WorktreePolicy, strings.Join(b.Skills, ","))
	}
	return nil
}

func runAgentNTMSpawn(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	ntmArgs, err := buildNTMSpawnArgs(args[0], agentNTMSpawnClaude, agentNTMSpawnCodex, agentNTMSpawnDir, !agentNTMSpawnExecute)
	if err != nil {
		return err
	}
	if !agentNTMSpawnExecute {
		fmt.Fprintf(cmd.OutOrStdout(), "ntm %s\n", strings.Join(ntmArgs, " "))
		return nil
	}
	c := exec.CommandContext(cmd.Context(), "ntm", ntmArgs...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	return c.Run()
}

func buildNTMSpawnArgs(session string, claudeCount, codexCount int, dir string, dryRun bool) ([]string, error) {
	if strings.TrimSpace(session) == "" {
		return nil, fmt.Errorf("session is required")
	}
	if claudeCount < 0 || codexCount < 0 {
		return nil, fmt.Errorf("--claude and --codex must be >= 0")
	}
	if claudeCount == 0 && codexCount == 0 {
		return nil, fmt.Errorf("at least one background agent is required")
	}
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	out := []string{
		"--robot-spawn=" + session,
		"--spawn-cc=" + strconv.Itoa(claudeCount),
		"--spawn-cod=" + strconv.Itoa(codexCount),
		"--spawn-dir=" + dir,
	}
	if dryRun {
		out = append(out, "--dry-run")
	}
	return out, nil
}
