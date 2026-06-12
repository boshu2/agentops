# Landing pipeline (land.sh) — AMENDMENT PASS run 2 — Phase 1 BEHAVIORS — **FROZEN**

> **FROZEN 2026-06-12 (definition of done for this run).** Extension of the
> frozen B1–B73 contract at `docs/plans/bdd-foundry/behaviors.md` (frozen
> 2026-06-12). B1–B73 are untouched: not renumbered, not regenerated, not
> weakened. This file defines **B74–B94**: B74–B90 fold the two independent
> judges' findings (crank-readiness 0.60, design-soundness 0.78 vs the 0.85
> bar); B91–B93 plus in-place amendments to B75, B78, B79, B81, B84–B90 fold
> the cross-family codex gap review (`behaviors-codex-gaps.md`, all 15 gaps
> dispositioned — see the Gap disposition table at the bottom); **B94** is the
> drift-guard repair split of B85's second concern (rollout evidence) so that
> bead 'done' stays 1:1 with a runnable test — no other scenario was
> renumbered or weakened. On Phase-2
> entry, B74–B94 are APPENDED to the frozen base file; the base preamble
> conventions (sandbox clone, "origin/main unchanged" = bare-remote SHA
> compare, scenario kinds) apply verbatim to every scenario below.
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
> invokes the system under test; *coverage manifest* = the checked-in map from
> appended behavior id → its verifying test or script (B91); *rollout
> evidence manifest* = the checked-in per-clone verify record (B94, split
> from B85 at drift-guard repair).
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
Scenario: B75 — the full suite is red ON ASSERTION: zero ok, zero harness crashes, appended behaviors included (happy)
  Given scripts/land.sh is the Phase-2 red placeholder (exits 97)
  When "tests/landing/run-acceptance.sh" (or the suite's bats entry point) runs
      over docs/plans/bdd-foundry/acceptance-tests/
  Then the TAP summary reports ZERO "ok" and a "not ok" count equal to the
      suite's enumerated test count, which is AT LEAST 73 + one test per
      sandbox-testable appended behavior (per the B91 coverage manifest) —
      the count is asserted against the manifest, not hardcoded to 73
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
> fixture. They are still hermetic to verify: every mutating check runs under
> the B92 hermetic-verification contract (disposable worktree or exact
> restore + residue check); the migration itself is the bead's job.

```gherkin
Scenario: B78 — the real repo's regen write set is declared in scripts/regen-manifest.txt, strictly formatted, and matches reality (happy)
  Given the cutover commit on the real repo
  Then scripts/regen-manifest.txt exists and is non-empty
    And the manifest format is STRICT: one normalized repo-relative path per
      line (no "../" or "./" segments, no absolute paths), no glob broader
      than a single directory's generated files, comment lines only with a
      leading "#"; the parity script REJECTS (exit nonzero, naming the line)
      duplicate paths, non-normalized entries, and any glob entry that also
      matches a source-owned path
    And on a clean tree, running "scripts/regen-all.sh" then
      "git status --porcelain" yields ONLY paths matched by the manifest
      (every written path is declared) — the written set is derived from the
      ACTUAL regen run's status output, never from re-reading the manifest
      under test
    And every manifest-declared path is actually written or verified by
      regen-all.sh (overdeclared/stale entries are independently detected:
      deleting any one line from a manifest copy makes the parity script exit
      nonzero naming the path, and adding a bogus path to a manifest copy
      ALSO makes it exit nonzero naming the path)
    And no manifest path is also source-owned (the B42 overlap rule holds on
      the real manifest: the parity script fails on a path that is both)
```

```gherkin
Scenario: B79 — the real repo's count-bearing docs are declared in scripts/count-docs.txt (happy)
  Given the cutover commit on the real repo
  Then scripts/count-docs.txt exists and lists every prose doc carrying a
      generated skill count (the ~11-doc set identified in the evidence base)
    And count-docs.txt obeys the same strict format rules as B78's manifest
      (normalized repo-relative paths, no duplicates, rejected with the line
      named on violation)
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
Scenario: B81 — real validate.yml declares land-gate-families STRUCTURALLY and the parity check holds (happy)
  Given the cutover commit on the real repo
  Then .github/workflows/validate.yml contains exactly one
      "land-gate-families" declaration, verified by a STRUCTURAL YAML parse
      (yq / python-yaml, not grep): it is a real key attached to the intended
      job/env block, not a comment, not a string inside another value
    And the parser REJECTS (exit nonzero, naming the defect): a commented-out
      declaration, duplicate declarations, and an empty family list
    And the B49 parity check, pointed at the REAL validate.yml via the same
      structural parse, exits 0 (land.sh's family list ⊇ the declared
      families) and each declared family maps to an actual CI job/step in
      validate.yml (no family token that no job implements)
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
Scenario: B82 — --install CHAINS onto a beads-managed pre-push hook, never clobbers, in documented order (happy)
  Given a sandbox clone whose .git/hooks/pre-push reproduces the real repo's
      shape: a "BEGIN/END BEADS INTEGRATION v1.0.5" marker block plus a
      pre-push.local dispatch (the cockpit gate), each instrumented to append
      its name to a probe log when executed
  When "scripts/land.sh --install" runs and a direct "git push origin main"
      is then attempted
  Then the probe log shows ALL THREE segments executed IN THE DOCUMENTED ORDER
      (the order printed by --help): the beads segment, the pre-push.local
      cockpit gate, and the land guard — and the push is rejected by the
      guard matching "use scripts/land.sh" (B17 semantics preserved through
      the chain)
    And the bytes between the BEADS INTEGRATION markers are identical
      before/after install (sha256 of the extracted segment)
    And pre-push.local's file is bit-identical before/after install
    And the installed hook file remains executable (test -x) after install
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
Scenario: B84 — --install policy on hookless and foreign-hook clones is PINNED: install on hookless, refuse on unrecognized (error)
  Given a clone with NO .git/hooks/pre-push at all
  When "scripts/land.sh --install" runs
  Then a guard hook is installed, and a direct push to main is rejected per
      B17 while a land.sh push succeeds per B63
  Given a clone whose pre-push is FOREIGN: executable content with no beads
      markers and no guard markers
  When "scripts/land.sh --install" runs
  Then the PINNED policy executes — install REFUSES with exit nonzero matching
      "existing pre-push not recognized — chain manually" — and this policy
      is printed in --help and stated in spec.md BEFORE Phase-3
      implementation begins (the policy is decided at freeze, not late-bound
      by the implementer)
    And the test ALSO proves the opposite behavior does not occur: after the
      refusal the foreign hook's bytes are sha256-identical and no guard
      segment text appears anywhere in .git/hooks/
    And the refusal holds across the control-flow trap variants — a foreign
      hook that begins with "exit 0", one that ends in "exec", one with no
      shebang, and one that is present but NOT executable — each refuses with
      the same message and zero byte changes (no variant is silently chained,
      overwritten, or "fixed")
    And recognized-chain installs (B82's shape) are unaffected by this policy:
      a beads-managed chain with pre-push.local still chains per B82
```

```gherkin
Scenario: B85 — guard install is mechanically verifiable per clone with strict verify; the cross-host residual is stated, not hidden (happy)
  Given any clone
  When "scripts/land.sh --install --verify" runs (or --status --json includes
      an "install" object)
  Then in a clone where install completed it exits 0 and reports
      machine-readably: guard present, guard version, chain intact
      (beads segment + pre-push.local + guard all detected where applicable)
      — the JSON keys ("guard_present", "guard_version", "chain", "defects"
      or the exact pinned set) and the exit codes are documented in --help
      and stable across runs
    And verify exits NONZERO with a distinct named defect for EACH of: a
      stale guard version (older than the installing land.sh's), duplicate
      guard segments, a malformed/unpaired marker, a hook file missing the
      executable bit, and a chain-order defect (guard not in the documented
      position) — one fault injected per case, each named in the JSON
      "defects" output
    And in a naked clone it exits nonzero naming exactly what is missing
    And the rollout check script (scripts/check-rollout-evidence.sh) SHIPS
      with this behavior — executable, validating the rollout evidence
      manifest shape and rejecting stale records — with NO dependence on the
      checked-in evidence records themselves (the durable two-clone evidence
      is B94, the split second concern of this scenario)
    And the spec and --help both carry the pinned residual statement: the
      landing lock is HOST-LOCAL — LAND_LOCK_DIR cannot serialize a Mac land
      against a bushido land; cross-host serialization is explicitly out of
      scope for land.sh v1 and is owned by the ag-arpk disposition (B87)
```

```gherkin
Scenario: B86 — landing doctrine flips repo-wide, not just CLAUDE.md (happy)
  Given the cutover commit on the real repo
  Then CLAUDE.md's Workflow "Land" phase instructs "scripts/land.sh" as the
      landing command (grep finds the literal "scripts/land.sh" in the Phases
      → Land step and in the Branch+PR-shape Land row)
    And a checked-in operator-doc sweep script runs over a pinned doc list —
      at minimum CLAUDE.md, AGENTS.md, AGENTS-WORKFLOW.md, AGENTS-CI.md,
      AGENTS-CODEX.md, AGENTS-RUNTIME.md, README.md, and
      docs/agent-workflow-reference.md — and exits nonzero naming any doc
      whose LIVE instructions still tell an agent to push directly to main
      as the landing path (grep for the old operative phrases — "Push to
      `main`" as an instruction, "rebase-on-reject (git serializes" —
      returns 0 matches outside explicitly historical/superseded notes)
    And direct-push language survives ONLY inside sections explicitly marked
      historical/superseded (the sweep recognizes the marker convention and
      the convention is documented in the sweep script's header)
    And every live landing instruction across the swept docs names
      "scripts/land.sh"
    And the pre-push hook description in the swept docs names the chained
      guard (beads segment + cockpit gate + land guard), matching B82's
      installed reality
```

---

## §E Cutover: ag-arpk disposition (B87) — amendment 3c

```gherkin
Scenario: B87 — ag-arpk is dispositioned with an explicit chosen path, named residual, and machine-readable tracker state (happy)
  Given the amendment pass has run
  When "BEADS_DIR=/Users/bo/dev/agentops/_beads br show ag-arpk" runs from the
      main checkout
  Then the bead is no longer an untouched open P1: it is EITHER
      deferred-with-reason (status/label/body records the deferral and the
      re-evaluation trigger) OR superseded (body names the superseding land.sh
      epic/bead id)
    And the machine-readable state AGREES with the prose: status, priority/
      labels, and dependency edges are set such that
      "BEADS_DIR=/Users/bo/dev/agentops/_beads br ready" and bv triage
      (run from the main checkout) no longer surface ag-arpk as unclaimed
      active work — UNLESS the chosen path is "keep merge-queue planned",
      in which case it carries the dependency edge / label that sequences it
      after the land.sh epic (a graph triage shows it blocked, not ready)
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
Scenario: B88 — the acceptance contract is implementation-agnostic through ONE invocation seam, including installed hooks (edge)
  Given the repaired suite
  Then every SUT invocation in the suite routes through a single helper that
      honors a LAND_BIN override (default: the lane's "scripts/land.sh") —
      "grep -rn 'scripts/land.sh' acceptance-tests/*.bats" finds no direct
      invocation outside helpers.bash and fixture-authoring heredocs
    And exporting LAND_BIN to a probe stub before one test flips that test's
      observed SUT output to the stub's (proving the seam is load-bearing)
    And the seam extends to INSTALLED artifacts: any land-command dispatch
      that --install writes into the guard segment goes through the same
      configured land command or a documented "scripts/land.sh"
      compatibility shim — with LAND_BIN set to a probe stub during install
      and a subsequent push, the probe (not a hardcoded path) is what the
      installed chain consults, so an "ao land" substrate cannot pass the
      suite while installed clones call the wrong binary
    And spec.md carries the one-paragraph implementation-choice note: the
      suite is the contract; an "ao land" Go implementation is PERMITTED AND
      PREFERRED over hardening ~2k lines of concurrency-critical Bash 3.2 if
      the implementer judges it cheaper; either substrate must pass the
      identical suite via the LAND_BIN seam
```

```gherkin
Scenario: B89 — LAND_LOCK_DIR's production default is pinned, deterministic, and origin-IDENTITY-keyed (happy)
  Given two clones/worktrees of the SAME origin URL on one host, with no
      LAND_LOCK_DIR env and no --lock-dir flag
  When each runs "scripts/land.sh --status --json"
  Then both report the IDENTICAL resolved "lock_dir" string, equal to the
      documented default: a stable state root
      (${XDG_STATE_HOME:-$HOME/.local/state}/land/<digest-of-canonical-origin>
      or the exact pinned formula printed in --help)
    And the digest is computed over a CANONICALIZED origin identity, and the
      canonicalization is pinned in --help: equivalent forms of the same
      remote — "git@github.com:org/repo.git", "ssh://git@github.com/org/repo",
      "https://github.com/org/repo.git", with or without trailing ".git" —
      all resolve to the SAME lock_dir (asserted pairwise on clones whose
      origin strings differ but identities match)
    And a clone of a DIFFERENT origin identity (different host or repo path)
      resolves a DIFFERENT lock_dir
    And the resolution is pure (no lock files created by --status, per B34)
    And mutual exclusion actually flows through the default: with no env
      overrides, two concurrent lands from two same-identity clones
      (including one pair with differing origin URL spellings) serialize per
      B6 (their hold intervals do not overlap)
```

```gherkin
Scenario: B90 — every bead's regression criteria are self-contained runnable commands that EXECUTE and select tests; bead reads fail closed (edge)
  Given all landing-redesign beads in br (the existing 16 plus this run's new
      ones), read via "BEADS_DIR=/Users/bo/dev/agentops/_beads br show <id>"
      from the main checkout
  Then every ACCEPTANCE and regression criterion contains the full runnable
      command (bats path + filter + required env), copy-paste executable from
      the repo root
    And a sweep over the shown bodies finds ZERO shorthand-only criteria —
      no "spine filter stays green", "previous filters still pass", or
      equivalent phrase WITHOUT an adjacent explicit command
    And the sweep EXECUTES every extracted command from a clean repo root with
      ONLY the env the bead declares, and exits nonzero naming the bead and
      command for: a filter that selects ZERO tests, a stale/missing path, or
      a run that dies for harness/setup reasons — the only passing outcomes
      are red-on-assertion (pre-implementation) or green (post-implementation)
    And the sweep FAILS CLOSED on its own dependencies: if br is absent, the
      _beads ledger path is missing, or any bead id is not found, it exits
      nonzero naming the missing tool/path/bead id — it never silently skips
      a bead check or substitutes cached prose for a live "br show"
    And the sweep itself is a checked-in script or a recorded command in the
      run-2 manifest so the check is repeatable at validate time
```

---

## §G Meta-gates from the cross-family review (B91–B93) — codex gaps 1, 2, 4

```gherkin
Scenario: B91 — every appended behavior is mapped to a mechanical verifier in a checked-in coverage manifest (happy)
  Given the Phase-2 acceptance work for this run
  Then a checked-in coverage manifest (e.g.
      docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/
      coverage-manifest.txt) maps EVERY behavior id B74–B94 to exactly one
      verifier: either a bats test selected by the suite entry point
      (sandbox-testable behaviors) or a named checked-in script/command (the
      real-repo, bead, and doc behaviors: e.g. audit-red.sh, the cutover
      verifiers, the B90 sweep, the B86 doc sweep)
    And a manifest checker script exits nonzero naming any appended behavior
      id with no mapped verifier, any mapped bats test the entry point does
      not select, and any mapped script that does not exist or is not
      executable
    And under the red placeholder, every mapped bats test fails
      red-on-assertion per B75's standard (no appended behavior may be
      satisfied by prose alone or by a test that never runs)
```

```gherkin
Scenario: B92 — real-repo verification is hermetic: no check dirties or damages the operator checkout (error)
  Given any verifier for the real-repo scenarios (B78–B81, B85, B86) that
      performs a mutating step — running regen-all.sh, editing a marker
      block's value, deleting a manifest line, or removing a validate.yml
      family token
  When the verifier runs against the real repo
  Then every mutating step executes EITHER in a disposable worktree/copy that
      is removed afterward OR against a scratch copy of the file, never the
      operator checkout's working tree in place
    And after the verifier completes (pass or fail), the operator checkout's
      "git status --porcelain" output and HEAD SHA are byte-identical to the
      pre-run capture — the verifier itself records both before and after and
      exits nonzero with "verifier residue" naming every leftover path or
      SHA mismatch if they differ
    And a verifier that crashes mid-run still leaves the operator checkout
      clean (the mutation target was never the checkout itself, so no
      restore step can be forgotten)
```

```gherkin
Scenario: B93 — --install is crash-safe: atomic write, backup, byte-identical hook on failure (error)
  Given a clone with the B82 chained hook shape
  When --install's hook write is interrupted or fails (each injected
      separately: kill mid-write, write to a full/read-only target, chmod
      failure)
  Then the surviving .git/hooks/pre-push is byte-identical to the pre-install
      hook (sha256 match) — install writes via temp-file + atomic rename in
      the same directory, never in-place edits
    And a backup of the prior hook exists at a documented path before any
      rename, and --help documents both the backup path and the recovery step
    And on any failure --install exits nonzero with a structured error naming
      the failed step (write/rename/chmod), never exit 0 with a half-written
      hook
    And after a SUCCESSFUL install the hook is executable and the backup of
      the prior version is retained (B83's idempotent rerun does not delete
      it)
```

---

## §H Drift-guard split (B94) — B85's second concern, bead-scoped

> **B94 is the split of B85's rollout-evidence concern**, carved out at the
> 2026-06-12 drift-guard repair so each bead's 'done' is exactly one runnable
> test: B85 keeps strict `--install --verify` (bead `run2-install-verify`);
> B94 owns the durable, checked-in two-clone rollout evidence (bead
> `run2-rollout-evidence`). The former compound acceptance's
> `scripts/check-rollout-evidence.sh` half is folded INTO the B94 bats test.
> Nothing else was renumbered.

```gherkin
Scenario: B94 — guard rollout evidence is durable, checked in, and staleness-rejected on the real repo (happy)
  Given the C5–C9 cutover is on the real repo's main and BOTH live clones
      (the Mac main checkout and the bushido checkout) are pulled
  When "scripts/land.sh --install" then "scripts/land.sh --install --verify"
      have been run on both clones
  Then the cutover's rollout criterion holds: verify PASSES on both clones,
      and the evidence is a CHECKED-IN rollout evidence manifest
      (<run-2 plan dir>/rollout-evidence.jsonl — one record per clone: clone
      identity/host, repo SHA at verify time, guard version, the exact
      command run, an ISO-8601 timestamp, and the raw verify JSON)
    And scripts/check-rollout-evidence.sh run from the real repo root exits 0
      against the checked-in manifest
    And the rollout check script exits nonzero if any record's guard version
      or repo SHA no longer matches the current cutover commit (stale
      evidence is rejected, not grandfathered) — proven in-test against a
      staleness-mutated copy of the manifest, so the rejection path is
      exercised by the same single bats test that is the owning bead's
      acceptance (one bats filter, no chained shell command)
```

---

## Amendment → scenario disposition table

| Judge amendment | Scenario(s) | Notes |
|---|---|---|
| 1. Harness bug: empty `scripts/gate.d/` untracked → 10 tests crash on redirect | B74, B75 | B75's audit script makes red-on-assertion a standing gate, not a one-time fix |
| 2. Defective bead: `ag-d3-fixture-guard-yk7rq` prose ACCEPTANCE | B77 | Amend in br; runnable `^B62` filter; old B25-smoke proxy removed |
| 3a. Real-repo migration (manifests, marker blocks, validate.yml declaration) | B78, B79, B80, B81 | validate.yml lane-coordination pinned as a bead constraint inside B81 |
| 3b. Hook-chain-aware `--install` + both-clone rollout + CLAUDE.md flip | B82, B83, B84, B85, B86, B94 | Chain-not-clobber proven by probe logs + segment sha256s; cross-host residual pinned in B85; rollout evidence split to B94 |
| 3c. `ag-arpk` disposition | B87 | Defer-with-reason or supersede; residual named explicitly |
| 4. Substrate note (Bash 3.2 vs `ao land` Go) | B88 | Made mechanical: LAND_BIN seam + spec paragraph |
| 5a. Pin LAND_LOCK_DIR production default | B89 | Origin-identity-keyed deterministic default |
| 5b. Normalize shorthand regression criteria | B90 | Sweep over br bodies — now executes the commands |
| 5c. B57 dead conditional in 08-crash-recovery.bats | B76 | Branches now differ materially; "already landed" asserted for post-push phases |

## Cross-family gap → disposition table (behaviors-codex-gaps.md, all 15)

| # | Gap | Disposition |
|---|---|---|
| 1 | B74–B90 not required to become runnable tests / join the suite | **FOLDED** → new B91 (coverage manifest + checker) + B75 amended (count asserted against manifest, not hardcoded 73) |
| 2 | Real-repo checks can dirty the operator checkout | **FOLDED** → new B92 (disposable worktree/scratch copy + residue check, pre/post SHA+status capture) |
| 3 | Doctrine flip scoped only to CLAUDE.md | **FOLDED** → B86 amended (pinned operator-doc list, checked-in sweep, historical-section marker convention) |
| 4 | No crash-safe/rollback contract for hook install | **FOLDED** → new B93 (temp+atomic rename, backup, byte-identical on failure, structured error, executable bit) |
| 5 | verify doesn't reject stale/duplicate/corrupt guard state | **FOLDED** → B85 amended (five named defect rejections, pinned JSON keys, stable exit codes in --help) |
| 6 | Lock-dir digest ignores origin URL canonicalization | **FOLDED** → B89 amended (canonical origin identity pinned in --help; equivalent ssh/https/scp forms digest identically; serialization asserted across differing spellings) |
| 7 | Chain preservation untested against control-flow traps | **FOLDED** → B82 amended (documented execution order + executable bit) + B84 amended (early-exit / exec / no-shebang / non-executable foreign variants all refuse, zero byte changes) |
| 8 | Manifests can be overbroad, ambiguous, self-fulfilling | **FOLDED** → B78 amended (strict format, normalization, duplicate/glob rejection, written-set derived from the actual run, overdeclared-path negative test) + B79 amended (same format rules) |
| 9 | land-gate-families parity foolable by grep-level checks | **FOLDED** → B81 amended (structural YAML parse; comment/duplicate/empty rejection; family→job mapping proven) |
| 10 | Bead acceptance commands not required to execute/select tests | **FOLDED** → B90 amended (sweep executes every extracted command; zero-selection, stale path, harness death all fail naming the bead) |
| 11 | br/_beads dependency failures unspecified | **FOLDED** → B90 amended (fail-closed: missing tool/ledger/bead id exits nonzero by name; no silent skip, no cached prose) |
| 12 | Two-clone rollout evidence is prose, not repeatable | **FOLDED** → B85 amended, then split to **B94** at drift-guard repair (checked-in rollout evidence manifest: clone identity, repo SHA, guard version, command, timestamp, raw verify JSON; staleness rejected) |
| 13 | B84 foreign-hook choice too late-bound | **FOLDED** → B84 amended (policy PINNED at freeze: refuse on unrecognized hooks with "chain manually"; opposite behavior proven absent; recognized chains unaffected) |
| 14 | LAND_BIN seam doesn't cover installed hooks/shims | **FOLDED** → B88 amended (installed guard dispatch goes through the configured land command or documented shim; LAND_BIN probe proven during install + push) |
| 15 | ag-arpk can stay operationally open while text looks updated | **FOLDED** → B87 amended (status/labels/deps must agree with prose; `br ready` + bv triage no longer surface it as unclaimed active work, or show it blocked when kept-planned) |

## Coverage map additions (failure-mode → scenario)

| Failure mode / risk class | Scenario(s) |
|---|---|
| Harness crash poisoning done-criteria | B74, B75, B76 |
| Appended behaviors satisfied by prose alone | B91 |
| Bead acceptance not runnable / shorthand / never executed | B77, B90 |
| Bead verifier silently skipping on missing br/_beads | B90 |
| Fixture-only acceptance: the real repo never changes | B78–B86 |
| Verifier damaging the operator checkout | B92 |
| Manifest overbreadth / self-fulfilling parity | B78, B79 |
| validate.yml declaration passing as a comment/duplicate | B81 |
| Doctrine flip leaving live direct-push instructions in sister docs | B86 |
| Hook clobbering a live beads chain on the real clones | B82, B83, B84 |
| Hook install crash leaving a half-written or dead hook | B93 |
| Foreign-hook policy decided after the fact | B84 |
| Stale/duplicate/corrupt guard state passing verify | B85 |
| Rollout evidence stale or unreproducible | B94 |
| Cross-host (Mac↔bushido) serialization gap hidden | B85, B87 |
| ag-arpk lingering as phantom-ready work in triage | B87 |
| Substrate lock-in to concurrency-critical Bash 3.2 | B88 |
| Installed hooks hardcoding the wrong substrate past the seam | B88 |
| Lock-dir default drift across clones / origin spellings | B89 |

## Out of scope for this amendment pass

- Re-opening or weakening any of B1–B73 (frozen; this pass only appends).
- Implementing cross-host landing serialization (B85/B87 pin the residual and
  its owner; building it is a future epic).
- GitHub merge-queue enablement itself (B87 dispositions the bead; it does not
  build the queue).
- Re-running the 100-gap codex review (run-1 disposition table stands).
- A formal JSON-Schema document for the --status/--verify JSON (B85 pins the
  key set and exit codes in --help; a schema file is welcome but not a done
  criterion for this run).
