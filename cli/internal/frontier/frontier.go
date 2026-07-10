// Package frontier computes the LKG (last-known-good) verified frontier over
// a trunk ref: the highest sha whose every ancestor is RESOLVED. It is the R3
// slice of the async-membrane redesign (age-ekam, epic age-xnet; design:
// .agents/plans/2026-07-09-merge-door-SYNTHESIS.md A1+A5-A8, P2 §invariants
// I2/I3/I4) — the invariant "no verdict = not done" relocated from the push
// path to bead closure and the frontier.
//
// The duel-hardened definition (uniform precedence, r3 polish):
//
//	RESOLVED(c) :=
//	  (1) a CONFIRMED pawl-verdict edge bound to c in the provenance ledger
//	∨ (2) verified-by-waiver: c satisfies the #trivial provenance-only
//	      predicate (the Go port of scripts/lib/trivial-waiver.sh, waiver.go)
//	∨ (3) verified-by-compensation: c is a machine-verified inverse patch of a
//	      REFUTED commit, evidenced by a deterministic L0 verdict edge whose
//	      proof is RE-VERIFIED here (CheckInverse — the check validates the
//	      proof, never the stamp; A8)
//	∨ (4) REFUTED(c) ∧ a VALID resolution edge exists (relation "resolves",
//	      strict-descendant compensator, acyclic, unique live edge per refuted
//	      sha, evidence floor = the refuting verdict's repro executed GREEN at
//	      the compensating sha + P0-fix-bead binding; A6/A7), whose compensator
//	      is itself RESOLVED.
//
// PRECEDENCE (uniform): an existing REFUTED verdict on c dominates ALL
// non-resolution arms — a REFUTED commit can only resolve via arm (4), never
// via a bare CONFIRMED, the waiver, or a compensation stamp on the same sha.
// Revert-of-revert (a REFUTED verdict on a compensation commit) is an
// unconditional ANDON: stop the line, no edge counts, the frontier holds.
//
// Consumers (R3b+): the yield report frontier line, the close gate
// (IsResolved), and the land lane.
package frontier

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// State classifies a commit's resolution status.
type State string

// Resolution states. StateAndon is distinct from StateUnresolved: an andon is
// a stop-the-line human escalation (revert war), not merely pending work.
const (
	StateResolved   State = "RESOLVED"
	StateUnresolved State = "UNRESOLVED"
	StateAndon      State = "ANDON"
)

// Arm names which disjunct of RESOLVED(c) resolved a commit.
type Arm string

// Resolution arms, one per disjunct of RESOLVED(c).
const (
	ArmConfirmed    Arm = "confirmed-verdict"
	ArmWaiver       Arm = "verified-by-waiver"
	ArmCompensation Arm = "verified-by-compensation"
	ArmResolution   Arm = "resolved-by-compensator"
)

// Resolution is the evaluated status of one commit.
type Resolution struct {
	// SHA is the full commit sha evaluated.
	SHA string
	// State is the resolution outcome.
	State State
	// Arm names the disjunct that resolved the commit (StateResolved only).
	Arm Arm
	// Reason explains an UNRESOLVED or ANDON outcome (empty when resolved).
	Reason string
}

// defaultBound is the walk depth when the caller passes bound <= 0: generous
// enough to cover any sane pending window (cap N≈3 per the synthesis) plus
// bind-commit interleave, cheap enough to stay sub-second.
const defaultBound = 200

// Result is a computed LKG frontier over a walked window of a trunk ref.
type Result struct {
	// Ref is the ref that was walked (e.g. "origin/main").
	Ref string
	// Tip is the full sha of the ref tip.
	Tip string
	// SHA is the LKG frontier: the highest walked sha whose every ancestor
	// within the window is RESOLVED. Commits below the walked horizon are the
	// pre-membrane verified prefix and are assumed resolved (the bounded-walk
	// floor) — callers must size bound to cover at least the pending window.
	// Empty when the walk reached the root without finding a resolved commit.
	SHA string
	// Andon is true when any walked commit is in ANDON state (revert-of-
	// revert). The frontier never advances past an andon'd commit.
	Andon bool
	// AndonReasons carries one reason per andon'd commit, walk order.
	AndonReasons []string
	// Pending lists the walked commits above the frontier (not yet part of
	// the verified prefix), newest first, with their evaluated Resolution.
	Pending []Resolution
	// Walked is the number of commits evaluated (window size actually seen).
	Walked int
}

// Evaluator evaluates RESOLVED(c) over one git repo and one immutable
// provenance-ledger snapshot. Results are memoized per full sha.
type Evaluator struct {
	repo  string
	edges []provenancegraph.Edge
	memo  map[string]Resolution
}

