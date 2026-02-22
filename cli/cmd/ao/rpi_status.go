package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var rpiStatusWatch bool

func init() {
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show active RPI phased runs",
		Long: `Display active and recent RPI phased runs.

Uses the run registry at .agents/rpi/runs/ as the primary source of truth.
Heartbeat files determine liveness (alive = heartbeat within last 5 minutes).
Tmux sessions are only probed for runs that lack a recent heartbeat, with a
bounded timeout to prevent blocking.

Also parses orchestration logs for phase history, durations, and verdicts.

Examples:
  ao rpi status
  ao rpi status -o json
  ao rpi status --watch`,
		RunE: runRPIStatus,
	}
	statusCmd.Flags().BoolVar(&rpiStatusWatch, "watch", false, "Poll every 5s and redraw (Ctrl-C to exit)")
	rpiCmd.AddCommand(statusCmd)
}

// --- rpiRun: log-parsed run data ---

// rpiRun represents a single orchestration run parsed from the log file.
type rpiRun struct {
	RunID      string            `json:"run_id"`
	Goal       string            `json:"goal,omitempty"`
	Phases     []rpiPhaseEntry   `json:"phases"`
	StartedAt  time.Time         `json:"started_at"`
	FinishedAt time.Time         `json:"finished_at,omitempty"`
	Duration   time.Duration     `json:"duration,omitempty"`
	Verdicts   map[string]string `json:"verdicts,omitempty"`
	Retries    map[string]int    `json:"retries,omitempty"`
	Status     string            `json:"status"` // running, completed, failed
	EpicID     string            `json:"epic_id,omitempty"`
}

// rpiPhaseEntry represents a single phase log entry within a run.
type rpiPhaseEntry struct {
	Name    string `json:"name"`
	Details string `json:"details"`
	Time    string `json:"time"`
}

// --- rpiRunInfo: state-file-based run data ---

