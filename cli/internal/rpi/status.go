package rpi

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// --- Status types ---

// RPIRun represents a single orchestration run parsed from the log file.
type RPIRun struct {
	RunID      string            `json:"run_id"`
	Goal       string            `json:"goal,omitempty"`
	Phases     []RPIPhaseEntry   `json:"phases"`
	StartedAt  time.Time         `json:"started_at"`
	FinishedAt time.Time         `json:"finished_at,omitempty"`
	Duration   time.Duration     `json:"duration,omitempty"`
	Verdicts   map[string]string `json:"verdicts,omitempty"`
	Retries    map[string]int    `json:"retries,omitempty"`
	Status     string            `json:"status"` // running, completed, failed
	EpicID     string            `json:"epic_id,omitempty"`
}

// RPIPhaseEntry represents a single phase log entry within a run.
type RPIPhaseEntry struct {
	Name    string `json:"name"`
	Details string `json:"details"`
	Time    string `json:"time"`
}

// RPIRunInfo holds state-file-based run data for display.
type RPIRunInfo struct {
	RunID         string    `json:"run_id"`
	Goal          string    `json:"goal,omitempty"`
	Phase         int       `json:"phase"`
	PhaseName     string    `json:"phase_name"`
	Status        string    `json:"status"`
	Reason        string    `json:"reason,omitempty"`
	EpicID        string    `json:"epic_id,omitempty"`
	TrackerMode   string    `json:"tracker_mode,omitempty"`
	TrackerReason string    `json:"tracker_reason,omitempty"`
	Worktree      string    `json:"worktree,omitempty"`
	StartedAt     string    `json:"started_at,omitempty"`
	Elapsed       string    `json:"elapsed,omitempty"`
	IsActive      bool      `json:"is_active"`
	LastHeartbeat time.Time `json:"last_heartbeat,omitempty"`
}

// RPIStatusOutput is the top-level status response.
type RPIStatusOutput struct {
	Active       []RPIRunInfo         `json:"active"`
	Historical   []RPIRunInfo         `json:"historical"`
	Runs         []RPIRunInfo         `json:"runs"`
	LogRuns      []RPIRun             `json:"log_runs,omitempty"`
	LiveStatuses []LiveStatusSnapshot `json:"live_statuses,omitempty"`
	Count        int                  `json:"count"`
}

// LiveStatusSnapshot holds a live-status file path and content.
type LiveStatusSnapshot struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// --- Log parsing ---

// LogLineRegex matches both old format and new format log lines.
var LogLineRegex = regexp.MustCompile(
	`^\[([^\]]+)\]\s+(?:\[([^\]]+)\]\s+)?([^:]+):\s+(.*)$`,
)

// OrchestrationLogState tracks state while parsing orchestration logs.
type OrchestrationLogState struct {
	RunMap           map[string]*RPIRun
	RunOrder         []string
	AnonymousCounter int
}

// OrchestrationLogEntry is a single parsed line from the orchestration log.
type OrchestrationLogEntry struct {
	Timestamp string
	RunID     string
	PhaseName string
	Details   string
	ParsedAt  time.Time
	HasTime   bool
}

// NewOrchestrationLogState creates a new log-parsing state tracker.
func NewOrchestrationLogState() *OrchestrationLogState {
	return &OrchestrationLogState{
		RunMap: make(map[string]*RPIRun),
	}
}

