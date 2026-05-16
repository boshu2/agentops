# The Seed

> The product is not a pile of skills. The product is the minimal set of starting conditions that, planted in any repo with an LLM, evolves toward whatever that repo's goals are.

## The Metaphor

DNA. The same genetic machinery -- the same 4 nucleotides, the same ribosomal translation, the same replication fork -- produces bacteria and blue whales. The difference is not the machinery. The difference is the fitness landscape.

AgentOps is the same. The same core seed, planted in a Go CLI repo or a Python web app or a Kubernetes operator, produces different systems. The goals define the fitness landscape. The seed provides the machinery. Evolution does the rest.

## The Core Seed

```
1. GOALS.md           -- what to optimize toward
2. .agents/           -- where knowledge accumulates
3. Hook readiness     -- optional rules that enforce the flywheel
4. CLAUDE.md section  -- instructions that start the flywheel
5. Core skills        -- the capabilities the agent can invoke
6. Bootstrap learning -- the first turn of the flywheel
```

Apply or repair the core seed with the golden path:

```bash
ao quick-start
# alias: ao quickstart
```

Optional layers are deliberately separate: `/bootstrap` adds PRODUCT.md,
README.md, and PROGRAM.md/AUTODEV.md; `bd init --prefix <prefix>` adds
tracking; `ao init --hooks` explicitly activates optional hooks; `ao init --with-schedule` opts
into continuous scheduling.

### 1. GOALS.md -- Fitness Specification

A file in the repo root with 2-3 directives and 5-7 gates. Each gate has a shell command that exits 0 (pass) or non-zero (fail). No soft measurement. No subjective scoring. Hard constraints only.

```markdown
## Directives
### 1. Increase test coverage
### 2. Reduce complexity hotspots

## Gates
| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| tests-pass | cd src && make test | 8 | All tests pass |
| coverage-floor | ./scripts/check-coverage.sh --min=70 | 6 | Coverage above 70% |
| lint-clean | cd src && make lint | 5 | No lint violations |
```

**Why it exists:** Goals are Meadows #3 -- system intent. Without goals, `/evolve` has nothing to measure, severity-based selection has nothing to sort, and regression gates have nothing to protect. Goals make the system directional instead of random.

**Meadows mapping:** #3 (goals), #8 (balancing feedback via regression gates that protect passing goals).

### 2. `.agents/` Directory -- Knowledge Flywheel Structure

An append-only ledger with cache-like semantics. Nothing gets overwritten. Every learning, council verdict, pattern, and decision is a new dated file. Freshness decay prunes what is stale. Git-ignored by default (local knowledge, not source code).

```
.agents/
  learnings/     -- extracted lessons (gold/silver/bronze tiers)
  patterns/      -- reusable solutions
  retros/        -- retrospective summaries
  council/       -- validation verdicts
  research/      -- exploration findings
  plans/         -- decomposed epics
  ao/            -- session index, provenance, metrics
```

**Why it exists:** The flywheel needs a place to write. Without `.agents/`, `ao forge` has nowhere to put learnings, `ao lookup` has nothing to retrieve, and knowledge dies with each session. This is the physical `K` stock from the equation.

**Meadows mapping:** #10 (material stocks), #7 (reinforcing feedback loop -- more knowledge enables better retrieval enables more knowledge).

### 3. Hook Readiness -- Optional Rule Configuration

Hooks that fire on session lifecycle events when activated. The minimum viable set:

| Event | Hook | What it does |
|-------|------|--------------|
| SessionStart | session-start | Load context signpost (`.agents/AGENTS.md`), clean stale state |
| SessionEnd | session-end | Extract learnings (`ao forge`), expire stale artifacts |
| Stop | stop | Close the feedback loop (`ao flywheel close-loop`) |

**Why it exists:** Hooks are Meadows #5 -- structural rules. Without hooks, knowledge extraction depends on the agent remembering to run `ao forge`. Agents forget. Hooks do not. The core seed keeps hook activation optional so setup is safe in any repo; `ao init --hooks` or `install-codex.sh --with-hooks` turns on structural enforcement when the operator is ready.

**Meadows mapping:** #5 (rules), #6 (information flows -- hooks ensure knowledge moves from session output to persistent storage to next session input).

### 4. CLAUDE.md Seed Section -- Flywheel Bootstrap Instructions

Two lines added to the repo's CLAUDE.md:

```markdown
## Knowledge Flywheel
See `.agents/AGENTS.md` for orientation. Run `ao lookup --query "topic"` when you need prior knowledge. Run `ao forge` at session end.
```

