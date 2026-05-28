// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/autodev"
	"github.com/boshu2/agentops/cli/internal/rpi"
)

const executionPacketFile = rpi.ExecutionPacketFile
const executionPacketRankedPacketPath = ".agents/rpi/ranked-packet.json"

// executionPacketProgram is a thin alias for the internal type.
type executionPacketProgram = rpi.ExecutionPacketProgram

type executionPacket struct {
	SchemaVersion           int                            `json:"schema_version"`
	Objective               string                         `json:"objective"`
	RunID                   string                         `json:"run_id,omitempty"`
	EpicID                  string                         `json:"epic_id,omitempty"`
	BeadID                  string                         `json:"bead_id,omitempty"`
	TrackingRepoRoot        string                         `json:"tracking_repo_root,omitempty"`
	BeadsDir                string                         `json:"beads_dir,omitempty"`
	PRURL                   string                         `json:"pr_url,omitempty"`
	MergeCommit             string                         `json:"merge_commit,omitempty"`
	PlanPath                string                         `json:"plan_path,omitempty"`
	Density                 *rpi.ExecutionPacketDensity    `json:"density,omitempty"`
	Artifacts               *rpi.ExecutionPacketArtifacts  `json:"artifacts,omitempty"`
	ContractSurfaces        []string                       `json:"contract_surfaces"`
	ValidationCommands      []string                       `json:"validation_commands,omitempty"`
	ValidationLanes         []rpi.ValidationLane           `json:"validation_lanes,omitempty"`
	TrackerMode             string                         `json:"tracker_mode"`
	TrackerHealth           *trackerHealth                 `json:"tracker_health,omitempty"`
	DoneCriteria            []string                       `json:"done_criteria,omitempty"`
	EpicCriteria            []rpi.Criterion                `json:"epic_criteria,omitempty"`
	BeadCriteria            map[string][]rpi.Criterion     `json:"bead_criteria,omitempty"`
	Complexity              string                         `json:"complexity,omitempty"`
	PreMortemVerdict        string                         `json:"pre_mortem_verdict,omitempty"`
	TestLevels              *rpi.ExecutionPacketTestLevels `json:"test_levels,omitempty"`
	RankedPacketPath        string                         `json:"ranked_packet_path,omitempty"`
	DiscoveryTimestamp      string                         `json:"discovery_timestamp,omitempty"`
	ProofArtifacts          []string                       `json:"proof_artifacts,omitempty"`
	EvaluatorArtifacts      map[string]string              `json:"evaluator_artifacts,omitempty"`
	ProofUpdatedAt          string                         `json:"proof_updated_at,omitempty"`
	AutodevProgram          *executionPacketProgram        `json:"autodev_program,omitempty"`
	MixedModeRequested      bool                           `json:"mixed_mode_requested,omitempty"`
	MixedModeEffective      bool                           `json:"mixed_mode_effective,omitempty"`
	PlannerVendor           string                         `json:"planner_vendor,omitempty"`
	ReviewerVendor          string                         `json:"reviewer_vendor,omitempty"`
	MixedModeDegradedReason string                         `json:"mixed_mode_degraded_reason,omitempty"`
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
	packet.Density = executionPacketDensityForState(cwd, state, &packet)
	packet.Artifacts = executionPacketArtifactsForPacket(packet)
	packet.TestLevels = executionPacketTestLevelsForState(state)
	packet.RankedPacketPath = executionPacketRankedPacketPath
	packet.DiscoveryTimestamp = time.Now().UTC().Format(time.RFC3339)

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

func executionPacketDensityForState(cwd string, state *phasedState, packet *executionPacket) *rpi.ExecutionPacketDensity {
	intent := ""
	complexity := ""
	if state != nil {
		intent = strings.TrimSpace(state.Goal)
		complexity = strings.TrimSpace(string(state.Complexity))
	}
	if intent == "" && packet != nil {
		intent = strings.TrimSpace(packet.Objective)
	}
	if complexity == "" {
		complexity = string(ComplexityStandard)
	}
	writeScope := []string{}
	evidence := []string{}
	if packet != nil {
		writeScope = append(writeScope, packet.ContractSurfaces...)
		evidence = append(evidence, packet.ValidationCommands...)
	}
	return &rpi.ExecutionPacketDensity{
		Intent: intent,
		Boundary: rpi.ExecutionPacketBoundary{
			BoundedContext: executionPacketBoundedContext(cwd),
			NonGoals:       []string{},
			WriteScope:     writeScope,
		},
		Evidence: evidence,
		Decision: fmt.Sprintf("Seeded by ao rpi discovery for %s complexity; detailed decisions live in linked artifacts.", complexity),
		Constraint: []string{
			"Keep raw research, plan prose, and council deliberation in referenced artifacts.",
			"Use repo execution profile lanes for validation selection.",
		},
		NextAction: executionPacketNextAction(state),
	}
}

func executionPacketBoundedContext(cwd string) string {
	root := executionPacketTrackingRepoRoot(cwd)
	name := filepath.Base(root)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "repository"
	}
	return name
}

func executionPacketNextAction(state *phasedState) string {
	if state != nil {
		epicID := strings.TrimSpace(state.EpicID)
		if epicID != "" && !isPlanFileEpic(epicID) {
			return "/crank " + epicID
		}
	}
	return "/crank .agents/rpi/execution-packet.json"
}

func executionPacketArtifactsForPacket(packet executionPacket) *rpi.ExecutionPacketArtifacts {
	return &rpi.ExecutionPacketArtifacts{
		PlanPath:         strings.TrimSpace(packet.PlanPath),
		RankedPacketPath: executionPacketRankedPacketPath,
	}
}

func executionPacketTestLevelsForState(state *phasedState) *rpi.ExecutionPacketTestLevels {
	complexity := ComplexityStandard
	testFirst := false
	if state != nil {
		complexity = state.Complexity
		testFirst = state.TestFirst
	}
	switch complexity {
	case ComplexityFast:
		return &rpi.ExecutionPacketTestLevels{
			Required:    []string{"L0"},
			Recommended: []string{"L1"},
			Rationale:   "Fast-path discovery keeps the autonomous proof floor small and recommends one focused unit-level check.",
		}
	case ComplexityFull:
		return &rpi.ExecutionPacketTestLevels{
			Required:    []string{"L0", "L1", "L2"},
			Recommended: []string{"L3"},
			Rationale:   "Full discovery expects contract, unit, and integration evidence before higher-level component coverage.",
		}
	default:
		required := []string{"L0", "L1"}
		recommended := []string{"L2"}
		if testFirst {
			required = append(required, "L2")
			recommended = []string{"L3"}
		}
		rationale := "Standard discovery requires contract and unit-level proof, with integration evidence recommended when the slice crosses adapters."
		if testFirst {
			rationale = "Test-first standard discovery requires contract, unit, and integration proof for the selected implementation slice."
		}
		return &rpi.ExecutionPacketTestLevels{
			Required:    required,
			Recommended: recommended,
			Rationale:   rationale,
		}
	}
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
