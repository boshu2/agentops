// Package verdictledger defines the verdict ledger (verdict-ledger.v1): an
// append-only, per-iteration record of each GOALS.md directive's scenario
// verdict. F2's `ao goals measure` appends one iteration record per directive
// per run; the F5.2 re-steer policy engine reads the ledger to detect failure
// streaks and cooldowns.
//
// Design choice — why a new package rather than extending rpi_ledger.go:
// rpi_ledger.go is a hash-chained, generic RPI *event* stream keyed by run_id
// and filtered one run at a time. The verdict ledger is a different concept:
// a typed, cross-iteration projection (directive_id -> ordered verdicts) whose
// whole purpose is streak/cooldown reasoning ACROSS runs. Forcing it into the
// per-run hash chain would fork the ledger record shape (PHASE 6 epic note
// explicitly forbids that). This package is the read-side projection the bead's
// PHASE 7 note calls for; it is self-contained and can be called from cmd/ao
// code (including rpi_ledger.go) without coupling to the RPI event chain.
package verdictledger

import (
	"regexp"
	"strings"
	"time"
)

// SchemaVersion is the discriminator value for the verdict-ledger.v1 artifact.
const SchemaVersion = "verdict-ledger.v1"

// ArtifactRelPath is the canonical runtime location of the ledger, relative to
// the project root. It lives under .agents/ per ADR-0003 (runtime artifact);
// the schema and fixtures are tracked.
const ArtifactRelPath = ".agents/goals/verdict-ledger.json"

// Record type discriminators.
const (
	// RecordIteration is one completed `ao goals measure` iteration's verdict.
	RecordIteration = "iteration"
	// RecordCooldown marks a re-steer proposal/application against a directive.
	RecordCooldown = "cooldown"
)

// Scenario verdict values for an iteration record.
const (
	VerdictPass    = "pass"
	VerdictFail    = "fail"
	VerdictSkip    = "skip"
	VerdictUnknown = "unknown"
)

// Cooldown kinds.
const (
	CooldownProposed = "proposed"
	CooldownApplied  = "applied"
)

// Mutation types (mirrors the re-steer-policy.v1 enum).
const (
	MutationPriorityBump    = "priority_bump"
	MutationSetpointTighten = "setpoint_tighten"
	MutationSetpointLoosen  = "setpoint_loosen"
	MutationSteerFlip       = "steer_flip"
)

// directiveIDPattern matches a stable GOALS.md directive ID.
var directiveIDPattern = regexp.MustCompile(`^d-[a-z0-9][a-z0-9-]*$`)

// Record is one verdict ledger entry. It is a tagged union discriminated by
// RecordType: an iteration record carries the scenario verdict fields; a
// cooldown record carries the re-steer event fields. Unused fields for a given
// record type are omitted on serialization.
type Record struct {
	RecordType  string `json:"record_type"`
	DirectiveID string `json:"directive_id"`
	RunTime     string `json:"run_timestamp"`

	// Iteration-record fields.
	ScenarioVerdict      string   `json:"scenario_verdict,omitempty"`
	ScenarioSatisfaction *float64 `json:"scenario_satisfaction,omitempty"`
	ScenarioCount        *int     `json:"scenario_count,omitempty"`
	EvaluatedCount       *int     `json:"evaluated_count,omitempty"`
	RunID                string   `json:"run_id,omitempty"`

	// Cooldown-record fields.
	CooldownKind string `json:"cooldown_kind,omitempty"`
	MutationType string `json:"mutation_type,omitempty"`
	Note         string `json:"note,omitempty"`
}

// Ledger is the full verdict-ledger.v1 document.
type Ledger struct {
	SchemaVersion string   `json:"schema_version"`
	GeneratedAt   string   `json:"generated_at"`
	Records       []Record `json:"records"`
}

// IterationInput holds the fields needed to append one iteration record.
type IterationInput struct {
	DirectiveID          string
	RunTime              time.Time
	ScenarioVerdict      string
	ScenarioSatisfaction float64
	ScenarioCount        int
	EvaluatedCount       int
	RunID                string
}

