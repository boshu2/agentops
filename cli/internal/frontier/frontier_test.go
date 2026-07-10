package frontier

// The five frontier-liveness acceptance tests (age-ekam, R3a of the async-
// membrane epic age-xnet). These ARE the slice contract — Gherkin from
// .agents/plans/2026-07-09-merge-door-SYNTHESIS.md (A1, A5-A8), written and
// EXECUTED RED before the implementation:
//
//	(a) REFUTED c1 → fix-forward c2 CONFIRMED + valid resolves edge
//	    ⇒ the frontier advances past both.
//	(b) REFUTED c1 → mechanical revert c2 with an L0 compensation edge +
//	    repro-green resolves edge ⇒ advances.
//	(c) an unresolved REFUTED holds the frontier; IsResolved(bead|sha) is the
//	    close-gate query (the R3b seam).
//	(d) a #trivial provenance-only commit advances via waiver; a REFUTED
//	    provenance-only commit does NOT (precedence: REFUTED dominates all
//	    non-resolution arms).
//	(e) revert-of-revert (a REFUTED on a compensation commit) ⇒ ANDON state,
//	    no advancement, no edge counts.
//
// Fixture fidelity (.claude/rules/go.md): every ledger fixture is round-
// tripped through the PRODUCTION writer/reader — provenancegraph.Store.Append
// (Seal + ValidateFields + hash chain) writes, Store.Read reads. Verdict-edge
// field shape mirrors buildVerdictCommitEdge (cli/cmd/ao/provenance_emit_verdict.go)
// and the committed docs/provenance/ledger.jsonl lines byte-for-byte in the
// fields that matter (from_id "<bead>@<sha7>", evidence_ref
// "pawl-verdict <bead> disposition=<D>", relation wasDerivedFrom, bead_id,
// trust_tier inferred). Compensation/resolution edges use this package's OWN
// production builders (CompensationVerdictEdge, ResolutionEdge) — the same
// constructors the compensator lane emits with. Git state is a real repo per
// test (t.TempDir), never a hand-built mock.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// --- git fixture helpers (real repos, real commits) ---

// gitRun runs git with args against repo and fails the test on error.
func gitRun(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=frontier-test", "GIT_AUTHOR_EMAIL=frontier@test",
		"GIT_COMMITTER_NAME=frontier-test", "GIT_COMMITTER_EMAIL=frontier@test",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// initRepo creates a fresh git repo with one base commit and returns
// (repoPath, baseSHA).
func initRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q", "-b", "main")
	base := commitFile(t, repo, "README.md", "base\n", "feat: base commit")
	return repo, base
}

// commitFile writes path (relative to repo) with content, commits it with msg,
// and returns the full commit sha.
func commitFile(t *testing.T, repo, rel, content, msg string) string {
	t.Helper()
	abs := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "--no-verify", "-m", msg)
	return gitRun(t, repo, "rev-parse", "HEAD")
}

// revertCommit mechanically reverts sha (git revert --no-edit) and returns the
// revert commit's sha — the production shape of the L0 mechanical-revert lane.
func revertCommit(t *testing.T, repo, sha string) string {
	t.Helper()
	gitRun(t, repo, "revert", "--no-edit", sha)
	return gitRun(t, repo, "rev-parse", "HEAD")
}

