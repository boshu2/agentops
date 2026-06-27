// Package yieldledger defines the dynamo yield ledger (yieldledger-event.v1): a
// durable, append-only, BEAD-KEYED operational event stream so an agent
// factory's per-bead yield is computable from DATA, not prose.
//
// Built for ag-grcz3 (the instrument) and read by ag-qzinh (the gauges) per
// .agents/specs/2026-06-14-yield-vector-and-ledger-event-gap.md. Before this,
// no per-bead operational stream existed: the EventBus is in-memory only (events
// vanish on exit), verdictledger is directive-scoped, the rpi ledger is
// run-scoped, and PROV is an edge ledger. This package closes that gap.
//
// Design — why a new package rather than extending verdictledger or rpi_ledger:
// verdictledger is a directive-keyed projection for streak/cooldown reasoning;
// rpi_ledger.go is a run-keyed hash-chained RPI event stream. The yield ledger
// is a third, distinct concept: a BEAD-keyed operational stream feeding the
// yield vector (A, R, A/R, Q, E, L). It mirrors verdictledger's shape (atomic
// append, Load + bead-keyed projections, validate-on-load) so it is familiar and
// self-contained — but its record model is the {event, bead_id, run_id, ts,
// body} envelope, not the verdict record.
//
// Emission is fail-open observability, NOT a gate: appending an event must never
// block a merge. Computing the gauges (A, R, A/R, Q, E, L) is out of scope here
// (that is ag-qzinh); this package only durably records the events the gauges
// read.
package yieldledger

import (
	"strings"
	"time"
	"unicode/utf8"
)

// SchemaVersion is the discriminator value for the yieldledger-event.v1 artifact.
const SchemaVersion = "yieldledger-event.v1"

// ArtifactRelPath is the canonical runtime location of the ledger, relative to
// the project root. It lives under .agents/ per ADR-0003 (runtime artifact); the
// schema and fixtures are tracked. The on-disk form is append-only JSONL (one
// event envelope per line — spec §D) so concurrent fungible lanes can each
// O_APPEND without read-modify-write contention.
const ArtifactRelPath = ".agents/yield/yield-ledger.jsonl"

// Event type discriminators (the envelope `event` field).
const (
	// EventAccept marks a bead reaching gated merge to trunk (terminal accept).
	EventAccept = "accept"
	// EventGateVerdict marks a review panel completing on a bead.
	EventGateVerdict = "gate-verdict"
	// EventUsage marks per-bead model spend being observed.
	EventUsage = "usage"
)

// Pawl disposition values projected onto a gate-verdict event.
const (
	DispositionConfirmed = "CONFIRMED"
	DispositionRefuted   = "REFUTED"
	DispositionEscalate  = "ESCALATE"
	DispositionHold      = "HOLD"
)

// Pawl diversity modes projected onto a gate-verdict event.
const (
	ModeFreshContext = "fresh-context"
	ModeMultiModel   = "multi-model"
	// ModeDeterministic marks a verdict from the deterministic membrane tier — a
	// ground-truth gate (build/test/check-*.sh) rejecting claimed-done work,
	// not an LLM panel (age-srl). cross_family is always false for this tier, so
	// it raises catch_rate without touching catch_rate_cross_family.
	ModeDeterministic = "deterministic"
)

// Usage phases — which phase of the bead lifecycle a spend occurred in.
const (
	PhasePlan         = "plan"
	PhaseImplement    = "implement"
	PhaseReview       = "review"
	PhaseRework       = "rework"
	PhaseCoordination = "coordination"
)

// Loss category hints (emit-time best guess; the authoritative L classification
// is a read-time join in ag-qzinh).
const (
	CategoryProductive   = "productive"
	CategoryRework       = "rework"
	CategoryCoordination = "coordination"
)

// PawlVerdictRef references a pawl-verdict.v1 object by bead_id + head_sha. It is
// the link between the yield ledger and the (closed) pawl-verdict schema; the
// ledger never inlines or mutates the pawl-verdict.
type PawlVerdictRef struct {
	BeadID  string `json:"bead_id"`
	HeadSHA string `json:"head_sha"`
}

// AcceptBody is the typed payload of an accept event. Feeds gauge A.
type AcceptBody struct {
	MergeSHA       string         `json:"merge_sha"`
	MergedBy       string         `json:"merged_by"`
	GateVerdictRef PawlVerdictRef `json:"gate_verdict_ref"`
}

