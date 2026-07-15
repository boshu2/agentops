// practices: [wiki-knowledge-surface, lean-startup]
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	minePkg "github.com/boshu2/agentops/cli/internal/mine"
)

var (
	mineSourcesFlag string
	mineSince       string
	mineOutputDir   string
	mineQuiet       bool
)

var mineCmd = &cobra.Command{
	Use:   "mine",
	Short: "Extract knowledge signal from git, .agents/, and code",
	Long: `Mine scans all reachable data sources for patterns and insights
that were never explicitly extracted into learnings or patterns.

Sources (--sources flag, comma-separated):
  git     Git log + diffs: recurring fix patterns, co-change clusters
  agents  .agents/research/ files not yet referenced in learnings
  code    gocyclo hotspots: functions edited repeatedly or high CC
  events  RPI C2 event streams: error patterns, gate verdicts (opt-in)

Output goes to .agents/mine/YYYY-MM-DD-HH.json (structured JSON).
Mine is non-destructive: it only reads and appends.

Examples:
  ao mine                           # all sources, last 26h
  ao mine --since 7d --sources git  # git only, last week
  ao mine --dry-run                 # show what would be extracted`,
	RunE: runMine,
}

func init() {
	mineCmd.GroupID = "knowledge"
	rootCmd.AddCommand(mineCmd)
	mineCmd.Flags().StringVar(&mineSourcesFlag, "sources", "git,agents,code",
		"Comma-separated sources to mine (git, agents, code, events)")
	mineCmd.Flags().StringVar(&mineSince, "since", "26h",
		"How far back to look (e.g. 26h, 7d)")
	mineCmd.Flags().StringVar(&mineOutputDir, "output-dir", ".agents/mine",
		"Directory for mine output JSON")
	mineCmd.Flags().BoolVar(&mineQuiet, "quiet", false, "Suppress progress output")
}

// ---------------------------------------------------------------------------
// Type aliases — preserve the historical cmd/ao shape so existing tests
// and callers keep compiling unchanged. The source of truth is now
// cli/internal/mine.
// ---------------------------------------------------------------------------

// MineReport is the top-level output of ao mine.
type MineReport = minePkg.Report

// GitFindings holds signal extracted from git log.
type GitFindings = minePkg.GitFindings

// AgentsFindings holds signal from .agents/ directory scanning.
type AgentsFindings = minePkg.AgentsFindings

// CodeFindings holds signal from code complexity analysis.
type CodeFindings = minePkg.CodeFindings

// ComplexityHotspot represents a high-complexity function with recent edits.
type ComplexityHotspot = minePkg.ComplexityHotspot

// EventsFindings holds signal extracted from RPI C2 event streams.
type EventsFindings = minePkg.EventsFindings

// EventErrorSummary captures an error event from a run.
type EventErrorSummary = minePkg.EventErrorSummary

// GateVerdictSummary captures a gate verdict event from a run.
type GateVerdictSummary = minePkg.GateVerdictSummary

func runMine(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	window, err := parseMineWindow(mineSince)
	if err != nil {
		return fmt.Errorf("parse --since: %w", err)
	}

	sources, err := splitSources(mineSourcesFlag)
	if err != nil {
		return err
	}

	if GetDryRun() {
		if GetOutput() == "json" {
			return encodeMineDryRunJSON(cmd.OutOrStdout(), sources, window)
		}
		return printMineDryRun(cmd.OutOrStdout(), sources, window)
	}

	opts := minePkg.RunOpts{
		Sources:      sources,
		Window:       window,
		OutputDir:    mineOutputDir,
		Quiet:        mineQuiet,
		ErrOut:       cmd.ErrOrStderr(),
		MineEventsFn: mineEvents,
	}

	report, err := minePkg.Run(cwd, opts)
	if err != nil {
		return err
	}

	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	if !mineQuiet {
		printMineSummary(cmd.OutOrStdout(), report)
	}

	return nil
}

// parseMineWindow parses a duration string with support for "h", "m", and "d" suffixes.
func parseMineWindow(s string) (time.Duration, error) { return minePkg.ParseWindow(s) }

// splitSources splits and validates a comma-separated source list.
func splitSources(s string) ([]string, error) { return minePkg.SplitSources(s) }

// mineAgentsDir scans .agents/research/ for files not referenced in learnings.
// Thin wrapper so existing tests keep calling the package-main symbol.
func mineAgentsDir(cwd string) (*AgentsFindings, error) { return minePkg.MineAgentsDir(cwd) }

// readDirContent reads all .md file contents from a directory.
func readDirContent(dir string) (map[string]string, error) { return minePkg.ReadDirContent(dir) }

// countRecentEdits counts how many commits touched a file within the given window.
func countRecentEdits(cwd, file string, window time.Duration) int {
	return minePkg.CountRecentEdits(cwd, file, window)
}

