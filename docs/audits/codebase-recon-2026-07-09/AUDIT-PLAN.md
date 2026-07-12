# Fresh-Model Codebase Recon — Audit Plan

- Date: 2026-07-09
- Bead: `age-fresh-model-codebase-recon-toqd`
- Model family: GPT-5 / Codex runtime (exact deployment identifier is not exposed to the task)
- Pinned base: `fbba8af5ace635104775ef18f34fef362ba368ce`
- Prior comparison run: `docs/audits/codebase-recon-2026-07-02/`
- Scope: the full tracked AgentOps repository, excluding private `_beads/` state and uncommitted files in the shared checkout

## Acceptance contract

### Happy path

Given the pinned AgentOps snapshot and the four applicable `codebase-*` skills,
when one fresh subagent executes each skill and the lead independently checks the material claims,
then this directory contains four substantive source reports, a deduplicated synthesis, a delta from the prior run, and machine-readable verification evidence.

### Edge path

Given a worker claim that cannot be reproduced from the pinned snapshot,
when the lead verifies the claim,
then the claim is marked `refuted` or `unverified`, excluded from promoted recommendations, and retained in the verification ledger for auditability.

## Non-goals

- Implementing remediation or changing product behavior.
- Treating narrative documentation as higher authority than executable or generated surfaces.
- Incorporating the dirty shared checkout into the audit snapshot.
- Promoting a worker's self-report without independent evidence.

## Wave and ownership map

| Wave | Worker contract | Owned tracked artifact |
|---|---|---|
| 1 | `codebase-archaeology` | `codebase-archaeology.md` |
| 1 | `codebase-audit` | `codebase-audit.md` |
| 1 | `codebase-pattern-extraction` | `codebase-pattern-extraction.md` |
| 2 | `codebase-report` | `codebase-report.md` |
| Lead | Synthesis and independent verification | `README.md`, `SYNTHESIS.md`, `DIFF-vs-2026-07-02.md`, `VERIFICATION.md`, `findings.json` |

The manifests are disjoint. Workers may read any tracked file but may write only their owned report and an untracked thin status record under `.agents/swarm/results/`. Workers do not stage, commit, or push.

## Coordination and rollback

The canonical checkout was dirty, so the audit runs in the dedicated worktree `/Users/bo/dev/agentops-wt/age-fresh-model-codebase-recon-toqd`. Per the operator's runtime choice, all fan-out uses Codex-native subagents. The claimed bead plus disjoint file manifests provide the coordination boundary; no external swarm runtime is part of this run.

Rollback is docs-only: revert the audit commit and remove the dedicated branch/worktree. No runtime or product files are in scope.

## Evidence for done

- All four owned reports exist and cite concrete `file:line` evidence.
- Material risk findings have deterministic reproduction commands or direct source traces.
- `findings.json` records each promoted finding and its verification state.
- `DIFF-vs-2026-07-02.md` classifies prior findings as fixed, persistent, regressed, or not rechecked.
- The final repository gate result and exact commands are recorded in `VERIFICATION.md`.