// ParseOrchestrationLogLine parses a single log line into an entry.
func ParseOrchestrationLogLine(line string) (OrchestrationLogEntry, bool) {
	matches := LogLineRegex.FindStringSubmatch(line)
	if matches == nil {
		return OrchestrationLogEntry{}, false
	}

	entry := OrchestrationLogEntry{
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

// ResolveRunID determines a run ID from the log entry, assigning anonymous IDs
// when necessary.
func (s *OrchestrationLogState) ResolveRunID(runID, phaseName string) string {
	if runID != "" {
		return runID
	}
	if phaseName == "start" {
		s.AnonymousCounter++
		return fmt.Sprintf("anon-%d", s.AnonymousCounter)
	}
	if s.AnonymousCounter == 0 {
		s.AnonymousCounter = 1
	}
	return fmt.Sprintf("anon-%d", s.AnonymousCounter)
}

// GetOrCreateRun returns the run for the given ID, creating it if needed.
func (s *OrchestrationLogState) GetOrCreateRun(runID string) *RPIRun {
	if run, exists := s.RunMap[runID]; exists {
		return run
	}

	run := &RPIRun{
		RunID:    runID,
		Verdicts: make(map[string]string),
		Retries:  make(map[string]int),
		Status:   "running",
	}
	s.RunMap[runID] = run
	s.RunOrder = append(s.RunOrder, runID)
	return run
}

// OrderedRuns returns the runs in discovery order.
func (s *OrchestrationLogState) OrderedRuns() []RPIRun {
	result := make([]RPIRun, 0, len(s.RunOrder))
	for _, id := range s.RunOrder {
		result = append(result, *s.RunMap[id])
	}
	return result
}

// ApplyOrchestrationLogEntry applies a parsed log entry to its run.
func ApplyOrchestrationLogEntry(run *RPIRun, entry OrchestrationLogEntry) {
	if entry.HasTime && run.StartedAt.IsZero() {
		run.StartedAt = entry.ParsedAt
	}

	run.Phases = append(run.Phases, RPIPhaseEntry{
		Name:    entry.PhaseName,
		Details: entry.Details,
		Time:    entry.Timestamp,
	})

	switch entry.PhaseName {
	case "start":
		run.Goal = ExtractGoalFromDetails(entry.Details)
	case "complete":
		ApplyCompletePhase(run, entry)
	default:
		ApplyNonTerminalPhase(run, entry)
	}
}

// ApplyCompletePhase handles a "complete" log entry.
func ApplyCompletePhase(run *RPIRun, entry OrchestrationLogEntry) {
	run.Status = "completed"
	if entry.HasTime {
		run.FinishedAt = entry.ParsedAt
		if !run.StartedAt.IsZero() {
			run.Duration = run.FinishedAt.Sub(run.StartedAt)
		}
	}
	run.EpicID = ExtractEpicFromDetails(entry.Details)
	ExtractVerdictsFromDetails(entry.Details, run.Verdicts)
}

// ApplyNonTerminalPhase handles non-start, non-complete log entries.
func ApplyNonTerminalPhase(run *RPIRun, entry OrchestrationLogEntry) {
	UpdateFailureStatus(run, entry.Details)
	UpdateRetryCount(run, entry.PhaseName, entry.Details)
	UpdateFinishedAtFromCompletedDuration(run, entry)
	UpdateInlineVerdicts(run, entry.PhaseName, entry.Details)
}

// UpdateFailureStatus marks a run as failed if details indicate failure.
func UpdateFailureStatus(run *RPIRun, details string) {
	if strings.HasPrefix(details, "FAILED:") || strings.HasPrefix(details, "FATAL:") {
		run.Status = "failed"
	}
}

// UpdateRetryCount increments the retry counter for a phase.
func UpdateRetryCount(run *RPIRun, phaseName, details string) {
	if strings.HasPrefix(details, "RETRY") {
		run.Retries[phaseName]++
	}
}

// UpdateFinishedAtFromCompletedDuration sets FinishedAt from a "completed in" line.
func UpdateFinishedAtFromCompletedDuration(run *RPIRun, entry OrchestrationLogEntry) {
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

// UpdateInlineVerdicts extracts verdict keywords from a log line.
func UpdateInlineVerdicts(run *RPIRun, phaseName, details string) {
	v := ExtractInlineVerdict(details)
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

// --- Extraction helpers ---

// ExtractGoalFromDetails extracts goal from "goal=\"...\" from=..." format.
func ExtractGoalFromDetails(details string) string {
	re := regexp.MustCompile(`goal="([^"]*)"`)
	m := re.FindStringSubmatch(details)
	if len(m) >= 2 {
		return m[1]
	}
	return details
}

// ExtractEpicFromDetails extracts epic ID from "epic=ag-xxx verdicts=..." format.
func ExtractEpicFromDetails(details string) string {
	re := regexp.MustCompile(`epic=(\S+)`)
	m := re.FindStringSubmatch(details)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// ExtractVerdictsFromDetails extracts verdicts from "verdicts=map[key:val ...]" format.
func ExtractVerdictsFromDetails(details string, verdicts map[string]string) {
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

// ExtractInlineVerdict looks for PASS/WARN/FAIL in a details string.
func ExtractInlineVerdict(details string) string {
	for _, v := range []string{"PASS", "WARN", "FAIL"} {
		if strings.Contains(details, v) {
			return v
		}
	}
	return ""
}

// --- Formatting helpers ---

// TruncateGoal truncates a string to maxLen, adding "..." if needed.
func TruncateGoal(goal string, maxLen int) string {
	if len(goal) <= maxLen {
		return goal
	}
	return goal[:maxLen-3] + "..."
}

// LastPhaseName returns the name of the last phase entry.
func LastPhaseName(phases []RPIPhaseEntry) string {
	if len(phases) == 0 {
		return ""
	}
	return phases[len(phases)-1].Name
}

// TotalRetries sums all retry counts across phases.
func TotalRetries(retries map[string]int) int {
	total := 0
	for _, v := range retries {
		total += v
	}
	return total
}

// FormatLogRunDuration formats a duration for display, truncated to seconds.
func FormatLogRunDuration(dur time.Duration) string {
	if dur <= 0 {
		return ""
	}
	return dur.Truncate(time.Second).String()
}

// FormattedLogRunStatus returns the status with verdict annotations.
func FormattedLogRunStatus(run RPIRun) string {
	verdictStr := JoinVerdicts(run.Verdicts)
	if verdictStr == "" || run.Status != "completed" {
		return run.Status
	}
	return run.Status + " [" + verdictStr + "]"
}

// JoinVerdicts joins a verdict map into a "key=val,key=val" string.
// Keys are sorted so output is deterministic across runs.
func JoinVerdicts(verdicts map[string]string) string {
	if len(verdicts) == 0 {
		return ""
	}
	keys := make([]string, 0, len(verdicts))
	for k := range verdicts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+verdicts[k])
	}
	return strings.Join(parts, ",")
}

// TrackerSummary returns a short tracker display string.
func TrackerSummary(trackerMode, trackerReason string) string {
	mode := strings.TrimSpace(trackerMode)
	if mode == "" {
		return "beads"
	}
	if mode != "tasklist" {
		return mode
	}
	if strings.TrimSpace(trackerReason) == "" {
		return mode
	}
	return TruncateGoal(mode+":"+trackerReason, 12)
}

// --- Filtering helpers ---

// FilterLogRunsAgainstRegistry removes log runs that already appear in the registry.
func FilterLogRunsAgainstRegistry(logRuns []RPIRun, registryRuns []RPIRunInfo) []RPIRun {
	if len(logRuns) == 0 || len(registryRuns) == 0 {
		return logRuns
	}
	registryIDs := make(map[string]struct{}, len(registryRuns))
	for _, run := range registryRuns {
		if strings.TrimSpace(run.RunID) == "" {
			continue
		}
		registryIDs[run.RunID] = struct{}{}
	}
	filtered := make([]RPIRun, 0, len(logRuns))
	for _, run := range logRuns {
		if _, ok := registryIDs[run.RunID]; ok {
			continue
		}
		filtered = append(filtered, run)
	}
	return filtered
}

// FilterLiveStatusesToActiveRuns keeps only live statuses whose paths match active run worktrees.
func FilterLiveStatusesToActiveRuns(liveStatuses []LiveStatusSnapshot, activeRuns []RPIRunInfo) []LiveStatusSnapshot {
	if len(liveStatuses) == 0 || len(activeRuns) == 0 {
		return nil
	}
	activePaths := make(map[string]struct{}, len(activeRuns))
	for _, run := range activeRuns {
		if strings.TrimSpace(run.Worktree) == "" {
			continue
		}
		activePaths[filepath.Clean(filepath.Join(run.Worktree, ".agents", "rpi", "live-status.md"))] = struct{}{}
	}
	filtered := make([]LiveStatusSnapshot, 0, len(liveStatuses))
	for _, snapshot := range liveStatuses {
		if _, ok := activePaths[filepath.Clean(snapshot.Path)]; !ok {
			continue
		}
		filtered = append(filtered, snapshot)
	}
	return filtered
}

// --- Phase classification helpers ---

// CompletedPhaseNumber returns the final phase number based on schema version.
func CompletedPhaseNumber(schemaVersion int) int {
	if schemaVersion >= 1 {
		return 3
	}
	return 6
}

// DisplayPhaseName returns a human-readable name for a phase number.
func DisplayPhaseName(schemaVersion, phase int) string {
	if schemaVersion >= 1 {
		phaseNames := map[int]string{
			1: "discovery",
			2: "implementation",
			3: "validation",
		}
		if phaseName := phaseNames[phase]; phaseName != "" {
			return phaseName
		}
		return fmt.Sprintf("phase-%d", phase)
	}

	legacyPhaseNames := map[int]string{
		1: "research",
		2: "plan",
		3: "pre-mortem",
		4: "crank",
		5: "vibe",
		6: "post-mortem",
	}
	if phaseName := legacyPhaseNames[phase]; phaseName != "" {
		return phaseName
	}
	return fmt.Sprintf("phase-%d", phase)
}

// ClassifyRunStatus derives a human-readable status from terminal metadata,
// liveness, and phase progress. The worktreeExists parameter should be true
// when worktreePath is empty (no worktree) or the directory exists.
func ClassifyRunStatus(terminalStatus string, isActive bool, phase, schemaVersion int, worktreeExists bool) string {
	if terminalStatus != "" {
		return terminalStatus
	}
	if isActive {
		return "running"
	}
	if phase >= CompletedPhaseNumber(schemaVersion) {
		return "completed"
	}
	if !worktreeExists {
		return "stale"
	}
	return "unknown"
}

// ClassifyRunReason returns a human-readable reason for non-active/non-completed runs.
func ClassifyRunReason(terminalReason string, isActive bool, worktreePath string, worktreeExists bool) string {
	if terminalReason != "" {
		return terminalReason
	}
	if !isActive && worktreePath != "" && !worktreeExists {
		return "worktree missing"
	}
	return ""
}
