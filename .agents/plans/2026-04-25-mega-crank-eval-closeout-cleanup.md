---
id: plan-2026-04-25-mega-crank-eval-closeout-cleanup
type: plan
date: 2026-04-25
source: ".agents/rpi/next-work.jsonl#agentops-dv5"
---

# Plan: Mega Crank Eval Closeout Cleanup

## Context

This is the next one-session crank after the `agentops-dv5` evaluation-environment longhaul. Product eval gates are green, but the closeout found four remaining repo-health gaps: canonical root disposition, closure-integrity replay for closed eval child beads, direct security toolchain governance coverage, and copied Beads CLI reference links.

`bd ready --json` and `bd list --status open --json` both show exactly four open beads. All four are feasible in one focused session if the canonical root disposition is handled first and branch edits are kept to disjoint file groups.

Applied findings:
- `f-2026-04-25-001` - Put worktree disposition and closure replay before starting more RPI/evolve cycles.
- `f-2026-04-14-002` - Use durable committed proof packets for closed beads instead of transient seed paths.
- `f-2026-04-03-001` - Treat evidence-only closure packets as first-class closure-audit proof.

Tracking status:
- No new bd issues are needed. The executable set is `agentops-6xe`, `agentops-aeg`, `agentops-i6j`, and `agentops-f6g`.
- `agentops-i6j` intentionally verifies stale before implementation because stale citations are the defect under repair.

## Files to Modify

| File | Change |
|------|--------|
| `/home/boful/dev/personal/agentops/.agents/learnings/2026-04-19-orchestrator-compression-anti-pattern.md` | Canonical root disposition only: inspect, preserve, commit, or ask before changing. |
| `/home/boful/dev/personal/agentops/.agents/patterns/pre-tag-ci-validation.md` | Canonical root disposition only: inspect, preserve, commit, or ask before changing. |
| `/home/boful/dev/personal/agentops/wiki/` | Canonical root disposition only: untracked local tree; do not delete automatically. |
| `skills/post-mortem/scripts/closure-integrity-audit.sh` | Improve scoped proof extraction if close reasons/full issue text contain durable file/proof evidence. |
| `skills/post-mortem/scripts/write-evidence-only-closure.sh` | Reuse for durable proof packets; patch only if packet content is insufficient. |
| `.agents/releases/evidence-only-closures/agentops-dv5.3.json` | **NEW** - Durable closure proof packet if audit extraction alone cannot replay. |
| `.agents/releases/evidence-only-closures/agentops-dv5.4.json` | **NEW** - Durable closure proof packet if audit extraction alone cannot replay. |
| `.agents/releases/evidence-only-closures/agentops-dv5.5.json` | **NEW** - Durable closure proof packet if audit extraction alone cannot replay. |
| `tests/e2e/closure-integrity-grace.sh` | Extend only if the closure-audit script behavior changes. |
| `skills/beads/references/CLI_REFERENCE.md` | Rewrite or replace stale upstream-relative links. |
| `skills-codex/beads/references/CLI_REFERENCE.md` | Mirror the Beads reference link repair for Codex runtime artifacts. |
| `skills/beads/scripts/validate.sh` | Optional: add a direct stale-link guard if validators lack coverage. |
| `skills-codex/beads/scripts/validate.sh` | Optional: mirror direct stale-link guard if added for shared Beads skill. |
| `evals/agentops-core/beads-issue-tracking.json` | Optional: add a deterministic stale-link regression case. |
| `evals/agentops-core/security-toolchain-governance.json` | **NEW** - Direct public canary for toolchain gate semantics. |
| `evals/agentops-core/fixtures/security-toolchain-governance-smoke.sh` | **NEW** - Offline mock fixture for security toolchain cases. |
| `.agents/evals/baselines/agentops-core.security-toolchain-governance.baseline.json` | **NEW** - Promoted baseline after the suite passes. |
| `docs/CI-CD.md` | Correct stale `scripts/security-toolchain-validate.sh` heading if touched by `agentops-f6g`. |
| `.beads/issues.jsonl` | Auto-sync from bd status/close updates; serialize writes. |

## Boundaries

**Always:** Use bd for issue state, keep deterministic evals offline, force-add ignored `.agents/evals/baselines/*.baseline.json` and closure packets when they are durable evidence, keep shared and Codex Beads references in sync, and rerun closure/worktree disposition checks before closing the session.

**Ask First:** Before deleting, reverting, stashing, or silently committing canonical-root user-owned changes under `/home/boful/dev/personal/agentops`, especially the untracked `wiki/` tree.

