// Package mine provides pure helpers and an in-process entry point for
// knowledge mining operations. Dream's INGEST stage and the cmd/ao cobra
// adapter both call mine.Run to execute a single mining pass; the cobra
// file remains as a thin wrapper that supplies flag/render concerns.
package mine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ValidSources enumerates the allowed source names for ao mine.
var ValidSources = map[string]bool{
	"git":    true,
	"agents": true,
	"code":   true,
	"events": true,
}

// ParseWindow parses a duration string with support for "h", "m", and "d" suffixes.
func ParseWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}
	suffix := s[len(s)-1:]
	numStr := s[:len(s)-1]
	switch suffix {
	case "d":
		days, err := strconv.Atoi(numStr)
		if err != nil {
			return 0, fmt.Errorf("invalid day count %q: %w", numStr, err)
		}
		if days <= 0 {
			return 0, fmt.Errorf("duration must be positive, got %q", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	case "h":
		hours, err := strconv.Atoi(numStr)
		if err != nil {
			return 0, fmt.Errorf("invalid hour count %q: %w", numStr, err)
		}
		if hours <= 0 {
			return 0, fmt.Errorf("duration must be positive, got %q", s)
		}
		return time.Duration(hours) * time.Hour, nil
	case "m":
		mins, err := strconv.Atoi(numStr)
		if err != nil {
			return 0, fmt.Errorf("invalid minute count %q: %w", numStr, err)
		}
		if mins <= 0 {
			return 0, fmt.Errorf("duration must be positive, got %q", s)
		}
		return time.Duration(mins) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unsupported duration suffix %q (use h, d, or m)", suffix)
	}
}

// SplitSources splits and validates a comma-separated source list.
func SplitSources(s string) ([]string, error) {
	parts := strings.Split(s, ",")
	var sources []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !ValidSources[p] {
			return nil, fmt.Errorf("unknown source %q (valid: git, agents, code, events)", p)
		}
		sources = append(sources, p)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no valid sources specified")
	}
	return sources, nil
}

// FindOrphanedResearch returns research file names not referenced in any learning content.
func FindOrphanedResearch(researchFiles []string, learningsContent map[string]string) []string {
	var orphaned []string
	for _, rf := range researchFiles {
		referenced := false
		for _, content := range learningsContent {
			if strings.Contains(content, rf) {
				referenced = true
				break
			}
		}
		if !referenced {
			orphaned = append(orphaned, rf)
		}
	}
	return orphaned
}

// ListMarkdownFiles returns names of .md files in a directory.
func ListMarkdownFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	return files, nil
}

// ReadDirContent reads all .md file contents from a directory.
func ReadDirContent(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	contents := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		contents[e.Name()] = string(data)
	}
	return contents, nil
}

// ---------------------------------------------------------------------------
// Report shape — mirrors the historical cmd/ao shape so the adapter can
// alias these types and existing tests stay green without modification.
// ---------------------------------------------------------------------------

// Report is the top-level output of a mine.Run pass. The shape is
// intentionally byte-compatible with the cobra command's historical JSON.
type Report struct {
	Timestamp    time.Time       `json:"timestamp"`
	SinceSeconds int64           `json:"since_seconds"`
	Sources      []string        `json:"sources"`
	Git          *GitFindings    `json:"git,omitempty"`
	Agents       *AgentsFindings `json:"agents,omitempty"`
	Code         *CodeFindings   `json:"code,omitempty"`
	Events       *EventsFindings `json:"events,omitempty"`
}

// GitFindings holds signal extracted from git log.
type GitFindings struct {
	CommitCount      int      `json:"commit_count"`
	TopCoChangeFiles []string `json:"top_co_change_files,omitempty"`
	RecurringFixes   []string `json:"recurring_fixes,omitempty"`
}

// AgentsFindings holds signal from .agents/ directory scanning.
type AgentsFindings struct {
	TotalResearch    int      `json:"total_research"`
	OrphanedResearch []string `json:"orphaned_research,omitempty"`
}

// CodeFindings holds signal from code complexity analysis.
type CodeFindings struct {
	Hotspots []ComplexityHotspot `json:"hotspots,omitempty"`
	Skipped  bool                `json:"skipped,omitempty"`
}

// EventsFindings holds signal extracted from RPI C2 event streams.
type EventsFindings struct {
	RunsScanned     int                  `json:"runs_scanned"`
	TotalEvents     int                  `json:"total_events"`
	EventTypeCounts map[string]int       `json:"event_type_counts,omitempty"`
	ErrorEvents     []EventErrorSummary  `json:"error_events,omitempty"`
	GateVerdicts    []GateVerdictSummary `json:"gate_verdicts,omitempty"`
}

