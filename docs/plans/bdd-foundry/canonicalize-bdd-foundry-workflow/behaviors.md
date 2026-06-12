# Behaviors — canonicalize the bdd-foundry workflow into agentops — FROZEN

> **FROZEN definition of DONE** (phase 2, 2026-06-12). Phase-1 draft hardened against the
> cross-family gap review (`behaviors-codex-gaps.md`) — all 15 gaps dispositioned below;
> every one folded. Every scenario is concrete enough to become a runnable test — shell
> assertions over files/git only. **No live workflow run** (LAW 0 + the ~1M-token cost
> rule): verification is `node --check` + structural greps + fixture runs of the
> install/drift scripts only.

## Verified ground facts (recon 2026-06-12, main checkout `/Users/bo/dev/agentops`)

- Source of truth: `~/.claude/workflows/bdd-foundry.js` — header reads **v5 (2026-06-12)**, 276 lines, `node --check` passes today.
- All structural markers present in the source, with their enforcement shapes confirmed by grep: `DIR-MISAIM` (v5; the conductor `throw` sits within 2 lines of the `includes('DIR-MISAIM')` check), `pre-run-N base snapshot` (v5), `DRIFT_SCHEMA` (v4), `beads.json` plumbing (v3), the tracker-write guard chain `cycleFree && uncovered.length === 0 && driftOk` (2 sites), `gap_dispositions` in the frozen-behaviors required schema, and the `HAZARD: not git-tracked` line at the end of the lineage header.
- House canonical home: `<repo>/.claude/workflows/<name>.js`, **git-tracked** — `git ls-files .claude/workflows/` already lists `bead-crank.js` and `operating-loop.js`.
- Distribution today is **manual copy, no mechanical follow**: `scripts/install.sh` has zero workflow wiring; `dotfiles/bin/link-skill` covers skills only; MAP-REPOS §J/G5 marks `~/.claude/workflows/` "unhomed (decide)" (bead `mto-pyss`).
- **Live proof of the drift hazard:** repo `bead-crank.js` ≠ `~/.claude/workflows/bead-crank.js` right now (repo got `2fe1753e7` on 2026-06-12; the user copy is dated Jun 4). Either follow mechanism satisfies S6/S7 — the testable invariant is resolution-to-canonical (symlink) or byte-identity + a failing drift check (copy).
- Tracker invocation: exactly `BEADS_DIR=/Users/bo/dev/agentops/_beads br …` from the main checkout; never from a worktree. The `_beads/` ledger is a PRIVATE nested repo — bead bodies never land in the public repo.
- Working state: this lane's cwd is the main checkout (not a worktree); another lane has previously edited the same source file — reconciliation is by lineage header, highest version wins, with per-hunk dispositions (E2).

## Feature

`bdd-foundry.js` becomes a git-tracked canonical file at `.claude/workflows/bdd-foundry.js`
in the agentops repo — byte-equal to an immutable pre-copy snapshot of the highest-lineage
source except the retired HAZARD line — with the `~/.claude/workflows/` copy mechanically
following the canonical file via a named re-runnable command proven in a clean-HOME fixture,
drift detectable by a check that fails red AND is wired into a blocking surface, the whole
landed arc bead-tracked with every commit citing the bead and the bead closed with evidence
anchored to the landed HEAD, and the change landed on main through the cockpit gate with no
collateral regen surfaces.

---

## Happy path

### S1 — canonical-file-tracked-at-house-path (happy)
```gherkin
Scenario: bdd-foundry.js is git-tracked at the same home as its sibling workflows
  Given the main checkout /Users/bo/dev/agentops with the change landed on main
  When I run `git -C /Users/bo/dev/agentops ls-files .claude/workflows/bdd-foundry.js`
  Then stdout is exactly ".claude/workflows/bdd-foundry.js"
  And `git -C /Users/bo/dev/agentops ls-files .claude/workflows/` still lists bead-crank.js and operating-loop.js (siblings untouched)
```