// GateVerdictBody is the typed payload of a gate-verdict event. Feeds gauges Q
// and E. It PROJECTS a pawl-verdict by reference and carries the difficulty
// weight, author_family (an emit-time orchestrator input), and the derived SoD
// bits that live ONLY in the yield ledger — never written back to the pawl.
type GateVerdictBody struct {
	Difficulty       float64        `json:"difficulty"`
	PawlVerdictRef   PawlVerdictRef `json:"pawl_verdict_ref"`
	Disposition      string         `json:"disposition"`
	HeadSHA          string         `json:"head_sha"`
	Attempt          int            `json:"attempt"`
	Mode             string         `json:"mode,omitempty"`
	AuthorContextID  string         `json:"author_context_id"`
	RefuterFamilies  []string       `json:"refuter_families"`
	AuthorFamily     string         `json:"author_family"`
	CrossFamily      bool           `json:"cross_family"`
	AuthorNeReviewer bool           `json:"author_ne_reviewer"`
	EvidencePresent  bool           `json:"evidence_present"`
	// Domain is the bounded-context / work-class tag the verdict happened in
	// (e.g. "concurrency", "gate-policy", "docs"). It gives the slow loop a
	// dimension to query escapes by — "what escaped in THIS domain." Reason is
	// the verdict's rationale; on a REFUTED verdict it is the what-was-missed
	// signal a derived check is built from. (age-membrane-memory-j9c6.1)
	Domain string `json:"domain,omitempty"`
	Reason string `json:"reason,omitempty"`
	// Detector* fields, when present on an overturning-REFUTED, make the escape
	// MECHANICALLY re-introducible: deriveFindingFromEscape compiles them into a
	// draft constraint (regex pattern + path globs) the gate enforces, so the
	// membrane doesn't just remember the escape — it BLOCKS its re-introduction.
	// Absent (process-gap escapes) -> the finding stays advisory. (EM.2.10)
	DetectorPattern     string `json:"detector_pattern,omitempty"`
	ConstraintPathGlobs string `json:"constraint_path_globs,omitempty"`
	DetectorKind        string `json:"detector_kind,omitempty"`
	// ClassKey is the catch-native, versioned class identity for a REFUTED catch
	// (see ClassKeyFor): slug(domain)/slug(normalized reason)[/slug(detector)],
	// computed at emit. Two catches of the same class collide on it, so recall and
	// recurrence work without anyone authoring a regex. Empty on non-catch verdicts.
	// (epic age-zpj5, membrane PROVE-FIRST S1)
	ClassKey string `json:"class_key,omitempty"`
	// AffectedPaths are concrete repo-relative FILE paths from the reviewed diff,
	// INDEPENDENT of DetectorPattern/ConstraintPathGlobs — so a judgment-class catch
	// (no detector) is still path-recallable. (epic age-zpj5, S1)
	AffectedPaths []string `json:"affected_paths,omitempty"`
}

// UsageBody is the typed payload of a usage event. Feeds gauges R, L, and A/R.
type UsageBody struct {
	TokensIn     int     `json:"tokens_in"`
	TokensOut    int     `json:"tokens_out"`
	CostUSD      float64 `json:"cost_usd"`
	WallClockS   float64 `json:"wall_clock_s"`
	Model        string  `json:"model"`
	Phase        string  `json:"phase"`
	CategoryHint string  `json:"category_hint,omitempty"`
}

// Event is one yield-ledger entry: the common envelope {event, bead_id, run_id,
// ts} plus exactly one typed body discriminated by Event. The unused body
// pointers are omitted on serialization so each event carries a single body.
type Event struct {
	Event  string `json:"event"`
	BeadID string `json:"bead_id"`
	RunID  string `json:"run_id"`
	TS     string `json:"ts"`

	Accept      *AcceptBody      `json:"-"`
	GateVerdict *GateVerdictBody `json:"-"`
	Usage       *UsageBody       `json:"-"`
}

// Ledger is the full yieldledger-event.v1 document.
type Ledger struct {
	SchemaVersion string  `json:"schema_version"`
	GeneratedAt   string  `json:"generated_at"`
	Events        []Event `json:"events"`
}

// AcceptInput holds the fields needed to append an accept event.
type AcceptInput struct {
	BeadID         string
	RunID          string
	TS             time.Time
	MergeSHA       string
	MergedBy       string
	GateVerdictRef PawlVerdictRef
}

