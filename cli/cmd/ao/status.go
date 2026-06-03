// practices: [dora-metrics, sre]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenance"
	"github.com/boshu2/agentops/cli/internal/storage"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show AgentOps status",
	Long: `Display the current state of AgentOps knowledge base.

Shows:
  - Number of sessions indexed
  - Recent sessions
  - Provenance statistics
  - Flywheel health summary
  - Storage locations

Examples:
  ao status
  ao status --json`,
	RunE: runStatus,
}

var (
	statusFlywheelTimeout      = 5 * time.Second
	statusComputeFlywheelBrief = computeStatusFlywheelBrief
	// statusNTMRunner returns raw `ntm --robot-status` JSON. Overridable in
	// tests so background-agent parsing can run without a live NTM/tmux.
	statusNTMRunner = defaultNTMStatusRunner
)

// backgroundAgentSessionPrefix matches NTM sessions that host AgentOps
// background agents (e.g. "agentops-bg", "agentops-bg-e2e").
const backgroundAgentSessionPrefix = "agentops-bg"

// backgroundStuckAfter is how long a pane may go without output before ao
// status flags it as stuck (a snapshot heuristic, not a hard SLA).
const backgroundStuckAfter = 15 * time.Minute

func init() {
	statusCmd.GroupID = "core"
	rootCmd.AddCommand(statusCmd)
}

type statusOutput struct {
	Initialized      bool                  `json:"initialized"`
	BaseDir          string                `json:"base_dir"`
	SessionCount     int                   `json:"session_count"`
	RecentSessions   []sessionInfo         `json:"recent_sessions,omitempty"`
	ProvenanceStats  *provStats            `json:"provenance_stats,omitempty"`
	Flywheel         *flywheelBrief        `json:"flywheel,omitempty"`
	QualitySignals   []qualitySignalInfo   `json:"quality_signals,omitempty"`
	BackgroundAgents []backgroundAgentInfo `json:"background_agents,omitempty"`
}

type sessionInfo struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Summary string `json:"summary,omitempty"`
	Path    string `json:"path"`
}

type provStats struct {
	TotalRecords     int `json:"total_records"`
	UniqueSessions   int `json:"unique_sessions"`
	UniqueWorkspaces int `json:"unique_workspaces"`
}

type flywheelBrief struct {
	Status                 string  `json:"status"`
	TotalArtifacts         int     `json:"total_artifacts"`
	Velocity               float64 `json:"velocity"`
	NewArtifacts           int     `json:"new_artifacts"`
	StaleArtifacts         int     `json:"stale_artifacts"`
	PromotedFindings       int     `json:"promoted_findings,omitempty"`
	PlanningRules          int     `json:"planning_rules,omitempty"`
	PreMortemChecks        int     `json:"pre_mortem_checks,omitempty"`
	UnconsumedItems        int     `json:"unconsumed_items,omitempty"`
	HighSeverityUnconsumed int     `json:"high_severity_unconsumed,omitempty"`
	LastForgeAge           string  `json:"last_forge_age,omitempty"`
	LastForgeTime          string  `json:"last_forge_time,omitempty"`
}

type qualitySignalInfo struct {
	Timestamp  string `json:"timestamp"`
	SignalType string `json:"signal_type"`
	Detail     string `json:"detail"`
	SessionID  string `json:"session_id,omitempty"`
}

// loadRecentSessions populates status with session count and recent sessions.
func loadRecentSessions(baseDir string, status *statusOutput) {
	fs := storage.NewFileStorage(storage.WithBaseDir(baseDir))
	sessions, err := fs.ListSessions()
	if err != nil {
		return
	}
	status.SessionCount = len(sessions)
	if len(sessions) == 0 {
		return
	}

	slices.SortFunc(sessions, func(a, b storage.IndexEntry) int {
		return b.Date.Compare(a.Date)
	})

	limit := 5
	if len(sessions) < limit {
		limit = len(sessions)
	}

	for _, s := range sessions[:limit] {
		status.RecentSessions = append(status.RecentSessions, sessionInfo{
			ID:      s.SessionID,
			Date:    s.Date.Format("2006-01-02"),
			Summary: truncateStatus(s.Summary, 60),
			Path:    filepath.Base(s.SessionPath),
		})
	}
}

// loadFlywheelBrief computes the flywheel health summary for status output with
// a small latency budget. The full `ao metrics` commands still perform complete
// analysis; status should remain an operator dashboard, not a long-running scan.
func loadFlywheelBrief(cwd string) *flywheelBrief {
	timeout := statusFlywheelTimeout
	compute := statusComputeFlywheelBrief
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ch := make(chan *flywheelBrief, 1)
	go func() {
		brief := compute(ctx, cwd)
		select {
		case ch <- brief:
		case <-ctx.Done():
		}
	}()

	select {
	case brief := <-ch:
		return brief
	case <-ctx.Done():
		return nil
	}
}

