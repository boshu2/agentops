package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const RoutingPolicySchemaVersion = 1

type RoutingAuthority string

const (
	RoutingAuthorityObserve       RoutingAuthority = "OBSERVE"
	RoutingAuthorityAdvisory      RoutingAuthority = "ADVISORY"
	RoutingAuthorityDelegated     RoutingAuthority = "DELEGATED"
	RoutingAuthorityAuthoritative RoutingAuthority = "AUTHORITATIVE"
)

type RoutingPolicy struct {
	SchemaVersion        int           `json:"schema_version"`
	PolicyID             string        `json:"policy_id"`
	DefaultLane          string        `json:"default_lane"`
	MaxTotalConcurrency  int           `json:"max_total_concurrency"`
	AutoMergeEnabled     bool          `json:"auto_merge_enabled"`
	Lanes                []RoutingLane `json:"lanes"`
	ManualMergeByDefault bool          `json:"manual_merge_by_default"`
}

type RoutingLane struct {
	ID                 string                `json:"id"`
	Enabled            bool                  `json:"enabled"`
	Authority          RoutingAuthority      `json:"authority"`
	Provider           string                `json:"provider"`
	Runtime            string                `json:"runtime"`
	Model              string                `json:"model"`
	TaskClasses        []string              `json:"task_classes"`
	MaxConcurrency     int                   `json:"max_concurrency"`
	CostHintUSDPerHour float64               `json:"cost_hint_usd_per_hour,omitempty"`
	LatencyHint        string                `json:"latency_hint,omitempty"`
	QualityPrior       string                `json:"quality_prior,omitempty"`
	YieldGate          *RoutingYieldGate     `json:"yield_gate,omitempty"`
	PromotionGate      *RoutingPromotionGate `json:"promotion_gate,omitempty"`
	MergeEligibility   *RoutingMergeGate     `json:"merge_eligibility,omitempty"`
	DisabledReason     string                `json:"disabled_reason,omitempty"`
}

type RoutingYieldGate struct {
	MinAcceptedPatchesPerHour float64 `json:"min_accepted_patches_per_hour"`
	MinSampleSize             int     `json:"min_sample_size"`
}

type RoutingPromotionGate struct {
	RequiresYieldEvidence bool `json:"requires_yield_evidence"`
}

type RoutingMergeGate struct {
	ManualMergeRequired               bool      `json:"manual_merge_required"`
	ValidationCommands                []string  `json:"validation_commands"`
	ValidationFailureTerminalEvent    EventType `json:"validation_failure_terminal_event"`
	RetainArtifactsOnFailure          bool      `json:"retain_artifacts_on_failure"`
	RetainWorktreeOnValidationFailure bool      `json:"retain_worktree_on_validation_failure"`
}

func DefaultFactoryRoutingPolicy() RoutingPolicy {
	return RoutingPolicy{
		SchemaVersion:        RoutingPolicySchemaVersion,
		PolicyID:             "repo-default",
		DefaultLane:          "frontier-codex",
		MaxTotalConcurrency:  2,
		AutoMergeEnabled:     false,
		ManualMergeByDefault: true,
		Lanes: []RoutingLane{
			{
				ID:                 "frontier-codex",
				Enabled:            true,
				Authority:          RoutingAuthorityDelegated,
				Provider:           "openai",
				Runtime:            "codex",
				Model:              "frontier-default",
				TaskClasses:        []string{"code_change", "test_repair", "docs_change"},
				MaxConcurrency:     2,
				LatencyHint:        "interactive",
				QualityPrior:       "default",
				CostHintUSDPerHour: 0,
				YieldGate: &RoutingYieldGate{
					MinAcceptedPatchesPerHour: 0,
					MinSampleSize:             0,
				},
				MergeEligibility: &RoutingMergeGate{
					ManualMergeRequired: true,
					ValidationCommands: []string{
						"cd cli && go test ./internal/daemon -run 'RoutingPolicy|FactoryProjection'",
						"scripts/pre-push-gate.sh --fast",
					},
					ValidationFailureTerminalEvent:    EventFactoryJobTerminal,
					RetainArtifactsOnFailure:          true,
					RetainWorktreeOnValidationFailure: true,
				},
			},
			{
				ID:             "local-observer",
				Enabled:        true,
				Authority:      RoutingAuthorityAdvisory,
				Provider:       "local",
				Runtime:        "mlx-or-ollama",
				Model:          "local-configured",
				TaskClasses:    []string{"scout", "retrieve", "summarize", "classify", "preflight", "critique"},
				MaxConcurrency: 1,
				LatencyHint:    "batch",
				QualityPrior:   "advisory",
				PromotionGate:  &RoutingPromotionGate{RequiresYieldEvidence: true},
				DisabledReason: "",
			},
			{
				ID:             "gascity-reference",
				Enabled:        false,
				Authority:      RoutingAuthorityObserve,
				Provider:       "gascity",
				Runtime:        "mt-olympus",
				Model:          "provider-selected",
				TaskClasses:    []string{"reference_runtime"},
				MaxConcurrency: 0,
				DisabledReason: "Not production-critical for milestone 1",
			},
		},
	}
}