type rpiRunInfo struct {
	RunID     string `json:"run_id"`
	Goal      string `json:"goal,omitempty"`
	Phase     int    `json:"phase"`
	PhaseName string `json:"phase_name"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"` // why a run is stale/failed (e.g. "worktree missing")
	EpicID    string `json:"epic_id,omitempty"`
	Worktree  string `json:"worktree,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	Elapsed   string `json:"elapsed,omitempty"`
	// Liveness metadata (not shown in table, used for categorisation)
	IsActive      bool      `json:"is_active"`
	LastHeartbeat time.Time `json:"last_heartbeat,omitempty"`
}

type rpiStatusOutput struct {
	Active       []rpiRunInfo         `json:"active"`
	Historical   []rpiRunInfo         `json:"historical"`
	Runs         []rpiRunInfo         `json:"runs"` // combined, kept for back-compat
	LogRuns      []rpiRun             `json:"log_runs,omitempty"`
	LiveStatuses []liveStatusSnapshot `json:"live_statuses,omitempty"`
	Count        int                  `json:"count"`
}

type liveStatusSnapshot struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// heartbeatLiveThreshold is the maximum age of a heartbeat for a run to be
// considered alive without probing tmux.
const heartbeatLiveThreshold = 5 * time.Minute

// tmuxProbeTimeout is the maximum time we will wait for a single tmux probe.
const tmuxProbeTimeout = 2 * time.Second

func runRPIStatus(cmd *cobra.Command, args []string) error {
	if rpiStatusWatch {
		return runRPIStatusWatch()
	}
	return runRPIStatusOnce()
}

func runRPIStatusOnce() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	output := buildRPIStatusOutput(cwd)
	if GetOutput() == "json" {
		return writeRPIStatusJSON(output)
	}

	return renderRPIStatusTable(cwd, output)
}

func buildRPIStatusOutput(cwd string) rpiStatusOutput {
	active, historical := discoverRPIRunsRegistryFirst(cwd)
	allRuns := append(active, historical...)

	return rpiStatusOutput{
		Active:       active,
		Historical:   historical,
		Runs:         allRuns,
		LogRuns:      discoverLogRuns(cwd),
		LiveStatuses: discoverLiveStatuses(cwd),
		Count:        len(allRuns),
	}
}

func writeRPIStatusJSON(output rpiStatusOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func renderRPIStatusTable(cwd string, output rpiStatusOutput) error {
	if len(output.Runs) == 0 && len(output.LogRuns) == 0 && len(output.LiveStatuses) == 0 {
		fmt.Println("No active RPI runs found.")
		return nil
	}

	if len(output.Active) > 0 {
		renderStateRunsSection("Active Runs", output.Active, "active", false)
	}
	if len(output.Historical) > 0 {
		renderStateRunsSection("Historical Runs", output.Historical, "historical", len(output.Active) > 0)
	}
	if len(output.LogRuns) > 0 {
		renderLogRunsSection(output.LogRuns)
	}
	if len(output.LiveStatuses) > 0 {
		renderLiveStatusesSection(cwd, output.LiveStatuses)
	}

	return nil
}

func renderStateRunsSection(title string, runs []rpiRunInfo, label string, withLeadingBlank bool) {
	if withLeadingBlank {
		fmt.Println()
	}

	// Check if any run has a reason to show the extra column.
	hasReason := false
	for _, r := range runs {
		if r.Reason != "" {
			hasReason = true
			break
		}
	}

	fmt.Println(title)
	if hasReason {
		fmt.Printf("%-14s %-26s %-14s %-12s %-20s %s\n", "RUN-ID", "GOAL", "PHASE", "STATUS", "REASON", "ELAPSED")
		fmt.Println(strings.Repeat("─", 100))
		for _, r := range runs {
			fmt.Printf("%-14s %-26s %-14s %-12s %-20s %s\n",
				r.RunID, truncateGoal(r.Goal, 24), r.PhaseName, r.Status, r.Reason, r.Elapsed)
		}
	} else {
		fmt.Printf("%-14s %-30s %-14s %-10s %s\n", "RUN-ID", "GOAL", "PHASE", "STATUS", "ELAPSED")
		fmt.Println(strings.Repeat("─", 82))
		for _, r := range runs {
			fmt.Printf("%-14s %-30s %-14s %-10s %s\n",
				r.RunID, truncateGoal(r.Goal, 28), r.PhaseName, r.Status, r.Elapsed)
		}
	}
	fmt.Printf("\n%d %s run(s) found.\n", len(runs), label)
}

func renderLogRunsSection(logRuns []rpiRun) {
	fmt.Printf("\n%-14s %-30s %-12s %-10s %-10s %s\n", "RUN-ID", "GOAL", "LAST-PHASE", "STATUS", "RETRIES", "DURATION")
	fmt.Println(strings.Repeat("─", 100))
	for _, lr := range logRuns {
		fmt.Printf("%-14s %-30s %-12s %-10s %-10d %s\n",
			lr.RunID,
			truncateGoal(lr.Goal, 28),
			lastPhaseName(lr.Phases),
			formattedLogRunStatus(lr),
			totalRetries(lr.Retries),
			formatLogRunDuration(lr.Duration),
		)
	}
	fmt.Printf("\n%d log run(s) found.\n", len(logRuns))
}

func renderLiveStatusesSection(cwd string, liveStatuses []liveStatusSnapshot) {
	fmt.Println("\nLive Status Files")
	fmt.Println(strings.Repeat("─", 100))
	for _, ls := range liveStatuses {
		path := ls.Path
		if rel, err := filepath.Rel(cwd, ls.Path); err == nil {
			path = rel
		}
		fmt.Printf("\n[%s]\n%s\n", path, strings.TrimSpace(ls.Content))
	}
}

func truncateGoal(goal string, maxLen int) string {
	if len(goal) <= maxLen {
		return goal
	}
	return goal[:maxLen-3] + "..."
}

func lastPhaseName(phases []rpiPhaseEntry) string {
	if len(phases) == 0 {
		return ""
	}
	return phases[len(phases)-1].Name
}

func totalRetries(retries map[string]int) int {
	total := 0
	for _, v := range retries {
		total += v
	}
	return total
}

func formatLogRunDuration(dur time.Duration) string {
	if dur <= 0 {
		return ""
	}
	return dur.Truncate(time.Second).String()
}

func formattedLogRunStatus(run rpiRun) string {
	verdictStr := joinVerdicts(run.Verdicts)
	if verdictStr == "" || run.Status != "completed" {
		return run.Status
	}
	return run.Status + " [" + verdictStr + "]"
}

func joinVerdicts(verdicts map[string]string) string {
	verdictStr := ""
	for k, v := range verdicts {
		if verdictStr != "" {
			verdictStr += ","
		}
		verdictStr += k + "=" + v
	}
	return verdictStr
}

// runRPIStatusWatch polls every 5s and redraws the display.
func runRPIStatusWatch() error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Draw immediately on first invocation
	clearScreen()
	if err := runRPIStatusOnce(); err != nil {
		return err
	}
	fmt.Printf("\n[watch mode — polling every 5s, Ctrl-C to exit]")

	for {
		select {
		case <-sigCh:
			fmt.Println("\nExiting watch mode.")
			return nil
		case <-ticker.C:
			clearScreen()
			if err := runRPIStatusOnce(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			fmt.Printf("\n[watch mode — polling every 5s, Ctrl-C to exit]")
		}
	}
}

// clearScreen emits ANSI escape sequences to clear the terminal and move cursor to top.
func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

// --- Log parsing ---

// logLineRegex matches both old format and new format log lines.
// Old: [timestamp] phase: details
// New: [timestamp] [runID] phase: details
var logLineRegex = regexp.MustCompile(
	`^\[([^\]]+)\]\s+(?:\[([^\]]+)\]\s+)?([^:]+):\s+(.*)$`,
)

// parseOrchestrationLog reads the orchestration log file and returns parsed runs.
// Handles both old format (no run-ID) and new format (with [runID] bracket).
// Groups entries by run-ID or start->complete blocks for old format.
func parseOrchestrationLog(logPath string) ([]rpiRun, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer f.Close() //nolint:errcheck

	state := newOrchestrationLogState()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		entry, ok := parseOrchestrationLogLine(scanner.Text())
		if !ok {
			continue
		}

		runID := state.resolveRunID(entry.RunID, entry.PhaseName)
		run := state.getOrCreateRun(runID)
		applyOrchestrationLogEntry(run, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan log: %w", err)
	}

	return state.orderedRuns(), nil
}

type orchestrationLogState struct {
	runMap           map[string]*rpiRun
	runOrder         []string
	anonymousCounter int
}

type orchestrationLogEntry struct {
	Timestamp string
	RunID     string
	PhaseName string
	Details   string
	ParsedAt  time.Time
	HasTime   bool
}

func newOrchestrationLogState() *orchestrationLogState {
	return &orchestrationLogState{
		runMap: make(map[string]*rpiRun),
	}
}

func parseOrchestrationLogLine(line string) (orchestrationLogEntry, bool) {
	matches := logLineRegex.FindStringSubmatch(line)
	if matches == nil {
		return orchestrationLogEntry{}, false
	}

	entry := orchestrationLogEntry{
		Timestamp: matches[1],
		RunID:     matches[2],
		PhaseName: strings.TrimSpace(matches[3]),
		Details:   strings.TrimSpace(matches[4]),
	}

	if parsedAt, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
		entry.ParsedAt = parsedAt
		entry.HasTime = true
	}

	return entry, true
}

func (s *orchestrationLogState) resolveRunID(runID, phaseName string) string {
	if runID != "" {
		return runID
	}
	if phaseName == "start" {
		s.anonymousCounter++
		return fmt.Sprintf("anon-%d", s.anonymousCounter)
	}
	if s.anonymousCounter == 0 {
		s.anonymousCounter = 1
	}
	return fmt.Sprintf("anon-%d", s.anonymousCounter)
}

func (s *orchestrationLogState) getOrCreateRun(runID string) *rpiRun {
	if run, exists := s.runMap[runID]; exists {
		return run
	}

	run := &rpiRun{
		RunID:    runID,
		Verdicts: make(map[string]string),
		Retries:  make(map[string]int),
		Status:   "running",
	}
	s.runMap[runID] = run
	s.runOrder = append(s.runOrder, runID)
	return run
}

func (s *orchestrationLogState) orderedRuns() []rpiRun {
	result := make([]rpiRun, 0, len(s.runOrder))
	for _, id := range s.runOrder {
		result = append(result, *s.runMap[id])
	}
	return result
}

func applyOrchestrationLogEntry(run *rpiRun, entry orchestrationLogEntry) {
	if entry.HasTime && run.StartedAt.IsZero() {
		run.StartedAt = entry.ParsedAt
	}

	run.Phases = append(run.Phases, rpiPhaseEntry{
		Name:    entry.PhaseName,
		Details: entry.Details,
		Time:    entry.Timestamp,
	})

	switch entry.PhaseName {
	case "start":
		run.Goal = extractGoalFromDetails(entry.Details)
	case "complete":
		applyCompletePhase(run, entry)
	default:
		applyNonTerminalPhase(run, entry)
	}
}

func applyCompletePhase(run *rpiRun, entry orchestrationLogEntry) {
	run.Status = "completed"
	if entry.HasTime {
		run.FinishedAt = entry.ParsedAt
		if !run.StartedAt.IsZero() {
			run.Duration = run.FinishedAt.Sub(run.StartedAt)
		}
	}
	run.EpicID = extractEpicFromDetails(entry.Details)
	extractVerdictsFromDetails(entry.Details, run.Verdicts)
}

func applyNonTerminalPhase(run *rpiRun, entry orchestrationLogEntry) {
	updateFailureStatus(run, entry.Details)
	updateRetryCount(run, entry.PhaseName, entry.Details)
	updateFinishedAtFromCompletedDuration(run, entry)
	updateInlineVerdicts(run, entry.PhaseName, entry.Details)
}

func updateFailureStatus(run *rpiRun, details string) {
	if strings.HasPrefix(details, "FAILED:") || strings.HasPrefix(details, "FATAL:") {
		run.Status = "failed"
	}
}

func updateRetryCount(run *rpiRun, phaseName, details string) {
	if strings.HasPrefix(details, "RETRY") {
		run.Retries[phaseName]++
	}
}

func updateFinishedAtFromCompletedDuration(run *rpiRun, entry orchestrationLogEntry) {
	if !strings.HasPrefix(entry.Details, "completed in ") {
		return
	}

	durStr := strings.TrimPrefix(entry.Details, "completed in ")
	if _, err := time.ParseDuration(durStr); err != nil {
		return
	}
	if entry.HasTime {
		run.FinishedAt = entry.ParsedAt
	}
}

func updateInlineVerdicts(run *rpiRun, phaseName, details string) {
	v := extractInlineVerdict(details)
	if v == "" {
		return
	}

	lphase := strings.ToLower(phaseName)
	ldetails := strings.ToLower(details)
	switch {
	case strings.Contains(lphase, "pre-mortem") || strings.Contains(ldetails, "pre-mortem verdict"):
		run.Verdicts["pre_mortem"] = v
	case strings.Contains(lphase, "vibe") || strings.Contains(ldetails, "vibe verdict"):
		run.Verdicts["vibe"] = v
	case strings.Contains(lphase, "post-mortem") || strings.Contains(ldetails, "post-mortem verdict"):
		run.Verdicts["post_mortem"] = v
	}
}

// extractGoalFromDetails extracts goal from "goal=\"...\" from=..." format.
func extractGoalFromDetails(details string) string {
	re := regexp.MustCompile(`goal="([^"]*)"`)
	m := re.FindStringSubmatch(details)
	if len(m) >= 2 {
		return m[1]
	}
	return details
}

