package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

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

type processInfo struct {
	PID     int
	PPID    int
	Command string
}

type cancelTarget struct {
	Kind         string
	RunID        string
	Root         string
	StatePath    string
	LeasePath    string
	WorktreePath string
	PIDs         []int
}

func runRPICancel(cmd *cobra.Command, args []string) error {
	runID := strings.TrimSpace(rpiCancelRunID)
	if !rpiCancelAll && runID == "" {
		return fmt.Errorf("specify --all or --run-id <id>")
	}

	sig, err := parseCancelSignal(rpiCancelSignal)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	procs, err := listProcesses()
	if err != nil {
		return err
	}

	targets := discoverCancelTargets(collectSearchRoots(cwd), runID, procs)
	if len(targets) == 0 {
		fmt.Println("No active runs matched cancel criteria.")
		return nil
	}

	selfPID := os.Getpid()
	var failures []string
	for _, target := range targets {
		pids := filterKillablePIDs(target.PIDs, selfPID)
		fmt.Printf("Cancel target: kind=%s run=%s signal=%s pids=%v\n", target.Kind, target.RunID, sig.String(), pids)
		if rpiCancelDryRun {
			continue
		}

		for _, pid := range pids {
			if killErr := syscall.Kill(pid, sig); killErr != nil && !errors.Is(killErr, syscall.ESRCH) {
				failures = append(failures, fmt.Sprintf("pid %d: %v", pid, killErr))
			}
		}

		if target.StatePath != "" {
			if markErr := markRunInterruptedByCancel(target); markErr != nil {
				failures = append(failures, fmt.Sprintf("run %s state update: %v", target.RunID, markErr))
			}
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("cancel completed with errors: %s", strings.Join(failures, "; "))
	}
	return nil
}

func discoverCancelTargets(roots []string, runID string, procs []processInfo) []cancelTarget {
	var targets []cancelTarget
	seen := make(map[string]struct{})
	for _, root := range roots {
		targets = append(targets, discoverRunRegistryTargets(root, runID, procs, seen)...)
		targets = append(targets, discoverSupervisorLeaseTargets(root, runID, procs, seen)...)
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Kind == targets[j].Kind {
			return targets[i].RunID < targets[j].RunID
		}
		return targets[i].Kind < targets[j].Kind
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
		statePath := filepath.Join(runsDir, entry.Name(), phasedStateFile)
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue
		}
		state, err := parsePhasedState(data)
		if err != nil || state.RunID == "" {
			continue
		}
		if runID != "" && state.RunID != runID {
			continue
		}
		key := "run:" + state.RunID
		if _, ok := seen[key]; ok {
			continue
		}
		isActive, _ := determineRunLiveness(root, state)
		if !isActive {
			continue
		}
		pids := collectRunProcessPIDs(state, procs)
		seen[key] = struct{}{}
		targets = append(targets, cancelTarget{
			Kind:         "phased",
			RunID:        state.RunID,
			Root:         root,
			StatePath:    statePath,
			WorktreePath: state.WorktreePath,
			PIDs:         pids,
		})
	}
	return targets
}

func discoverSupervisorLeaseTargets(root, runID string, procs []processInfo, seen map[string]struct{}) []cancelTarget {
	leasePath := filepath.Join(root, ".agents", "rpi", "supervisor.lock")
	data, err := os.ReadFile(leasePath)
	if err != nil {
		return nil
	}
	var meta supervisorLeaseMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	if meta.RunID == "" || meta.PID <= 0 {
		return nil
	}
	if runID != "" && meta.RunID != runID {
		return nil
	}
	if !processExists(meta.PID, procs) {
		return nil
	}

	key := "lease:" + meta.RunID
	if _, ok := seen[key]; ok {
		return nil
	}
	pids := append([]int{meta.PID}, descendantPIDs(meta.PID, procs)...)
	sort.Ints(pids)
	seen[key] = struct{}{}

	return []cancelTarget{{
		Kind:      "supervisor",
		RunID:     meta.RunID,
		Root:      root,
		LeasePath: leasePath,
		PIDs:      dedupeInts(pids),
	}}
}

