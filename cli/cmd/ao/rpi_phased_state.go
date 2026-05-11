// practices: [agile-manifesto, dora-metrics]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
)

// --- Phase result artifacts ---

// phaseResultFileFmt is the filename pattern for per-phase result artifacts.
// Each phase writes "phase-{N}-result.json" to .agents/rpi/.
// Contract: docs/contracts/rpi-phase-result.schema.json
const phaseResultFileFmt = "phase-%d-result.json"

// phaseResult is a structured artifact written after each phase completes or fails.
// Schema: docs/contracts/rpi-phase-result.schema.json
type phaseResult struct {
	SchemaVersion   int               `json:"schema_version"`
	RunID           string            `json:"run_id"`
	Phase           int               `json:"phase"`
	PhaseName       string            `json:"phase_name"`
	Status          string            `json:"status"`
	Retries         int               `json:"retries,omitempty"`
	Error           string            `json:"error,omitempty"`
	Backend         string            `json:"backend,omitempty"`
	Artifacts       map[string]string `json:"artifacts,omitempty"`
	Verdicts        map[string]string `json:"verdicts,omitempty"`
	StartedAt       string            `json:"started_at"`
	CompletedAt     string            `json:"completed_at,omitempty"`
	DurationSeconds float64           `json:"duration_seconds,omitempty"`
}

// writePhaseResult writes a phase-result.json artifact (named phase-{N}-result.json) atomically (write to .tmp, rename).
// Uses VerbosePrintf so it stays in cmd/ao.
func writePhaseResult(cwd string, result *phaseResult) error {
	stateDir := filepath.Join(cwd, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0750); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal phase result: %w", err)
	}

	finalPath := filepath.Join(stateDir, fmt.Sprintf(phaseResultFileFmt, result.Phase))
	tmpPath := finalPath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write phase result tmp: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("rename phase result: %w", err)
	}

	VerbosePrintf("Phase result written to %s\n", finalPath)
	return nil
}

// validatePriorPhaseResult delegates to internal/rpi.ValidatePriorPhaseResult.
func validatePriorPhaseResult(cwd string, expectedPhase int) error {
	return cliRPI.ValidatePriorPhaseResult(cwd, expectedPhase)
}

// --- State persistence ---

const phasedStateFile = "phased-state.json"

// rpiRunRegistryDir delegates to internal/rpi.RPIRunRegistryDir.
func rpiRunRegistryDir(cwd, runID string) string {
	return cliRPI.RPIRunRegistryDir(cwd, runID)
}

// savePhasedState writes orchestrator state to disk atomically.
// Uses VerbosePrintf and C2 events so it stays in cmd/ao.
func savePhasedState(cwd string, state *phasedState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	data = append(data, '\n')

	roots := artifactRootsForState(cwd, state)
	for i, root := range roots {
		if writeErr := writePhasedStateData(root, state.RunID, data); writeErr != nil {
			if i == 0 {
				return writeErr
			}
			VerbosePrintf("Warning: mirror state write skipped for %s: %v\n", root, writeErr)
		}
	}

	// Mirror state to additional roots discovered via mirrorRootsForEvent
	// and emit C2 tracking events for observability.
	mirrorStateToPeers(cwd, state, data)

	return nil
}