// GateVerdictInput holds the fields needed to append a gate-verdict event.
type GateVerdictInput struct {
	BeadID              string
	RunID               string
	TS                  time.Time
	Difficulty          float64
	PawlVerdictRef      PawlVerdictRef
	Disposition         string
	HeadSHA             string
	Attempt             int
	Mode                string
	AuthorContextID     string
	RefuterFamilies     []string
	AuthorFamily        string
	CrossFamily         bool
	AuthorNeReviewer    bool
	EvidencePresent     bool
	Domain              string
	Reason              string
	DetectorPattern     string
	ConstraintPathGlobs string
	DetectorKind        string
	AffectedPaths       []string
}

// UsageInput holds the fields needed to append a usage event.
type UsageInput struct {
	BeadID       string
	RunID        string
	TS           time.Time
	TokensIn     int
	TokensOut    int
	CostUSD      float64
	WallClockS   float64
	Model        string
	Phase        string
	CategoryHint string
}

// validDisposition reports whether d is a recognized pawl disposition.
func validDisposition(d string) bool {
	switch d {
	case DispositionConfirmed, DispositionRefuted, DispositionEscalate, DispositionHold:
		return true
	default:
		return false
	}
}

// validMode reports whether m is a recognized pawl diversity mode. The empty
// string is allowed (the projected pawl mode is optional; absent => default).
func validMode(m string) bool {
	switch m {
	case "", ModeFreshContext, ModeMultiModel, ModeDeterministic:
		return true
	default:
		return false
	}
}

// validPhase reports whether p is a recognized usage phase.
func validPhase(p string) bool {
	switch p {
	case PhasePlan, PhaseImplement, PhaseReview, PhaseRework, PhaseCoordination:
		return true
	default:
		return false
	}
}

// validCategoryHint reports whether c is a recognized loss category hint. The
// empty string is allowed (the hint is optional and advisory).
func validCategoryHint(c string) bool {
	switch c {
	case "", CategoryProductive, CategoryRework, CategoryCoordination:
		return true
	default:
		return false
	}
}

// validRef reports whether a PawlVerdictRef is structurally complete.
func validRef(r PawlVerdictRef) bool {
	// Count CODE POINTS (not bytes) so the floor matches the schema's minLength:7,
	// which is code-point-counted — a multi-byte head_sha can't byte-pass but rune-fail.
	return strings.TrimSpace(r.BeadID) != "" && utf8.RuneCountInString(strings.TrimSpace(r.HeadSHA)) >= 7
}

// newAcceptEvent builds an accept Event from validated input.
func newAcceptEvent(in AcceptInput) Event {
	return Event{
		Event:  EventAccept,
		BeadID: in.BeadID,
		RunID:  in.RunID,
		TS:     in.TS.UTC().Format(time.RFC3339),
		Accept: &AcceptBody{
			MergeSHA:       in.MergeSHA,
			MergedBy:       in.MergedBy,
			GateVerdictRef: in.GateVerdictRef,
		},
	}
}

// newGateVerdictEvent builds a gate-verdict Event from validated input.
func newGateVerdictEvent(in GateVerdictInput) Event {
	return Event{
		Event:  EventGateVerdict,
		BeadID: in.BeadID,
		RunID:  in.RunID,
		TS:     in.TS.UTC().Format(time.RFC3339),
		GateVerdict: &GateVerdictBody{
			Difficulty:          in.Difficulty,
			PawlVerdictRef:      in.PawlVerdictRef,
			Disposition:         in.Disposition,
			HeadSHA:             in.HeadSHA,
			Attempt:             in.Attempt,
			Mode:                in.Mode,
			AuthorContextID:     in.AuthorContextID,
			RefuterFamilies:     cleanStrings(in.RefuterFamilies),
			AuthorFamily:        in.AuthorFamily,
			CrossFamily:         in.CrossFamily,
			AuthorNeReviewer:    in.AuthorNeReviewer,
			EvidencePresent:     in.EvidencePresent,
			Domain:              in.Domain,
			Reason:              in.Reason,
			DetectorPattern:     in.DetectorPattern,
			ConstraintPathGlobs: in.ConstraintPathGlobs,
			DetectorKind:        in.DetectorKind,
			AffectedPaths:       cleanStrings(in.AffectedPaths),
			ClassKey:            classKeyIfCatch(in.Disposition, in.Domain, in.Reason, in.DetectorPattern),
		},
	}
}

