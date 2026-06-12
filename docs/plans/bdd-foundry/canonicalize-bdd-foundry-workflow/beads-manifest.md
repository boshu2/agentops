# Beads Manifest — canonicalize-bdd-foundry-workflow

> **Status: OPERATOR-VALIDATED 2026-06-12 — collapsed to ONE arc bead.** Operator
> re-validation (independent fresh-context review + suite re-run) made four calls:
> (1) **Tracker shape = ONE bead, not 23.** The suite's own evidence contract requires a
> singular `BEAD_ID`; the DAG below is arc-level ATDD (beads only cascade green together,
> single rollback semantic) — it is the EXECUTION CHECKLIST for the arc, not 23 tracker rows.
> (2) **C9.1/C9.2 applied pre-arc** (happy.bats S4 v7-tolerant grep; error.bats X3
> string-literal branch), suite re-run: 23 not-ok / 0 ok / 0 harness errors — the
> `suite-amendments` bead is DROPPED from the checklist (done before the arc, closing the
> self-grading hazard). (3) **X4 posture: keep the blocking parent per spec** (absent⇒SKIP
> keeps clean machines green); rollout duty: run the arg-scoped install on bushido the same
> day, else its stale copy blocks unrelated pushes there. (4) **Symlink write-through hazard
> accepted consciously**: post-canonicalization, hotfixing the live file = editing the repo
> canonical + committing; hotfix lanes must be told (it forces fixes into git — the point).
>
> Prior status for the record — **REPAIRED — needed operator re-validation.** Repair pass
> 2026-06-12 against the verdict (score 0.78, drift_guard=FAIL) + `validate-codex.md`:
> (1) the drift offender `gate-registration` is now one-invocation-per-test (the
> `go test ./internal/gates/...` clause that listed 32 tests is replaced by an
> existence-guarded run of exactly ONE named test); (2) the five thin beads
> (`precommit-syntax-gate`, `suite-amendments`, `gate-registration`,
> `evidence-artifacts`, `law0-exceptions`) gained **machinery inspection** acceptance
> that checks the amended test files / Go registration / evidence.env 12-key schema
> DIRECTLY, so they can no longer self-grade via the scenario filters they themselves
> amend. Coverage holes: 0 and cycle_free=true were already clean — DAG untouched.
> NOT yet in the tracker. Tracker writes happen at crank time, from the main checkout
> ONLY, as `BEADS_DIR=/Users/bo/dev/agentops/_beads br ...` — never from a worktree.

## Conductor's mechanical findings (verbatim)

- coverage holes: [none]
- cycle_free=true
- rejected=[]

All 23 keys below passed the admission gate. One bead per frozen scenario (S1–S12,
E1–E6, X1–X5). Acceptance tests are anchored `bats --filter` invocations against the
executed-red suite in `acceptance-tests/`; anchors (`'^S1 '`) prevent S1 matching
S10/S11/S12. Many tests assert landed end-state (the suite is arc-level ATDD): a
bead's test goes green once the bead AND its deps complete; final validation is the
full 23-green suite.

**Acceptance format rule (drift-guard contract):** each fenced `ACCEPTANCE` block
contains exactly ONE test-runner invocation enumerating exactly ONE test in list mode
(`bats --count` = 1, or `go test -list` naming 1 test). Beads whose work product is the
test/evidence machinery itself additionally carry a **Machinery inspection** block —
plain shell (grep/sed/cmp/for), never a test runner — that asserts the amended files /
registrations / schemas directly, so a bead cannot be closed by the very scenario filter
it amends.

---

## 1. prestate-worktree

**Title:** Capture pre-state.porcelain before any work and establish the mandatory implementation worktree

**Why:** E4 makes worktree isolation unconditional: `git status --porcelain` of the main checkout must be captured at work start (C7.1) so post-landing state can be diffed against it, and every implementation edit must happen in `wt-<bead-id>` on `feat/<bead-id>-<slug>` regardless of main-checkout cleanliness. This bead is the root of the DAG: pre-state MUST precede every other write.

