# Landing pipeline (land.sh) — AMENDMENT PASS run 2 — Phase 1 BEHAVIORS

> **Extension of the frozen B1–B73 contract** at
> `docs/plans/bdd-foundry/behaviors.md` (frozen 2026-06-12). B1–B73 are
> untouched: not renumbered, not regenerated, not weakened. This file defines
> **B74–B90**, the new scenarios that fold the two independent judges' findings
> (crank-readiness 0.60, design-soundness 0.78 vs the 0.85 bar). On Phase-1
> freeze of this run, B74–B90 are APPENDED to the frozen base file; the base
> preamble conventions (sandbox clone, "origin/main unchanged" = bare-remote
> SHA compare, scenario kinds) apply verbatim to every scenario below.
>
> **Field verification backing these scenarios (2026-06-12, this session):**
> - `helpers.bash:123` creates `scripts/gate.d/` with `mkdir -p` and commits via
>   `git add -A` — git cannot track an empty dir, so lane clones lack it and 10
>   tests (B13, B33, B49, B50, B52, B59, B60, B67, B70, B71) crash on redirect
>   ("No such file or directory") instead of failing on assertion.
> - `08-crash-recovery.bats` B57: the `if [ "$phase" = "push" ] ...` conditional
>   has byte-identical branches (`[ "$status" -eq 0 ]` both sides) — dead code.
> - Real repo `.git/hooks/pre-push` IS a beads-managed chain
>   (`BEGIN/END BEADS INTEGRATION v1.0.5` markers + `pre-push.local` cockpit
>   gate dispatch). `--install` must CHAIN, never clobber.
> - Real repo has NO `scripts/regen-manifest.txt`, NO `scripts/count-docs.txt`,
>   and real `.github/workflows/validate.yml` has NO `land-gate-families:`
>   declaration — the fixture invented all three; the real repo never changed.
> - `ag-arpk` (GitHub merge queue on main) is OPEN P1 in br — the only
>   cross-host serializer option on the table.
> - `ag-d3-fixture-guard-yk7rq`'s ACCEPTANCE is a manual prose recipe whose only
>   bats command (B25) proves harness smoke, not the B62 guard behavior.
>
> **Vocabulary additions:** *fixture suite* = the B1–B73 bats suite run in the
> sandbox; *cutover* = changes to the REAL agentops repo (manifests, marker
> blocks, validate.yml declaration, hook install, CLAUDE.md doctrine);
> *beads segment* = the marker-delimited `BEGIN/END BEADS INTEGRATION` block in
> `.git/hooks/pre-push`; *guard segment* = the marker-delimited block land.sh
> `--install` owns; *SUT seam* = the single helper through which the suite
> invokes the system under test.
>
> **Kind key (unchanged):** happy = the designed path; edge = unusual-but-legal
> inputs/timing; error = the tool/process must refuse or abort cleanly.

---

## Feature: amendment pass — harness repair, bead repair, and real-repo cutover

```gherkin
Feature: land.sh redesign run 2 — red-on-assertion harness, runnable bead
  acceptance, and a cutover lane that makes the REAL repo land through land.sh
  As the operator of a hot multi-lane repo
  I want the acceptance suite to fail only on assertions, every bead to carry a
  runnable done-criterion, and the real repo (not just the fixture) migrated
  So that the 16 existing beads are actually crankable and the redesign changes
  reality instead of admiring the fixture
```

---

## §A Harness repair (B74–B76) — amendment 1 + minor 5c

```gherkin
Scenario: B74 — fixture gate.d directory survives every lane clone (happy)
  Given seed_fixture has built the fixture repo and pushed it to the bare remote
  When a lane clone is created via new_lane
  Then "git -C $SEED ls-files scripts/gate.d/" lists at least one tracked entry
      (a .gitkeep or equivalent — the directory is tracked, not incidental)
    And the directory "scripts/gate.d" exists in the fresh lane clone
    And writing "$lane/scripts/gate.d/99-probe.sh" via shell redirect succeeds
      (exit 0, file present) — the redirect-crash class is structurally gone
    And run-acceptance.sh (or helpers.bash) carries a harness self-check that
      fails the run with "fixture defect: scripts/gate.d untracked" if the
      tracked entry ever disappears
```

