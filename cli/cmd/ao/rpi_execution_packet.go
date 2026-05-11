// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/autodev"
	"github.com/boshu2/agentops/cli/internal/rpi"
)

const executionPacketFile = rpi.ExecutionPacketFile

// executionPacketProgram is a thin alias for the internal type.
type executionPacketProgram = rpi.ExecutionPacketProgram

type executionPacket struct {
	SchemaVersion           int                        `json:"schema_version"`
	Objective               string                     `json:"objective"`
	RunID                   string                     `json:"run_id,omitempty"`
	EpicID                  string                     `json:"epic_id,omitempty"`
	BeadID                  string                     `json:"bead_id,omitempty"`
	TrackingRepoRoot        string                     `json:"tracking_repo_root,omitempty"`
	BeadsDir                string                     `json:"beads_dir,omitempty"`
	PRURL                   string                     `json:"pr_url,omitempty"`
	MergeCommit             string                     `json:"merge_commit,omitempty"`
	PlanPath                string                     `json:"plan_path,omitempty"`
	ContractSurfaces        []string                   `json:"contract_surfaces"`
	ValidationCommands      []string                   `json:"validation_commands,omitempty"`
	ValidationLanes         []rpi.ValidationLane       `json:"validation_lanes,omitempty"`
	TrackerMode             string                     `json:"tracker_mode"`
	TrackerHealth           *trackerHealth             `json:"tracker_health,omitempty"`
	DoneCriteria            []string                   `json:"done_criteria,omitempty"`
	EpicCriteria            []rpi.Criterion            `json:"epic_criteria,omitempty"`
	BeadCriteria            map[string][]rpi.Criterion `json:"bead_criteria,omitempty"`
	Complexity              string                     `json:"complexity,omitempty"`
	ProofArtifacts          []string                   `json:"proof_artifacts,omitempty"`
	EvaluatorArtifacts      map[string]string          `json:"evaluator_artifacts,omitempty"`
	ProofUpdatedAt          string                     `json:"proof_updated_at,omitempty"`
	AutodevProgram          *executionPacketProgram    `json:"autodev_program,omitempty"`
	MixedModeRequested      bool                       `json:"mixed_mode_requested,omitempty"`
	MixedModeEffective      bool                       `json:"mixed_mode_effective,omitempty"`
	PlannerVendor           string                     `json:"planner_vendor,omitempty"`
	ReviewerVendor          string                     `json:"reviewer_vendor,omitempty"`
	MixedModeDegradedReason string                     `json:"mixed_mode_degraded_reason,omitempty"`
}

type repoExecutionProfile struct {
	ValidationCommands []string             `json:"validation_commands"`
	ValidationLanes    []rpi.ValidationLane `json:"validation_lanes"`
}

func writeExecutionPacketSeed(cwd string, state *phasedState) error {
	tracker := detectTrackerHealth(state.Opts.BDCommand, state.Opts.LookPath)
	packet := executionPacket{
		SchemaVersion:      1,
		Objective:          state.Goal,
		RunID:              state.RunID,
		EpicID:             state.EpicID,
		BeadID:             executionPacketBeadID(state),
		TrackingRepoRoot:   executionPacketTrackingRepoRoot(cwd),
		BeadsDir:           executionPacketBeadsDir(cwd),
		PRURL:              firstNonEmptyTrimmed(os.Getenv("AGENTOPS_PR_URL"), os.Getenv("GITHUB_PR_URL"), os.Getenv("PR_URL")),
		MergeCommit:        firstNonEmptyTrimmed(os.Getenv("AGENTOPS_MERGE_COMMIT"), os.Getenv("GITHUB_MERGE_COMMIT")),
		ContractSurfaces:   []string{},
		TrackerMode:        tracker.Mode,
		TrackerHealth:      &tracker,
		Complexity:         string(state.Complexity),
		MixedModeRequested: state.Opts.Mixed,
	}
	profile, err := loadRepoExecutionProfile(cwd)
	if err != nil {
		return err
	}
	if isPlanFileEpic(state.EpicID) {
		packet.PlanPath = planFileFromEpic(state.EpicID)
	} else if planPath, err := discoverPlanFile(cwd); err == nil {
		packet.PlanPath = planPath
	}

	if _, err := os.Stat(filepath.Join(cwd, "docs", "contracts", "repo-execution-profile.md")); err == nil {
		packet.ContractSurfaces = append(packet.ContractSurfaces, "docs/contracts/repo-execution-profile.md")
	}
	if _, err := os.Stat(repoExecutionProfileJSONPath(cwd)); err == nil {
		packet.ContractSurfaces = append(packet.ContractSurfaces, "docs/contracts/repo-execution-profile.json")
	}
	packet.ValidationCommands = append(packet.ValidationCommands, profile.ValidationCommands...)
	packet.ValidationLanes = append(packet.ValidationLanes, profile.ValidationLanes...)

	if state.ProgramPath != "" {
		packet.ContractSurfaces = append(packet.ContractSurfaces, state.ProgramPath)
		prog, _, err := autodev.LoadProgram(filepath.Join(cwd, state.ProgramPath))
		if err != nil {
			return fmt.Errorf("load %s for execution packet: %w", state.ProgramPath, err)
		}
		packet.ValidationCommands = append(packet.ValidationCommands, prog.ValidationCommands...)
		packet.DoneCriteria = append(packet.DoneCriteria, prog.StopConditions...)
		packet.AutodevProgram = &executionPacketProgram{
			Path:               state.ProgramPath,
			MutableScope:       prog.MutableScope,
			ImmutableScope:     prog.ImmutableScope,
			ExperimentUnit:     prog.ExperimentUnit,
			ValidationCommands: prog.ValidationCommands,
			DecisionPolicy:     prog.DecisionPolicy,
			StopConditions:     prog.StopConditions,
		}
	}

	data, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal execution packet: %w", err)
	}
	data = append(data, '\n')
	if err := writeExecutionPacketData(cwd, state, state.RunID, data); err != nil {
		return fmt.Errorf("write execution packet: %w", err)
	}
	return nil
}