// NewEvaluator returns an Evaluator over repo (a git checkout root) and edges
// (a ledger snapshot, e.g. provenancegraph.Store.Read()).
func NewEvaluator(repo string, edges []provenancegraph.Edge) *Evaluator {
	return &Evaluator{repo: repo, edges: edges, memo: map[string]Resolution{}}
}

// Resolve evaluates RESOLVED(sha) and returns the commit's Resolution. A sha
// that does not resolve to a commit in the repo is UNRESOLVED (fail-closed),
// never an error: an unverifiable object cannot enter the verified prefix.
func (ev *Evaluator) Resolve(sha string) Resolution {
	full, err := ev.fullSHA(sha)
	if err != nil {
		return Resolution{SHA: sha, State: StateUnresolved,
			Reason: fmt.Sprintf("sha %s does not resolve to a commit: %v", short7(sha), err)}
	}
	return ev.resolve(full, map[string]bool{})
}

// resolve is the memoized, cycle-guarded core of Resolve. visiting carries
// the resolution-edge recursion path: revisiting a sha means the "resolves"
// chain is cyclic, which is fail-closed UNRESOLVED (acyclicity, A7). Cycle
// verdicts are not memoized — they depend on the entry point of the walk.
func (ev *Evaluator) resolve(full string, visiting map[string]bool) Resolution {
	if r, ok := ev.memo[full]; ok {
		return r
	}
	if visiting[full] {
		return Resolution{SHA: full, State: StateUnresolved,
			Reason: fmt.Sprintf("resolution-edge cycle at %s — acyclicity violated, fail-closed", short7(full))}
	}
	visiting[full] = true
	defer delete(visiting, full)

	r := ev.evaluate(full, visiting)
	ev.memo[full] = r
	return r
}

// evaluate applies the four arms of RESOLVED(c) with uniform precedence.
func (ev *Evaluator) evaluate(full string, visiting map[string]bool) Resolution {
	refuted := ev.refutedVerdicts(full)
	if len(refuted) > 0 {
		// Revert-of-revert: a REFUTED verdict on a compensation commit is an
		// unconditional andon — no edge counts, a human arbitrates.
		if ev.isCompensationCommit(full) {
			return Resolution{SHA: full, State: StateAndon,
				Reason: fmt.Sprintf("revert-of-revert: compensation commit %s is itself REFUTED — stop the line, ledger arbitrates", short7(full))}
		}
		// PRECEDENCE: REFUTED dominates every non-resolution arm.
		return ev.resolveViaResolutionEdge(full, refuted, visiting)
	}

	// Arm 1: CONFIRMED pawl-verdict edge bound to c.
	if ev.hasConfirmedPawlVerdict(full) {
		return Resolution{SHA: full, State: StateResolved, Arm: ArmConfirmed}
	}
	// Arm 2: verified-by-waiver (#trivial provenance-only predicate).
	if st, _ := TrivialWaiver(ev.repo, full); st == WaiverWaived {
		return Resolution{SHA: full, State: StateResolved, Arm: ArmWaiver}
	}
	// Arm 3: verified-by-compensation (L0 edge + re-verified inverse proof).
	if r, claimed := ev.resolveViaCompensation(full); claimed {
		return r
	}
	return Resolution{SHA: full, State: StateUnresolved,
		Reason: fmt.Sprintf("no verdict, waiver, or compensation proof bound to %s (pending review)", short7(full))}
}

// resolveViaCompensation implements arm 3. Returns (resolution, true) when an
// L0 compensation edge is bound to full — resolved only if the deterministic
// proof RE-VERIFIES (A8: validate the proof, never the stamp); an edge whose
// proof fails is fail-closed UNRESOLVED with a loud reason. (nil, false) when
// no compensation edge claims this commit.
func (ev *Evaluator) resolveViaCompensation(full string) (Resolution, bool) {
	edge, ok := ev.compensationEdgeFor(full)
	if !ok {
		return Resolution{}, false
	}
	refutedSHA := parseToken(edge.EvidenceRef, "inverse-of")
	if refutedSHA == "" {
		return Resolution{SHA: full, State: StateUnresolved,
			Reason: fmt.Sprintf("compensation edge on %s carries no inverse-of target — fail-closed", short7(full))}, true
	}
	if parseToken(edge.EvidenceRef, "disposition") != "CONFIRMED" {
		return Resolution{SHA: full, State: StateUnresolved,
			Reason: fmt.Sprintf("compensation edge on %s is not disposition=CONFIRMED — fail-closed", short7(full))}, true
	}
	// The inverted commit must actually be REFUTED: compensation exists only
	// as the inverse of a refuted change (A5).
	target, err := ev.fullSHA(refutedSHA)
	if err != nil || len(ev.refutedVerdicts(target)) == 0 {
		return Resolution{SHA: full, State: StateUnresolved,
			Reason: fmt.Sprintf("compensation edge on %s inverts %s, which carries no REFUTED verdict — fail-closed", short7(full), short7(refutedSHA))}, true
	}
	// Re-verify the machine proof: the verification IS the review.
	if err := CheckInverse(ev.repo, full, target); err != nil {
		return Resolution{SHA: full, State: StateUnresolved,
			Reason: fmt.Sprintf("compensation proof for %s failed re-verification: %v", short7(full), err)}, true
	}
	return Resolution{SHA: full, State: StateResolved, Arm: ArmCompensation}, true
}

