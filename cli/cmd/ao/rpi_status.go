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
	"strings"
	"syscall"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/spf13/cobra"
)

var rpiStatusWatch bool
var rpiStatusDaemon bool
var rpiStatusDaemonURL string
var rpiStatusDaemonFallback bool

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
  ao rpi status --json
  ao rpi status --watch`,
		RunE: runRPIStatus,
	}
	statusCmd.Flags().BoolVar(&rpiStatusWatch, "watch", false, "Poll every 5s and redraw (Ctrl-C to exit)")
	statusCmd.Flags().BoolVar(&rpiStatusDaemon, "daemon", false, "Read RPI status from agentopsd")
	statusCmd.Flags().StringVar(&rpiStatusDaemonURL, "daemon-url", "", "Daemon base URL (defaults to activation file)")
	statusCmd.Flags().BoolVar(&rpiStatusDaemonFallback, "daemon-fallback", true, "Fall back to local RPI registry when daemon status is unavailable")
	rpiCmd.AddCommand(statusCmd)
}

// Type aliases delegate to internal/rpi.
type rpiRun = cliRPI.RPIRun
type rpiPhaseEntry = cliRPI.RPIPhaseEntry
type rpiRunInfo = cliRPI.RPIRunInfo
type rpiStatusOutput = cliRPI.RPIStatusOutput
type liveStatusSnapshot = cliRPI.LiveStatusSnapshot

// heartbeatLiveThreshold is the maximum age of a heartbeat for a run to be
// considered alive without probing tmux.
const heartbeatLiveThreshold = 5 * time.Minute

// tmuxProbeTimeout is the maximum time we will wait for a single tmux probe.
const tmuxProbeTimeout = 2 * time.Second

// rpiStatusMaxSiblingFiles bounds expensive sibling artifact scans.
const rpiStatusMaxSiblingFiles = 24

// rpiStatusMaxLogFileBytes bounds orchestration log parsing during status.
const rpiStatusMaxLogFileBytes int64 = 2 * 1024 * 1024

// rpiStatusMaxLiveStatusBytes bounds live-status markdown reads during status.
const rpiStatusMaxLiveStatusBytes int64 = 256 * 1024

func runRPIStatus(cmd *cobra.Command, args []string) error {
	if rpiStatusWatch {
		return runRPIStatusWatch(cobraContext(cmd))
	}
	return runRPIStatusOnceContext(cobraContext(cmd))
}

func runRPIStatusOnce() error {
	return runRPIStatusOnceContext(context.Background())
}

func runRPIStatusOnceContext(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	output, err := buildRPIStatusOutputForMode(ctx, cwd, rpiStatusDaemonModeOptions{
		Enabled:  rpiStatusDaemon,
		URL:      rpiStatusDaemonURL,
		Fallback: rpiStatusDaemonFallback,
	})
	if err != nil {
		return err
	}
	if GetOutput() == "json" {
		return writeRPIStatusJSON(output)
	}

	return renderRPIStatusTable(cwd, output)
}

type rpiStatusDaemonModeOptions struct {
	Enabled  bool
	URL      string
	Fallback bool
}

func buildRPIStatusOutputForMode(ctx context.Context, cwd string, opts rpiStatusDaemonModeOptions) (rpiStatusOutput, error) {
	if !opts.Enabled {
		return buildRPIStatusOutput(cwd), nil
	}
	output, err := buildDaemonRPIStatusOutput(ctx, cwd, opts.URL)
	if err == nil {
		return output, nil
	}
	if !opts.Fallback {
		return rpiStatusOutput{}, err
	}
	VerbosePrintf("daemon RPI status unavailable; falling back to registry: %v\n", err)
	return buildRPIStatusOutput(cwd), nil
}

func buildRPIStatusOutput(cwd string) rpiStatusOutput {
	active, historical := discoverRPIRunsRegistryFirst(cwd)
	allRuns := make([]rpiRunInfo, 0, len(active)+len(historical))
	allRuns = append(allRuns, active...)
	allRuns = append(allRuns, historical...)
	logRuns := filterLogRunsAgainstRegistry(discoverLogRuns(cwd), allRuns)
	liveStatuses := filterLiveStatusesToActiveRuns(discoverLiveStatuses(cwd), active)

	return rpiStatusOutput{
		Active:       active,
		Historical:   historical,
		Runs:         allRuns,
		LogRuns:      logRuns,
		LiveStatuses: liveStatuses,
		Count:        len(allRuns),
	}
}

func buildDaemonRPIStatusOutput(ctx context.Context, cwd, explicitURL string) (rpiStatusOutput, error) {
	baseURL, err := resolveDaemonURL(cwd, explicitURL)
	if err != nil {
		return rpiStatusOutput{}, err
	}
	status, err := fetchDaemonStatus(ctx, baseURL)
	if err != nil {
		return rpiStatusOutput{}, err
	}
	return buildRPIStatusOutputFromDaemon(status), nil
}

func buildRPIStatusOutputFromDaemon(status daemonpkg.ReadOnlyStatusResponse) rpiStatusOutput {
	var active []rpiRunInfo
	var historical []rpiRunInfo
	for _, job := range status.Queue.Jobs {
		run, ok := daemonRPIJobToRunInfo(job)
		if !ok {
			continue
		}
		if run.IsActive {
			active = append(active, run)
		} else {
			historical = append(historical, run)
		}
	}
	allRuns := make([]rpiRunInfo, 0, len(active)+len(historical))
	allRuns = append(allRuns, active...)
	allRuns = append(allRuns, historical...)
	return rpiStatusOutput{
		Active:     active,
		Historical: historical,
		Runs:       allRuns,
		Count:      len(allRuns),
	}
}

func daemonRPIJobToRunInfo(job daemonpkg.QueueJobState) (rpiRunInfo, bool) {
	var runID, goal, epicID, phaseName, startedAt string
	var phase int
	switch job.JobType {
	case daemonpkg.JobTypeRPIRun:
		spec, err := daemonpkg.RPIRunJobSpecFromPayload(job.Payload)
		if err != nil {
			return rpiRunInfo{}, false
		}
		runID = spec.RunID
		goal = spec.Goal
		epicID = spec.EpicID
		phase = firstDaemonRPIPhase(job.Artifacts, spec.StartPhase)
		phaseName = daemonpkg.RPIPhaseName(phase)
	case daemonpkg.JobTypeRPIPhase:
		spec, err := daemonpkg.RPIPhaseJobSpecFromPayload(job.Payload)
		if err != nil {
			return rpiRunInfo{}, false
		}
		runID = spec.RunID
		goal = spec.Goal
		epicID = spec.EpicID
		phase = firstDaemonRPIPhase(job.Artifacts, spec.Phase)
		phaseName = daemonpkg.RPIPhaseName(phase)
	default:
		return rpiRunInfo{}, false
	}
	if phaseName == "" {
		phaseName = fmt.Sprintf("phase-%d", phase)
	}
	startedAt = firstNonEmpty(job.CreatedAt, job.UpdatedAt)
	status := string(job.Status)
	reason := daemonRPIJobReason(job)
	isActive := !daemonRPIJobTerminal(job.Status)
	return rpiRunInfo{
		RunID:     runID,
		Goal:      goal,
		Phase:     phase,
		PhaseName: phaseName,
		Status:    status,
		Reason:    reason,
		EpicID:    epicID,
		Worktree:  "agentopsd",
		StartedAt: startedAt,
		Elapsed:   daemonRPIElapsed(startedAt),
		IsActive:  isActive,
	}, true
}

func firstDaemonRPIPhase(artifacts map[string]string, fallback int) int {
	if artifacts != nil {
		if raw := strings.TrimSpace(artifacts["active_phase"]); raw != "" {
			var phase int
			if _, err := fmt.Sscanf(raw, "%d", &phase); err == nil && phase > 0 {
				return phase
			}
		}
	}
	return fallback
}

func daemonRPIJobReason(job daemonpkg.QueueJobState) string {
	if job.Failure != nil {
		return firstNonEmpty(job.Failure.Message, string(job.Failure.Code))
	}
	if job.Status == daemonpkg.JobStatusRetryWaiting {
		return "lease expired; waiting for retry"
	}
	return ""
}

func daemonRPIElapsed(startedAt string) string {
	if startedAt == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339Nano, startedAt); err == nil {
		return time.Since(t).Truncate(time.Second).String()
	}
	if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
		return time.Since(t).Truncate(time.Second).String()
	}
	return ""
}

func daemonRPIJobTerminal(status daemonpkg.JobStatus) bool {
	switch status {
	case daemonpkg.JobStatusCompleted, daemonpkg.JobStatusFailed, daemonpkg.JobStatusCancelled:
		return true
	default:
		return false
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
	hasTracker := false
	for _, r := range runs {
		if r.Reason != "" {
			hasReason = true
		}
		if r.TrackerMode != "" && r.TrackerMode != "beads" {
			hasTracker = true
		}
	}

	fmt.Println(title)
	switch {
	case hasReason && hasTracker:
		fmt.Printf("%-14s %-22s %-14s %-12s %-10s %-18s %s\n", "RUN-ID", "GOAL", "PHASE", "STATUS", "TRACKER", "REASON", "ELAPSED")
		fmt.Println(strings.Repeat("─", 112))
		for _, r := range runs {
			fmt.Printf("%-14s %-22s %-14s %-12s %-10s %-18s %s\n",
				r.RunID, truncateGoal(r.Goal, 20), r.PhaseName, r.Status, trackerSummary(r), truncateGoal(r.Reason, 18), r.Elapsed)
		}
	case hasReason:
		fmt.Printf("%-14s %-26s %-14s %-12s %-20s %s\n", "RUN-ID", "GOAL", "PHASE", "STATUS", "REASON", "ELAPSED")
		fmt.Println(strings.Repeat("─", 100))
		for _, r := range runs {
			fmt.Printf("%-14s %-26s %-14s %-12s %-20s %s\n",
				r.RunID, truncateGoal(r.Goal, 24), r.PhaseName, r.Status, r.Reason, r.Elapsed)
		}
	case hasTracker:
		fmt.Printf("%-14s %-24s %-14s %-12s %-12s %s\n", "RUN-ID", "GOAL", "PHASE", "STATUS", "TRACKER", "ELAPSED")
		fmt.Println(strings.Repeat("─", 96))
		for _, r := range runs {
			fmt.Printf("%-14s %-24s %-14s %-12s %-12s %s\n",
				r.RunID, truncateGoal(r.Goal, 22), r.PhaseName, r.Status, trackerSummary(r), r.Elapsed)
		}
	default:
		fmt.Printf("%-14s %-30s %-14s %-10s %s\n", "RUN-ID", "GOAL", "PHASE", "STATUS", "ELAPSED")
		fmt.Println(strings.Repeat("─", 82))
		for _, r := range runs {
			fmt.Printf("%-14s %-30s %-14s %-10s %s\n",
				r.RunID, truncateGoal(r.Goal, 28), r.PhaseName, r.Status, r.Elapsed)
		}
	}
	fmt.Printf("\n%d %s run(s) found.\n", len(runs), label)
}

func trackerSummary(run rpiRunInfo) string {
	return cliRPI.TrackerSummary(run.TrackerMode, run.TrackerReason)
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

// --- Thin wrappers delegating to internal/rpi ---

func truncateGoal(goal string, maxLen int) string {
	return cliRPI.TruncateGoal(goal, maxLen)
}

func lastPhaseName(phases []rpiPhaseEntry) string {
	return cliRPI.LastPhaseName(phases)
}

func totalRetries(retries map[string]int) int {
	return cliRPI.TotalRetries(retries)
}

func formatLogRunDuration(dur time.Duration) string {
	return cliRPI.FormatLogRunDuration(dur)
}

func formattedLogRunStatus(run rpiRun) string {
	return cliRPI.FormattedLogRunStatus(run)
}

func joinVerdicts(verdicts map[string]string) string {
	return cliRPI.JoinVerdicts(verdicts)
}

// --- Watch mode ---

// runRPIStatusWatch polls every 5s and redraws the display.
func runRPIStatusWatch(ctx context.Context) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		signal.Stop(sigCh)
		close(sigCh)
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Draw immediately on first invocation
	clearScreen()
	if err := runRPIStatusOnceContext(ctx); err != nil {
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
			if err := runRPIStatusOnceContext(ctx); err != nil {
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

// --- Log parsing type aliases and thin wrappers ---

// Type aliases for log-parsing state machine (used by tests in this package).
type orchestrationLogState = cliRPI.OrchestrationLogState
type orchestrationLogEntry = cliRPI.OrchestrationLogEntry

// logLineRegex delegates to internal/rpi.
var logLineRegex = cliRPI.LogLineRegex

func newOrchestrationLogState() *cliRPI.OrchestrationLogState {
	return cliRPI.NewOrchestrationLogState()
}

func parseOrchestrationLogLine(line string) (cliRPI.OrchestrationLogEntry, bool) {
	return cliRPI.ParseOrchestrationLogLine(line)
}

func applyOrchestrationLogEntry(run *rpiRun, entry cliRPI.OrchestrationLogEntry) {
	cliRPI.ApplyOrchestrationLogEntry(run, entry)
}

func extractGoalFromDetails(details string) string {
	return cliRPI.ExtractGoalFromDetails(details)
}

func extractEpicFromDetails(details string) string {
	return cliRPI.ExtractEpicFromDetails(details)
}

func extractVerdictsFromDetails(details string, verdicts map[string]string) {
	cliRPI.ExtractVerdictsFromDetails(details, verdicts)
}

func extractInlineVerdict(details string) string {
	return cliRPI.ExtractInlineVerdict(details)
}

func updateFailureStatus(run *rpiRun, details string) {
	cliRPI.UpdateFailureStatus(run, details)
}

func updateRetryCount(run *rpiRun, phaseName, details string) {
	cliRPI.UpdateRetryCount(run, phaseName, details)
}

func updateFinishedAtFromCompletedDuration(run *rpiRun, entry cliRPI.OrchestrationLogEntry) {
	cliRPI.UpdateFinishedAtFromCompletedDuration(run, entry)
}

func updateInlineVerdicts(run *rpiRun, phaseName, details string) {
	cliRPI.UpdateInlineVerdicts(run, phaseName, details)
}

func applyCompletePhase(run *rpiRun, entry cliRPI.OrchestrationLogEntry) {
	cliRPI.ApplyCompletePhase(run, entry)
}

func applyNonTerminalPhase(run *rpiRun, entry cliRPI.OrchestrationLogEntry) {
	cliRPI.ApplyNonTerminalPhase(run, entry)
}

// parseOrchestrationLog reads the orchestration log file and returns parsed runs.
func parseOrchestrationLog(logPath string) ([]rpiRun, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer func() { _ = f.Close() }() //nolint:errcheck

	state := cliRPI.NewOrchestrationLogState()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		entry, ok := cliRPI.ParseOrchestrationLogLine(scanner.Text())
		if !ok {
			continue
		}

		runID := state.ResolveRunID(entry.RunID, entry.PhaseName)
		run := state.GetOrCreateRun(runID)
		cliRPI.ApplyOrchestrationLogEntry(run, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan log: %w", err)
	}

	return state.OrderedRuns(), nil
}

// discoverLogRuns finds and parses orchestration logs in cwd and siblings.
func discoverLogRuns(cwd string) []rpiRun {
	var allRuns []rpiRun

	// Check current directory
	logPath := filepath.Join(cwd, ".agents", "rpi", "phased-orchestration.log")
	if runs, err := parseOrchestrationLogBounded(logPath); err == nil {
		allRuns = append(allRuns, runs...)
	}

	// Check sibling worktree directories
	parent := filepath.Dir(cwd)
	pattern := filepath.Join(parent, "*-rpi-*", ".agents", "rpi", "phased-orchestration.log")
	matches, err := filepath.Glob(pattern)
	if err == nil {
		if len(matches) > rpiStatusMaxSiblingFiles {
			matches = matches[:rpiStatusMaxSiblingFiles]
		}
		for _, match := range matches {
			// Skip if same as cwd log
			if match == logPath {
				continue
			}
			if runs, err := parseOrchestrationLogBounded(match); err == nil {
				allRuns = append(allRuns, runs...)
			}
		}
	}

	return allRuns
}

func parseOrchestrationLogBounded(logPath string) ([]rpiRun, error) {
	info, err := os.Stat(logPath)
	if err != nil {
		return nil, err
	}
	if info.Size() > rpiStatusMaxLogFileBytes {
		return nil, fmt.Errorf("skip oversized orchestration log %s (%d bytes)", logPath, info.Size())
	}
	return parseOrchestrationLog(logPath)
}

func filterLogRunsAgainstRegistry(logRuns []rpiRun, registryRuns []rpiRunInfo) []rpiRun {
	return cliRPI.FilterLogRunsAgainstRegistry(logRuns, registryRuns)
}

func filterLiveStatusesToActiveRuns(liveStatuses []liveStatusSnapshot, activeRuns []rpiRunInfo) []liveStatusSnapshot {
	return cliRPI.FilterLiveStatusesToActiveRuns(liveStatuses, activeRuns)
}

func discoverLiveStatuses(cwd string) []liveStatusSnapshot {
	var snapshots []liveStatusSnapshot
	seen := make(map[string]struct{})

	add := func(path string) {
		if len(snapshots) >= rpiStatusMaxSiblingFiles+1 {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() > rpiStatusMaxLiveStatusBytes {
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
		if len(matches) > rpiStatusMaxSiblingFiles {
			matches = matches[:rpiStatusMaxSiblingFiles]
		}
		for _, match := range matches {
			add(match)
		}
	}

	return snapshots
}

// --- Registry-first run discovery ---

// discoverRPIRunsRegistryFirst is the primary discovery path.
func discoverRPIRunsRegistryFirst(cwd string) (active, historical []rpiRunInfo) {
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
// compatibility with existing tests.
func discoverRPIRuns(cwd string) []rpiRunInfo {
	active, historical := discoverRPIRunsRegistryFirst(cwd)
	all := make([]rpiRunInfo, 0, len(active)+len(historical))
	all = append(all, active...)
	all = append(all, historical...)
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

// tryAddSearchRoot normalizes and validates a path, then appends it to roots
// if it is a valid, unseen directory.
func tryAddSearchRoot(path string, seen map[string]struct{}, roots *[]string) {
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
	*roots = append(*roots, stored)
}

// collectSearchRoots returns the cwd plus any Git worktree roots attached to
// the same repository.
func collectSearchRoots(cwd string) []string {
	roots := []string{}
	seen := make(map[string]struct{})

	tryAddSearchRoot(cwd, seen, &roots)

	if discovered := discoverGitWorktreeRoots(cwd); len(discovered) > 0 {
		for _, root := range discovered {
			tryAddSearchRoot(root, seen, &roots)
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
		tryAddSearchRoot(m, seen, &roots)
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
		return nil
	}

	runs := make([]rpiRunInfo, 0, len(entries))
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
			TrackerMode:   state.TrackerMode,
			TrackerReason: state.TrackerReason,
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
func determineRunLiveness(cwd string, state *phasedState) (bool, time.Time) {
	if state.WorktreePath != "" {
		if _, err := os.Stat(state.WorktreePath); err != nil {
			hb := readRunHeartbeat(cwd, state.RunID)
			return false, hb
		}
	}

	hb := readRunHeartbeat(cwd, state.RunID)
	if !hb.IsZero() && time.Since(hb) < heartbeatLiveThreshold {
		return true, hb
	}

	if checkTmuxSessionAlive(state.RunID) {
		return true, hb
	}

	return false, hb
}

// classifyRunStatus derives a human-readable status string.
func classifyRunStatus(state phasedState, isActive bool) string {
	worktreeExists := true
	if state.WorktreePath != "" {
		if _, err := os.Stat(state.WorktreePath); err != nil {
			worktreeExists = false
		}
	}
	return cliRPI.ClassifyRunStatus(state.TerminalStatus, isActive, state.Phase, state.SchemaVersion, worktreeExists)
}

// classifyRunReason returns a human-readable reason for non-active/non-completed runs.
func classifyRunReason(state phasedState, isActive bool) string {
	worktreeExists := true
	if state.WorktreePath != "" {
		if _, err := os.Stat(state.WorktreePath); err != nil {
			worktreeExists = false
		}
	}
	return cliRPI.ClassifyRunReason(state.TerminalReason, isActive, state.WorktreePath, worktreeExists)
}

// --- State-file based discovery (legacy, kept for backward compat) ---

func loadRPIRun(dir string) (rpiRunInfo, bool) {
	runs := scanRegistryRuns(dir)
	if len(runs) > 0 {
		best := runs[0]
		for _, r := range runs[1:] {
			if r.StartedAt > best.StartedAt {
				best = r
			}
		}
		return best, true
	}

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
		TrackerMode:   state.TrackerMode,
		TrackerReason: state.TrackerReason,
		Worktree:      dir,
		StartedAt:     state.StartedAt,
		Elapsed:       elapsed,
		IsActive:      isActive,
		LastHeartbeat: lastHB,
	}, true
}

// determineRunStatus checks if a tmux session ao-rpi-<runID>-* exists.
func determineRunStatus(state phasedState) string {
	isActive, _ := determineRunLiveness("", &state)
	return classifyRunStatus(state, isActive)
}

func completedPhaseNumber(state phasedState) int {
	return cliRPI.CompletedPhaseNumber(state.SchemaVersion)
}

func displayPhaseName(state phasedState) string {
	return cliRPI.DisplayPhaseName(state.SchemaVersion, state.Phase)
}

// checkTmuxSessionAlive checks if any tmux session matching ao-rpi-<runID>-* exists.
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