```gherkin
Scenario: B75 — the full suite is red ON ASSERTION: 73 not-ok, zero harness crashes (happy)
  Given scripts/land.sh is the Phase-2 red placeholder (exits 97)
  When "tests/landing/run-acceptance.sh" (or the suite's bats entry point) runs
      over docs/plans/bdd-foundry/acceptance-tests/
  Then the TAP summary reports exactly 73 "not ok" and 0 "ok"
    And the combined output contains ZERO occurrences of
      "No such file or directory" attributable to harness setup
      (grep -c over the captured run output == 0 for that pattern in
       setup/seed/new_lane stack traces)
    And for EACH of the ten previously-poisoned tests
      (B13, B33, B49, B50, B52, B59, B60, B67, B70, B71) the bats failure
      trace points at an assertion line of the test body (a "run land" /
      "[ \"$status\" ... ]" / "[[ \"$output\" ... ]]" line), never at a
      setup() or fixture line
    And this red-on-assertion audit is itself a checked-in script
      (e.g. acceptance-tests/audit-red.sh) that exits 0 on the above and
      nonzero naming every test whose failure is a crash, so any future
      harness regression is mechanically caught
```

```gherkin
Scenario: B76 — B57's dead conditional is repaired: post-push phases assert "already landed" distinctly (edge)
  Given the B57 crash-point matrix in 08-crash-recovery.bats
  When the SAME lane reruns after a crash at phases "push" and "pre-release"
  Then the rerun exits 0 AND its output matches "already landed"
    AND the bare remote gains ZERO new patch-ids from the rerun
      (remote_patch_ids count identical before/after the rerun)
  When the SAME lane reruns after a crash at the pre-push phases
      (rebase, regen-write, regen-commit, gate)
  Then the rerun exits 0 AND the lane's patch appears on main EXACTLY once
      (no "already landed" required — a real land occurred)
    And structurally: 08-crash-recovery.bats contains no if/else whose two
      branches are byte-identical (asserted by the B75 audit script or an
      equivalent lint over the suite)
```

---

## §B Bead repair (B77) — amendment 2

```gherkin
Scenario: B77 — ag-d3-fixture-guard-yk7rq carries a runnable B62 acceptance, not a prose recipe (happy)
  Given the amended bead in br (read from the MAIN checkout:
      BEADS_DIR=/Users/bo/dev/agentops/_beads br show ag-d3-fixture-guard-yk7rq)
  Then its ACCEPTANCE section contains an executable bats filter command of the
      form "bats docs/plans/bdd-foundry/acceptance-tests -f '^B62'" (plus any
      required env), and that exact command is copy-paste runnable from the
      repo root
    And the ACCEPTANCE contains no manual prose steps as the done-criterion
      (no "manually", "by hand", "inspect", "eyeball" as the operative verb)
    And running the stated filter TODAY (red placeholder) yields not-ok ON
      ASSERTION (per B75's standard), and the filtered test exercises the
      fixture-guard behavior itself: a push to the fixture bare remote WITHOUT
      a valid LAND_PUSH_NONCE is rejected by the pre-receive hook and a push
      WITH it is accepted — not merely that seed_fixture completes (the old
      B25-smoke proxy is gone)
```

---

## §C Cutover: real-repo migration (B78–B81) — amendment 3a

> These scenarios run against the REAL agentops checkout, not the sandbox
> fixture. They are still hermetic to verify: every assertion is a read or a
> clean-tree command (`--check` modes); the migration itself is the bead's job.

```gherkin
Scenario: B78 — the real repo's regen write set is declared in scripts/regen-manifest.txt and matches reality (happy)
  Given the cutover commit on the real repo
  Then scripts/regen-manifest.txt exists and is non-empty
    And on a clean tree, running "scripts/regen-all.sh" then
      "git status --porcelain" yields ONLY paths matched by the manifest
      (every written path is declared)
    And every manifest-declared path is actually written or verified by
      regen-all.sh (a parity script exits 0; deleting any one line from a
      manifest copy makes the parity script exit nonzero naming the path)
    And no manifest path is also source-owned (the B42 overlap rule holds on
      the real manifest: the parity script fails on a path that is both)
```

