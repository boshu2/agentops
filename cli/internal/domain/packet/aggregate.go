// Package packet defines the canonical ExecutionPacket aggregate root.
package packet

// ExecutionPacket is the domain-canonical rich work packet. It mirrors
// schemas/execution-packet.schema.json and is the persisted form used by the
// packet repository. Legacy slim packets are migrated at the storage boundary.
type ExecutionPacket struct {
	SchemaVersion           int                                   `json:"schema_version"`
	Objective               string                                `json:"objective"`
	RunID                   string                                `json:"run_id,omitempty"`
	EpicID                  string                                `json:"epic_id,omitempty"`
	BeadID                  string                                `json:"bead_id,omitempty"`
	TrackingRepoRoot        string                                `json:"tracking_repo_root,omitempty"`
	BeadsDir                string                                `json:"beads_dir,omitempty"`
	PRURL                   string                                `json:"pr_url,omitempty"`
	MergeCommit             string                                `json:"merge_commit,omitempty"`
	PlanPath                string                                `json:"plan_path,omitempty"`
	Density                 *ExecutionPacketDensity               `json:"density,omitempty"`
	Artifacts               *ExecutionPacketArtifacts             `json:"artifacts,omitempty"`
	ContractSurfaces        []string                              `json:"contract_surfaces,omitempty"`
	ValidationCommands      []string                              `json:"validation_commands,omitempty"`
	ValidationLanes         []ValidationLane                      `json:"validation_lanes,omitempty"`
	TrackerMode             string                                `json:"tracker_mode,omitempty"`
	TrackerHealth           map[string]any                        `json:"tracker_health,omitempty"`
	DoneCriteria            []string                              `json:"done_criteria,omitempty"`
	EpicCriteria            []Criterion                           `json:"epic_criteria,omitempty"`
	BeadCriteria            map[string][]Criterion                `json:"bead_criteria,omitempty"`
	Routing                 map[string]ExecutionPacketRouting     `json:"routing,omitempty"`
	Spec                    *ExecutionPacketSpec                  `json:"spec,omitempty"`
	Complexity              Complexity                            `json:"complexity,omitempty"`
	PreMortemVerdict        ExecutionPacketVerdict                `json:"pre_mortem_verdict,omitempty"`
	DefaultVerdict          ExecutionPacketVerdict                `json:"default_verdict,omitempty"`
	TestLevels              *ExecutionPacketTestLevels            `json:"test_levels,omitempty"`
	RankedPacketPath        string                                `json:"ranked_packet_path,omitempty"`
	DiscoveryTimestamp      string                                `json:"discovery_timestamp,omitempty"`
	ProofArtifacts          []string                              `json:"proof_artifacts,omitempty"`
	EvaluatorArtifacts      map[string]string                     `json:"evaluator_artifacts,omitempty"`
	ProofUpdatedAt          string                                `json:"proof_updated_at,omitempty"`
	AutodevProgram          *ExecutionPacketProgram               `json:"autodev_program,omitempty"`
	MixedModeRequested      bool                                  `json:"mixed_mode_requested,omitempty"`
	MixedModeEffective      bool                                  `json:"mixed_mode_effective,omitempty"`
	PlannerVendor           string                                `json:"planner_vendor,omitempty"`
	ReviewerVendor          string                                `json:"reviewer_vendor,omitempty"`
	MixedModeDegradedReason string                                `json:"mixed_mode_degraded_reason,omitempty"`
	OrchestrationDecision   *ExecutionPacketOrchestrationDecision `json:"orchestration_decision,omitempty"`
	Phase                   string                                `json:"phase,omitempty"`
	Source                  string                                `json:"source,omitempty"`
	DiscoveryArtifacts      []string                              `json:"discovery_artifacts,omitempty"`
	Scope                   *DiscoveryArtifactScope               `json:"scope,omitempty"`
	AbortGates              []string                              `json:"abort_gates,omitempty"`
	TDDMatrix               []string                              `json:"tdd_matrix,omitempty"`
	Risks                   []string                              `json:"risks,omitempty"`
	GeneratedAt             string                                `json:"generated_at,omitempty"`
	Issues                  []ExecutionPacketIssue                `json:"issues,omitempty"`
	DoneWhen                []string                              `json:"done_when,omitempty"`
	LikelyBlocker           string                                `json:"likely_blocker,omitempty"`
	IgnoreToday             []string                              `json:"ignore_today,omitempty"`
}