// extractEpicFromDetails extracts epic ID from "epic=ag-xxx verdicts=..." format.
func extractEpicFromDetails(details string) string {
	re := regexp.MustCompile(`epic=(\S+)`)
	m := re.FindStringSubmatch(details)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// extractVerdictsFromDetails extracts verdicts from "verdicts=map[key:val ...]" format.
func extractVerdictsFromDetails(details string, verdicts map[string]string) {
	re := regexp.MustCompile(`verdicts=map\[([^\]]*)\]`)
	m := re.FindStringSubmatch(details)
	if len(m) < 2 {
		return
	}
	pairs := strings.Fields(m[1])
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			verdicts[parts[0]] = parts[1]
		}
	}
}

// extractInlineVerdict looks for PASS/WARN/FAIL in a details string.
func extractInlineVerdict(details string) string {
	for _, v := range []string{"PASS", "WARN", "FAIL"} {
		if strings.Contains(details, v) {
			return v
		}
	}
	return ""
}

// discoverLogRuns finds and parses orchestration logs in cwd and siblings.
func discoverLogRuns(cwd string) []rpiRun {
	var allRuns []rpiRun

	// Check current directory
	logPath := filepath.Join(cwd, ".agents", "rpi", "phased-orchestration.log")
	if runs, err := parseOrchestrationLog(logPath); err == nil {
		allRuns = append(allRuns, runs...)
	}

	// Check sibling worktree directories
	parent := filepath.Dir(cwd)
	pattern := filepath.Join(parent, "*-rpi-*", ".agents", "rpi", "phased-orchestration.log")
	matches, err := filepath.Glob(pattern)
	if err == nil {
		for _, match := range matches {
			// Skip if same as cwd log
			if match == logPath {
				continue
			}
			if runs, err := parseOrchestrationLog(match); err == nil {
				allRuns = append(allRuns, runs...)
			}
		}
	}

	return allRuns
}