func computeStatusFlywheelBrief(ctx context.Context, cwd string) *flywheelBrief {
	if ctx.Err() != nil {
		return nil
	}
	metrics, err := computeMetrics(cwd, 7)
	if err != nil || ctx.Err() != nil {
		return nil
	}
	populateGoldenSignals(cwd, 7, metrics)
	if ctx.Err() != nil {
		return nil
	}
	brief := &flywheelBrief{
		Status:         metrics.HealthStatus(),
		TotalArtifacts: metrics.TotalArtifacts,
		Velocity:       metrics.Velocity,
		NewArtifacts:   metrics.NewArtifacts,
		StaleArtifacts: metrics.StaleArtifacts,
	}
	if scorecard, err := loadStigmergicScorecard(cwd); err == nil {
		brief.PromotedFindings = scorecard.PromotedFindings
		brief.PlanningRules = scorecard.PlanningRules
		brief.PreMortemChecks = scorecard.PreMortemChecks
		brief.UnconsumedItems = scorecard.UnconsumedItems
		brief.HighSeverityUnconsumed = scorecard.HighSeverityUnconsumed
	}
	if lastForge := findLastForgeTime(cwd); !lastForge.IsZero() {
		brief.LastForgeTime = lastForge.Format("2006-01-02 15:04")
		brief.LastForgeAge = formatDurationBrief(time.Since(lastForge))
	}
	return brief
}

func loadQualitySignals(agentsDir string, limit int) []qualitySignalInfo {
	if limit <= 0 {
		return nil
	}
	path := filepath.Join(agentsDir, "signals", "session-quality.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	signals := make([]qualitySignalInfo, 0, limit)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var signal qualitySignalInfo
		if err := json.Unmarshal([]byte(line), &signal); err != nil {
			continue
		}
		if signal.Timestamp == "" && signal.SignalType == "" && signal.Detail == "" {
			continue
		}
		signals = append(signals, signal)
	}
	if len(signals) <= limit {
		return signals
	}
	return signals[len(signals)-limit:]
}

// backgroundAgentInfo is a single NTM background-agent pane projected for
// `ao status`. Runtime, health, last check-in, context model, and the expected
// mailbox are derived from `ntm --robot-status` and the roster convention. Bead
// and branch/worktree are surfaced when known (from mcp-agent-mail); they stay
// empty until that data path is wired, rather than being fabricated.
type backgroundAgentInfo struct {
	Session      string `json:"session"`
	Runtime      string `json:"runtime"`
	Mailbox      string `json:"mailbox,omitempty"`
	Bead         string `json:"bead,omitempty"`
	Branch       string `json:"branch,omitempty"`
	PaneIdx      int    `json:"pane_idx"`
	Active       bool   `json:"active"`
	Health       string `json:"health"`
	ProcessState string `json:"process_state,omitempty"`
	LastCheckIn  string `json:"last_check_in,omitempty"`
	ContextModel string `json:"context_model,omitempty"`
}

// bgNTMStatusPayload mirrors the subset of `ntm --robot-status` JSON that ao
// status consumes. Kept local to status.go so the background-agent fields
// (last_output_ts, is_active, process_state) can grow independently of the
// `ao agent ntm-status` structs.
type bgNTMStatusPayload struct {
	Sessions []bgNTMSession `json:"sessions"`
}

type bgNTMSession struct {
	Name   string       `json:"name"`
	Agents []bgNTMAgent `json:"agents"`
}

type bgNTMAgent struct {
	Type             string `json:"type"`
	PaneIdx          int    `json:"pane_idx"`
	IsActive         bool   `json:"is_active"`
	LastOutputTS     string `json:"last_output_ts"`
	ProcessState     string `json:"process_state"`
	ProcessStateName string `json:"process_state_name"`
	ContextModel     string `json:"context_model"`
}

func defaultNTMStatusRunner(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, "ntm", "--robot-status").Output()
}

// loadBackgroundAgents probes NTM for AgentOps background-agent sessions. It is
// fail-open: when NTM is absent or errors, it returns nil so `ao status` still
// renders the rest of its report.
func loadBackgroundAgents(ctx context.Context, now time.Time) []backgroundAgentInfo {
	raw, err := statusNTMRunner(ctx)
	if err != nil {
		return nil
	}
	return parseBackgroundAgents(raw, now)
}

