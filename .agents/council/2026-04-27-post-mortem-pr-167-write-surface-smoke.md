---
id: post-mortem-2026-04-27-pr-167-write-surface-smoke
type: post-mortem
date: 2026-04-27
source: "PR #167"
---

# Post-Mortem: PR #167 Write-Surface Smoke Scanner

> RPI streak: unavailable | Sessions: unavailable | Last verdict: unavailable.
> `.agents/rpi/rpi-state.json` was absent in the clean post-mortem worktree.

**Epic:** recent / bead `ag-0af`
**PR:** https://github.com/boshu2/agentops/pull/167
**Merge commit:** `1e48827008d9c834e26967049287d18be5be5f32`
**Fix commit:** `b11244371dd6bf1fe124fedc1503ca27cb1ab465`
**Duration:** 4m active post-merge retrospective
**Cycle-Time Trend:** faster than the prior structured post-mortem (`24m`) because the fix touched one file and had already passed remote CI.

## Summary

PR #167 fixed the failing `go-build` job on `main`. The failure was a parity gap:
`scripts/check-agents-write-surfaces.sh` recognized `.agents` write-surface
references expressed as `filepath.Join(cwd, ".agents", "<surface>")`, but the
Go smoke scanner in `cli/cmd/ao/agents_smoke_test.go` only recognized literal
`.agents/<surface>` strings.

The shipped change added a `filepath.Join(..., ".agents", "<surface>")` regex
to the Go scanner and added regression fixture coverage for the `packets`
surface. PR #167 merged on 2026-04-27 at 18:31:03Z.

Proof collected:

- GitHub `Validate` workflow run `25012259076` completed successfully.
- PR #167 has 32 successful `Validate` check runs, including `go-build`,
  `bats-tests`, `smoke-test`, `windows-smoke`, and `summary`.
- Clean-worktree rerun:
  `go test ./cmd/ao -run 'TestAgentsWriteSurfaces_EachAllowlistEntryHasProductionReference|TestScanProductionAgentsReferences_FindsKnownLiteral' -count=1 -v`.
- Clean-worktree rerun: `scripts/check-agents-write-surfaces.sh --json | jq -r '.status'` returned `ok`.
- Clean-worktree rerun: `bats tests/scripts/check-agents-write-surfaces.bats` passed 16 tests.

## Checkpoint Policy

| Check | Status | Detail |
|---|---|---|
| Chain loaded | SKIP | `.agents/ao/chain.jsonl` is absent in the clean post-mortem worktree, so this is a standalone post-mortem |
| Prior phases locked | SKIP | No ratchet chain rows were available to replay |
| No FAIL verdicts | SKIP | Chain replay unavailable; relied on bead `ag-0af`, PR #167, local proof, and green CI |
| Artifacts exist | PASS | PR #167, merge commit `1e488270`, fix commit `b1124437`, and changed file `cli/cmd/ao/agents_smoke_test.go` are present |
| Idempotency | PASS | No existing `source_epic:"ag-0af"` next-work batch was present |

## Council Verdict: PASS

| Judge | Verdict | Key Finding |
|---|---|---|
| Plan-Compliance | PASS | The bead asked to align the Go smoke scanner with the shell write-surface scanner; PR #167 did exactly that |
| Tech-Debt | WARN | The shell and Go gates still duplicate scanner syntax knowledge, so future syntax additions need paired coverage |
| Learnings | PASS | Contract validators that mirror each other need explicit syntax-parity fixtures, not only file-list parity checks |

### Implementation Assessment

The fix is correctly scoped. The only code change is in
`cli/cmd/ao/agents_smoke_test.go`, where the production reference scanner now
collects both literal `.agents/<name>` references and `filepath.Join` references.
The test fixture now includes `filepath.Join(cwd, ".agents", "packets", "promoted")`
and asserts that `packets` is discovered.

No production command behavior changed. No CLI docs, user docs, examples, or
embedded artifacts needed regeneration.

### Concerns

No blocking defect was found in the merged change. The residual risk is
maintenance drift: the shell gate and Go smoke test intentionally mirror each
other, but they do not share one implementation. The regression test covers the
observed drift form; future scanner syntax changes should update both gates in
the same slice.

## Plan Vs Delivered

Planned by bead `ag-0af`:

- validate the `main` CI failure in `go-build`
- confirm the cause was `agents_smoke_test.go` missing `filepath.Join` detection
- update the Go scanner to match the shell write-surface gate
- verify focused tests

Delivered:

- root-caused the latest `Validate` failure to the Go scanner's narrower pattern
- added `joinRe` for `filepath.Join(..., ".agents", "<surface>")`
- added fixture coverage for the `packets` surface
- reran focused Go, shell, and BATS validation
- pushed PR #167, verified full GitHub CI, and merged it into `main`

Scope adjustment:

- No docs or examples were changed because the behavior is internal CI coverage,
  not a user-facing command or contract wording change.

## Prediction Accuracy

