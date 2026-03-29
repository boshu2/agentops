# AgentOps — One-Page Brief

**The problem:** AI coding tools behave like contractors with amnesia. Every session starts from zero — no memory of what broke last week, no record of decisions already made, no awareness of what was tried and abandoned. You brief them today. Tomorrow you brief them again.

---

## What It Is

A repo-native operating layer for stateless coding agents.

AgentOps gives every session a mission, a phased workflow, quality gates, and a durable memory substrate on disk. Work begins with repo context instead of a blank prompt, passes through explicit plan and code judgment, and ends by feeding findings, learnings, and next work back into the environment.

The institutional knowledge stops walking out the door because the repo keeps it.

---

## Four Load-Bearing Layers

### Skills — The Orchestration Layer
Structured workflows for discovery, execution, validation, recovery, and release. Skills route work into the right primitive chain instead of leaving the agent to improvise the lifecycle.

### Hooks — The Enforcement Layer
Runtime hooks fire at session start/end, prompt submission, tool boundaries, stop, and task completion. They keep guidance, validation, and loop closure active even when the operator does not remember every step.

### `ao` CLI — The Retrieval and Ratchet Layer
The CLI handles retrieval, ratchet checkpoints, flywheel closure, goals, curation, and phased execution support. It is the mechanical bridge between plain files, workflows, and enforceable progress.

### `.agents/` — The Stigmergic Memory Layer
Plain files on disk: research, brainstorms, findings, learnings, handoffs, ratchet traces, next-work queues. No single session has to remember the plan. The environment carries it forward.

---

## How a Session Works

```
Session starts
  -> Startup hooks retrieve lightweight context and continuity hints
  -> Discovery scopes the work and pressure-tests the plan

Implementation runs
  -> Fresh workers execute in bounded waves
  -> Validation gates challenge the output before closure

Session ends
  -> Learnings, findings, and next work are harvested
  -> Flywheel closure updates what the next session will see

Next session starts with a richer environment than this one did.
```

---

## Key Properties

| Property | Detail |
|----------|--------|
| **Local-only** | No telemetry, no cloud, no vendor accounts. Nothing phones home. |
| **Open source** | Every line auditable. Apache 2.0 licensed. |
| **Multi-tool** | Works with Claude Code, Codex, Cursor, OpenCode. Not locked to one vendor. |
| **Air-gap compatible** | Runs fully offline. Knowledge base is plain files. |
| **Auditable trail** | Every learning, decision, and review verdict written to `.agents/` with timestamps. |

---

## The Compound Effect

```
Without AgentOps:  [2 hrs] → [2 hrs] → [2 hrs] → [2 hrs]  =  8 hours total
With AgentOps:     [2 hrs] → [10 min] → [2 min] → instant  =  ~2.2 hours total
                    learn     recall     refine    mastered
```

By session 100, the repo already carries prior failures, design choices, planning rules, and validated patterns that new sessions can load before they repeat old mistakes.

---

## Development Model

The most accurate current framing is:

```text
12-Factor AgentOps -> operating conditions
Stigmergic Spiral -> macro lifecycle for stateless builders
Brownian Ratchet -> chaos filtered into locked progress
Knowledge Flywheel -> durable learning and loop closure
```

The claim is not "better models." The claim is "better repo mechanics around the models you already have."

---

*AgentOps — github.com/boshu2/agentops*

---

## Appendix: System Map

### Scale

```
┌──────────────────────────────────────────────────────────────────┐
│                    AgentOps at a Glance                          │
├───────────────────┬──────────────────────┬───────────────────────┤
│    54 Skills      │   52 CLI Commands    │   7 Hook Events       │
│  (45 + 9 split)   │  (audited surfaces)  │  (runtime manifest)   │
└───────────────────┴──────────────────────┴───────────────────────┘
```

### The Pipeline — Primitive Chains in Motion

`/rpi` orchestrates the macro lifecycle. Each phase expands into its own skill chain.

```
GOALS.md
  -> /evolve
      -> /rpi
          -> Discovery: /brainstorm -> /research -> /plan -> /pre-mortem
          -> Implementation: /crank -> /swarm -> /implement
          -> Validation: /validation -> /vibe -> /post-mortem -> /retro -> /forge
```

### Judgment Layer — Everything Flows Through Council

```
                   ┌──────────────────────────────┐
                   │           /council           │
                   │  (independent reviewers      │
                   │   debate, verdict gates work)│
                   └───────────┬──────────────────┘
                               │ used by
          ┌────────────────────┼────────────────────┐
          ▼                    ▼                    ▼
   /pre-mortem              /vibe              /post-mortem
   (validate plans          (validate code     (wrap-up +
    before building)         before shipping)   learnings)
```

### Knowledge Handoff — Skills and CLI Working Together

```
   SURFACE                 CLI / FILE PRIMITIVE          RESULT
   ───────                 ────────────────────          ──────
/research          ->    ao lookup + ao search      Prior repo context loaded
/plan              ->    findings registry          Reusable risks loaded pre-decomposition
/post-mortem       ->    ao forge + ao session      Learnings harvested and session closed
/vibe              ->    ao ratchet record          Validation checkpoint persisted
/evolve            ->    ao goals measure           Worst fitness gap selected
/recover           ->    handoff artifacts          Interrupted work resumed from disk
```

### Hooks — Automatic Enforcement

```
TRIGGER                   HOOK                        WHAT IT DOES
───────                   ────                        ────────────
Session starts         session-start.sh            Inject prior knowledge
Session ends           session-end-maintenance.sh  Harvest learnings
Agent stops            ao-flywheel-close.sh        Close learning loop
Prompt submit         prompt-nudge.sh             Remind missing intent / ratchet state
Pre tool use          pre-mortem-gate.sh          Require review before risky work
Post tool use         go-complexity-precommit.sh  Block over-complex edits
Task complete         task-validation-gate.sh     Execute compiled validation constraints
```

### CLI Command Groups

```
RETRIEVAL / CURATION        VALIDATION / RATCHETS    WORKFLOW / FITNESS
────────────────────        ─────────────────────    ──────────────────
ao lookup                   ao ratchet status        ao rpi phased
ao search                   ao ratchet record        ao rpi status
ao forge                    ao ratchet check         ao goals measure
ao curate                   ao constraint activate   ao goals steer
ao maturity                 ao constraint review     ao flywheel status
ao dedup                    ao session close         ao hooks list
ao contradict               ao temper validate       ao status
ao notebook                                          ao doctor
ao extract
```