// newUsageEvent builds a usage Event from validated input.
func newUsageEvent(in UsageInput) Event {
	return Event{
		Event:  EventUsage,
		BeadID: in.BeadID,
		RunID:  in.RunID,
		TS:     in.TS.UTC().Format(time.RFC3339),
		Usage: &UsageBody{
			TokensIn:     in.TokensIn,
			TokensOut:    in.TokensOut,
			CostUSD:      in.CostUSD,
			WallClockS:   in.WallClockS,
			Model:        in.Model,
			Phase:        in.Phase,
			CategoryHint: in.CategoryHint,
		},
	}
}

// validateEvent returns a non-empty string describing the first structural
// defect in e, or "" if e is well-formed. The envelope is checked first, then
// the typed body keyed by the `event` discriminator.
func validateEvent(e Event) string {
	if strings.TrimSpace(e.BeadID) == "" {
		return "missing bead_id"
	}
	if strings.TrimSpace(e.RunID) == "" {
		return "missing run_id"
	}
	if strings.TrimSpace(e.TS) == "" {
		return "missing ts"
	}
	if _, err := time.Parse(time.RFC3339, e.TS); err != nil {
		return "ts not RFC3339: " + e.TS
	}
	switch e.Event {
	case EventAccept:
		return validateAcceptEvent(e)
	case EventGateVerdict:
		return validateGateVerdictEvent(e)
	case EventUsage:
		return validateUsageEvent(e)
	default:
		return "invalid event: " + e.Event
	}
}

// validateAcceptEvent checks the accept-specific fields of e.
func validateAcceptEvent(e Event) string {
	if e.Accept == nil {
		return "accept event missing body"
	}
	if e.GateVerdict != nil || e.Usage != nil {
		return "accept event carries a foreign body"
	}
	if len(strings.TrimSpace(e.Accept.MergeSHA)) < 7 {
		return "accept body merge_sha too short"
	}
	if strings.TrimSpace(e.Accept.MergedBy) == "" {
		return "accept body missing merged_by"
	}
	if !validRef(e.Accept.GateVerdictRef) {
		return "accept body invalid gate_verdict_ref"
	}
	return ""
}

// validateGateVerdictEvent checks the gate-verdict-specific fields of e.
func validateGateVerdictEvent(e Event) string {
	if e.GateVerdict == nil {
		return "gate-verdict event missing body"
	}
	if e.Accept != nil || e.Usage != nil {
		return "gate-verdict event carries a foreign body"
	}
	b := e.GateVerdict
	if b.Difficulty < 0 {
		return "gate-verdict body negative difficulty"
	}
	if !validRef(b.PawlVerdictRef) {
		return "gate-verdict body invalid pawl_verdict_ref"
	}
	if !validDisposition(b.Disposition) {
		return "gate-verdict body invalid disposition: " + b.Disposition
	}
	if utf8.RuneCountInString(strings.TrimSpace(b.HeadSHA)) < 7 { // code points, matching schema minLength:7
		return "gate-verdict body head_sha too short"
	}
	if b.Attempt < 1 {
		return "gate-verdict body attempt must be >= 1"
	}
	if !validMode(b.Mode) {
		return "gate-verdict body invalid mode: " + b.Mode
	}
	if strings.TrimSpace(b.AuthorContextID) == "" {
		return "gate-verdict body missing author_context_id"
	}
	if strings.TrimSpace(b.AuthorFamily) == "" {
		return "gate-verdict body missing author_family"
	}
	return ""
}

// validateUsageEvent checks the usage-specific fields of e.
func validateUsageEvent(e Event) string {
	if e.Usage == nil {
		return "usage event missing body"
	}
	if e.Accept != nil || e.GateVerdict != nil {
		return "usage event carries a foreign body"
	}
	b := e.Usage
	if b.TokensIn < 0 || b.TokensOut < 0 {
		return "usage body negative token count"
	}
	if b.CostUSD < 0 {
		return "usage body negative cost_usd"
	}
	if b.WallClockS < 0 {
		return "usage body negative wall_clock_s"
	}
	if strings.TrimSpace(b.Model) == "" {
		return "usage body missing model"
	}
	if !validPhase(b.Phase) {
		return "usage body invalid phase: " + b.Phase
	}
	if !validCategoryHint(b.CategoryHint) {
		return "usage body invalid category_hint: " + b.CategoryHint
	}
	return ""
}
