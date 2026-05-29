// practices: [design-by-contract, output-contract-parity]
package orchestration

import (
	"fmt"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// SchemaVersionV1 is the schema version that OrchestrationResult mirrors:
// schemas/orchestration-result.v1.schema.json. Every conformant result MUST
// carry this exact value so consumers can dispatch on shape stability.
const SchemaVersionV1 = 1

// Verdict status enum values. These mirror the `verdict.status` enum in
// orchestration-result.v1.schema.json and are the only legal Status values.
const (
	// VerdictStatusPass marks a run whose work succeeded against its
	// acceptance criteria.
	VerdictStatusPass = "PASS"
	// VerdictStatusWarn marks a run that succeeded with caveats worth
	// surfacing to a human or downstream tier.
	VerdictStatusWarn = "WARN"
	// VerdictStatusFail marks a run whose work did not meet its acceptance
	// criteria.
	VerdictStatusFail = "FAIL"
)

// Verdict confidence enum values. These mirror the `verdict.confidence`
// enum in orchestration-result.v1.schema.json and are the only legal
// Confidence values.
const (
	// VerdictConfidenceHigh signals strong evidence behind the Status.
	VerdictConfidenceHigh = "HIGH"
	// VerdictConfidenceMedium signals moderate evidence behind the Status.
	VerdictConfidenceMedium = "MEDIUM"
	// VerdictConfidenceLow signals weak evidence behind the Status.
	VerdictConfidenceLow = "LOW"
)

// Verdict is the pass/warn/fail judgement a backend tier reaches about a
// unit of work, paired with the tier's confidence in that judgement. It
// mirrors the `verdict` object of orchestration-result.v1.schema.json.
//
// Both fields are required and enum-constrained: Status is one of
// PASS/WARN/FAIL and Confidence is one of HIGH/MEDIUM/LOW. Use Validate
// (via OrchestrationResult.Validate) to self-check membership.
type Verdict struct {
	// Status is the pass/warn/fail outcome. One of VerdictStatus*.
	Status string `json:"status"`
	// Confidence is the tier's confidence in Status. One of
	// VerdictConfidence*.
	Confidence string `json:"confidence"`
}

// OrchestrationResult is the OUTPUT-CONTRACT PARITY shape that EVERY
// backend tier (NTM, Claude-native, Codex, beads floor) MUST emit. It is
// the Go mirror of schemas/orchestration-result.v1.schema.json.
//
// Parity is what makes the safe-degradation ladder correctness-preserving:
// because every tier returns the same shape, a caller can degrade from the
// preferred NTM swarm down to the beads floor without changing how it reads
// the outcome. This type is the canonical contract all adapters conform to;
// each adapter is responsible for populating it and SHOULD call Validate to
// self-check before returning.
//
// Field-to-schema mapping:
//
//   - SchemaVersion -> schema_version (required, const 1)
//   - Backend       -> backend        (required, enum ntm/claude/codex/beads)
//   - ResultPaths   -> result_paths   (required, array of repo-relative paths)
//   - Verdict       -> verdict        (required, status + confidence)
//   - TaskID        -> task_id        (optional, e.g. a bead ID)
type OrchestrationResult struct {
	// SchemaVersion is the contract version. MUST equal SchemaVersionV1.
	SchemaVersion int `json:"schema_version"`
	// Backend is the tier that produced this result.
	Backend ports.Backend `json:"backend"`
	// ResultPaths are repo-root-relative paths to the artifacts this run
	// wrote. Required and non-empty for a conformant result.
	ResultPaths []string `json:"result_paths"`
	// Verdict is the tier's pass/warn/fail judgement plus confidence.
	Verdict Verdict `json:"verdict"`
	// TaskID identifies the task this result fulfills (e.g. a bead ID).
	// Optional; may be empty.
	TaskID string `json:"task_id,omitempty"`
}

// validBackends is the set of backend values the schema's `backend` enum
// permits. Kept in lockstep with the ports.Backend ladder constants.
var validBackends = map[ports.Backend]bool{
	ports.BackendNTM:    true,
	ports.BackendClaude: true,
	ports.BackendCodex:  true,
	ports.BackendBeads:  true,
}

// validVerdictStatuses is the set of legal Verdict.Status values.
var validVerdictStatuses = map[string]bool{
	VerdictStatusPass: true,
	VerdictStatusWarn: true,
	VerdictStatusFail: true,
}

// validVerdictConfidences is the set of legal Verdict.Confidence values.
var validVerdictConfidences = map[string]bool{
	VerdictConfidenceHigh:   true,
	VerdictConfidenceMedium: true,
	VerdictConfidenceLow:    true,
}

// Validate self-checks that the result conforms to the parity contract in
// orchestration-result.v1.schema.json. Any backend tier can call it to
// prove the result it is about to return is parity-conformant before it
// leaves the adapter, which is the mechanism that keeps degradation
// correctness-preserving.
//
// It returns a descriptive error on the first violation found and nil when
// every required field is present and every enum-constrained field is a
// member of its enum:
//
//   - SchemaVersion MUST equal SchemaVersionV1.
//   - Backend MUST be one of the ports.Backend ladder values.
//   - ResultPaths MUST be non-empty and contain no empty strings.
//   - Verdict.Status MUST be one of PASS/WARN/FAIL.
//   - Verdict.Confidence MUST be one of HIGH/MEDIUM/LOW.
//
// TaskID is optional and is not validated.
func (r OrchestrationResult) Validate() error {
	if r.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("schema_version: want %d, got %d", SchemaVersionV1, r.SchemaVersion)
	}
	if !validBackends[r.Backend] {
		return fmt.Errorf("backend: %q is not a valid backend", r.Backend)
	}
	if len(r.ResultPaths) == 0 {
		return fmt.Errorf("result_paths: must be non-empty")
	}
	for i, p := range r.ResultPaths {
		if p == "" {
			return fmt.Errorf("result_paths[%d]: must not be empty", i)
		}
	}
	if !validVerdictStatuses[r.Verdict.Status] {
		return fmt.Errorf("verdict.status: %q is not one of PASS/WARN/FAIL", r.Verdict.Status)
	}
	if !validVerdictConfidences[r.Verdict.Confidence] {
		return fmt.Errorf("verdict.confidence: %q is not one of HIGH/MEDIUM/LOW", r.Verdict.Confidence)
	}
	return nil
}
