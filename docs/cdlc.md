# Context Development Life Cycle (CDLC)

> **TL;DR:** DevOps gave us the SDLC — a disciplined lifecycle for code. CDLC is the same thing for context. Every phase of the software development lifecycle has a context counterpart. AgentOps implements all of them.

---

## The Parallel

In 2009, DevOps asked: *what if ops looked more like dev?* The answer was CI/CD, infrastructure as code, and the SDLC infinity loop — Plan, Code, Build, Test, Release, Deploy, Operate, Monitor.

CDLC asks the same question about context: *what if the instructions, knowledge, and constraints we feed to coding agents were engineered with the same rigor as the code they produce?*

The answer is the same shape. Different substrate.

```
     SDLC (code)                    CDLC (context)
    ┌──────────┐                   ┌──────────┐
    │   Plan   │                   │ Generate │
    │   Code   │                   │ Compile  │
    │  Build   │                   │   Test   │
    │   Test   │                   │Distribute│
    │ Release  │                   │ Deliver  │
    │  Deploy  │                   │ Observe  │
    │ Operate  │                   │  Adapt   │
    │ Monitor  │                   │          │
    └──────────┘                   └──────────┘
         ↕                              ↕
    infinity loop                  infinity loop
```

The SDLC produces deployable artifacts. The CDLC produces injectable context. Both compound through feedback loops. Both degrade without discipline.

---

## The Seven Phases

### 1. Generate

Create the context that agents will consume. Prompts, skills, instructions, specifications.

| SDLC parallel | Plan + Code |
|---|---|
| **What it means** | Author skills, write agent.md instructions, pull documentation, create specs |
| **Why it matters** | Context that isn't created doesn't exist. Agents start from zero without it. |

**AgentOps implementation:**

- `/research` — investigate before writing context
- `/plan` — decompose goals into structured implementation specs
- SKILL.md authoring — reusable context packages with triggers, steps, and output contracts
- `ao inject --for=<skill>` — pull library documentation into the context window
- MCP integrations — pull context from GitLab, GitHub, Slack, tickets

The generation phase is where most teams stop. They write a Claude.md, maybe a few rules, and call it done. CDLC says generation is one-seventh of the work.

### 2. Compile

Assemble raw context into phase-appropriate, role-scoped, freshness-weighted packets.

| SDLC parallel | Build |
|---|---|
| **What it means** | Select, rank, trim, and package context for the current task |
| **Why it matters** | Raw context is too large, too stale, or too broad. Compilation makes it precise. |

**AgentOps implementation:**

- `ao context assemble` — build phase-scoped context packets
- `ao inject` — retrieve decay-ranked learnings, trim to token budget
- `ao compile` — rebuild the derived knowledge wiki (Mine → Grow → Defrag → Lint)
- `ao maturity --expire/--evict` — remove stale context before it pollutes the window
- Finding compiler — distill raw findings into prevention rules

This is the phase that separates a context compiler from a prompt builder. A prompt builder concatenates. A compiler selects, ranks, trims, and delivers the minimum viable context for the current phase.

### 3. Test

Validate that context produces the intended agent behavior.

| SDLC parallel | Test |
|---|---|
| **What it means** | Run evals on context: does SKILL.md X produce behavior Y? |
| **Why it matters** | You change two lines in your Claude.md. Do you know the impact? |

**AgentOps implementation:**

- `/pre-mortem` — validate plans before implementation (LLM-as-judge)
- `/vibe` — validate code after implementation (multi-model consensus)
- `/council` — multi-judge adversarial review
- `ao eval run` — deterministic eval suites with scoring dimensions
- `context_comprehension` dimension — structural quality assessment of SKILL.md files
- Baseline A/B — skill-on vs skill-off delta measurement

Testing context is fundamentally different from testing code. Evals are non-deterministic. You run them five times and measure pass rate. Error budgets replace pass/fail. This is the hardest phase to get right, and the one most teams skip entirely.

### 4. Distribute

Package and share context across projects, teams, and runtimes.

| SDLC parallel | Release |
|---|---|
| **What it means** | Version context, resolve dependencies, publish to registries |
| **Why it matters** | Context that lives in one person's head (or one repo's Claude.md) doesn't scale. |

**AgentOps implementation:**

- Skills registry — 170+ skills as distributable context packages
- `/converter` — export skills to Cursor rules, Codex format, OpenCode config
- `ao compile` — package the knowledge wiki for distribution
- Cross-runtime compatibility — same skills target Claude Code, Codex CLI, Cursor, and OpenCode
- `install.sh` — one-line installation of the full context package

Distribution is where context becomes an organizational asset. One team fixes a testing pattern, packages it as a skill, and every other team gets the fix on next install.

### 5. Deliver

Inject the right context into the right session at the right time.

| SDLC parallel | Deploy |
|---|---|
| **What it means** | Load context into the agent's window at session start |
| **Why it matters** | A compiled context packet is worthless if it doesn't reach the agent. |

**AgentOps implementation:**

- `SessionStart` hooks — automatic context loading on every session
- `ao inject` — decay-ranked retrieval with token budgeting
- `ao lookup` — on-demand knowledge search during a session
- SkillLoadEvent — track which skills were loaded (citation pipeline)
- Phase-scoped delivery — `/research` gets different context than `/implement`