func discoverLiveStatuses(cwd string) []liveStatusSnapshot {
	var snapshots []liveStatusSnapshot
	seen := make(map[string]struct{})

	add := func(path string) {
		if _, ok := seen[path]; ok {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		seen[path] = struct{}{}
		snapshots = append(snapshots, liveStatusSnapshot{
			Path:    path,
			Content: string(data),
		})
	}

	// Current directory live-status.
	add(filepath.Join(cwd, ".agents", "rpi", "live-status.md"))

	// Sibling worktree live-status files.
	parent := filepath.Dir(cwd)
	pattern := filepath.Join(parent, "*-rpi-*", ".agents", "rpi", "live-status.md")
	matches, err := filepath.Glob(pattern)
	if err == nil {
		for _, match := range matches {
			add(match)
		}
	}

	return snapshots
}

// --- Registry-first run discovery ---

// discoverRPIRunsRegistryFirst is the primary discovery path.
// It scans .agents/rpi/runs/ for all run directories, reads state and heartbeat
// files, and uses heartbeat age to separate active from historical runs.
// Tmux is only probed for runs that lack a recent heartbeat, with a bounded
// per-probe timeout.
//
// Returns (active, historical) slices.
func discoverRPIRunsRegistryFirst(cwd string) (active, historical []rpiRunInfo) {
	// Collect all search roots: cwd + sibling worktrees.
	roots := collectSearchRoots(cwd)

	seen := make(map[string]struct{})
	for _, root := range roots {
		runs := scanRegistryRuns(root)
		for _, r := range runs {
			if _, ok := seen[r.RunID]; ok {
				continue
			}
			seen[r.RunID] = struct{}{}
			if r.IsActive {
				active = append(active, r)
			} else {
				historical = append(historical, r)
			}
		}
	}
	return active, historical
}

// discoverRPIRuns is the legacy discovery function kept for backward
// compatibility with existing tests.  It returns all runs (active + historical)
// discovered via the registry-first path, falling back to the flat state file
// when the registry is empty.
func discoverRPIRuns(cwd string) []rpiRunInfo {
	active, historical := discoverRPIRunsRegistryFirst(cwd)
	all := append(active, historical...)
	if len(all) > 0 {
		return all
	}

	// Fallback: flat phased-state.json (backward compatibility for pre-registry runs)
	var fallback []rpiRunInfo
	if run, ok := loadRPIRun(cwd); ok {
		fallback = append(fallback, run)
	}
	parent := filepath.Dir(cwd)
	pattern := filepath.Join(parent, "*-rpi-*", ".agents", "rpi", "phased-state.json")
	matches, err := filepath.Glob(pattern)
	if err == nil {
		for _, match := range matches {
			wtDir := filepath.Dir(filepath.Dir(filepath.Dir(match)))
			if wtDir == cwd {
				continue
			}
			if run, ok := loadRPIRun(wtDir); ok {
				fallback = append(fallback, run)
			}
		}
	}
	return fallback
}

// collectSearchRoots returns the cwd plus any Git worktree roots attached to
// the same repository. This allows status/cleanup/cancel commands to discover
// runs created from other worktrees, not just sibling *-rpi-* directories.
// If git worktree discovery fails, we fall back to the historical sibling glob.
func collectSearchRoots(cwd string) []string {
	roots := []string{}
	seen := make(map[string]struct{})
	addRoot := func(path string) {
		if path == "" {
			return
		}
		normalized := normalizeSearchRootPath(path)
		if _, ok := seen[normalized]; ok {
			return
		}
		info, err := os.Stat(normalized)
		if err != nil || !info.IsDir() {
			return
		}
		stored := filepath.Clean(path)
		if abs, err := filepath.Abs(stored); err == nil {
			stored = filepath.Clean(abs)
		}
		seen[normalized] = struct{}{}
		roots = append(roots, stored)
	}

	addRoot(cwd)

	if discovered := discoverGitWorktreeRoots(cwd); len(discovered) > 0 {
		for _, root := range discovered {
			addRoot(root)
		}
		return roots
	}

	// Backward-compatible fallback: sibling *-rpi-* pattern.
	parent := filepath.Dir(cwd)
	pattern := filepath.Join(parent, "*-rpi-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return roots
	}
	for _, m := range matches {
		addRoot(m)
	}
	return roots
}

func normalizeSearchRootPath(path string) string {
	clean := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil && resolved != "" {
		return filepath.Clean(resolved)
	}
	if abs, err := filepath.Abs(clean); err == nil {
		return filepath.Clean(abs)
	}
	return clean
}

func discoverGitWorktreeRoots(cwd string) []string {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var roots []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		if path == "" {
			continue
		}
		roots = append(roots, path)
	}
	return roots
}

