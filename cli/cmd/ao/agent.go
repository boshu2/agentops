// practices: [hexagonal-architecture, design-by-contract]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/background"
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

	agentNTMSpawnClaude     int
	agentNTMSpawnCodex      int
	agentNTMSpawnCodexModel string
	agentNTMSpawnDir        string
	agentNTMSpawnExecute    bool

	agentEligibleFile         string
	agentEligibleEligibleOnly bool
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

var agentEligibleCmd = &cobra.Command{
	Use:   "eligible",
	Short: "Filter ready beads for background-agent eligibility",
	Long: `Filter candidate beads for safe NTM background-agent execution.
By default the command reads 'bd ready --limit 0 --json'. Use --file to test
against a fixture. A candidate must opt in with background-agent-safe /
background_eligible / managed_eligible label or background_eligible metadata,
and holdout/evaluator/PII/human/operator-gated work is excluded.`,
	Args: cobra.NoArgs,
	RunE: runAgentEligible,
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentBundleCmd)
	agentCmd.AddCommand(agentRosterCmd)
	agentCmd.AddCommand(agentNTMSpawnCmd)
	agentCmd.AddCommand(agentEligibleCmd)
	agentBundleCmd.Flags().StringVar(&agentBundleRuntime, "runtime", "", "Target runtime: managed | codex-ntm | claude-ntm (required)")
	agentBundleCmd.Flags().StringVar(&agentBundleSkills, "skills", "", "Comma-separated skill names (default: session-bootstrap,standards,validation,provenance)")
	agentBundleCmd.Flags().StringVar(&agentBundleSandbox, "sandbox", "", "Sandbox placement: self-hosted | cloud")
	agentBundleCmd.Flags().StringVar(&agentBundleOut, "out", "", "Write the bundle to this path instead of stdout")
	agentBundleCmd.Flags().BoolVar(&agentBundleJSON, "json", false, "Emit machine-readable JSON (always JSON for now; reserved for parity)")

	agentRosterCmd.Flags().BoolVar(&agentRosterJSON, "json", false, "Emit machine-readable JSON")

	agentNTMSpawnCmd.Flags().IntVar(&agentNTMSpawnClaude, "claude", 1, "Number of Claude background agents")
	agentNTMSpawnCmd.Flags().IntVar(&agentNTMSpawnCodex, "codex", 1, "Number of Codex background agents")
	agentNTMSpawnCmd.Flags().StringVar(&agentNTMSpawnCodexModel, "codex-model", "gpt-5.5", "Codex model for manual Codex panes (set empty to use NTM's default Codex spawn)")
	agentNTMSpawnCmd.Flags().StringVar(&agentNTMSpawnDir, "dir", ".", "Working directory for the NTM session")
	agentNTMSpawnCmd.Flags().BoolVar(&agentNTMSpawnExecute, "execute", false, "Execute ntm instead of printing a dry-run command")

	agentEligibleCmd.Flags().StringVar(&agentEligibleFile, "file", "", "Read candidate bead JSON from this file instead of running bd ready")
	agentEligibleCmd.Flags().BoolVar(&agentEligibleEligibleOnly, "eligible-only", false, "Emit only eligible candidates")
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
	manualCodex := strings.TrimSpace(agentNTMSpawnCodexModel) != "" && agentNTMSpawnCodex > 0
	ntmCodex := agentNTMSpawnCodex
	if manualCodex {
		ntmCodex = 0
	}
	ntmArgs, err := buildNTMSpawnArgs(args[0], agentNTMSpawnClaude, ntmCodex, agentNTMSpawnDir, !agentNTMSpawnExecute)
	if err != nil {
		return err
	}
	if !agentNTMSpawnExecute {
		fmt.Fprintf(cmd.OutOrStdout(), "ntm %s\n", strings.Join(ntmArgs, " "))
		if manualCodex {
			for _, tmuxArgs := range buildManualCodexPaneArgs(args[0], agentNTMSpawnCodex, agentNTMSpawnDir, agentNTMSpawnCodexModel) {
				fmt.Fprintf(cmd.OutOrStdout(), "tmux %s\n", strings.Join(tmuxArgs, " "))
			}
		}
		return nil
	}
	c := exec.CommandContext(cmd.Context(), "ntm", ntmArgs...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	if err := c.Run(); err != nil {
		return err
	}
	if manualCodex {
		for _, tmuxArgs := range buildManualCodexPaneArgs(args[0], agentNTMSpawnCodex, agentNTMSpawnDir, agentNTMSpawnCodexModel) {
			tmuxCmd := exec.CommandContext(cmd.Context(), "tmux", tmuxArgs...)
			tmuxCmd.Stdout = cmd.OutOrStdout()
			tmuxCmd.Stderr = cmd.ErrOrStderr()
			if err := tmuxCmd.Run(); err != nil {
				return err
			}
		}
	}
	return nil
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

func buildManualCodexPaneArgs(session string, count int, dir, model string) [][]string {
	if count <= 0 {
		return nil
	}
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	if strings.TrimSpace(model) == "" {
		model = "gpt-5.5"
	}
	cmd := "codex --dangerously-bypass-approvals-and-sandbox -m " + shellQuoteArg(model) +
		" -c model_reasoning_effort='xhigh' -c model_reasoning_summary_format=experimental"
	out := make([][]string, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, []string{"split-window", "-t", session + ":", "-c", dir, cmd})
	}
	return out
}

func shellQuoteArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func runAgentEligible(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	candidates, err := loadBackgroundCandidates(cmd, agentEligibleFile)
	if err != nil {
		return err
	}
	decisions := background.FilterEligible(candidates)
	if agentEligibleEligibleOnly {
		filtered := decisions[:0]
		for _, d := range decisions {
			if d.Eligible {
				filtered = append(filtered, d)
			}
		}
		decisions = filtered
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(decisions)
}

func loadBackgroundCandidates(cmd *cobra.Command, file string) ([]background.Candidate, error) {
	var raw []byte
	var err error
	if strings.TrimSpace(file) != "" {
		raw, err = os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read candidate file: %w", err)
		}
	} else {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		c := exec.CommandContext(ctx, "bd", "ready", "--limit", "0", "--json")
		raw, err = c.Output()
		if err != nil {
			return loadBackgroundCandidatesViaSQL(ctx)
		}
	}
	var candidates []background.Candidate
	if err := json.Unmarshal(raw, &candidates); err != nil {
		return nil, fmt.Errorf("parse candidate JSON: %w", err)
	}
	return candidates, nil
}

type bdSQLCandidate struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Priority  int    `json:"priority"`
	IssueType string `json:"issue_type"`
	LabelsCSV string `json:"labels_csv"`
}

func loadBackgroundCandidatesViaSQL(ctx context.Context) ([]background.Candidate, error) {
	query := "select i.id, i.title, i.priority, i.issue_type, " +
		"group_concat(l.label) as labels_csv from issues i " +
		"left join labels l on l.issue_id=i.id " +
		"where i.status='open' and i.is_blocked=0 " +
		"group by i.id, i.title, i.priority, i.issue_type"
	c := exec.CommandContext(ctx, "bd", "sql", "--json", query)
	raw, err := c.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(raw))
		if msg != "" {
			return nil, fmt.Errorf("bd ready and sql fallback failed: %s: %w", msg, err)
		}
		return nil, fmt.Errorf("bd ready and sql fallback failed: %w", err)
	}
	var rows []bdSQLCandidate
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse bd sql candidate JSON: %w", err)
	}
	candidates := make([]background.Candidate, 0, len(rows))
	for _, row := range rows {
		candidates = append(candidates, background.Candidate{
			ID:       row.ID,
			Title:    row.Title,
			Priority: row.Priority,
			Type:     row.IssueType,
			Labels:   splitLabelsCSV(row.LabelsCSV),
		})
	}
	return candidates, nil
}

func splitLabelsCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
