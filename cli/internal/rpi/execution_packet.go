package rpi

// ExecutionPacketFile is the canonical filename for execution packets.
const ExecutionPacketFile = "execution-packet.json"

// ExecutionPacket is the typed workpacket projection used by RPI code. The
// domain packet aggregate carries the slim persisted form.
type ExecutionPacket struct {
	Routing        map[string]ExecutionPacketRouting `json:"routing,omitempty"`
	DefaultVerdict ExecutionPacketVerdict            `json:"default_verdict,omitempty"`
	Spec           ExecutionPacketSpec               `json:"spec,omitempty"`
}

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

// EffectiveVerdict is the canonical packet verdict read: missing
// default_verdict resolves fail-closed instead of relying on schema default
// annotations.
func (p ExecutionPacket) EffectiveVerdict() ExecutionPacketVerdict {
	switch p.DefaultVerdict {
	case ExecutionPacketVerdictPass, ExecutionPacketVerdictWarn, ExecutionPacketVerdictFail:
		return p.DefaultVerdict
	default:
		// Absent OR unrecognized/malformed -> fail-closed to FAIL. Since .1 adds no
		// schema-on-load, a loaded packet may carry a junk default_verdict; never treat
		// an unvalidated value as authoritative.
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
	BoundedContext string   `json:"bounded_context"`
	NonGoals       []string `json:"non_goals"`
	WriteScope     []string `json:"write_scope"`
}

// ExecutionPacketArtifacts links the compact packet to larger durable
// artifacts. Empty paths are omitted so early seed packets can remain valid
// before discovery has produced every artifact.
type ExecutionPacketArtifacts struct {
	ResearchPath     string `json:"research_path,omitempty"`
	PlanPath         string `json:"plan_path,omitempty"`
	PreMortemPath    string `json:"pre_mortem_path,omitempty"`
	RankedPacketPath string `json:"ranked_packet_path,omitempty"`
}

// ExecutionPacketTestLevels records the test pyramid levels selected for the
// handoff. Required levels are the minimum autonomous proof floor; recommended
// levels are advisory unless a bead acceptance criterion names them.
type ExecutionPacketTestLevels struct {
	Required    []string `json:"required"`
	Recommended []string `json:"recommended"`
	Rationale   string   `json:"rationale"`
}
