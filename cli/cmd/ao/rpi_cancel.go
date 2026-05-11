// practices: [agile-manifesto, dora-metrics]
package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/spf13/cobra"
)

// parseCancelSignal wraps rpi.ParseCancelSignal.
func parseCancelSignal(raw string) (syscall.Signal, error) { return rpi.ParseCancelSignal(raw) }

// filterKillablePIDs wraps rpi.FilterKillablePIDs.
func filterKillablePIDs(pids []int, selfPID int) []int {
	return rpi.FilterKillablePIDs(pids, selfPID)
}

// dedupeInts wraps rpi.DedupeInts.
func dedupeInts(in []int) []int { return rpi.DedupeInts(in) }

// descendantPIDs wraps rpi.DescendantPIDs.
func descendantPIDs(parentPID int, procs []rpi.ProcessInfo) []int {
	return rpi.DescendantPIDs(parentPID, procs)
}

// processExists wraps rpi.ProcessExists.
func processExists(pid int, procs []rpi.ProcessInfo) bool {
	return rpi.ProcessExists(pid, procs)
}

var (
	rpiCancelRunID  string
	rpiCancelAll    bool
	rpiCancelSignal string
	rpiCancelDryRun bool
)

func init() {
	cancelCmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel in-flight RPI runs",
		Long: `Cancel active RPI orchestration runs via a CLI kill switch.

By default this sends SIGTERM to the orchestrator PID (and its descendants)
for matching active runs discovered in the run registry and supervisor lease.
Expired/corrupted lease metadata is treated as stale and ignored.

Examples:
  ao rpi cancel --all
  ao rpi cancel --run-id 760fc86f0c0f
  ao rpi cancel --all --signal KILL`,
		RunE: runRPICancel,
	}
	cancelCmd.Flags().StringVar(&rpiCancelRunID, "run-id", "", "Cancel one active run by run ID")
	cancelCmd.Flags().BoolVar(&rpiCancelAll, "all", false, "Cancel all active runs discovered under current/sibling roots")
	cancelCmd.Flags().StringVar(&rpiCancelSignal, "signal", "TERM", "Signal to send: TERM|KILL|INT")
	cancelCmd.Flags().BoolVar(&rpiCancelDryRun, "dry-run", false, "Show what would be cancelled without sending signals")
	rpiCmd.AddCommand(cancelCmd)
}

// processInfo is a thin alias for rpi.ProcessInfo used by cancel internals.
type processInfo = rpi.ProcessInfo

// cancelTarget is a thin alias for rpi.CancelTarget used by cancel internals.
type cancelTarget = rpi.CancelTarget

func runRPICancel(cmd *cobra.Command, args []string) error {
	runID := strings.TrimSpace(rpiCancelRunID)
	if !rpiCancelAll && runID == "" {
		return fmt.Errorf("specify --all or --run-id <id>")
	}

	sig, err := parseCancelSignal(rpiCancelSignal)
	if err != nil {
		return err
	}

	targets, err := resolveCancelTargets()
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		fmt.Println("No active runs matched cancel criteria.")
		return nil
	}

	failures := executeCancelTargets(targets, sig)
	if len(failures) > 0 {
		return fmt.Errorf("cancel completed with errors: %s", strings.Join(failures, "; "))
	}
	return nil
}

func resolveCancelTargets() ([]cancelTarget, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	procs, err := rpi.ListProcesses()
	if err != nil {
		return nil, err
	}
	runID := strings.TrimSpace(rpiCancelRunID)
	return discoverCancelTargets(collectSearchRoots(cwd), runID, procs), nil
}

func executeCancelTargets(targets []cancelTarget, sig syscall.Signal) []string {
	selfPID := os.Getpid()
	var failures []string
	for _, target := range targets {
		failures = append(failures, cancelOneTarget(target, sig, selfPID)...)
	}
	return failures
}

func cancelOneTarget(target cancelTarget, sig syscall.Signal, selfPID int) []string {
	pids := filterKillablePIDs(target.PIDs, selfPID)
	fmt.Printf("Cancel target: kind=%s run=%s signal=%s pids=%v\n", target.Kind, target.RunID, sig.String(), pids)
	if rpiCancelDryRun {
		return nil
	}

	var failures []string
	for _, pid := range pids {
		if killErr := sendSignal(pid, sig); killErr != nil {
			failures = append(failures, fmt.Sprintf("pid %d: %v", pid, killErr))
		}
	}

	if target.StatePath != "" {
		if markErr := markRunInterruptedByCancel(target); markErr != nil {
			failures = append(failures, fmt.Sprintf("run %s state update: %v", target.RunID, markErr))
		}
	}
	return failures
}

