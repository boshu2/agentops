# Acceptance tests — canonicalize-bdd-foundry-workflow (phase 2, ATDD)

> Executable definition of done for the FROZEN behaviors in [`behaviors.md`](behaviors.md).
> Framework: **bats** (the house shell-test framework, same as `tests/scripts/*.bats`).
> Written test-first 2026-06-12 — the full suite is **RED (23/23 failing)** until the arc lands.
> Every failure is an intentional `ACCEPTANCE-FAIL:` assertion, not a harness error.

## Run the whole suite (one line)

```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/
```

(Verbose: add `--print-output-on-failure`. Single scenario: `bats <file>.bats --filter 'S7'`.)

## Scenario → test map

| Scenario id | Test name | File |
|---|---|---|
| S1 | `S1 canonical-file-tracked-at-house-path` | `acceptance-tests/happy.bats` |
| S2 | `S2 canonical-equals-immutable-snapshot-except-hazard-line` | `acceptance-tests/happy.bats` |
| S3 | `S3 hazard-line-retired-and-replaced` | `acceptance-tests/happy.bats` |
| S4 | `S4 syntax-markers-and-enforcement-shapes` | `acceptance-tests/happy.bats` |
| S5 | `S5 meta-block-pure-literal` | `acceptance-tests/happy.bats` |
| S6 | `S6 installed-copy-mechanically-follows-via-named-command` | `acceptance-tests/happy.bats` |
| S7 | `S7 drift-check-fails-red-and-blocks` | `acceptance-tests/happy.bats` |
| S8 | `S8 whole-arc-bead-cited-and-closed` | `acceptance-tests/happy.bats` |
| S9 | `S9 immutable-pre-write-source-snapshot` | `acceptance-tests/happy.bats` |
| S10 | `S10 candidate-sweep-recorded-before-winner` | `acceptance-tests/happy.bats` |
| S11 | `S11 clean-home-fixture-idempotent-portable` | `acceptance-tests/happy.bats` |
| S12 | `S12 evidence-anchored-to-landed-head` | `acceptance-tests/happy.bats` |
| E1 | `E1 source-already-v6-takes-highest` | `acceptance-tests/edge.bats` |
| E2 | `E2 same-version-divergent-hand-merge-with-hunk-dispositions` | `acceptance-tests/edge.bats` |
| E3 | `E3 change-surface-disjoint-no-regen-tax` | `acceptance-tests/edge.bats` |
| E4 | `E4 worktree-isolation-unconditional` | `acceptance-tests/edge.bats` |
| E5 | `E5 installed-local-edits-backed-up-or-refused` | `acceptance-tests/edge.bats` |
| E6 | `E6 sibling-drift-scoped-blocking-report-only-elsewhere` | `acceptance-tests/edge.bats` |
| X1 | `X1 syntax-failure-blocks-the-copy` | `acceptance-tests/error.bats` |
| X2 | `X2 missing-marker-blocks-the-push` | `acceptance-tests/error.bats` |
| X3 | `X3 no-live-run-and-no-law0-surface-anywhere` | `acceptance-tests/error.bats` |
| X4 | `X4 dangling-or-misaimed-follow-fails` | `acceptance-tests/error.bats` |
| X5 | `X5 tracker-never-run-from-worktree` | `acceptance-tests/error.bats` |

Shared helpers: `acceptance-tests/helpers.bash` (loaded by all three files).

## Evidence contract (what the implementing arc MUST produce)

The behaviors repeatedly require things "recorded in the plan-dir evidence". The tests make
that mechanical via **`evidence.env`** in this plan dir — a flat KEY=VALUE file sourced by
the suite. The arc that builds the feature writes it; the suite stays red until it exists
and every artifact it points at is real.

### `evidence.env` required keys

