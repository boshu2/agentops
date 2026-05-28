// practices: [ai-assisted-dev, code-complete]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/vibecheck"
)

var (
	vibeCheckMarkdown bool
	vibeCheckSince    string
	vibeCheckRepo     string
	vibeCheckFull     bool
)

var vibeCheckCmd = &cobra.Command{
	Use:     "vibe-check",
	Aliases: []string{"vibecheck"},
	Short:   "Analyze codebase health and vibe",
	Long: `Run a comprehensive vibe-check analysis on your repository.

Analyzes:
  - Commit timeline and patterns (velocity, rework, spirals)
  - Code quality metrics (complexity, trust)
  - Detects problematic patterns (amnesia, drift, test lies, logging gaps)
  - Computes overall health grade (A-F)

Output modes:
  --json     Structured JSON result
  --markdown Formatted markdown report

Examples:
  ao vibe-check
  ao vibe-check --since 30d
  ao vibe-check --repo /path/to/repo --json
  ao vibe-check --markdown --full`,
	RunE: runVibeCheck,
}

func init() {
	vibeCheckCmd.GroupID = "core"
	rootCmd.AddCommand(vibeCheckCmd)
	vibeCheckCmd.Flags().BoolVar(&vibeCheckMarkdown, "markdown", false, "Output as markdown report")
	vibeCheckCmd.Flags().StringVar(&vibeCheckSince, "since", "7d", "Time window for analysis (e.g., 7d, 30d, 90d)")
	vibeCheckCmd.Flags().StringVar(&vibeCheckRepo, "repo", ".", "Path to git repository")
	vibeCheckCmd.Flags().BoolVar(&vibeCheckFull, "full", false, "Show all metrics and findings (verbose)")
}

func runVibeCheck(cmd *cobra.Command, args []string) error {
	if GetDryRun() {
		fmt.Printf("[dry-run] Would analyze vibe-check for repo: %s\n", vibeCheckRepo)
		return nil
	}

	// Parse the 'since' duration
	duration, err := parseDuration(vibeCheckSince)
	if err != nil {
		return fmt.Errorf("invalid duration format: %w", err)
	}

	repoPath, err := resolveVibeCheckRepoPath(vibeCheckRepo)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}

	// Run analysis
	opts := vibecheck.AnalyzeOptions{
		RepoPath: repoPath,
		Since:    time.Now().Add(-duration),
	}

	result, err := vibecheck.Analyze(opts)
	if err != nil {
		return fmt.Errorf("vibe-check analysis failed: %w", err)
	}

	// Output result based on format
	if GetOutput() == "json" {
		return outputVibeCheckJSON(result)
	}

	if vibeCheckMarkdown {
		return outputVibeCheckMarkdown(result)
	}

	// Default: table output
	return outputVibeCheckTable(result)
}

func resolveVibeCheckRepoPath(repoPath string) (string, error) {
	if repoPath == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		repoPath = cwd
	}

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = absPath
	cmd.Env = gitDiscoveryEnv()
	out, err := cmd.Output()
	if err != nil {
		return absPath, nil
	}

	return strings.TrimSpace(string(out)), nil
}

func gitDiscoveryEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		switch {
		case strings.HasPrefix(entry, "GIT_DIR="):
			continue
		case strings.HasPrefix(entry, "GIT_WORK_TREE="):
			continue
		case strings.HasPrefix(entry, "GIT_COMMON_DIR="):
			continue
		default:
			env = append(env, entry)
		}
	}
	return env
}