// resolveViaResolutionEdge implements arm 4: REFUTED(c) resolves only through
// a VALID resolution edge to a strict-descendant compensator that is itself
// RESOLVED. Every validation failure is fail-closed UNRESOLVED (the frontier
// holds); a REFUTED compensator is an unconditional ANDON.
func (ev *Evaluator) resolveViaResolutionEdge(full string, refuted []provenancegraph.Edge, visiting map[string]bool) Resolution {
	unresolved := func(format string, args ...any) Resolution {
		return Resolution{SHA: full, State: StateUnresolved, Reason: fmt.Sprintf(format, args...)}
	}

	edges := ev.resolutionEdgesFor(full)
	if len(edges) == 0 {
		return unresolved("REFUTED at %s with no resolution edge — frontier holds until a compensator lands", short7(full))
	}
	if len(edges) > 1 {
		return unresolved("uniqueness violation: %d live resolution edges for %s (exactly one allowed) — fail-closed", len(edges), short7(full))
	}
	edge := edges[0]
	if edge.FromType != "commit" || edge.ToType != "commit" {
		return unresolved("malformed resolution edge for %s: endpoints must be commit→commit", short7(full))
	}
	comp, err := ev.fullSHA(edge.FromID)
	if err != nil {
		return unresolved("resolution edge for %s names unresolvable compensator %s — fail-closed", short7(full), short7(edge.FromID))
	}
	// Revert-of-revert via the edge: the compensator is itself REFUTED.
	if len(ev.refutedVerdicts(comp)) > 0 {
		return Resolution{SHA: full, State: StateAndon,
			Reason: fmt.Sprintf("revert-of-revert: compensator %s of REFUTED %s is itself REFUTED — no edge counts, stop the line", short7(comp), short7(full))}
	}
	if !ev.strictDescendant(full, comp) {
		return unresolved("compensator %s is not a strict descendant of refuted %s — fail-closed", short7(comp), short7(full))
	}
	if reason := validateEvidenceFloor(edge, comp, refuted); reason != "" {
		return unresolved("resolution edge for %s rejected: %s", short7(full), reason)
	}
	// The compensator must itself be RESOLVED (recursion, cycle-guarded).
	cr := ev.resolve(comp, visiting)
	switch cr.State {
	case StateAndon:
		return Resolution{SHA: full, State: StateAndon, Reason: cr.Reason}
	case StateResolved:
		return Resolution{SHA: full, State: StateResolved, Arm: ArmResolution}
	default:
		return unresolved("compensator %s of %s is not itself RESOLVED: %s", short7(comp), short7(full), cr.Reason)
	}
}

// validateEvidenceFloor enforces A6 on a resolution edge: (i) the refuting
// verdict named by the edge must be one actually bound to the refuted sha,
// (ii) the refuting verdict's repro must be recorded GREEN at the compensating
// sha, (iii) the edge must bind the auto-filed P0 fix bead. Returns "" when
// the floor holds, else the rejection reason.
func validateEvidenceFloor(edge provenancegraph.Edge, comp string, refuted []provenancegraph.Edge) string {
	verdictID := parseToken(edge.EvidenceRef, "verdict")
	if verdictID == "" {
		return "no refuting-verdict binding (verdict=... missing)"
	}
	known := false
	for _, r := range refuted {
		if r.FromID == verdictID {
			known = true
			break
		}
	}
	if !known {
		return fmt.Sprintf("verdict=%s does not match any REFUTED verdict bound to the refuted sha", verdictID)
	}
	repro := parseToken(edge.EvidenceRef, "repro")
	status, reproSHA, ok := strings.Cut(repro, "@")
	if !ok || !strings.EqualFold(status, "green") {
		return "no green repro record (repro=green@<compensating-sha> missing) — a superficial fix must not advance the frontier"
	}
	if !bindsSHA(reproSHA, comp) {
		return fmt.Sprintf("repro recorded green at %s, not at the compensating sha %s", short7(reproSHA), short7(comp))
	}
	p0 := parseToken(edge.EvidenceRef, "p0")
	if p0 == "" || edge.BeadID != p0 {
		return "no P0 fix-bead binding (p0=<bead> token and bead_id must both carry the auto-filed fix bead)"
	}
	return ""
}

