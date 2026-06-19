package rpi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// run_discovery.go — registry-backed RPI run discovery + liveness, migrated out
// of cmd/ao (rpi_status.go) so the next-work-proof reader and keeper commands
// (worktree.go active-run detection, context/codex proof classification) stay
// self-contained after the ao rpi command surface is removed (ADR-0009
// teardown, age-3pdt / age-uco1 layer 2).
//
// Decoupled from the engine's full phasedState struct: it reads only the run
// registry's persisted fields into the lean registryRunState below, so deleting
// the engine command files does not strand discovery. Logic is preserved from
// the cmd/ao originals; the tmux-command resolution uses ResolveToolchain with
// env/default only (the rare config-file-only tmux override degrades to
// heartbeat liveness rather than pulling the cmd/ao config layer down here).

const phasedStateFileName = "phased-state.json"

// heartbeatLiveThreshold is the maximum age of a heartbeat for a run to be
// considered live.
const heartbeatLiveThreshold = 5 * time.Minute

// tmuxProbeTimeout bounds the single `tmux ls` probe.
const tmuxProbeTimeout = 5 * time.Second

// registryRunState holds only the persisted run-registry fields discovery reads.
// json tags are copied verbatim from the engine phasedState struct so the same
// on-disk phased-state.json parses identically.
type registryRunState struct {
	SchemaVersion  int    `json:"schema_version"`
	Goal           string `json:"goal"`
	EpicID         string `json:"epic_id,omitempty"`
	TrackerMode    string `json:"tracker_mode,omitempty"`
	TrackerReason  string `json:"tracker_reason,omitempty"`
	Phase          int    `json:"phase"`
	StartedAt      string `json:"started_at"`
	WorktreePath   string `json:"worktree_path,omitempty"`
	RunID          string `json:"run_id,omitempty"`
	TerminalStatus string `json:"terminal_status,omitempty"`
	TerminalReason string `json:"terminal_reason,omitempty"`
}

// DiscoverRunsRegistryFirst returns active and historical runs discovered from
// the registry under every attached worktree root.
func DiscoverRunsRegistryFirst(cwd string) (active, historical []RPIRunInfo) {
	roots := CollectSearchRoots(cwd)

	seen := make(map[string]struct{})
	for _, root := range roots {
		runs := ScanRegistryRuns(root)
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

// DiscoverRuns returns all discovered runs (active first, then historical),
// falling back to flat phased-state.json for pre-registry runs.
func DiscoverRuns(cwd string) []RPIRunInfo {
	active, historical := DiscoverRunsRegistryFirst(cwd)
	all := make([]RPIRunInfo, 0, len(active)+len(historical))
	all = append(all, active...)
	all = append(all, historical...)
	if len(all) > 0 {
		return all
	}

	// Fallback: flat phased-state.json (backward compatibility for pre-registry runs)
	var fallback []RPIRunInfo
	if run, ok := LoadRPIRun(cwd); ok {
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
			if run, ok := LoadRPIRun(wtDir); ok {
				fallback = append(fallback, run)
			}
		}
	}
	return fallback
}

// ScanRegistryRuns reads all run directories under <root>/.agents/rpi/runs/ and
// returns an RPIRunInfo for each valid run.
func ScanRegistryRuns(root string) []RPIRunInfo {
	runsDir := filepath.Join(root, ".agents", "rpi", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil
	}

	runs := make([]RPIRunInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		statePath := filepath.Join(runsDir, runID, phasedStateFileName)
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}
		var state registryRunState
		if err := json.Unmarshal(data, &state); err != nil || state.RunID == "" {
			continue
		}

		runs = append(runs, runInfoFromState(root, state))
	}
	return runs
}

// LoadRPIRun loads the best run for a single directory: the registry's newest
// run if present, else the flat phased-state.json (legacy).
func LoadRPIRun(dir string) (RPIRunInfo, bool) {
	runs := ScanRegistryRuns(dir)
	if len(runs) > 0 {
		best := runs[0]
		for _, r := range runs[1:] {
			if r.StartedAt > best.StartedAt {
				best = r
			}
		}
		return best, true
	}

	stateFile := filepath.Join(dir, ".agents", "rpi", phasedStateFileName)
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return RPIRunInfo{}, false
	}

	var state registryRunState
	if err := json.Unmarshal(data, &state); err != nil {
		return RPIRunInfo{}, false
	}
	if state.RunID == "" {
		return RPIRunInfo{}, false
	}

	return runInfoFromState(dir, state), true
}

