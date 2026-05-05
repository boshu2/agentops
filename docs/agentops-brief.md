# AgentOps — One-Page Brief

**The problem:** AI coding tools behave like contractors with amnesia. Every session starts from zero — no memory of what broke last week, no record of decisions already made, no awareness of what was tried and abandoned. You brief them today. Tomorrow you brief them again.

---

## What It Is

A repo-native operational layer for coding agents.

AgentOps gives every session three product layers: a **Context Compiler** that loads the right repo context before work starts, **Validation Gates** that challenge plans and code before they ship, and a **Knowledge Flywheel** that extracts learnings and feeds them back so the next session starts smarter.

The institutional knowledge stops walking out the door because the repo keeps it.

---

## Internal Proof Contract

Most coding-agent tooling handles prompt construction and routing well. The failure mode comes after that. Internally, AgentOps proves the product through a three-gap lifecycle contract (see [docs/context-lifecycle.md](context-lifecycle.md)):

| Gap | Problem | AgentOps response |
|-----|---------|-------------------|
| **Validation** (internal: judgment validation) | The agent ships without risk context that would challenge its choices | `/pre-mortem` before implementation, `/vibe` before commit, `/council` for multi-judge review |
| **Bookkeeping** (internal: durable learning) | Solved problems recur because nothing extracts, scores, or retrieves the lesson | `.agents/` ledger, `ao lookup`, finding registry, `/retro` extraction, freshness curation |
| **Closure** (internal: loop closure) | Completed work does not produce better next work | `/post-mortem` harvests learnings and next-work, finding compiler promotes failures into constraints, `GOALS.md` + `/evolve` turn findings into measurable improvements |

The compound effect below only works because Validation Gates catch the problem,
the Knowledge Flywheel preserves the lesson, and the Context Compiler ensures
the next session loads better context before repeating the mistake.

---

## Three Product Layers

### Layer 1: Context Compiler
Assembles the right context for the right phase. Research gets prior knowledge; plan gets a compressed summary; workers get fresh context per wave. Skills, hooks, and the `ao` CLI collaborate to load, scope, and trim context to the token budget before the agent sees it.

### Layer 2: Validation Gates
Challenges plans before build and code before commit. Multi-model councils (`/council`, `/vibe`, `/pre-mortem`) return auditable verdicts — PASS, WARN, or FAIL. Gates block, not advise. Runtime hooks enforce them even when the operator forgets.

### Layer 3: Knowledge Flywheel
Extracts learnings from completed work, scores them for quality, promotes durable patterns, and re-injects them at the next session start. `.agents/` carries state on disk; `ao forge`, `ao lookup`, and maturity controls keep the loop closing.

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
Public category    -> context compiler for coding agents
Product layers     -> Context Compiler + Validation Gates + Knowledge Flywheel
Internal proof     -> three-gap lifecycle contract
Runtime mechanics  -> Brownian Ratchet + Stigmergic Spiral + Knowledge Flywheel
```

The claim is not "better models." The claim is "better repo mechanics around
the models you already have." Three product layers deliver that: the Context
Compiler loads the right context, Validation Gates block bad output, and the
Knowledge Flywheel ensures every session leaves the repo smarter. The
three-gap contract remains the internal proof model.

---

*AgentOps — github.com/boshu2/agentops*

---

## Appendix: System Map

### Scale

```
┌──────────────────────────────────────────────────────────────────┐
│                    AgentOps at a Glance                          │
├───────────────────┬──────────────────────┬───────────────────────┤
│ 66 shared skills  │   `ao` Control Plane │   7 Hook Events       │
│ plus runtime      │ repo-native retrieval│  runtime manifest     │
│    artifacts      │ goals, and automation│                       │
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

### Validation Layer — Everything Flows Through Council

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
Session starts         session-start.sh            Stage runtime state
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