// gitRunAt is gitRun with a pinned author+committer date, for topology tests
// whose assertions must not depend on the second-granular fixture clock
// (rev-list emits the newest pending commit first, so sibling order across a
// merge is committer-date order).
func gitRunAt(t *testing.T, repo, stamp string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=frontier-test", "GIT_AUTHOR_EMAIL=frontier@test",
		"GIT_COMMITTER_NAME=frontier-test", "GIT_COMMITTER_EMAIL=frontier@test",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_DATE="+stamp, "GIT_COMMITTER_DATE="+stamp,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitFileAt is commitFile with a pinned commit date (via gitRunAt).
func commitFileAt(t *testing.T, repo, rel, content, msg, stamp string) string {
	t.Helper()
	abs := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitRunAt(t, repo, stamp, "add", "-A")
	gitRunAt(t, repo, stamp, "commit", "-q", "--no-verify", "-m", msg)
	return gitRunAt(t, repo, stamp, "rev-parse", "HEAD")
}

// --- ledger fixture helpers (production writer/reader round-trip) ---

// newLedger returns a Store on a fresh path OUTSIDE any git repo so ledger
// writes never perturb the waiver predicate's path checks.
func newLedger(t *testing.T) *provenancegraph.Store {
	t.Helper()
	return provenancegraph.NewStore(filepath.Join(t.TempDir(), "ledger.jsonl"))
}

// appendVerdict appends a pawl-verdict edge for (bead, sha, disposition) in the
// exact production shape of buildVerdictCommitEdge / the committed ledger:
// from_id "<bead>@<sha7>", from_type verdict, to_id full sha, to_type commit,
// relation wasDerivedFrom, trust_tier inferred, evidence_ref
// "pawl-verdict <bead> disposition=<D>", bead_id set.
func appendVerdict(t *testing.T, store *provenancegraph.Store, bead, sha, disposition string) provenancegraph.Edge {
	t.Helper()
	e := provenancegraph.Edge{
		FromID:      bead + "@" + sha[:7],
		FromType:    "verdict",
		ToID:        sha,
		ToType:      "commit",
		Relation:    "wasDerivedFrom",
		TrustTier:   "inferred",
		EvidenceRef: "pawl-verdict " + bead + " disposition=" + disposition,
		BeadID:      bead,
		TS:          nowUTC(),
	}
	res, err := store.Append(e)
	if err != nil {
		t.Fatalf("append %s verdict for %s@%s: %v", disposition, bead, sha[:7], err)
	}
	return res.Edge
}

// mustAppend appends an already-built edge (from a production builder) and
// fails the test on any validation/seal error — proving the builder emits a
// schema-valid edge the production writer accepts.
func mustAppend(t *testing.T, store *provenancegraph.Store, e provenancegraph.Edge) provenancegraph.Edge {
	t.Helper()
	res, err := store.Append(e)
	if err != nil {
		t.Fatalf("append edge %s --%s--> %s: %v", e.FromID, e.Relation, e.ToID, err)
	}
	return res.Edge
}

// readLedger round-trips the ledger through the production reader.
func readLedger(t *testing.T, store *provenancegraph.Store) []provenancegraph.Edge {
	t.Helper()
	edges, err := store.Read()
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	return edges
}

// computeFrontier runs Compute and fails the test on error.
func computeFrontier(t *testing.T, repo string, edges []provenancegraph.Edge) *Result {
	t.Helper()
	res, err := Compute(repo, edges, "HEAD", 50)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	return res
}

// --- (a) REFUTED → fix-forward CONFIRMED + valid resolves edge ⇒ advances ---

func TestFrontier_RefutedThenFixForwardAdvances(t *testing.T) {
	repo, base := initRepo(t)
	c1 := commitFile(t, repo, "pkg/feature.go", "broken\n", "feat: c1 lands broken")
	c2 := commitFile(t, repo, "pkg/feature.go", "fixed\n", "fix: c2 fix-forward")

	store := newLedger(t)
	appendVerdict(t, store, "age-base", base, "CONFIRMED")
	refuting := appendVerdict(t, store, "age-c1", c1, "REFUTED")
	appendVerdict(t, store, "age-c1-fix", c2, "CONFIRMED")
	mustAppend(t, store, ResolutionEdge("age-c1-fix", c2, c1, refuting.FromID))

	res := computeFrontier(t, repo, readLedger(t, store))
	if res.SHA != c2 {
		t.Errorf("frontier = %q, want fix-forward tip %q (must advance past REFUTED c1 AND its compensator)", res.SHA, c2)
	}
	if res.Andon {
		t.Errorf("Andon = true, want false: a validly resolved REFUTED is not an andon")
	}

	ev := NewEvaluator(repo, readLedger(t, store))
	r := ev.Resolve(c1)
	if r.State != StateResolved {
		t.Errorf("Resolve(c1) state = %s (%s), want RESOLVED via resolution edge", r.State, r.Reason)
	}
	if r.Arm != ArmResolution {
		t.Errorf("Resolve(c1) arm = %s, want %s", r.Arm, ArmResolution)
	}
}

// --- (b) REFUTED → mechanical revert + L0 compensation + resolves ⇒ advances ---

func TestFrontier_RefutedThenMechanicalRevertAdvances(t *testing.T) {
	repo, base := initRepo(t)
	c1 := commitFile(t, repo, "pkg/feature.go", "regression\n", "feat: c1 lands a regression")
	c2 := revertCommit(t, repo, c1)

	// The checker is the machine verification the L0 edge attests: a true
	// mechanical revert MUST pass it before the edge may be emitted.
	if err := CheckInverse(repo, c2, c1); err != nil {
		t.Fatalf("CheckInverse(revert, refuted) = %v, want nil for a mechanical git revert", err)
	}

	store := newLedger(t)
	appendVerdict(t, store, "age-base", base, "CONFIRMED")
	refuting := appendVerdict(t, store, "age-c1", c1, "REFUTED")
	// NO model CONFIRMED verdict on c2 by design (A5): the deterministic
	// inverse verification IS the review.
	mustAppend(t, store, CompensationVerdictEdge("age-c1-fix", c2, c1))
	mustAppend(t, store, ResolutionEdge("age-c1-fix", c2, c1, refuting.FromID))

	res := computeFrontier(t, repo, readLedger(t, store))
	if res.SHA != c2 {
		t.Errorf("frontier = %q, want revert tip %q (mechanical revert resolves via verified-by-compensation)", res.SHA, c2)
	}
	if res.Andon {
		t.Errorf("Andon = true, want false")
	}

	ev := NewEvaluator(repo, readLedger(t, store))
	r := ev.Resolve(c2)
	if r.State != StateResolved || r.Arm != ArmCompensation {
		t.Errorf("Resolve(c2) = state %s arm %s (%s), want RESOLVED via %s", r.State, r.Arm, r.Reason, ArmCompensation)
	}
}

// --- (c) unresolved REFUTED holds the frontier; IsResolved is the R3b seam ---

func TestFrontier_UnresolvedRefutedHoldsFrontier(t *testing.T) {
	repo, base := initRepo(t)
	c1 := commitFile(t, repo, "pkg/broken.go", "broken\n", "feat: c1 refuted, never resolved")
	c2 := commitFile(t, repo, "pkg/other.go", "fine\n", "feat: c2 confirmed on top")

	store := newLedger(t)
	appendVerdict(t, store, "age-base", base, "CONFIRMED")
	appendVerdict(t, store, "age-c1", c1, "REFUTED")
	appendVerdict(t, store, "age-c2", c2, "CONFIRMED")

	edges := readLedger(t, store)
	res := computeFrontier(t, repo, edges)
	if res.SHA != base {
		t.Errorf("frontier = %q, want %q: an unresolved REFUTED ancestor must hold the frontier even under a CONFIRMED tip", res.SHA, base)
	}

	// The close-gate query (R3b seam): by sha and by bead.
	ev := NewEvaluator(repo, edges)
	if ok, rs, err := ev.IsResolved(c1); err != nil || ok {
		t.Errorf("IsResolved(c1 sha) = (%v, %+v, %v), want unresolved", ok, rs, err)
	}
	if ok, _, err := ev.IsResolved("age-c1"); err != nil || ok {
		t.Errorf("IsResolved(age-c1 bead) = (%v, _, %v), want (false, nil): the close gate must refuse this bead", ok, err)
	}
	if ok, _, err := ev.IsResolved("age-c2"); err != nil || !ok {
		t.Errorf("IsResolved(age-c2 bead) = (%v, _, %v), want (true, nil): c2's own sha IS resolved even while the frontier holds", ok, err)
	}
	if _, _, err := ev.IsResolved("age-never-landed"); err == nil {
		t.Errorf("IsResolved(unknown bead) error = nil, want error: a bead with no landed sha cannot be proven resolved")
	}
}

// --- (d) #trivial advances via waiver; REFUTED provenance-only does NOT ---

func TestFrontier_TrivialWaiverAdvances_RefutedDominatesWaiver(t *testing.T) {
	repo, base := initRepo(t)
	t1 := commitFile(t, repo, "docs/provenance/ledger.jsonl", "{\"edge\":1}\n",
		"chore(provenance): bind pawl verdict #trivial")
	t2 := commitFile(t, repo, "docs/provenance/ledger.jsonl", "{\"edge\":1}\n{\"edge\":2}\n",
		"chore(provenance): bind another verdict #trivial")

	store := newLedger(t)
	appendVerdict(t, store, "age-base", base, "CONFIRMED")
	// t1 carries NO verdict — it must resolve via the waiver arm alone.
	// t2 is byte-for-byte the same waiver shape but carries a REFUTED verdict:
	// precedence says REFUTED dominates the waiver arm (A7 hardened in r3).
	appendVerdict(t, store, "age-t2", t2, "REFUTED")

	edges := readLedger(t, store)
	res := computeFrontier(t, repo, edges)
	if res.SHA != t1 {
		t.Errorf("frontier = %q, want %q: waiver advances t1, REFUTED t2 must NOT waiver-resolve", res.SHA, t1)
	}

	ev := NewEvaluator(repo, edges)
	r1 := ev.Resolve(t1)
	if r1.State != StateResolved || r1.Arm != ArmWaiver {
		t.Errorf("Resolve(t1) = state %s arm %s (%s), want RESOLVED via %s", r1.State, r1.Arm, r1.Reason, ArmWaiver)
	}
	r2 := ev.Resolve(t2)
	if r2.State != StateUnresolved {
		t.Errorf("Resolve(t2) state = %s, want UNRESOLVED: REFUTED dominates the waiver arm", r2.State)
	}
	if !strings.Contains(r2.Reason, "REFUTED") {
		t.Errorf("Resolve(t2) reason = %q, want it to name the dominating REFUTED verdict", r2.Reason)
	}
}

// --- (e) revert-of-revert ⇒ ANDON, no advancement, no edge counts ---

func TestFrontier_RevertOfRevertAndon(t *testing.T) {
	repo, base := initRepo(t)
	c1 := commitFile(t, repo, "pkg/feature.go", "v1\n", "feat: c1 lands, later refuted")
	c2 := revertCommit(t, repo, c1)

	store := newLedger(t)
	appendVerdict(t, store, "age-base", base, "CONFIRMED")
	refuting := appendVerdict(t, store, "age-c1", c1, "REFUTED")
	mustAppend(t, store, CompensationVerdictEdge("age-c1-fix", c2, c1))
	mustAppend(t, store, ResolutionEdge("age-c1-fix", c2, c1, refuting.FromID))
	// The revert war: the compensation commit itself gets REFUTED.
	appendVerdict(t, store, "age-c1-fix", c2, "REFUTED")

	edges := readLedger(t, store)
	res := computeFrontier(t, repo, edges)
	if !res.Andon {
		t.Fatalf("Andon = false, want true: a REFUTED on a compensation commit is an unconditional andon")
	}
	if res.SHA != base {
		t.Errorf("frontier = %q, want %q: no advancement under andon — no edge counts", res.SHA, base)
	}
	if len(res.AndonReasons) == 0 || !strings.Contains(strings.Join(res.AndonReasons, " "), "revert-of-revert") {
		t.Errorf("AndonReasons = %v, want a revert-of-revert reason naming the contested commit", res.AndonReasons)
	}

	ev := NewEvaluator(repo, edges)
	if r := ev.Resolve(c2); r.State != StateAndon {
		t.Errorf("Resolve(c2) state = %s (%s), want ANDON", r.State, r.Reason)
	}
	// c1's resolution edge points at an andon'd compensator: it must not count.
	if r := ev.Resolve(c1); r.State == StateResolved {
		t.Errorf("Resolve(c1) = RESOLVED, want not-resolved: an edge to an andon'd compensator never counts")
	}
}

// --- the mainline rule: frontier candidates come from the first-parent chain ---

// TestFrontier_MergeSideBranchNeverCandidate pins the first-parent-lineage
// rule: frontier CANDIDATES are restricted to the trunk's first-parent chain.
// In a merge where the mainline commit M is unresolved but the side-branch
// commit S is RESOLVED (and newer, so a full-ancestry candidate walk would
// reach S first and its {S, base} closure is all-RESOLVED), the frontier must
// NOT be S — a done-pointer onto a merge side-branch sha is meaningless. It
// must be the highest resolved first-parent commit at/below the merge base.
// Resolving M then advances the frontier to M; resolving the merge commit too
// advances it to the tip (the merge qualifies because S is also resolved).
func TestFrontier_MergeSideBranchNeverCandidate(t *testing.T) {
	repo, base := initRepo(t)
	// Side branch off base: S is CONFIRMED and pinned NEWER than mainline M,
	// so a full-ancestry walk visits S before M (the exact defect shape).
	gitRunAt(t, repo, "2027-01-01T01:00:00Z", "checkout", "-q", "-b", "side")
	s := commitFileAt(t, repo, "pkg/side.go", "side\n", "feat: side-branch slice, confirmed", "2027-01-01T02:00:00Z")
	gitRunAt(t, repo, "2027-01-01T02:30:00Z", "checkout", "-q", "main")
	m := commitFileAt(t, repo, "pkg/mainline.go", "mainline\n", "feat: mainline slice, unverified", "2027-01-01T01:30:00Z")
	gitRunAt(t, repo, "2027-01-01T03:00:00Z", "merge", "--no-ff", "--no-edit", "side")
	merge := gitRun(t, repo, "rev-parse", "HEAD")

	store := newLedger(t)
	appendVerdict(t, store, "age-base", base, "CONFIRMED")
	appendVerdict(t, store, "age-side", s, "CONFIRMED")
	// M (mainline) and the merge commit carry NO verdicts.

	res := computeFrontier(t, repo, readLedger(t, store))
	if res.SHA == s {
		t.Fatalf("frontier = %q — a merge SIDE-BRANCH sha was selected; the frontier must be a first-parent-lineage commit of the ref", s)
	}
	if res.SHA != base {
		t.Errorf("frontier = %q, want merge-base %q (the highest resolved first-parent commit below unresolved mainline %s)", res.SHA, base, m[:7])
	}

	// Resolving M advances the frontier along the mainline — to M, not yet the
	// merge (the merge commit itself is still unverdicted).
	appendVerdict(t, store, "age-m", m, "CONFIRMED")
	res = computeFrontier(t, repo, readLedger(t, store))
	if res.SHA != m {
		t.Errorf("frontier = %q, want mainline %q after M resolves", res.SHA, m)
	}

	// Resolving the merge commit advances to the tip: it qualifies precisely
	// because ancestry closure spans ALL parents and S is resolved too.
	appendVerdict(t, store, "age-merge", merge, "CONFIRMED")
	res = computeFrontier(t, repo, readLedger(t, store))
	if res.SHA != merge {
		t.Errorf("frontier = %q, want merge tip %q once every parent line is resolved", res.SHA, merge)
	}
}

// TestFrontier_MergeBlockedByUnresolvedSideBranch pins the other half of the
// mainline rule: candidates are first-parent-only, but resolution COVERAGE
// still spans all parents — a merge commit whose side-branch parent is
// unresolved must not qualify even when the merge and the whole mainline are
// CONFIRMED. The frontier holds at the last resolved first-parent commit
// below the merge.
func TestFrontier_MergeBlockedByUnresolvedSideBranch(t *testing.T) {
	repo, base := initRepo(t)
	gitRunAt(t, repo, "2027-01-01T01:00:00Z", "checkout", "-q", "-b", "side")
	s := commitFileAt(t, repo, "pkg/side.go", "side\n", "feat: side-branch slice, never verified", "2027-01-01T02:00:00Z")
	gitRunAt(t, repo, "2027-01-01T02:30:00Z", "checkout", "-q", "main")
	m := commitFileAt(t, repo, "pkg/mainline.go", "mainline\n", "feat: mainline slice, confirmed", "2027-01-01T01:30:00Z")
	gitRunAt(t, repo, "2027-01-01T03:00:00Z", "merge", "--no-ff", "--no-edit", "side")
	merge := gitRun(t, repo, "rev-parse", "HEAD")

	store := newLedger(t)
	appendVerdict(t, store, "age-base", base, "CONFIRMED")
	appendVerdict(t, store, "age-m", m, "CONFIRMED")
	appendVerdict(t, store, "age-merge", merge, "CONFIRMED")
	// S carries no verdict: the merge's side parent line is unresolved.

	res := computeFrontier(t, repo, readLedger(t, store))
	if res.SHA == merge || res.SHA == s {
		t.Fatalf("frontier = %q — a merge with an UNRESOLVED side-branch parent must not qualify (I3: closure spans all parents)", res.SHA)
	}
	if res.SHA != m {
		t.Errorf("frontier = %q, want %q (the last resolved first-parent commit below the blocked merge)", res.SHA, m)
	}
}

// --- supporting L1s: the duel-hardened validation surface ---

// TestResolutionEdge_EvidenceFloor exercises A6/A7: a resolves edge is valid
// ONLY with the repro-green-at-compensator record, the P0-bead binding, a
// strict-descendant compensator, and per-refuted-sha uniqueness. Each violation
// must hold the frontier (fail-closed).
func TestResolutionEdge_EvidenceFloor(t *testing.T) {
	setup := func(t *testing.T) (repo, base, c1, c2 string, store *provenancegraph.Store, refuting provenancegraph.Edge) {
		repo, base = initRepo(t)
		c1 = commitFile(t, repo, "pkg/feature.go", "broken\n", "feat: c1 broken")
		c2 = commitFile(t, repo, "pkg/feature.go", "fixed\n", "fix: c2 fix-forward")
		store = newLedger(t)
		appendVerdict(t, store, "age-base", base, "CONFIRMED")
		refuting = appendVerdict(t, store, "age-c1", c1, "REFUTED")
		appendVerdict(t, store, "age-c1-fix", c2, "CONFIRMED")
		return
	}

	t.Run("missing repro-green token holds", func(t *testing.T) {
		repo, base, c1, c2, store, refuting := setup(t)
		e := ResolutionEdge("age-c1-fix", c2, c1, refuting.FromID)
		e.EvidenceRef = "resolves verdict=" + refuting.FromID + " p0=age-c1-fix" // repro token stripped
		mustAppend(t, store, e)
		res := computeFrontier(t, repo, readLedger(t, store))
		if res.SHA != base {
			t.Errorf("frontier = %q, want %q: no green repro at the fix sha ⇒ the edge does not count", res.SHA, base)
		}
	})

	t.Run("repro green at the WRONG sha holds", func(t *testing.T) {
		repo, base, c1, c2, store, refuting := setup(t)
		e := ResolutionEdge("age-c1-fix", c2, c1, refuting.FromID)
		e.EvidenceRef = "resolves verdict=" + refuting.FromID + " repro=green@" + c1 + " p0=age-c1-fix"
		mustAppend(t, store, e)
		res := computeFrontier(t, repo, readLedger(t, store))
		if res.SHA != base {
			t.Errorf("frontier = %q, want %q: repro must be green AT THE COMPENSATING SHA", res.SHA, base)
		}
	})

	t.Run("missing P0-bead binding holds", func(t *testing.T) {
		repo, base, c1, c2, store, refuting := setup(t)
		e := ResolutionEdge("age-c1-fix", c2, c1, refuting.FromID)
		e.EvidenceRef = "resolves verdict=" + refuting.FromID + " repro=green@" + c2
		e.BeadID = ""
		mustAppend(t, store, e)
		res := computeFrontier(t, repo, readLedger(t, store))
		if res.SHA != base {
			t.Errorf("frontier = %q, want %q: a resolves edge without its auto-filed P0 fix bead does not count", res.SHA, base)
		}
	})

	t.Run("non-descendant compensator holds", func(t *testing.T) {
		repo, base, c1, c2, store, refuting := setup(t)
		_ = c2
		// base is an ANCESTOR of c1, not a strict descendant.
		e := ResolutionEdge("age-c1-fix", base, c1, refuting.FromID)
		mustAppend(t, store, e)
		res := computeFrontier(t, repo, readLedger(t, store))
		if res.SHA != base {
			t.Errorf("frontier = %q, want %q: the compensator must be a strict descendant of the refuted sha", res.SHA, base)
		}
	})

	t.Run("duplicate live edges violate uniqueness and hold", func(t *testing.T) {
		repo, base, c1, c2, store, refuting := setup(t)
		mustAppend(t, store, ResolutionEdge("age-c1-fix", c2, c1, refuting.FromID))
		second := ResolutionEdge("age-c1-fix-2", c2, c1, refuting.FromID)
		mustAppend(t, store, second)
		res := computeFrontier(t, repo, readLedger(t, store))
		if res.SHA != base {
			t.Errorf("frontier = %q, want %q: one live resolution edge per refuted sha", res.SHA, base)
		}
	})
}

// TestPrecedence_RefutedDominatesConfirmed pins the r3 "uniform precedence"
// polish: an existing REFUTED on c blocks EVERY non-resolution arm — including
// a bare CONFIRMED on the same sha (a revert war is arbitrated by the ledger's
// resolution edges, never by which verdict happened to land last).
func TestPrecedence_RefutedDominatesConfirmed(t *testing.T) {
	repo, base := initRepo(t)
	c1 := commitFile(t, repo, "pkg/contested.go", "v1\n", "feat: contested commit")

	store := newLedger(t)
	appendVerdict(t, store, "age-base", base, "CONFIRMED")
	appendVerdict(t, store, "age-c1", c1, "REFUTED")
	appendVerdict(t, store, "age-c1", c1, "CONFIRMED")

	ev := NewEvaluator(repo, readLedger(t, store))
	r := ev.Resolve(c1)
	if r.State == StateResolved {
		t.Errorf("Resolve(c1) = RESOLVED via %s, want not-resolved: REFUTED can only resolve via a resolution edge (arm 4)", r.Arm)
	}
}

// TestCheckInverse_RejectsNonInverse pins the deterministic checker: a commit
// that is NOT the byte-exact inverse patch of the refuted commit must fail.
func TestCheckInverse_RejectsNonInverse(t *testing.T) {
	repo, _ := initRepo(t)
	c1 := commitFile(t, repo, "pkg/feature.go", "v1\n", "feat: c1")
	notInverse := commitFile(t, repo, "pkg/feature.go", "v2\n", "feat: c2 is a forward change, not a revert")

	if err := CheckInverse(repo, notInverse, c1); err == nil {
		t.Errorf("CheckInverse(non-revert, c1) = nil, want error: only a byte-exact inverse patch verifies")
	}
	// Self-inversion is meaningless.
	if err := CheckInverse(repo, c1, c1); err == nil {
		t.Errorf("CheckInverse(c1, c1) = nil, want error")
	}
}

// TestTrivialWaiver_Lockstep is the pairing contract with
// scripts/lib/trivial-waiver.sh: same four outcomes, same marker discipline,
// same fail-closed diff verification. If the script's semantics change, this
// table is the tripwire that forces the Go port to follow.
func TestTrivialWaiver_Lockstep(t *testing.T) {
	repo, _ := initRepo(t)

	cases := []struct {
		name    string
		rel     string
		content string
		msg     string
		want    WaiverStatus
	}{
		{"trailing subject marker, provenance-only", "docs/provenance/a.jsonl", "{}\n",
			"chore(provenance): bind verdict #trivial", WaiverWaived},
		{"body trailer marker, provenance-only", "docs/provenance/b.jsonl", "{}\n",
			"chore(provenance): bind verdict\n\n#trivial", WaiverWaived},
		{"mid-subject prose mention does not waive", "docs/provenance/c.jsonl", "{}\n",
			"fix(pawl): prevent #trivial from bypassing pawl", WaiverNoMarker},
		{"marker but non-provenance path refused", "pkg/code.go", "package pkg\n",
			"feat: sneaky code change #trivial", WaiverRefused},
		{"no marker at all", "docs/provenance/d.jsonl", "{}\n",
			"chore(provenance): bind verdict", WaiverNoMarker},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sha := commitFile(t, repo, tc.rel, tc.content, tc.msg)
			got, _ := TrivialWaiver(repo, sha)
			if got != tc.want {
				t.Errorf("TrivialWaiver = %v, want %v", got, tc.want)
			}
		})
	}
}
