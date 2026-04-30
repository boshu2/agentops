> **Status:** olympus v4 reference, NOT canonical for agentopsd. This file is a verbatim port from olympus/docs/specs/v4/ for cross-reference only. Where this disagrees with agentopsd's canonical design at `.agents/design/2026-04-28-design-agentops-daemon-gascity-vertical-slices.md`, **agentopsd canonical wins**.

# Context as Control Plane

> *"The context window is the agent. Go code is plumbing."*

**Date:** 2026-02-15
**Status:** Draft (v4)

---

## Overview

Context is not input to the agent — context IS the agent. Two agents with identical models but different context bundles produce entirely different work. Olympus treats context assembly with the same rigor as code compilation: deterministic inputs, hashable output, full provenance, diffable between attempts.

Every agent execution starts with `ol context build` or `ol hero embark`. Both produce the same artifact: a context bundle. The bundle is the build artifact. The agent is the runtime.

---

## Context Bundle

A context bundle contains everything an agent needs to execute a bead. Nothing more, nothing less.

### Bundle Contents

| Section | Source | Purpose |
|---------|--------|---------|
| **Bead spec** | `bd show <id>` | What to do — title, acceptance criteria, file paths, constraints |
| **Role AGENTS.md** | `roles/<role>/AGENTS.md` | Who you are — personality, boundaries, capabilities |
| **Relevant learnings** | `ao inject` | What we've learned — anti-patterns, domain knowledge, constraints |
| **Prior attempt feedback** | Run ledger (odyssey records) | What went wrong last time — specific failure details and guidance |
| **Relevant source files** | Bead spec file paths | What exists — current state of code being modified |

### Assembly Order

Bundle sections are assembled in a fixed order. This ordering is intentional — it establishes priority. When context window limits force truncation, later sections are trimmed first:

1. Role (identity — never trimmed)
2. Bead spec (task — never trimmed)
3. Implementation constraints (search-first constraint — never trimmed, implement mode only)
4. Prior attempt feedback (most actionable context for retries)
5. Learnings (accumulated knowledge relevant to this work)
6. Operational context (workspace conventions — trimmable, research/plan/implement modes)
7. Source files (reference material — can be regenerated from disk)

Sections 3 and 6 are injected only when the assembly mode activates them (see Mode-Aware Assembly below).

### Content Hash

The bundle produces a SHA-256 content hash over the rendered prompt text. Same inputs produce the same hash. This hash links the bundle to its run ledger entry, making it possible to answer: "what exact context did the agent have when it produced this result?"

Fields excluded from hash: `built_at` timestamp, `token_estimate` (derived, not input).

---

## Mode-Aware Assembly

`ol context build --mode <mode>` activates phase-appropriate context shaping. Each Zeus phase has
different context needs; the mode flag selects the right shape automatically.

### Assembly Modes

| Mode | `--mode` | Budget | Operational | Search Constraint |
|------|----------|--------|-------------|-------------------|
| Default (no mode) | `""` | unlimited | no | no |
| Research | `research` | 200K tokens | yes | no |
| Plan | `plan` | 100K tokens | yes | no |
| Implement | `implement` | 80K tokens | yes | yes |
| Validate | `validate` | 40K tokens | no | no |

### Mode Behaviors

**Token budget:** When `--mode` is set and no explicit `TokenBudget` is provided, the mode's
default budget is applied. An explicit `--budget` always overrides the mode default.

**Operational context section:** Injects workspace conventions (working directory, test command,
build command, git branch) before source files. Trimmable under tight budgets.

**Search constraint section:** Injects the "search before implementing" constraint after bead
spec (never trimmed). Instructs agents to grep/glob for existing implementations before writing
new code. Activates only in implement mode.

### Search-Before-Implementing Constraint

The constraint text injected in implement mode:

```
# Implementation Constraints

BEFORE writing new code, search the codebase:
- Use grep/glob to find existing implementations of the pattern
- Read related files before creating new ones
- Do NOT assume something is not implemented — verify first
- If you find existing code, extend it; do not duplicate it
- Search for the concept in at least 3 ways before concluding it is absent
```

This constraint is `canTrim: false` — it survives even under severe token budget pressure.

### Backward Compatibility

`BuildOpts.Mode == ""` (the default) produces identical output to all prior builds. No
mode = no new sections, no budget constraint, same content hash for same inputs.

---

## Run Ledger

The run ledger is an append-only record store scoped to each bead. It answers: "what happened every time we tried to work this bead?"

### Storage

```
.ol/runs/{bead-id}/
  1-embark.json        # Provenance at claim time
  1-validate.json      # Validation outcome
  1-odyssey.json       # Failure routing decision (only on failure)
  2-embark.json        # Second attempt provenance
  2-validate.json      # Second attempt outcome
```