**Never:** Revert user changes to the canonical root, vendor upstream Beads docs as symlinks, make live model/runtime tests blocking for this crank, or broaden the eval suite beyond the four ready beads.

## Baseline Audit

| Metric | Command | Result |
|--------|---------|--------|
| Ready beads | `bd ready --json` | 4 ready: `agentops-aeg`, `agentops-6xe`, `agentops-f6g`, `agentops-i6j`. |
| Open beads | `bd list --status open --json` | 4 open, same set as ready; no hidden open backlog. |
| In-progress beads | `bd list --status in_progress --json` | `[]`. |
| Current branch state | `git status --short --branch` | `## codex/eval-env-discovery...origin/codex/eval-env-discovery [ahead 79]`; no dirty files before this plan. |
| Canonical root state | `git -C /home/boful/dev/personal/agentops status --short --branch` | `main` dirty: two tracked `.agents` files plus untracked `wiki/`. |
| Bead citation freshness | `ao beads verify agentops-aeg; ao beads verify agentops-6xe; ao beads verify agentops-f6g; ao beads verify agentops-i6j` | First three have fresh citations; `agentops-i6j` has 6 stale citations, which are the target links. |
| Closure-integrity replay | `bash skills/post-mortem/scripts/closure-integrity-audit.sh --scope auto agentops-dv5` | 6 children checked, 3 pass, 3 fail: `agentops-dv5.3` timing miss, `agentops-dv5.4` parser miss, `agentops-dv5.5` parser miss. |
| Beads stale link mentions | `rg -n '(CONFIG\\.md|DAEMON\\.md|GIT_INTEGRATION\\.md|\\.\\./AGENTS\\.md|\\.\\./README\\.md|\\.\\./LABELS\\.md)' skills/beads/references/CLI_REFERENCE.md skills-codex/beads/references/CLI_REFERENCE.md` | 12 matches, 6 in shared Beads reference and 6 in Codex copy. |
| Security eval coverage count | `jq -r '.id' evals/agentops-core/*.json \| wc -l` | 54 public canary suites. |
| Security eval baseline count | `find .agents/evals/baselines -maxdepth 1 -type f -name '*.baseline.json' \| wc -l` | 54 promoted baselines. |
| Direct toolchain suite exists | `test -f evals/agentops-core/security-toolchain-governance.json; echo $?` | `1`; no direct suite yet. |
| Existing toolchain tests | `rg -n '^test_[a-zA-Z0-9_]+\\(\\)' tests/scripts/test-toolchain-validate.sh tests/scripts/test-security-gate.sh` | 8 direct `toolchain-validate.sh` tests and 5 `security-gate.sh` test functions; security-gate emits 6 PASS because one function has two assertions. |
| CI non-blocking policy | `sed -n '250,330p' .github/workflows/validate.yml` | `security-toolchain-gate` has `continue-on-error: true` and runs `./scripts/security-gate.sh --mode quick`. |
| CI docs drift | `sed -n '180,280p' docs/CI-CD.md` | Security docs say `scripts/security-gate.sh` delegates to `scripts/toolchain-validate.sh`, but heading still says nonexistent `scripts/security-toolchain-validate.sh`. |

## Implementation

### 1. `agentops-6xe` - Resolve canonical root disposition

Claim the bead first:

```bash
bd update agentops-6xe --status in_progress --json
```

In canonical root `/home/boful/dev/personal/agentops`:
- Inspect the two tracked `.agents` diffs with `git diff -- .agents/learnings/2026-04-19-orchestrator-compression-anti-pattern.md .agents/patterns/pre-tag-ci-validation.md`.
- Inspect the untracked `wiki/` tree with `find wiki -maxdepth 2 -type f | sort | sed -n '1,80p'`.
- If the diffs are clearly AgentOps-generated metadata from the current work, preserve them with an explicit root commit.
- If any content appears user-authored or unrelated, stop the root-disposition action and report the exact paths needing owner disposition. Do not delete, revert, or stash.

Acceptance:
- `git -C /home/boful/dev/personal/agentops status --short --branch` is clean, or the remaining dirtiness is explicitly preserved/accepted by the user.
- `bash scripts/check-worktree-disposition.sh` from the eval worktree passes or reports only an accepted preserved state.

Mechanical checks:
- `git -C /home/boful/dev/personal/agentops status --short --branch`
- `bash scripts/check-worktree-disposition.sh`

### 2. `agentops-aeg` - Repair eval epic closure replay

Claim the bead after root disposition is understood:

```bash
bd update agentops-aeg --status in_progress --json
```

