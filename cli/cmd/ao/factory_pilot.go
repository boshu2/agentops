package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	daemonpkg "github.com/boshu2/agentops/cli/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	factoryPilotGoal               string
	factoryPilotRunID              string
	factoryPilotWorktreeRoot       string
	factoryPilotValidationCommands []string
)

type factoryPilotOptions struct {
	Goal               string
	RunID              string
	WorktreeRoot       string
	ValidationCommands []string
	Now                func() time.Time
	Policy             daemonpkg.RoutingPolicy
}

type factoryPilotResult struct {
	SchemaVersion             int                         `json:"schema_version"`
	Command                   string                      `json:"command"`
	RunID                     string                      `json:"run_id"`
	Goal                      string                      `json:"goal"`
	Mode                      string                      `json:"mode"`
	WorktreeRoot              string                      `json:"worktree_root"`
	MaxConcurrency            int                         `json:"max_concurrency"`
	AutoMergeEnabled          bool                        `json:"auto_merge_enabled"`
	ManualMergeRequired       bool                        `json:"manual_merge_required"`
	ValidationCommands        []string                    `json:"validation_commands"`
	Phases                    []factoryPilotPhase         `json:"phases"`
	Slots                     []factoryPilotSlot          `json:"slots"`
	Worktrees                 []factoryPilotWorktree      `json:"worktrees"`
	ExcludedLanes             []factoryPilotExcludedLane  `json:"excluded_lanes"`
	EventTemplates            []factoryPilotEventTemplate `json:"event_templates"`
	YieldObservationTemplates []factoryPilotYieldTemplate `json:"yield_observation_templates"`
	RetainedFailureArtifacts  []string                    `json:"retained_failure_artifacts"`
	ManualMerge               string                      `json:"manual_merge"`
	Runbook                   string                      `json:"runbook"`
}

type factoryPilotPhase struct {
	Name                string `json:"name"`
	BaselineOrTreatment string `json:"baseline_or_treatment"`
	WorkerCount         int    `json:"worker_count"`
	MaxConcurrency      int    `json:"max_concurrency"`
	LaneID              string `json:"lane_id"`
}

type factoryPilotSlot struct {
	SlotID                 string                     `json:"slot_id"`
	Phase                  string                     `json:"phase"`
	WorkerIndex            int                        `json:"worker_index"`
	LaneID                 string                     `json:"lane_id"`
	Provider               string                     `json:"provider"`
	Runtime                string                     `json:"runtime"`
	Model                  string                     `json:"model"`
	Authority              daemonpkg.RoutingAuthority `json:"authority"`
	TaskClass              string                     `json:"task_class"`
	JobID                  string                     `json:"job_id"`
	WorktreeID             string                     `json:"worktree_id"`
	Branch                 string                     `json:"branch"`
	WorktreePath           string                     `json:"worktree_path"`
	MaxConcurrencySnapshot int                        `json:"max_concurrency_snapshot"`
	RetentionPolicy        string                     `json:"retention_policy"`
	MergeDisposition       string                     `json:"merge_disposition"`
}

type factoryPilotWorktree struct {
	WorktreeID       string `json:"worktree_id"`
	OwnerRunID       string `json:"owner_run_id"`
	OwnerJobID       string `json:"owner_job_id"`
	OwnerSlotID      string `json:"owner_slot_id"`
	Branch           string `json:"branch"`
	Path             string `json:"path"`
	RetentionPolicy  string `json:"retention_policy"`
	MergeDisposition string `json:"merge_disposition"`
}

type factoryPilotExcludedLane struct {
	LaneID         string                     `json:"lane_id"`
	Provider       string                     `json:"provider"`
	Runtime        string                     `json:"runtime"`
	Authority      daemonpkg.RoutingAuthority `json:"authority"`
	DisabledReason string                     `json:"disabled_reason"`
}

type factoryPilotEventTemplate struct {
	EventType daemonpkg.EventType `json:"event_type"`
	Phase     string              `json:"phase"`
	JobID     string              `json:"job_id"`
	Payload   map[string]any      `json:"payload"`
}

type factoryPilotYieldTemplate struct {
	BaselineOrTreatment string   `json:"baseline_or_treatment"`
	LaneID              string   `json:"lane_id"`
	EventType           string   `json:"event_type"`
	RequiredArtifacts   []string `json:"required_artifacts"`
	RequiredMetrics     []string `json:"required_metrics"`
}

