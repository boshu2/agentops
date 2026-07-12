# Codebase Recon — 2026-07-09

> **Historical pinned snapshot.** These reports describe commit
> `fbba8af5ace635104775ef18f34fef362ba368ce`; they are evidence, not current
> operating guidance. See [Resolution — 2026-07-12](RESOLUTION-2026-07-12.md)
> for the current disposition of F-04 through F-15.

Fresh-model, full-repository AgentOps audit at
`fbba8af5ace635104775ef18f34fef362ba368ce`, executed with Codex-native
subagents under bead `age-fresh-model-codebase-recon-toqd`.

## Start here

1. [SYNTHESIS.md](SYNTHESIS.md) — deduplicated verdict, strongest findings, and ranked actions.
2. [VERIFICATION.md](VERIFICATION.md) — independent reproduction evidence and gate results.
3. [DIFF-vs-2026-07-02.md](DIFF-vs-2026-07-02.md) — finding-by-finding movement since the previous recon.
4. [findings.json](findings.json) — machine-readable promoted findings and verification states.

## Source reports

Each report was produced by a fresh Codex-native subagent with one owned file.
The workers shared a pinned, isolated worktree but had disjoint write scopes and
did not stage, commit, or push.

| Skill | Durable report | Primary lens |
|---|---|---|
| `codebase-archaeology` | [codebase-archaeology.md](codebase-archaeology.md) | Executable architecture, flows, types, configuration, proof posture |
| `codebase-audit` | [codebase-audit.md](codebase-audit.md) | Security, correctness, reliability, CLI, and gate integrity |
| `codebase-pattern-extraction` | [codebase-pattern-extraction.md](codebase-pattern-extraction.md) | Repeated mechanisms, adoption gaps, extraction candidates |
| `codebase-report` | [codebase-report.md](codebase-report.md) | Reusable deep technical architecture report |

The acceptance contract, file-ownership map, non-goals, coordination fallback,
and rollback are frozen in [AUDIT-PLAN.md](AUDIT-PLAN.md).

## Repeat this when a new model ships

Use this directory as the run shape, not as an answer key:

1. Pin one clean commit and record the exact model/runtime identity available to the session.
2. Run the four `codebase-*` skills in fresh contexts; do not let workers read the current run's sibling reports before independent inspection.
3. Give every worker one disjoint report manifest and keep the lead as sole committer.
4. Compare with the previous recon only after each lane has formed its own conclusions.
5. Dispatch a fresh refuter against promoted risk findings; reproduce executable claims and record refutations.
6. Synthesize only verified findings, preserving adjusted/refuted claims in the evidence ledger.
7. Run the repository gate and record pre-existing versus audit-introduced failures separately.

This cadence is deliberately model-comparable: the repository snapshot, lenses,
artifact names, severity normalization, and verification bar remain stable while
the reviewing model changes.