**Why it exists:** Hooks handle the automation, but CLAUDE.md provides the fallback for environments where hooks are not configured and the explanation for environments where they are. It bridges the gap between "hooks fire automatically" and "the agent understands why." This is belt-and-suspenders: structural enforcement (hooks) plus cognitive priming (instructions).

**Meadows mapping:** #6 (information flows -- ensures the agent is aware of the flywheel even if hooks are not installed).

### 5. Core Skills -- Agent Capabilities

Four skills installed globally, available in any repo:

| Skill | Role | Meadows level |
|-------|------|---------------|
| `/evolve` | Measure goals, fix worst gap, validate, learn, repeat | #4 (self-organization) |
| `/research` | Explore codebase, surface findings, inject prior knowledge | #7 (reinforcing feedback) |
| `/implement` | Full lifecycle for one task: plan, build, validate, learn | #8 (balancing feedback) |
| `/vibe` | Code quality review with multi-model council | #8 (balancing feedback) |

**Why it exists:** The seed needs agency. GOALS.md defines intent, `.agents/` stores knowledge, hooks enforce rules -- but without skills, the agent has no structured way to act on goals, validate work, or extract learnings. Skills are the verbs that operate on the nouns.

**Meadows mapping:** #4 (self-organization -- `/evolve` changes the system's own rules based on measured fitness).

### 6. Bootstrap Learning -- First Turn of the Flywheel

One learning file created at seed time:

```
.agents/learnings/YYYY-MM-DD-seed-bootstrap.md
---
type: decision
confidence: high
tags: [seed, bootstrap]
---
This repo was seeded on DATE with goals: GOAL_1, GOAL_2, GOAL_3.
Initial state: SUMMARY. Run /evolve to begin improvement.
```

**Why it exists:** A flywheel with zero learnings is a flywheel that has never turned. The bootstrap learning ensures `ao lookup` has something to retrieve on the very first session. It primes the reinforcing loop (Meadows #7) so the system starts compounding immediately instead of running one empty cycle first.

**Meadows mapping:** #7 (reinforcing feedback -- the initial push that starts the flywheel turning).

---

## The Fractal Property

The same pattern operates at every scale:

```
attempt -> validate -> learn -> constrain
```

| Scale | Attempt | Validate | Learn | Constrain |
|-------|---------|----------|-------|-----------|
| Single function | Write code | Run tests | Extract pattern | Add test |
| Single issue | `/implement` | `/vibe` | `/retro` | Close issue |
| Epic | `/crank` (waves) | Council consensus | `/post-mortem` | Regression gate |
| Repository | `/evolve` (cycles) | Goal measurement | Learning extraction | Constraint compiler |

Each level treats the one below as a black box. Each level produces the same outputs: validated work + extracted knowledge + tighter constraints. The seed does not prescribe which level you operate at. It provides the machinery. The fitness landscape determines the scale.

---

## Why Exactly 6 Elements

| Element | Without it | The system is... |
|---------|-----------|-----------------|
| GOALS.md | No fitness function | Random (no direction) |
| `.agents/` | No knowledge storage | Memoryless (no compounding) |
| Hooks | No automatic enforcement | Fragile (depends on agent memory) |
| CLAUDE.md section | No cognitive priming | Opaque (agent does not understand why) |
| Core skills | No structured agency | Passive (cannot act on goals) |
| Bootstrap learning | No initial flywheel state | Cold-start (first cycle is empty) |

Remove any one element and the system degrades to a qualitatively different thing. Add more elements and you are designing a specific system rather than providing starting conditions for an evolved one. The constraint is intentional: the seed must be small enough to fit on one screen, complete enough to produce emergence.

---

## What the Seed Is Not

- **Not a template.** Templates produce identical copies. Seeds produce different systems depending on goals.
- **Not a framework.** Frameworks constrain what you can build. The seed constrains how the system learns, not what it builds.
- **Not configuration.** Configuration tunes parameters (Meadows #12). The seed operates at the level of rules (#5), self-organization (#4), and goals (#3).

---

## See Also

- [strategic-direction.md](strategic-direction.md) -- consolidation decision, Meadows mapping, paradigm shifts
- [the-science.md](the-science.md) -- the dK/dt equation and escape velocity condition
- [how-it-works.md](how-it-works.md) -- operational mechanics (hooks, ratchet, flywheel)
- [ARCHITECTURE.md](ARCHITECTURE.md) -- the 5 architectural pillars