func discoverCancelTargets(roots []string, runID string, procs []processInfo) []cancelTarget {
	var targets []cancelTarget
	seen := make(map[string]struct{})
	for _, root := range roots {
		targets = append(targets, discoverRunRegistryTargets(root, runID, procs, seen)...)
		targets = append(targets, discoverSupervisorLeaseTargets(root, runID, procs, seen)...)
	}
	slices.SortFunc(targets, func(a, b cancelTarget) int {
		if c := cmp.Compare(a.Kind, b.Kind); c != 0 {
			return c
		}
		return cmp.Compare(a.RunID, b.RunID)
	})
	return targets
}

func discoverRunRegistryTargets(root, runID string, procs []processInfo, seen map[string]struct{}) []cancelTarget {
	runsDir := filepath.Join(root, ".agents", "rpi", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil
	}

	var targets []cancelTarget
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if t, ok := tryRunRegistryEntry(root, runsDir, entry.Name(), runID, procs, seen); ok {
			targets = append(targets, t)
		}
	}
	return targets
}

func tryRunRegistryEntry(root, runsDir, name, runID string, procs []processInfo, seen map[string]struct{}) (cancelTarget, bool) {
	statePath := filepath.Join(runsDir, name, phasedStateFile)
	data, err := os.ReadFile(statePath)
	if err != nil {
		return cancelTarget{}, false
	}
	state, err := parsePhasedState(data)
	if err != nil || state.RunID == "" {
		return cancelTarget{}, false
	}
	if runID != "" && state.RunID != runID {
		return cancelTarget{}, false
	}
	key := "run:" + state.RunID
	if _, ok := seen[key]; ok {
		return cancelTarget{}, false
	}
	isActive, _ := determineRunLiveness(root, state)
	if !isActive {
		return cancelTarget{}, false
	}
	pids := collectRunProcessPIDs(state, procs)
	seen[key] = struct{}{}
	return cancelTarget{
		Kind:         "phased",
		RunID:        state.RunID,
		Root:         root,
		StatePath:    statePath,
		WorktreePath: state.WorktreePath,
		PIDs:         pids,
	}, true
}

func discoverSupervisorLeaseTargets(root, runID string, procs []processInfo, seen map[string]struct{}) []cancelTarget {
	leasePath := filepath.Join(root, ".agents", "rpi", "supervisor.lock")
	meta, ok := loadActiveSupervisorLease(leasePath, runID, procs)
	if !ok {
		return nil
	}

	key := "lease:" + meta.RunID
	if _, ok := seen[key]; ok {
		return nil
	}
	pids := append([]int{meta.PID}, rpi.DescendantPIDs(meta.PID, procs)...)
	seen[key] = struct{}{}

	return []cancelTarget{{
		Kind:      "supervisor",
		RunID:     meta.RunID,
		Root:      root,
		LeasePath: leasePath,
		PIDs:      rpi.DedupeInts(pids),
	}}
}

func loadActiveSupervisorLease(leasePath, runID string, procs []processInfo) (supervisorLeaseMetadata, bool) {
	data, err := os.ReadFile(leasePath)
	if err != nil {
		return supervisorLeaseMetadata{}, false
	}
	var meta supervisorLeaseMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return supervisorLeaseMetadata{}, false
	}
	if meta.RunID == "" || meta.PID <= 0 {
		return supervisorLeaseMetadata{}, false
	}
	if runID != "" && meta.RunID != runID {
		return supervisorLeaseMetadata{}, false
	}
	if rpi.SupervisorLeaseExpired(meta, time.Now().UTC()) {
		return supervisorLeaseMetadata{}, false
	}
	if !rpi.ProcessExists(meta.PID, procs) {
		return supervisorLeaseMetadata{}, false
	}
	return meta, true
}

func collectRunProcessPIDs(state *phasedState, procs []processInfo) []int {
	return rpi.CollectRunProcessPIDs(state.OrchestratorPID, state.RunID, state.WorktreePath, procs)
}

// supervisorLeaseMetadataExpired delegates to rpi.SupervisorLeaseExpired.
func supervisorLeaseMetadataExpired(meta supervisorLeaseMetadata, now time.Time) bool {
	return rpi.SupervisorLeaseExpired(meta, now)
}

func markRunInterruptedByCancel(target cancelTarget) error {
	if target.StatePath == "" || target.RunID == "" {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	reason := "cancelled by ao rpi cancel"

	if err := rpi.PatchStateWithCancelFields(target.StatePath, reason, now); err != nil {
		return fmt.Errorf("update run state: %w", err)
	}

	return maybeCancelFlatState(target, reason, now)
}

func maybeCancelFlatState(target cancelTarget, reason, now string) error {
	flatPath := filepath.Join(target.Root, ".agents", "rpi", phasedStateFile)
	flatData, err := os.ReadFile(flatPath)
	if err != nil {
		return nil
	}
	var flatRaw map[string]any
	if err := json.Unmarshal(flatData, &flatRaw); err != nil {
		return nil
	}
	if runVal, _ := flatRaw["run_id"].(string); runVal != target.RunID {
		return nil
	}
	return rpi.PatchStateWithCancelFields(flatPath, reason, now)
}
