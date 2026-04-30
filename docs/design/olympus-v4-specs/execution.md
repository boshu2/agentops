> **Status:** olympus v4 reference, NOT canonical for agentopsd. This file is a verbatim port from olympus/docs/specs/v4/ for cross-reference only. Where this disagrees with agentopsd's canonical design at `.agents/design/2026-04-28-design-agentops-daemon-gascity-vertical-slices.md`, **agentopsd canonical wins**.

# Olympus v4 Execution

**Status:** Normative
**Date:** 2026-02-15
**Depends On:** `architecture.md`, `validation.md`, `knowledge.md`

---

## Overview

Execution is how work moves from "quest created" to "code merged and learnings stored". Three layers compose to drive this: the **Quest Lifecycle** manages the high-level arc, the **Zeus Phase Machine** sequences the phases within a quest, and the **HERO Loop** executes individual beads within phases.

All execution is CLI-driven. The daemon is a mechanical poll loop that calls the same commands a human would. There is no intelligence in the daemon process.

---

## 1. Quest Lifecycle

A quest is an epic bead with child beads representing decomposed work.

```
create → execute (Zeus phases) → complete | cancel
```

### Commands

```bash
ol quest create "title" --spec=path   # Create epic bead + quest branch
ol quest list [--status STATUS]       # List quests
ol quest show <id> [-v]               # Show quest + child bead progress
ol quest complete <id>                # Complete (validation-gated)
ol quest cancel <id> --reason "..."   # Cancel quest
```

### States

| State | Description | Transition |
|-------|-------------|------------|
| Created | Epic bead exists, children may not | Execute |
| Active | Zeus phases running | Complete, Cancel |
| Complete | All children closed, validation passed | Terminal |
| Cancelled | Abandoned with reason | Terminal |

Quest completion is validation-gated. The worker cannot declare its own quest complete.

---

## 2. Zeus Phase Machine

Zeus sequences a quest through thirteen phases. Each phase produces artifacts. Gates require external validation.

```
RESEARCH → PLAN → PLAN_GATE → PRE_MORTEM → PRE_MORTEM_GATE →
CRANK_PREP → CRANK_VALIDATE → VIBE_GATE → POST_MORTEM →
RETRO → FLYWHEEL → FITNESS_GATE → DONE
```

### Phase Details

| Phase | What Happens | Artifacts |
|-------|-------------|-----------|
| RESEARCH | Deep exploration of the problem space | `.agents/research/*.md` |
| PLAN | Decompose into beads with acceptance criteria | `.agents/plans/*.md` + child beads |
| PLAN_GATE | External review of the plan | `.agents/council/*plan-review*` |
| PRE_MORTEM | Stress-test the plan, find failure modes | `.agents/council/*pre-mortem*` |
| PRE_MORTEM_GATE | Validate pre-mortem addressed risks | Gate decision |
| CRANK_PREP | Execute beads via HERO loop | Committed code + closed beads |
| CRANK_VALIDATE | Stage 1 validation (build/vet/test) | Validation results in run ledger |
| VIBE_GATE | External review of implementation | `.agents/council/*vibe*` |
| POST_MORTEM | Analyze what happened | `.agents/council/*post-mortem*` |
| RETRO | Extract lessons learned | `.agents/retros/*.md` |
| FLYWHEEL | Compile learnings into constraint tests | `.agents/learnings/*.md` |
| FITNESS_GATE | Deterministic regression check against quest fitness baseline | Gate decision |
| DONE | Quest complete | — |

### Gate Behavior

Gates are validation checkpoints. They never self-grade.

- **PLAN_GATE, PRE_MORTEM_GATE, VIBE_GATE, POST_MORTEM:** External council review. On failure, exit code 2 (escalate to human).
- **CRANK_VALIDATE:** Deterministic. `go build && go vet && go test`. No LLM can override a failing test.
- **FITNESS_GATE:** Deterministic baseline fitness check. On regression, exit code 2 (escalate to human).
- **Quest Completion:** Independent validation gate. Always escalates on infrastructure error.

### Skill Routing

Each phase maps to a skill invocation when driven by subprocess dispatch:

| Phase | Skill |
|-------|-------|
| RESEARCH | `/research` |
| PLAN | `/plan` |
| PLAN_GATE | `/council validate plan` |
| PRE_MORTEM | `/pre-mortem` |
| CRANK_PREP | `/crank` |
| VIBE_GATE | `/vibe` |
| POST_MORTEM | `/post-mortem` |
| RETRO | `/retro` |
| FLYWHEEL | `/retro` |
| FITNESS_GATE | Deterministic (no skill) |

---

## 3. HERO Loop

The HERO loop executes beads within CRANK_PREP. Four phases, each a standalone CLI command.

```
HUNT → EMBARK → RATCHET → ODYSSEY
  ^                          |
  └──── retry budget left ───┘
```

### Hunt -- Find Ready Work

```bash
ol hero hunt --quest <id>
```

Reads child beads, computes the ready set, outputs the wave as JSON.

**Readiness:** A bead is ready when its status is `open`, all dependencies are `merged` or `closed`, and attempt count is below `max_retries` (3).

### Embark -- Claim and Build Context

```bash
ol hero embark <bead-id> --quest <id> [--attempt N] [--feedback "..."]
```