```gherkin
Scenario: B79 — the real repo's count-bearing docs are declared in scripts/count-docs.txt (happy)
  Given the cutover commit on the real repo
  Then scripts/count-docs.txt exists and lists every prose doc carrying a
      generated skill count (the ~11-doc set identified in the evidence base)
    And the count checker run against the real manifest exits 0 on a clean tree
    And planting a bare numeric skill-count OUTSIDE marker blocks in any doc
      NOT listed in the manifest makes the repo-wide sweep exit nonzero naming
      that doc and line (the B48 sweep generalizes to the real repo)
```

```gherkin
Scenario: B80 — the ~11 prose docs carry generator-owned marker blocks; hand-asserted counts are extinct (happy)
  Given the cutover commit on the real repo
  Then in every doc listed in scripts/count-docs.txt, every skill-count
      occurrence sits inside marker blocks
      ("<!-- count:skills -->N<!-- /count -->" or the pinned equivalent)
    And the repo-wide sweep for numeric skill-count literals OUTSIDE marker
      blocks across the manifested docs returns 0 matches
    And editing one marker block's value to a wrong number, then running the
      counts generator, restores the generated value (byte-level check)
    And the conversion commit itself passes "scripts/regen-all.sh --check"
      and the repo's standard CI gate (the migration introduces zero drift)
```

```gherkin
Scenario: B81 — real validate.yml declares land-gate-families and the parity check holds (happy)
  Given the cutover commit on the real repo
  Then .github/workflows/validate.yml contains exactly one
      "land-gate-families:" declaration line listing the gate families land.sh
      must run
    And the B49 parity check, pointed at the REAL validate.yml, exits 0
      (land.sh's family list ⊇ the declared families)
    And removing one family token from a working copy of validate.yml makes
      the parity check exit nonzero naming the missing family
    And (bead-level constraint, recorded here so it cannot be dropped): the
      bead that lands this declaration MUST coordinate with or sequence after
      the active lane currently holding validate.yml — asserted in the bead
      body per B90's self-containment rule, not by this test
```

---

## §D Cutover: hook-chain-aware --install + doctrine flip (B82–B86) — amendment 3b

```gherkin
Scenario: B82 — --install CHAINS onto a beads-managed pre-push hook, never clobbers (happy)
  Given a sandbox clone whose .git/hooks/pre-push reproduces the real repo's
      shape: a "BEGIN/END BEADS INTEGRATION v1.0.5" marker block plus a
      pre-push.local dispatch (the cockpit gate), each instrumented to append
      its name to a probe log when executed
  When "scripts/land.sh --install" runs and a direct "git push origin main"
      is then attempted
  Then the probe log shows ALL THREE segments executed: the beads segment, the
      pre-push.local cockpit gate, and the land guard — and the push is
      rejected by the guard matching "use scripts/land.sh" (B17 semantics
      preserved through the chain)
    And the bytes between the BEADS INTEGRATION markers are identical
      before/after install (sha256 of the extracted segment)
    And pre-push.local's file is bit-identical before/after install
    And when land.sh itself pushes (live lock + nonce per B63), the full chain
      still runs and permits the push
```

```gherkin
Scenario: B83 — --install is idempotent and upgrades ONLY its own guard segment (edge)
  Given a clone where --install has already completed on the chained hook
  When "scripts/land.sh --install" reruns
  Then it exits 0 matching "already installed" and .git/hooks/pre-push is
      byte-identical (sha256 before == after)
  Given the guard segment is rewritten in place to carry an OLDER version
      string between its own markers
  When "scripts/land.sh --install" reruns
  Then ONLY the guard segment's bytes change (beads segment and
      pre-push.local extractions sha256-identical), the new guard version is
      logged as an upgrade ("guard upgraded <old> -> <new>"), and exit is 0
```

