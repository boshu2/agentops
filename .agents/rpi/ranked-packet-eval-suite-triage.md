---
id: ranked-packet-2026-04-29-eval-suite-triage
type: ranked-packet
date: 2026-04-29
---

# Ranked Packet: Eval Suite Triage

## Objective

Classify what the full AgentOps eval suite really measures, then improve, refactor, merge, or kill checks based on signal quality.

## Prior Art Applied

1. `.agents/research/2026-04-24-agentops-evaluation-environment.md`
   - Applies directly. It identified the original gap: AgentOps had many proof fragments but needed a unified eval substrate, baselines, scorecards, and runtime/model comparison.

2. `.agents/plans/2026-04-24-agentops-evaluation-environment.md`
   - Applies directly. Most planned eval infrastructure now exists and should be audited against its original intent.

3. `.agents/council/2026-04-24-pre-mortem-agentops-evaluation-environment.md`
   - Applies directly. It predicted overfitting, live-runtime flakiness, and scenario-authoring gaps.

4. `.agents/council/2026-04-25-post-mortem-agentops-eval-environment-longhaul.md`
   - Applies directly. It says the eval foundation landed, while warning that public product gates can pass despite proof/disposition gaps.

## Findings Applied

- `.agents/findings/f-2026-04-27-004.md` — mirrored validators and duplicate scanners drift. This shapes the plan's requirement to deduplicate gate wrappers and add paired fixture coverage before deleting checks.
- `.agents/findings/f-2026-04-14-002.md` — closure proof must cite durable artifacts. This shapes baseline and discovery packet requirements.
- `.agents/findings/f-2026-04-14-001.md` — command refactors require paired command tests. This shapes every `ao eval` or `scripts/eval-agentops.sh` change.

## Current Inventory Snapshot

Commands run during discovery:

```bash
cd cli && env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao eval coverage --root ../evals/agentops-core --json
jq -r '.cases[]?.expectations[]?.type' evals/agentops-core/*.json | sort | uniq -c | sort -nr
find .agents/evals/baselines -maxdepth 1 -name '*.baseline.json' -type f | wc -l
jq -r '[.id,.baseline_policy.mode] | @tsv' evals/agentops-core/*.json | sort
```

Observed:

- 56 public canary suites.
- 281 cases.
- 219 critical cases.
- 54 promoted baseline files.
- Every suite currently declares `baseline_policy.mode = none`.
- Expectation mix: 1083 `artifact_contains`, 211 `stdout_contains`, 137 `exit_code`, 1 `schema_valid`.
- Case mix: 143 artifact-only cases and 137 shell command cases.

## Carry-Forward

The plan should treat the current suite as useful but not uniformly behavioral. The immediate win is a taxonomy and audit loop that lets the factory say:

- keep: high-signal deterministic canaries and baseline comparisons;
- refactor: exact-string artifact checks into focused invariant tests;
- merge: duplicated gate wrappers;
- kill: checks whose only proof is stale generated text already covered by a stronger gate.
