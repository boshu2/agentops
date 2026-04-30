> **Status:** olympus v4 reference, NOT canonical for agentopsd. This file is a verbatim port from olympus/docs/specs/v4/ for cross-reference only. Where this disagrees with agentopsd's canonical design at `.agents/design/2026-04-28-design-agentops-daemon-gascity-vertical-slices.md`, **agentopsd canonical wins**.

# Validation

> Workers never validate their own output.

**Date:** 2026-02-15
**Status:** Draft

---

## The Invariant

The agent that wrote the code does not get to decide if it works. Validation is a separate command, run in a separate invocation, against a separate copy of the worktree. This is non-negotiable. An LLM that grades its own output will always find reasons to pass it.

This is not a policy. It is the architecture. The `ol validate` commands exist as standalone CLI invocations precisely so that no worker context can influence the outcome.

---

## Stage 1 -- Mechanical

**CLI:** `ol validate stage1 --quest <id> --bead <id> --worktree <path>`

Three checks run in parallel against the worktree:

| Check | Command | Timeout |
|-------|---------|---------|
| Build | `go build ./...` | 60s |
| Vet | `go vet ./...` | 60s |
| Test | `go test ./...` | 60s |

**Runtime:** ~5 seconds for this codebase. All three run concurrently.

**Result:** Binary pass/fail. If any check returns non-zero, Stage 1 fails. There is no partial credit. There is no "passed with warnings." The code compiles, the vet is clean, and the tests pass -- or it fails.

**Hard gate semantics:** No LLM can override a failing test. No human can override a failing test. No council vote, no consensus mechanism, no review process changes the outcome. The test either passes or it does not. Fix the code.

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All checks passed |
| `1` | Infrastructure error (tool missing, timeout, disk full) |
| `2` | One or more checks failed |

Exit code 1 means the validation itself broke -- not that the code is bad. Retry or fix the environment. Exit code 2 means the code is bad. Fix the code.

## MemRL Policy Modes (Validation/Ratchet Contract)

`memrl_mode` is a routing policy input for odyssey/ratchet behavior and defaults to `off`.

| Mode | Behavior | Hard-Gate Interaction |
|------|----------|-----------------------|
| `off` | Existing retry/escalate behavior only | No change |
| `observe` | Compute/log policy recommendation only | No change |
| `enforce` | Apply deterministic retry/escalate policy table | May reduce retries, never turns a failing gate into pass |

Contract invariants:

1. Stage 1 and Stage 2 exit-code contracts stay unchanged (`0/1/2`).
2. `memrl_mode` never overrides deterministic validation failures.
3. Unknown/invalid mode MUST normalize to `off`.
4. Missing or unknown failure classes MUST deterministically fall back to `escalate` in enforce mode.

---

## Stage 2 -- Merge Gate

**CLI:** `ol validate stage2 --quest <id> --bead <id> --demigod-branch <branch>`

### Preconditions

1. **Stage 1 must have passed.** Stage 2 reads the run ledger and checks the latest validation record for this bead. If Stage 1 has not passed, Stage 2 refuses to run. No exceptions.

2. **Independent review.** The merge requires a check that did not come from the worker. This is either a human approval or an external validation tool. The point is separation of concerns: the entity that wrote the code is not the entity that approves the merge.

### Merge Behavior

On success: merges the demigod branch into the quest branch. The merge commit includes provenance (bead ID, attempt number, source branch). This is a permanent ratchet -- merged code does not go backward.

On conflict: returns `MERGE_CONFLICT` with the list of conflicting files. No automatic resolution. Conflicts are information; they mean two pieces of work touched the same thing and a human (or a fresh agent with both contexts) needs to decide.

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Merged successfully |
| `1` | Infrastructure error (git failure, ledger unreadable) |
| `2` | Precondition failed (Stage 1 not passed, review missing, merge conflict) |

---

## Constraint Tests

Learnings from failures compile into tests. Not documentation. Not guidelines. Tests.

The rule is binary: if a failure pattern can be caught by a test, write the test. If it cannot be expressed as a test, do not pretend it can be enforced. There is no graduated severity. There is no "report" mode that upgrades to "warn" that upgrades to "block." Either the test blocks the bad pattern or it does not exist.

### How They Work

1. A failure occurs (Stage 1 fail, production bug, post-mortem finding).
2. The `/retro` or `/post-mortem` skill extracts the root cause.
3. If the root cause is mechanically detectable, a test is written.
4. The test runs as part of `go test ./...` in Stage 1.
5. The pattern never recurs.