### Record Types

**Embark record** — Written when an agent claims a bead and begins work.

```json
{
  "kind": "embark",
  "attempt": 1,
  "bead_id": "ol-569.2",
  "quest_id": "ol-569",
  "content_hash": "a1b2c3d4...",
  "git_head": "abc1234",
  "strategy_name": "Standard",
  "timestamp": "2026-02-15T10:00:00Z"
}
```

**Validate record** — Written after Stage 1 or Stage 2 validation completes.

```json
{
  "kind": "validate",
  "attempt": 1,
  "bead_id": "ol-569.2",
  "quest_id": "ol-569",
  "passed": false,
  "summary": "go test failed: TestEntryPointWiring",
  "timestamp": "2026-02-15T10:15:00Z",
  "steps": [
    {"name": "go build", "passed": true, "exit_code": 0, "duration": "8s"},
    {"name": "go vet", "passed": true, "exit_code": 0, "duration": "2s"},
    {"name": "go test", "passed": false, "exit_code": 1, "duration": "12s"}
  ]
}
```

**Odyssey record** — Written when a failed attempt is routed for retry or escalation (`action` is `retry` or `escalate`).

```json
{
  "kind": "odyssey",
  "attempt": 1,
  "bead_id": "ol-569.2",
  "quest_id": "ol-569",
  "action": "retry",
  "feedback": "TestEntryPointWiring failed — new command not wired in root.go",
  "timestamp": "2026-02-15T10:16:00Z"
}
```

### Append-Only Invariant

Records are created atomically (`O_EXCL`). Existing records are never overwritten. If attempt 1 failed and attempt 2 succeeds, both records remain. The ledger is a complete history, not a current-state snapshot.

---

## Bundle Provenance

Every bundle carries provenance — the exact inputs that produced it. This makes agent executions auditable, not anecdotal.

| Field | Value | Purpose |
|-------|-------|---------|
| `git_head` | Commit SHA at build time | Pins the codebase state |
| `bead_id` | Bead identifier | Links to work unit |
| `quest_id` | Parent quest | Links to epic context |
| `attempt` | 1-based attempt number | Distinguishes retries |
| `content_hash` | SHA-256 of rendered prompt | Proves exact context content |
| `strategy_name` | Selected strategy | Records routing decision |
| `sources` | File paths included | Tracks what code was visible |
| `learnings` | Injected learning slugs | Tracks what knowledge was available |

Provenance is written to the bundle manifest (`.ol/prompts/<bead>-attempt-<n>.bundle.json`) and to the embark record in the run ledger. Two copies, same data, different access patterns.

---

## Context Diffing

`ol context diff --bead <id>` compares bundles between attempts. This answers the question every debugging session starts with: "what changed?"

### What Gets Diffed

- **Added/removed learnings** — New knowledge injected after a failure.
- **Feedback text** — Specific guidance added from the odyssey record.
- **Strategy changes** — Escalation from Standard to Evolutionary.
- **Source file changes** — Code modified between attempts.
- **Content hash change** — Proves the context was actually different.

### Example Output

```
Context diff for ol-569.2 (attempt 1 → attempt 2):

  Strategy:   Standard → Standard (unchanged)
  Git HEAD:   abc1234 → abc1234 (unchanged)

  + learning:  dead-code-at-entry-point
  + feedback:  "TestEntryPointWiring failed — wire new command in root.go"

  Content hash: a1b2c3d4 → e5f6a7b8
```

If the diff is empty — same hash, same content — then retrying with identical context is pointless. The system flags this. Fresh context or a different strategy is needed.

---

## Run History

`ol context runs --bead <id>` shows all attempts with their outcomes. This is the timeline view of a bead's execution history.

### Example Output

```
Runs for ol-569.2:

  Attempt  Strategy    Result   Duration  Hash
  1        Standard    FAIL     22s       a1b2c3d4
  2        Standard    PASS     18s       e5f6a7b8

  First-attempt success: no
  Total attempts: 2
  Feedback injected on retry: yes
```

### What This Enables

- **Debugging** — Trace exactly why attempt 1 failed and attempt 2 succeeded.
- **Pattern detection** — Beads that consistently fail on first attempt signal poor spec quality.
- **Cost analysis** — Multi-attempt beads cost more. The run history shows where cost accumulates.
- **Flywheel input** — Patterns across run histories feed back into learnings.

---

## References

- `docs/specs/v4/context.md` — Bundle generation pipeline and manifest schema
- `docs/specs/v4/execution.md` — Run ledger format and telemetry surfaces
- `docs/specs/v4/architecture.md` — Context bundle definition and data flow