**scenario_ref:** E4

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/edge.bats --filter '^E4 '
```

**Deps:** (none — DAG root)

---

## 2. candidate-sweep

**Title:** Execute and record the full candidate sweep in candidate-sweep.md, naming exactly one WINNER with the rule applied

**Why:** S10 requires every candidate copy of bdd-foundry.js enumerated BEFORE a winner is declared: home-dir ls, repo ls-files, a probe of every git-worktree path, plan-dir *.js — each with head -1 version + SHA256, exactly one WINNER (expected: the ~/.claude v7 file), and the rule line (highest lineage wins / single-source-no-reconciliation). Includes one verbatim `BEADS_DIR=/Users/bo/dev/agentops/_beads br` invocation record (X5 support).

**scenario_ref:** S10

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/happy.bats --filter '^S10 '
```

**Deps:** prestate-worktree

---

## 3. source-snapshot

**Title:** Take the immutable pre-write source snapshot of the S10 winner with pinned SHA256

**Why:** S9: `source-snapshot-<UTC-ts>.js` + `source-snapshot.sha256` (relative basename, `shasum -c` clean) saved BEFORE any write to canonical or installed paths; snapshot timestamp must precede the first commit touching the canonical file; every later content comparison (S2/E1/E2) runs against this file, never the live ~/.claude path. Never edited afterward.

**scenario_ref:** S9

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/happy.bats --filter '^S9 '
```

**Deps:** candidate-sweep

---

## 4. reconciliation-vacuous

**Title:** Disposition the E2 reconciliation branch: single-source rule line in the sweep, or full hunk-disposition table

**Why:** E2: if two same-version divergent claimants exist, save reconciliation.diff + a per-hunk kept/superseded/rejected-with-reason table and merge all lineage lines; in the expected single-source case the scenario is vacuously satisfied and candidate-sweep.md must state the 'single vN source, no reconciliation needed' rule line. This bead proves the branch taken is recorded, not skipped.

**scenario_ref:** E2

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/edge.bats --filter '^E2 '
```

**Deps:** candidate-sweep

---

## 5. canonical-file

**Title:** Create .claude/workflows/bdd-foundry.js in the worktree: byte-equal to the snapshot except the single HAZARD-line swap, committed citing the bead

**Why:** S2 (and C1): canonical content equals the immutable snapshot with exactly one changed hunk — the '// HAZARD: not git-tracked' line replaced in place (1 removed / 1 added) by the CANONICAL header line; first-line version equals the snapshot's; all lineage lines (`grep -c '^// v[2345]' >= 4`) preserved. Pre-commit verification: `node --check`, marker script green, S5 meta greps empty.

**scenario_ref:** S2

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/happy.bats --filter '^S2 '
```

**Deps:** prestate-worktree, source-snapshot

---

## 6. lineage-highest

**Title:** Honor the highest-lineage winner: canonical header version equals the v7 source's N, snapshot taken of THAT source, S4 greps re-grounded and green

**Why:** E1: a newer lineage version at snapshot time (v7, per spec ground fact 1) wins over the prompt's v5 assumption — the canonical file's header version must equal the source's N, the S9 snapshot must be of that source, and the S4 marker + enforcement greps must be re-run against that content and still pass.

**scenario_ref:** E1

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/edge.bats --filter '^E1 '
```

**Deps:** source-snapshot, canonical-file

---

## 7. precommit-syntax-gate

**Title:** Enforce node --check before every canonical commit: no broken candidate ever enters branch history

**Why:** X1: a canonical candidate failing `node --check` never gets committed — the verification step aborts with the node error surfaced, and `git log` on the branch for .claude/workflows/bdd-foundry.js shows no commit containing a broken candidate. This is the pre-commit discipline wrapped around the canonical-file work.

**scenario_ref:** X1

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/error.bats --filter '^X1 '
```

**Machinery inspection (direct, non-test commands — the gate must REJECT, not just landed-state parse):**
```bash
# 1. Negative fixture: the verification command rejects a syntax-broken candidate and surfaces the node error.
t="$(mktemp -d)"; printf 'export const broken = {\n' > "$t/broken.js"
if node --check "$t/broken.js" 2>"$t/err"; then echo 'FAIL: gate accepted a syntax-broken candidate'; rm -rf "$t"; exit 1; fi
grep -q 'SyntaxError' "$t/err" || { echo 'FAIL: node error not surfaced'; rm -rf "$t"; exit 1; }
rm -rf "$t"
# 2. No commit in branch history for the canonical file carries a broken candidate.
t="$(mktemp -d)"
for sha in $(git -C /Users/bo/dev/agentops log --format=%H -- .claude/workflows/bdd-foundry.js); do
  git -C /Users/bo/dev/agentops show "$sha:.claude/workflows/bdd-foundry.js" > "$t/c.js"
  node --check "$t/c.js" || { echo "FAIL: broken candidate committed at $sha"; rm -rf "$t"; exit 1; }