// scanRegistryRuns reads all run directories under <root>/.agents/rpi/runs/
// and returns rpiRunInfo for each valid run.
func scanRegistryRuns(root string) []rpiRunInfo {
	runsDir := filepath.Join(root, ".agents", "rpi", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		// Directory may not exist yet; fall through silently.
		return nil
	}

	var runs []rpiRunInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		statePath := filepath.Join(runsDir, runID, phasedStateFile)
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}
		state, err := parsePhasedState(data)
		if err != nil || state.RunID == "" {
			continue
		}

		// Determine liveness from heartbeat first, tmux as fallback.
		isActive, lastHB := determineRunLiveness(root, state)

		phaseName := displayPhaseName(*state)
		status := classifyRunStatus(*state, isActive)
		reason := classifyRunReason(*state, isActive)

		elapsed := ""
		if state.StartedAt != "" {
			if t, err := time.Parse(time.RFC3339, state.StartedAt); err == nil {
				elapsed = time.Since(t).Truncate(time.Second).String()
			}
		}

		runs = append(runs, rpiRunInfo{
			RunID:         state.RunID,
			Goal:          state.Goal,
			Phase:         state.Phase,
			PhaseName:     phaseName,
			Status:        status,
			Reason:        reason,
			EpicID:        state.EpicID,
			Worktree:      root,
			StartedAt:     state.StartedAt,
			Elapsed:       elapsed,
			IsActive:      isActive,
			LastHeartbeat: lastHB,
		})
	}
	return runs
}