### S2 — canonical-equals-immutable-snapshot-except-hazard-line (happy)
```gherkin
Scenario: canonical content equals the immutable pre-copy snapshot, sole delta = the HAZARD line swap
  Given the immutable source snapshot saved per S9 (NOT the live ~/.claude/workflows/ path, which S6 mutates)
  When I run `diff <plan-dir>/source-snapshot-<ts>.js .claude/workflows/bdd-foundry.js`
  Then the only changed hunk is the single header line that began "// HAZARD: not git-tracked"
  And the canonical file's first line matches the regex "bdd-foundry v(\d+)" with the version equal to the snapshot's (5, or higher per E1)
  And every lineage line is present: `grep -c "^// v[2345]" .claude/workflows/bdd-foundry.js` ≥ 4
```

### S3 — hazard-line-retired-and-replaced (happy)
```gherkin
Scenario: the HAZARD line is gone, replaced by one line naming the canonical home
  Given the canonical .claude/workflows/bdd-foundry.js on main
  When I run `grep -c "HAZARD: not git-tracked" .claude/workflows/bdd-foundry.js`
  Then the count is 0
  And `grep -c "HAZARD: not git-tracked" ~/.claude/workflows/bdd-foundry.js` is also 0 (the installed copy follows)
  And exactly one header comment line names ".claude/workflows/bdd-foundry.js" as canonical AND states that ~/.claude/workflows/bdd-foundry.js is a copy/symlink per the install pattern
```

### S4 — syntax-markers-and-enforcement-shapes (happy)
```gherkin
Scenario: canonical file parses, carries every hardening marker, AND the markers still enforce (not comment fossils)
  Given the canonical .claude/workflows/bdd-foundry.js on main
  When I run `node --check .claude/workflows/bdd-foundry.js`
  Then it exits 0
  And `grep -c "DRIFT_SCHEMA" .claude/workflows/bdd-foundry.js` ≥ 2 (v4 drift-guard: definition + use)
  And `grep -c "beads\.json" .claude/workflows/bdd-foundry.js` ≥ 3 (v3 file plumbing in beadify, manifest, drift-guard, tracker-write stages)
  And `grep -c "DIR-MISAIM" .claude/workflows/bdd-foundry.js` ≥ 2 (v5 preflight: prompt + conductor throw)
  And `grep -c "pre-run-N base snapshot" .claude/workflows/bdd-foundry.js` ≥ 1 (v5 base snapshot)
  # Enforcement shapes (gap 11) — grounded against the live v5 source, all pass today:
  And `grep -A2 "includes('DIR-MISAIM')" .claude/workflows/bdd-foundry.js | grep -c "throw"` ≥ 1 (the preflight THROWS, it isn't a comment)
  And `grep -c "cycleFree && uncovered.length === 0 && driftOk" .claude/workflows/bdd-foundry.js` ≥ 2 (tracker write still guarded by the full chain, at both the gate and the summary)
  And `grep -c "gap_dispositions" .claude/workflows/bdd-foundry.js` ≥ 2 (the frozen-behaviors schema still REQUIRES dispositions)
  # In the verbatim path these are doubly guaranteed by S2/S9 byte-equality; in the E1/E2
  # merge branches these greps are the floor and are MANDATORY.
```

### S5 — meta-block-pure-literal (happy)
```gherkin
Scenario: the exported meta block is a pure object literal (Workflow-tool discoverable)
  Given the canonical .claude/workflows/bdd-foundry.js
  When I extract the lines from "export const meta = {" through its closing "}" (the contiguous block sed prints)
  Then the block contains no "${" template interpolation
  And the block references no runtime identifier (no occurrence of `args`, `TRACKER`, `DIR`, `RUN_TAG` as bare tokens)
  And `node --input-type=module -e "import('file://$PWD/.claude/workflows/bdd-foundry.js')"` is NOT required — the literal check is static (no execution)
```