// IsResolved is the close-gate query (the R3b seam): given a bead id or a
// commit sha, report whether the referenced landed commit(s) are RESOLVED.
//
//   - A hex string (>=7 chars) is treated as a commit sha and evaluated
//     directly.
//   - Anything else is treated as a bead id: its landed shas are the commits
//     bound by the bead's verdict edges (from_id "<bead>@...", bead_id) and
//     wasGeneratedBy bead→commit edges. ALL landed shas must be RESOLVED.
//     A bead with no landed sha in the ledger returns an error — closure
//     cannot be proven for work the ledger never saw (fail-closed).
//
// The returned Resolutions carry the per-sha detail for refusal messages.
func (ev *Evaluator) IsResolved(beadOrSHA string) (bool, []Resolution, error) {
	ref := strings.TrimSpace(beadOrSHA)
	if ref == "" {
		return false, nil, fmt.Errorf("empty bead/sha reference")
	}
	if isHexToken(ref) && len(ref) >= minSHAPrefixLen {
		r := ev.Resolve(ref)
		return r.State == StateResolved, []Resolution{r}, nil
	}
	shas := ev.landedSHAsForBead(ref)
	if len(shas) == 0 {
		return false, nil, fmt.Errorf("bead %q has no landed commit bound in the provenance ledger — cannot prove resolution (fail-closed)", ref)
	}
	all := true
	rs := make([]Resolution, 0, len(shas))
	for _, sha := range shas {
		r := ev.Resolve(sha)
		rs = append(rs, r)
		if r.State != StateResolved {
			all = false
		}
	}
	return all, rs, nil
}

// landedSHAsForBead collects the distinct commit shas the ledger binds to a
// bead: verdict edges whose node id is "<bead>@<sha7>" (or bead_id matches)
// and wasGeneratedBy bead→commit edges. First-seen order.
func (ev *Evaluator) landedSHAsForBead(bead string) []string {
	seen := map[string]bool{}
	var shas []string
	add := func(sha string) {
		if isHexToken(sha) && len(sha) >= minSHAPrefixLen && !seen[sha] {
			seen[sha] = true
			shas = append(shas, sha)
		}
	}
	for _, e := range ev.edges {
		switch {
		case isVerdictEdge(e) && (e.BeadID == bead || strings.HasPrefix(e.FromID, bead+"@")):
			add(e.ToID)
		case e.Relation == "wasGeneratedBy" && e.FromType == "bead" && e.FromID == bead && e.ToType == "commit":
			add(e.ToID)
		}
	}
	return shas
}

// Compute walks up to bound commits of ref in repo and returns the LKG
// frontier over edges (a ledger snapshot). bound <= 0 uses defaultBound. The
// frontier is always a FIRST-PARENT-LINEAGE commit of ref (the mainline) —
// merge side-branch shas are never candidates. The walk is ancestry-exact
// within the window: a candidate qualifies only when it and every ancestor
// reachable inside the window are RESOLVED (merge parents included —
// invariant I3); parents beyond the horizon are the pre-membrane verified
// prefix, assumed resolved (callers size bound to cover at least the pending
// window).
func Compute(repo string, edges []provenancegraph.Edge, ref string, bound int) (*Result, error) {
	if bound <= 0 {
		bound = defaultBound
	}
	order, parents, err := revListParents(repo, ref, bound)
	if err != nil {
		return nil, err
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("ref %q has no commits to walk", ref)
	}

	ev := NewEvaluator(repo, edges)
	window := make(map[string]bool, len(order))
	for _, sha := range order {
		window[sha] = true
	}

	res := &Result{Ref: ref, Tip: order[0], Walked: len(order)}
	resolutions := make(map[string]Resolution, len(order))
	for _, sha := range order {
		r := ev.Resolve(sha)
		resolutions[sha] = r
		if r.State == StateAndon {
			res.Andon = true
			res.AndonReasons = append(res.AndonReasons, r.Reason)
		}
	}

	frontier := findFrontier(order, parents, window, resolutions)
	if frontier == "" {
		// Nothing in the window qualifies: the frontier lies at the horizon
		// floor (the parent below the oldest walked commit), if the walk was
		// cut by bound rather than by running out of history.
		frontier = horizonFloor(order, parents, window)
	}
	res.SHA = frontier

	covered := ancestryClosure(frontier, parents, window)
	for _, sha := range order {
		if !covered[sha] {
			res.Pending = append(res.Pending, resolutions[sha])
		}
	}
	return res, nil
}

