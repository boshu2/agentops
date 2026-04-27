---
id: post-mortem-2026-04-27-branch-audit-merge-cleanup
type: post-mortem
date: 2026-04-27
source: ".agents/plans/2026-04-27-land-eval-environment.md"
target: branch-audit-2026-04-27
verdict: WARN
---

# Post-Mortem: Branch Audit and Merge Cleanup

**Target:** Recent branch-disposition pass, centered on `ag-3lx` plus all open/stale remote branches.
**Duration reviewed:** 2026-04-27 branch audit through final pushed closeout commit `f7f687de`.
**Mode:** Inline Codex post-mortem; `ao council` is not a repo CLI command, so the retrospective council was performed from measured repo/GitHub evidence.

> RPI streak: N/A - `.agents/rpi/rpi-state.json` is absent.

## Council Verdict: WARN

| Judge | Verdict | Key Finding |
|---|---|---|
| Plan-Compliance | PASS | Every actionable branch was classified and disposed: merge-ready work merged, no-op PR closed, stale residue archive-tagged/deleted, and preserve refs left intact. |
| Tech-Debt | WARN | `agentops-eval-advisory` still has a GitHub-only failure profile despite local eval success; this is tracked as `ag-y7w`. Formal `bd children ag-3lx` linkage is also absent even though the named beads are closed and evidenced. |
| Learnings | PASS | The session reinforced three reusable patterns: branch triage buckets, stacked-PR patience, and separating PR merge from branch deletion when local worktrees may hold refs. |

## Branch Disposition

| Class | Branches | Result |
|---|---|---|
| Merged active PRs | `triage/2026-04-27`, `nightly/2026-04-27`, `codex/post-mortem-pr-167`, `feat/eval-foundation`, `feat/eval-cli-integration`, `feat/eval-canaries-closeout` | Merged via PRs #164, #165, #168, #169, #170, #171. |
| Closed no-op | `claude/review-fix-merge-prs-j1drJ` | PR #166 closed after rebase produced an empty/no-op diff. |
| Archived stale residue | `claude/amazing-curie-X4FPF`, `claude/review-fix-merge-prs-j1drJ`, `codex/codex-hooks-audit-20260425`, `codex/competitive-posture`, `codex/discovery-agents-control-plane-20260425`, `codex/discovery-go-cli-quality`, `codex/harvest-praxis`, `codex/postmortem-finding-generator-sidecars` | Archive tags pushed, then remote heads deleted. |
| Landed source branch | `codex/eval-env-discovery` | Work split/merged through PRs #169-#171; archived as `archive/codex-eval-env-discovery` at `8b47ba14`; remote head deleted. |
| Preserved refs | `codex/preserve-evolve-umbrella-dream-subcycle-20260412`, `codex/preserve-go-cli-quality-gc-bridge-20260424`, `codex/preserve-nightly-retrospective-20260426`, `codex/preserve-sessions-tool-noise-filter`, `codex/preserve-sessions-tool-noise-filter-compact` | Intentionally retained. |

Final remote heads after cleanup: `main` plus the five `codex/preserve-*` refs above. Final local worktree list had only `/Users/bo/dev/agentops [main]`.

## Plan Compliance

| Planned / Requested | Delivered | Delta |
|---|---|---|
| Investigate every branch and identify stale/current state. | All remote heads were classified into active PR, no-op, stale residue, landed source branch, or preserve ref. | Complete. |
| Merge complete work or finish the branch then merge. | Six PRs merged; PR #171 was finished by updating eval expectations and fixture isolation before merge. | Complete. |
| Clean stale branches. | Stale remote heads were archive-tagged and deleted; current remote branch list verified. | Complete. |
| Preserve legitimate unfinished work. | Only documented `codex/preserve-*` refs remain. | Complete. |
| Push closeout to remote. | `main` pushed through `f7f687de`; `git status --short --branch` is clean and up to date. | Complete. |

## Four-Surface Closure