// EventErrorSummary captures an error event from a run.
type EventErrorSummary struct {
	RunID     string `json:"run_id"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// GateVerdictSummary captures a gate verdict event from a run.
type GateVerdictSummary struct {
	RunID   string `json:"run_id"`
	Phase   int    `json:"phase"`
	Type    string `json:"type"`
	Verdict string `json:"verdict"`
}

// ---------------------------------------------------------------------------
// RunOpts / Run orchestrator
// ---------------------------------------------------------------------------

// RunOpts configures a single mine.Run pass. Dream's INGEST stage
// calls mine.Run(cwd, opts) in-process to discover git signal, orphaned
// research, complexity hotspots, and RPI event patterns; the cmd/ao
// adapter uses the same entry for `ao mine` operator invocations.
//
// Events-source support is dependency-injected because the upstream
// helpers (scanRegistryRuns / loadRPIC2Events) live in cmd/ao package
// main and cannot be imported from internal/. If MineEventsFn is nil
// and "events" is in Sources, the events source is treated as a no-op.
type RunOpts struct {
	// Context bounds source execution. Nil means context.Background().
	Context context.Context

	// Sources restricts the scan to a subset of {git,agents,code,events}.
	// Must be pre-validated via SplitSources.
	Sources []string

	// Window is how far back git/code/events should look.
	Window time.Duration

	// OutputDir is where the dated JSON report is written. Required
	// unless DryRun is true.
	OutputDir string

	// EmitWorkItems, when true, appends actionable mine findings to
	// .agents/rpi/next-work.jsonl for evolve to pick up.
	EmitWorkItems bool

	// Quiet suppresses soft-fail warning output. Hard errors are still
	// returned.
	Quiet bool

	// DryRun short-circuits all write operations. Callers should render
	// a dry-run summary themselves; Run returns a zero Report and nil
	// error when DryRun is true.
	DryRun bool

	// Now returns the current time. Defaults to time.Now.
	Now func() time.Time

	// ErrOut receives soft-fail warning messages for per-source failures
	// (unless Quiet is set). Nil ErrOut silently discards warnings.
	ErrOut io.Writer

	// MineEventsFn is the injected callback that drives the "events"
	// source. It is called only when Sources contains "events".
	// Signature matches the historical cmd/ao.mineEvents helper.
	MineEventsFn func(cwd string, window time.Duration) (*EventsFindings, error)
}

// Run executes one mine pass and returns the assembled Report. Run
// never writes to stdout/stderr beyond the ErrOut writer supplied via
// RunOpts; all rendering (JSON output, human summary) stays in the
// caller. Run does:
//
//  1. Run each requested source (git, agents, code, events), logging
//     per-source failures to ErrOut as warnings.
//  2. Write the dated + latest JSON report to OutputDir (unless DryRun).
//  3. Optionally emit actionable work items to .agents/rpi/next-work.jsonl.
//
// Source failures are soft: the Report is returned with whatever
// findings succeeded. Hard failures (writing the report, emitting work
// items when configured) return an error.
func Run(cwd string, opts RunOpts) (*Report, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	report := &Report{
		Timestamp:    opts.Now().UTC(),
		SinceSeconds: int64(opts.Window.Seconds()),
		Sources:      opts.Sources,
	}

	if opts.DryRun {
		// Dry-run does not execute sources or write output; the caller
		// is responsible for rendering a dry-run summary.
		return report, nil
	}

	if err := runSources(ctx, cwd, opts, report); err != nil {
		return report, err
	}

	if err := writeReport(opts.OutputDir, report); err != nil {
		return report, err
	}

	if opts.EmitWorkItems {
		if err := EmitWorkItems(cwd, report); err != nil && !opts.Quiet && opts.ErrOut != nil {
			fmt.Fprintf(opts.ErrOut, "warning: emit-work-items: %v\n", err)
		}
	}

	return report, nil
}

// runSources dispatches each requested source, accumulating findings
// into report. Per-source failures are logged to opts.ErrOut as
// warnings when Quiet is false.
func runSources(ctx context.Context, cwd string, opts RunOpts, report *Report) error {
	for _, src := range opts.Sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		switch src {
		case "git":
			findings, gitErr := MineGitLogContext(ctx, cwd, opts.Window)
			if gitErr != nil {
				warnSource(opts, "git", gitErr)
				continue
			}
			report.Git = findings
		case "agents":
			findings, agErr := MineAgentsDir(cwd)
			if agErr != nil {
				warnSource(opts, "agents", agErr)
				continue
			}
			report.Agents = findings
		case "code":
			findings, codeErr := MineCodeComplexityContext(ctx, cwd, opts.Window)
			if codeErr != nil {
				warnSource(opts, "code", codeErr)
				continue
			}
			report.Code = findings
		case "events":
			if opts.MineEventsFn == nil {
				continue // events requested but no callback wired — no-op
			}
			findings, evErr := opts.MineEventsFn(cwd, opts.Window)
			if evErr != nil {
				warnSource(opts, "events", evErr)
				continue
			}
			report.Events = findings
		}
	}
	return ctx.Err()
}

// warnSource logs a per-source failure to opts.ErrOut unless Quiet is set.
func warnSource(opts RunOpts, src string, err error) {
	if opts.Quiet || opts.ErrOut == nil {
		return
	}
	fmt.Fprintf(opts.ErrOut, "warning: %s source: %v\n", src, err)
}

// writeReport writes the mine report as JSON to the output directory
// (both <date>.json and latest.json).
func writeReport(dir string, r *Report) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mine report: %w", err)
	}
	dateStr := r.Timestamp.Format("2006-01-02-15")
	return WriteMineReportJSON(dir, data, dateStr)
}

// ---------------------------------------------------------------------------
// Source implementations — moved from cmd/ao/mine.go
// ---------------------------------------------------------------------------

// gitLogFixRe matches commit subjects beginning with fix/bugfix/hotfix.
var gitLogFixRe = regexp.MustCompile(`(?i)^[0-9a-f]+ (fix|bugfix|hotfix)`)

// MineGitLog extracts signal from git log within the given time window.
// Shells out to `git log`; callers with no git repo should exclude the
// git source from opts.Sources.
func MineGitLog(cwd string, window time.Duration) (*GitFindings, error) {
	return MineGitLogContext(context.Background(), cwd, window)
}

// MineGitLogContext extracts signal from git log with context cancellation.
func MineGitLogContext(ctx context.Context, cwd string, window time.Duration) (*GitFindings, error) {
	sinceArg := fmt.Sprintf("--since=%d seconds ago", int64(window.Seconds()))
	cmd := exec.CommandContext(ctx, "git", "log", sinceArg, "--name-only", "--pretty=format:%H %s")
	cmd.Dir = cwd

	out, err := cmd.Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("git log: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git log: %w", err)
	}

	findings := &GitFindings{}
	fileFreq := make(map[string]int)
	var fixPatterns []string

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Commit line: starts with a hash
		if len(line) >= 41 && line[40] == ' ' {
			findings.CommitCount++
			if gitLogFixRe.MatchString(line) {
				// Extract the subject after the hash
				subject := line[41:]
				fixPatterns = append(fixPatterns, subject)
			}
		} else {
			// File line
			fileFreq[line]++
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return findings, fmt.Errorf("scan git log output: %w", scanErr)
	}

	// Build co-change clusters: files appearing in >=3 distinct commits
	type fileCount struct {
		file  string
		count int
	}
	var frequent []fileCount
	for f, c := range fileFreq {
		if c >= 3 {
			frequent = append(frequent, fileCount{f, c})
		}
	}
	sort.Slice(frequent, func(i, j int) bool {
		return frequent[i].count > frequent[j].count
	})

	if len(frequent) > 0 {
		topFiles := make([]string, len(frequent))
		for i, fc := range frequent {
			topFiles[i] = fc.file
		}
		findings.TopCoChangeFiles = topFiles
	}

	findings.RecurringFixes = fixPatterns

	return findings, nil
}

// MineAgentsDir scans .agents/research/ for files not referenced in
// .agents/learnings/.
func MineAgentsDir(cwd string) (*AgentsFindings, error) {
	researchDir := filepath.Join(cwd, ".agents", "research")
	learningsDir := filepath.Join(cwd, ".agents", "learnings")

	findings := &AgentsFindings{}

	researchFiles, err := ListMarkdownFiles(researchDir)
	if err != nil {
		if os.IsNotExist(err) {
			return findings, nil
		}
		return findings, fmt.Errorf("read research dir: %w", err)
	}
	findings.TotalResearch = len(researchFiles)
	if len(researchFiles) == 0 {
		return findings, nil
	}

	learningsContent, err := ReadDirContent(learningsDir)
	if err != nil && !os.IsNotExist(err) {
		return findings, fmt.Errorf("read learnings dir: %w", err)
	}

	findings.OrphanedResearch = FindOrphanedResearch(researchFiles, learningsContent)
	return findings, nil
}

// gocycloLineRe matches gocyclo output lines like
// "15 main runMine cli/cmd/ao/mine.go:100:1".
var gocycloLineRe = regexp.MustCompile(`^(\d+)\s+(\S+)\s+(\S+)\s+(\S+):(\d+):\d+$`)

// MineCodeComplexity runs gocyclo on the cli/ subtree and correlates
// hotspots with recent git edits. If gocyclo is not installed or fails,
// returns a skipped finding without error.
func MineCodeComplexity(cwd string, window time.Duration) (*CodeFindings, error) {
	return MineCodeComplexityContext(context.Background(), cwd, window)
}

// MineCodeComplexityContext runs gocyclo with context cancellation.
func MineCodeComplexityContext(ctx context.Context, cwd string, window time.Duration) (*CodeFindings, error) {
	findings := &CodeFindings{}

	gocycloPath, err := exec.LookPath("gocyclo")
	if err != nil {
		findings.Skipped = true
		return findings, nil
	}

	cmd := exec.CommandContext(ctx, gocycloPath, "-top", "10", "cli/")
	cmd.Dir = cwd

	out, err := cmd.Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return findings, ctxErr
		}
		// gocyclo may fail if cli/ doesn't exist
		findings.Skipped = true
		return findings, nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		matches := gocycloLineRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		cc, _ := strconv.Atoi(matches[1])
		funcName := matches[3]
		file := matches[4]

		recentEdits := CountRecentEditsContext(ctx, cwd, file, window)

		findings.Hotspots = append(findings.Hotspots, ComplexityHotspot{
			File:        file,
			Func:        funcName,
			Complexity:  cc,
			RecentEdits: recentEdits,
		})
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return findings, fmt.Errorf("scan gocyclo output: %w", scanErr)
	}

	return findings, nil
}

// CountRecentEdits counts how many commits touched a file within the
// given window.
func CountRecentEdits(cwd, file string, window time.Duration) int {
	return CountRecentEditsContext(context.Background(), cwd, file, window)
}

// CountRecentEditsContext counts recent commits with context cancellation.
func CountRecentEditsContext(ctx context.Context, cwd, file string, window time.Duration) int {
	sinceArg := fmt.Sprintf("--since=%d seconds ago", int64(window.Seconds()))
	cmd := exec.CommandContext(ctx, "git", "log", sinceArg, "--oneline", "--", file)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0
	}
	return len(lines)
}

// ---------------------------------------------------------------------------
// Work item emission — moved from cmd/ao/mine.go
// ---------------------------------------------------------------------------

// CollectMineWorkItems builds actionable work items from a Report
// (complexity hotspots + orphaned research).
func CollectMineWorkItems(r *Report) []WorkItemEmit {
	var items []WorkItemEmit
	if r.Code != nil {
		items = append(items, CollectWorkItemsFromHotspots(r.Code.Hotspots)...)
	}
	if r.Agents != nil {
		items = append(items, CollectWorkItemsFromOrphans(r.Agents.OrphanedResearch)...)
	}
	return items
}

// EmitWorkItems translates mine findings into next-work.jsonl entries
// for evolve to pick up. Orphaned research files map to severity:medium;
// code hotspots map to severity:high. Dedup is item-level — each item
// gets a stable ID, only new items are emitted.
func EmitWorkItems(cwd string, r *Report) error {
	items := CollectMineWorkItems(r)
	if len(items) == 0 {
		return nil // nothing to emit
	}

	nextWorkPath := filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")
	if err := os.MkdirAll(filepath.Dir(nextWorkPath), 0o750); err != nil {
		return fmt.Errorf("ensure next-work dir: %w", err)
	}

	existingIDs, err := LoadExistingMineIDs(nextWorkPath)
	if err != nil {
		return fmt.Errorf("load existing mine IDs: %w", err)
	}
	var newItems []WorkItemEmit
	for _, item := range items {
		if !existingIDs[item.ID] {
			newItems = append(newItems, item)
		}
	}
	if len(newItems) == 0 {
		return nil // all items already present
	}

	return WriteWorkItems(nextWorkPath, newItems, r.Timestamp.UTC().Format(time.RFC3339))
}