// parseBackgroundAgents projects raw `ntm --robot-status` JSON into one record
// per agent pane in a background-agent session. Non-agent panes (shell
// controllers with an unknown runtime) are skipped.
func parseBackgroundAgents(raw []byte, now time.Time) []backgroundAgentInfo {
	var payload bgNTMStatusPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	var out []backgroundAgentInfo
	for _, s := range payload.Sessions {
		if !strings.HasPrefix(s.Name, backgroundAgentSessionPrefix) {
			continue
		}
		for _, a := range s.Agents {
			runtime := normalizeBackgroundRuntime(a.Type)
			if runtime == "" {
				continue
			}
			out = append(out, backgroundAgentInfo{
				Session:      s.Name,
				Runtime:      runtime,
				Mailbox:      backgroundMailbox(runtime),
				PaneIdx:      a.PaneIdx,
				Active:       a.IsActive,
				Health:       classifyBackgroundHealth(a, now),
				ProcessState: a.ProcessStateName,
				LastCheckIn:  formatBackgroundCheckIn(a.LastOutputTS, now),
				ContextModel: a.ContextModel,
			})
		}
	}
	return out
}

// normalizeBackgroundRuntime maps NTM pane types to AgentOps runtime names.
// Returns "" for non-agent panes (e.g. an unknown shell controller).
func normalizeBackgroundRuntime(ntmType string) string {
	switch strings.ToLower(strings.TrimSpace(ntmType)) {
	case "claude", "cc", "claude-code":
		return "claude"
	case "codex", "cod":
		return "codex"
	default:
		return ""
	}
}

// backgroundMailbox returns the expected mcp-agent-mail worker identity for a
// runtime, matching the NTM background-agent roster convention.
func backgroundMailbox(runtime string) string {
	switch runtime {
	case "claude":
		return "agentops-claude-ntm-worker"
	case "codex":
		return "agentops-codex-ntm-worker"
	default:
		return ""
	}
}

// classifyBackgroundHealth derives ok|stuck|error from a single robot-status
// snapshot: a zombie/dead process is an error; a pane silent past
// backgroundStuckAfter is stuck; otherwise ok.
func classifyBackgroundHealth(a bgNTMAgent, now time.Time) string {
	switch strings.ToUpper(strings.TrimSpace(a.ProcessState)) {
	case "Z", "X":
		return "error"
	}
	if strings.Contains(strings.ToLower(a.ProcessStateName), "zombie") {
		return "error"
	}
	if ts, err := time.Parse(time.RFC3339, a.LastOutputTS); err == nil {
		if now.Sub(ts) > backgroundStuckAfter {
			return "stuck"
		}
	}
	return "ok"
}

// formatBackgroundCheckIn renders the age of a pane's last output as a brief
// "<dur> ago" string. Returns "" when the timestamp is missing/unparseable.
func formatBackgroundCheckIn(lastOutputTS string, now time.Time) string {
	if strings.TrimSpace(lastOutputTS) == "" {
		return ""
	}
	ts, err := time.Parse(time.RFC3339, lastOutputTS)
	if err != nil {
		return ""
	}
	d := now.Sub(ts)
	if d < 0 {
		d = 0
	}
	return formatDurationBrief(d) + " ago"
}

func runStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	baseDir := filepath.Join(cwd, storage.DefaultBaseDir)
	status := &statusOutput{
		BaseDir: baseDir,
	}

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		status.Initialized = false
		return outputStatus(status)
	}
	status.Initialized = true

	loadRecentSessions(baseDir, status)

	provPath := filepath.Join(baseDir, storage.ProvenanceDir, storage.ProvenanceFile)
	graph, err := provenance.NewGraph(provPath)
	if err == nil {
		stats := graph.GetStats()
		status.ProvenanceStats = &provStats{
			TotalRecords:     stats.TotalRecords,
			UniqueSessions:   stats.UniqueSessions,
			UniqueWorkspaces: stats.UniqueWorkspaces,
		}
	}

	status.Flywheel = loadFlywheelBrief(cwd)
	status.QualitySignals = loadQualitySignals(filepath.Dir(baseDir), 10)
	status.BackgroundAgents = loadBackgroundAgents(cmd.Context(), time.Now())

	return outputStatus(status)
}

