# Behavioral Discipline

AgentOps does not just add orchestration. It also pushes coding agents toward better behavior.

The core idea is simple: good agents do not silently assume, overbuild, wander outside scope, or declare victory without proof. This page shows what that means in practice.

## The Four Behaviors

| Behavior | What it prevents |
|---|---|
| **Think before coding** | hidden assumptions, silent confusion, wrong interpretation |
| **Simplicity first** | bloated abstractions, speculative flexibility, oversized patches |
| **Surgical changes** | drive-by refactors, unrelated edits, noisy diffs |
| **Goal-driven execution** | weak verification, "looks done" changes, proof by assertion |

AgentOps enforces this through the behavioral discipline standard in [`skills/standards/references/behavioral-discipline.md`](https://github.com/boshu2/agentops/blob/main/skills/standards/references/behavioral-discipline.md), and through the active `/implement` and `/review` skill flows.

## Before/After Examples

### 1. Ambiguous Request

**User request:** "Make search faster"

**Before**

- The agent silently chooses one meaning of "faster"
- It adds caching, async work, and extra config knobs
- It ships a larger patch than the question justified

**After**

- The agent asks whether "faster" means latency, throughput, or perceived speed
- It chooses the smallest change that matches the answer
- It verifies against the metric that actually mattered

**Why this is better:** the work is driven by the real goal, not by the agent's guess.

### 2. Overbuilt Solution

**User request:** "Add a discount helper"

**Before**

- The agent creates strategy objects, factories, and multiple extension points
- It adds flexibility nobody asked for
- The code becomes harder to understand than the problem itself

**After**

- The agent starts with a small helper such as `calculate_discount(amount, percent)`
- It adds complexity only when the requirement actually expands
- The patch stays proportional to the problem

**Why this is better:** complexity should be earned by requirements, not imagined in advance.

### 3. Drive-By Editing

**User request:** "Fix empty emails crashing the validator"

**Before**

- The agent rewrites adjacent validation logic
- It changes comments, formatting, and unrelated username rules
- The diff is noisy and hard to review

**After**

- The agent adds a reproducer for the empty-email case
- It fixes only the email path and only the cleanup caused by that fix
- Unrelated problems are noted separately instead of bundled into the patch

**Why this is better:** every changed line maps to the stated goal.

### 4. Weak Proof

**User request:** "Improve this AgentOps skill"

**Before**

- The agent edits the skill text and says it is done
- It skips the mirrored Codex artifact or does not validate it
- The repo gets instruction drift

**After**

- The agent updates the shared skill contract and the checked-in Codex copy
- It regenerates the affected hash metadata when needed
- It runs the relevant validation commands before claiming completion

**Why this is better:** completion is defined by evidence, not by the existence of an edit.

## What to Look For in a Good Agent Diff

- The request is restated clearly before coding starts.
- Ambiguity is surfaced instead of silently resolved.
- The patch is smaller than the first idea, not larger.
- Unrelated files stay untouched.
- The verification matches the behavior that was changed.

## Where This Lives in AgentOps

- [`README.md`](https://github.com/boshu2/agentops/blob/main/README.md) gives the front-door version.
- [`skills/implement/SKILL.md`](skills/implement.md) requires an execution frame before editing.
- [`skills/review/SKILL.md`](skills/review.md) checks for hidden assumptions, speculative abstractions, and weak proof.
- [`skills/standards/references/behavioral-discipline.md`](https://github.com/boshu2/agentops/blob/main/skills/standards/references/behavioral-discipline.md) is the reusable reference that the skills load.