// parseDuration parses durations like "7d", "30d", "90d", "1w", etc.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)

	// Handle common suffixes
	if strings.HasSuffix(s, "d") {
		var days int
		_, err := fmt.Sscanf(s, "%dd", &days)
		if err != nil {
			return 0, fmt.Errorf("invalid days format: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	if strings.HasSuffix(s, "w") {
		var weeks int
		_, err := fmt.Sscanf(s, "%dw", &weeks)
		if err != nil {
			return 0, fmt.Errorf("invalid weeks format: %s", s)
		}
		return time.Duration(weeks) * 7 * 24 * time.Hour, nil
	}

	// Fallback to time.ParseDuration
	return time.ParseDuration(s)
}

// outputVibeCheckJSON outputs the result as JSON.
func outputVibeCheckJSON(result *vibecheck.VibeCheckResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// outputVibeCheckMarkdown outputs the result as markdown.
func outputVibeCheckMarkdown(result *vibecheck.VibeCheckResult) error {
	fmt.Printf("# Vibe Check Report\n\n")
	fmt.Printf("## Overall Health: **%s** (%.1f%%)\n\n", result.Grade, result.Score)
	printMarkdownMetrics(result.Metrics)
	printMarkdownFindings(result.Findings)
	printMarkdownEvents(result.Events)
	return nil
}

// printMarkdownMetrics renders the metrics section of the markdown report.
func printMarkdownMetrics(metrics map[string]float64) {
	fmt.Printf("## Metrics\n\n")
	if len(metrics) == 0 {
		return
	}
	fmt.Println("| Metric | Value |")
	fmt.Println("|--------|-------|")
	names := make([]string, 0, len(metrics))
	for name := range metrics {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Printf("| %s | %.2f |\n", name, metrics[name])
	}
	fmt.Println()
}

// printMarkdownFindings renders the findings section of the markdown report.
func printMarkdownFindings(findings []vibecheck.Finding) {
	fmt.Printf("## Findings\n\n")
	if len(findings) == 0 {
		fmt.Println("No issues found.")
		return
	}
	for _, finding := range findings {
		printMarkdownFinding(finding)
	}
}

// severityEmoji maps finding severity to its markdown emoji.
func severityEmoji(severity string) string {
	switch severity {
	case "error":
		return "❌"
	case "info":
		return "ℹ️"
	default:
		return "⚠️"
	}
}

// printMarkdownFinding renders a single finding in markdown.
func printMarkdownFinding(finding vibecheck.Finding) {
	fmt.Printf("### %s %s\n\n", severityEmoji(finding.Severity), finding.Category)
	fmt.Printf("**Message:** %s\n\n", finding.Message)
	if finding.File != "" {
		fmt.Printf("**File:** %s", finding.File)
		if finding.Line > 0 {
			fmt.Printf(":%d", finding.Line)
		}
		fmt.Printf("\n\n")
	}
}

// printMarkdownEvents renders the events section if --full is enabled.
func printMarkdownEvents(events []vibecheck.TimelineEvent) {
	if !vibeCheckFull || len(events) == 0 {
		return
	}
	fmt.Printf("## Recent Events (%d commits)\n\n", len(events))
	fmt.Println("| Date | Author | Message |")
	fmt.Println("|------|--------|---------|")
	for _, event := range events {
		msg := event.Message
		if len(msg) > 50 {
			msg = msg[:50] + "..."
		}
		fmt.Printf("| %s | %s | %s |\n", event.Timestamp.Format("2006-01-02"), event.Author, msg)
	}
	fmt.Println()
}

// outputVibeCheckTable outputs the result as a formatted table.
func outputVibeCheckTable(result *vibecheck.VibeCheckResult) error {
	// Header
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Printf("║ Vibe Check Report                                  %s  %3.0f%% ║\n", result.Grade, result.Score)
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Metrics
	fmt.Println("Metrics:")
	fmt.Println("────────")
	if len(result.Metrics) > 0 {
		// Sort metrics by name
		names := make([]string, 0, len(result.Metrics))
		for name := range result.Metrics {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			val := result.Metrics[name]
			fmt.Printf("  %-30s %8.2f\n", name, val)
		}
	} else {
		fmt.Println("  (no metrics)")
	}
	fmt.Println()

	// Findings
	fmt.Println("Findings:")
	fmt.Println("─────────")
	if len(result.Findings) > 0 {
		for _, finding := range result.Findings {
			severity := "[" + strings.ToUpper(finding.Severity[:1]) + "]"
			fmt.Printf("  %s %s - %s\n", severity, finding.Category, finding.Message)
			if finding.File != "" {
				location := finding.File
				if finding.Line > 0 {
					location = fmt.Sprintf("%s:%d", finding.File, finding.Line)
				}
				fmt.Printf("      at %s\n", location)
			}
		}
	} else {
		fmt.Println("  ✓ No issues detected")
	}
	fmt.Println()

	// Summary
	if vibeCheckFull {
		fmt.Printf("Summary: %d commits analyzed, %d findings, grade %s\n",
			len(result.Events), len(result.Findings), result.Grade)
	}

	fmt.Println()
	return nil
}
