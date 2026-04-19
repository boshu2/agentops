# Context Lifecycle Contract

> Publicly, AgentOps sells bookkeeping, validation, primitives, and flows.
> This page explains the internal proof gaps those promises have to close:
> validation risk, bookkeeping decay, and loop closure.

## Internal Proof Contract

Most coding-agent tooling is strong at prompt construction and agent routing.
The failure mode comes after that:

1. **Validation** is missing. Internally, this gap is tracked as
   **judgment validation**: the agent chooses an approach without loading the
   risk context that would challenge it before or after implementation.
2. **Bookkeeping** is missing. Internally, this gap is tracked as
   **durable learning**: solved problems come back as if they were never
   solved.
3. **Flows** are present, but they do not compound. Internally, this gap is
   tracked as **loop closure**: completed work does not reliably produce better
   next work, better rules, or better future context.

AgentOps treats those three gaps as one lifecycle contract. Skills and CLI
primitives are the operator surface; the proof obligation is that each cycle
actually closes these gaps.

## Gap 1: Validation

**Problem.** Compile/test checks are not enough. An agent can ship the happy path while missing architecture fit, edge cases, or risk context.

**Observable symptoms:**

- A plan looks coherent but silently picks the wrong middleware stack, abstraction, or integration point
- The implementation passes basic checks but fails on error paths, compatibility edges, or workflow constraints
- Validation happens only after the work is already expensive to unwind

**AgentOps mechanisms:**