```gherkin
Scenario: B84 — --install on hookless and foreign-hook clones is explicit, never a silent overwrite (error)
  Given a clone with NO .git/hooks/pre-push at all
  When "scripts/land.sh --install" runs
  Then a guard hook is installed, and a direct push to main is rejected per
      B17 while a land.sh push succeeds per B63
  Given a clone whose pre-push is FOREIGN: executable content with no beads
      markers and no guard markers
  When "scripts/land.sh --install" runs
  Then ONE documented behavior occurs (printed in --help and asserted):
      EITHER the foreign hook is preserved and chained (a subsequent push
      executes the foreign content — probe log proves it — and then the guard)
      OR install refuses with exit nonzero matching
      "existing pre-push not recognized — chain manually"
    And in NO variant is the foreign content silently deleted or overwritten:
      its bytes remain in the hook chain or untouched on refusal
```

```gherkin
Scenario: B85 — guard rollout is mechanically verifiable per clone; the cross-host residual is stated, not hidden (happy)
  Given any clone
  When "scripts/land.sh --install --verify" runs (or --status --json includes
      an "install" object)
  Then in a clone where install completed it exits 0 and reports
      machine-readably: guard present, guard version, chain intact
      (beads segment + pre-push.local + guard all detected where applicable)
    And in a naked clone it exits nonzero naming exactly what is missing
    And the cutover's rollout criterion is: this verify PASSES on BOTH live
      clones (the Mac checkout and the bushido checkout), evidence captured
      per clone
    And the spec and --help both carry the pinned residual statement: the
      landing lock is HOST-LOCAL — LAND_LOCK_DIR cannot serialize a Mac land
      against a bushido land; cross-host serialization is explicitly out of
      scope for land.sh v1 and is owned by the ag-arpk disposition (B87)
```

```gherkin
Scenario: B86 — CLAUDE.md doctrine flips from bare-push to land.sh (happy)
  Given the cutover commit on the real repo
  Then CLAUDE.md's Workflow "Land" phase instructs "scripts/land.sh" as the
      landing command (grep finds the literal "scripts/land.sh" in the Phases
      → Land step and in the Branch+PR-shape Land row)
    And no surviving instruction in the Workflow/Land/Multi-agent sections
      tells an agent to push directly to main as the landing path
      (grep for the old operative phrases — "Push to `main`" as an
      instruction, "rebase-on-reject (git serializes" — returns 0 matches
      outside explicitly historical/superseded notes)
    And the pre-push hook description names the chained guard (beads segment +
      cockpit gate + land guard), matching B82's installed reality
```

---

## §E Cutover: ag-arpk disposition (B87) — amendment 3c

```gherkin
Scenario: B87 — ag-arpk is dispositioned with an explicit chosen path and named residual (happy)
  Given the amendment pass has run
  When "BEADS_DIR=/Users/bo/dev/agentops/_beads br show ag-arpk" runs from the
      main checkout
  Then the bead is no longer an untouched open P1: it is EITHER
      deferred-with-reason (status/label/body records the deferral and the
      re-evaluation trigger) OR superseded (body names the superseding land.sh
      epic/bead id)
    And the disposition text names the exact residual it leaves: cross-host
      (Mac ↔ bushido) landing serialization is NOT provided by land.sh's
      host-local lock, and GitHub merge queue remains the only listed
      serializer option for that gap
    And the disposition states which residual-handling choice was made
      (accept the residual with land.sh-only, or keep merge-queue as the
      planned cross-host serializer) — a reader can tell what is and is not
      protected after cutover
```

---

## §F Substrate seam, lock-dir pin, bead hygiene (B88–B90) — amendments 4 + 5

```gherkin
Scenario: B88 — the acceptance contract is implementation-agnostic through ONE invocation seam (edge)
  Given the repaired suite
  Then every SUT invocation in the suite routes through a single helper that
      honors a LAND_BIN override (default: the lane's "scripts/land.sh") —
      "grep -rn 'scripts/land.sh' acceptance-tests/*.bats" finds no direct
      invocation outside helpers.bash and fixture-authoring heredocs
    And exporting LAND_BIN to a probe stub before one test flips that test's
      observed SUT output to the stub's (proving the seam is load-bearing)
    And spec.md carries the one-paragraph implementation-choice note: the
      suite is the contract; an "ao land" Go implementation is PERMITTED AND
      PREFERRED over hardening ~2k lines of concurrency-critical Bash 3.2 if
      the implementer judges it cheaper; either substrate must pass the
      identical suite via the LAND_BIN seam
```