// printFlywheelHealth prints the flywheel health section for table output.
func printFlywheelHealth(fw *flywheelBrief) {
	fmt.Println("\nFlywheel Health")
	fmt.Println("───────────────")
	fmt.Printf("  Status:     %s\n", fw.Status)
	fmt.Printf("  Artifacts:  %d total, %d new (7d), %d stale (90d+)\n",
		fw.TotalArtifacts, fw.NewArtifacts, fw.StaleArtifacts)
	velocitySign := "+"
	if fw.Velocity < 0 {
		velocitySign = ""
	}
	fmt.Printf("  Velocity:   %s%.3f/week\n", velocitySign, fw.Velocity)
	if fw.LastForgeAge != "" {
		fmt.Printf("  Last forge: %s ago\n", fw.LastForgeAge)
	}
	if fw.PromotedFindings > 0 || fw.PlanningRules > 0 || fw.PreMortemChecks > 0 {
		fmt.Printf("  Signals:    %d findings, %d rules, %d checks\n",
			fw.PromotedFindings, fw.PlanningRules, fw.PreMortemChecks)
	}
	if fw.UnconsumedItems > 0 || fw.HighSeverityUnconsumed > 0 {
		fmt.Printf("  Backlog:    %d items, %d high severity\n",
			fw.UnconsumedItems, fw.HighSeverityUnconsumed)
	}
}

// printBackgroundAgents renders the NTM background-agent section for table
// output. A "-" marks fields NTM does not carry (bead/branch come from
// mcp-agent-mail when available).
func printBackgroundAgents(agents []backgroundAgentInfo) {
	fmt.Println("\nBackground Agents (NTM)")
	fmt.Println("───────────────────────")
	for _, a := range agents {
		active := ""
		if a.Active {
			active = " *active"
		}
		fmt.Printf("  %s  pane=%d  runtime=%s  health=%s  mailbox=%s%s\n",
			a.Session, a.PaneIdx, emptyDashStatus(a.Runtime), emptyDashStatus(a.Health),
			emptyDashStatus(a.Mailbox), active)
		fmt.Printf("      bead=%s  branch=%s  last_check_in=%s  model=%s\n",
			emptyDashStatus(a.Bead), emptyDashStatus(a.Branch),
			emptyDashStatus(a.LastCheckIn), emptyDashStatus(a.ContextModel))
	}
}

// emptyDashStatus renders a blank field as "-" for human-readable output.
func emptyDashStatus(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func outputStatus(status *statusOutput) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	fmt.Println("AgentOps Status")
	fmt.Println("==============")
	fmt.Println()

	if !status.Initialized {
		fmt.Println("Status: Not initialized")
		fmt.Println()
		fmt.Println("Run 'ao init' to initialize AgentOps in this directory.")
		return nil
	}

	fmt.Println("Status: Initialized ✓")
	fmt.Printf("Base Directory: %s\n", status.BaseDir)
	fmt.Println()

	fmt.Printf("Sessions: %d\n", status.SessionCount)

	if len(status.RecentSessions) > 0 {
		fmt.Println("\nRecent Sessions:")
		for _, s := range status.RecentSessions {
			fmt.Printf("  %s  %s\n", s.Date, s.Summary)
		}
	}

	if status.ProvenanceStats != nil {
		fmt.Println("\nProvenance:")
		fmt.Printf("  Records: %d\n", status.ProvenanceStats.TotalRecords)
		fmt.Printf("  Sessions: %d\n", status.ProvenanceStats.UniqueSessions)
		if status.ProvenanceStats.UniqueWorkspaces > 0 {
			fmt.Printf("  Workspaces: %d\n", status.ProvenanceStats.UniqueWorkspaces)
		}
	}

	if status.Flywheel != nil {
		printFlywheelHealth(status.Flywheel)
	}

	if len(status.QualitySignals) > 0 {
		fmt.Println("\nSession Quality Signals")
		fmt.Println("───────────────────────")
		for _, signal := range status.QualitySignals {
			fmt.Printf("  %s  %s  %s\n",
				truncateStatus(signal.Timestamp, 20),
				truncateStatus(signal.SignalType, 24),
				truncateStatus(signal.Detail, 80))
		}
	}

	if len(status.BackgroundAgents) > 0 {
		printBackgroundAgents(status.BackgroundAgents)
	}

	fmt.Println("\nCommands:")
	fmt.Println("  ao forge transcript <path>  - Extract knowledge from transcript")
	fmt.Println("  ao search <query>           - Search knowledge base")
	fmt.Println("  ao trace <artifact>         - Trace provenance")
	fmt.Println("  ao flywheel status          - Detailed flywheel metrics")
	fmt.Println("  ao agent ntm-status <name>  - Per-pane NTM background-agent detail")

	return nil
}

func truncateStatus(s string, maxLen int) string {
	// Remove newlines
	s = firstLine(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

// findLastForgeTime returns the modification time of the most recent retro or learning artifact.
func findLastForgeTime(baseDir string) time.Time {
	var latest time.Time
	dirs := []string{
		filepath.Join(baseDir, ".agents", "retros"),
		filepath.Join(baseDir, ".agents", "learnings"),
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
	}
	return latest
}

// formatDurationBrief formats a duration as a human-friendly short string (e.g., "2h", "3d").
func formatDurationBrief(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dw", days/7)
}
