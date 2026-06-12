# Missing Behaviors Review

INTENT: identify the highest-risk behaviors missing from `docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/behaviors.md` before the amendment pass is treated as complete.

Review stance: independent cross-family contract review. I treated B74-B90 as the amendment contract and checked B1-B73 only to avoid re-listing gaps already covered by the frozen base.

## Top Gaps

1. B74-B90 are not required to become runnable acceptance tests or join the one-command suite.
   B75 still expects exactly 73 failing tests under the red placeholder, which lets an implementer satisfy the amendment while never adding executable tests for B74-B90. Add a meta-scenario requiring every appended behavior to have a mapped bats test, to be selected by the suite entry point, and to fail red-on-assertion before implementation.

2. Real-repo cutover checks can dirty or damage the operator checkout.
   B78-B81 intentionally run against the real repo and include mutating actions: `regen-all.sh`, editing count markers, and removing validate.yml family tokens. Add behavior requiring every real-repo negative check to run in a disposable worktree or restore the exact pre-test SHA/status afterward, with failure if the verifier leaves any uncommitted or untracked residue.

3. Doctrine flip is scoped too narrowly to `CLAUDE.md`.
   B86 can pass while `AGENTS.md`, `AGENTS-WORKFLOW.md`, `docs/agent-workflow-reference.md`, README snippets, or quick-reference blocks still instruct agents to push directly to main. Add a repo-wide operator-doc sweep that permits direct-push language only in explicitly historical/superseded sections and requires every live landing instruction to name `scripts/land.sh`.

4. Hook installation has no crash-safe or rollback contract.
   B82-B84 cover happy-path preservation, idempotency, and foreign hooks, but not interruption during hook rewrite, disk-full, permission errors, or failed chmod. Add behavior requiring install to write via temp+atomic rename, preserve a backup, leave the prior hook byte-identical on failure, keep the hook executable, and report a nonzero structured install error.

5. `--install --verify` does not prove the installed guard is current and singular.
   B85 reports guard presence/version but never says an old version, duplicate guard blocks, malformed marker pairs, or mixed old/new segments must fail. Add behavior requiring verify to reject stale versions, duplicate segments, corrupt markers, missing executable bit, and chain-order defects, with a pinned JSON schema and stable exit codes.

6. The origin-keyed default lock directory ignores remote URL canonicalization.
   B89 only compares clones with the same origin URL string. Two same-host clones of the same remote can use `git@github.com:org/repo.git`, `ssh://git@github.com/org/repo`, or `https://github.com/org/repo.git` and silently get different locks. Add behavior either canonicalizing equivalent origin identities to one digest or explicitly detecting/refusing non-canonical origin URLs before relying on default serialization.

7. Hook-chain preservation is not tested against control-flow traps.
   B82/B84 can be satisfied by preserving bytes while placing the guard after an existing `exit`, `exec`, or non-tail hook path that prevents later segments from running in realistic hooks. Add variants with early-exit foreign hooks, non-executable existing hooks, missing shebangs, and hooks that dispatch `pre-push.local`; done should prove the chosen chain strategy executes the intended segments in the documented order or refuses without changing bytes.

8. Real manifests can be overbroad, ambiguous, or self-fulfilling.
   B78-B80 say manifests exist and parity holds, but an implementer could use broad globs like `docs/**`, comments parsed as paths, duplicate paths, non-normalized `../` entries, or a checker that trusts the same buggy manifest it is validating. Add behavior requiring a strict manifest format, normalized repo-relative paths, duplicate rejection, no broad entries that cover source-owned files, and negative tests where undeclared writes and overdeclared paths are independently detected.

9. `land-gate-families` parity can be fooled by grep-level YAML checks.
   B81 requires exactly one declaration line, but not that it is valid workflow YAML, non-comment, non-empty, attached to the intended job, or complete relative to required CI jobs. Add behavior that parses `.github/workflows/validate.yml` structurally, rejects commented or duplicate declarations, fails on empty family lists, and proves each required CI gate family is represented in both validate.yml and land.sh's gate plan.

10. Bead acceptance commands are not required to execute and select tests.
   B77 runs one B62 command, but B90 only requires runnable-looking commands in bead text. A typoed filter that selects zero tests, a stale path, or a command requiring hidden local env could slip through. Add behavior that extracts every bead acceptance/regression command, runs it from a clean repo root with the declared env only, and fails if it selects zero tests or exits for harness/setup reasons instead of the expected red/green assertion result.

11. `br`/private `_beads` dependency failures are not specified.
   B77, B87, and B90 depend on `/Users/bo/dev/agentops/_beads` and `br show`, but they do not say what happens when `br` is absent, the private ledger is missing, the bead id is not found, or command output changes. Add fail-closed behavior requiring the verifier to name the missing tool/path/bead and exit nonzero, never silently skip bead checks or substitute cached prose.

12. Two-clone rollout evidence is prose, not a repeatable done condition.
   B85 says verify must pass on Mac and bushido with evidence captured, but it does not pin where that evidence lives, what command generated it, what commit/version it applies to, or how stale evidence is rejected. Add behavior requiring a checked-in rollout manifest with clone identity, repo SHA, guard version, command, timestamp, and verify JSON; changing the guard version or commit should invalidate old evidence.

13. The B84 foreign-hook choice remains too late-bound.
   B84 allows either chain-preserve or refuse, as long as it is documented. That lets implementation choose after the fact and makes rollout expectations unclear for real clones. Add a done condition pinning the chosen policy in `--help` and spec before implementation, plus tests proving the opposite behavior does not occur.

14. The LAND_BIN seam does not cover installed hooks and shims.
   B88 ensures acceptance-test invocations route through one helper, but `--install` may still generate hooks that hard-code `scripts/land.sh`, making an `ao land` implementation pass tests while installed clones call the wrong substrate. Add behavior requiring installed guard segments to dispatch through the same configured land command or through a documented `scripts/land.sh` compatibility shim, and verify this with `LAND_BIN` set to a probe during install and push.

15. `ag-arpk` disposition can remain operationally open while text appears updated.
   B87 allows prose stating "deferred" or "superseded", but does not require machine-readable status/labels/dependencies to stop `br ready`/`bv` from surfacing the P1 as active work. Add behavior requiring the bead's status, priority/labels, dependency edge, and body to agree; a graph triage command should no longer present `ag-arpk` as an unhandled blocker unless the chosen path is "keep merge queue planned."
