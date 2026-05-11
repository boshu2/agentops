package eval

import "time"

type Tier string

const (
	TierDeterministic Tier = "deterministic"
	TierHeadless      Tier = "headless"
	TierLive          Tier = "live"
	TierRelease       Tier = "release"
)

type Visibility string

const (
	VisibilityPublicCanary   Visibility = "public_canary"
	VisibilityPrivateHoldout Visibility = "private_holdout"
)

type Runtime string

const (
	RuntimeStatic Runtime = "static"
	RuntimeMock   Runtime = "mock"
	RuntimeShell  Runtime = "shell"
	RuntimeClaude Runtime = "claude"
	RuntimeCodex  Runtime = "codex"
	RuntimeManual Runtime = "manual"
)

type EvidenceKind string

const (
	EvidenceKindContractCanary     EvidenceKind = "contract_canary"
	EvidenceKindGateWrapper        EvidenceKind = "gate_wrapper"
	EvidenceKindBehaviorFixture    EvidenceKind = "behavior_fixture"
	EvidenceKindBaselineRegression EvidenceKind = "baseline_regression"
	EvidenceKindScorecardFixture   EvidenceKind = "scorecard_fixture"
	EvidenceKindLiveRuntime        EvidenceKind = "live_runtime"
	EvidenceKindHoldout            EvidenceKind = "holdout"
)

type Dimension string

const (
	DimensionCorrectness          Dimension = "correctness"
	DimensionProcessAdherence     Dimension = "process_adherence"
	DimensionArtifactQuality      Dimension = "artifact_quality"
	DimensionRuntimeCompatibility Dimension = "runtime_compatibility"
	DimensionEfficiency           Dimension = "efficiency"
	DimensionSafety               Dimension = "safety"
	DimensionLearningClosure        Dimension = "learning_closure"
	DimensionContextComprehension   Dimension = "context_comprehension"
)

type Status string

const (
	StatusPass         Status = "pass"
	StatusFail         Status = "fail"
	StatusError        Status = "error"
	StatusSkipped      Status = "skipped"
	StatusInconclusive Status = "inconclusive"
)

type Verdict string

const (
	VerdictPass         Verdict = "pass"
	VerdictFail         Verdict = "fail"
	VerdictImprovement  Verdict = "improvement"
	VerdictRegression   Verdict = "regression"
	VerdictAdvisory     Verdict = "advisory"
	VerdictInconclusive Verdict = "inconclusive"
)

type BaselineMode string

const (
	BaselineModeNone    BaselineMode = "none"
	BaselineModeCompare BaselineMode = "compare"
	BaselineModePromote BaselineMode = "promote"
)

type NetworkAccess string

const (
	NetworkDisabled NetworkAccess = "disabled"
	NetworkEnabled  NetworkAccess = "enabled"
	NetworkUnknown  NetworkAccess = "unknown"
)

type ContextVariant string

const (
	ContextVariantOff ContextVariant = "context_off"
	ContextVariantOn  ContextVariant = "context_on"
)

type ContextAttribution string

const (
	ContextAttributionRetrieved    ContextAttribution = "retrieved"
	ContextAttributionApplied      ContextAttribution = "applied"
	ContextAttributionContradicted ContextAttribution = "contradicted"
	ContextAttributionIgnored      ContextAttribution = "ignored"
	ContextAttributionHelpful      ContextAttribution = "helpful"
	ContextAttributionHarmful      ContextAttribution = "harmful"
)

type Suite struct {
	SchemaVersion  int              `json:"schema_version"`
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Description    string           `json:"description,omitempty"`
	Practices      []string         `json:"practices,omitempty"`
	Domain         string           `json:"domain"`
	Visibility     Visibility       `json:"visibility"`
	Tier           Tier             `json:"tier"`
	EvidenceKind   EvidenceKind     `json:"evidence_kind,omitempty"`
	Owners         []string         `json:"owners,omitempty"`
	Tags           []string         `json:"tags,omitempty"`
	Allowed        []Runtime        `json:"allowed_runtimes,omitempty"`
	Environment    SuiteEnvironment `json:"environment,omitempty"`
	Fixtures       []Fixture        `json:"fixtures,omitempty"`
	Scoring        Scoring          `json:"scoring"`
	BaselinePolicy BaselinePolicy   `json:"baseline_policy"`
	Cases          []Case           `json:"cases"`
}

type SuiteEnvironment struct {
	OfflineRequired  bool     `json:"offline_required,omitempty"`
	Network          string   `json:"network,omitempty"`
	ScrubEnvPrefixes []string `json:"scrub_env_prefixes,omitempty"`
	IsolateHome      bool     `json:"isolate_home,omitempty"`
	IsolateCodexHome bool     `json:"isolate_codex_home,omitempty"`
	TimeoutSeconds   int      `json:"timeout_seconds,omitempty"`
	MaxAttempts      int      `json:"max_attempts,omitempty"`
	DisableHooks     bool     `json:"disable_hooks,omitempty"`
}

