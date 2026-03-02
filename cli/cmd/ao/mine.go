package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
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

	"github.com/spf13/cobra"
)

var (
	mineSourcesFlag    string
	mineSince          string
	mineOutputDir      string
	mineQuiet          bool
	mineEmitWorkItems  bool
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
		"Comma-separated sources to mine (git, agents, code)")
	mineCmd.Flags().StringVar(&mineSince, "since", "26h",
		"How far back to look (e.g. 26h, 7d)")
	mineCmd.Flags().StringVar(&mineOutputDir, "output-dir", ".agents/mine",
		"Directory for mine output JSON")
	mineCmd.Flags().BoolVar(&mineQuiet, "quiet", false, "Suppress progress output")
	mineCmd.Flags().BoolVar(&mineEmitWorkItems, "emit-work-items", false,
		"Append actionable mine findings to .agents/rpi/next-work.jsonl for evolve to pick up")
}

// MineReport is the top-level output of ao mine.
type MineReport struct {
	Timestamp    time.Time       `json:"timestamp"`
	SinceSeconds int64           `json:"since_seconds"`
	Sources      []string        `json:"sources"`
	Git          *GitFindings    `json:"git,omitempty"`
	Agents       *AgentsFindings `json:"agents,omitempty"`
	Code         *CodeFindings   `json:"code,omitempty"`
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

// ComplexityHotspot represents a high-complexity function with recent edits.
type ComplexityHotspot struct {
	File        string `json:"file"`
	Func        string `json:"func"`
	Complexity  int    `json:"complexity"`
	RecentEdits int    `json:"recent_edits"`
}

// validMineSources enumerates the allowed source names.
var validMineSources = map[string]bool{
	"git":    true,
	"agents": true,
	"code":   true,
}

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
		return printMineDryRun(cmd.OutOrStdout(), sources, window)
	}

	report := &MineReport{
		Timestamp:    time.Now().UTC(),
		SinceSeconds: int64(window.Seconds()),
		Sources:      sources,
	}

	for _, src := range sources {
		switch src {
		case "git":
			findings, gitErr := mineGitLog(cwd, window)
			if gitErr != nil {
				if !mineQuiet {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: git source: %v\n", gitErr)
				}
				continue
			}
			report.Git = findings
		case "agents":
			findings, agErr := mineAgentsDir(cwd)
			if agErr != nil {
				if !mineQuiet {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: agents source: %v\n", agErr)
				}
				continue
			}
			report.Agents = findings
		case "code":
			findings, codeErr := mineCodeComplexity(cwd, window)
			if codeErr != nil {
				if !mineQuiet {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: code source: %v\n", codeErr)
				}
				continue
			}
			report.Code = findings
		}
	}

	if err := writeMineReport(mineOutputDir, report); err != nil {
		return err
	}

	if mineEmitWorkItems {
		if err := emitMineWorkItems(cwd, report); err != nil && !mineQuiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: emit-work-items: %v\n", err)
		}
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
// "d" is converted to hours (e.g. "7d" → 168h).
func parseMineWindow(s string) (time.Duration, error) {
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

// splitSources splits and validates a comma-separated source list.
func splitSources(s string) ([]string, error) {
	parts := strings.Split(s, ",")
	var sources []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !validMineSources[p] {
			return nil, fmt.Errorf("unknown source %q (valid: git, agents, code)", p)
		}
		sources = append(sources, p)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no valid sources specified")
	}
	return sources, nil
}