done
rm -rf "$t"
```

**Deps:** canonical-file

---

## 8. meta-pure-literal

**Title:** Verify the exported meta block is a pure object literal (no interpolation, no runtime identifiers)

**Why:** S5: the 'export const meta = {...}' block must contain no '${' interpolation and reference no bare runtime identifier (args/TRACKER/DIR/RUN_TAG) so the Workflow tool can discover it statically. v7 already passes on content (spec ground fact 4) — this bead is the verification that the landed canonical preserves it.

**scenario_ref:** S5

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/happy.bats --filter '^S5 '
```

**Deps:** canonical-file

---

## 9. suite-amendments

**Title:** Apply the two C9 derived-test amendments: v7-tolerant S4 throw-window grep and X3 string-literal exception clause

**Why:** Spec ground facts 2+3 make two suite assertions unsatisfiable against any compliant canonical file: S4's `includes('DIR-MISAIM')` grep (v7 re-keyed to startsWith) and X3's comment-only LAW-0 exception (the REGISTER template literal quotes `claude -p` as a string literal, which frozen X3 explicitly permits). Amend happy.bats (`grep -A6 -E "includes\('DIR-MISAIM'\)|startsWith\('DIR-MISAIM'\)"`) and error.bats (accept comment-prefixed OR quoted/template-string hits, file:line listing still mandatory), each with a one-line provenance comment citing E1 / frozen-X3. No other test file changes; behaviors.md untouched.

**scenario_ref:** S4

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/happy.bats --filter '^S4 '
```

**Machinery inspection (direct, non-test commands — this bead MUTATES the suite, so the
amendments themselves are inspected; rerunning S4 alone would go green the moment the
test is edited, correct or not):**
```bash
pd=/Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests
# C9.1 — happy.bats S4 carries the v7-tolerant throw-window grep (alternation + -A6 window)…
grep -qF "startsWith('DIR-MISAIM')" "$pd/happy.bats" || { echo 'FAIL: v7-tolerant startsWith alternation missing from happy.bats'; exit 1; }
grep -F "startsWith('DIR-MISAIM')" "$pd/happy.bats" | grep -qF "includes('DIR-MISAIM')" || { echo 'FAIL: amendment must be an alternation, not a replacement (E1 floor unchanged)'; exit 1; }
grep -F "startsWith('DIR-MISAIM')" "$pd/happy.bats" | grep -qF -- '-A6' || { echo 'FAIL: -A6 throw window missing'; exit 1; }
# …and the unamended v5-only form (the unsatisfiable grep at frozen happy.bats:54) is GONE.
! grep -qF "grep -A2 \"includes('DIR-MISAIM')\"" "$pd/happy.bats" || { echo 'FAIL: stale includes-only -A2 grep still present'; exit 1; }
grep -qE '#.*E1' "$pd/happy.bats" || { echo 'FAIL: happy.bats provenance comment citing E1 missing'; exit 1; }
# C9.2 — error.bats X3 keeps the comment branch, ADDS the string-literal branch, keeps file:line mandatory.
grep -qE '\(#\|//\|\\\*\)' "$pd/error.bats" || { echo 'FAIL: comment-prefix acceptance branch lost'; exit 1; }
grep -qiE 'string.literal|quoted|template' "$pd/error.bats" || { echo 'FAIL: string-literal acceptance branch (frozen-X3 clause) missing from error.bats'; exit 1; }
grep -qF 'not listed file:line in landed-evidence.md' "$pd/error.bats" || { echo 'FAIL: file:line-in-evidence mandate lost'; exit 1; }
grep -q 'frozen-X3' "$pd/error.bats" || { echo 'FAIL: error.bats provenance comment citing frozen-X3 missing'; exit 1; }
# No other test file changes: edge.bats and helpers.bash carry no amendment provenance markers.
! grep -qE 'frozen-X3|v7-tolerant' "$pd/edge.bats" "$pd/helpers.bash" || { echo 'FAIL: amendment leaked outside happy.bats/error.bats'; exit 1; }
```

**Deps:** canonical-file

---

## 10. install-script

**Title:** Write scripts/install-workflows.sh: generic, idempotent, $HOME/cwd-root resolved, clean-HOME proven

**Why:** S11 (and C2): the NAMED re-runnable follow command. No-arg installs every .claude/workflows/*.js (no bdd-foundry special case); repo root from `git rev-parse`, dest `$HOME/.claude/workflows` (mkdir -p); `ln -sfn` semantics with byte-equal-replace; zero /Users/bo literals; touches only $HOME, never the repo; second run exits 0 byte-identical with no litter. Proven in a mktemp-HOME fixture from a non-Bo repo path.

**scenario_ref:** S11

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/happy.bats --filter '^S11 '
```