type Fixture struct {
	Path     string `json:"path"`
	Purpose  string `json:"purpose"`
	Required bool   `json:"required,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

type Scoring struct {
	AggregateThreshold float64            `json:"aggregate_threshold"`
	CriticalThreshold  *float64           `json:"critical_threshold,omitempty"`
	Dimensions         []ScoringDimension `json:"dimensions"`
}

type ScoringDimension struct {
	Name      Dimension `json:"name"`
	Weight    float64   `json:"weight"`
	Threshold float64   `json:"threshold"`
	Critical  bool      `json:"critical,omitempty"`
}

type BaselinePolicy struct {
	Mode                   string   `json:"mode"`
	BaselineRef            string   `json:"baseline_ref,omitempty"`
	BaselinePath           string   `json:"baseline_path,omitempty"`
	MaxAggregateRegression *float64 `json:"max_aggregate_regression,omitempty"`
	MaxDimensionRegression *float64 `json:"max_dimension_regression,omitempty"`
	RepeatCount            int      `json:"repeat_count,omitempty"`
	PromotionRequires      []string `json:"promotion_requires,omitempty"`
	BlockingGate           string   `json:"blocking_gate,omitempty"`
}

type Case struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	Kind           string         `json:"kind"`
	Objective      string         `json:"objective"`
	EvidenceKind   EvidenceKind   `json:"evidence_kind,omitempty"`
	Runtime        Runtime        `json:"runtime,omitempty"`
	TimeoutSeconds int            `json:"timeout_seconds,omitempty"`
	Fixtures       []string       `json:"fixtures,omitempty"`
	Inputs         map[string]any `json:"inputs,omitempty"`
	Expectations   []Expectation  `json:"expectations"`
	Dimensions     []Dimension    `json:"dimensions,omitempty"`
	Critical       bool           `json:"critical,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
}

type Expectation struct {
	Type      string   `json:"type"`
	Target    string   `json:"target,omitempty"`
	Value     any      `json:"value,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`
	Required  *bool    `json:"required,omitempty"`
}

type SuiteRef struct {
	ID         string     `json:"id"`
	Path       string     `json:"path"`
	Visibility Visibility `json:"visibility"`
	Tier       Tier       `json:"tier"`
	Version    string     `json:"version,omitempty"`
	SHA256     string     `json:"sha256,omitempty"`
}

type GitRecord struct {
	CandidateRef string   `json:"candidate_ref"`
	CandidateSHA string   `json:"candidate_sha"`
	BaselineRef  string   `json:"baseline_ref,omitempty"`
	BaselineSHA  string   `json:"baseline_sha,omitempty"`
	Dirty        bool     `json:"dirty"`
	DirtyPaths   []string `json:"dirty_paths,omitempty"`
}

type RuntimeRecord struct {
	Name           Runtime `json:"name"`
	Version        string  `json:"version,omitempty"`
	Model          string  `json:"model,omitempty"`
	Profile        string  `json:"profile,omitempty"`
	Live           bool    `json:"live"`
	Attempts       int     `json:"attempts,omitempty"`
	TimeoutSeconds int     `json:"timeout_seconds,omitempty"`
	SkippedReason  string  `json:"skipped_reason,omitempty"`
}

type EnvironmentRecord struct {
	ScrubbedEnvPrefixes []string      `json:"scrubbed_env_prefixes"`
	IsolatedHome        bool          `json:"isolated_home"`
	IsolatedCodexHome   bool          `json:"isolated_codex_home"`
	NetworkAccess       NetworkAccess `json:"network_access"`
	HooksDisabled       bool          `json:"hooks_disabled,omitempty"`
	HostNotes           []string      `json:"host_notes,omitempty"`
}

type BaselineRecord struct {
	Mode              BaselineMode `json:"mode"`
	BaselineRunID     string       `json:"baseline_run_id,omitempty"`
	BaselineRef       string       `json:"baseline_ref,omitempty"`
	BaselinePath      string       `json:"baseline_path,omitempty"`
	PromotedFromRunID string       `json:"promoted_from_run_id,omitempty"`
	PromotedAt        *time.Time   `json:"promoted_at,omitempty"`
	PromotedBy        string       `json:"promoted_by,omitempty"`
	Rationale         string       `json:"rationale,omitempty"`
}

type ComparisonItem struct {
	CaseID    string    `json:"case_id,omitempty"`
	Dimension Dimension `json:"dimension"`
	Delta     float64   `json:"delta"`
	Reason    string    `json:"reason"`
}

type BaselineComparison struct {
	Verdict        Verdict               `json:"verdict"`
	BaselineRunID  string                `json:"baseline_run_id,omitempty"`
	BaselineScore  float64               `json:"baseline_score,omitempty"`
	AggregateDelta float64               `json:"aggregate_delta"`
	DimensionDelta map[Dimension]float64 `json:"dimension_deltas,omitempty"`
	Regressions    []ComparisonItem      `json:"regressions,omitempty"`
	Improvements   []ComparisonItem      `json:"improvements,omitempty"`
}