### S6 — installed-copy-mechanically-follows-via-named-command (happy)
```gherkin
Scenario: ~/.claude/workflows/bdd-foundry.js follows the canonical file by a named re-runnable mechanism, not by hand
  Given the change is landed on main and the install step applied
  When I inspect ~/.claude/workflows/bdd-foundry.js
  Then EITHER `readlink ~/.claude/workflows/bdd-foundry.js` resolves (after normalization) to /Users/bo/dev/agentops/.claude/workflows/bdd-foundry.js
  Or `cmp -s ~/.claude/workflows/bdd-foundry.js /Users/bo/dev/agentops/.claude/workflows/bdd-foundry.js` exits 0 AND a named drift-check command exists per S7
  And the follow state was produced by a NAMED re-runnable command (script or installer target) recorded in the plan-dir evidence — hand-`ln`/hand-`cp` with no committed command does NOT satisfy this scenario
  And the mechanism is the SAME one wired for the repo's other workflows in this change (one pattern, no bdd-foundry special case)
```

### S7 — drift-check-fails-red-and-blocks (happy)
```gherkin
Scenario: the freshness check catches divergence by mutation AND is invoked by a blocking surface
  Given the drift/freshness check added by this change (copy branch; a symlink-resolution check otherwise)
  And a named blocking parent that invokes it — the install-validation script or the cockpit gate wiring — recorded in the plan-dir evidence
  When I copy the canonical file to a temp dir, append one byte to the temp installed-copy stand-in, and run the check pointed at the pair
  Then the check exits non-zero and its output names "bdd-foundry.js"
  And invoking the named blocking parent against the same mutated fixture also exits non-zero (the check is wired in, not orphaned)
  And run against the real un-mutated pair, both the check and the parent exit 0
```

### S8 — whole-arc-bead-cited-and-closed (happy)
```gherkin
Scenario: the entire landed arc is bead-tracked, every commit cites the bead, and the bead closes with evidence
  Given the bead created via `BEADS_DIR=/Users/bo/dev/agentops/_beads br create …` from the main checkout
  When I run `BEADS_DIR=/Users/bo/dev/agentops/_beads br show <id>` and `git log --oneline -20 main`
  Then the bead id matches ^ag-
  And EVERY commit in the arc that touches .claude/workflows/bdd-foundry.js, the install/freshness wiring file(s), or docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/** contains that bead id in its message (not only the workflow-file commit)
  And after landing, the bead status is closed, with a close note naming the final landed commit SHA
  And `git log --all --oneline -- _beads/` in the PUBLIC repo shows no new commit (the ledger stays in its private nested repo)
```

### S9 — immutable-pre-write-source-snapshot (happy)
```gherkin
Scenario: an immutable snapshot of the chosen source is saved BEFORE any write to canonical or installed paths
  Given the winning source file chosen per S10's sweep, and no write yet by this lane to .claude/workflows/bdd-foundry.js or ~/.claude/workflows/bdd-foundry.js
  When the snapshot step runs
  Then docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/source-snapshot-<UTC-ts>.js exists
  And its SHA256 is recorded in docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/source-snapshot.sha256
  And every later content comparison (S2, E1, E2) runs against this snapshot file, never against the live ~/.claude/workflows/ path
  And after landing, `shasum -a 256 -c source-snapshot.sha256` still passes (the snapshot was never edited post-hoc)
```