var factoryPilotCmd = &cobra.Command{
	Use:   "pilot",
	Short: "Print a bounded 1-vs-2 cloud-frontier factory pilot plan",
	Long: `Print the smallest safe factory pilot plan for comparing one worker
against two cloud/frontier workers.

The command does not merge or dispatch workers. It emits the slot/worktree
allocation, required validation commands, lifecycle event templates, yield
observation templates, disabled reference lanes, and manual merge instructions
an operator can execute through agentopsd-backed factory work.`,
	Args: cobra.NoArgs,
	RunE: runFactoryPilot,
}

func init() {
	factoryCmd.AddCommand(factoryPilotCmd)
	factoryPilotCmd.Flags().StringVar(&factoryPilotGoal, "goal", "", "Pilot objective to compare with 1-vs-2 cloud/frontier workers")
	factoryPilotCmd.Flags().StringVar(&factoryPilotRunID, "run-id", "", "Stable pilot run id (default generated from current time)")
	factoryPilotCmd.Flags().StringVar(&factoryPilotWorktreeRoot, "worktree-root", "", "Factory-owned worktree root for allocated worker worktrees")
	factoryPilotCmd.Flags().StringArrayVar(&factoryPilotValidationCommands, "validation", nil, "Validation command required before manual merge review (repeatable)")
}

func runFactoryPilot(cmd *cobra.Command, args []string) error {
	cwd, err := resolveProjectDir()
	if err != nil {
		return err
	}
	result, err := buildFactoryPilotPlan(cwd, factoryPilotOptions{
		Goal:               factoryPilotGoal,
		RunID:              factoryPilotRunID,
		WorktreeRoot:       factoryPilotWorktreeRoot,
		ValidationCommands: factoryPilotValidationCommands,
	})
	if err != nil {
		return err
	}
	return outputFactoryPilotResult(cmd, result)
}

func buildFactoryPilotPlan(cwd string, opts factoryPilotOptions) (factoryPilotResult, error) {
	goal := strings.TrimSpace(opts.Goal)
	if goal == "" {
		return factoryPilotResult{}, errors.New("factory pilot requires --goal")
	}
	policy := opts.Policy
	if policy.SchemaVersion == 0 {
		policy = daemonpkg.DefaultFactoryRoutingPolicy()
	}
	if err := daemonpkg.ValidateRoutingPolicy(policy); err != nil {
		return factoryPilotResult{}, fmt.Errorf("routing policy: %w", err)
	}
	lane, ok := policy.LaneByID(policy.DefaultLane)
	if !ok {
		return factoryPilotResult{}, fmt.Errorf("default lane %q not found", policy.DefaultLane)
	}
	if err := validateFactoryPilotLane(lane); err != nil {
		return factoryPilotResult{}, err
	}
	commands := factoryPilotValidationCommandsFor(lane, opts.ValidationCommands)
	if len(commands) == 0 {
		return factoryPilotResult{}, errors.New("factory pilot requires validation commands")
	}

	runID := strings.TrimSpace(opts.RunID)
	if runID == "" {
		now := time.Now
		if opts.Now != nil {
			now = opts.Now
		}
		runID = "factory-pilot-" + now().UTC().Format("20060102-150405")
	}
	worktreeRoot := strings.TrimSpace(opts.WorktreeRoot)
	if worktreeRoot == "" {
		worktreeRoot = filepath.Join(cwd, ".agents", "ao", "factory", "pilots", runID, "worktrees")
	}

	result := factoryPilotResult{
		SchemaVersion:            1,
		Command:                  "ao factory pilot",
		RunID:                    runID,
		Goal:                     goal,
		Mode:                     "plan-only-manual-merge",
		WorktreeRoot:             worktreeRoot,
		MaxConcurrency:           2,
		AutoMergeEnabled:         false,
		ManualMergeRequired:      true,
		ValidationCommands:       commands,
		ExcludedLanes:            factoryPilotExcludedLanes(policy),
		RetainedFailureArtifacts: []string{"validation", "diff", "logs", "transcript", "worktree"},
		ManualMerge:              "manual merge only after every validation command passes",
		Runbook:                  "docs/runbooks/cloud-frontier-pilot.md",
	}
	for _, phase := range []factoryPilotPhase{
		{Name: "baseline-1-worker", BaselineOrTreatment: "baseline", WorkerCount: 1, MaxConcurrency: 1, LaneID: lane.ID},
		{Name: "treatment-2-workers", BaselineOrTreatment: "treatment", WorkerCount: 2, MaxConcurrency: 2, LaneID: lane.ID},
	} {
		result.Phases = append(result.Phases, phase)
		addFactoryPilotPhase(&result, phase, lane)
	}
	return result, nil
}

