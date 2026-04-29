---
id: brainstorm-2026-04-29-eval-suite-triage
type: brainstorm
date: 2026-04-29
---

# Brainstorm: Eval Suite Triage

## Problem Statement

The AgentOps eval suite now runs a broad deterministic public canary set, but the word "eval" currently covers several different signals: static contract tripwires, command wrappers around existing gates, baseline comparisons, scorecard category checks, and a small amount of behavioral fixture work. The concrete problem is to classify what each suite really proves, improve the high-signal layers, refactor ambiguous or brittle checks, and remove or merge low-signal duplicates.

## Approaches Considered

### Approach 1: Keep The Suite As A Broad CI Wrapper

This treats evals as a curated top-level wrapper around the repo's important tests and validators.

Pros:
- Lowest implementation cost.
- Keeps one command for broad smoke proof.
- Useful during refactors because it exercises many surfaces.

Cons:
- Continues to blur "behavioral eval" with "gate wrapper."
- Makes baseline deltas less meaningful because many cases are binary static checks.
- Encourages new suites to copy exact-string `artifact_contains` patterns instead of proving behavior.

Effort: small.

Red-team result: high risk. The hidden cost is suite bloat and repeated execution of the same lower-level gates. The wrong assumption is that breadth equals product signal.

### Approach 2: Replace With Full Behavioral Evals

This would de-emphasize static canaries and move toward live or fixture-based agent tasks with scored outcomes.

Pros:
- Better aligns "eval" with actual agent outcome quality.
- Gives stronger better/worse signals for RPI, skills, and runtime changes.
- Supports model-upgrade and live runtime decisions.

Cons:
- Too large for a single refactor.
- Live runtime access, auth, cost, variance, and private holdouts are not ready to block PRs.
- Removing current canaries too quickly would lose useful contract tripwires.

Effort: large.

Red-team result: high risk if done as a replacement. The first thing that breaks is deterministic CI reliability.

### Approach 3: Layered Signal Taxonomy And Triage

This keeps the current suite, but names what each case measures and then refactors by layer: contract canaries, executable gate wrappers, behavioral fixtures, baseline comparisons, scorecards, live runtime, and private holdouts.

Pros:
- Preserves useful canaries while making their limits explicit.
- Creates a mechanical kill/refactor path for low-signal checks.
- Lets `--fast` become a real fast lane and moves heavier deterministic wrappers to a full lane.
- Enables future behavioral/live eval work without pretending it already exists.

Cons:
- Requires schema, docs, runner, and suite metadata changes.
- Some suites will need careful replacement proof before deletion.
- The first pass is governance-heavy, not a visible product feature.

Effort: medium.

Red-team result: acceptable. The main risk is taxonomy churn; it is contained by adding conformance checks before rewriting suites.

## Selected Approach

Use Approach 3: layered signal taxonomy and triage.

The selected direction is to preserve the eval system as the product-level proof harness, but stop treating all passing canaries as equal. The next plan should make evidence kind explicit, align baseline policy with promoted artifacts, refactor brittle artifact-only canaries, split fast/full lanes, add real behavioral scorecard fixtures, and keep live runtime/holdout lanes opt-in until variance is calibrated.

## Open Questions

- What budget should `scripts/eval-agentops.sh --fast` target after the split: under 30 seconds, under 60 seconds, or "only changed-surface critical smoke"?
- Should promoted baselines stay under `.agents/evals/baselines/`, or should public canary baselines move to a non-dot checked-in path for clearer governance?
- Which suites are allowed to remain pure contract tripwires, and what metadata threshold makes that acceptable?
- Who can promote baselines once baseline policy becomes explicit: any committer, release flow only, or a named maintainer action?

## Next Step: $plan

Run:

```bash
$plan --auto "Refactor AgentOps eval suite signal quality by adding evidence-kind taxonomy, baseline policy alignment, artifact-only canary triage, fast/full lane split, behavioral scorecard fixtures, and opt-in live-runtime/holdout lanes."
```