**Deps:** canonical-file

---

## 11. install-backup

**Title:** Implement the E5 divergent-install branch: byte-faithful backup before replace, backup path printed

**Why:** E5: replacing the installed file never silently destroys local edits — when ~/.claude/workflows/bdd-foundry.js is a regular file with different bytes, install-workflows.sh first `cp -p`'s it to `bdd-foundry.js.pre-canonicalize-<UTC-ts>`, prints the backup path, then installs the symlink; `cmp -s` between backup and pre-replacement bytes exits 0.

**scenario_ref:** E5

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/edge.bats --filter '^E5 '
```

**Deps:** install-script

---

## 12. drift-check-script

**Title:** Write scripts/check-workflow-drift.sh: blocking on bdd-foundry.js, DRIFT-REPORT-only for siblings, and file the sibling-drift remediation bead

**Why:** E6 (and C3): blocking set = bdd-foundry.js only (absent => SKIP exit 0; dangling symlink / misresolved / divergent copy => exit 1 naming the path); every other repo-tracked workflow (bead-crank.js, operating-loop.js) is report-only — 'DRIFT-REPORT: <name>' on stdout, never the exit code. At crank time, file the sibling user-copy drift remediation bead from the main checkout (`BEADS_DIR=/Users/bo/dev/agentops/_beads br create ...`) and record its id as SIBLING_DRIFT_BEAD_ID; no commit in this arc touches sibling content.

**scenario_ref:** E6

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/edge.bats --filter '^E6 '
```

**Deps:** canonical-file, install-script

---

## 13. marker-check-script

**Title:** Write scripts/check-bdd-foundry-markers.sh: every S4 floor checked, first violation fails non-zero naming the missing marker

**Why:** X2 (and C4): losing any hardening marker or enforcement shape is the clobber this change exists to prevent. The script takes the candidate as $1 (default: repo canonical) and enforces all S4 floors — `node --check`, DRIFT_SCHEMA>=2, beads.json>=3, DIR-MISAIM>=2, pre-run-N base snapshot>=1, the v5/v7-tolerant DIR-MISAIM throw window>=1, guard chain 'cycleFree && uncovered.length === 0 && driftOk'>=2, gap_dispositions>=2 — failing non-zero and naming the missing marker. node missing => loud FAIL (admission gates fail closed).

**scenario_ref:** X2

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/error.bats --filter '^X2 '
```

**Deps:** canonical-file

---

## 14. blocking-parent

**Title:** Write scripts/validate-workflow-install.sh: the named blocking parent that runs drift check + marker check and propagates failure

**Why:** S7 (and C5): the freshness check must be invoked by a named blocking surface, not orphaned. The thin parent runs check-workflow-drift.sh then check-bdd-foundry-markers.sh (argless, repo canonical), exits non-zero if either fails, propagating output. Against the mutated temp-pair fixture both the check and the parent exit non-zero naming bdd-foundry.js; against the real un-mutated pair both exit 0. Recorded as BLOCKING_PARENT_CMD.

**scenario_ref:** S7

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/happy.bats --filter '^S7 '
```

**Deps:** drift-check-script, marker-check-script

---

## 15. gate-registration

**Title:** Register workflow.install-drift in the Go gate registry (always-run, blocking, Fast|Full) with the paired L1 registration test

**Why:** X4 (and C6): a broken follow mechanism must surface through the named blocking parent automatically, not only when run by hand. New cli/internal/gates/checks/workflow_install.go registers {ID: workflow.install-drift, Tiers: Fast|Full, Blocking: true, Backing: validate-workflow-install.sh} with NO Match globs (the fixture mutation lives in $HOME, invisible to changed-file routing); paired workflow_install_test.go asserts the exact registration via the SINGLE named test `TestWorkflowInstallDriftRegistration` (pinned here so acceptance and drift guard agree). Dangling-symlink and divergent-copy failures exit non-zero naming the offending path through the parent.