// determineRunLiveness decides whether a run is alive.
//
// Priority:
//  0. If worktree path is set but the directory is gone, the run cannot be alive.
//  1. If heartbeat file exists and is recent (< heartbeatLiveThreshold), the run
//     is alive without any tmux probe.
//  2. If heartbeat is absent or stale, probe tmux with a bounded timeout.
//  3. If neither heartbeat nor tmux session is found, the run is historical.
//
// Returns (isActive bool, lastHeartbeat time.Time).
func determineRunLiveness(cwd string, state *phasedState) (bool, time.Time) {
	// Short-circuit: if worktree path is recorded but gone, run is dead.
	if state.WorktreePath != "" {
		if _, err := os.Stat(state.WorktreePath); err != nil {
			hb := readRunHeartbeat(cwd, state.RunID)
			return false, hb
		}
	}

	hb := readRunHeartbeat(cwd, state.RunID)
	if !hb.IsZero() && time.Since(hb) < heartbeatLiveThreshold {
		// Recent heartbeat — alive without tmux probe.
		return true, hb
	}

	// Heartbeat absent or stale: probe tmux with bounded timeout.
	if checkTmuxSessionAlive(state.RunID) {
		return true, hb
	}

	return false, hb
}

// classifyRunStatus derives a human-readable status string.
// Uses terminal metadata, worktree presence, liveness, and phase number.
func classifyRunStatus(state phasedState, isActive bool) string {
	// Terminal metadata takes precedence (written by interrupt/failure handlers).
	if state.TerminalStatus != "" {
		return state.TerminalStatus
	}
	if isActive {
		return "running"
	}
	if state.Phase >= completedPhaseNumber(state) {
		return "completed"
	}
	// Check if worktree path is set but directory is missing → stale.
	if state.WorktreePath != "" {
		if _, err := os.Stat(state.WorktreePath); err != nil {
			return "stale"
		}
	}
	return "unknown"
}

