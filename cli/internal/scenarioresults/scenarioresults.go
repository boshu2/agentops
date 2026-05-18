// Package scenarioresults defines the structured per-scenario satisfaction
// artifact (scenario-results.v1) produced by the RPI evaluator and consumed by
// `ao goals measure` (F2). It provides typed structs plus a deterministic
// writer/loader pair so a missing artifact yields a clean "unknown/skip"
// signal rather than a crash or a false pass.
package scenarioresults

import (
	"regexp"
	"strings"
)

// SchemaVersion is the discriminator value for the scenario-results.v1 artifact.
const SchemaVersion = "scenario-results.v1"

// ArtifactRelPath is the canonical runtime location of the artifact, relative
// to the project root.
const ArtifactRelPath = ".agents/rpi/scenario-results.json"

// Verdict outcomes for a single scenario result.
const (
	VerdictPass = "pass"
	VerdictFail = "fail"
	VerdictSkip = "skip"
)

// directiveIDPattern matches a stable GOALS.md directive ID.
var directiveIDPattern = regexp.MustCompile(`^d-[a-z0-9][a-z0-9-]*$`)

// scenarioIDPattern matches a scenario ID (human s-YYYY-MM-DD-NNN or agent auto-*).
var scenarioIDPattern = regexp.MustCompile(`^(s-\d{4}-\d{2}-\d{2}-\d{3}|auto-.+)$`)

// ScenarioResult is one scenario's satisfaction outcome for an RPI iteration.
type ScenarioResult struct {
	ScenarioID  string   `json:"scenario_id"`
	DirectiveID string   `json:"directive_id"`
	Score       float64  `json:"score"`
	Threshold   float64  `json:"threshold"`
	Verdict     string   `json:"verdict"`
	JudgedAt    string   `json:"judged_at"`
	Evidence    []string `json:"evidence"`
}

// Artifact is the full scenario-results.v1 document.
type Artifact struct {
	SchemaVersion string           `json:"schema_version"`
	RunID         string           `json:"run_id"`
	Iteration     int              `json:"iteration"`
	GeneratedAt   string           `json:"generated_at"`
	Results       []ScenarioResult `json:"results"`
}

// ValidVerdict reports whether v is a recognized scenario verdict.
func ValidVerdict(v string) bool {
	switch v {
	case VerdictPass, VerdictFail, VerdictSkip:
		return true
	default:
		return false
	}
}

// ValidDirectiveID reports whether id matches the stable directive ID pattern.
func ValidDirectiveID(id string) bool {
	return directiveIDPattern.MatchString(id)
}

// ValidScenarioID reports whether id matches the scenario ID pattern.
func ValidScenarioID(id string) bool {
	return scenarioIDPattern.MatchString(id)
}

// validateResult returns a non-nil error describing the first structural
// defect in r, or nil if r is well-formed.
func validateResult(r ScenarioResult) string {
	if strings.TrimSpace(r.ScenarioID) == "" {
		return "missing scenario_id"
	}
	if !ValidScenarioID(r.ScenarioID) {
		return "invalid scenario_id: " + r.ScenarioID
	}
	if !ValidDirectiveID(r.DirectiveID) {
		return "invalid directive_id: " + r.DirectiveID
	}
	if !ValidVerdict(r.Verdict) {
		return "invalid verdict: " + r.Verdict
	}
	if r.Score < 0 || r.Score > 1 {
		return "score out of range [0,1]"
	}
	if r.Threshold < 0 || r.Threshold > 1 {
		return "threshold out of range [0,1]"
	}
	if strings.TrimSpace(r.JudgedAt) == "" {
		return "missing judged_at"
	}
	return ""
}