**scenario_ref:** X4

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/error.bats --filter '^X4 '
```
```bash
# Exactly ONE Go test, existence-guarded (a -run anchor on a missing test would pass vacuously).
cd /Users/bo/dev/agentops/cli
[ "$(go test -list '^TestWorkflowInstallDriftRegistration$' ./internal/gates/checks/ | grep -cx 'TestWorkflowInstallDriftRegistration')" -eq 1 ] || { echo 'FAIL: TestWorkflowInstallDriftRegistration not defined'; exit 1; }
go test -run '^TestWorkflowInstallDriftRegistration$' -count=1 ./internal/gates/checks/
```

**Machinery inspection (direct, non-test commands — the registration source itself, so a
missing registration cannot ride a green package suite):**
```bash
f=/Users/bo/dev/agentops/cli/internal/gates/checks/workflow_install.go
test -f "$f" || { echo 'FAIL: workflow_install.go missing'; exit 1; }
grep -qF '"workflow.install-drift"' "$f" || { echo 'FAIL: check ID not registered'; exit 1; }
grep -qE 'Blocking:[[:space:]]*true' "$f" || { echo 'FAIL: not blocking'; exit 1; }
grep -qF 'validate-workflow-install.sh' "$f" || { echo 'FAIL: backing script not wired'; exit 1; }
grep -q 'Fast' "$f" && grep -q 'Full' "$f" || { echo 'FAIL: Fast|Full tiers missing'; exit 1; }
! grep -qE 'Match[A-Za-z]*:' "$f" || { echo 'FAIL: Match globs present — must be always-run ($HOME mutations are invisible to changed-file routing)'; exit 1; }
```

**Deps:** blocking-parent

---

## 16. land-main

**Title:** Land the whole arc on main through the cockpit gate: canonical file git-tracked at the house path, siblings untouched

**Why:** S1: `git ls-files .claude/workflows/bdd-foundry.js` outputs exactly that path on main, with bead-crank.js and operating-loop.js still listed untouched. This bead is the landing move: rebase the worktree branch on main, run `ao gate check --fast --scope head` green, then the conductor/operator pushes (rebase-on-reject). Every commit in ARC_BASE_SHA..LANDED_SHA cites the arc bead id.

**scenario_ref:** S1

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/happy.bats --filter '^S1 '
```

**Deps:** canonical-file, lineage-highest, precommit-syntax-gate, meta-pure-literal, suite-amendments, install-script, install-backup, drift-check-script, marker-check-script, blocking-parent, gate-registration, reconciliation-vacuous

---

## 17. gate-surface-clean

**Title:** Prove the change surface is disjoint: only canonical + wiring + plan dir changed, gate green with no regen fallout

**Why:** E3: `git diff --name-only main...HEAD` is limited to .claude/workflows/bdd-foundry.js, the install/freshness wiring files, and the plan dir; nothing under skills/ or docs/contracts/ is modified; `ao gate check --fast --scope head` exits 0 with no check demanding a skills/context-map regeneration.

**scenario_ref:** E3

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/edge.bats --filter '^E3 '
```

**Deps:** land-main

---

## 18. apply-real-home

**Title:** Apply C10 against the real HOME: run INSTALL_CMD once (backing up the live v7 user file), then drift check and blocking parent both exit 0

**Why:** S6: ~/.claude/workflows/bdd-foundry.js must follow the canonical file by the NAMED re-runnable mechanism — readlink resolves to /Users/bo/dev/agentops/.claude/workflows/bdd-foundry.js (symlink decision) — produced by 'bash scripts/install-workflows.sh bdd-foundry.js', the SAME pattern the repo's other workflows use (no special case). Run from the main checkout after landing; the pre-replace backup double-holds the S9 snapshot bytes.

**scenario_ref:** S6

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/happy.bats --filter '^S6 '
```

**Deps:** land-main

---

## 19. hazard-line-replaced

**Title:** Verify the HAZARD line is retired everywhere and exactly one CANONICAL header line names the new home

