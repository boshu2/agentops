# Glossary

Project-specific terms used throughout AgentOps documentation.

## Symbols

### `.agents/`
Per-repo local directory where AgentOps stores learnings, plans, findings, handoffs, and run state. Plain text for local grep/diff/review. Repo-root `.agents/` is ignored by policy and must not be tracked because it can churn and may contain sensitive session context.

### `MEMORY.md`
Per-repo durable memory pointer for high-value context. In AgentOps 3.0, session startup pulls orientation explicitly with `ao session bootstrap`, then `ao inject` / `ao lookup`; older or opt-in lifecycle hooks may also consume the file, but AgentOps ships none by default.

## A

### AgentOps
A context compiler for coding agents. Three product layers — **Context Compiler**, **Validation Gates**, and **Knowledge Flywheel** — turn raw session signal into better next context so sessions compound instead of restarting from zero. [Full documentation](https://github.com/boshu2/agentops/blob/main/README.md)

### Atomic Work
A unit of work with no shared mutable state with concurrent workers. Pure function model: input (issue spec + codebase snapshot) → output (patch + verification). This isolation property is what enables parallel wave execution — workers cannot interfere with each other. Enforced by fresh context per worker and lead-only commits.

## B

### Beads
Git-native issue tracking system accessed via the `br` CLI (beads_rust). Issues live in `_beads/` inside your repo and sync through normal git operations — no external service required. [Full documentation](../skills/beads-br/SKILL.md)

### Bookkeeping
AgentOps' public term for repo-native capture, retrieval, promotion, decay, and resurfacing of what sessions learn. `.agents/`, `/retro`, `/curate --mode=forge`, `/compile`, `ao inject`, and `ao lookup` are all bookkeeping surfaces. [Full documentation](https://github.com/boshu2/agentops/blob/main/README.md#how-bookkeeping-compounds)

### Brownian Ratchet
The core execution model: spawn parallel agents (chaos), validate their output with a multi-model council (filter), and merge passing results to main (ratchet). Progress locks forward — failed agents are discarded cheaply because fresh context means no contamination. [Full documentation](how-it-works.md#the-brownian-ratchet)

## C

### Codex Team
Parallel Codex (OpenAI) execution agents orchestrated by Claude for cross-vendor parallel task execution — folded into `/swarm` (the retired `/codex-team`, cp-bi2). [Full documentation](../skills/swarm/SKILL.md)

### Compact / PreCompact
Runtime event fired when an agent prunes its conversation history. AgentOps 3.0 is hookless — capture context before compaction with `ao handoff` / `ao inject` rather than a runtime hook.

### Compile
A lifecycle step that rolls session-level signal into durable knowledge. Runs via `compile-session-defrag.sh` at `SessionEnd` and via `ao compile` on demand. Produces the inputs that `ao inject` pulls from.

### Context Compiler
The technical framing for AgentOps. Raw session signal becomes reusable knowledge, compiled prevention, and better next work. The public story is operational layer; the context compiler is the architectural explanation behind it. [Full documentation](https://github.com/boshu2/agentops/blob/main/README.md)

### Context Window
The bounded token budget an agent has in a single session. AgentOps assumes this is always finite and sometimes shrinking under compaction. The Ralph Wiggum Pattern, fresh-context waves, and `PreCompact` snapshots all exist to work around context-window limits rather than fight them.

### Council
The core validation primitive. Spawns independent judge agents (Claude and/or Codex) that review work from different perspectives, deliberate, and converge on a verdict: PASS, WARN, or FAIL. Foundation for `/vibe`, `/pre-mortem`, and `/post-mortem`. [Full documentation](../skills/council/SKILL.md)

### Crank
A skill (`/crank`) that executes an epic by spawning parallel worker agents in dependency-ordered waves. Each worker gets fresh context, writes files, and reports back; the lead validates and commits. Runs until every issue in the epic is closed. [Full documentation](../skills/crank/SKILL.md)

## D

### Discovery
The first phase of the current RPI lifecycle (Discovery → Implementation → Validation). Replaces the older "Research" framing when used at the orchestrator level; `/research` is still the underlying sub-skill.

### Dream (Overnight Run)
A long-haul autonomous run that executes while you are away, emitting morning work packets with evidence, target files, and follow-up commands. Also called an **overnight run** in older docs; both names refer to the same flow.

## E

### Epic
A group of related issues that together accomplish a goal. Created by `/plan`, executed by `/crank`. Each epic has a dependency graph that determines which issues can run in parallel (same wave) and which must wait (later waves). [Full documentation](SKILLS.md#plan)

### Extract
An internal process that pulls learnings, patterns, and decisions from session transcripts and artifacts into structured knowledge files. Now handled by `/curate --mode=forge` (promote step). [Full documentation](../skills/postmortem/SKILL.md)

## F

### FIRE Loop
The reconciliation engine that implements the Brownian Ratchet: **F**ind (read current state), **I**gnite (spawn parallel agents), **R**eap (harvest and validate results), **E**scalate (handle failures and blockers). Used by `/crank` for autonomous epic execution. [Full documentation](brownian-ratchet.md#the-fire-loop)

### Flywheel (Knowledge Flywheel)
The automated loop that extracts learnings from completed work, scores them for quality, and re-injects them at the next session start. Knowledge compounds when retrieval and usage outpace decay and scale friction; otherwise it plateaus until controls improve. [Full documentation](ARCHITECTURE.md#pillar-4-knowledge-flywheel)

### Flywheel Health
A composite measure of whether the knowledge flywheel is actually compounding: retrieval rate, promotion rate, decay rate, and injection hit rate. Surfaced by `ao flywheel` commands and used by `/evolve` to steer improvements.

### Forge
Transcript mining that pulls knowledge artifacts — decisions, patterns, failures, and fixes — into `.agents/`. Folded into `/curate --mode=forge`; the `ao forge` CLI is unchanged. [Full documentation](../skills/postmortem/SKILL.md)

## G

### Gate
A checkpoint that blocks progress until a condition is met. AgentOps 3.0 is hookless: the routine release gate is the local cockpit Go gate (`ao gate check`) run in the pre-push hook before a push to `main`; `validate.yml` is a tag/PR/manual CI backstop. Skill-level gates like `/pre-mortem` and `/vibe` are run explicitly in the operating loop, not auto-fired by runtime hooks.

## H

### Harvest
A curation step that pulls learning candidates from recent sessions, scores them, and filters low-confidence output before they enter the flywheel. Invoked via `ao harvest` or inside `/curate --mode=forge` (promote step).

### Handoff
A skill (`/handoff`) that creates structured session handoff documents so another agent or future session can continue work with full context. [Full documentation](../skills/handoff/SKILL.md)

### Holdout
An isolated scenario file under `.agents/holdout/` used for behavioral validation. Read/glob/grep access to holdout directories is gated by `holdout-isolation-gate.sh` so validator and evaluee paths do not cross-contaminate. Schema: [`scenario.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/scenario.v1.schema.json).

### Hook
A shell script that fires automatically on agent lifecycle events. **AgentOps 3.0 ships zero hooks** — workflow is guided by skills + the `ao` CLI, and the routine release authority is the local cockpit Go gate (`ao gate check`) run in the pre-push hook (with `validate.yml` as a tag/PR/manual CI backstop). If you want runtime hooks, author your own with the `cc-hooks` skill; they are not part of the default product surface.

## I

### Inject
Historical name for session-start knowledge loading. Retired as a skill: on-demand retrieval is `ao lookup --query "<topic>"`, and knowledge activation (beliefs/playbooks/briefings/gaps) lives in the operationalize skill. [Full documentation](../skills/operationalize/SKILL.md)

### Issue
A discrete unit of trackable work, stored as a bead. Created by `/plan`, executed by `/implement` or `/crank`. Has status, dependencies, and parent/child relationships. [Full documentation](SKILLS.md#beads-br)

## J

### Judge
An agent in a council that evaluates work from a specific perspective (security, architecture, correctness, etc.). Judges deliberate asynchronously, then the lead consolidates verdicts. [Full documentation](../skills/council/SKILL.md)

## L

### Level
A learning progression stage (L1-L5) that indicates the maturity of a knowledge artifact, from raw observation to validated organizational knowledge. [Full documentation](ARCHITECTURE.md#knowledge-artifacts)

## O

### Operational Invariant
A cross-cutting rule that applies to all skills and agents. Examples: workers must not commit (lead-only), `main` push blocked until the cockpit gate passes, pre-mortem required for 3+ issue epics. Invariants are not guidelines — in hookless 3.0 they are mechanically enforced by the Go gate registry (`ao gate check`) and explicit `ao` CLI checks. [Full documentation](ARCHITECTURE.md#operational-invariants)

## P

### Pool
A knowledge quality tier — pending, tempered, or promoted. Artifacts start in pending, get tempered through repeated validation and use, and can be promoted to the permanent knowledge base. [Full documentation](ARCHITECTURE.md#knowledge-artifacts)

### Post-mortem
A skill (`/post-mortem`) that runs after work is complete. Convenes a council to validate the implementation, runs a retro to extract learnings, and suggests the next `/rpi` command to continue the improvement loop. [Full documentation](../skills/postmortem/SKILL.md)

### Pre-mortem
A skill (`/pre-mortem`) that runs before implementation begins. Judges simulate failures against the plan — including spec-completeness checks — and surface problems while they are still cheap to fix. A FAIL verdict sends the plan back for revision. [Full documentation](../skills/premortem/SKILL.md)

### Profile
A documentation grouping for domain-specific workflows and standards. Profiles organize coding standards and validation rules by language or domain. [Full documentation](../skills/standards/SKILL.md)

## R

### Ralph Wiggum Pattern (Ralph Loop)
The practice of giving every worker agent a fresh context window instead of letting context accumulate across tasks. Named after the [Ralph Wiggum pattern](https://ghuntley.com/ralph/). Each wave spawns new workers with clean context, preventing bleed-through and contamination from prior work. [Full documentation](how-it-works.md#ralph-wiggum-pattern-fresh-context-every-wave)

### Ratchet
A mechanism that locks progress forward so it cannot regress. Once a gate is passed (e.g., vibe validation), the ratchet records that state and the gate / pawl enforces it going forward. Combined with the Brownian Ratchet execution model, this ensures quality only moves in one direction. [Full documentation](../skills/postmortem/SKILL.md)

### Research
The first phase of the RPI lifecycle. Deep codebase exploration using Explore agents that produce structured findings in `.agents/research/`. [Full documentation](../skills/research/SKILL.md)

### Retro
Quick-capture of learnings from completed work — decisions made, patterns discovered, and failures encountered — fed into the knowledge flywheel and scored for specificity, actionability, and novelty. Folded into `/post-mortem --quick` (the retired `/retro`, cp-bzj). [Full documentation](../skills/postmortem/SKILL.md)

### RPI (Research-Plan-Implement)
The historical name for AgentOps' full lifecycle workflow. In current runtime terms, `/rpi` orchestrates **Discovery -> Implementation -> Validation** while `ao rpi phased` enforces fresh context windows between those phases. The older acronym persists in product language and command names, but validation and loop closure are now first-class parts of the executable lifecycle. [Full documentation](ARCHITECTURE.md#the-phased-lifecycle)

### RPI Phase
One of the three named stages inside an RPI run: **Discovery**, **Implementation**, **Validation**. Each phase gets a fresh context window and emits a [`rpi-phase-result.schema.json`](contracts/rpi-phase-result.schema.json) artifact. Distinct from the broader RPI workflow.

## S

### Session Lifecycle
The full arc of a coding-agent session: `SessionStart` → many `UserPromptSubmit` / `PreToolUse` / `PostToolUse` cycles → `Stop` → `SessionEnd`. AgentOps 3.0 is hookless — it works the lifecycle through skills + the `ao` CLI rather than attaching runtime hooks. See [`workflows/session-lifecycle.md`](workflows/session-lifecycle.md).

### Skill
A self-contained capability defined by a `SKILL.md` file with YAML frontmatter. Skills are the primary unit of functionality in AgentOps — each one has triggers, instructions, and optional reference docs loaded just-in-time. AgentOps currently ships 63 shared skills, with runtime-specific artifacts maintained alongside them. [Full documentation](SKILLS.md)

### Swarm
A skill (`/swarm`) that spawns parallel worker agents with fresh context. Each wave gets a new team; the lead validates and commits. Workers never commit directly. [Full documentation](../skills/swarm/SKILL.md)

## T

### Tempered
A knowledge quality state indicating an artifact has been validated through multiple uses across sessions. Tempered knowledge has higher confidence than pending and can be promoted to the permanent knowledge base. [Full documentation](ARCHITECTURE.md#knowledge-artifacts)

## V

### Validation Gates
The second product layer (Layer 2). Multi-model councils challenge plans before build and code before commit, returning auditable verdicts — PASS, WARN, or FAIL. Gates block progress, not advise. Encompasses `/council`, `/vibe`, `/pre-mortem`, `/post-mortem`, and the local cockpit Go gate (`ao gate check`). Maps to the Validation gap (judgment validation) in the [Context Lifecycle Contract](context-lifecycle.md).

### Vibe
A skill (`/vibe`) that validates code after implementation by running a council of judges against the changes. Produces a PASS, WARN, or FAIL verdict. A passing vibe is typically required by the push gate before code can be pushed to the remote. [Full documentation](../skills/validate/SKILL.md)

## W

### Wave
A batch of issues within an epic that can be executed in parallel because they have no dependencies on each other. Waves are ordered by the dependency graph: Wave 1 contains leaf issues, Wave 2 contains issues that depend on Wave 1, and so on. Each wave spawns fresh worker agents. [Full documentation](../skills/crank/SKILL.md)

### Worker
An agent executing a single task in a swarm. Each worker gets fresh context (no bleed-through from other workers), writes files but never commits — the team lead validates and commits. [Full documentation](../skills/swarm/SKILL.md)
