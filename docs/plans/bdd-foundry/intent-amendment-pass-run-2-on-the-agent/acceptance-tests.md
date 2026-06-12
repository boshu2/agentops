# land.sh amendment pass (run 2) acceptance tests — index (bdd-foundry Phase 2, ATDD)

> **Status: RED by design — executed and verified 2026-06-12 (post drift-guard
> repair): `1..21`, zero `ok`, zero harness crashes, every failure on a
> test-body assertion line.** (Prior record `1..20`; the monolithic B85 test
> was split during the drift-guard repair pass — the rollout-evidence concern
> is now its OWN scenario id **B94** with its own test, so every scenario id
> selects exactly one test — see the scenario map below and the REPAIRED
> beads-manifest.)
> Each test is the executable definition of done for one scenario
> B74–B94 in [`behaviors.md`](behaviors.md) (this run's appended contract over
> the frozen B1–B73 base; B94 = the split of B85's second concern).
> **No runnable test, no bead.**
>
> **Run the whole suite (one line):**
> ```bash
> bash docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests/run-acceptance.sh
> ```
> (Equivalent single pass without the totality/no-skip gate:
> `bats docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests`.)
>
> Requires `bats` (1.13 used), `jq`, `python3`, `git`, and `br` (tracker reads
> are FAIL-CLOSED, never skipped). Hermetic to verify: sandbox scenarios live
> in `mktemp -d` fixtures (run-1 harness); real-repo scenarios read the
> operator checkout and perform every mutating step in a **disposable clone or
> scratch file copy only** (the B92 contract); tracker scenarios read beads via
> `br show` from the **main checkout** (`BR_MAIN_CHECKOUT`, default
> `/Users/bo/dev/agentops`, `BEADS_DIR=$BR_MAIN_CHECKOUT/_beads`) — never from
> a worktree.
>
> **Pinned run-2 observable contract** (cutover verifier script paths, guard
> segment markers, foreign-hook refusal message, `--install --verify` JSON
> keys, install fault seams `LAND_TEST_INSTALL_FAIL={write,rename,chmod}`,
> backup path `.git/hooks/pre-push.pre-land-install.bak`, `LAND_BIN` substrate
> seam, lock-dir default formula, coverage/rollout manifest paths and shapes,
> br access rules) lives in ONE place:
> [`acceptance-tests/helpers2.bash`](acceptance-tests/helpers2.bash) header.
> The spec phase may renegotiate a pin by editing helpers2 — never by
> weakening a scenario (that re-opens Phase 1). The run-1 contract in
> `../acceptance-tests/helpers.bash` still applies verbatim.

## Scenario → test map

Test names embed the scenario id: `@test "B<n>: …"`. One test per scenario,
one scenario per test — 21 tests over 21 scenarios (B74–B94; B94 is the
drift-guard split of B85's rollout-evidence concern, owned by
`run2-rollout-evidence` while `run2-install-verify` keeps B85), and every
manifest ACCEPTANCE filter `-f '^B<n>:'` selects exactly one test. All paths
relative to
`docs/plans/bdd-foundry/intent-amendment-pass-run-2-on-the-agent/acceptance-tests/`.

| Scenario | Test (file) |
|---|---|
| B74 | `B74: fixture gate.d directory survives every lane clone` — `12-amend-harness.bats` |
| B75 | `B75: full suite is red ON ASSERTION — audit-red.sh is checked in, manifest-wired, and passes` — `12-amend-harness.bats` |
| B76 | `B76: B57 dead conditional repaired — post-push reruns assert 'already landed' distinctly` — `12-amend-harness.bats` |
| B77 | `B77: ag-d3-fixture-guard-yk7rq carries a runnable B62 acceptance, not a prose recipe` — `13-amend-bead.bats` |
| B78 | `B78: real repo regen write set is declared, strictly formatted, and matches reality` — `14-amend-cutover-manifests.bats` |
| B79 | `B79: real repo count-bearing docs are declared in scripts/count-docs.txt` — `14-amend-cutover-manifests.bats` |
| B80 | `B80: the manifested prose docs carry generator-owned marker blocks; hand-asserted counts are extinct` — `14-amend-cutover-manifests.bats` |
| B81 | `B81: real validate.yml declares land-gate-families STRUCTURALLY and the parity check holds` — `14-amend-cutover-manifests.bats` |
| B82 | `B82: --install CHAINS onto a beads-managed pre-push hook, never clobbers, in documented order` — `15-amend-install-chain.bats` |
| B83 | `B83: --install is idempotent and upgrades ONLY its own guard segment` — `15-amend-install-chain.bats` |
| B84 | `B84: --install policy PINNED — install on hookless, refuse on unrecognized foreign hooks` — `15-amend-install-chain.bats` |
| B85 (bead: run2-install-verify) | `B85: strict verify — pinned JSON keys, five DISTINCT named defects, naked-clone diagnosis, residual stated, checker ships bead-scoped` — `15-amend-install-chain.bats` |
| B86 | `B86: landing doctrine flips repo-wide, not just CLAUDE.md` — `16-amend-doctrine.bats` |
| B87 | `B87: ag-arpk is dispositioned — explicit chosen path, named residual, machine-readable state agrees` — `17-amend-arpk.bats` |
| B88 | `B88: the acceptance contract is implementation-agnostic through ONE LAND_BIN seam, including installed hooks` — `18-amend-seam-lock-beads.bats` |
| B89 | `B89: LAND_LOCK_DIR's production default is pinned, deterministic, and origin-IDENTITY-keyed` — `18-amend-seam-lock-beads.bats` |
| B90 | `B90: every bead's regression criteria are self-contained runnable commands; the sweep EXECUTES them and fails closed` — `18-amend-seam-lock-beads.bats` |
| B91 | `B91: every appended behavior maps to a mechanical verifier in a checked-in coverage manifest` — `19-amend-meta.bats` |
| B92 | `B92: real-repo verification is hermetic — no check dirties or damages the operator checkout` — `19-amend-meta.bats` |
| B93 | `B93: --install is crash-safe — atomic write, backup, byte-identical hook on failure` — `15-amend-install-chain.bats` |
| B94 (bead: run2-rollout-evidence; split of B85's second concern) | `B94: rollout evidence — CHECKED-IN per-clone records, fresh, checker-validated on the real repo` — `15-amend-install-chain.bats` |

## Current red anchors (field-verified 2026-06-12, all fail on assertion)

- B74: `git -C $SEED ls-files scripts/gate.d/` is empty (the `mkdir -p`/`git
  add -A` defect from the judges' finding — git can't track an empty dir).
- B75: `acceptance-tests/audit-red.sh` (base suite dir) does not exist.
- B76: red placeholder exits 97 on rerun; the byte-identical if/else in
  `08-crash-recovery.bats:24` is caught by the checked-in lint
  (`helpers2.bash :: find_identical_if_else`, validated against live data).
- B77: the bead's ACCEPTANCE carries the old `^B25:` smoke proxy, no `^B62`
  filter command.
- B78/B79: `scripts/regen-manifest.txt` / `scripts/count-docs.txt` absent from
  the real repo (the fixture invented them; the cutover must create them).
- B80: depends on `scripts/count-docs.txt` (absent).
- B81: real `validate.yml` has no `land-gate-families` declaration.
- B82–B85, B89, B93: `scripts/land.sh` is the Phase-2 red placeholder (97).
- B94: `rollout-evidence.jsonl` is not checked in (first assertion `[ -s … ]`
  fails); the checker script is also absent from the real repo.
- B86: CLAUDE.md's Land phase + Land row still say ``Push to `main` ``.
- B87: `ag-arpk` is an untouched OPEN P1 with no disposition text and IS
  surfaced by `br ready --limit 0`.
- B88: the run-1 helpers ignore `LAND_BIN`; the heredoc-aware invocation scan
  (`find_direct_sut_invocations`, validated: 28 direct invocations in the
  run-1 suite, zero false positives on assertion strings) is non-empty;
  `spec.md` lacks the `ao land`/`LAND_BIN` implementation-choice note.
- B90/B92: `scripts/sweep-bead-acceptance.sh` / `scripts/with-hermetic-check.sh`
  absent.
- B91: `coverage-manifest.txt` + `check-coverage-manifest.sh` absent.

## Harness notes (for the spec + implementation phases)

- `helpers2.bash` layers on the run-1 harness (`../acceptance-tests/helpers.bash`)
  and repoints `repo_under_test` (this suite sits one directory deeper).
- **Chained-hook fixture** (`make_chained_hook`) reproduces the real repo's
  pre-push shape — `BEGIN/END BEADS INTEGRATION v1.0.5` marker block +
  `pre-push.local` cockpit-gate dispatch — each segment instrumented to append
  its name to `$PROBE_LOG`, so chain order and chain survival are asserted
  mechanically (probe sequence + per-segment sha256).
- **Foreign-hook variants** (B84): `exit0`, `exectrap`, `noshebang`, `noexec` —
  all must refuse with "not recognized" + "chain manually" and zero byte changes.
- **Guard defect injections** (B85): stale version, duplicated segment,
  unpaired marker, missing exec bit, chain-order displacement — one per case,
  and the five reported defect tokens must be pairwise DISTINCT (asserted via
  `sort -u` count, not merely "defects nonempty").
- **B80's "passes the repo's standard CI gate" clause** is proven by CI on the
  cutover commit itself (running the full omnibus gate inside a bats test is
  not reasonable); the test asserts the regen `--check` half byte-mechanically.
- **B75/B90 runtime warning:** at green, B75 executes `audit-red.sh` (a full
  placeholder-forced base-suite run via the `LAND_BIN` seam) and B90 executes
  every bead's extracted acceptance command — both are deliberately heavy;
  they are the standing gates the behaviors demand, not unit tests.
- **Totality gate:** `run-acceptance.sh` fails fast on any skip/focus marker
  and on any B74..B94 scenario missing or duplicated (every scenario id
  present exactly once; no exceptions).
