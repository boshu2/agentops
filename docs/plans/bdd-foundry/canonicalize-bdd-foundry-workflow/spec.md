# Spec — canonicalize-bdd-foundry-workflow (phase 3)

> Derived ONLY from [`behaviors.md`](behaviors.md) (FROZEN) + the executable suite in
> [`acceptance-tests/`](acceptance-tests/) (23 red). Its single job: make those 23 tests pass.
> Anything not required by a scenario is out of scope.

## Ground facts at spec time (2026-06-12, re-verified against the live source)

1. **The S10 winner is now v7**, not v5/v6: `~/.claude/workflows/bdd-foundry.js` header reads
   `bdd-foundry v7 (2026-06-12)` (sentinel-slot guard). E1 governs: highest lineage wins;
   the suite already enforces `vc == snapshot == max(sweep) ≥ 6`.
2. **v7 re-keyed the DIR-misaim guard**: the literal `includes('DIR-MISAIM')` no longer exists —
   the guard is now `…startsWith('DIR-MISAIM')` (lines ~187–189) with the `throw` 6 lines below.
   S4's `grep -A2 "includes('DIR-MISAIM')" | grep -c throw` is **unsatisfiable** against any
   compliant canonical file (behavior change beyond the HAZARD swap is frozen out of scope).
   The enforcement INTENT ("the preflight THROWS, not a comment fossil") holds in v7.