func validateFactoryPilotLane(lane daemonpkg.RoutingLane) error {
	if !lane.Enabled {
		return fmt.Errorf("factory pilot lane %q must be enabled", lane.ID)
	}
	if lane.Provider != "openai" || lane.Runtime != "codex" {
		return fmt.Errorf("factory pilot requires cloud/frontier codex lane, got provider=%q runtime=%q", lane.Provider, lane.Runtime)
	}
	if lane.Authority != daemonpkg.RoutingAuthorityDelegated {
		return fmt.Errorf("factory pilot lane authority = %q, want DELEGATED", lane.Authority)
	}
	if lane.MergeEligibility == nil || !lane.MergeEligibility.ManualMergeRequired {
		return fmt.Errorf("factory pilot lane %q must require manual merge eligibility", lane.ID)
	}
	return nil
}

func factoryPilotValidationCommandsFor(lane daemonpkg.RoutingLane, override []string) []string {
	source := override
	if source == nil && lane.MergeEligibility != nil {
		source = lane.MergeEligibility.ValidationCommands
	}
	commands := make([]string, 0, len(source))
	for _, command := range source {
		if trimmed := strings.TrimSpace(command); trimmed != "" {
			commands = append(commands, trimmed)
		}
	}
	return commands
}

func addFactoryPilotPhase(result *factoryPilotResult, phase factoryPilotPhase, lane daemonpkg.RoutingLane) {
	for worker := 1; worker <= phase.WorkerCount; worker++ {
		slot := newFactoryPilotSlot(*result, phase, lane, worker)
		result.Slots = append(result.Slots, slot)
		result.Worktrees = append(result.Worktrees, factoryPilotWorktree{
			WorktreeID:       slot.WorktreeID,
			OwnerRunID:       result.RunID,
			OwnerJobID:       slot.JobID,
			OwnerSlotID:      slot.SlotID,
			Branch:           slot.Branch,
			Path:             slot.WorktreePath,
			RetentionPolicy:  slot.RetentionPolicy,
			MergeDisposition: slot.MergeDisposition,
		})
		result.EventTemplates = append(result.EventTemplates, factoryPilotEventTemplates(*result, phase, slot)...)
	}
	result.YieldObservationTemplates = append(result.YieldObservationTemplates, factoryPilotYieldTemplate{
		BaselineOrTreatment: phase.BaselineOrTreatment,
		LaneID:              lane.ID,
		EventType:           string(daemonpkg.EventFactoryYieldObservation),
		RequiredArtifacts:   []string{"routing", "validation", "merge", "diff", "transcript"},
		RequiredMetrics:     []string{"accepted_patches", "wall_clock_minutes", "review_minutes", "recovery_minutes", "model_cost_usd", "conflict_count", "defect_count", "operator_interventions"},
	})
}

func newFactoryPilotSlot(result factoryPilotResult, phase factoryPilotPhase, lane daemonpkg.RoutingLane, worker int) factoryPilotSlot {
	suffix := fmt.Sprintf("%s-%d", phase.BaselineOrTreatment, worker)
	slotID := "slot-" + suffix
	jobID := "job-" + suffix
	worktreeID := "wt-" + suffix
	branch := fmt.Sprintf("factory/%s/%s", result.RunID, suffix)
	return factoryPilotSlot{
		SlotID:                 slotID,
		Phase:                  phase.Name,
		WorkerIndex:            worker,
		LaneID:                 lane.ID,
		Provider:               lane.Provider,
		Runtime:                lane.Runtime,
		Model:                  lane.Model,
		Authority:              lane.Authority,
		TaskClass:              "code_change",
		JobID:                  jobID,
		WorktreeID:             worktreeID,
		Branch:                 branch,
		WorktreePath:           filepath.Join(result.WorktreeRoot, worktreeID),
		MaxConcurrencySnapshot: phase.MaxConcurrency,
		RetentionPolicy:        "retain_on_failure",
		MergeDisposition:       "manual_pending",
	}
}