**Why:** S3: `grep -c 'HAZARD: not git-tracked'` is 0 in the canonical file AND in the installed ~/.claude copy (the copy follows), and exactly one header comment line satisfies all four S3 greps — starts //, names .claude/workflows/bdd-foundry.js, says canonical (case-insensitive), and references ~/.claude/workflows with the install pattern. Verifiable only after the real-HOME apply.

**scenario_ref:** S3

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/happy.bats --filter '^S3 '
```

**Deps:** canonical-file, apply-real-home

---

## 20. evidence-artifacts

**Title:** Write evidence.env (all 12 required keys) and landed-evidence.md anchored to the landed HEAD

**Why:** S12: landed-evidence.md is written AFTER the last file change and records the landed HEAD SHA, `shasum -a 256` of the canonical file at that SHA (re-derivable via `git show`), the readlink/cmp follow result, and the gate command with exit code 0 — and the recorded SHA is ancestor-or-equal of main. evidence.env carries the exact C7.4 key set (BEAD_ID, SIBLING_DRIFT_BEAD_ID, ARC_BASE_SHA, LANDED_SHA, WORKTREE_PATH, WORK_BRANCH, the four CMD strings, ADDED_SCRIPTS, WIRING_FILES) that the whole suite sources.

**scenario_ref:** S12

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/happy.bats --filter '^S12 '
```

**Machinery inspection (direct, non-test commands — S12 only requires LANDED_SHA; the full
12-key C7.4 schema is enforced HERE, not via incidental downstream consumers):**
```bash
env_file=/Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/evidence.env
test -f "$env_file" || { echo 'FAIL: evidence.env missing'; exit 1; }
for k in BEAD_ID SIBLING_DRIFT_BEAD_ID ARC_BASE_SHA LANDED_SHA WORKTREE_PATH WORK_BRANCH \
         INSTALL_CMD DRIFT_CHECK_CMD BLOCKING_PARENT_CMD MARKER_CHECK_CMD ADDED_SCRIPTS WIRING_FILES; do
  grep -qE "^${k}=.+" "$env_file" || { echo "FAIL: missing/empty key $k"; exit 1; }
done
[ "$(grep -cE '^[A-Z_]+=' "$env_file")" -eq 12 ] || { echo 'FAIL: evidence.env must carry exactly the 12 C7.4 keys'; exit 1; }
# Spot-check anchored values, not just presence.
grep -qE '^LANDED_SHA=[0-9a-f]{7,40}$' "$env_file" || { echo 'FAIL: LANDED_SHA not a sha'; exit 1; }
grep -qE '^WORKTREE_PATH=/Users/bo/dev/agentops/wt-' "$env_file" || { echo 'FAIL: WORKTREE_PATH not the mandated wt-<bead-id> shape'; exit 1; }
git -C /Users/bo/dev/agentops merge-base --is-ancestor "$(grep '^LANDED_SHA=' "$env_file" | cut -d= -f2)" main || { echo 'FAIL: LANDED_SHA not ancestor-or-equal of main'; exit 1; }
```

**Deps:** land-main, apply-real-home

---

## 21. law0-exceptions

**Title:** Prove no live run and no LAW-0 surface: recorded commands static-only, LAW-0 grep clean, the REGISTER string-literal exception listed file:line

**Why:** X3: every recorded verification command is node --check / grep / diff / cmp / shasum / readlink / git / ls / sed / fixture script runs — never the Workflow tool, claude in any form, or executing bdd-foundry.js; the LAW-0 grep over changed executable surfaces yields zero executable matches, and the single v7 REGISTER template-literal hit (which documents the prohibition) is listed in landed-evidence.md as .claude/workflows/bdd-foundry.js:<line>, recomputed in the landed file. Added scripts carry no LAW-0 strings at all.