// findFrontier returns the newest first-parent-lineage sha whose window-
// ancestry is fully RESOLVED, or "" when none qualifies. CANDIDATES are
// restricted to the ref's first-parent chain from the tip (order[0], then
// parents[sha][0], ...): the frontier is a done-pointer onto the trunk's
// mainline, and a pointer to a merge side-branch sha is meaningless.
// Resolution COVERAGE still spans ALL parents (memoized DFS over the full
// parent graph) — a merge's side branch must be fully RESOLVED for the
// mainline commit above it to qualify (invariant I3).
func findFrontier(order []string, parents map[string][]string, window map[string]bool, resolutions map[string]Resolution) string {
	memo := map[string]bool{}
	var allResolved func(sha string) bool
	allResolved = func(sha string) bool {
		if v, ok := memo[sha]; ok {
			return v
		}
		memo[sha] = false // cycle guard; git DAGs are acyclic but stay safe
		if resolutions[sha].State != StateResolved {
			return false
		}
		for _, p := range parents[sha] {
			if window[p] && !allResolved(p) {
				return false
			}
		}
		memo[sha] = true
		return true
	}
	for sha := order[0]; window[sha]; {
		if allResolved(sha) {
			return sha
		}
		ps := parents[sha]
		if len(ps) == 0 {
			break // mainline root reached inside the window
		}
		sha = ps[0]
	}
	return ""
}

// horizonFloor returns the first-parent commit hanging below the walked window
// (the assumed-resolved pre-membrane prefix), or "" when the mainline chain
// reached a root inside the window. Like findFrontier, the floor stays on the
// ref's first-parent lineage — the fallback frontier must be a mainline
// commit too, never a side-branch parent that happens to dangle below the
// horizon.
func horizonFloor(order []string, parents map[string][]string, window map[string]bool) string {
	for sha := order[0]; ; {
		ps := parents[sha]
		if len(ps) == 0 {
			return "" // root: no pre-membrane prefix below
		}
		next := ps[0]
		if !window[next] {
			return next
		}
		sha = next
	}
}

// ancestryClosure returns the set of window commits at-or-below start
// (inclusive), following parent links inside the window.
func ancestryClosure(start string, parents map[string][]string, window map[string]bool) map[string]bool {
	covered := map[string]bool{}
	if !window[start] {
		return covered
	}
	stack := []string{start}
	for len(stack) > 0 {
		sha := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if covered[sha] {
			continue
		}
		covered[sha] = true
		for _, p := range parents[sha] {
			if window[p] && !covered[p] {
				stack = append(stack, p)
			}
		}
	}
	return covered
}

// --- git plumbing (repo-scoped, deterministic flags) ---

// gitOutput runs git -C repo args... and returns trimmed stdout.
func gitOutput(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...) // #nosec G204 -- fixed binary; args are package-internal git plumbing
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// fullSHA resolves any commit-ish to its full sha, failing on non-commits.
func (ev *Evaluator) fullSHA(sha string) (string, error) {
	return gitOutput(ev.repo, "rev-parse", "--verify", "--quiet", sha+"^{commit}")
}

// strictDescendant reports whether descendant strictly descends from ancestor
// (ancestor reachable from descendant, and not the same commit). Any git
// failure is fail-closed false.
func (ev *Evaluator) strictDescendant(ancestor, descendant string) bool {
	if ancestor == descendant {
		return false
	}
	cmd := exec.Command("git", "-C", ev.repo, "merge-base", "--is-ancestor", ancestor, descendant) // #nosec G204 -- fixed binary; shas are rev-parse-verified
	return cmd.Run() == nil
}

// revListParents walks ref up to bound commits and returns the commit order
// (newest first) plus each commit's parent list.
func revListParents(repo, ref string, bound int) ([]string, map[string][]string, error) {
	out, err := gitOutput(repo, "rev-list", "--parents", "-n", fmt.Sprintf("%d", bound), ref)
	if err != nil {
		return nil, nil, fmt.Errorf("walking %s: %w", ref, err)
	}
	var order []string
	parents := map[string][]string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		order = append(order, fields[0])
		parents[fields[0]] = fields[1:]
	}
	return order, parents, nil
}

// short7 returns the 7-char short form of a sha for messages.
func short7(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
