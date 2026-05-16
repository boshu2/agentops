# AgentOps — One-Page Brief

AgentOps is an SDLC control plane for agentic software development. It keeps the books, compiles context, gates output, and compounds learning so coding agents can work in small, verifiable slices instead of cold one-off prompts.

`.agents/` is the substrate: a wiki of markdown files in your repo, version-controlled with your code, that agents read, traverse, and contribute to. The kind of wiki your team should already have. AgentOps automates the discipline of building one.

*The only verifiable moat in this uncertain time is context. Models will get smarter, harnesses will commoditize, agents will get cheaper. Your accumulated context — the lessons learned about your individual problems, the patterns that worked, the decisions that survived review — is the one asset that compounds and doesn't get eaten by the next vendor release. That's what your company actually is.*

AgentOps is the shovel. Start digging.

---

## What It Is

An SDLC control plane backed by a repo-native, version-controlled, mechanically maintained wiki for your agents.

<!-- agentops:claim:AOP-CLAIM-BRIEF-FOUR-LAYERS -->
AgentOps gives every session four product layers: **Bookkeeping** that records what agents tried and validated, a **Context Compiler** that loads the right repo context before work starts, **Validation Gates** that challenge plans and code before they ship, and a **Knowledge Flywheel** that extracts learnings and feeds them back so the next session starts smarter.

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
the Bookkeeping layer preserves the trace, the Knowledge Flywheel preserves the lesson, and the Context Compiler ensures
the next session loads better context before repeating the mistake.

---

## Four Product Layers

### Layer 0: Bookkeeping
Records the operational memory agents do not keep for themselves: attempts, decisions, citations, verdicts, handoffs, findings, retros, and post-mortems. The work leaves a trace in `.agents/`.

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
| **Local-first** | No AgentOps-managed telemetry or hosted control plane. Model runtimes, Git remotes, installers, and external tools are operator-selected dependencies. |
| **Open source** | Every line auditable. Apache 2.0 licensed. |
| **Multi-tool** | Works with Claude Code, Codex, Cursor, OpenCode. Not locked to one vendor. |
| **Constrained-network fit** | Repo-local evidence and plain files fit mirrored, reviewed, or disconnected operator workflows. |
| **Auditable trail** | Every learning, decision, and review verdict written to `.agents/` with timestamps. |

---

## The Compound Effect

```
Without AgentOps:  [2 hrs] → [2 hrs] → [2 hrs] → [2 hrs]  =  8 hours total
With AgentOps:     [2 hrs] → [10 min] → [2 min] → instant  =  ~2.2 hours total
                    learn     recall     refine    mastered
```

<!-- agentops:claim:AOP-CLAIM-BRIEF-VALIDATED-PATTERNS -->
By session 100, the repo already carries prior failures, design choices, planning rules, and validated patterns that new sessions can load before they repeat old mistakes.

---

## Development Model

The most accurate current framing is:

```text
Public category    -> SDLC control plane for coding agents
Product layers     -> Bookkeeping + Context Compiler + Validation Gates + Knowledge Flywheel
Internal proof     -> three-gap lifecycle contract
Runtime mechanics  -> Brownian Ratchet + Stigmergic Spiral + Knowledge Flywheel
```

The claim is not "better models." The claim is "better repo mechanics around
the models you already have." Four product layers deliver that: Bookkeeping
preserves the evidence trail, the Context Compiler loads the right context,
Validation Gates block bad output, and the Knowledge Flywheel ensures every
session leaves the repo smarter. The
three-gap contract remains the internal proof model.

---

## What if the labs ship this natively?

They will. Anthropic's Managed Agents is the first move; others will follow. That's fine — the value isn't in this tool, it's in the corpus you build with it. AgentOps is bridge infrastructure: your `.agents/` directory is plain markdown in your repo, so if a frontier vendor ships native equivalents in 12 months, your corpus carries forward unchanged.

---

## See also

- [docs/wiki-for-agents.md](wiki-for-agents.md) — the wiki framing as a standalone document.
- [docs/trust-factory.md](trust-factory.md) — AgentOps mapped to the five-step trust factory primitive.

---

*AgentOps — github.com/boshu2/agentops*

---

## Appendix: System Map

### Scale

```
┌──────────────────────────────────────────────────────────────────┐
│                    AgentOps at a Glance                          │
├───────────────────┬──────────────────────┬───────────────────────┤
│ 73 shared skills  │   `ao` Control Plane │   12 Hook Events      │
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
Prompt submit         factory-router.sh           Route explicit factory intake
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
