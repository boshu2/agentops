# Glossary

Project-specific terms used throughout AgentOps documentation.

## Symbols

### `.agents/`
Per-repo local directory where AgentOps stores learnings, plans, findings, handoffs, and run state. Plain text — `git grep`, `diff`, and `git log` all work on it. Ignored by default; set `AGENTOPS_GITIGNORE_AUTO=0` to commit it.

### `MEMORY.md`
Per-repo durable memory file loaded automatically by some runtimes at session start. Compiled by `SessionStart` and `SessionEnd` hooks from `.agents/` artifacts. Primary pointer surface in `AGENTOPS_STARTUP_CONTEXT_MODE=manual`.

## A

### AgentOps
The operational layer for coding agents. Publicly, AgentOps adds bookkeeping, validation, primitives, and flows so sessions compound instead of restarting from zero. Technically, it acts as a context compiler around your existing models and tools. [Full documentation](https://github.com/boshu2/agentops/blob/main/README.md)

### Atomic Work
A unit of work with no shared mutable state with concurrent workers. Pure function model: input (issue spec + codebase snapshot) → output (patch + verification). This isolation property is what enables parallel wave execution — workers cannot interfere with each other. Enforced by fresh context per worker and lead-only commits.

## B

### Beads
Git-native issue tracking system accessed via the `bd` CLI. Issues live in `.beads/` inside your repo and sync through normal git operations — no external service required. [Full documentation](skills/beads.md)

### Bookkeeping
AgentOps' public term for repo-native capture, retrieval, promotion, decay, and resurfacing of what sessions learn. `.agents/`, `/retro`, `/forge`, `/compile`, `ao inject`, and `ao lookup` are all bookkeeping surfaces. [Full documentation](https://github.com/boshu2/agentops/blob/main/README.md#how-bookkeeping-compounds)

### Brownian Ratchet
The core execution model: spawn parallel agents (chaos), validate their output with a multi-model council (filter), and merge passing results to main (ratchet). Progress locks forward — failed agents are discarded cheaply because fresh context means no contamination. [Full documentation](how-it-works.md#the-brownian-ratchet)

## C

### Codex Team
A skill (`/codex-team`) that spawns parallel Codex (OpenAI) execution agents orchestrated by Claude, enabling cross-vendor parallel task execution. [Full documentation](skills/codex-team.md)

### Compact / PreCompact
Runtime event fired when an agent prunes its conversation history. AgentOps uses `precompact-snapshot.sh` to capture signal before compaction so nothing is lost. See [`HOOKS.md`](HOOKS.md).

### Compile
A lifecycle step that rolls session-level signal into durable knowledge. Runs via `compile-session-defrag.sh` at `SessionEnd` and via `ao compile` on demand. Produces the inputs that `ao inject` pulls from.

### Context Compiler
The technical framing for AgentOps. Raw session signal becomes reusable knowledge, compiled prevention, and better next work. The public story is operational layer; the context compiler is the architectural explanation behind it. [Full documentation](https://github.com/boshu2/agentops/blob/main/README.md)

### Context Window
The bounded token budget an agent has in a single session. AgentOps assumes this is always finite and sometimes shrinking under compaction. The Ralph Wiggum Pattern, fresh-context waves, and `PreCompact` snapshots all exist to work around context-window limits rather than fight them.

### Council
The core validation primitive. Spawns independent judge agents (Claude and/or Codex) that review work from different perspectives, deliberate, and converge on a verdict: PASS, WARN, or FAIL. Foundation for `/vibe`, `/pre-mortem`, and `/post-mortem`. [Full documentation](skills/council.md)

### Crank
A skill (`/crank`) that executes an epic by spawning parallel worker agents in dependency-ordered waves. Each worker gets fresh context, writes files, and reports back; the lead validates and commits. Runs until every issue in the epic is closed. [Full documentation](skills/crank.md)

## D

### Discovery
The first phase of the current RPI lifecycle (Discovery → Implementation → Validation). Replaces the older "Research" framing when used at the orchestrator level; `/research` is still the underlying sub-skill.

### Dream (Overnight Run)
A long-haul autonomous run that executes while you are away, emitting morning work packets with evidence, target files, and follow-up commands. Also called an **overnight run** in older docs; both names refer to the same flow.

## E

### Epic
A group of related issues that together accomplish a goal. Created by `/plan`, executed by `/crank`. Each epic has a dependency graph that determines which issues can run in parallel (same wave) and which must wait (later waves). [Full documentation](SKILLS.md#plan)

### Extract
An internal process that pulls learnings, patterns, and decisions from session transcripts and artifacts into structured knowledge files. Now handled by `/forge --promote`. [Full documentation](skills/forge.md)

## F

### FIRE Loop
The reconciliation engine that implements the Brownian Ratchet: **F**ind (read current state), **I**gnite (spawn parallel agents), **R**eap (harvest and validate results), **E**scalate (handle failures and blockers). Used by `/crank` for autonomous epic execution. [Full documentation](brownian-ratchet.md#the-fire-loop)

### Flywheel (Knowledge Flywheel)
The automated loop that extracts learnings from completed work, scores them for quality, and re-injects them at the next session start. Knowledge compounds when retrieval and usage outpace decay and scale friction; otherwise it plateaus until controls improve. [Full documentation](ARCHITECTURE.md#pillar-4-knowledge-flywheel)

### Flywheel Health
A composite measure of whether the knowledge flywheel is actually compounding: retrieval rate, promotion rate, decay rate, and injection hit rate. Surfaced by `ao flywheel` commands and used by `/evolve` to steer improvements.

### Forge
An internal skill that mines session transcripts for knowledge artifacts — decisions, patterns, failures, and fixes — and stores them in `.agents/`. [Full documentation](skills/forge.md)

## G

### Gate
A checkpoint enforced by a hook that blocks progress until a condition is met. For example, the push gate blocks `git push` until `/vibe` has passed, and the pre-mortem gate blocks `/crank` until `/pre-mortem` has passed.

## H

### Harvest
A curation step that pulls learning candidates from recent sessions, scores them, and filters low-confidence output before they enter the flywheel. Invoked via `ao harvest` or inside `/forge --promote`.

### Handoff
A skill (`/handoff`) that creates structured session handoff documents so another agent or future session can continue work with full context. [Full documentation](skills/handoff.md)

### Holdout
An isolated scenario file under `.agents/holdout/` used for behavioral validation. Read/glob/grep access to holdout directories is gated by `holdout-isolation-gate.sh` so validator and evaluee paths do not cross-contaminate. Schema: [`scenario.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/scenario.v1.schema.json).

### Hook
A shell script that fires automatically on agent lifecycle events. AgentOps currently registers 7 hook event sections in `hooks/hooks.json`, spanning session lifecycle, prompt routing, tool-time gates, and task completion. All hooks can be disabled with `AGENTOPS_HOOKS_DISABLED=1`. [Full documentation](HOOKS.md)

## I

### Inject
An internal skill triggered at session start that loads relevant prior knowledge from `.agents/` into the current session context. [Full documentation](skills/inject.md)

### Issue
A discrete unit of trackable work, stored as a bead. Created by `/plan`, executed by `/implement` or `/crank`. Has status, dependencies, and parent/child relationships. [Full documentation](SKILLS.md#beads)

## J

### Judge
An agent in a council that evaluates work from a specific perspective (security, architecture, correctness, etc.). Judges deliberate asynchronously, then the lead consolidates verdicts. [Full documentation](skills/council.md)

## L

### Level
A learning progression stage (L1-L5) that indicates the maturity of a knowledge artifact, from raw observation to validated organizational knowledge. [Full documentation](ARCHITECTURE.md#knowledge-artifacts)

## O

### Operational Invariant
A cross-cutting rule enforced by hooks that applies to all skills and agents. Examples: workers must not commit (lead-only), push blocked until /vibe passes, pre-mortem required for 3+ issue epics. Invariants are not guidelines — they are mechanically enforced. [Full documentation](ARCHITECTURE.md#operational-invariants)

## P

### Pool
A knowledge quality tier — pending, tempered, or promoted. Artifacts start in pending, get tempered through repeated validation and use, and can be promoted to the permanent knowledge base. [Full documentation](ARCHITECTURE.md#knowledge-artifacts)

### Post-mortem
A skill (`/post-mortem`) that runs after work is complete. Convenes a council to validate the implementation, runs a retro to extract learnings, and suggests the next `/rpi` command to continue the improvement loop. [Full documentation](skills/post-mortem.md)

### Pre-mortem
A skill (`/pre-mortem`) that runs before implementation begins. Judges simulate failures against the plan — including spec-completeness checks — and surface problems while they are still cheap to fix. A FAIL verdict sends the plan back for revision. [Full documentation](skills/pre-mortem.md)

### Profile
A documentation grouping for domain-specific workflows and standards. Profiles organize coding standards and validation rules by language or domain. [Full documentation](skills/standards.md)

### Provenance
An internal skill that traces the lineage and sources of knowledge artifacts — where a learning came from, which sessions produced it, and how it was validated. [Full documentation](skills/provenance.md)

## R

### Ralph Wiggum Pattern (Ralph Loop)
The practice of giving every worker agent a fresh context window instead of letting context accumulate across tasks. Named after the [Ralph Wiggum pattern](https://ghuntley.com/ralph/). Each wave spawns new workers with clean context, preventing bleed-through and contamination from prior work. [Full documentation](how-it-works.md#ralph-wiggum-pattern--fresh-context-every-wave)

### Ratchet
A mechanism that locks progress forward so it cannot regress. Once a gate is passed (e.g., vibe validation), the ratchet records that state and hooks enforce it going forward. Combined with the Brownian Ratchet execution model, this ensures quality only moves in one direction. [Full documentation](skills/ratchet.md)

### Research
The first phase of the RPI lifecycle. Deep codebase exploration using Explore agents that produce structured findings in `.agents/research/`. [Full documentation](skills/research.md)

### Retro
A skill (`/retro`) that extracts learnings from completed work — decisions made, patterns discovered, and failures encountered — and feeds them into the knowledge flywheel. Learnings are scored for specificity, actionability, and novelty. [Full documentation](skills/retro.md)

### RPI (Research-Plan-Implement)
The historical name for AgentOps' full lifecycle workflow. In current runtime terms, `/rpi` orchestrates **Discovery -> Implementation -> Validation** while `ao rpi phased` enforces fresh context windows between those phases. The older acronym persists in product language and command names, but validation and loop closure are now first-class parts of the executable lifecycle. [Full documentation](ARCHITECTURE.md#the-phased-lifecycle)

### RPI Phase
One of the three named stages inside an RPI run: **Discovery**, **Implementation**, **Validation**. Each phase gets a fresh context window and emits a [`rpi-phase-result.schema.json`](contracts/rpi-phase-result.schema.json) artifact. Distinct from the broader RPI workflow.

## S

### Session Lifecycle
The full arc of a coding-agent session: `SessionStart` → many `UserPromptSubmit` / `PreToolUse` / `PostToolUse` cycles → `Stop` → `SessionEnd`. AgentOps attaches hooks to each of these events. See [`HOOKS.md`](HOOKS.md) and [`workflows/session-lifecycle.md`](workflows/session-lifecycle.md).

### Skill
A self-contained capability defined by a `SKILL.md` file with YAML frontmatter. Skills are the primary unit of functionality in AgentOps — each one has triggers, instructions, and optional reference docs loaded just-in-time. AgentOps currently ships 66 shared skills, with runtime-specific artifacts maintained alongside them. [Full documentation](SKILLS.md)

### Swarm
A skill (`/swarm`) that spawns parallel worker agents with fresh context. Each wave gets a new team; the lead validates and commits. Workers never commit directly. [Full documentation](skills/swarm.md)

## T

### Tempered
A knowledge quality state indicating an artifact has been validated through multiple uses across sessions. Tempered knowledge has higher confidence than pending and can be promoted to the permanent knowledge base. [Full documentation](ARCHITECTURE.md#knowledge-artifacts)

## V

### Vibe
A skill (`/vibe`) that validates code after implementation by running a council of judges against the changes. Produces a PASS, WARN, or FAIL verdict. A passing vibe is typically required by the push gate before code can be pushed to the remote. [Full documentation](skills/vibe.md)

## W

### Wave
A batch of issues within an epic that can be executed in parallel because they have no dependencies on each other. Waves are ordered by the dependency graph: Wave 1 contains leaf issues, Wave 2 contains issues that depend on Wave 1, and so on. Each wave spawns fresh worker agents. [Full documentation](skills/crank.md)

### Worker
An agent executing a single task in a swarm. Each worker gets fresh context (no bleed-through from other workers), writes files but never commits — the team lead validates and commits. [Full documentation](skills/swarm.md)