// mirrorStateToPeers writes the serialised state to every mirror root returned
// by mirrorRootsForEvent that is NOT the primary cwd.  For each mirror root it
// emits a "state.mirrored" C2 event on success or "state.mirror.failed" on error.
// Uses VerbosePrintf and C2 events so it stays in cmd/ao.
func mirrorStateToPeers(cwd string, state *phasedState, data []byte) {
	if state == nil || state.RunID == "" {
		return
	}
	primaryClean := filepath.Clean(cwd)
	for _, mirrorRoot := range mirrorRootsForEvent(cwd, state.RunID) {
		if filepath.Clean(mirrorRoot) == primaryClean {
			continue
		}
		if writeErr := writePhasedStateData(mirrorRoot, state.RunID, data); writeErr != nil {
			VerbosePrintf("Warning: mirror state write failed for %s: %v\n", mirrorRoot, writeErr)
			if _, evErr := appendRPIC2Event(cwd, rpiC2EventInput{
				RunID: state.RunID, Source: "orchestrator",
				Type:    "state.mirror.failed",
				Message: fmt.Sprintf("mirror state write failed for %s", mirrorRoot),
				Details: map[string]any{"mirror_root": mirrorRoot, "error": writeErr.Error()},
			}); evErr != nil {
				VerbosePrintf("Warning: could not emit state.mirror.failed event: %v\n", evErr)
			}
			continue
		}
		if _, evErr := appendRPIC2Event(cwd, rpiC2EventInput{
			RunID: state.RunID, Source: "orchestrator",
			Type:    "state.mirrored",
			Message: fmt.Sprintf("state mirrored to %s", mirrorRoot),
			Details: map[string]any{"mirror_root": mirrorRoot, "file": phasedStateFile},
		}); evErr != nil {
			VerbosePrintf("Warning: could not emit state.mirrored event: %v\n", evErr)
		}
	}
}

// writePhasedStateData writes state to a root directory.
// Uses VerbosePrintf so it stays in cmd/ao.
func writePhasedStateData(root, runID string, data []byte) error {
	stateDir := filepath.Join(root, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0750); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	flatPath := filepath.Join(stateDir, phasedStateFile)
	if err := writePhasedStateAtomic(flatPath, data); err != nil {
		return fmt.Errorf("write state: %w", err)
	}

	if runID != "" {
		runDir := rpiRunRegistryDir(root, runID)
		if mkErr := os.MkdirAll(runDir, 0750); mkErr != nil {
			VerbosePrintf("Warning: create run registry dir: %v\n", mkErr)
		} else {
			registryPath := filepath.Join(runDir, phasedStateFile)
			if wErr := writePhasedStateAtomic(registryPath, data); wErr != nil {
				VerbosePrintf("Warning: write run registry state: %v\n", wErr)
			}
		}
	}

	VerbosePrintf("State saved to %s\n", flatPath)
	return nil
}

// artifactRootsForState returns persistence roots for state artifacts.
// Uses inferSupervisorRepoRoot (which shells out to git) so it stays in cmd/ao.
func artifactRootsForState(cwd string, state *phasedState) []string {
	roots := []string{cwd}
	if state == nil || state.WorktreePath == "" {
		return roots
	}

	if filepath.Clean(state.WorktreePath) != filepath.Clean(cwd) {
		return roots
	}

	if repoRoot := inferSupervisorRepoRoot(cwd, state.RunID); repoRoot != "" && repoRoot != filepath.Clean(cwd) {
		roots = append(roots, repoRoot)
	}
	return roots
}

func inferSupervisorRepoRoot(cwd, runID string) string {
	ctx, cancel := context.WithTimeout(context.Background(), worktreeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-common-dir")
	cmd.Dir = cwd
	if out, err := cmd.Output(); err == nil {
		commonDir := filepath.Clean(strings.TrimSpace(string(out)))
		if commonDir != "" {
			if !filepath.IsAbs(commonDir) {
				commonDir = filepath.Clean(filepath.Join(cwd, commonDir))
			}
			repoRoot := filepath.Dir(commonDir)
			if repoRoot != "" {
				return filepath.Clean(repoRoot)
			}
		}
	}

	if strings.TrimSpace(runID) == "" {
		return ""
	}
	suffix := "-rpi-" + runID
	base := filepath.Base(cwd)
	if !strings.HasSuffix(base, suffix) {
		return ""
	}
	repoBase := strings.TrimSuffix(base, suffix)
	if repoBase == "" {
		return ""
	}
	candidate := filepath.Join(filepath.Dir(cwd), repoBase)
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return filepath.Clean(candidate)
	}
	return ""
}