func repoExecutionProfileJSONPath(cwd string) string {
	return filepath.Join(cwd, "docs", "contracts", "repo-execution-profile.json")
}

func loadRepoExecutionProfile(cwd string) (*repoExecutionProfile, error) {
	path := repoExecutionProfileJSONPath(cwd)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &repoExecutionProfile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read repo execution profile: %w", err)
	}
	var profile repoExecutionProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("parse repo execution profile: %w", err)
	}
	return &profile, nil
}

func executionPacketBeadID(state *phasedState) string {
	if state == nil || isPlanFileEpic(state.EpicID) {
		return ""
	}
	return strings.TrimSpace(state.EpicID)
}

func executionPacketTrackingRepoRoot(cwd string) string {
	root := strings.TrimSpace(cwd)
	if root == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return filepath.Clean(root)
}

func executionPacketBeadsDir(cwd string) string {
	if env := strings.TrimSpace(os.Getenv("BEADS_DIR")); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return filepath.Clean(abs)
		}
		return filepath.Clean(env)
	}
	candidate := filepath.Join(executionPacketTrackingRepoRoot(cwd), ".beads")
	if _, err := os.Stat(candidate); err == nil {
		return filepath.Clean(candidate)
	}
	return ""
}

func writeExecutionPacketData(cwd string, state *phasedState, runID string, data []byte) error {
	roots := []string{cwd}
	if state != nil {
		roots = artifactRootsForState(cwd, state)
	}

	runID = strings.TrimSpace(runID)
	for i, root := range roots {
		if err := writeExecutionPacketDataToRoot(root, runID, data); err != nil {
			if i == 0 {
				return err
			}
			VerbosePrintf("Warning: mirror execution packet write skipped for %s: %v\n", root, err)
		}
	}
	return nil
}

func writeExecutionPacketDataToRoot(root, runID string, data []byte) error {
	stateDir := filepath.Join(root, ".agents", "rpi")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		return fmt.Errorf("create execution packet directory: %w", err)
	}

	flatPath := filepath.Join(stateDir, executionPacketFile)
	if err := writePhasedStateAtomic(flatPath, data); err != nil {
		return fmt.Errorf("write execution packet latest alias: %w", err)
	}

	if runID != "" {
		runDir := rpiRunRegistryDir(root, runID)
		if err := os.MkdirAll(runDir, 0o750); err != nil {
			return fmt.Errorf("create execution packet run archive directory: %w", err)
		}
		archivePath := filepath.Join(runDir, executionPacketFile)
		if err := writePhasedStateAtomic(archivePath, data); err != nil {
			return fmt.Errorf("write execution packet run archive: %w", err)
		}
	}

	VerbosePrintf("Execution packet saved to %s\n", flatPath)
	return nil
}

type executionPacketAliasSnapshot struct {
	path    string
	data    []byte
	existed bool
}

func captureExecutionPacketAliasSnapshot(root string) (*executionPacketAliasSnapshot, error) {
	path := filepath.Join(root, ".agents", "rpi", executionPacketFile)
	data, err := os.ReadFile(path)
	if err == nil {
		return &executionPacketAliasSnapshot{path: path, data: data, existed: true}, nil
	}
	if os.IsNotExist(err) {
		return &executionPacketAliasSnapshot{path: path}, nil
	}
	return nil, fmt.Errorf("snapshot execution packet latest alias: %w", err)
}

func (s *executionPacketAliasSnapshot) restore() error {
	if s == nil {
		return nil
	}
	if !s.existed {
		if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove dry-run execution packet latest alias: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return fmt.Errorf("restore execution packet latest alias directory: %w", err)
	}
	if err := writePhasedStateAtomic(s.path, s.data); err != nil {
		return fmt.Errorf("restore execution packet latest alias: %w", err)
	}
	return nil
}