Preferred fix path:
- Teach `skills/post-mortem/scripts/closure-integrity-audit.sh` to extract scoped/proof evidence from structured bd data or full issue text, not only the description subset, when close reasons contain validation evidence.
- Keep evidence-only packets as the strongest proof surface, matching `skills/post-mortem/references/closure-integrity-audit.md`.
- Backfill durable packets for `agentops-dv5.3`, `agentops-dv5.4`, and `agentops-dv5.5` only where the audit cannot mechanically infer file evidence from existing bead data.

Acceptance:
- `bash skills/post-mortem/scripts/closure-integrity-audit.sh --scope auto agentops-dv5` passes 6/6 with no failures.
- Any new packet under `.agents/releases/evidence-only-closures/` has the expected `target_id`, `evidence_mode`, and `repo_state` fields.
- Closure replay no longer depends on transient local seed files.

Mechanical checks:
- `bash skills/post-mortem/scripts/closure-integrity-audit.sh --scope auto agentops-dv5`
- `bash tests/e2e/closure-integrity-grace.sh`
- `jq -e '.target_id and .evidence_mode and .repo_state' .agents/releases/evidence-only-closures/agentops-dv5.3.json` if packet is created; repeat for `.4` and `.5`.

### 3. `agentops-i6j` - Repair copied Beads CLI reference links

Claim the bead:

```bash
bd update agentops-i6j --status in_progress --json
```

Repair strategy:
- Rewrite `CONFIG.md`, `DAEMON.md`, `GIT_INTEGRATION.md`, `../AGENTS.md`, `../LABELS.md`, and `../README.md` links to existing local reference files or plain text where the upstream target is not vendored.
- Apply the same content repair to `skills/beads/references/CLI_REFERENCE.md` and `skills-codex/beads/references/CLI_REFERENCE.md`.
- Add validator coverage only if existing Beads validators do not catch stale copied links after the rewrite.

Acceptance:
- `ao beads verify agentops-i6j` exits 0 with 0 stale citations.
- The stale-link `rg` command returns no matches.
- Shared and Codex Beads skill validators pass.

Mechanical checks:
- `ao beads verify agentops-i6j`
- `! rg -n '(CONFIG\\.md|DAEMON\\.md|GIT_INTEGRATION\\.md|\\.\\./AGENTS\\.md|\\.\\./README\\.md|\\.\\./LABELS\\.md)' skills/beads/references/CLI_REFERENCE.md skills-codex/beads/references/CLI_REFERENCE.md`
- `bash skills/beads/scripts/validate.sh && bash skills-codex/beads/scripts/validate.sh`
- `scripts/eval-agentops.sh --suite evals/agentops-core/beads-issue-tracking.json`

### 4. `agentops-f6g` - Add direct security toolchain governance eval

Claim the bead:

```bash
bd update agentops-f6g --status in_progress --json
```

Add a deterministic suite and fixture:
- `evals/agentops-core/security-toolchain-governance.json`
- `evals/agentops-core/fixtures/security-toolchain-governance-smoke.sh`

Suite cases should cover:
- `scripts/toolchain-validate.sh --gate --json` exit semantics for `PASS`, critical findings, and high findings.
- Separation between `security_high` and `quality_high` when the JSON summary distinguishes security vs quality scanner findings.
- `scripts/security-gate.sh --require-tools` returning exit 4 when mock tools are `not_installed` or `error`.
- Invalid toolchain JSON through `scripts/security-gate.sh --json` producing parse-error output and non-zero exit.
- Quick-mode skip contract for slow tools.
- CI policy that `security-toolchain-gate` remains non-blocking while still uploading artifacts.

Also fix `docs/CI-CD.md` if touched so the heading names `scripts/toolchain-validate.sh`, not the nonexistent `scripts/security-toolchain-validate.sh`.

Acceptance:
- The new suite runs and compares successfully.
- A promoted baseline exists at `.agents/evals/baselines/agentops-core.security-toolchain-governance.baseline.json`.
- Existing release/security suites continue to pass.

Mechanical checks:
- `bash tests/scripts/test-toolchain-validate.sh`
- `bash tests/scripts/test-security-gate.sh`
- `scripts/eval-agentops.sh --suite evals/agentops-core/security-toolchain-governance.json --promote-baseline --promoted-by codex --rationale "Initial direct security toolchain governance baseline."`
- `scripts/eval-agentops.sh --suite evals/agentops-core/security-toolchain-governance.json`
- `scripts/eval-agentops.sh --suite evals/agentops-core/release-security-gates.json`
- `scripts/eval-agentops.sh --suite evals/agentops-core/security-suite-behavioral-gates.json`

## Conformance Checks

