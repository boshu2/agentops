// practices: [design-by-contract, ddd-bounded-context]
package main

// Codex task-packet + run-receipt schema types. Extracted from codex.go so they
// remain in the spine (default build) even though the codex lifecycle COMMAND is
// archived behind //go:build legacy (ADR-0012, age-h4y3): the spine's
// `ao converge` builds a non-mutating judge packet (codexTaskPacket) and verifies
// the returned receipt (codexRunReceipt) to enforce author_neq_validator. These
// are pure data contracts — no dependency on the archived command surface — so
// they must live in an UNTAGGED file or the spine build breaks with "undefined".

type codexTaskPacket struct {
	SchemaVersion    int                       `json:"schema_version"`
	PacketID         string                    `json:"packet_id"`
	CreatedAt        string                    `json:"created_at"`
	Objective        string                    `json:"objective"`
	Role             string                    `json:"role"`
	CWD              string                    `json:"cwd"`
	AllowedPaths     []string                  `json:"allowed_paths,omitempty"`
	ForbiddenActions []string                  `json:"forbidden_actions,omitempty"`
	Sandbox          string                    `json:"sandbox"`
	Auth             codexTaskAuthGuard        `json:"auth"`
	Dispatch         codexTaskDispatchPolicy   `json:"dispatch"`
	Execution        codexTaskExecution        `json:"execution"`
	Output           codexTaskOutputContract   `json:"output"`
	Evidence         codexTaskEvidenceContract `json:"evidence"`
	Resume           *codexTaskResume          `json:"resume,omitempty"`
	StopCondition    string                    `json:"stop_condition"`
	Notes            []string                  `json:"notes,omitempty"`
	// AuthorIdentity carries the identity of the agent that AUTHORED the work
	// being judged, so the author_neq_validator asymmetry can be enforced on the
	// dispatched judge (the judge must be a different identity than the author).
	AuthorIdentity string `json:"author_identity,omitempty"`
}

// AuthorID returns the identity of the work author carried by the packet (used by
// the converge author_neq_validator guard).
func (p codexTaskPacket) AuthorID() string {
	return p.AuthorIdentity
}

type codexTaskAuthGuard struct {
	RequiredMode           string   `json:"required_mode"`
	RejectEnv              []string `json:"reject_env"`
	LoginStatusMustContain string   `json:"login_status_must_contain"`
	ForbidAPIKey           bool     `json:"forbid_api_key"`
}

type codexTaskDispatchPolicy struct {
	Mode        string   `json:"mode"`
	MutatesRepo bool     `json:"mutates_repo"`
	Command     []string `json:"command"`
	Notes       string   `json:"notes,omitempty"`
}

type codexTaskExecution struct {
	Argv             []string          `json:"argv"`
	Stdin            codexTaskStdin    `json:"stdin"`
	TimeoutSeconds   int               `json:"timeout_seconds"`
	PromptPath       string            `json:"prompt_path,omitempty"`
	OutputSchemaPath string            `json:"output_schema_path,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
}

type codexTaskStdin struct {
	Mode             string `json:"mode"`
	CloseAfterPrompt bool   `json:"close_after_prompt"`
}

type codexTaskOutputContract struct {
	CaptureMode      string `json:"capture_mode"`
	FinalMessagePath string `json:"final_message_path"`
	JSONLPath        string `json:"jsonl_path,omitempty"`
	SchemaPath       string `json:"schema_path,omitempty"`
	ReceiptPath      string `json:"receipt_path"`
}

type codexTaskEvidenceContract struct {
	ReceiptPath      string   `json:"receipt_path"`
	RequiredCommands []string `json:"required_commands"`
	Artifacts        []string `json:"artifacts,omitempty"`
}

type codexTaskResume struct {
	Policy      string `json:"policy"`
	SessionID   string `json:"session_id,omitempty"`
	AllowResume bool   `json:"allow_resume"`
}

type codexRunReceipt struct {
	SchemaVersion     int                  `json:"schema_version"`
	ReceiptID         string               `json:"receipt_id"`
	PacketID          string               `json:"packet_id"`
	CodexSessionID    string               `json:"codex_session_id,omitempty"`
	StartedAt         string               `json:"started_at"`
	EndedAt           string               `json:"ended_at"`
	CWD               string               `json:"cwd"`
	Sandbox           string               `json:"sandbox"`
	AuthMode          string               `json:"auth_mode"`
	AuthStatus        string               `json:"auth_status"`
	Command           codexReceiptCommand  `json:"command"`
	Stdin             codexReceiptStdin    `json:"stdin"`
	TimeoutSeconds    int                  `json:"timeout_seconds"`
	TimedOut          bool                 `json:"timed_out"`
	ExitCode          int                  `json:"exit_code"`
	Outputs           codexReceiptOutputs  `json:"outputs"`
	ChangedFiles      []string             `json:"changed_files"`
	CommandsRun       []codexCommandResult `json:"commands_run"`
	Verdict           codexReceiptVerdict  `json:"verdict"`
	ResumeFromSession string               `json:"resume_from_session,omitempty"`
	Evidence          []codexEvidenceRef   `json:"evidence,omitempty"`
	FailureReason     string               `json:"failure_reason,omitempty"`
}

type codexReceiptCommand struct {
	Argv []string `json:"argv"`
}

type codexReceiptStdin struct {
	Mode         string `json:"mode"`
	ClosedAt     string `json:"closed_at"`
	BytesWritten int    `json:"bytes_written"`
}

type codexReceiptOutputs struct {
	FinalMessagePath string `json:"final_message_path"`
	JSONLPath        string `json:"jsonl_path,omitempty"`
	SchemaPath       string `json:"schema_path,omitempty"`
	ReceiptPath      string `json:"receipt_path"`
}

type codexCommandResult struct {
	Command       string `json:"command"`
	ExitCode      int    `json:"exit_code"`
	OutputExcerpt string `json:"output_excerpt,omitempty"`
}

type codexReceiptVerdict struct {
	Status           string `json:"status"`
	JudgeSource      string `json:"judge_source"`
	Summary          string `json:"summary"`
	AuthorID         string `json:"author_id,omitempty"`
	JudgeName        string `json:"judge_name,omitempty"`
	JudgeProgram     string `json:"judge_program,omitempty"`
	JudgeModelFamily string `json:"judge_model_family,omitempty"`
}

type codexEvidenceRef struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}
