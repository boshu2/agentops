package ports

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// FindingArtifact is the input to FindingCompilerPort.Compile. ID is
// the finding's stable identifier (matches the registry's id field).
// Frontmatter is the YAML key/value bag from the promoted artifact
// (.agents/findings/<id>.md frontmatter, governed by
// finding-artifact.schema.json). Body is the markdown content below
// the frontmatter. Adapters MAY use Frontmatter["compiler_targets"]
// to decide which CompiledOutputKind values to emit.
type FindingArtifact struct {
	ID               string
	Frontmatter      map[string]string
	Body             string
	DetectorEvidence *DetectorEvidence
}

// DetectorFixture is a stored positive or explicit negative control. Content
// is evaluated by the compiler; Ref is the immutable citation persisted in the
// resulting shadow evidence.
type DetectorFixture struct {
	Ref     string `json:"ref"`
	Content string `json:"content"`
}

// DetectorPrecisionEvidence summarizes observations gathered while a detector
// ran in non-blocking shadow mode.
type DetectorPrecisionEvidence struct {
	EvidenceRef    string `json:"evidence_ref"`
	Samples        int    `json:"samples"`
	TruePositives  int    `json:"true_positives"`
	FalsePositives int    `json:"false_positives"`
}

// DetectorEvidence is the deterministic proof input for mechanical promotion.
// At least one stored positive and one explicit negative control are required.
type DetectorEvidence struct {
	PositiveFixtures []DetectorFixture          `json:"positive_fixtures"`
	NegativeControls []DetectorFixture          `json:"negative_controls"`
	Precision        *DetectorPrecisionEvidence `json:"precision,omitempty"`
}

// DetectorReplayResult is the safe persisted projection of replay evidence. It
// carries references and counts, never fixture contents.
type DetectorReplayResult struct {
	PositiveRefs        []string
	NegativeControlRefs []string
	Precision           *DetectorPrecisionEvidence
}

// CompiledOutputKind enumerates the three compiler targets named in
// docs/contracts/finding-compiler.md ("Compiler Targets" table). The
// string values match the kebab-case names used in the contract's
// `compiler_targets` field.
type CompiledOutputKind string

const (
	CompiledOutputPlanningRule   CompiledOutputKind = "plan"
	CompiledOutputPremortemCheck CompiledOutputKind = "premortem"
	CompiledOutputConstraint     CompiledOutputKind = "constraint"
)

// ReplayDetectorEvidence evaluates stored positives and explicit negative
// controls against the detector. ready=false means the finding remains
// advisory; malformed evidence or an invalid detector is an error.
func ReplayDetectorEvidence(pattern, kind string, evidence *DetectorEvidence) (DetectorReplayResult, bool, error) {
	if evidence == nil || len(evidence.PositiveFixtures) == 0 || len(evidence.NegativeControls) == 0 {
		return DetectorReplayResult{}, false, nil
	}
	if kind == "" {
		kind = "regex"
	}
	if kind != "regex" {
		return DetectorReplayResult{}, false, fmt.Errorf("ports: unsupported detector kind %q", kind)
	}
	detector, err := regexp.Compile(pattern)
	if err != nil {
		return DetectorReplayResult{}, false, fmt.Errorf("ports: invalid detector pattern: %w", err)
	}
	result := DetectorReplayResult{Precision: evidence.Precision}
	for i, fixture := range evidence.PositiveFixtures {
		if strings.TrimSpace(fixture.Ref) == "" || fixture.Content == "" {
			return DetectorReplayResult{}, false, fmt.Errorf("ports: positive fixture %d requires ref and content", i)
		}
		if !detector.MatchString(fixture.Content) {
			return DetectorReplayResult{}, false, nil
		}
		result.PositiveRefs = append(result.PositiveRefs, fixture.Ref)
	}
	for i, fixture := range evidence.NegativeControls {
		if strings.TrimSpace(fixture.Ref) == "" || fixture.Content == "" {
			return DetectorReplayResult{}, false, fmt.Errorf("ports: negative control %d requires ref and content", i)
		}
		if detector.MatchString(fixture.Content) {
			return DetectorReplayResult{}, false, nil
		}
		result.NegativeControlRefs = append(result.NegativeControlRefs, fixture.Ref)
	}
	if precision := evidence.Precision; precision != nil {
		if strings.TrimSpace(precision.EvidenceRef) == "" || precision.Samples <= 0 ||
			precision.TruePositives < 0 || precision.FalsePositives < 0 ||
			precision.TruePositives+precision.FalsePositives != precision.Samples {
			return DetectorReplayResult{}, false, fmt.Errorf("ports: malformed detector precision evidence")
		}
	}
	return result, true, nil
}

// CompiledOutput is one materialized artifact emitted by Compile. Path
// is the relative output path (e.g. `.agents/planning-rules/<id>.md`);
// adapters that don't write to a filesystem may treat Path as a logical
// key. Body is the file content the adapter would persist. Kind names
// which compiler target produced this output.
type CompiledOutput struct {
	Kind CompiledOutputKind
	Path string
	Body []byte
}

// FindingCompilerPort is the BC1 compile-side. It turns a promoted
// finding artifact into the advisory and mechanical outputs named in
// docs/contracts/finding-compiler.md "Compiler Targets" — planning
// rules, premortem checks, and constraints. Callers — the
// `ao compile` path, dream's compounding loop, and any future
// cross-repo finding ingester — depend on this port so the compile
// behavior can be exercised against an in-memory adapter without
// standing up the real `.agents/findings/`, planning-rules,
// premortem-checks, and constraints surfaces.
//
// Contract:
//
//   - Compile MUST return a non-nil (possibly empty) slice on success.
//   - The returned slice MUST NOT include duplicate Path values; a
//     given output is materialized at most once per artifact.
//   - When the input's Frontmatter constrains `compiler_targets`,
//     adapters SHOULD honor that constraint. When `compiler_targets`
//     is absent, the adapter chooses defaults (and documents them).
//   - Context cancellation MUST be honored on a best-effort basis.
//
// See docs/contracts/ubiquitous-language.md (BC1 row) for the
// canonical Corpus context surface and corpus_reader.go /
// corpus_writer.go for the read+write counterparts.
type FindingCompilerPort interface {
	Compile(ctx context.Context, artifact FindingArtifact) ([]CompiledOutput, error)
}