Delivery is the moment where compilation meets the session. Right context, right window, right time. Phase-specific. Role-scoped. Freshness-weighted.

### 6. Observe

Monitor whether delivered context produces good outcomes.

| SDLC parallel | Operate + Monitor |
|---|---|
| **What it means** | Track agent behavior, capture correction signals, measure session outcomes |
| **Why it matters** | Without observation, context quality is a guess. |

**AgentOps implementation:**

- `quality-signals.sh` — detect user corrections and repeated prompts in real time
- SkillLoadEvent + session-outcome — link "what was loaded" to "how it went"
- Citation tracking — `.agents/ao/citations.jsonl` records every artifact retrieval
- Context monitor — track context window usage and budget
- `ao session-outcome` — compute session reward signal from transcript patterns

Observation is the phase that closes the gap between "we shipped context" and "the context worked." Every PR rejection is feedback on context. Every user correction is a signal. Every production failure in generated code traces back to missing context.

### 7. Adapt

Feed observations back into context improvement. Close the loop.

| SDLC parallel | Feedback → Plan (restart) |
|---|---|
| **What it means** | Use session outcomes to improve context for next session |
| **Why it matters** | Without adaptation, the same context produces the same mistakes forever. |

**AgentOps implementation:**

- MemRL feedback — cited artifacts receive session reward, updating utility scores
- Quality-signal → flywheel wiring — user corrections reduce skill utility
- `ao forge transcript` — extract learnings from completed sessions
- `ao flywheel close-loop` — score, promote, and curate extracted knowledge
- `/evolve` — autonomous reconciliation loop that fixes the worst fitness gap
- `/dream` — overnight compounding that runs the full adapt cycle unattended

Adaptation is where the CDLC becomes a flywheel. Each session's outcomes improve the next session's context. Knowledge that works gets promoted. Knowledge that fails gets demoted. The system compounds.

---

## SDLC → CDLC Mapping Table

| SDLC Phase | CDLC Phase | Key Question | AgentOps Surface |
|---|---|---|---|
| Plan | Generate | What context should exist? | `/research`, `/plan`, SKILL.md |
| Code + Build | Compile | How is context assembled for this task? | `ao context assemble`, `ao inject`, `ao compile` |
| Test | Test | Does this context produce the right behavior? | `/pre-mortem`, `/vibe`, `ao eval run` |
| Release | Distribute | How do others get this context? | Skills registry, `/converter`, `install.sh` |
| Deploy | Deliver | Did the right context reach the agent? | `SessionStart` hooks, `ao inject`, SkillLoadEvent |
| Operate | Observe | Is the context working in practice? | `quality-signals.sh`, citation tracking, session-outcome |
| Monitor → Plan | Adapt | What should change for next time? | MemRL feedback, `/forge`, `/evolve`, `/dream` |

---

## The Leverage Hierarchy

Not all phases are equal. Donella Meadows ranked twelve places to intervene in a system, from weakest (#12: tweak a number) to strongest (#1: change the paradigm). The CDLC phases climb that ladder.

| Leverage | Meadows Point | CDLC Phase | What It Means |
|---|---|---|---|
| Low | #12–#10: Parameters, buffers, structure | **Generate** | Writing a better prompt helps, but it's the lowest-leverage thing you can do. Most teams stop here. |
| Medium | #9–#8: Delays, balancing feedback | **Compile**, **Test** | Assembling the right context and validating it before delivery. Feedback loops that catch errors. |
| Threshold | #6: Information flows | **Distribute**, **Deliver** | Making context available where it's needed. The point where individual effort becomes organizational capability. |
| High | #5: Rules | **Observe** | Measuring what actually happens. Rules that govern what gets promoted, demoted, or discarded. |
| Highest | #4–#3: Self-organization, goals | **Adapt** | The system improves itself. Learnings promote automatically. Goals reconcile. The flywheel compounds without human intervention. |

The pattern: the phases most teams skip are the ones Meadows says matter most. Writing a prompt is #12. Building a system that improves its own context based on what it observes is #4. That's an 8-level leverage gap.

Full leverage-point mapping: [docs/leverage-points.md](leverage-points.md). Convergence map tying each CDLC phase to all five theoretical pillars: [docs/the-science.md](the-science.md#part-6-the-convergence--cdlc-as-the-unifying-spine).

---

## Why This Matters

LLMs are engines. Context is fuel. You can't tune the engine — that's the model vendor's job. But you can engineer the fuel. The CDLC is how.

DevOps proved that disciplined systems around indeterministic workers (humans) produce reliable output. SRE proved it again with SLOs and error budgets. Kubernetes proved it for infrastructure with control loops.

CDLC is the same proof for coding agents. The model stays the same. The context compounds. The system gets better with each use.

---

## See Also

- [The Science](the-science.md) — DevOps Three Ways applied to knowledge flow
- [Context Lifecycle Contract](context-lifecycle.md) — the three-gap internal proof model
- [Knowledge Flywheel](knowledge-flywheel.md) — the six-stage compounding system
- [Scale Without Swarms](scale-without-swarms.md) — why context quality beats agent count
