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

	agentInitRuntime string
	agentInitMailbox string

	agentAssignBead       string
	agentAssignTo         string
	agentAssignBranch     string
	agentAssignFiles      string
	agentAssignSkills     string
	agentAssignValidation string
	agentAssignSession    string
	agentAssignTTL        string

	agentNTMStatusJSON bool

	agentNTMStopExecute bool
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

var agentInitPromptCmd = &cobra.Command{
	Use:   "init-prompt",
	Short: "Print the NTM background-agent initialization prompt",
	Long: `Print the prompt an operator or NTM lead sends to a newly-started
Claude/Codex background session. The prompt orients the worker, requires
skills-as-contract execution, mcp-agent-mail coordination, and no bead claim
until assignment.`,
	Args: cobra.NoArgs,
	RunE: runAgentInitPrompt,
}

var agentAssignPromptCmd = &cobra.Command{
	Use:   "assign-prompt",
	Short: "Print an mcp-agent-mail assignment prompt for a background worker",
	Long: `Print the assignment message a lead sends to an NTM background worker
through mcp-agent-mail. The message names the bead, branch/worktree, file
reservation manifest, skills to use, and validation evidence expected back.`,
	Args: cobra.NoArgs,
	RunE: runAgentAssignPrompt,
}

var agentAssignCmd = &cobra.Command{
	Use:   "assign",
	Short: "Send a background-agent assignment through Agent Mail",
	Long: `Send a background-agent assignment through the NTM Agent Mail bridge.
The command reserves the declared file paths before sending the assignment
message. With the global --dry-run flag, it emits JSON evidence with a
copy-paste fallback instead of touching live Agent Mail.`,
	Args: cobra.NoArgs,
	RunE: runAgentAssign,
}