| Mechanism | Source | Role |
|-----------|--------|------|
| `/pre-mortem` | [skills/pre-mortem/SKILL.md](skills/pre-mortem.md) | Loads plan-review validation before code exists |
| `/vibe` | [skills/vibe/SKILL.md](skills/vibe.md) | Runs post-implementation validation instead of stopping at build/test |
| `/council` | [skills/council/SKILL.md](skills/council.md) | Supplies multi-judge review for plans and code |
| Pre-mortem gate hook | `hooks/pre-mortem-gate.sh` + [hooks/hooks.json](https://github.com/boshu2/agentops/blob/main/hooks/hooks.json) | Prevents large implementation work from skipping plan validation |
| Task-validation constraint hook | `hooks/task-validation-gate.sh` + `.agents/constraints/index.json` | Task-validation executes active compiled constraints for mechanically detectable findings |
| Product-aware review context | [PRODUCT.md](https://github.com/boshu2/agentops/blob/main/PRODUCT.md) | Injects product and DX perspectives into validation flows |

**Supporting failure modes addressed inside this gap:**

- context contamination inside long sessions
- architecture drift from choosing the wrong existing pattern
- review culture that depends on a human noticing problems after the fact

## Gap 2: Bookkeeping

**Problem.** Notes are not learning. If solved work is not extracted, scored, retrieved, and re-used, the same repo keeps paying for the same lesson.

**Observable symptoms:**

- An auth bug fixed on Monday comes back on Wednesday
- The agent re-runs the same dead-end investigation in a new session
- The repo accumulates artifacts, but not reusable bookkeeping

**AgentOps mechanisms:**

| Mechanism | Source | Role |
|-----------|--------|------|
| `.agents/` ledger | [Knowledge Ledger](#the-knowledge-ledger--session-to-session-flow) | Stores plans, learnings, patterns, council outputs, and next-work artifacts on disk |
| Finding registry | [docs/contracts/finding-registry.md](contracts/finding-registry.md) | Stores reusable structured findings that planning and validation can load before rediscovering the same failure |
| `ao lookup` / injection | [Knowledge Ledger](#the-knowledge-ledger--session-to-session-flow) and `ao` CLI | Retrieves repo-specific context at session start and task boundaries |
| `/retro` and `/post-mortem` extraction | [skills/post-mortem/SKILL.md](skills/post-mortem.md) | Turns completed work into reusable learnings and patterns |
| Freshness / maturity controls | `ao maturity`, `ao dedup`, `ao contradict` | Keeps retrieval focused on useful, current knowledge |
| Compile cycle | [GOALS.md](https://github.com/boshu2/agentops/blob/main/GOALS.md) directive 5 | Mines missed signal, defrags stale knowledge, and flags oscillation |

**Supporting failure modes addressed inside this gap:**

- session amnesia between independent runs
- stale or contradictory learnings swamping retrieval
- bookkeeping systems that store notes without curation or reinforcement

## Gap 3: Closure

**Problem.** A session is not complete when code exists. It is complete when the work has been judged, the learning has been harvested, and the system knows what to do next.

**Observable symptoms:**

- Work ends with a code diff but no extracted lesson
- The next session starts without knowing what the last one changed
- Teams still perform the refinement loop by hand: inspect, restate, retry

**AgentOps mechanisms:**

| Mechanism | Source | Role |
|-----------|--------|------|
| `/post-mortem` | [skills/post-mortem/SKILL.md](skills/post-mortem.md) | Validates shipped work, extracts learnings, and harvests next work |
| Finding registry + compiler path | [docs/contracts/finding-registry.md](contracts/finding-registry.md), [docs/contracts/finding-compiler.md](contracts/finding-compiler.md), `hooks/finding-compiler.sh` | Promotes reusable findings into advisory artifacts and active constraint index entries |
| Task-validation constraint execution | `hooks/task-validation-gate.sh` + `.agents/constraints/index.json` | Turns mechanically detectable findings into enforced validation checks before task completion |
| Flywheel close hook | `hooks/ao-flywheel-close.sh` + [docs/how-it-works.md](how-it-works.md) | Closes the feedback loop at stop time |
| GOALS + `/evolve` | [GOALS.md](https://github.com/boshu2/agentops/blob/main/GOALS.md) and `/evolve` flows | Turns findings into measurable next work instead of leaving them as loose notes |
| Ratchet + run registry | `ao ratchet`, `.agents/rpi/next-work.jsonl` | Records what passed, what remains, and what should be worked next |
| Phase chaining | [README.md](https://github.com/boshu2/agentops/blob/main/README.md) full pipeline | Makes `research -> plan -> pre-mortem -> crank -> post-mortem` the normal operating shape |

**Supporting failure modes addressed inside this gap:**

- knowledge decay after extraction because nothing reuses it
- repeated human triage to decide "what did this teach us?"
- completed work that never becomes better context or better constraints

## Evidence Map

| Gap | Mechanism | Durable Artifact / Contract | Proof Surface |
|-----|-----------|-----------------------------|---------------|
| Validation | `/pre-mortem` | `skills/pre-mortem/SKILL.md` | Plan review before implementation |
| Validation | `/vibe` | `skills/vibe/SKILL.md` | Code review before commit/merge |
| Validation | pre-mortem gate | `hooks/pre-mortem-gate.sh`, `hooks/hooks.json` | Runtime hook enforcement |
| Bookkeeping | extraction + retrieval | `.agents/`, `ao lookup`, `ao forge`, finding registry, finding artifacts | Repo-specific context and reusable structured findings loaded into later sessions |
| Bookkeeping | curation | `ao maturity`, `ao dedup`, `ao contradict` | Freshness, contradiction, and duplication control |
| Bookkeeping | Compile | `GOALS.md`, Compile checks | Daily maintenance of learning quality |
| Closure | `/post-mortem` + finding compiler | `skills/post-mortem/SKILL.md`, `docs/contracts/finding-registry.md`, `docs/contracts/finding-compiler.md` | Learnings + next work harvested from completed work; reusable findings re-enter planning/review and compile into preventive artifacts |
| Closure | task-validation compiled enforcement | `hooks/task-validation-gate.sh`, `.agents/constraints/index.json` | Task-validation executes active compiled constraints before completion is accepted |
| Closure | flywheel close hook | `hooks/ao-flywheel-close.sh` | Stop-time closure of the feedback loop |
| Closure | goals / evolve | `GOALS.md`, flywheel-proof gate | Proof that the system compounds across sessions |

## What AgentOps Does Not Claim

- It does not claim that prompt engineering or routing are unimportant.
- It does not claim that every loop-closing behavior must be fully autonomous.
- It does not claim that raw recall alone is enough; the contract depends on validation, curation, and re-use.
- It does not claim that new runtime machinery should be invented when an existing command, hook, or gate already covers the gap.

## The Knowledge Ledger — Session-to-Session Flow

```
Session N ends
    → ao forge: mine transcript for learnings, decisions, patterns
    → ao notebook update: merge insights into MEMORY.md
    → ao memory sync: sync to repo-root MEMORY.md (cross-runtime)
    → ao maturity --expire: mark stale artifacts (freshness decay ~17%/week)
    → ao maturity --evict: archive what's decayed past threshold
    → ao feedback-loop: citation-to-utility feedback (MemRL)

Session N+1 starts
    → ao lookup (on demand): score artifacts by recency + utility
      ├── Local .agents/ learnings & patterns (1.0x weight)
      ├── Global ~/.agents/ cross-repo knowledge (0.8x weight)
      ├── Work-scoped boost: active issue gets 1.5x (--bead)
      ├── Predecessor handoff: what the last session was doing (--predecessor)
      └── Trim to ~1000 tokens — lightweight, not encyclopedic
    → Agent starts where the last one left off
```

Three tiers, descending priority: local `.agents/` → global `~/.agents/` → legacy `~/.claude/patterns/`. Each session starts with a small, curated packet — not a data dump. If the task needs deeper context, the agent searches `.agents/` on demand.

## See Also

- [README.md](https://github.com/boshu2/agentops/blob/main/README.md) for the repo-level overview
- [How It Works](how-it-works.md) for runtime mechanics and hook behavior
- [Knowledge Flywheel](knowledge-flywheel.md) for extraction, retrieval, and compounding details
- [The Science](the-science.md) for the formal decay/escape-velocity model
