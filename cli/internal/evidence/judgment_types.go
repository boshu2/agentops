package evidence

import "github.com/boshu2/agentops/cli/internal/parser"

// JudgeProfile is supplied independently by the consumer, never selected by a
// verdict or receipt. An empty effort imposes no actual-effort requirement;
// a nonempty effort requires runtime reporting, not a launch-option echo.
type JudgeProfile struct {
	ID      string `json:"id"`
	Runtime string `json:"runtime"`
	Model   string `json:"model"`
	Family  string `json:"family"`
	Effort  string `json:"effort"`
}

// TranscriptBinding points to exact native invocation bytes. SHA256 covers the
// selected [Start,End) span without normalization, not reconstructed JSON.
type TranscriptBinding struct {
	Path   string `json:"path"`
	Start  int64  `json:"start"`
	End    int64  `json:"end"`
	SHA256 string `json:"sha256"`
}

// JudgmentReceipt is an external, caller/runtime-authored receipt referenced by
// verdict.v2 evidence_refs. It adds no fields to verdict.v2. The verifier
// independently parses model/context/termination from Transcript; it never
// treats Requested or a model-authored identity claim as an actual identity.
type JudgmentReceipt struct {
	Version               string            `json:"version"`
	Requested             JudgeProfile      `json:"requested"`
	SubjectManifestDigest string            `json:"subject_manifest_digest"`
	AcceptanceDigest      string            `json:"acceptance_digest"`
	AuthorContextID       string            `json:"author_context_id"`
	Transcript            TranscriptBinding `json:"transcript"`
	ExitCode              *int              `json:"exit_code"`
	TimedOut              *bool             `json:"timed_out"`
	Truncated             *bool             `json:"truncated"`
	CleanupVerified       *bool             `json:"cleanup_verified"`
	Omissions             []string          `json:"omissions"`
}

// JudgmentOptions separates the consumer's expected subject, immutable intent,
// author, required profiles and provider access policy from candidate evidence.
// The helper is a local reader: it never transmits bytes or authorizes a model
// launch. The invoking runtime must enforce disclosure policy before dispatch.
type JudgmentOptions struct {
	Root            string
	EvidenceRoot    string
	Manifest        string
	BaseManifest    string
	Intent          string
	AuthorContextID string
	Required        []JudgeProfile
	// RequiredCriteria is the caller's complete acceptance ID set. Nil retains
	// legacy provenance behavior; a selected set must be nonempty and unique.
	// IDs are never derived from candidate verdicts or parsed from intent prose.
	RequiredCriteria []string
	AllowedProviders []string
	Verdicts         []string
}

type JudgmentLeg struct {
	ID          string                  `json:"id"`
	Satisfied   bool                    `json:"satisfied"`
	VerdictPath string                  `json:"verdict_path,omitempty"`
	Verdict     string                  `json:"verdict,omitempty"`
	ReceiptRef  string                  `json:"receipt_ref,omitempty"`
	Native      *parser.RuntimeMetadata `json:"native,omitempty"`
	Problems    []string                `json:"problems"`
}

// JudgmentResult describes mechanical coverage only. All original verdicts and
// their evidence files remain unchanged; Satisfied is not a new semantic PASS.
type JudgmentResult struct {
	Satisfied             bool          `json:"satisfied"`
	SubjectManifestDigest string        `json:"subject_manifest_digest"`
	AcceptanceDigest      string        `json:"acceptance_digest"`
	RequiredCriteria      []string      `json:"required_criteria,omitempty"`
	Legs                  []JudgmentLeg `json:"legs"`
	Problems              []string      `json:"problems"`
}