func ParseRoutingPolicyJSON(data []byte) (RoutingPolicy, error) {
	var policy RoutingPolicy
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return policy, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("routing policy JSON must contain exactly one object")
		}
		return policy, err
	}
	if err := ValidateRoutingPolicy(policy); err != nil {
		return policy, err
	}
	return policy, nil
}

func ValidateRoutingPolicy(policy RoutingPolicy) error {
	if policy.SchemaVersion != RoutingPolicySchemaVersion {
		return fmt.Errorf("schema_version mismatch: got %d want %d", policy.SchemaVersion, RoutingPolicySchemaVersion)
	}
	if strings.TrimSpace(policy.PolicyID) == "" {
		return fmt.Errorf("policy_id is required")
	}
	if strings.TrimSpace(policy.DefaultLane) == "" {
		return fmt.Errorf("default_lane is required")
	}
	if policy.MaxTotalConcurrency <= 0 {
		return fmt.Errorf("max_total_concurrency must be > 0")
	}
	if policy.AutoMergeEnabled {
		return fmt.Errorf("auto_merge_enabled must be false")
	}
	if !policy.ManualMergeByDefault {
		return fmt.Errorf("manual_merge_by_default must be true")
	}
	if len(policy.Lanes) == 0 {
		return fmt.Errorf("lanes are required")
	}

	seen := make(map[string]bool, len(policy.Lanes))
	defaultLaneEnabled := false
	for i, lane := range policy.Lanes {
		if err := validateRoutingLane(policy, lane); err != nil {
			return fmt.Errorf("lanes[%d] %q: %w", i, lane.ID, err)
		}
		id := strings.TrimSpace(lane.ID)
		if seen[id] {
			return fmt.Errorf("duplicate lane id %q", id)
		}
		seen[id] = true
		if id == policy.DefaultLane && lane.Enabled {
			defaultLaneEnabled = true
		}
	}
	if !seen[policy.DefaultLane] {
		return fmt.Errorf("default_lane %q does not exist", policy.DefaultLane)
	}
	if !defaultLaneEnabled {
		return fmt.Errorf("default_lane %q must be enabled", policy.DefaultLane)
	}
	return nil
}

func (policy RoutingPolicy) LaneByID(id string) (RoutingLane, bool) {
	for _, lane := range policy.Lanes {
		if lane.ID == id {
			return lane, true
		}
	}
	return RoutingLane{}, false
}

func (policy RoutingPolicy) SelectLane(taskClass string) (RoutingLane, error) {
	taskClass = strings.TrimSpace(taskClass)
	if taskClass == "" {
		return RoutingLane{}, fmt.Errorf("task_class is required")
	}
	for _, lane := range policy.Lanes {
		if lane.Enabled && laneSupportsTaskClass(lane, taskClass) {
			return lane, nil
		}
	}
	return RoutingLane{}, fmt.Errorf("no enabled routing lane supports task_class %q", taskClass)
}

func validateRoutingLane(policy RoutingPolicy, lane RoutingLane) error {
	if err := validateRoutingLaneRequiredFields(lane); err != nil {
		return err
	}
	if err := validateRoutingLaneConcurrency(policy, lane); err != nil {
		return err
	}
	return validateRoutingLaneGates(lane)
}