// CooldownInput holds the fields needed to append one cooldown record.
type CooldownInput struct {
	DirectiveID  string
	RunTime      time.Time
	CooldownKind string
	MutationType string
	Note         string
}

// ValidDirectiveID reports whether id matches the stable directive ID pattern.
func ValidDirectiveID(id string) bool {
	return directiveIDPattern.MatchString(id)
}

// ValidVerdict reports whether v is a recognized scenario verdict.
func ValidVerdict(v string) bool {
	switch v {
	case VerdictPass, VerdictFail, VerdictSkip, VerdictUnknown:
		return true
	default:
		return false
	}
}

// validMutationType reports whether m is a recognized mutation type.
func validMutationType(m string) bool {
	switch m {
	case MutationPriorityBump, MutationSetpointTighten, MutationSetpointLoosen, MutationSteerFlip:
		return true
	default:
		return false
	}
}

// ptrFloat and ptrInt build pointers for optional numeric fields.
func ptrFloat(v float64) *float64 { return &v }
func ptrInt(v int) *int           { return &v }

// newIterationRecord builds an iteration Record from validated input.
func newIterationRecord(in IterationInput) Record {
	return Record{
		RecordType:           RecordIteration,
		DirectiveID:          in.DirectiveID,
		RunTime:              in.RunTime.UTC().Format(time.RFC3339),
		ScenarioVerdict:      in.ScenarioVerdict,
		ScenarioSatisfaction: ptrFloat(in.ScenarioSatisfaction),
		ScenarioCount:        ptrInt(in.ScenarioCount),
		EvaluatedCount:       ptrInt(in.EvaluatedCount),
		RunID:                in.RunID,
	}
}

// newCooldownRecord builds a cooldown Record from validated input.
func newCooldownRecord(in CooldownInput) Record {
	return Record{
		RecordType:   RecordCooldown,
		DirectiveID:  in.DirectiveID,
		RunTime:      in.RunTime.UTC().Format(time.RFC3339),
		CooldownKind: in.CooldownKind,
		MutationType: in.MutationType,
		Note:         in.Note,
	}
}

// validateRecord returns a non-empty string describing the first structural
// defect in r, or "" if r is well-formed.
func validateRecord(r Record) string {
	if !ValidDirectiveID(r.DirectiveID) {
		return "invalid directive_id: " + r.DirectiveID
	}
	if strings.TrimSpace(r.RunTime) == "" {
		return "missing run_timestamp"
	}
	if _, err := time.Parse(time.RFC3339, r.RunTime); err != nil {
		return "run_timestamp not RFC3339: " + r.RunTime
	}
	switch r.RecordType {
	case RecordIteration:
		return validateIterationRecord(r)
	case RecordCooldown:
		return validateCooldownRecord(r)
	default:
		return "invalid record_type: " + r.RecordType
	}
}

// validateIterationRecord checks the iteration-specific fields of r.
func validateIterationRecord(r Record) string {
	if !ValidVerdict(r.ScenarioVerdict) {
		return "invalid scenario_verdict: " + r.ScenarioVerdict
	}
	if r.ScenarioSatisfaction == nil {
		return "missing scenario_satisfaction"
	}
	if *r.ScenarioSatisfaction < 0 || *r.ScenarioSatisfaction > 1 {
		return "scenario_satisfaction out of range [0,1]"
	}
	if r.ScenarioCount == nil || *r.ScenarioCount < 0 {
		return "missing or negative scenario_count"
	}
	if r.EvaluatedCount == nil || *r.EvaluatedCount < 0 {
		return "missing or negative evaluated_count"
	}
	return ""
}

// validateCooldownRecord checks the cooldown-specific fields of r.
func validateCooldownRecord(r Record) string {
	if r.CooldownKind != CooldownProposed && r.CooldownKind != CooldownApplied {
		return "invalid cooldown_kind: " + r.CooldownKind
	}
	if !validMutationType(r.MutationType) {
		return "invalid mutation_type: " + r.MutationType
	}
	return ""
}
