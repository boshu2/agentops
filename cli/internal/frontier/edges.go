package frontier

// edges.go defines THE two ledger edge shapes the async-membrane compensator
// lane emits and the frontier consumes (age-ekam; synthesis A5/A6/A7), plus
// the classification helpers that read them back. The builders here are the
// production constructors — the compensator lane and the tests emit through
// the same functions, appended via provenancegraph.Store (Seal + hash chain).
//
// Shape 1 — the deterministic L0 compensation verdict edge (arm 3,
// verified-by-compensation). Emitted ONLY after CheckInverse passes; the
// machine verification IS the review (no model):
//
//	from_id      "<p0-bead>@<compensator-sha7>"   (verdict-node convention)
//	from_type    "verdict"
//	to_id        <compensator full sha>
//	to_type      "commit"
//	relation     "wasDerivedFrom"
//	trust_tier   "inferred"
//	evidence_ref "l0-compensation <p0-bead> disposition=CONFIRMED inverse-of=<refuted-full-sha>"
//	bead_id      <p0-bead>
//	reviewer_family "deterministic"
//
// Shape 2 — the resolution edge (arm 4). Appended by the compensator after
// the refuting verdict's repro executes GREEN at the compensating sha:
//
//	from_id      <compensator full sha>
//	from_type    "commit"
//	to_id        <refuted full sha>
//	to_type      "commit"
//	relation     "resolves"
//	trust_tier   "authored"
//	evidence_ref "resolves verdict=<refuting-verdict-node-id> repro=green@<compensator-sha> p0=<p0-bead>"
//	bead_id      <p0-bead>
//
// The evidence_ref token grammar follows the established pawl-verdict
// convention (space-separated key=value fields, exact-token parsing — see
// cli/cmd/ao/provenance_show.go parseDisposition): tokens are matched whole,
// never as substrings, so "xdisposition=CONFIRMED" or a prose mention cannot
// forge a field.

