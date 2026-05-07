package rpi

// ExecutionPacketFile is the canonical filename for execution packets.
const ExecutionPacketFile = "execution-packet.json"

// ExecutionPacketProgram describes an autodev program embedded in an execution packet.
type ExecutionPacketProgram struct {
	Path               string   `json:"path"`
	MutableScope       []string `json:"mutable_scope,omitempty"`
	ImmutableScope     []string `json:"immutable_scope,omitempty"`
	ExperimentUnit     string   `json:"experiment_unit,omitempty"`
	ValidationCommands []string `json:"validation_commands,omitempty"`
	DecisionPolicy     []string `json:"decision_policy,omitempty"`
	StopConditions     []string `json:"stop_conditions,omitempty"`
}

// Criterion is a single acceptance criterion attached to an epic or bead in an
// execution packet. CheckType is a closed enum:
//
//   - test_pass
//   - command_exit_zero
//   - file_exists
//   - grep_match
//   - manual
//   - council_judge
//   - custom_rubric
//
// When CheckType == "custom_rubric", AgentJudge MUST be a non-empty string
// naming the council or judge that owns the verdict.
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
	Name                string   `json:"name"`
	Command             string   `json:"command"`
	Purpose             string   `json:"purpose,omitempty"`
	ReadOnly            bool     `json:"read_only"`
	WritesArtifacts     bool     `json:"writes_artifacts"`
	ArtifactPaths       []string `json:"artifact_paths,omitempty"`
	IsolatedAgentsHome  bool     `json:"isolated_agents_home"`
	ReleaseOnly         bool     `json:"release_only"`
	MutationEscapeHatch *string  `json:"mutation_escape_hatch"`
	CostClass           string   `json:"cost_class,omitempty"`
	AutoSelect          string   `json:"auto_select,omitempty"`
	TimeoutSeconds      int      `json:"timeout_seconds,omitempty"`
	ExpensiveReason     string   `json:"expensive_reason,omitempty"`
}