func validateRoutingLaneRequiredFields(lane RoutingLane) error {
	if strings.TrimSpace(lane.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if err := ValidateRoutingAuthority(lane.Authority); err != nil {
		return err
	}
	if strings.TrimSpace(lane.Provider) == "" {
		return fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(lane.Runtime) == "" {
		return fmt.Errorf("runtime is required")
	}
	if strings.TrimSpace(lane.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if len(lane.TaskClasses) == 0 {
		return fmt.Errorf("task_classes are required")
	}
	return nil
}

func validateRoutingLaneConcurrency(policy RoutingPolicy, lane RoutingLane) error {
	if lane.MaxConcurrency < 0 {
		return fmt.Errorf("max_concurrency must be >= 0")
	}
	if lane.Enabled && lane.MaxConcurrency == 0 {
		return fmt.Errorf("enabled lane max_concurrency must be > 0")
	}
	if lane.MaxConcurrency > policy.MaxTotalConcurrency {
		return fmt.Errorf("max_concurrency %d exceeds max_total_concurrency %d", lane.MaxConcurrency, policy.MaxTotalConcurrency)
	}
	return nil
}

func validateRoutingLaneGates(lane RoutingLane) error {
	if lane.YieldGate != nil {
		if lane.YieldGate.MinAcceptedPatchesPerHour < 0 {
			return fmt.Errorf("yield_gate.min_accepted_patches_per_hour must be >= 0")
		}
		if lane.YieldGate.MinSampleSize < 0 {
			return fmt.Errorf("yield_gate.min_sample_size must be >= 0")
		}
	}
	if lane.Provider == "local" && lane.Authority != RoutingAuthorityObserve && lane.Authority != RoutingAuthorityAdvisory {
		return fmt.Errorf("local provider authority must be OBSERVE or ADVISORY")
	}
	if lane.Authority == RoutingAuthorityAuthoritative &&
		(lane.PromotionGate == nil || !lane.PromotionGate.RequiresYieldEvidence) {
		return fmt.Errorf("AUTHORITATIVE lanes require promotion_gate.requires_yield_evidence")
	}
	if isGasCityLane(lane) && lane.Enabled && laneHasProductionTaskClass(lane) {
		return fmt.Errorf("GasCity/Mt. Olympus production lanes are disabled for milestone 1")
	}
	if !lane.Enabled && strings.TrimSpace(lane.DisabledReason) == "" {
		return fmt.Errorf("disabled lanes require disabled_reason")
	}
	if lane.MergeEligibility != nil {
		if err := validateRoutingMergeGate(*lane.MergeEligibility); err != nil {
			return err
		}
	}
	if laneRequiresMergeGate(lane) && lane.MergeEligibility == nil {
		return fmt.Errorf("merge-eligible lanes require merge_eligibility validation commands")
	}
	return nil
}

func validateRoutingMergeGate(gate RoutingMergeGate) error {
	if !gate.ManualMergeRequired {
		return fmt.Errorf("merge_eligibility.manual_merge_required must be true")
	}
	if len(gate.ValidationCommands) == 0 {
		return fmt.Errorf("merge_eligibility.validation_commands are required")
	}
	for i, command := range gate.ValidationCommands {
		if strings.TrimSpace(command) == "" {
			return fmt.Errorf("merge_eligibility.validation_commands[%d] is required", i)
		}
	}
	if err := ValidateEventType(gate.ValidationFailureTerminalEvent); err != nil {
		return fmt.Errorf("merge_eligibility.validation_failure_terminal_event: %w", err)
	}
	if gate.ValidationFailureTerminalEvent != EventFactoryJobTerminal {
		return fmt.Errorf("merge_eligibility.validation_failure_terminal_event must be %q", EventFactoryJobTerminal)
	}
	if !gate.RetainArtifactsOnFailure {
		return fmt.Errorf("merge_eligibility.retain_artifacts_on_failure must be true")
	}
	if !gate.RetainWorktreeOnValidationFailure {
		return fmt.Errorf("merge_eligibility.retain_worktree_on_validation_failure must be true")
	}
	return nil
}

func ValidateRoutingAuthority(authority RoutingAuthority) error {
	switch authority {
	case RoutingAuthorityObserve, RoutingAuthorityAdvisory, RoutingAuthorityDelegated, RoutingAuthorityAuthoritative:
		return nil
	default:
		return fmt.Errorf("invalid routing authority %q", authority)
	}
}

func laneSupportsTaskClass(lane RoutingLane, taskClass string) bool {
	for _, candidate := range lane.TaskClasses {
		if strings.EqualFold(strings.TrimSpace(candidate), taskClass) {
			return true
		}
	}
	return false
}

func laneHasProductionTaskClass(lane RoutingLane) bool {
	for _, taskClass := range lane.TaskClasses {
		switch strings.ToLower(strings.TrimSpace(taskClass)) {
		case "code_change", "test_repair", "docs_change", "merge_decision":
			return true
		}
	}
	return false
}

func laneRequiresMergeGate(lane RoutingLane) bool {
	if !lane.Enabled || !laneHasProductionTaskClass(lane) {
		return false
	}
	switch lane.Authority {
	case RoutingAuthorityDelegated, RoutingAuthorityAuthoritative:
		return true
	default:
		return false
	}
}

func isGasCityLane(lane RoutingLane) bool {
	return strings.EqualFold(lane.Provider, "gascity") || strings.EqualFold(lane.Runtime, "mt-olympus")
}