| Key | Meaning | Consumed by |
|---|---|---|
| `BEAD_ID` | the arc's bead (`^ag-`) | S8, E4 |
| `SIBLING_DRIFT_BEAD_ID` | follow-up bead for the bead-crank drift (`^ag-`) | E6 |
| `ARC_BASE_SHA` | main SHA the work branch was cut/rebased from | S8, E3, E6, X1, X3 |
| `LANDED_SHA` | final landed HEAD SHA on main | S8, S11, S12, E3, E6, X1, X3 |
| `WORKTREE_PATH` | implementation worktree (`wt-<bead-id>…`) | E4, X5 |
| `WORK_BRANCH` | `<type>/<bead-id>-…` branch | E4 |
| `INSTALL_CMD` | the NAMED re-runnable follow command (S6) | S6, S11, E5 |
| `DRIFT_CHECK_CMD` | the freshness/resolution check (S7) | S6, S7, E6, X4 |
| `BLOCKING_PARENT_CMD` | the blocking surface that invokes the check (S7) | S7, X4 |
| `MARKER_CHECK_CMD` | marker/enforcement verification; takes the candidate file as `$1` | X2 |
| `ADDED_SCRIPTS` | space-sep repo-relative NEW install/drift script files | S6, S11 |
| `WIRING_FILES` | space-sep repo-relative non-plan non-canonical files touched (⊇ `ADDED_SCRIPTS`) | S8, E3 |

### Command-string contract

`INSTALL_CMD` / `DRIFT_CHECK_CMD` / `BLOCKING_PARENT_CMD` / `MARKER_CHECK_CMD` are executed
by the suite as `( cd <repo-root> && HOME=<dir> eval "<cmd>" )`. Therefore the scripts MUST:

- resolve the repo root from cwd/git, and the installed dir from **`$HOME/.claude/workflows`** —
  never hardcoded `/Users/bo` paths (S11 greps the added scripts for `/Users/bo` = 0);
- exit non-zero on bdd-foundry drift/dangling-follow, naming `bdd-foundry.js` in output;
- treat sibling (bead-crank / operating-loop) drift as a **report line, never exit-code** (E6);
- be idempotent and auto-create the workflows dir in a clean `$HOME` (S11);
- refuse-or-backup when the installed file holds divergent local edits (E5; backup pattern
  `bdd-foundry.js.pre-canonicalize-<ts>` or a backup path printed in output).

### Other plan-dir artifacts the suite asserts

| Artifact | Spec | Consumed by |
|---|---|---|
| `source-snapshot-<YYYYMMDDTHHMMSSZ>.js` | immutable pre-write snapshot of the S10 winner (UTC ts in name) | S2, S9, E1, E2 |
| `source-snapshot.sha256` | `shasum -a 256` record; `shasum -c` must pass forever | S9 |
| `candidate-sweep.md` | the recorded sweep: home-dir `ls -la`, repo `ls-files`, `worktree list` probe, plan-dir `*.js` probe; per-candidate version + SHA256; exactly one `WINNER`; the rule applied | S10, E1, E2 |
| `landed-evidence.md` | landed HEAD SHA, canonical SHA256 at that SHA, readlink/cmp result, gate exit 0; LAW-0 comment exceptions as `file:line`; tracker invocations in the exact `BEADS_DIR=/Users/bo/dev/agentops/_beads br …` form | S12, X3, X5 |
| `pre-state.porcelain` | `git status --porcelain` of the main checkout captured at work start | E4 |
| `reconciliation.diff` + `reconciliation*.md`/`*disposition*.md` | only if two same-version divergent claimants existed (E2; else vacuous via the sweep's "single vN source" line) | E2 |

## Interpretation notes (where the frozen text needed a mechanical reading)

- **E1 live ground fact:** at test-authoring time `~/.claude/workflows/bdd-foundry.js` is already
  **v6** — the E1 branch is active, so the suite enforces canonical version ≥ 6 and
  == the snapshot's, ≥ the highest version recorded in the sweep.
- **S5 bare-token regex** excludes hyphenated literals (`DIR-MISAIM` in a string is not the
  runtime identifier `DIR`).
- **S9 ordering** is verified mechanically: snapshot filename UTC ts ≤ author time of the first
  commit touching the canonical file.
- **E3/S11 fixtures** use detached `git worktree add` checkouts at `LANDED_SHA` under the bats
  tmpdir (auto-cleaned; `worktree prune` in teardown), so the gate and install runs never touch
  the shared checkout.
- **No live workflow run, LAW 0:** the suite itself only uses `node --check`, grep/diff/cmp/shasum/
  readlink/git/ls/sed, `br show` (read-only), `ao gate check`, and fixture runs of the
  install/drift scripts — exactly the verification surface behaviors.md permits.

## Current status (2026-06-12, pre-implementation)

`1..23` — **23 `not ok`, 0 `ok`** (red as required). Confirmed every failure is an
`ACCEPTANCE-FAIL` assertion (missing canonical file / evidence.env / plan-dir artifacts),
not a bats or shell error.
