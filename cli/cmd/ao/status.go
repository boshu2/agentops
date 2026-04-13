package main

import (
	"encoding/json"
	"fmt"
	"os"
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

func init() {
	statusCmd.GroupID = "core"
	rootCmd.AddCommand(statusCmd)
}

type statusOutput struct {
	Initialized     bool                `json:"initialized"`
	BaseDir         string              `json:"base_dir"`
	SessionCount    int                 `json:"session_count"`
	RecentSessions  []sessionInfo       `json:"recent_sessions,omitempty"`
	ProvenanceStats *provStats          `json:"provenance_stats,omitempty"`
	Flywheel        *flywheelBrief      `json:"flywheel,omitempty"`
	QualitySignals  []qualitySignalInfo `json:"quality_signals,omitempty"`
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

// loadFlywheelBrief computes the flywheel health summary for status output.
func loadFlywheelBrief(cwd string) *flywheelBrief {
	metrics, err := computeMetrics(cwd, 7)
	if err != nil {
		return nil
	}
	populateGoldenSignals(cwd, 7, metrics)
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

	fmt.Println("\nCommands:")
	fmt.Println("  ao forge transcript <path>  - Extract knowledge from transcript")
	fmt.Println("  ao search <query>           - Search knowledge base")
	fmt.Println("  ao trace <artifact>         - Trace provenance")
	fmt.Println("  ao flywheel status          - Detailed flywheel metrics")

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