| Surface | Verdict | Evidence |
|---|---|---|
| Code | PASS | Eval foundation, CLI integration, and canaries landed via merge commits `9c7262f2`, `2361d13a`, and `73cfb55f`; PR #171 closeout fixes were committed as `00ebd7ce`. |
| Documentation | PASS | Eval contracts/docs and closeout artifacts landed; doc-release and smoke gates passed during the landing. |
| Examples | PASS | 54 eval canaries and baselines landed under `evals/agentops-core/` and `.agents/evals/baselines/`. |
| Proof | WARN | Local gates passed and latest full Validate summary passed on `14153c56`, but warn-only `agentops-eval-advisory` remains red in GitHub and needs a dedicated follow-up. |

## Validation Evidence

- `bash scripts/eval-agentops.sh --fast`: PASS locally on PR #171 before merge, 54 suites, failures=0, warnings=0.
- `bash tests/docs/validate-doc-release.sh`: PASS, 1,575 links checked, 0 broken.
- `cd cli && make build && make test`: PASS before PR #171 merge.
- `bash scripts/validate-next-work-contract-parity.sh && bash tests/scripts/test-next-work-contract-parity.sh`: PASS after closeout source repair.
- `./tests/smoke-test.sh --verbose`: PASS after final metadata closeout.
- `bash scripts/validate-learning-coherence.sh`: PASS, 44 files checked, 0 failures, 0 warnings.
- GitHub Validate run `25018481578`: summary PASS on `14153c56`; `agentops-eval-advisory` and `security-toolchain-gate` remained red but are continue-on-error jobs.

## Closure Integrity

Mechanical audit command:

```bash
bash skills/post-mortem/scripts/closure-integrity-audit.sh --scope auto ag-3lx
```

Result: WARN/metadata gap. The script reported no discovered children because the six concrete beads are linked by dependencies and descriptions, not as formal `bd children ag-3lx` children.

Manual evidence check:

- `ag-l0w`, `ag-5p8`, `ag-aez`, `ag-664`, `ag-rnb`, and `ag-xsy` all resolve with `bd show --json`.
- Prereq beads have commit-backed evidence: `5893c525`, `972c674b`, `f342d053`.
- PR beads close against PR URLs and merge commits: #169 `9c7262f2`, #170 `2361d13a`, #171 `73cfb55f`.
- No generic/phantom titles were found among the named beads.

Follow-up implication: future branch-landing epics should either attach the PR/prereq beads as formal children or write an evidence-only closure packet so closure-integrity replay does not depend on prose.

## Learnings

- Branch audit should use explicit buckets: active PRs, no-op/superseded work, stale residue, landed source branch, and preserve refs. This was captured in `.agents/learnings/2026-04-27-branch-audit-triage-classes.md`.
- Stacked PRs should be allowed to rebase naturally after lower stack layers merge; this avoided unnecessary conflict work for PR #171.
- `gh pr merge --delete-branch` can partially fail if a local worktree holds the branch; merge and remote branch deletion should be separate steps when sibling worktrees are possible.

## Knowledge Lifecycle

Phase 3 scanned 18 unprocessed learning files since `.agents/ao/last-processed`; no deduplication or retirement was applied in this focused closeout. The closeout hooks decayed metadata on three existing knowledge files. Phase 4 added one next-work item for the GitHub-only eval advisory gap and updated the processing marker. Phase 5 archived 0 stale learnings.

## Follow-Up Items

| Bead | Priority | Reason |
|---|---|---|
| `ag-y7w` | P2 | GitHub-only `agentops-eval-advisory` failures need reproduction and repair. |
| `ag-bdd` | P3 | Eval schema should add `security` to the domain enum. |
| `ag-7x8` | P3 | Security-toolchain fixture PATH override needs fresh macOS verification. |

## Flywheel: Next Cycle

Based on this post-mortem, the highest-priority follow-up is:

> **Investigate GitHub-only agentops-eval-advisory failures** (bug, medium)
> Local eval passed, but GitHub Actions still reports advisory failures in the non-blocking eval job.

Ready to run:

```bash
$rpi "Investigate GitHub-only agentops-eval-advisory failures"
```

Or see all harvested items in `.agents/rpi/next-work.jsonl`.