// classifyRunReason returns a human-readable reason for non-active/non-completed runs.
func classifyRunReason(state phasedState, isActive bool) string {
	if state.TerminalReason != "" {
		return state.TerminalReason
	}
	if !isActive && state.WorktreePath != "" {
		if _, err := os.Stat(state.WorktreePath); err != nil {
			return "worktree missing"
		}
	}
	return ""
}

// --- State-file based discovery (legacy, kept for backward compat) ---

func loadRPIRun(dir string) (rpiRunInfo, bool) {
	// Try registry-first: scan .agents/rpi/runs/ for the most recent run.
	runs := scanRegistryRuns(dir)
	if len(runs) > 0 {
		// Return the most recently started run.
		best := runs[0]
		for _, r := range runs[1:] {
			if r.StartedAt > best.StartedAt {
				best = r
			}
		}
		return best, true
	}

	// Fallback: flat phased-state.json for backward compatibility.
	stateFile := filepath.Join(dir, ".agents", "rpi", "phased-state.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return rpiRunInfo{}, false
	}

	var state phasedState
	if err := json.Unmarshal(data, &state); err != nil {
		return rpiRunInfo{}, false
	}

	if state.RunID == "" {
		return rpiRunInfo{}, false
	}

	phaseName := displayPhaseName(state)

	// Determine status via heartbeat + tmux session liveness.
	isActive, lastHB := determineRunLiveness(dir, &state)
	status := classifyRunStatus(state, isActive)

	elapsed := ""
	if state.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339, state.StartedAt); err == nil {
			elapsed = time.Since(t).Truncate(time.Second).String()
		}
	}

	reason := classifyRunReason(state, isActive)

	return rpiRunInfo{
		RunID:         state.RunID,
		Goal:          state.Goal,
		Phase:         state.Phase,
		PhaseName:     phaseName,
		Status:        status,
		Reason:        reason,
		EpicID:        state.EpicID,
		Worktree:      dir,
		StartedAt:     state.StartedAt,
		Elapsed:       elapsed,
		IsActive:      isActive,
		LastHeartbeat: lastHB,
	}, true
}