func factoryPilotEventTemplates(result factoryPilotResult, phase factoryPilotPhase, slot factoryPilotSlot) []factoryPilotEventTemplate {
	basePayload := map[string]any{
		"run_id":    result.RunID,
		"task_id":   result.Goal,
		"lane_id":   slot.LaneID,
		"provider":  slot.Provider,
		"runtime":   slot.Runtime,
		"model":     slot.Model,
		"authority": string(slot.Authority),
	}
	return []factoryPilotEventTemplate{
		{EventType: daemonpkg.EventFactoryJobSubmitted, Phase: phase.Name, JobID: slot.JobID, Payload: withPilotPayload(basePayload, "objective", result.Goal)},
		{EventType: daemonpkg.EventFactoryRoutingDecided, Phase: phase.Name, JobID: slot.JobID, Payload: withPilotPayload(basePayload, "reason", "bounded cloud-frontier pilot")},
		{EventType: daemonpkg.EventFactorySlotAllocated, Phase: phase.Name, JobID: slot.JobID, Payload: withPilotPayload(basePayload, "slot_id", slot.SlotID, "max_concurrency_snapshot", slot.MaxConcurrencySnapshot)},
		{EventType: daemonpkg.EventFactoryWorktreeAllocated, Phase: phase.Name, JobID: slot.JobID, Payload: withPilotPayload(basePayload, "worktree_id", slot.WorktreeID, "slot_id", slot.SlotID, "path", slot.WorktreePath, "branch", slot.Branch, "owner_job_id", slot.JobID, "retention_policy", slot.RetentionPolicy, "merge_disposition", slot.MergeDisposition)},
		{EventType: daemonpkg.EventFactoryValidationStarted, Phase: phase.Name, JobID: slot.JobID, Payload: withPilotPayload(basePayload, "validation_id", "val-"+slot.JobID, "commands", result.ValidationCommands, "level", "L3")},
		{EventType: daemonpkg.EventFactoryMergeDecision, Phase: phase.Name, JobID: slot.JobID, Payload: withPilotPayload(basePayload, "decision", string(daemonpkg.FactoryMergeDecisionManualPending), "decider", "operator", "reason", "manual merge required", "manual_command", "git merge "+slot.Branch)},
		{EventType: daemonpkg.EventFactoryYieldObservation, Phase: phase.Name, JobID: slot.JobID, Payload: withPilotPayload(basePayload, "baseline_or_treatment", phase.BaselineOrTreatment, "validation_status", string(daemonpkg.FactoryValidationStatusPassed), "merge_status", string(daemonpkg.FactoryMergeDecisionManualPending), "artifact_refs", map[string]string{"routing": "routing.json", "validation": "validation.json", "merge": "merge-decision.json", "diff": "diff.patch", "transcript": "transcript.jsonl"})},
	}
}

func withPilotPayload(base map[string]any, pairs ...any) map[string]any {
	payload := make(map[string]any, len(base)+len(pairs)/2)
	for key, value := range base {
		payload[key] = value
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if ok {
			payload[key] = pairs[i+1]
		}
	}
	return payload
}

func factoryPilotExcludedLanes(policy daemonpkg.RoutingPolicy) []factoryPilotExcludedLane {
	excluded := []factoryPilotExcludedLane{}
	for _, lane := range policy.Lanes {
		if lane.Enabled && lane.Provider != "gascity" && lane.Runtime != "mt-olympus" {
			continue
		}
		excluded = append(excluded, factoryPilotExcludedLane{
			LaneID:         lane.ID,
			Provider:       lane.Provider,
			Runtime:        lane.Runtime,
			Authority:      lane.Authority,
			DisabledReason: lane.DisabledReason,
		})
	}
	return excluded
}

func outputFactoryPilotResult(cmd *cobra.Command, result factoryPilotResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Factory Pilot Plan\n")
	fmt.Fprintf(cmd.OutOrStdout(), "==================\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Run: %s\n", result.RunID)
	fmt.Fprintf(cmd.OutOrStdout(), "Goal: %s\n", result.Goal)
	fmt.Fprintf(cmd.OutOrStdout(), "Max concurrency: %d\n", result.MaxConcurrency)
	fmt.Fprintf(cmd.OutOrStdout(), "Merge: manual only\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Runbook: %s\n\n", result.Runbook)
	fmt.Fprintf(cmd.OutOrStdout(), "Validation:\n")
	for _, command := range result.ValidationCommands {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", command)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nPhases:\n")
	for _, phase := range result.Phases {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %d worker(s), max concurrency %d\n", phase.Name, phase.WorkerCount, phase.MaxConcurrency)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nWorktrees:\n")
	for _, worktree := range result.Worktrees {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s (%s)\n", worktree.WorktreeID, worktree.Path, worktree.RetentionPolicy)
	}
	return nil
}