import (
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// compensationEvidencePrefix marks a deterministic L0 compensation verdict
// edge's evidence_ref. Prefix-matched exactly (trailing space included) so a
// prose mention cannot masquerade as a compensation stamp.
const compensationEvidencePrefix = "l0-compensation "

// pawlEvidencePrefix marks a model pawl-verdict edge's evidence_ref (the
// emit-verdict convention: "pawl-verdict <bead> disposition=<D>").
const pawlEvidencePrefix = "pawl-verdict "

// relationResolves is the resolution-edge relation (schema-supported, A7).
const relationResolves = "resolves"

// minSHAPrefixLen is the shortest sha prefix accepted for commit binding,
// matching the provenance_show/done.go convention.
const minSHAPrefixLen = 7

// CompensationVerdictEdge builds the deterministic L0 verdict edge attesting
// that compensatorSHA is the machine-verified inverse patch of refutedSHA.
// Callers MUST run CheckInverse(repo, compensatorSHA, refutedSHA) first and
// emit only on success — and the frontier RE-VERIFIES the proof on read (A8),
// so a stamp without a true inverse never resolves anything.
func CompensationVerdictEdge(p0Bead, compensatorSHA, refutedSHA string) provenancegraph.Edge {
	return provenancegraph.Edge{
		FromID:         p0Bead + "@" + short7(compensatorSHA),
		FromType:       "verdict",
		ToID:           compensatorSHA,
		ToType:         "commit",
		Relation:       "wasDerivedFrom",
		TrustTier:      "inferred",
		EvidenceRef:    compensationEvidencePrefix + p0Bead + " disposition=CONFIRMED inverse-of=" + refutedSHA,
		BeadID:         p0Bead,
		ReviewerFamily: "deterministic",
		TS:             nowUTC(),
	}
}

// ResolutionEdge builds the "resolves" edge from a compensating commit to the
// REFUTED commit it resolves. refutingVerdictID is the from_id of the REFUTED
// verdict edge being compensated ("<bead>@<sha7>"); p0Bead is the auto-filed
// P0 fix bead (A6's binding). The repro=green@<sha> token asserts the
// refuting verdict's repro was executed GREEN at the compensating sha — the
// caller must have actually run it; the frontier rejects the edge when the
// token is absent or names a different sha.
func ResolutionEdge(p0Bead, compensatorSHA, refutedSHA, refutingVerdictID string) provenancegraph.Edge {
	return provenancegraph.Edge{
		FromID:      compensatorSHA,
		FromType:    "commit",
		ToID:        refutedSHA,
		ToType:      "commit",
		Relation:    relationResolves,
		TrustTier:   "authored",
		EvidenceRef: "resolves verdict=" + refutingVerdictID + " repro=green@" + compensatorSHA + " p0=" + p0Bead,
		BeadID:      p0Bead,
		TS:          nowUTC(),
	}
}

// --- ledger classification (read side) ---

// isVerdictEdge reports whether e is a verdict→commit edge (the shape
// emit-verdict writes: relation wasDerivedFrom, from_type verdict, to_type
// commit) — the same discipline as confirmedVerdictEdgeIn in cli/cmd/ao.
func isVerdictEdge(e provenancegraph.Edge) bool {
	return e.Relation == "wasDerivedFrom" && e.FromType == "verdict" && e.ToType == "commit"
}

// refutedVerdicts returns every verdict edge bound to sha carrying an exact
// disposition=REFUTED token. Detection is deliberately broad across verdict
// shapes (pawl or otherwise): an unexplained REFUTED must dominate,
// fail-closed.
func (ev *Evaluator) refutedVerdicts(sha string) []provenancegraph.Edge {
	var out []provenancegraph.Edge
	for _, e := range ev.edges {
		if isVerdictEdge(e) && bindsSHA(sha, e.ToID) && parseToken(e.EvidenceRef, "disposition") == "REFUTED" {
			out = append(out, e)
		}
	}
	return out
}

// hasConfirmedPawlVerdict reports whether sha carries a CONFIRMED pawl-verdict
// edge (arm 1). Deliberately NARROW: only the "pawl-verdict " evidence shape
// counts — an L0 compensation edge also carries disposition=CONFIRMED but must
// route through arm 3, where its machine proof is re-verified, never through
// the bare-confirmation arm.
func (ev *Evaluator) hasConfirmedPawlVerdict(sha string) bool {
	for _, e := range ev.edges {
		if isVerdictEdge(e) && bindsSHA(sha, e.ToID) &&
			strings.HasPrefix(e.EvidenceRef, pawlEvidencePrefix) &&
			parseToken(e.EvidenceRef, "disposition") == "CONFIRMED" {
			return true
		}
	}
	return false
}

// compensationEdgeFor returns the L0 compensation verdict edge bound to sha,
// if exactly one exists. More than one compensation stamp on a single commit
// is ambiguous and fail-closed (ok=false).
func (ev *Evaluator) compensationEdgeFor(sha string) (provenancegraph.Edge, bool) {
	var found []provenancegraph.Edge
	for _, e := range ev.edges {
		if isVerdictEdge(e) && bindsSHA(sha, e.ToID) && strings.HasPrefix(e.EvidenceRef, compensationEvidencePrefix) {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		return provenancegraph.Edge{}, false
	}
	return found[0], true
}

// resolutionEdgesFor returns every live "resolves" edge targeting refuted sha.
// The ledger is append-only with no tombstones, so every committed edge is
// live; uniqueness (exactly one per refuted sha) is enforced by the caller.
func (ev *Evaluator) resolutionEdgesFor(sha string) []provenancegraph.Edge {
	var out []provenancegraph.Edge
	for _, e := range ev.edges {
		if e.Relation == relationResolves && bindsSHA(sha, e.ToID) {
			out = append(out, e)
		}
	}
	return out
}

// isCompensationCommit reports whether sha acts as a compensator anywhere in
// the ledger: it is the source of a resolves edge, or carries an L0
// compensation verdict edge. A REFUTED on such a commit is a revert-of-revert.
func (ev *Evaluator) isCompensationCommit(sha string) bool {
	for _, e := range ev.edges {
		if e.Relation == relationResolves && bindsSHA(sha, e.FromID) {
			return true
		}
		if isVerdictEdge(e) && bindsSHA(sha, e.ToID) && strings.HasPrefix(e.EvidenceRef, compensationEvidencePrefix) {
			return true
		}
	}
	return false
}

// --- token parsing (exact-token, never substring) ---

// parseToken extracts the value of a whole-token "key=value" field from an
// evidence_ref. Same discipline as cli/cmd/ao parseDisposition: fields are
// whitespace-split and prefix-cut exactly, so a substring or prose mention
// never parses as a field. Returns "" when absent.
func parseToken(evidenceRef, key string) string {
	prefix := key + "="
	for _, field := range strings.Fields(evidenceRef) {
		if v, ok := strings.CutPrefix(field, prefix); ok {
			return v
		}
	}
	return ""
}

// isHexToken reports whether s is non-empty hex (either case).
func isHexToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// bindsSHA reports whether two commit identifiers name the same commit:
// either is a case-insensitive hex prefix (>= minSHAPrefixLen) of the other —
// the cli/cmd/ao shaBindsCommit convention.
func bindsSHA(a, b string) bool {
	x, y := strings.ToLower(a), strings.ToLower(b)
	if len(x) < minSHAPrefixLen || len(y) < minSHAPrefixLen {
		return false
	}
	if !isHexToken(x) || !isHexToken(y) {
		return false
	}
	return strings.HasPrefix(x, y) || strings.HasPrefix(y, x)
}

// nowUTC returns the current UTC time in RFC3339 — the ledger ts convention
// (mirrors provenance_emit_verdict.go).
func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