### S10 — candidate-sweep-recorded-before-winner (happy)
```gherkin
Scenario: every candidate copy of bdd-foundry.js is enumerated before a winner is declared
  Given the run log docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/candidate-sweep.md
  When I read it
  Then it records the executed results of ALL of: `ls -la ~/.claude/workflows/ | grep bdd-foundry`, `git -C /Users/bo/dev/agentops ls-files '.claude/workflows/'`, a per-checkout probe of every path in `git -C /Users/bo/dev/agentops worktree list` for .claude/workflows/bdd-foundry.js, and `ls docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/*.js`
  And each candidate found is listed with its header version (`head -1`) and SHA256
  And exactly one candidate is marked WINNER, with the rule applied stated: highest lineage version wins (E1); same-version divergent bytes go to hand-merge (E2)
  And if only one candidate exists the log states "single v5 source, no reconciliation needed"
```

### S11 — clean-home-fixture-idempotent-portable (happy)
```gherkin
Scenario: the follow mechanism works in a clean HOME, twice, from a non-Bo repo path
  Given a fixture with HOME=$(mktemp -d) containing NO .claude/workflows directory
  And a copy of the repo at a fixture path that is not /Users/bo/dev/agentops
  When the named install/follow command from S6 is run once, and then a second time
  Then after the first run, $HOME/.claude/workflows/bdd-foundry.js exists (directory auto-created) and satisfies the S6 follow relation against the FIXTURE repo path
  And the second run exits 0 and leaves a byte-identical readlink/cmp result — no nested symlinks, no duplicate entries, no backup litter, no new dirty state in the fixture repo (`git status --porcelain` unchanged)
  And `grep -rn "/Users/bo" <the install + drift script file(s) added by this change>` returns 0 matches (paths come from $HOME / repo-root resolution or flags, never hardcoded)
```

### S12 — evidence-anchored-to-landed-head (happy)
```gherkin
Scenario: verification evidence is bound to the exact bytes that landed on main
  Given docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/landed-evidence.md written AFTER the last file change in the arc
  When I compare it against main
  Then it records: the landed HEAD SHA, `shasum -a 256` of .claude/workflows/bdd-foundry.js at that SHA, the readlink/cmp result for ~/.claude/workflows/bdd-foundry.js, and the exit code (0) of the gate command — each entry timestamped after the final edit
  And `git show <recorded-SHA>:.claude/workflows/bdd-foundry.js | shasum -a 256` equals the recorded file hash
  And the recorded SHA is an ancestor-or-equal of main (`git merge-base --is-ancestor <SHA> main` exits 0)
```

---

## Edge cases

### E1 — source-already-v6-takes-highest (edge)
```gherkin
Scenario: a newer lineage version at snapshot time wins over this prompt's v5 assumption
  Given at S9 snapshot time `head -1` of the S10 winner matches "bdd-foundry v(\d+)" with N ≥ 6
  When the canonical file is created
  Then the canonical file's header version equals the source's N (the highest), not 5
  And the S9 snapshot is taken of THAT source, and S2 compares against it
  And the S4 marker + enforcement greps are re-run against that content and still pass
```

### E2 — same-version-divergent-hand-merge-with-hunk-dispositions (edge)
```gherkin
Scenario: two claimants at the same lineage version are reconciled hunk-by-hunk, with every semantic hunk dispositioned
  Given a second file claiming "v5" in its header exists whose bytes differ from the S10 winner
  When reconciliation runs before the copy
  Then a saved diff of the two v5 claimants exists at docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/reconciliation.diff
  And a companion table in the same dir assigns EVERY non-comment hunk in that diff an explicit disposition: kept / superseded / rejected-with-reason — lineage-comment-only merges are NOT a valid reconciliation
  And the canonical file contains every distinct "// v" lineage line present in either claimant
  And the merged result passes `node --check` AND all S4 enforcement greps
  (If no second divergent copy exists at run time — the expected case — this scenario is vacuously satisfied and candidate-sweep.md states "single v5 source, no reconciliation needed")
```

### E3 — change-surface-disjoint-no-regen-tax (edge)
```gherkin
Scenario: the change touches only its own surface; the cockpit gate passes without regen fallout
  Given the work branch in its worktree, rebased on main
  When I run `git diff --name-only main...HEAD` and then `ao gate check --fast --scope head` (the pre-push gate)
  Then the changed paths are limited to: .claude/workflows/bdd-foundry.js, the install/freshness wiring file(s) from S6/S7, and docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/**
  And no path under skills/ or docs/contracts/ is modified
  And the gate exits 0 with no check demanding a skills/context-map regeneration
```

### E4 — worktree-isolation-unconditional (edge)
```gherkin
Scenario: ALL implementation edits happen in a worktree, whether or not the main checkout is dirty
  Given the repo contract (worktree-mandatory; agents do not edit the shared checkout)
  When this lane implements the change
  Then the run log names the worktree path (`git worktree add wt-<bead-id> -b <type>/<bead-id>-…`) and branch used for every implementation edit
  And `git -C /Users/bo/dev/agentops status --porcelain` after landing shows the SAME pre-existing dirty set captured at work start (no new modifications, no clobbered entries) apart from untracked plan-dir files this run wrote
  And a clean main checkout at work start does NOT license editing it directly — the worktree evidence is required either way
```

### E5 — installed-local-edits-backed-up-or-refused (edge)
```gherkin
Scenario: replacing the installed file never silently destroys local edits
  Given ~/.claude/workflows/bdd-foundry.js is a regular file whose bytes differ from the chosen source snapshot (e.g. uncaptured local v6 edits)
  When the install/follow step from S6 runs
  Then it either refuses with a non-zero exit and an actionable message naming the divergent path
  Or it first copies the existing file to a named backup path (~/.claude/workflows/bdd-foundry.js.pre-canonicalize-<ts> or equivalent, printed in its output) before replacing
  And in the backup branch, `cmp -s <backup> <pre-replacement bytes>` exits 0 (the backup is faithful)
```

### E6 — sibling-drift-scoped-blocking-report-only-elsewhere (edge)
```gherkin
Scenario: the drift check blocks on bdd-foundry.js only; known sibling drift is reported, not fixed and not blocking
  Given the live pre-existing drift on bead-crank.js (repo ≠ ~/.claude copy, verified 2026-06-12)
  When the S7 drift check runs in its blocking mode
  Then its blocking scope covers .claude/workflows/bdd-foundry.js; sibling workflow drift is emitted as a non-fatal report line, never an exit-code failure of THIS change's gate
  And no commit in this arc modifies bead-crank.js or operating-loop.js content
  And a follow-up bead for the sibling remediation exists (`BEADS_DIR=/Users/bo/dev/agentops/_beads br create …` from the main checkout), its id recorded in the plan-dir evidence — making any future global blocking mode someone else's gated arc
```

---

## Error cases

### X1 — syntax-failure-blocks-the-copy (error)
```gherkin
Scenario: a canonical candidate that fails node --check never gets committed
  Given a candidate .claude/workflows/bdd-foundry.js in the worktree where `node --check` exits non-zero
  When the verification step runs before commit
  Then the step aborts with the node error surfaced
  And `git log --oneline -- .claude/workflows/bdd-foundry.js` on the branch shows no commit containing the broken candidate
```

### X2 — missing-marker-blocks-the-push (error)
```gherkin
Scenario: losing any hardening marker or enforcement shape (the clobber this change exists to prevent) blocks landing
  Given a candidate canonical file where any one of the S4 greps — the four markers (DRIFT_SCHEMA / beads.json / DIR-MISAIM / "pre-run-N base snapshot") OR the three enforcement shapes (DIR-MISAIM throw window / tracker-write guard chain / gap_dispositions schema requirement) — returns fewer matches than its S4 floor
  When the pre-push verification runs
  Then verification exits non-zero naming the missing marker or enforcement shape
  And no push to main occurs (the commit is amended/fixed in the worktree first)
```

### X3 — no-live-run-and-no-law0-surface-anywhere (error)
```gherkin
Scenario: verification never executes the workflow, and no changed executable surface smuggles a LAW-0 call
  Given the verification commands recorded in the plan dir / landed evidence for this change
  And the full changed-file list from `git diff --name-only main...HEAD`
  When I inspect each recorded command AND grep every changed executable file (install/drift scripts, .claude/workflows/bdd-foundry.js) for `claude -p`, `claude --print`, and `gemini -p`
  Then every recorded command is one of: node --check, grep, diff/cmp, shasum, readlink, git, ls/sed/find, or a fixture run of the install/drift scripts (static inspection + sandboxed wiring only)
  And no recorded command invokes the Workflow tool, `claude` (in any form), or executes bdd-foundry.js
  And the LAW-0 grep over changed executable surfaces yields zero executable matches — any hit is inside a comment/string-literal documenting the prohibition, and each such exception is listed in the evidence with file:line
```

### X4 — dangling-or-misaimed-follow-fails (error)
```gherkin
Scenario: a broken follow mechanism is caught, not silently tolerated
  Given the installed path ~/.claude/workflows/bdd-foundry.js is a symlink whose target does not exist, OR a copy whose bytes differ from canonical (the bead-crank.js failure mode, live today)
  When the S7 check (or symlink-resolution check) runs
  Then it exits non-zero and names the offending path
  And the failure surfaces through the named blocking parent from S7 (not only when the check is run by hand)
```

### X5 — tracker-never-run-from-worktree (error)
```gherkin
Scenario: bead writes happen only from the main checkout with the exact BEADS_DIR invocation
  Given a shell whose cwd is the work worktree (where `git rev-parse --git-dir` ≠ `git rev-parse --git-common-dir`)
  When any tracker write for this change is needed
  Then the command is run from /Users/bo/dev/agentops (main checkout) as `BEADS_DIR=/Users/bo/dev/agentops/_beads br …`
  And no new `.beads/` or `_beads/` directory appears inside the worktree afterward (`test ! -e wt-<bead-id>/_beads`)
```

---

## Gap dispositions (cross-family review, all 15 — FROZEN)

| # | Gap (short) | Disposition | Where |
|---|---|---|---|
| 1 | No immutable pre-copy snapshot | **folded** | new S9; S2 rewritten to compare against the snapshot, never the live `~/.claude` path |
| 2 | Hand-merge can drop code, keep comments | **folded** | E2 now requires a per-hunk disposition table (kept/superseded/rejected-with-reason) for every non-comment hunk |
| 3 | No candidate-discovery sweep | **folded** | new S10: recorded sweep over home dir, repo, every `git worktree list` checkout, plan snapshots; winner named with the rule applied |
| 4 | Install path unproven; hand repair passes | **folded** | S6 requires a NAMED re-runnable command; S11 proves it in an isolated-HOME fixture |
| 5 | Drift check can exist unwired | **folded** | S7 (+X4) now requires a named blocking parent whose mutated-fixture run also exits non-zero |
| 6 | Local installed edits can be clobbered | **folded** | new E5: refuse-or-backup before replacement, backup byte-faithful |
| 7 | Clean-home install uncovered | **folded** | S11: temp HOME with no `.claude/workflows` — directory auto-created, follow relation holds |
| 8 | No idempotency requirement | **folded** | S11: second run exits 0, byte-identical result, no litter/dirt |
| 9 | Worktree isolation conditional on dirty main | **folded** | E4 made unconditional: worktree path+branch in the run log regardless of main-checkout cleanliness |
| 10 | Hardcoded /Users/bo paths can pass | **folded** | S11: fixture runs from a non-Bo repo path + `grep -rn "/Users/bo"` = 0 over the added scripts |
| 11 | Marker greps don't prove enforcement | **folded** | S4 + X2 gain enforcement-shape greps grounded on the live v5 source (DIR-MISAIM throw window, `cycleFree && uncovered.length === 0 && driftOk` ×2, `gap_dispositions` schema); verbatim path doubly covered by S2/S9 byte-equality |
| 12 | LAW 0 checked only on recorded commands | **folded** | X3 extended: grep every changed executable surface for `claude -p` / `claude --print` / `gemini -p`; comment-only exceptions listed file:line |
| 13 | Bead citation/closure partial | **folded** | S8 extended: EVERY arc commit (workflow file, wiring, plan dir) cites the bead; bead closed with a note naming the final landed SHA |
| 14 | Sibling (bead-crank) drift undispositioned | **folded** | new E6: blocking scope = bdd-foundry.js only; sibling drift report-only; remediation bead filed and recorded — the fix itself stays out of scope (its own bead, per Out-of-scope below) |
| 15 | Evidence not anchored to landed content | **folded** | new S12: landed-evidence.md records HEAD SHA + canonical SHA256 + follow result + gate exit, post-final-edit; hashes re-derivable from `git show` |

## Out of scope (named so nobody widens the arc)

- Fixing the pre-existing `bead-crank.js` repo↔user drift — E6 *reports* it and *files* the remediation bead; remediating content is that bead's arc, not this one.
- The fleet-wide workflow home decision (`mto-pyss` / MAP-REPOS G5) — this change canonicalizes within agentops per the existing house pattern; it does not decide dotfiles deployment.
- Any behavioral change to bdd-foundry.js itself beyond the single HAZARD-line swap.