| Issue | Check Type | Check |
|-------|------------|-------|
| `agentops-6xe` | command | `git -C /home/boful/dev/personal/agentops status --short --branch` |
| `agentops-6xe` | command | `bash scripts/check-worktree-disposition.sh` |
| `agentops-aeg` | command | `bash skills/post-mortem/scripts/closure-integrity-audit.sh --scope auto agentops-dv5` |
| `agentops-aeg` | tests | `bash tests/e2e/closure-integrity-grace.sh` |
| `agentops-aeg` | content_check | New closure packets, if any, validate with `jq -e '.target_id and .evidence_mode and .repo_state'`. |
| `agentops-i6j` | command | `ao beads verify agentops-i6j` |
| `agentops-i6j` | content_check | Stale-link `rg` returns no matches in shared/Codex Beads CLI references. |
| `agentops-i6j` | tests | `bash skills/beads/scripts/validate.sh && bash skills-codex/beads/scripts/validate.sh` |
| `agentops-i6j` | eval | `scripts/eval-agentops.sh --suite evals/agentops-core/beads-issue-tracking.json` |
| `agentops-f6g` | files_exist | `evals/agentops-core/security-toolchain-governance.json`, `evals/agentops-core/fixtures/security-toolchain-governance-smoke.sh`, `.agents/evals/baselines/agentops-core.security-toolchain-governance.baseline.json` |
| `agentops-f6g` | tests | `bash tests/scripts/test-toolchain-validate.sh && bash tests/scripts/test-security-gate.sh` |
| `agentops-f6g` | eval | `scripts/eval-agentops.sh --suite evals/agentops-core/security-toolchain-governance.json` |

## Verification

Targeted gates:

```bash
git -C /home/boful/dev/personal/agentops status --short --branch
bash scripts/check-worktree-disposition.sh
bash skills/post-mortem/scripts/closure-integrity-audit.sh --scope auto agentops-dv5
bash tests/e2e/closure-integrity-grace.sh
ao beads verify agentops-i6j
bash skills/beads/scripts/validate.sh && bash skills-codex/beads/scripts/validate.sh
bash tests/scripts/test-toolchain-validate.sh
bash tests/scripts/test-security-gate.sh
scripts/eval-agentops.sh --suite evals/agentops-core/beads-issue-tracking.json
scripts/eval-agentops.sh --suite evals/agentops-core/security-toolchain-governance.json
scripts/eval-agentops.sh --suite evals/agentops-core/release-security-gates.json
scripts/eval-agentops.sh --suite evals/agentops-core/security-suite-behavioral-gates.json
```

Closeout gates:

```bash
cd cli && env -u AGENTOPS_RPI_RUNTIME go test ./cmd/ao ./internal/eval
scripts/eval-agentops.sh --fast
bash tests/scripts/test-headless-runtime-skills.sh
bash scripts/validate-codex-rpi-contract.sh
bash scripts/validate-codex-lifecycle-guards.sh
bash scripts/audit-codex-parity.sh
git diff --check
scripts/pre-push-gate.sh --fast
```

## Issues

### Issue 1: `agentops-6xe` - Resolve canonical root knowledge metadata dirtiness

**Dependencies:** None
**Acceptance:** Canonical root disposition is clean or explicitly preserved; worktree disposition gate passes or has user-approved remaining state.
**Description:** Inspect and resolve root-owned `.agents` metadata and untracked `wiki/` without disturbing the eval worktree.

### Issue 2: `agentops-aeg` - Repair closure-integrity evidence for eval epic children

**Dependencies:** Issue 1 for final closeout, but branch implementation can proceed once root risk is understood.
**Acceptance:** Closure-integrity audit for `agentops-dv5` passes all six children; any evidence-only packets are durable and schema-valid.
**Description:** Improve closure evidence extraction and/or backfill durable proof packets for `agentops-dv5.3`, `.4`, and `.5`.

### Issue 3: `agentops-i6j` - Audit copied Beads CLI reference relative links

**Dependencies:** None.
**Acceptance:** Stale upstream-relative links are removed or replaced in shared and Codex Beads CLI references; `ao beads verify agentops-i6j` is clean.
**Description:** Repair copied upstream Beads links without vendoring unnecessary docs or adding symlinks.

### Issue 4: `agentops-f6g` - Add direct security toolchain governance eval

**Dependencies:** None, but run after Beads link repair if sharing final eval runtime.
**Acceptance:** Direct security-toolchain canary and baseline exist; mock fixture covers gate semantics, missing tools, invalid JSON, quick skips, and CI non-blocking policy.
**Description:** Add the missing eval layer for `scripts/toolchain-validate.sh` and security-gate wrapper behavior.

## Execution Order