// determineRunStatus checks if a tmux session ao-rpi-<runID>-* exists.
// Returns "running" if a matching tmux session is alive, "completed" if the
// state file indicates all phases are done, or "unknown" otherwise.
func determineRunStatus(state phasedState) string {
	isActive, _ := determineRunLiveness("", &state)
	return classifyRunStatus(state, isActive)
}

func completedPhaseNumber(state phasedState) int {
	// Schema v1+ uses consolidated phased orchestration: 1=discovery, 2=implementation, 3=validation.
	if state.SchemaVersion >= 1 {
		return 3
	}
	// Legacy phased state used six steps.
	return 6
}

func displayPhaseName(state phasedState) string {
	if state.SchemaVersion >= 1 {
		phaseNames := map[int]string{
			1: "discovery",
			2: "implementation",
			3: "validation",
		}
		if phaseName := phaseNames[state.Phase]; phaseName != "" {
			return phaseName
		}
		return fmt.Sprintf("phase-%d", state.Phase)
	}

	// Legacy fallback (pre-consolidation).
	legacyPhaseNames := map[int]string{
		1: "research",
		2: "plan",
		3: "pre-mortem",
		4: "crank",
		5: "vibe",
		6: "post-mortem",
	}
	if phaseName := legacyPhaseNames[state.Phase]; phaseName != "" {
		return phaseName
	}
	return fmt.Sprintf("phase-%d", state.Phase)
}

// checkTmuxSessionAlive checks if any tmux session matching ao-rpi-<runID>-* exists.
// Each probe is bounded by tmuxProbeTimeout to prevent blocking indefinitely.
func checkTmuxSessionAlive(runID string) bool {
	if runID == "" {
		return false
	}
	tmuxCommand := "tmux"
	if tc, err := resolveRPIToolchainDefaults(); err == nil {
		tmuxCommand = tc.TmuxCommand
	} else {
		VerbosePrintf("Warning: could not resolve RPI toolchain for tmux probe: %v\n", err)
	}
	// Probe consolidated 3-phase session names: ao-rpi-<runID>-p<N>, N=1..3.
	for i := 1; i <= 3; i++ {
		sessionName := fmt.Sprintf("ao-rpi-%s-p%d", runID, i)
		ctx, cancel := context.WithTimeout(context.Background(), tmuxProbeTimeout)
		cmd := exec.CommandContext(ctx, tmuxCommand, "has-session", "-t", sessionName)
		err := cmd.Run()
		cancel()
		if err == nil {
			return true
		}
	}
	return false
}

// locateRunMetadata finds the phasedState for a given run ID.
// It searches the run registry across cwd and sibling directories, then falls
// back to the flat phased-state.json. This is used by resume to locate a run
// without relying on cwd heuristics alone.
func locateRunMetadata(cwd, runID string) (*phasedState, string, error) {
	roots := collectSearchRoots(cwd)
	for _, root := range roots {
		registryDir := rpiRunRegistryDir(root, runID)
		if registryDir == "" {
			continue
		}
		statePath := filepath.Join(registryDir, phasedStateFile)
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}
		state, err := parsePhasedState(data)
		if err != nil || state.RunID != runID {
			continue
		}
		return state, root, nil
	}

	// Fallback: flat phased-state.json in cwd (backward compatibility).
	flatPath := filepath.Join(cwd, ".agents", "rpi", phasedStateFile)
	data, err := os.ReadFile(flatPath)
	if err != nil {
		return nil, "", fmt.Errorf("run %s not found in registry or flat state", runID)
	}
	state, err := parsePhasedState(data)
	if err != nil {
		return nil, "", fmt.Errorf("parse flat state for run %s: %w", runID, err)
	}
	if state.RunID != runID {
		return nil, "", fmt.Errorf("run %s not found (flat state contains run %s)", runID, state.RunID)
	}
	return state, cwd, nil
}