// artifactRootsForRun infers persistence roots from stored run state.
// Used by heartbeat updates, where only cwd and runID are available.
func artifactRootsForRun(cwd, runID string) []string {
	if runID == "" {
		return []string{cwd}
	}
	statePath := filepath.Join(rpiRunRegistryDir(cwd, runID), phasedStateFile)
	data, err := os.ReadFile(statePath)
	if err != nil {
		return []string{cwd}
	}
	state, err := parsePhasedState(data)
	if err != nil {
		return []string{cwd}
	}
	return artifactRootsForState(cwd, state)
}

// writePhasedStateAtomic delegates to internal/rpi.WritePhasedStateAtomic.
func writePhasedStateAtomic(path string, data []byte) error {
	return cliRPI.WritePhasedStateAtomic(path, data)
}

// loadPhasedState reads orchestrator state from disk.
// It first tries the per-run registry directory (most recent run), then falls
// back to the flat .agents/rpi/phased-state.json path for backward compatibility.
func loadPhasedState(cwd string) (*phasedState, error) {
	flatPath := filepath.Join(cwd, ".agents", "rpi", phasedStateFile)

	// Try to find the most recently modified state in any run registry directory.
	runData, runErr := cliRPI.LoadLatestRunRegistryState(cwd)
	if runErr == nil && runData != nil {
		runState, parseErr := parsePhasedState(runData)
		if parseErr == nil && runState != nil {
			// Prefer registry state only when it is newer than (or the same as) the flat file.
			flatInfo, flatStatErr := os.Stat(flatPath)
			if flatStatErr != nil {
				// Flat file does not exist — use registry state.
				return runState, nil
			}
			registryPath := filepath.Join(rpiRunRegistryDir(cwd, runState.RunID), phasedStateFile)
			registryInfo, regStatErr := os.Stat(registryPath)
			if regStatErr == nil && !registryInfo.ModTime().Before(flatInfo.ModTime()) {
				return runState, nil
			}
		}
	}

	// Fall back to flat path.
	data, err := os.ReadFile(flatPath)
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	return parsePhasedState(data)
}

// loadLatestRunRegistryState scans .agents/rpi/runs/ and returns the state
// from the most recently modified run directory, or nil if none exists.
func loadLatestRunRegistryState(cwd string) (*phasedState, error) {
	data, err := cliRPI.LoadLatestRunRegistryState(cwd)
	if err != nil {
		return nil, err
	}
	return parsePhasedState(data)
}

// parsePhasedState parses JSON bytes into a phasedState with nil-safe maps.
// Uses internal/rpi.NormalizeParsedState for backward-compatible defaults.
func parsePhasedState(data []byte) (*phasedState, error) {
	var state phasedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}
	cliRPI.NormalizeParsedState(&state.Goal, &state.Phase, &state.Cycle,
		&state.StartPhase, len(phases), &state.Verdicts, &state.Attempts)
	return &state, nil
}

// updateRunHeartbeat writes the current UTC timestamp to
// .agents/rpi/runs/<run-id>/heartbeat.txt atomically.
// Uses VerbosePrintf so it stays in cmd/ao.
func updateRunHeartbeat(cwd, runID string) {
	if runID == "" {
		return
	}
	ts := time.Now().UTC().Format(time.RFC3339Nano) + "\n"
	for _, root := range artifactRootsForRun(cwd, runID) {
		runDir := rpiRunRegistryDir(root, runID)
		if err := os.MkdirAll(runDir, 0750); err != nil {
			VerbosePrintf("Warning: create run dir for heartbeat: %v\n", err)
			continue
		}
		heartbeatPath := filepath.Join(runDir, "heartbeat.txt")
		if err := writePhasedStateAtomic(heartbeatPath, []byte(ts)); err != nil {
			VerbosePrintf("Warning: update heartbeat: %v\n", err)
		}
	}
}

// readRunHeartbeat delegates to internal/rpi.ReadRunHeartbeat.
func readRunHeartbeat(cwd, runID string) time.Time {
	return cliRPI.ReadRunHeartbeat(cwd, runID)
}

// runHeartbeatAge delegates to internal/rpi.RunHeartbeatAge.
func runHeartbeatAge(cwd, runID string) (time.Duration, bool) {
	return cliRPI.RunHeartbeatAge(cwd, runID)
}