1. Claim the bead (status → `in-progress`)
2. Build context bundle: learnings, constraints, prior feedback
3. Write bundle to `.ol/prompts/<bead-id>-attempt-<N>.bundle.json`
4. Write embark record to run ledger

On build failure, release the claim (status → `open`).

### Ratchet -- Validate and Lock Progress

```bash
ol hero ratchet --quest <id>
```

Reads the run ledger for all quest children. Reports pass/fail/pending per bead. The ratchet does not perform validation — it reads results written by `ol validate`. No self-grading.

**Irreversibility:** Merged code stays merged. Closed beads stay closed. Progress only moves forward.

### Odyssey -- Handle Failures

```bash
ol hero odyssey <bead-id> --quest <id> [--feedback "..."]
```

Routes failed beads:

- **Retry** (attempt < max_retries): Reset bead to `open`, rebuild context with failure feedback, re-embark.
- **Escalate** (attempt >= max_retries): Write escalation record, exit code 2.

One bead = one attempt per HERO cycle. If it fails, spawn fresh with feedback. The attempt is disposable. The feedback compounds.

---

## 4. Wave Scheduling

Waves are dependency-based. No adaptive N, no strategy selection, no diversity seeds.

### Algorithm

1. Build dependency DAG from quest children
2. Detect cycles via iterative DFS; exclude cycle members (warn to stderr)
3. Filter to ready beads (open + all deps satisfied + under retry budget)
4. Sort: priority descending, then dependency depth ascending (shallower first), then ID (stable)
5. Trim to capacity (`max_demigods`, default 4)

### Output

```json
{
  "wave": [{"id": "ol-600.3", ...}],
  "blocked": [{"id": "ol-600.5", "status": "blocked"}],
  "completed": [{"id": "ol-600.1", "status": "closed"}],
  "cycles": []
}
```

If cycles exist, non-cycle beads still schedule. Cycle members appear in `cycles` with a warning.

---

## 5. Execution Modes

Two modes drive the same commands. Both produce identical artifacts and obey identical validation gates.

### Manual (Agent-Native)

The Claude session IS Zeus. The agent calls CLI commands directly:

```bash
ol zeus step --quest <id>       # Returns current phase + what to do
# ... agent does work ...
ol zeus step --quest <id>       # Step again (advances automatically)
# ... repeat until exit 42 ...
```

This is the preferred mode. The agent has full conversational context and can reason about intermediate results.

### Daemon-Driven (Apollo Poll Loop)

The daemon is a mechanical poll loop. No LLM in the daemon process.

```
loop:
  scan for active quests
  for each quest:
    ol zeus step --quest <id>
    route exit code (dispatch work if needed)
  sleep(poll_interval)
```

The daemon adds three things manual mode lacks:

1. **Cross-quest scheduling** — Allocate capacity across a portfolio, not one quest at a time.
2. **Continuous operation** — No human needed to start the next cycle.
3. **Batch reporting** — Summary artifacts for each autopilot run.

### Subprocess Dispatch

Both modes can dispatch work to `claude -p` subprocesses. Each dispatch writes forensic evidence:

```
.ol/zeus/<quest-id>/dispatch-<phase>/
  prompt-attempt<n>.txt      # Written before spawn
  stdout-attempt<n>.txt      # Streamed during execution
  stderr-attempt<n>.txt      # Streamed during execution
  attempt-<n>.json           # Updated on completion
```

Retry policy: up to 2 retries. Binary not found → immediate escalate. Non-zero exit with artifacts → treat as success. Non-zero without artifacts → retry.

---

## 6. Exit Codes

Every command that advances a state machine uses this contract:

| Code | Meaning |
|------|---------|
| 0 | Success / more phases remain |
| 1 | Error |
| 2 | Human needed (gate failure, budget exceeded, escalation) |
| 42 | Quest complete (all phases done) |

Exit code 42 is architectural. It propagates up through HERO, Zeus, and autopilot.

---

## State Persistence

All state is on disk. Nothing in memory. Any crash recovers by reading files.

| What | Where | Format |
|------|-------|--------|
| Bead state | `.beads/issues.jsonl` | JSONL (git-committed) |
| Run records | `.ol/runs/<bead-id>/` | JSON per attempt |
| Quest events | `.ol/quests/<quest-id>/events.jsonl` | JSONL (append-only) |
| Context bundles | `.ol/prompts/` | JSON per attempt |
| Zeus checkpoint | `.ol/zeus/<quest-id>/checkpoint.yaml` | YAML |
| Dispatch forensics | `.ol/zeus/<quest-id>/dispatch-<phase>/` | Mixed |

---

## The Brownian Ratchet

Everything above implements one pattern:

| Concept | Implementation |
|---------|---------------|
| **Chaos** | Agents working beads with fresh context each attempt |
| **Filter** | `ol validate` — external, deterministic, no self-grading |
| **Ratchet** | Merged code, closed beads, stored learnings — irreversible |
| **Energy** | Context bundles with compounded knowledge from prior failures |

Don't optimize the ephemeral. Each agent attempt is disposable. What matters: merged code (ratcheted) and learnings (compounded). Failures produce feedback that makes the next attempt better.

---

*Consolidated from hero-loop.md, state-machines.md, autonomous-mode.md, autopilot.md — 2026-02-15*