type Artifact struct {
	Path    string `json:"path"`
	Purpose string `json:"purpose"`
	Kind    string `json:"kind,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
}

type CaseResult struct {
	ID              string                `json:"id"`
	Status          Status                `json:"status"`
	Score           float64               `json:"score"`
	DimensionScores map[Dimension]float64 `json:"dimension_scores"`
	DurationMS      int64                 `json:"duration_ms,omitempty"`
	Critical        bool                  `json:"critical,omitempty"`
	Artifacts       []Artifact            `json:"artifacts,omitempty"`
	FailureMessage  string                `json:"failure_message,omitempty"`
	Diagnostics     []string              `json:"diagnostics,omitempty"`
}

type ContextVariantRun struct {
	Variant          ContextVariant `json:"variant"`
	ContextRootLabel string         `json:"context_root_label"`
	RunID            string         `json:"run_id"`
	AggregateScore   float64        `json:"aggregate_score"`
	Status           Status         `json:"status,omitempty"`
}

type ContextCaseVariantResult struct {
	Variant          ContextVariant `json:"variant"`
	ContextRootLabel string         `json:"context_root_label"`
	RunID            string         `json:"run_id"`
	Status           Status         `json:"status"`
	Score            float64        `json:"score"`
}

type ContextEvidence struct {
	Summary  string `json:"summary"`
	Artifact string `json:"artifact,omitempty"`
	Excerpt  string `json:"excerpt,omitempty"`
}

type ContextArtifactAttribution struct {
	Artifact    string             `json:"artifact"`
	Attribution ContextAttribution `json:"attribution"`
	Evidence    string             `json:"evidence,omitempty"`
}

type ContextCaseDelta struct {
	CaseID                 string                       `json:"case_id"`
	ContextOff             ContextCaseVariantResult     `json:"context_off"`
	ContextOn              ContextCaseVariantResult     `json:"context_on"`
	ScoreDelta             float64                      `json:"score_delta"`
	StatusDelta            int                          `json:"status_delta"`
	TokenDelta             *int                         `json:"token_delta,omitempty"`
	ToolCallDelta          *int                         `json:"tool_call_delta,omitempty"`
	DecisionEvidence       []ContextEvidence            `json:"decision_evidence,omitempty"`
	IgnoredContextEvidence []ContextEvidence            `json:"ignored_context_evidence,omitempty"`
	DegradedReason         string                       `json:"degraded_reason,omitempty"`
	ArtifactAttribution    []ContextArtifactAttribution `json:"artifact_attribution,omitempty"`
}

type ContextDeltaScorecard struct {
	SchemaVersion  int                `json:"schema_version"`
	SuiteID        string             `json:"suite_id"`
	SuitePath      string             `json:"suite_path"`
	GeneratedAt    time.Time          `json:"generated_at"`
	ContextOff     ContextVariantRun  `json:"context_off"`
	ContextOn      ContextVariantRun  `json:"context_on"`
	AggregateDelta float64            `json:"aggregate_delta"`
	PerCase        []ContextCaseDelta `json:"per_case"`
}

type RunRecord struct {
	SchemaVersion      int                   `json:"schema_version"`
	RunID              string                `json:"run_id"`
	Suite              SuiteRef              `json:"suite"`
	StartedAt          time.Time             `json:"started_at"`
	CompletedAt        *time.Time            `json:"completed_at,omitempty"`
	Status             Status                `json:"status"`
	Verdict            Verdict               `json:"verdict"`
	Git                GitRecord             `json:"git"`
	Runtime            RuntimeRecord         `json:"runtime"`
	Environment        EnvironmentRecord     `json:"environment"`
	Baseline           *BaselineRecord       `json:"baseline,omitempty"`
	CaseResults        []CaseResult          `json:"case_results"`
	AggregateScore     float64               `json:"aggregate_score"`
	DimensionScores    map[Dimension]float64 `json:"dimension_scores"`
	BaselineComparison *BaselineComparison   `json:"baseline_comparison,omitempty"`
	Artifacts          []Artifact            `json:"artifacts,omitempty"`
	Notes              []string              `json:"notes,omitempty"`
}

type RunOptions struct {
	SuitePath    string
	RunID        string
	Runtime      Runtime
	OutputPath   string
	BaselinePath string
	WorkDir      string
	Now          func() time.Time
	// Env injects run-scoped variables into deterministic command cases.
	// Context A/B uses this to set AO_AGENTS_DIR per leg without mutating
	// process-global environment.
	Env map[string]string
	// OverrideDisableHooks forces the run to behave as if the loaded suite
	// declared Environment.DisableHooks=true, without mutating the suite
	// file. Used by RunBaselineAB to pair a single suite into skill-on and
	// skill-off runs.
	OverrideDisableHooks bool
}

type CompareOptions struct {
	MaxAggregateRegression float64
	MaxDimensionRegression float64
	OutputPath             string
}

type BaselineOptions struct {
	OutputPath string
	PromotedBy string
	Rationale  string
	WorkDir    string
	Now        func() time.Time
}