var agentNTMStatusCmd = &cobra.Command{
	Use:   "ntm-status [session]",
	Short: "Show NTM background-agent session status",
	Long: `Show NTM background-agent session status by delegating to
ntm --robot-status. With a session argument, the output is narrowed to that
session and summarized for operators; --json emits the filtered machine-readable
record. Without a session, the raw NTM status JSON is passed through.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAgentNTMStatus,
}

var agentNTMStopCmd = &cobra.Command{
	Use:   "ntm-stop <session>",
	Short: "Render or execute a safe NTM background-agent stop command",
	Long: `Render the NTM command that stops an AgentOps background-agent
session. By default this is a dry run and prints the command. Pass --execute to
call 'ntm kill <session> --force' for real. Stop live sessions only when the
operator explicitly intends cleanup.`,
	Args: cobra.ExactArgs(1),
	RunE: runAgentNTMStop,
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentBundleCmd)
	agentCmd.AddCommand(agentRosterCmd)
	agentCmd.AddCommand(agentNTMSpawnCmd)
	agentCmd.AddCommand(agentNTMStopCmd)
	agentCmd.AddCommand(agentEligibleCmd)
	agentCmd.AddCommand(agentInitPromptCmd)
	agentCmd.AddCommand(agentAssignPromptCmd)
	agentCmd.AddCommand(agentAssignCmd)
	agentCmd.AddCommand(agentNTMStatusCmd)
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

	agentInitPromptCmd.Flags().StringVar(&agentInitRuntime, "runtime", "", "Runtime identity to include (claude-ntm|codex-ntm)")
	agentInitPromptCmd.Flags().StringVar(&agentInitMailbox, "mailbox", "", "Expected mcp-agent-mail identity, if preassigned")

	agentAssignPromptCmd.Flags().StringVar(&agentAssignBead, "bead", "", "Bead id to assign (required)")
	agentAssignPromptCmd.Flags().StringVar(&agentAssignBranch, "branch", "", "Branch/worktree for the worker")
	agentAssignPromptCmd.Flags().StringVar(&agentAssignFiles, "files", "", "Comma-separated file paths/globs to reserve")
	agentAssignPromptCmd.Flags().StringVar(&agentAssignSkills, "skills", "", "Comma-separated skills the worker should use")
	agentAssignPromptCmd.Flags().StringVar(&agentAssignValidation, "validation", "", "Validation command/evidence expected from the worker")

	agentAssignCmd.Flags().StringVar(&agentAssignBead, "bead", "", "Bead id to assign (required)")
	agentAssignCmd.Flags().StringVar(&agentAssignTo, "to", "", "Comma-separated mcp-agent-mail recipient(s) (required)")
	agentAssignCmd.Flags().StringVar(&agentAssignBranch, "branch", "", "Branch/worktree for the worker")
	agentAssignCmd.Flags().StringVar(&agentAssignFiles, "files", "", "Comma-separated file paths/globs to reserve (required)")
	agentAssignCmd.Flags().StringVar(&agentAssignSkills, "skills", "", "Comma-separated skills the worker should use")
	agentAssignCmd.Flags().StringVar(&agentAssignValidation, "validation", "", "Validation command/evidence expected from the worker")
	agentAssignCmd.Flags().StringVar(&agentAssignSession, "session", background.DefaultAssignmentSession, "NTM session/project key for Agent Mail")
	agentAssignCmd.Flags().StringVar(&agentAssignTTL, "ttl", "2h", "File reservation TTL for NTM lock")

	agentNTMStatusCmd.Flags().BoolVar(&agentNTMStatusJSON, "json", false, "Emit filtered machine-readable JSON")
	agentNTMStopCmd.Flags().BoolVar(&agentNTMStopExecute, "execute", false, "Execute ntm kill instead of printing a dry-run command")
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
			fmt.Fprintln(cmd.OutOrStdout(), "# Codex panes use manual tmux split-window so --codex-model can override NTM's default Codex model.")
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

func runAgentInitPrompt(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	fmt.Fprint(cmd.OutOrStdout(), buildAgentInitPrompt(agentInitRuntime, agentInitMailbox))
	return nil
}

func runAgentAssignPrompt(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	if strings.TrimSpace(agentAssignBead) == "" {
		return fmt.Errorf("--bead is required")
	}
	fmt.Fprint(cmd.OutOrStdout(), buildAgentAssignmentPrompt(agentAssignBead, agentAssignBranch, splitLabelsCSV(agentAssignFiles), splitLabelsCSV(agentAssignSkills), agentAssignValidation))
	return nil
}

func runAgentAssign(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true
	if strings.TrimSpace(agentAssignBead) == "" {
		return fmt.Errorf("--bead is required")
	}
	if strings.TrimSpace(agentAssignTo) == "" {
		return fmt.Errorf("--to is required")
	}
	if strings.TrimSpace(agentAssignFiles) == "" {
		return fmt.Errorf("--files is required so assignment reservations are explicit")
	}
	req := background.AssignmentRequest{
		Bead:       agentAssignBead,
		To:         splitLabelsCSV(agentAssignTo),
		Branch:     agentAssignBranch,
		Files:      splitLabelsCSV(agentAssignFiles),
		Skills:     splitLabelsCSV(agentAssignSkills),
		Validation: agentAssignValidation,
		Session:    agentAssignSession,
		DryRun:     GetDryRun(),
	}
	var transport background.AssignmentTransport
	if !GetDryRun() {
		transport = background.NewNTMAssignmentTransport(execCommandRunner{}, agentAssignTTL)
	}
	evidence, err := background.AssignBackgroundAgent(cmd.Context(), req, transport)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(evidence)
}

type agentAssignState struct {
	bead       string
	to         string
	branch     string
	files      string
	skills     string
	validation string
	session    string
	ttl        string
}

func agentAssignStateSnapshot() agentAssignState {
	return agentAssignState{
		bead:       agentAssignBead,
		to:         agentAssignTo,
		branch:     agentAssignBranch,
		files:      agentAssignFiles,
		skills:     agentAssignSkills,
		validation: agentAssignValidation,
		session:    agentAssignSession,
		ttl:        agentAssignTTL,
	}
}

func restoreAgentAssignState(state agentAssignState) {
	agentAssignBead = state.bead
	agentAssignTo = state.to
	agentAssignBranch = state.branch
	agentAssignFiles = state.files
	agentAssignSkills = state.skills
	agentAssignValidation = state.validation
	agentAssignSession = state.session
	agentAssignTTL = state.ttl
}

func buildAgentInitPrompt(runtimeName, mailbox string) string {
	var sb strings.Builder
	sb.WriteString("You are an AgentOps background agent running under NTM.\n\n")
	if strings.TrimSpace(runtimeName) != "" {
		sb.WriteString("Runtime profile: ")
		sb.WriteString(strings.TrimSpace(runtimeName))
		sb.WriteString("\n")
	}
	if strings.TrimSpace(mailbox) != "" {
		sb.WriteString("Expected mcp-agent-mail identity: ")
		sb.WriteString(strings.TrimSpace(mailbox))
		sb.WriteString("\n")
	}
	sb.WriteString(`
Initialize, then wait for operator assignment:
1. Run ` + "`ao session bootstrap --json`" + `.
2. Read AGENTS.md/CLAUDE.md as needed.
3. Register or confirm your mcp-agent-mail identity.
4. Do not claim or edit any bead until assigned via mcp-agent-mail or an operator message.
5. Before editing, reserve file paths through mcp-agent-mail and use one worktree per bead.
6. When assigned, use skills as the execution contract; do not use deprecated ` + "`ao rpi`" + ` / ` + "`ao evolve`" + ` wrappers.

After initialization, respond with a one-line READY including your runtime and mailbox identity.
`)
	return sb.String()
}

func buildAgentAssignmentPrompt(bead, branch string, files, skills []string, validation string) string {
	if strings.TrimSpace(branch) == "" {
		branch = "cursor/<bead>-<slug>-<session>"
	}
	if len(skills) == 0 {
		skills = []string{"research", "implement", "validation", "provenance"}
	}
	if strings.TrimSpace(validation) == "" {
		validation = "run the smallest relevant tests plus `scripts/pre-push-gate.sh --fast` when code/docs changed"
	}
	var sb strings.Builder
	sb.WriteString("BACKGROUND AGENT ASSIGNMENT\n\n")
	sb.WriteString("Bead: ")
	sb.WriteString(strings.TrimSpace(bead))
	sb.WriteString("\n")
	sb.WriteString("Branch/worktree: ")
	sb.WriteString(branch)
	sb.WriteString("\n")
	sb.WriteString("Skills: ")
	sb.WriteString(strings.Join(skills, ", "))
	sb.WriteString("\n")
	sb.WriteString("Validation: ")
	sb.WriteString(validation)
	sb.WriteString("\n\n")
	sb.WriteString("Working-directory note: file paths are repo-root relative. Go CLI validation commands that reference `./cmd/ao` should run from `cli/` (for example `cd cli && go test ./cmd/ao -run Agent`).\n\n")
	sb.WriteString("Before editing:\n")
	sb.WriteString("1. Confirm this assignment in the mcp-agent-mail thread.\n")
	sb.WriteString("2. Reserve these file paths/globs through mcp-agent-mail:\n")
	if len(files) == 0 {
		sb.WriteString("   - <lead must provide file manifest before edits>\n")
	} else {
		for _, f := range files {
			sb.WriteString("   - ")
			sb.WriteString(f)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("3. Create/use one worktree for this bead; do not edit the shared checkout.\n")
	sb.WriteString("4. Use skills as the execution contract; do not run deprecated `ao rpi` / `ao evolve` wrappers.\n\n")
	sb.WriteString("Closeout:\n")
	sb.WriteString("- Reply with branch, commits, tests, provenance/evidence paths, and any scope escapes.\n")
	sb.WriteString("- Do not self-merge.\n")
	return sb.String()
}

func runAgentNTMStatus(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	c := exec.CommandContext(cmd.Context(), "ntm", "--robot-status")
	raw, err := c.Output()
	if err != nil {
		return fmt.Errorf("ntm --robot-status: %w", err)
	}
	if len(args) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), string(raw))
		return nil
	}
	session, err := filterNTMStatus(raw, args[0])
	if err != nil {
		return err
	}
	if agentNTMStatusJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(session)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\tpanes=%d\tagents=%d\n", session.Name, session.Panes, len(session.Agents))
	for _, agent := range session.Agents {
		fmt.Fprintf(cmd.OutOrStdout(), "pane=%d\ttype=%s\tstate=%s\tmodel=%s\n",
			agent.PaneIdx, emptyDash(agent.Type), emptyDash(agent.ProcessStateName), emptyDash(agent.ContextModel))
	}
	return nil
}

type ntmStatusPayload struct {
	Sessions []ntmSessionStatus `json:"sessions"`
}

type ntmSessionStatus struct {
	Name   string           `json:"name"`
	Panes  int              `json:"panes"`
	Agents []ntmAgentStatus `json:"agents"`
}

type ntmAgentStatus struct {
	Type             string `json:"type"`
	PaneIdx          int    `json:"pane_idx"`
	ProcessStateName string `json:"process_state_name"`
	ContextModel     string `json:"context_model"`
}

func filterNTMStatus(raw []byte, sessionName string) (ntmSessionStatus, error) {
	var payload ntmStatusPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ntmSessionStatus{}, fmt.Errorf("parse ntm status: %w", err)
	}
	for _, session := range payload.Sessions {
		if session.Name == sessionName {
			return session, nil
		}
	}
	return ntmSessionStatus{}, fmt.Errorf("ntm session %q not found", sessionName)
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func runAgentNTMStop(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	ntmArgs, err := buildNTMStopArgs(args[0])
	if err != nil {
		return err
	}
	if !agentNTMStopExecute {
		fmt.Fprintf(cmd.OutOrStdout(), "ntm %s\n", strings.Join(ntmArgs, " "))
		return nil
	}
	c := exec.CommandContext(cmd.Context(), "ntm", ntmArgs...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	return c.Run()
}

func buildNTMStopArgs(session string) ([]string, error) {
	if strings.TrimSpace(session) == "" {
		return nil, fmt.Errorf("session is required")
	}
	return []string{"kill", session, "--force"}, nil
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
