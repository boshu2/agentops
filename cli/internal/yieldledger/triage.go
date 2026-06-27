package yieldledger

import (
	"regexp"
	"strings"
)

// Triage is the HONEST recurrence+compilability instrument (epic age-zpj5, S4): it
// reads the CATCH corpus and answers "does the compiler thesis have fuel?" with a
// pre-registered numeric rule, NEVER a fabricated number. Its current realistic answer
// is INSUFFICIENT-DATA — the corpus is below the power floor — which is the PROVE-FIRST
// design's whole point: we do NOT build the compiler on faith.

// Pre-registered constants. class_key_v1-versioned; the 0.20/0.33 thresholds are
// conventions (not science), re-committable ONCE before a first GO, never silently.
const (
	TriagePowerFloor      = 15   // min distinct classes WITH a stored reason before any GO/NO-GO
	TriageAxis1Recurrence = 0.20 // below -> one-off-dominated -> MEMORY-ONLY
	TriageAxis2Compilable = 0.33 // below -> mostly judgment-class -> CURATED
)

// Triage decision outcomes.
const (
	DecisionInsufficientData = "INSUFFICIENT-DATA"
	DecisionMemoryOnly       = "MEMORY-ONLY"
	DecisionCurated          = "CURATED"
	DecisionGo               = "GO"
)

// Per-class compilability assessment states (the bounded per-recurring-class protocol).
const (
	AssessCompilable    = "compilable"
	AssessNotCompilable = "not_compilable"
	AssessUnassessed    = "unassessed"
)

// TriageResult is the computed read. All counts are corpus properties; Decision is a
// deterministic function of them via TriageDecide.
type TriageResult struct {
	DistinctClasses         int
	RecurringClasses        int
	ClassesWithStoredReason int
	// UnclassifiedFloor is the count of reason-less / domain-less REFUTEDs — counted as a
	// floor ONLY, NEVER assigned a synthesized reason/class (the no-fabrication rule).
	UnclassifiedFloor     int
	AssessedCompilable    int
	AssessedNotCompilable int
	Unassessed            int
	Axis1Recurrence       float64 // recurring / distinct
	Axis2Compilable       float64 // compilable / recurring (unassessed in denominator)
	Axis2Coverage         float64 // (compilable + not) / recurring — 100% required for a GO/CURATED
	Decision              string
}

// TriageDecide applies the PRE-REGISTERED NUMERIC RULE to the corpus counts. Pure +
// deterministic + total. Axis-2 puts unassessed in the denominator (a small assessed
// subset cannot inflate it) AND requires 100% coverage before any GO/CURATED.
func TriageDecide(distinct, recurring, classesWithStoredReason, assessedCompilable, assessedNotCompilable int) TriageResult {
	r := TriageResult{
		DistinctClasses:         distinct,
		RecurringClasses:        recurring,
		ClassesWithStoredReason: classesWithStoredReason,
		AssessedCompilable:      assessedCompilable,
		AssessedNotCompilable:   assessedNotCompilable,
		Unassessed:              recurring - assessedCompilable - assessedNotCompilable,
	}
	if recurring > 0 {
		r.Axis1Recurrence = float64(recurring) / float64(distinct)
		r.Axis2Compilable = float64(assessedCompilable) / float64(recurring)
		r.Axis2Coverage = float64(assessedCompilable+assessedNotCompilable) / float64(recurring)
	} else {
		// No recurring classes: coverage is vacuously complete; axis1=0 routes to MEMORY-ONLY
		// (once above the power floor), never a false GO.
		r.Axis2Coverage = 1.0
	}
	switch {
	case classesWithStoredReason < TriagePowerFloor || r.Axis2Coverage < 1.0:
		r.Decision = DecisionInsufficientData
	case r.Axis1Recurrence < TriageAxis1Recurrence:
		r.Decision = DecisionMemoryOnly
	case r.Axis2Compilable < TriageAxis2Compilable:
		r.Decision = DecisionCurated
	default:
		r.Decision = DecisionGo
	}
	return r
}

// AssessCompilability is the pure all-instances TP-replay assessment (S4): a detector is
// ASSESSED-COMPILABLE iff it MATCHES every stored bad-instance content (true-positive on
// EVERY instance — an overfit detector that hits one but misses another is NOT compilable)
// AND does NOT match the clean-HEAD content (zero false-positive). An empty/invalid
// detector or zero instances is not_compilable (nothing to compile / cannot prove). The
// git-content fetch is the caller's job; this function is pure over the supplied contents.
func AssessCompilability(detector string, badInstanceContents []string, cleanHeadContent string) string {
	if strings.TrimSpace(detector) == "" || len(badInstanceContents) == 0 {
		return AssessNotCompilable
	}
	re, err := regexp.Compile(detector)
	if err != nil {
		return AssessNotCompilable // an uncompilable regex is not a sound mechanical check
	}
	for _, content := range badInstanceContents { // TP: must hit EVERY bad instance
		if !re.MatchString(content) {
			return AssessNotCompilable
		}
	}
	if re.MatchString(cleanHeadContent) { // FP: must NOT hit clean HEAD
		return AssessNotCompilable
	}
	return AssessCompilable
}

// TriageCorpus reads the ledger, derives the catch-corpus counts (recurrence over
// DetectCatches class_keys; the reason-less REFUTEDs are the UnclassifiedFloor, counted
// via a separate scan and NEVER given a synthesized reason), assesses each recurring class
// via the injected `assess` closure (which returns AssessCompilable/NotCompilable/Unassessed
// — the git-content TP-replay is injected so this is mockable + testable), and applies
// TriageDecide. (epic age-zpj5, S4)
func TriageCorpus(l *Ledger, assess func(Catch) string) TriageResult {
	catches := DetectCatches(l)
	distinct := len(catches)
	// DetectCatches already requires a non-empty reason, so every returned class carries a
	// STORED reason — the floor count is the SEPARATE reason-less REFUTEDs, never these.
	withReason := distinct
	recurring, compilable, notCompilable := 0, 0, 0
	for _, c := range catches {
		if c.HitCount < 2 {
			continue
		}
		recurring++
		switch assess(c) {
		case AssessCompilable:
			compilable++
		case AssessNotCompilable:
			notCompilable++
			// AssessUnassessed (or anything else) leaves coverage < 1.0 -> INSUFFICIENT-DATA.
		}
	}
	r := TriageDecide(distinct, recurring, withReason, compilable, notCompilable)
	r.UnclassifiedFloor = unclassifiedRefutedCount(l)
	return r
}

// unclassifiedRefutedCount counts REFUTED gate-verdicts that are NOT classifiable — no
// reason/domain OR sentinel-stamped (DomainUnclassified / ReasonUnspecified) — the
// unclassified floor. Uses the SAME isClassifiableCatch predicate as DetectCatches so a
// sentinel row is counted in the floor and NEVER also as a class. (epic age-zpj5, S4)
func unclassifiedRefutedCount(l *Ledger) int {
	if l == nil {
		return 0
	}
	n := 0
	for _, ev := range l.Events {
		if ev.Event != EventGateVerdict || ev.GateVerdict == nil {
			continue
		}
		gv := ev.GateVerdict
		if gv.Disposition != DispositionRefuted {
			continue
		}
		if !isClassifiableCatch(gv.Domain, gv.Reason) {
			n++
		}
	}
	return n
}