func collectRunProcessPIDs(state *phasedState, procs []processInfo) []int {
	set := make(map[int]struct{})

	addWithDescendants := func(pid int) {
		if pid <= 1 || !processExists(pid, procs) {
			return
		}
		set[pid] = struct{}{}
		for _, child := range descendantPIDs(pid, procs) {
			set[child] = struct{}{}
		}
	}

	addWithDescendants(state.OrchestratorPID)

	sessionNeedle := fmt.Sprintf("ao-rpi-%s-p", state.RunID)
	for _, proc := range procs {
		cmd := proc.Command
		if strings.Contains(cmd, sessionNeedle) {
			addWithDescendants(proc.PID)
		}
		if state.WorktreePath != "" && strings.Contains(cmd, state.WorktreePath) {
			addWithDescendants(proc.PID)
		}
	}

	var pids []int
	for pid := range set {
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	return pids
}

func filterKillablePIDs(pids []int, selfPID int) []int {
	var out []int
	for _, pid := range dedupeInts(pids) {
		if pid <= 1 || pid == selfPID {
			continue
		}
		out = append(out, pid)
	}
	sort.Ints(out)
	return out
}

func parseCancelSignal(raw string) (syscall.Signal, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "", "TERM", "SIGTERM":
		return syscall.SIGTERM, nil
	case "KILL", "SIGKILL":
		return syscall.SIGKILL, nil
	case "INT", "SIGINT":
		return syscall.SIGINT, nil
	default:
		return 0, fmt.Errorf("unsupported signal %q (valid: TERM|KILL|INT)", raw)
	}
}

func listProcesses() ([]processInfo, error) {
	cmd := exec.Command("ps", "-axo", "pid=,ppid=,command=")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	var procs []processInfo
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		procs = append(procs, processInfo{
			PID:     pid,
			PPID:    ppid,
			Command: strings.Join(fields[2:], " "),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse process list: %w", err)
	}
	return procs, nil
}

func processExists(pid int, procs []processInfo) bool {
	for _, proc := range procs {
		if proc.PID == pid {
			return true
		}
	}
	return false
}

func descendantPIDs(parentPID int, procs []processInfo) []int {
	children := make(map[int][]int)
	for _, proc := range procs {
		children[proc.PPID] = append(children[proc.PPID], proc.PID)
	}

	var out []int
	queue := []int{parentPID}
	seen := map[int]struct{}{parentPID: {}}

	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		for _, child := range children[pid] {
			if _, ok := seen[child]; ok {
				continue
			}
			seen[child] = struct{}{}
			out = append(out, child)
			queue = append(queue, child)
		}
	}
	sort.Ints(out)
	return out
}

func dedupeInts(in []int) []int {
	set := make(map[int]struct{}, len(in))
	var out []int
	for _, n := range in {
		if _, ok := set[n]; ok {
			continue
		}
		set[n] = struct{}{}
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

func markRunInterruptedByCancel(target cancelTarget) error {
	if target.StatePath == "" || target.RunID == "" {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	reason := "cancelled by ao rpi cancel"

	update := func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		raw["terminal_status"] = "interrupted"
		raw["terminal_reason"] = reason
		raw["terminated_at"] = now
		updated, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			return err
		}
		updated = append(updated, '\n')
		return writePhasedStateAtomic(path, updated)
	}

	if err := update(target.StatePath); err != nil {
		return fmt.Errorf("update run state: %w", err)
	}

	flatPath := filepath.Join(target.Root, ".agents", "rpi", phasedStateFile)
	flatData, err := os.ReadFile(flatPath)
	if err != nil {
		return nil
	}
	var flatRaw map[string]interface{}
	if err := json.Unmarshal(flatData, &flatRaw); err != nil {
		return nil
	}
	if runVal, _ := flatRaw["run_id"].(string); runVal != target.RunID {
		return nil
	}
	return update(flatPath)
}