3. **The v7 source contains one LAW-0 string hit**: the `REGISTER` template literal
   (currently line 64) quotes `` `claude -p` `` to *document the prohibition*. It is a string
   literal, exactly what frozen X3 permits ("comment/string-literal documenting the
   prohibition") — but the X3 test only accepts comment-prefixed lines. Unsatisfiable as written.
4. All other S4 floors pass against v7 today (DRIFT_SCHEMA=2, beads.json=7, DIR-MISAIM=9,
   pre-run-N=2, guard chain=2, gap_dispositions=4, `node --check` OK). The `export const meta`
   block is already a pure literal (S5 passes on content). The HAZARD line is exactly one line
   (line 32). Lineage `^// v[2345]` count is 4.
5. House gate registry: `cli/internal/gates/checks` — one `gates.Register`/seed entry per check,
   `Backing: <scripts/*.sh>`, routed by `Match` globs or always-run when `Match` is empty.
   No reverse parity test forces registry scripts into `validate.yml`, so a registry-only
   registration is safe. The cockpit pre-push hook runs `ao gate check --fast`.

**Consequence of 2+3:** this arc must make TWO surgical amendments to the derived test
artifacts (suite files live in the plan dir, not the frozen behaviors) — see C9. The frozen
behaviors themselves need no change: E1 explicitly anticipates re-grounding the greps on the
winner's content, and X3's frozen text already allows string-literal exceptions listed file:line.

## Decisions (one line each)

| Decision | Choice | Why |
|---|---|---|
| Follow mechanism | **symlink** (`ln -sfn` to the repo canonical) | drift becomes structurally impossible for bdd-foundry; matches the house `link-skill` pattern; S6's symlink branch is the simpler proof |
| E5 divergent-install semantics | **backup, then replace** (not refuse) | the real first application MUST replace the live v7 user file; backup `bdd-foundry.js.pre-canonicalize-<UTC-ts>`, path printed |
| Blocking parent | `scripts/validate-workflow-install.sh`, **registered in the Go gate registry** (always-run, blocking, Fast\|Full) | satisfies S7/X4 "wired in, not orphaned" both mechanically and honestly (every cockpit `ao gate check` run invokes it); cheap under fixture `$HOME` (no `go run`-under-foreign-HOME cache rebuild) |
| Install scope on the real HOME | `install-workflows.sh bdd-foundry.js` (arg-scoped) | leaves the drifted sibling user copies untouched — sibling remediation is E6's follow-up bead, not this arc |
| Missing-install vs broken-install | absent `$HOME/.claude/workflows/bdd-foundry.js` ⇒ SKIP (report line, exit 0); present-but-divergent or dangling symlink ⇒ FAIL | keeps CI/clean machines green while X4/S7 still fail red |

## Components

### C1 — canonical file `.claude/workflows/bdd-foundry.js` (S1, S2, S3, S4, S5, E1, X1)

Byte-equal to the S9 snapshot of the S10 winner **except one line**: line
`// HAZARD: not git-tracked; canonicalize into agentops to end the multi-lane clobber.`
is replaced in place (1 removed / 1 added, same position) by exactly:

```
// CANONICAL: .claude/workflows/bdd-foundry.js (agentops repo, git-tracked); ~/.claude/workflows/bdd-foundry.js is a symlink/copy installed via scripts/install-workflows.sh
```

Constraints the line must satisfy (S3's grep chain): starts `//`, contains the literal
`.claude/workflows/bdd-foundry.js`, the word "canonical" (case-insensitive), and
`~/.claude/workflows` — and is the ONLY line in the file matching all four (verify with the
S3 grep before commit; no other v7 line currently collides). Pre-commit verification:
`node --check`, C4 marker script green, S5 meta-block greps empty (X1: never commit a
candidate that fails any of these).

### C2 — `scripts/install-workflows.sh` (S6, S11, E5)

The NAMED re-runnable follow command. `bash scripts/install-workflows.sh [name.js ...]`;
no args = every `.claude/workflows/*.js` (generic — the script body never names a workflow,
killing the S6 special-case trap from the install side).

- Repo root: `git rev-parse --show-toplevel` from cwd. Dest: `$HOME/.claude/workflows`
  (`mkdir -p`). **Zero `/Users/bo` literals** (S11 grep).
- Per workflow: dest is a symlink (incl. dangling) → `ln -sfn <abs-repo-root>/.claude/workflows/<name>`
  (idempotent: identical readlink on re-run). Dest is a regular file byte-equal → replace with
  symlink. Dest is a regular file with **different bytes** → `cp -p` to
  `<dest>.pre-canonicalize-$(date -u +%Y%m%dT%H%M%SZ)`, print the backup path, then `ln -sfn`
  (E5 backup branch, byte-faithful). Absent → `ln -sfn`.
- Touches ONLY `$HOME`; never writes into the repo (S11: fixture repo porcelain unchanged).
  Exit non-zero on any real failure.

Recorded `INSTALL_CMD='bash scripts/install-workflows.sh bdd-foundry.js'`.

### C3 — `scripts/check-workflow-drift.sh` (S7, E6, X4)

The freshness/resolution check. Repo root from cwd-git; installed dir from `$HOME`.

- **Blocking set = `bdd-foundry.js`:** absent entirely → `SKIP: bdd-foundry.js not installed`
  exit 0; dangling symlink → exit 1 naming the offending path (X4); symlink → must realpath-resolve
  to the repo canonical, else exit 1; regular file → `cmp -s` against canonical, else exit 1.
  Every failure message contains `bdd-foundry.js`.
- **Report-only set = every other repo-tracked `.claude/workflows/*.js`** (today: bead-crank.js,
  operating-loop.js — both named in the header comment, satisfying S6's sibling grep): same
  comparison, divergence emits `DRIFT-REPORT: <name> …` to stdout, **never** affects the exit
  code (E6: exit 0 with `bead-crank` in output on the mutated-sibling fixture).
- No LAW-0 strings anywhere in this or any added script (X3 greps changed `*.sh`).

Recorded `DRIFT_CHECK_CMD='bash scripts/check-workflow-drift.sh'`.

### C4 — `scripts/check-bdd-foundry-markers.sh` (S4, X2)

Marker/enforcement floor check. `$1` = candidate file, default = repo canonical
(so the gate can run it argless). Fails non-zero **naming the missing marker** on the first
floor violated; all floors, exactly S4's:

| Check | Floor | Failure output must contain |
|---|---|---|
| `node --check $1` | exit 0 | node's error |
| `grep -c 'DRIFT_SCHEMA'` | ≥ 2 | `DRIFT_SCHEMA` |
| `grep -c 'beads\.json'` | ≥ 3 | `beads.json` |
| `grep -c 'DIR-MISAIM'` | ≥ 2 | `DIR-MISAIM` |
| `grep -c 'pre-run-N base snapshot'` | ≥ 1 | the marker |
| `grep -A6 -E "includes\('DIR-MISAIM'\)\|startsWith\('DIR-MISAIM'\)" \| grep -c throw` | ≥ 1 | `DIR-MISAIM`/`throw` |
| `grep -Fc 'cycleFree && uncovered.length === 0 && driftOk'` | ≥ 2 | `cycleFree` (X2 accepts cycleFree\|guard\|tracker-write) |
| `grep -c 'gap_dispositions'` | ≥ 2 | `gap_dispositions` |

The throw-window pattern is v5/v7-tolerant by design (ground fact 2). `node` missing ⇒ FAIL
loudly (admission gates fail closed; node exists on Mac/bushido/CI).

Recorded `MARKER_CHECK_CMD='bash scripts/check-bdd-foundry-markers.sh'`.

### C5 — `scripts/validate-workflow-install.sh` (S7, X4 — the named blocking parent)

Thin parent: run C3, then C4 (argless, repo canonical); exit non-zero if either fails,
propagating their output. This is the surface recorded as
`BLOCKING_PARENT_CMD='bash scripts/validate-workflow-install.sh'`.

### C6 — gate registration (S7 "blocking", E3)

New file `cli/internal/gates/checks/workflow_install.go`: one registration
`{ID: "workflow.install-drift", Tiers: gates.Fast | gates.Full, Blocking: true, Backing: "validate-workflow-install.sh"}`
— **always-run** (no `Match`): the fixture mutation lives in `$HOME`, which changed-file routing
can never see, so routing would orphan the check. Paired `workflow_install_test.go` asserts the
registration exists with exactly these fields (house L1; satisfies the cli/** test-pair gate).
CI/clean-machine safety comes from C3's SKIP-when-absent. Verify `cd cli && go build ./... &&
go vet ./... && go test ./...` before commit; confirm `ao gate check --fast --scope head` green
in the worktree (E3, incl. no context-map/regen demand — no `skills/**` or `docs/contracts/**`
path is touched).

### C7 — plan-dir evidence artifacts (S9, S10, S12, E4, X3, X5)

All under this plan dir, written in this order:

1. `pre-state.porcelain` — `git -C /Users/bo/dev/agentops status --porcelain` captured **before
   any work** (E4 compares post-landing against it).
2. `candidate-sweep.md` (S10) — executed output of: `ls -la ~/.claude/workflows/ | grep bdd-foundry`,
   `git -C /Users/bo/dev/agentops ls-files '.claude/workflows/'`, a probe of every
   `git worktree list` path for the file, `ls <plan-dir>/*.js`; each candidate with `head -1`
   version + SHA256; exactly one `WINNER` (expected: the ~/.claude v7 file); rule line —
   either `highest lineage version wins` or `single v7 source, no reconciliation needed`
   (E2 vacuous branch). Include one verbatim
   `BEADS_DIR=/Users/bo/dev/agentops/_beads br …` invocation here or in landed-evidence.md (X5).
3. `source-snapshot-<YYYYMMDDTHHMMSSZ>.js` + `source-snapshot.sha256` (S9) — `cp` of the winner
   taken BEFORE any write to canonical/installed paths; sha256 file uses the relative basename
   (the suite runs `shasum -c` from the plan dir); snapshot timestamp must precede the first
   commit touching the canonical file. Never edited afterward. S2/E1/E2 compare against this
   file only.
4. `evidence.env` — exact keys/values:

   ```
   BEAD_ID=<ag-…>                  SIBLING_DRIFT_BEAD_ID=<ag-…>
   ARC_BASE_SHA=<sha>              LANDED_SHA=<sha>
   WORKTREE_PATH=/Users/bo/dev/agentops/wt-<bead-id>   WORK_BRANCH=feat/<bead-id>-<slug>
   INSTALL_CMD='bash scripts/install-workflows.sh bdd-foundry.js'
   DRIFT_CHECK_CMD='bash scripts/check-workflow-drift.sh'
   BLOCKING_PARENT_CMD='bash scripts/validate-workflow-install.sh'
   MARKER_CHECK_CMD='bash scripts/check-bdd-foundry-markers.sh'
   ADDED_SCRIPTS='scripts/install-workflows.sh scripts/check-workflow-drift.sh scripts/check-bdd-foundry-markers.sh scripts/validate-workflow-install.sh'
   WIRING_FILES='<ADDED_SCRIPTS> cli/internal/gates/checks/workflow_install.go cli/internal/gates/checks/workflow_install_test.go'
   ```

5. `landed-evidence.md` (S12, X3, X5) — written AFTER the last file change: landed HEAD SHA;
   `shasum -a 256` of the canonical file at that SHA (must equal
   `git show <SHA>:.claude/workflows/bdd-foundry.js | shasum -a 256`); the readlink/cmp follow
   result; the gate command with `exit code: 0`; the LAW-0 string-literal exception listed as
   `.claude/workflows/bdd-foundry.js:<line-of-REGISTER>` (ground fact 3 — recompute the line in
   the landed file); every tracker invocation verbatim in the
   `BEADS_DIR=/Users/bo/dev/agentops/_beads br …` form; recorded verification commands limited
   to node --check / grep / diff / cmp / shasum / readlink / git / ls / sed / fixture script runs
   (no `node …bdd-foundry.js` without `--check`, no Workflow-tool mention).

### C8 — beads (S8, E6, X5)

From the **main checkout only** (`git-dir == git-common-dir` there), exact form
`BEADS_DIR=/Users/bo/dev/agentops/_beads br …`:

- `BEAD_ID`: the arc bead, created before implementation, cited in **every** commit in
  `ARC_BASE_SHA..LANDED_SHA` (they all touch canonical/plan-dir/WIRING_FILES surfaces);
  closed after landing with a note naming the landed SHA (≥ first 7 chars).
- `SIBLING_DRIFT_BEAD_ID`: the bead-crank/operating-loop user-copy drift remediation bead
  (report-only finding → someone else's gated arc), id recorded in landed-evidence.md.
- Never from the worktree; no `_beads/`/`.beads/` may appear in the worktree; no `_beads/`
  commit in the public repo (the ledger repo is pushed separately).

### C9 — two derived-test amendments (ground facts 2+3; E1's re-grounding clause)

In-arc, bead-cited, each with a one-line provenance comment citing E1 / frozen-X3:

1. `acceptance-tests/happy.bats` S4: replace the throw-window grep with the v7-tolerant form
   used by C4 — `grep -A6 -E "includes\('DIR-MISAIM'\)|startsWith\('DIR-MISAIM'\)" "$CANON" | grep -c 'throw'` ≥ 1.
   (Frozen E1: "the S4 marker + enforcement greps are re-run against that content and still
   pass" — the grep tracks the winner's enforcement shape, the floor is unchanged.)
2. `acceptance-tests/error.bats` X3: accept a hit line when it is comment-prefixed **or** the
   match sits inside a quoted/template string (mechanical proxy: a `` ` ``/`'`/`"` precedes the
   match on the line) — the `file:line`-listed-in-evidence requirement stays mandatory for every
   hit. (Frozen X3 text: "comment/**string-literal** documenting the prohibition".)

No other test file changes. The frozen behaviors.md is not touched.

### C10 — real-HOME application (S3, S6, S7)

After landing on main, from the main checkout with real `$HOME`: run `INSTALL_CMD` once
(backs up the live v7 user file — which is also the S9 snapshot, double-held — and installs
the symlink), then `DRIFT_CHECK_CMD` and `BLOCKING_PARENT_CMD` (both exit 0), then the full
bats suite. 23 green is the exit criterion.

## Behavior → component map

| Scenario | Satisfied by |
|---|---|
| S1 | C1 committed + landed |
| S2 | C1 (1-line swap) vs C7.3 snapshot |
| S3 | C1 replacement line + C10 (installed copy follows) |
| S4 | C1 content (v7 floors pass today) + C9.1 amendment + C4 |
| S5 | C1 (v7 meta already pure — no change needed) |
| S6 | C2 + C10; symlink branch; ADDED_SCRIPTS sibling grep via C3's named report set |
| S7 | C3 (red on mutation) + C5/C6 (blocking parent wired) |
| S8 | C8 + commit discipline |
| S9 | C7.3 (pre-write, sha-pinned, timestamp ≤ first canonical commit) |
| S10 | C7.2 |
| S11 | C2 (cwd-root + $HOME resolution, mkdir -p, idempotent ln -sfn, no /Users/bo, no repo writes) |
| S12 | C7.5 |
| E1 | C7.2/C7.3 take the v7 winner; C1 header = v7; C9.1 |
| E2 | vacuous (single source expected) — C7.2 states the rule line; else reconciliation.diff + disposition table per frozen text |
| E3 | change surface = C1 + ADDED_SCRIPTS + C6 files + plan dir only; gate green in worktree |
| E4 | C7.1 pre-state + worktree-mandatory implementation (`wt-<bead-id>`, `feat/<bead-id>-…`) |
| E5 | C2 backup branch |
| E6 | C3 report-only siblings + C8 sibling bead + no sibling-content commit |
| X1 | C1 pre-commit verification (node --check before every canonical commit) |
| X2 | C4 (+ C6 makes it push-blocking) |
| X3 | C7.5 exception list + C9.2 + no LAW-0 strings in added scripts |
| X4 | C3 dangling-symlink branch + C5/C6 |
| X5 | C8 + C7 verbatim invocation records |

## Sequence (ordering is load-bearing)

1. C7.1 pre-state → C8 beads → C7.2 sweep → C7.3 snapshot (all before any canonical/installed write).
2. `git worktree add wt-<bead-id> -b feat/<bead-id>-<slug>` → C1, C2–C5 scripts (shellcheck-clean),
   C6 Go registration+test, C9 amendments — verify (node --check, C4, `cd cli && make test`,
   `ao gate check --fast --scope head`) — commit(s) citing BEAD_ID.
3. Land on main through the cockpit gate (operator/conductor pushes; rebase-on-reject).
4. C10 install + checks against real HOME → C7.4 evidence.env → C7.5 landed-evidence.md →
   close BEAD_ID with the landed SHA → run the bats suite → 23/23 green.

## Out of scope (re-affirmed from the frozen behaviors)

Sibling content remediation (the filed bead's arc), the fleet-wide workflow-home decision
(`mto-pyss`), any bdd-foundry.js behavior change beyond the HAZARD-line swap, and
`scripts/install.sh` / dotfiles wiring.