**scenario_ref:** X3

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/error.bats --filter '^X3 '
```

**Machinery inspection (direct, non-test commands — matches FROZEN X3, independent of the
amended error.bats this bead's dep just rewrote):**
```bash
le=/Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/landed-evidence.md
canon=/Users/bo/dev/agentops/.claude/workflows/bdd-foundry.js
# The REGISTER string-literal exception is listed file:line — and RECOMPUTED in the landed file.
ln=$(grep -oE '\.claude/workflows/bdd-foundry\.js:[0-9]+' "$le" | head -1 | cut -d: -f2)
[ -n "$ln" ] || { echo 'FAIL: no .claude/workflows/bdd-foundry.js:<line> exception listed in landed-evidence.md'; exit 1; }
line=$(sed -n "${ln}p" "$canon")
printf '%s' "$line" | grep -qE 'claude (-p|--print)' || { echo "FAIL: listed line $ln does not hold the LAW-0 string in the landed file"; exit 1; }
# Frozen-X3 proxy: the hit must be comment-prefixed OR sit inside a quoted/template string — never executable.
printf '%s' "$line" | grep -qE '^[[:space:]]*(#|//|\*)' || printf '%s' "$line" | grep -qE "[\`'\"][^\`'\"]*claude (-p|--print)" || { echo "FAIL: line $ln is executable, not comment/string-literal"; exit 1; }
# Added scripts carry NO LAW-0 strings at all.
! grep -nE 'claude -p|claude --print|gemini -p' \
  /Users/bo/dev/agentops/scripts/install-workflows.sh \
  /Users/bo/dev/agentops/scripts/check-workflow-drift.sh \
  /Users/bo/dev/agentops/scripts/check-bdd-foundry-markers.sh \
  /Users/bo/dev/agentops/scripts/validate-workflow-install.sh || { echo 'FAIL: LAW-0 string in an added script'; exit 1; }
# Recorded verification commands are static-only: no execution of the workflow without --check, no Workflow-tool run recorded.
! { grep -E 'node[^-]*bdd-foundry\.js' "$le" | grep -v -- '--check' | grep -q .; } || { echo 'FAIL: live bdd-foundry.js execution recorded'; exit 1; }
```

**Deps:** evidence-artifacts, suite-amendments

---

## 22. bead-discipline

**Title:** Close the arc bead with the landed SHA: every arc commit cites the bead, ledger stays private

**Why:** S8: the bead id matches ^ag-, EVERY commit touching the canonical file, wiring files, or plan dir in ARC_BASE_SHA..LANDED_SHA contains it, the bead is closed (from the main checkout, `BEADS_DIR=/Users/bo/dev/agentops/_beads br`) with a close note naming the landed SHA (>= 7 chars), and `git log --all -- _beads/` in the PUBLIC repo shows no new commit.

**scenario_ref:** S8

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/happy.bats --filter '^S8 '
```

**Deps:** land-main, evidence-artifacts

---

## 23. tracker-main-only

**Title:** Prove tracker writes never ran from the worktree and every invocation is in the exact BEADS_DIR form

**Why:** X5: all bead writes happen from /Users/bo/dev/agentops (where git-dir == git-common-dir) as `BEADS_DIR=/Users/bo/dev/agentops/_beads br ...`; no .beads/ or _beads/ directory appears inside the worktree (`test ! -e wt-<bead-id>/_beads`); every tracker invocation is recorded verbatim in the plan-dir evidence.

**scenario_ref:** X5

**ACCEPTANCE:**
```bash
bats /Users/bo/dev/agentops/docs/plans/bdd-foundry/canonicalize-bdd-foundry-workflow/acceptance-tests/error.bats --filter '^X5 '
```

**Deps:** evidence-artifacts

---

## Dependency graph (by key)

```
prestate-worktree (root)
└─ candidate-sweep
   ├─ source-snapshot
   │  └─ canonical-file (also deps: prestate-worktree)
   │     ├─ lineage-highest (also deps: source-snapshot)
   │     ├─ precommit-syntax-gate
   │     ├─ meta-pure-literal
   │     ├─ suite-amendments
   │     ├─ install-script
   │     │  ├─ install-backup
   │     │  └─ drift-check-script (also deps: canonical-file)
   │     ├─ marker-check-script
   │     │  └─ blocking-parent (also deps: drift-check-script)
   │     │     └─ gate-registration
   │     └─ land-main (deps: all 12 implementation beads)
   │        ├─ gate-surface-clean
   │        ├─ apply-real-home
   │        │  ├─ hazard-line-replaced (also deps: canonical-file)
   │        │  └─ evidence-artifacts (also deps: land-main)
   │        │     ├─ law0-exceptions (also deps: suite-amendments)
   │        │     ├─ bead-discipline (also deps: land-main)
   │        │     └─ tracker-main-only
   └─ reconciliation-vacuous → land-main
```

cycle_free=true (conductor-verified). 23 beads / 23 frozen scenarios — coverage holes: [none]. rejected=[].