Constraint tests are regular Go tests. They live in `_test.go` files alongside the code they protect. They run in the same `go test ./...` invocation as everything else. No special framework. No separate enforcement tool. No configuration file.

### What Makes a Good Constraint Test

- It fails when the anti-pattern is present.
- It passes when the anti-pattern is absent.
- It has a comment explaining what failure it prevents and when it was added.
- It runs in under one second.

### What Does Not Become a Constraint Test

- Style preferences (use a linter).
- Things that require human judgment.
- Patterns that cannot be detected mechanically.

If you cannot write a test that reliably detects the problem, the answer is not a weaker form of enforcement. The answer is that this particular problem is not mechanically enforceable.

---

## Run Ledger

Every validation attempt is recorded. No exceptions. The ledger is the source of truth for what happened, not memory, not logs, not the agent's claim about what it did.

**Location:** `.ol/runs/<bead-id>/<attempt>-validate.json`

### Record Fields

| Field | Purpose |
|-------|---------|
| `bead_id` | What was validated |
| `quest_id` | Parent quest |
| `attempt` | Attempt number (monotonically increasing) |
| `passed` | Boolean outcome |
| `bundle_hash` | Hash of the context bundle used |
| `git_head` | Git HEAD at validation time |
| `timestamp` | When it ran |
| `steps` | Per-check results (command, exit code, duration, truncated output) |

### Why This Matters

The run ledger exists so that Stage 2 can verify Stage 1 passed without trusting anyone's word for it. It exists so that `ol context runs --bead <id>` can show every attempt, every failure, every fix. It exists so that when something goes wrong, you can trace exactly what happened.

The `bundle_hash` and `git_head` fields make runs reproducible. Given the same bundle and the same git state, validation produces the same result. If it does not, that is a bug in the validation infrastructure, not in the code being validated.

---

## Competitive Context

> Source: `.agents/research/2026-02-16-competitive-product-analysis.md`

The "no self-grading" invariant is not just a design choice -- it is Olympus's strongest competitive differentiator. A survey of 15 AI coding tools (February 2026) found that **every competing product self-grades**:

| Product | Validation Model | External Gate? |
|---------|-----------------|----------------|
| Devin (Cognition) | Agent runs tests, iterates, opens PR | Human PR review only |
| OpenAI Codex | RL-trained self-correction, retries until tests pass | Human review only |
| Cursor | Same agent writes and validates via lint/test loops | No (hooks since 1.7, but same context) |
| Windsurf | Auto-linter-fix on generated code | No |
| Aider | Lint/test with auto-repair loop | Partial (external tool signals, but same LLM interprets) |
| Claude Code | Bash-based test execution in agent loop | No (unless user constructs subagent reviewer) |
| SWE-agent | Test suite execution with retry | No |
| OpenHands | Pydantic schemas + test execution | No |
| AutoCodeRover | Test suite + retry up to threshold | No |
| Cline | Human approves every action | Yes (human-in-the-loop, but doesn't scale) |
| GitHub Copilot | Self-healing agent mode | No |
| Augment Code | AI-powered code review (same provider) | No |

Cline is the only product with genuine external validation (human approval per action), but human-in-the-loop is O(n) in task count and does not scale to autonomous operation.

**What makes Olympus's approach unique:**

1. **Architectural separation.** `ol validate` is a separate CLI command, not a method call inside the worker. Separate invocation, separate process, separate context window. The validator literally cannot see the worker's reasoning.

2. **Mechanical gates cannot be overridden.** Stage 1 (`go build/vet/test`) returns a binary pass/fail. No LLM, no council vote, no human can override a failing test. The competing pattern is "agent interprets test output and decides whether to retry" -- which allows the agent to rationalize failures.

3. **Multi-model consensus at ratchet points.** Plan approval and quest completion require agreement from multiple models. No competitor uses multi-model consensus for validation integrity. (Competitors use multi-model for cost optimization or user preference, not for two-person integrity.)

4. **Learnings compile into tests.** Constraint tests convert past failures into `*_test.go` files that run in Stage 1. No competitor converts experience into executable build gates. Devin's Knowledge Entries and Claude Code's Auto Memory are the closest, but both are advisory (markdown/text), not enforcement (test failures).

---

## Summary

Two stages. Three checks. Binary outcomes. No self-grading.

Stage 1 is mechanical and cannot be overridden. Stage 2 requires Stage 1 and independent review. Constraint tests are regular Go tests that block recurrence of known failures. The run ledger records everything.

That is the entire validation model.

---

*v4 validation spec -- 2026-02-15*