// runInfoFromState builds an RPIRunInfo from a parsed registry state rooted at
// the given worktree directory, computing liveness, status, reason and elapsed.
func runInfoFromState(root string, state registryRunState) RPIRunInfo {
	isActive, lastHB := DetermineRunLiveness(root, state.RunID, state.WorktreePath)

	elapsed := ""
	if state.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339, state.StartedAt); err == nil {
			elapsed = time.Since(t).Truncate(time.Second).String()
		}
	}

	return RPIRunInfo{
		RunID:         state.RunID,
		Goal:          state.Goal,
		Phase:         state.Phase,
		PhaseName:     DisplayPhaseName(state.SchemaVersion, state.Phase),
		Status:        classifyRunStatus(state, isActive),
		Reason:        classifyRunReason(state, isActive),
		EpicID:        state.EpicID,
		TrackerMode:   state.TrackerMode,
		TrackerReason: state.TrackerReason,
		Worktree:      root,
		StartedAt:     state.StartedAt,
		Elapsed:       elapsed,
		IsActive:      isActive,
		LastHeartbeat: lastHB,
	}
}

// DetermineRunLiveness decides whether a run is alive from its heartbeat, tmux
// session, and worktree presence. A vanished worktree forces not-alive.
func DetermineRunLiveness(cwd, runID, worktreePath string) (bool, time.Time) {
	if worktreePath != "" {
		if _, err := os.Stat(worktreePath); err != nil {
			hb := ReadRunHeartbeat(cwd, runID)
			return false, hb
		}
	}

	hb := ReadRunHeartbeat(cwd, runID)
	if !hb.IsZero() && time.Since(hb) < heartbeatLiveThreshold {
		return true, hb
	}

	if checkTmuxSessionAlive(runID) {
		return true, hb
	}

	return false, hb
}

func classifyRunStatus(state registryRunState, isActive bool) string {
	worktreeExists := true
	if state.WorktreePath != "" {
		if _, err := os.Stat(state.WorktreePath); err != nil {
			worktreeExists = false
		}
	}
	return ClassifyRunStatus(state.TerminalStatus, isActive, state.Phase, state.SchemaVersion, worktreeExists)
}

func classifyRunReason(state registryRunState, isActive bool) string {
	worktreeExists := true
	if state.WorktreePath != "" {
		if _, err := os.Stat(state.WorktreePath); err != nil {
			worktreeExists = false
		}
	}
	return ClassifyRunReason(state.TerminalReason, isActive, state.WorktreePath, worktreeExists)
}

// --- tmux liveness (memoized once per process) ---

var (
	rpiTmuxMu       sync.Mutex
	rpiTmuxLoaded   bool
	rpiTmuxSessions map[string]struct{}
)

// liveTmuxSessions returns the set of tmux session names, captured once per
// process. An absent tmux server or any probe error yields an empty set.
func liveTmuxSessions() map[string]struct{} {
	rpiTmuxMu.Lock()
	defer rpiTmuxMu.Unlock()
	if rpiTmuxLoaded {
		return rpiTmuxSessions
	}
	rpiTmuxSessions = probeTmuxSessions()
	rpiTmuxLoaded = true
	return rpiTmuxSessions
}

// resetTmuxSessionCache clears the memoized snapshot so the next
// liveTmuxSessions call re-probes. Only tests that swap the tmux binary or PATH
// need this; production never calls it.
func resetTmuxSessionCache() {
	rpiTmuxMu.Lock()
	defer rpiTmuxMu.Unlock()
	rpiTmuxLoaded = false
	rpiTmuxSessions = nil
}

// probeTmuxSessions runs a single `tmux ls` and parses the session names.
func probeTmuxSessions() map[string]struct{} {
	sessions := map[string]struct{}{}
	tmuxCommand := DefaultTmuxCommand
	if tc, err := ResolveToolchain(ResolveToolchainOptions{}); err == nil && tc.TmuxCommand != "" {
		tmuxCommand = tc.TmuxCommand
	}
	ctx, cancel := context.WithTimeout(context.Background(), tmuxProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, tmuxCommand, "ls", "-F", "#{session_name}").Output()
	if err != nil {
		return sessions // no tmux server or probe failure: empty set
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			sessions[name] = struct{}{}
		}
	}
	return sessions
}

// tmuxSessionAlive reports whether any ao-rpi-<runID>-p{1,2,3} session is
// present in the given session set.
func tmuxSessionAlive(runID string, sessions map[string]struct{}) bool {
	if runID == "" {
		return false
	}
	for i := 1; i <= 3; i++ {
		if _, ok := sessions[fmt.Sprintf("ao-rpi-%s-p%d", runID, i)]; ok {
			return true
		}
	}
	return false
}

// checkTmuxSessionAlive checks if any tmux session matching ao-rpi-<runID>-*
// exists.
func checkTmuxSessionAlive(runID string) bool {
	if runID == "" {
		return false
	}
	return tmuxSessionAlive(runID, liveTmuxSessions())
}