| Prior Finding | Prediction | Result |
|---|---|---|
| `f-2026-04-14-001` | Production command refactors can miss paired tests | HIT avoided; the change modified a Go test file and added direct scanner fixture coverage |
| `f-2026-04-27-002` | Propagation-surface drift can hide when only one gate learns a new representation | HIT; the incident was exactly shell/Go scanner representation drift, and PR #167 repaired the missing Go side |

## Four-Surface Closure

| Surface | Result | Evidence |
|---|---|---|
| Code | PASS | `cli/cmd/ao/agents_smoke_test.go` now scans literal and `filepath.Join` `.agents` references |
| Documentation | PASS | No user-facing behavior or contract text changed; CI policy already requires `go-build` and write-surface validation |
| Examples | PASS | No CLI examples changed; the fixture example in the Go smoke test now covers the new syntax |
| Proof | PASS | Focused Go test, shell gate, BATS suite, and full GitHub `Validate` workflow all passed |

## Closure Integrity

| Check | Result | Details |
|---|---|---|
| Evidence precedence | PASS | Closure is commit-backed by `b1124437`, merge-backed by `1e488270`, and linked to closed bead `ag-0af` |
| Phantom bead detection | PASS | `ag-0af` has a specific title and description naming the failing scanner and validation scope |
| Orphaned children | SKIP | No child bead hierarchy was in scope |
| Multi-wave regression | PASS | Single-commit PR; no later wave removed earlier scanner coverage |
| Stretch goals | PASS | No stretch goals were closed |

## Metadata Verification

Mechanical checks:

- One changed file in `b1124437^..b1124437`; the file exists in the merge commit.
- No docs or Markdown links changed.
- No ASCII diagrams changed.
- PR #167 status is `MERGED`, not draft, with merge commit `1e488270`.

Metadata warnings:

- The canonical root worktree still has unrelated local `.agents` state changes.
  This post-mortem was generated in a clean linked worktree from `origin/main`
  to avoid mixing those changes into the artifact branch.

## Test Pyramid Assessment

| Scope | Planned | Actual | Gaps | Action |
|---|---|---|---|---|
| Go smoke scanner parity | L1 | Focused Go test now covers literal and `filepath.Join` scanner forms | none for the observed failure | keep |
| Shell write-surface gate | L2/script | `scripts/check-agents-write-surfaces.sh --json` returned `ok`; BATS suite passed 16 tests | duplicated scanner knowledge remains | next-work item 1 |
| Remote CI | CI | GitHub `Validate` workflow passed all blocking jobs and summary | none | keep |

## Learnings

### What Went Well

- The failing CI log pointed directly at missing `.agents` surfaces, and the shell
  gate already had the correct interpretation.
- A one-file fix plus a focused fixture made the failure mode reproducible and
  cheap to validate.

### What Was Hard

- The confusing part was not the `.agents` contract itself; it was that two
  validators enforced the same contract with different pattern coverage.

### Do Differently Next Time

- When a shell gate and Go smoke test intentionally mirror each other, update
  both syntax recognizers and add fixture coverage for the newly supported form
  in the same slice.

### Patterns to Reuse

- Reproduce CI-only scanner drift with a small fixture that names the exact
  representation form that was missed.

### Anti-Patterns to Avoid

- Treating parity comments like "mirrors the regex" as enough proof that two
  validators actually recognize the same syntax.

## Findings Registry

- Added reusable finding `f-2026-04-27-004`:
  duplicated contract scanners need paired syntax fixtures when either scanner
  learns a new representation.

## Maintenance Phases

| Phase | Result | Detail |
|---|---|---|
| Process Backlog | WARN | Existing backlog contains unprocessed learnings newer than `.agents/ao/last-processed`; this post-mortem only scored and promoted the new learning to avoid broad unrelated churn |
| Activate | PASS | New learning promoted to `MEMORY.md`; finding compiler rerun after registry update |
| Retire | SKIP | No stale learning retirement was attempted in this narrow post-mortem |
| Harvest | PASS | One low-severity next-work item appended to `.agents/rpi/next-work.jsonl` |

## Prior Findings Resolution Tracking

Before this post-mortem batch:

- Entries: 57
- Total items: 216
- Resolved: 174
- Unresolved: 42
- Resolution rate: 80.56%

## Next Work

| Priority | Item | Type | Severity | Rationale |
|---|---|---|---|---|
| 1 | Centralize or fixture-lock agents write-surface scanner parity | tech-debt | low | The incident is fixed, but the shell and Go validators still carry separate recognizers for the same contract |

## Knowledge Lifecycle Summary

- Learnings written: 1
- Findings written: 1
- MEMORY promotions: 1
- Next-work items harvested: 1
- Stale retirements: 0

## Flywheel: Next Cycle

Based on this post-mortem, the highest-priority follow-up is:

> **Centralize or fixture-lock agents write-surface scanner parity** (tech-debt, low)
> Reduce future drift between the shell write-surface gate and the Go smoke scanner by sharing scanner fixtures or centralizing the recognized syntax list.

Ready to run:

```bash
$rpi "Centralize or fixture-lock agents write-surface scanner parity"
```
