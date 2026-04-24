---
id: brainstorm-2026-04-24-agentops-evaluation-environment
type: brainstorm
date: 2026-04-24
---

# Brainstorm: AgentOps Evaluation Environment

## Problem Statement

AgentOps needs a repeatable evaluation environment that can tell whether changes to skills, hooks, or the `ao` CLI improve or degrade real agent outcomes across RPI flows, Claude/Codex runtime behavior, and new model releases.

## Approaches Considered

### Approach 1: Extend Existing CI Gates

Add more checks to the current validation scripts, especially `validate-headless-runtime-skills.sh`, Codex parity scripts, hook tests, and RPI tests.

Pros:
- Smallest implementation footprint.
- Builds on existing CI and scripts.
- Good for catching structural regressions.

Cons:
- Keeps eval logic scattered.
- Does not create baselines, scorecards, or model comparison reports.
- Still cannot answer whether a skill is better or worse after a prompt/body change.

Red-team result: HIGH RISK. It fails the baseline and cross-runtime comparison requirements.

### Approach 2: External Eval Service

Use an external eval framework or service to run model prompts against AgentOps scenarios and score outputs.

Pros:
- Could provide dashboards, retries, and model comparison quickly.
- Separates eval execution from the repo.

Cons:
- Violates AgentOps' local/auditable posture.
- Harder to test hooks and `ao` CLI behavior in the real repo filesystem.
- Adds auth, cost, and vendor coupling before the local contract is mature.

Red-team result: HIGH RISK. It defers the most important repo-native contracts.

### Approach 3: Repo-Native `ao eval` Subsystem

Create a checked-in eval pack format plus `ao eval` commands for running deterministic suites, optional live runtime/model suites, baseline capture, candidate-vs-baseline comparison, and scorecard generation.

Pros:
- Directly measures skills, hooks, CLI behavior, and RPI artifacts in real worktrees.
- Produces durable, reviewable baselines and scorecards.
- Can support Claude, Codex, and future GPT models through a runtime adapter layer.
- Matches existing retrieval-bench and security-suite patterns.

Cons:
- Larger implementation.
- Needs careful split between deterministic CI gates and costly/nondeterministic live model gates.
- Requires scenario corpus governance to avoid overfitting.

Red-team result: viable if implemented in tiers.

## Selected Approach

Select Approach 3: a repo-native layered eval subsystem.

The initial shape should be:

- `evals/` for checked-in public canary suites, seed repos, rubrics, and manifests.
- `.agents/evals/` for generated run outputs, local baselines, and trend reports.
- `ao eval run` to execute suites against a candidate branch, runtime, and model.
- `ao eval compare` to compare candidate results with a known-good baseline.
- `ao eval baseline` to capture and promote baselines intentionally.
- Scorecards with deterministic dimensions first: artifact presence, command success, test pass/fail, phase order, hook behavior, CLI JSON validity, scenario satisfaction, runtime load, and transcript/tool-error signals.

Live Claude/Codex/model execution should be tiered:

- PR/fast: deterministic fixture suites and mocked runtime adapters.
- Nightly/local opt-in: live headless Claude/Codex execution with isolated HOME/CODEX_HOME and env scrubbing.
- Release/model-upgrade: repeated RPI canary runs with variance reporting and baseline comparison.

## Open Questions

- Where should private holdout suites live for local-only evaluation without leaking into checked-in public canaries?
- What is the minimum RPI canary suite that deserves a hard 100% pass gate?
- How many repeated live runs are enough before a model/runtime regression is credible?
- Should the first implementation expose `ao eval` as a top-level command immediately, or start with `ao rpi eval` plus a reusable internal package?

## Next Step: $plan

Run `$plan --auto "build a repo-native AgentOps eval subsystem with checked-in eval packs, ao eval run/compare/baseline commands, RPI canary suites, Claude/Codex runtime adapters, and scorecards for skill/hook/CLI regressions"`.