**Wave 0 - Claim and preserve context:** Claim the four beads with `bd update <id> --status in_progress --json`, record current `git status`, and avoid broad edits until root disposition is understood.

**Wave 1 - Serial root gate:** Execute `agentops-6xe` first because it blocks `scripts/pre-push-gate.sh --fast` and final push readiness.

**Wave 2 - Parallel-safe branch repairs:** Execute `agentops-aeg` and `agentops-i6j`. They touch disjoint files, except `.beads/issues.jsonl` status export must remain serialized.

**Wave 3 - Eval expansion:** Execute `agentops-f6g` after the repository is cleaner, then promote the new baseline.

**Wave 4 - Full validation and closeout:** Run targeted checks, full fast eval, Codex runtime guards, pre-push gate, close the four beads with evidence, and push if implementing this plan in a full crank session.

## File Dependency Matrix

| Issue | Reads | Writes | Generated/Force-Add |
|-------|-------|--------|---------------------|
| `agentops-6xe` | Canonical root git state, canonical root `.agents` files, `wiki/` tree | Canonical root only, if owner disposition is clear | Possible root commit outside eval worktree |
| `agentops-aeg` | `bd show agentops-dv5.*`, closure audit scripts, closure audit reference | `skills/post-mortem/scripts/closure-integrity-audit.sh`, optional test updates | `.agents/releases/evidence-only-closures/agentops-dv5.{3,4,5}.json` |
| `agentops-i6j` | Shared/Codex Beads CLI references, Beads validators | `skills/beads/references/CLI_REFERENCE.md`, `skills-codex/beads/references/CLI_REFERENCE.md`, optional validator/eval case | None expected |
| `agentops-f6g` | `scripts/toolchain-validate.sh`, `scripts/security-gate.sh`, release/security eval suites, CI workflow | New security toolchain suite/fixture, optional `docs/CI-CD.md` correction | `.agents/evals/baselines/agentops-core.security-toolchain-governance.baseline.json` |

## File-Conflict Matrix

| Pair | Conflict Risk | Decision |
|------|---------------|----------|
| `agentops-6xe` vs all branch beads | Medium | Serial first. It operates in canonical root, not the eval worktree. |
| `agentops-aeg` vs `agentops-i6j` | Low | Parallel-safe by file path. Serialize bd status updates. |
| `agentops-aeg` vs `agentops-f6g` | Low | Parallel-safe by file path. Shared only through final eval runner. |
| `agentops-i6j` vs `agentops-f6g` | Low | Parallel-safe unless both choose to edit `evals/agentops-core/beads-issue-tracking.json`; avoid by keeping Beads fix validator-local unless needed. |
| Any issue vs `.beads/issues.jsonl` | Medium | Do not hand-edit; let bd auto-sync after serialized status/close commands. |

## Cross-Wave Shared File Registry

| Path | Owner | Rule |
|------|-------|------|
| `.beads/issues.jsonl` | bd auto-sync | Status and close updates only; no manual edits. |
| `.agents/evals/runs/` | eval runner | Generated run artifacts are evidence, not primary source; commit only intentional durable summaries/baselines. |
| `.agents/evals/baselines/` | `agentops-f6g` | Force-add the new security-toolchain baseline after promotion. |
| `.agents/releases/evidence-only-closures/` | `agentops-aeg` | Force-add only valid durable closure packets. |
| `/home/boful/dev/personal/agentops` | `agentops-6xe` | Treat as user/root-owned state; no destructive cleanup. |

## Planning Rules Compliance

| Rule | Status | Justification |
|------|--------|---------------|
| PR-001: Mechanical Enforcement | PASS | Every issue has at least one command/content check. |
| PR-002: External Validation | PASS | Uses bd verification, eval runner, shell tests, closure audit, and pre-push gate. |
| PR-003: Feedback Loops | PASS | Closure packets and new eval baseline turn post-mortem gaps into replayable artifacts. |
| PR-004: Separation Over Layering | PASS | Repairs target the missing proof/eval surfaces directly instead of adding broad wrappers. |
| PR-005: Process Gates First | PASS | Canonical root disposition and closure audit run before another RPI/evolve loop. |
| PR-006: Cross-Layer Consistency | PASS | Shared/Codex Beads refs are paired; CI docs and workflow policy are checked together. |
| PR-007: Phased Rollout | PASS | Root cleanup, replay repair, link hygiene, eval expansion, and full closeout are separated into waves. |

Unchecked rules: 0

## Next Steps

Run a pre-mortem or go directly into the next crank with this execution packet:

```bash
$agentops:crank .agents/plans/2026-04-25-mega-crank-eval-closeout-cleanup.md
```
