// practices: [design-by-contract, output-contract-parity]
package orchestration

import (
	"fmt"
	"time"
)

// InstrumentSchemaVersionV1 is the schema version for instrument-lane JSON output.
const InstrumentSchemaVersionV1 = 1

// Instrument command names.
const (
	InstrumentCommandTools     = "tools"
	InstrumentCommandPreflight = "preflight"
	InstrumentCommandVerify    = "verify"
	InstrumentCommandRoute     = "route"
	InstrumentCommandStatus    = "status"
	InstrumentCommandShape     = "shape"
)

// Ledger event types appended to docs/provenance/ledger.jsonl.
const (
	LedgerEventPreflight = "orchestration.preflight.v1"
	LedgerEventVerify    = "orchestration.verify.v1"
)

// Evidence tiers for verify fallback chain.
const (
	EvidenceTierStrong = "strong"
	EvidenceTierWeak   = "weak"
	EvidenceTierNone   = "none"
)

// CheckStatus is one row in an instrument check list.
type CheckStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// ToolReport is one tool row from ao orchestrate tools.
type ToolReport struct {
	ID          string   `json:"id"`
	Available   bool     `json:"available"`
	Version     string   `json:"version,omitempty"`
	MissingDeps []string `json:"missing_deps,omitempty"`
}

// RouteRecommendation is the output of ao orchestrate route.
type RouteRecommendation struct {
	Shape       string `json:"shape,omitempty"`
	Profile     string `json:"profile,omitempty"`
	Rationale   string `json:"rationale,omitempty"`
	NextCommand string `json:"next_command,omitempty"`
}

// InstrumentResult is the JSON envelope for instrument-lane commands.
type InstrumentResult struct {
	SchemaVersion        int                  `json:"schema_version"`
	Command              string               `json:"command"`
	Profile              string               `json:"profile,omitempty"`
	Session              string               `json:"session,omitempty"`
	RunID                string               `json:"run_id,omitempty"`
	Verdict              Verdict              `json:"verdict"`
	CoordinationDegraded bool                 `json:"coordination_degraded,omitempty"`
	LedgerUnwritten      bool                 `json:"ledger_unwritten,omitempty"`
	LedgerEventType      string               `json:"ledger_event_type,omitempty"`
	EvidenceTier         string               `json:"evidence_tier,omitempty"`
	Checks               []CheckStatus        `json:"checks,omitempty"`
	Tools                []ToolReport         `json:"tools,omitempty"`
	Route                *RouteRecommendation `json:"route,omitempty"`
	Panes                []ProfilePane        `json:"panes,omitempty"`
}

// NewRunID returns a coarse run identifier for ledger idempotency within a process.
func NewRunID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// IdempotencyKey builds the ledger dedupe key: command:profile:session:run_id.
func IdempotencyKey(command, profile, session, runID string) string {
	return fmt.Sprintf("%s:%s:%s:%s", command, profile, session, runID)
}

// ExitCodeForVerdict maps verdict status to process exit code (0 PASS/WARN, 1 FAIL).
func ExitCodeForVerdict(status string) int {
	if status == VerdictStatusFail {
		return 1
	}
	return 0
}

// ApplyLedgerFailure downgrades PASS to WARN when ledger append failed.
func ApplyLedgerFailure(r *InstrumentResult) {
	if r == nil {
		return
	}
	r.LedgerUnwritten = true
	if r.Verdict.Status == VerdictStatusPass {
		r.Verdict.Status = VerdictStatusWarn
	}
}

// AggregateVerdictFromChecks sets verdict from check rows (FAIL > WARN > PASS).
func AggregateVerdictFromChecks(checks []CheckStatus) Verdict {
	status := VerdictStatusPass
	conf := VerdictConfidenceHigh
	for _, c := range checks {
		switch c.Status {
		case VerdictStatusFail:
			return Verdict{Status: VerdictStatusFail, Confidence: VerdictConfidenceHigh}
		case VerdictStatusWarn:
			status = VerdictStatusWarn
			conf = VerdictConfidenceMedium
		}
	}
	return Verdict{Status: status, Confidence: conf}
}