// ExecutionPacketIssue is a slim-packet bead entry (the issues[] array emitted
// by ao rpi): id/title plus optional wave and blocked_by dependency edges.
type ExecutionPacketIssue struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Wave      int      `json:"wave,omitempty"`
	BlockedBy []string `json:"blocked_by,omitempty"`
}

// ExecutionPacketProgram describes an autodev program embedded in an execution packet.
type ExecutionPacketProgram struct {
	Path               string   `json:"path,omitempty"`
	MutableScope       []string `json:"mutable_scope,omitempty"`
	ImmutableScope     []string `json:"immutable_scope,omitempty"`
	ExperimentUnit     string   `json:"experiment_unit,omitempty"`
	ValidationCommands []string `json:"validation_commands,omitempty"`
	DecisionPolicy     []string `json:"decision_policy,omitempty"`
	StopConditions     []string `json:"stop_conditions,omitempty"`
}

// ExecutionPacketModelFamily is the rostered model family label used for typed
// per-bead routing inside an execution packet.
type ExecutionPacketModelFamily string

const (
	ExecutionPacketModelClaude ExecutionPacketModelFamily = "claude"
	ExecutionPacketModelCodex  ExecutionPacketModelFamily = "codex"
	ExecutionPacketModelGemini ExecutionPacketModelFamily = "gemini"
)

// ExecutionPacketVerdict is the packet verdict vocabulary. The packet defaults
// to FAIL when no explicit verdict is present.
type ExecutionPacketVerdict string

const (
	ExecutionPacketVerdictPass ExecutionPacketVerdict = "PASS"
	ExecutionPacketVerdictWarn ExecutionPacketVerdict = "WARN"
	ExecutionPacketVerdictFail ExecutionPacketVerdict = "FAIL"

	DefaultExecutionPacketVerdict = ExecutionPacketVerdictFail
)

// Back-compat aliases for the former slim aggregate names.
type Verdict = ExecutionPacketVerdict

const (
	VerdictPass    = ExecutionPacketVerdictPass
	VerdictWarn    = ExecutionPacketVerdictWarn
	VerdictFail    = ExecutionPacketVerdictFail
	DefaultVerdict = DefaultExecutionPacketVerdict
)

// EffectiveVerdict is the canonical packet verdict read. It is the last-line
// fail-closed guard for packets that bypass schema validation (legacy or
// on-disk packets the schema enum never vetted): anything that is not exactly
// PASS, WARN, or FAIL — absent, empty, or malformed — resolves to the
// fail-closed default rather than passing an unvetted value through.
func (p ExecutionPacket) EffectiveVerdict() ExecutionPacketVerdict {
	switch p.DefaultVerdict {
	case ExecutionPacketVerdictPass, ExecutionPacketVerdictWarn, ExecutionPacketVerdictFail:
		return p.DefaultVerdict
	default:
		return DefaultExecutionPacketVerdict
	}
}

// ExecutionPacketRouting records the typed implementer/reviewer assignment for
// a bead or slice. The packet stores these values keyed by bead ID.
type ExecutionPacketRouting struct {
	Implementer ExecutionPacketModelFamily `json:"implementer"`
	Reviewer    ExecutionPacketModelFamily `json:"reviewer"`
	Rationale   string                     `json:"rationale"`
}

// ExecutionPacketSpec links the work packet to the behavior-first red test that
// encodes the implementation contract.
type ExecutionPacketSpec struct {
	TestPath string `json:"test_path"`
	RedTest  string `json:"red_test"`
}

// Criterion is a single acceptance criterion attached to an epic or bead in an
// execution packet.
type Criterion struct {
	ID               string  `json:"id"`
	Description      string  `json:"description"`
	CheckType        string  `json:"check_type"`
	CheckCommand     string  `json:"check_command,omitempty"`
	EvidencePath     string  `json:"evidence_path,omitempty"`
	EvidenceRequired bool    `json:"evidence_required"`
	Weight           float64 `json:"weight"`
	Optional         bool    `json:"optional"`
	AgentJudge       string  `json:"agent_judge,omitempty"`
}

