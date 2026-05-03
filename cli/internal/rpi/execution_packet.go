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
