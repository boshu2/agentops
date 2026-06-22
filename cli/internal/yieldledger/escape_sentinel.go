package yieldledger

import "path/filepath"

// DomainUnclassified is the sentinel domain stamped on an overturning-REFUTED
// (an escape) whose producer supplied no domain. The yield-ledger emit is
// fail-open observability — a caller that REJECTED a domain-less escape would
// silently DROP the event under its `|| true` guard, losing the very escape the
// self-improving loop most needs. So instead of rejecting, the writer records
// the escape with this sentinel: the event is NEVER lost, and the gap is VISIBLE
// debt — queryable via `ao membrane recall --domain UNCLASSIFIED` and treated as
// debt (never as a real domain) by derive-checks. (EM.2.1 / tz2s.2.1)
const DomainUnclassified = "UNCLASSIFIED"

// ReasonUnspecified is the sentinel "what was missed" stamped on an
// overturning-REFUTED (an escape) whose producer supplied no reason. Same
// rationale as DomainUnclassified: the fail-open emit must never DROP an escape,
// so a missing reason becomes visible debt rather than a rejection. Every escape
// must carry BOTH a domain (routing key) and a reason (what was missed); the
// writer guarantees both at the chokepoint for every emitter. (EM.2.1)
const ReasonUnspecified = "unspecified"

// LedgerPath returns the absolute yield-ledger path for a project root. Exported
// so callers (the emit command) can load the pre-append ledger to decide whether
// the writer will stamp the escape sentinel — without duplicating the path join.
func LedgerPath(projectRoot string) string {
	return filepath.Join(projectRoot, filepath.FromSlash(ArtifactRelPath))
}

// IsOverturningRefuted reports whether appending in would form an escape: a
// REFUTED verdict at an attempt strictly higher than a prior CONFIRMED for the
// SAME bead in the SAME run already on the ledger. This mirrors DetectEscapes'
// definition (the decoupled, non-circular failure label) at WRITE time — before
// the event is appended — so the writer can guarantee an escape carries a domain.
func IsOverturningRefuted(l *Ledger, in GateVerdictInput) bool {
	if l == nil || in.Disposition != DispositionRefuted {
		return false
	}
	for _, ev := range l.Events {
		if ev.Event != EventGateVerdict || ev.GateVerdict == nil {
			continue
		}
		if ev.BeadID != in.BeadID || ev.RunID != in.RunID {
			continue
		}
		if ev.GateVerdict.Disposition == DispositionConfirmed && ev.GateVerdict.Attempt < in.Attempt {
			return true
		}
	}
	return false
}

// StampEscapeSentinels applies the escape sentinels for an append, robust to a
// degraded ledger. With a cleanly-loaded ledger (loadErr == nil) it uses precise
// overturn detection via ApplyEscapeSentinels. If the ledger could NOT be read
// (loadErr != nil — which means a CORRUPT/unreadable EXISTING ledger: a MISSING
// ledger is not an error, it loads as empty per LoadPath), it fails SAFE — it
// cannot rule out a prior CONFIRMED, so a REFUTED with an empty domain/reason is
// stamped regardless. The "every escape carries domain+reason" guarantee must
// survive a degraded ledger; a sentinel on a non-overturning REFUTED is harmless
// (DetectEscapes won't classify it as an escape). substituted=true if either was
// stamped, so the caller can surface it as visible debt.
func StampEscapeSentinels(existing *Ledger, loadErr error, in GateVerdictInput) (GateVerdictInput, bool) {
	if loadErr == nil {
		return ApplyEscapeSentinels(existing, in)
	}
	if in.Disposition != DispositionRefuted {
		return in, false
	}
	substituted := false
	if in.Domain == "" {
		in.Domain = DomainUnclassified
		substituted = true
	}
	if in.Reason == "" {
		in.Reason = ReasonUnspecified
		substituted = true
	}
	return in, substituted
}

// ApplyEscapeSentinels returns in with the escape sentinels stamped when in is an
// overturning-REFUTED (an escape): an empty Domain becomes DomainUnclassified and
// an empty Reason becomes ReasonUnspecified, so every escape carries BOTH a domain
// and a reason. substituted=true when either was stamped. Otherwise it returns in
// unchanged with substituted=false. Pure: the caller supplies the current ledger
// and decides whether to warn. Idempotent — a value already set is left untouched.
func ApplyEscapeSentinels(l *Ledger, in GateVerdictInput) (GateVerdictInput, bool) {
	if !IsOverturningRefuted(l, in) {
		return in, false
	}
	substituted := false
	if in.Domain == "" {
		in.Domain = DomainUnclassified
		substituted = true
	}
	if in.Reason == "" {
		in.Reason = ReasonUnspecified
		substituted = true
	}
	return in, substituted
}