// ValidationLane carries repo execution profile validation metadata through
// RPI packets while preserving the legacy validation_commands list.
type ValidationLane struct {
	Name                string   `json:"name,omitempty"`
	Command             string   `json:"command,omitempty"`
	Purpose             string   `json:"purpose,omitempty"`
	ReadOnly            bool     `json:"read_only"`
	WritesArtifacts     bool     `json:"writes_artifacts"`
	ArtifactPaths       []string `json:"artifact_paths,omitempty"`
	IsolatedAgentsHome  bool     `json:"isolated_agents_home"`
	ReleaseOnly         bool     `json:"release_only"`
	MutationEscapeHatch *string  `json:"mutation_escape_hatch,omitempty"`
	CostClass           string   `json:"cost_class,omitempty"`
	AutoSelect          string   `json:"auto_select,omitempty"`
	TimeoutSeconds      int      `json:"timeout_seconds,omitempty"`
	ExpensiveReason     string   `json:"expensive_reason,omitempty"`
}

// ExecutionPacketDensity carries the dense phase-boundary context that
// discovery passes to implementation without copying raw research or plan prose.
type ExecutionPacketDensity struct {
	Intent     string                  `json:"intent"`
	Boundary   ExecutionPacketBoundary `json:"boundary"`
	Evidence   []string                `json:"evidence"`
	Decision   string                  `json:"decision"`
	Constraint []string                `json:"constraint"`
	NextAction string                  `json:"next_action"`
}

// ExecutionPacketBoundary describes the work boundary for the next phase.
type ExecutionPacketBoundary struct {
	BoundedContext       string   `json:"bounded_context"`
	NonGoals             []string `json:"non_goals"`
	WriteScope           []string `json:"write_scope,omitempty"`
	WriteScopeFirstSlice []string `json:"write_scope_first_slice,omitempty"`
}

// ExecutionPacketArtifacts links the compact packet to larger durable
// artifacts.
type ExecutionPacketArtifacts struct {
	ResearchPath         string   `json:"research_path,omitempty"`
	PlanPath             string   `json:"plan_path,omitempty"`
	PreMortemPath        string   `json:"pre_mortem_path,omitempty"`
	RankedPacketPath     string   `json:"ranked_packet_path,omitempty"`
	PerspectivePlanPaths []string `json:"perspective_plan_paths,omitempty"`
	SynthesisPacketPath  string   `json:"synthesis_packet_path,omitempty"`
	FableApprovalPath    string   `json:"fable_approval_path,omitempty"`
	ApprovalEdgePath     string   `json:"approval_edge_path,omitempty"`
}

// ExecutionPacketTestLevels records the test pyramid levels selected for the
// handoff. Required levels are the minimum autonomous proof floor.
type ExecutionPacketTestLevels struct {
	Required    []TestLevel `json:"required"`
	Recommended []TestLevel `json:"recommended"`
	Rationale   string      `json:"rationale"`
}

type TestLevel string

const (
	L0 TestLevel = "L0"
	L1 TestLevel = "L1"
	L2 TestLevel = "L2"
	L3 TestLevel = "L3"
)

type Complexity string

const (
	ComplexityFast     Complexity = "fast"
	ComplexityStandard Complexity = "standard"
	ComplexityFull     Complexity = "full"
)

type ExecutionPacketOrchestrationDecision struct {
	ChosenShape     string   `json:"chosen_shape"`
	PredicatesFired []string `json:"predicates_fired,omitempty"`
	Rationale       string   `json:"rationale,omitempty"`
	Timestamp       string   `json:"ts,omitempty"`
}

type DiscoveryArtifactScope struct {
	InScope     []string `json:"in_scope,omitempty"`
	OutOfScope  []string `json:"out_of_scope,omitempty"`
	LOCEstimate int      `json:"loc_estimate,omitempty"`
}