// mineGitLog extracts signal from git log within the given time window.
func mineGitLog(cwd string, window time.Duration) (*GitFindings, error) {
	sinceArg := fmt.Sprintf("--since=%d seconds ago", int64(window.Seconds()))
	cmd := exec.Command("git", "log", sinceArg, "--name-only", "--pretty=format:%H %s")
	cmd.Dir = cwd

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("git log: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git log: %w", err)
	}

	findings := &GitFindings{}
	fileFreq := make(map[string]int)
	var fixPatterns []string
	fixRe := regexp.MustCompile(`(?i)^[0-9a-f]+ (fix|bugfix|hotfix)`)

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Commit line: starts with a hash
		if len(line) >= 41 && line[40] == ' ' {
			findings.CommitCount++
			if fixRe.MatchString(line) {
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

// mineAgentsDir scans .agents/research/ for files not referenced in learnings.
func mineAgentsDir(cwd string) (*AgentsFindings, error) {
	researchDir := filepath.Join(cwd, ".agents", "research")
	learningsDir := filepath.Join(cwd, ".agents", "learnings")

	findings := &AgentsFindings{}

	// Read research files
	researchEntries, err := os.ReadDir(researchDir)
	if err != nil {
		if os.IsNotExist(err) {
			return findings, nil
		}
		return findings, fmt.Errorf("read research dir: %w", err)
	}

	var researchFiles []string
	for _, e := range researchEntries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			researchFiles = append(researchFiles, e.Name())
		}
	}
	findings.TotalResearch = len(researchFiles)

	if len(researchFiles) == 0 {
		return findings, nil
	}

	// Read all learnings content to check for references
	learningsContent, err := readDirContent(learningsDir)
	if err != nil && !os.IsNotExist(err) {
		return findings, fmt.Errorf("read learnings dir: %w", err)
	}

	// Check each research file for references in learnings
	for _, rf := range researchFiles {
		referenced := false
		for _, content := range learningsContent {
			if strings.Contains(content, rf) {
				referenced = true
				break
			}
		}
		if !referenced {
			findings.OrphanedResearch = append(findings.OrphanedResearch, rf)
		}
	}

	return findings, nil
}