// printMineDryRun prints what would be extracted without actually doing it.
func printMineDryRun(w io.Writer, sources []string, window time.Duration) error {
	fmt.Fprintf(w, "[dry-run] ao mine\n")
	fmt.Fprintf(w, "  sources: %s\n", strings.Join(sources, ", "))
	fmt.Fprintf(w, "  window:  %s\n", window)
	fmt.Fprintf(w, "  output:  %s\n", mineOutputDir)
	fmt.Fprintln(w, "\nNo files will be written.")
	return nil
}

// encodeMineDryRunJSON emits a single JSON document describing the dry-run
// parameters. This keeps `ao mine --json --dry-run` parseable by piped
// consumers (jq, scripts, tests) instead of dumping human text onto stdout
// while the `--json` flag is set.
func encodeMineDryRunJSON(w io.Writer, sources []string, window time.Duration) error {
	payload := struct {
		DryRun       bool     `json:"dry_run"`
		Sources      []string `json:"sources"`
		WindowString string   `json:"window"`
		OutputDir    string   `json:"output_dir"`
		Note         string   `json:"note"`
	}{
		DryRun:       true,
		Sources:      sources,
		WindowString: window.String(),
		OutputDir:    mineOutputDir,
		Note:         "No files will be written.",
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// printMineSummary prints a human-readable summary of the mine report.
func printMineSummary(w io.Writer, r *MineReport) {
	fmt.Fprintln(w, "Mine complete.")
	if r.Git != nil {
		fmt.Fprintf(w, "  git: %d commits", r.Git.CommitCount)
		if len(r.Git.TopCoChangeFiles) > 0 {
			fmt.Fprintf(w, ", %d co-change files", len(r.Git.TopCoChangeFiles))
		}
		if len(r.Git.RecurringFixes) > 0 {
			fmt.Fprintf(w, ", %d fix patterns", len(r.Git.RecurringFixes))
		}
		fmt.Fprintln(w)
	}
	if r.Agents != nil {
		fmt.Fprintf(w, "  agents: %d research files, %d orphaned\n",
			r.Agents.TotalResearch, len(r.Agents.OrphanedResearch))
	}
	if r.Code != nil {
		if r.Code.Skipped {
			fmt.Fprintln(w, "  code: skipped (gocyclo not installed)")
		} else {
			fmt.Fprintf(w, "  code: %d hotspots\n", len(r.Code.Hotspots))
		}
	}
	if r.Events != nil {
		fmt.Fprintf(w, "  events: %d runs scanned, %d total events", r.Events.RunsScanned, r.Events.TotalEvents)
		if len(r.Events.ErrorEvents) > 0 {
			fmt.Fprintf(w, ", %d errors", len(r.Events.ErrorEvents))
		}
		fmt.Fprintln(w)
	}
}

// mineEvents scans historical event streams as inert evidence. It discovers
// files directly and never imports or invokes the removed RPI controller.
func mineEvents(cwd string, window time.Duration) (*EventsFindings, error) {
	entries, err := os.ReadDir(filepath.Join(cwd, ".agents", "rpi", "runs"))
	if err != nil {
		if os.IsNotExist(err) {
			return &EventsFindings{}, nil
		}
		return nil, err
	}
	if len(entries) == 0 {
		return &EventsFindings{}, nil
	}

	cutoff := time.Now().Add(-window)
	findings := &EventsFindings{
		EventTypeCounts: make(map[string]int),
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		events, err := loadRPIC2Events(cwd, runID)
		if err != nil || len(events) == 0 {
			continue
		}

		includedRun := false
		for _, ev := range events {
			if ts, parseErr := time.Parse(time.RFC3339, ev.Timestamp); parseErr == nil && ts.Before(cutoff) {
				continue
			}
			if !includedRun {
				findings.RunsScanned++
				includedRun = true
			}
			findings.TotalEvents++
			findings.EventTypeCounts[ev.Type]++

			if ev.Type == "error" {
				findings.ErrorEvents = append(findings.ErrorEvents, EventErrorSummary{
					RunID:     runID,
					Message:   ev.Message,
					Timestamp: ev.Timestamp,
				})
			}

			if strings.HasPrefix(ev.Type, "gate.") && strings.HasSuffix(ev.Type, ".verdict") {
				verdict := ""
				if ev.Details != nil {
					var d map[string]interface{}
					if json.Unmarshal(ev.Details, &d) == nil {
						if v, ok := d["verdict"].(string); ok {
							verdict = v
						}
					}
				}
				findings.GateVerdicts = append(findings.GateVerdicts, GateVerdictSummary{
					RunID:   runID,
					Phase:   ev.Phase,
					Type:    ev.Type,
					Verdict: verdict,
				})
			}
		}
	}

	return findings, nil
}