```gherkin
Scenario: B89 — LAND_LOCK_DIR's production default is pinned, deterministic, and origin-keyed (happy)
  Given two clones/worktrees of the SAME origin URL on one host, with no
      LAND_LOCK_DIR env and no --lock-dir flag
  When each runs "scripts/land.sh --status --json"
  Then both report the IDENTICAL resolved "lock_dir" string, equal to the
      documented default: a stable state root
      (${XDG_STATE_HOME:-$HOME/.local/state}/land/<digest-of-origin-URL> or
      the exact pinned formula printed in --help)
    And a clone of a DIFFERENT origin URL resolves a DIFFERENT lock_dir
    And the resolution is pure (no lock files created by --status, per B34)
    And mutual exclusion actually flows through the default: with no env
      overrides, two concurrent lands from the two same-origin clones
      serialize per B6 (their hold intervals do not overlap)
```

```gherkin
Scenario: B90 — every bead's regression criteria are self-contained runnable commands (edge)
  Given all landing-redesign beads in br (the existing 16 plus this run's new
      ones), read via "BEADS_DIR=/Users/bo/dev/agentops/_beads br show <id>"
      from the main checkout
  Then every ACCEPTANCE and regression criterion contains the full runnable
      command (bats path + filter + required env), copy-paste executable from
      the repo root
    And a sweep over the shown bodies finds ZERO shorthand-only criteria —
      no "spine filter stays green", "previous filters still pass", or
      equivalent phrase WITHOUT an adjacent explicit command
    And the sweep itself is a checked-in script or a recorded command in the
      run-2 manifest so the check is repeatable at validate time
```

---

## Amendment → scenario disposition table

| Judge amendment | Scenario(s) | Notes |
|---|---|---|
| 1. Harness bug: empty `scripts/gate.d/` untracked → 10 tests crash on redirect | B74, B75 | B75's audit script makes red-on-assertion a standing gate, not a one-time fix |
| 2. Defective bead: `ag-d3-fixture-guard-yk7rq` prose ACCEPTANCE | B77 | Amend in br; runnable `^B62` filter; old B25-smoke proxy removed |
| 3a. Real-repo migration (manifests, marker blocks, validate.yml declaration) | B78, B79, B80, B81 | validate.yml lane-coordination pinned as a bead constraint inside B81 |
| 3b. Hook-chain-aware `--install` + both-clone rollout + CLAUDE.md flip | B82, B83, B84, B85, B86 | Chain-not-clobber proven by probe logs + segment sha256s; cross-host residual pinned in B85 |
| 3c. `ag-arpk` disposition | B87 | Defer-with-reason or supersede; residual named explicitly |
| 4. Substrate note (Bash 3.2 vs `ao land` Go) | B88 | Made mechanical: LAND_BIN seam + spec paragraph |
| 5a. Pin LAND_LOCK_DIR production default | B89 | Origin-keyed deterministic default |
| 5b. Normalize shorthand regression criteria | B90 | Sweep over br bodies |
| 5c. B57 dead conditional in 08-crash-recovery.bats | B76 | Branches now differ materially; "already landed" asserted for post-push phases |

## Coverage map additions (failure-mode → scenario)

| Failure mode / risk class | Scenario(s) |
|---|---|
| Harness crash poisoning done-criteria | B74, B75, B76 |
| Bead acceptance not runnable / shorthand | B77, B90 |
| Fixture-only acceptance: the real repo never changes | B78–B86 |
| Hook clobbering a live beads chain on the real clones | B82, B83, B84 |
| Cross-host (Mac↔bushido) serialization gap hidden | B85, B87 |
| Substrate lock-in to concurrency-critical Bash 3.2 | B88 |
| Lock-dir default drift across clones | B89 |

## Out of scope for this amendment pass

- Re-opening or weakening any of B1–B73 (frozen; this pass only appends).
- Implementing cross-host landing serialization (B85/B87 pin the residual and
  its owner; building it is a future epic).
- GitHub merge-queue enablement itself (B87 dispositions the bead; it does not
  build the queue).
- Re-running the 100-gap codex review (run-1 disposition table stands).