// readDirContent reads all .md file contents from a directory, returning a map of filename→content.
func readDirContent(dir string) (map[string]string, error) {
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

// gocycloLineRe matches gocyclo output lines like "15 main runMine cli/cmd/ao/mine.go:100:1"
var gocycloLineRe = regexp.MustCompile(`^(\d+)\s+(\S+)\s+(\S+)\s+(\S+):(\d+):\d+$`)

// mineCodeComplexity runs gocyclo and correlates with recent git edits.
func mineCodeComplexity(cwd string, window time.Duration) (*CodeFindings, error) {
	findings := &CodeFindings{}

	gocycloPath, err := exec.LookPath("gocyclo")
	if err != nil {
		findings.Skipped = true
		return findings, nil
	}

	cmd := exec.Command(gocycloPath, "-top", "10", "cli/")
	cmd.Dir = cwd

	out, err := cmd.Output()
	if err != nil {
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

		recentEdits := countRecentEdits(cwd, file, window)

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

// countRecentEdits counts how many commits touched a file within the given window.
func countRecentEdits(cwd, file string, window time.Duration) int {
	sinceArg := fmt.Sprintf("--since=%d seconds ago", int64(window.Seconds()))
	cmd := exec.Command("git", "log", sinceArg, "--oneline", "--", file)
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

// writeMineReport writes the mine report as JSON to the output directory.
func writeMineReport(dir string, r *MineReport) error {
	if dir == "" {
		return fmt.Errorf("output directory must not be empty")
	}
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create mine output dir: %w", err)
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mine report: %w", err)
	}

	// Write dated file
	dateStr := r.Timestamp.Format("2006-01-02-15")
	datedPath := filepath.Join(dir, dateStr+".json")
	if err := os.WriteFile(datedPath, data, 0o644); err != nil {
		return fmt.Errorf("write dated report: %w", err)
	}

	// Write latest.json
	latestPath := filepath.Join(dir, "latest.json")
	if err := os.WriteFile(latestPath, data, 0o644); err != nil {
		return fmt.Errorf("write latest report: %w", err)
	}

	return nil
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

// collectMineWorkItems builds work items from a mine report.
// Code hotspots map to severity:high; orphaned research files map to severity:medium.
func collectMineWorkItems(r *MineReport) []mineWorkItemEmit {
	var items []mineWorkItemEmit
	if r.Code != nil {
		for _, h := range r.Code.Hotspots {
			item := mineWorkItemEmit{
				Title:       fmt.Sprintf("Reduce complexity: %s in %s (CC=%d)", h.Func, h.File, h.Complexity),
				Type:        "refactor",
				Severity:    "high",
				Source:      "athena-mine",
				Description: fmt.Sprintf("Function %s in %s has cyclomatic complexity %d with %d recent edits. Extract helpers to reduce CC below 15.", h.Func, h.File, h.Complexity, h.RecentEdits),
				Evidence:    fmt.Sprintf("complexity=%d recent_edits=%d", h.Complexity, h.RecentEdits),
			}
			item.ID = mineWorkItemID(item)
			items = append(items, item)
		}
	}
	if r.Agents != nil {
		for _, orphan := range r.Agents.OrphanedResearch {
			item := mineWorkItemEmit{
				Title:       fmt.Sprintf("Rescue orphan: %s", orphan),
				Type:        "knowledge-gap",
				Severity:    "medium",
				Source:      "athena-mine",
				Description: fmt.Sprintf("Research file %q exists in .agents/research/ but is not referenced in any learning. Extract its key insights into a learning file.", orphan),
				Evidence:    "not referenced in .agents/learnings/",
			}
			item.ID = mineWorkItemID(item)
			items = append(items, item)
		}
	}
	return items
}

// loadExistingMineIDs scans a JSONL file for unconsumed athena-mine item IDs.
// Returns an empty map with nil error when the file does not exist.
// Propagates other errors (permission denied, corrupt read, etc.).
func loadExistingMineIDs(path string) (map[string]bool, error) {
	ids := make(map[string]bool)
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ids, nil
		}
		return nil, err
	}
	if len(existing) == 0 {
		return ids, nil
	}
	for _, line := range strings.Split(string(existing), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry struct {
			SourceEpic string             `json:"source_epic"`
			Consumed   bool               `json:"consumed"`
			Items      []mineWorkItemEmit `json:"items"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil &&
			entry.SourceEpic == "athena-mine" && !entry.Consumed {
			for _, it := range entry.Items {
				if it.ID != "" {
					ids[it.ID] = true
				}
			}
		}
	}
	return ids, nil
}

// writeMineWorkItems appends one JSONL line per work item to the given path.
func writeMineWorkItems(path string, items []mineWorkItemEmit, ts string) error {
	type emitEntry struct {
		SourceEpic string             `json:"source_epic"`
		Timestamp  string             `json:"timestamp"`
		Items      []mineWorkItemEmit `json:"items"`
		Consumed   bool               `json:"consumed"`
		ConsumedBy *string            `json:"consumed_by"`
		ConsumedAt *string            `json:"consumed_at"`
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open next-work.jsonl: %w", err)
	}
	defer f.Close()

	for _, item := range items {
		entry := emitEntry{
			SourceEpic: "athena-mine",
			Timestamp:  ts,
			Items:      []mineWorkItemEmit{item},
			Consumed:   false,
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshal work item entry: %w", err)
		}
		data = append(data, '\n')
		if _, writeErr := f.Write(data); writeErr != nil {
			return writeErr
		}
	}
	return nil
}

// emitMineWorkItems translates mine findings into next-work.jsonl entries for evolve.
// Orphaned research files map to severity:medium; code hotspots map to severity:high.
// Dedup: item-level — each item gets a stable ID; only new items are emitted.
func emitMineWorkItems(cwd string, r *MineReport) error {
	items := collectMineWorkItems(r)
	if len(items) == 0 {
		return nil // nothing to emit
	}

	nextWorkPath := filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")
	if err := os.MkdirAll(filepath.Dir(nextWorkPath), 0o750); err != nil {
		return fmt.Errorf("ensure next-work dir: %w", err)
	}

	existingIDs, err := loadExistingMineIDs(nextWorkPath)
	if err != nil {
		return fmt.Errorf("load existing mine IDs: %w", err)
	}
	var newItems []mineWorkItemEmit
	for _, item := range items {
		if !existingIDs[item.ID] {
			newItems = append(newItems, item)
		}
	}
	if len(newItems) == 0 {
		return nil // all items already present
	}

	return writeMineWorkItems(nextWorkPath, newItems, r.Timestamp.UTC().Format(time.RFC3339))
}

// mineWorkItemEmit is a single work item within a next-work.jsonl entry.
type mineWorkItemEmit struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Evidence    string `json:"evidence,omitempty"`
}

// mineWorkItemID generates a stable ID from the item's identifying fields.
func mineWorkItemID(item mineWorkItemEmit) string {
	h := sha256.New()
	h.Write([]byte(item.Title))
	h.Write([]byte(item.Type))
	return hex.EncodeToString(h.Sum(nil))[:16]
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
}
