---
name: using-agentops
description: Explain AgentOps workflows.
practices:
- wiki-knowledge-surface
- pragmatic-programmer
- agile-manifesto
hexagonal_role: generic
consumes: []
produces:
- documentation
context_rel: []
skill_api_version: 1
user-invocable: false
context:
  window: isolated
  intent:
    mode: none
  sections:
    exclude:
    - HISTORY
    - INTEL
    - TASK
  intel_scope: none
metadata:
  tier: meta
  dependencies: []
  internal: true
output_contract: 'stdout: operating guide'
---
# AgentOps Operating Model

AgentOps is the operational layer for coding agents.

Publicly, it gives you four things:

- **Bookkeeping** — captured learnings, findings, and reusable context
- **Validation** — plan and code review before work ships
- **Primitives** — single skills, hooks, and CLI surfaces
- **Flows** — named compositions like `/research`, `/validate`, and `rpi`

Technically, AgentOps acts as a context compiler: raw session signal becomes reusable knowledge, compiled prevention, and better next work.

## Core Flow: RPI

```
Research → Plan → Implement → Validate
    ↑                            │
    └──── Knowledge Flywheel ────┘
```

### Research Phase

```bash
/research <topic>      # Deep codebase exploration
ao search "<query>"    # Search existing knowledge
ao search "<query>" --cite retrieved  # Record adoption when a search result is reused
ao lookup <id>         # Pull full content of specific learning
ao lookup --query "x"  # Search knowledge by relevance
```

**Output:** `.agents/research/<topic>.md`

### Plan Phase

```bash
/pre-mortem <spec>     # Simulate failures (error/rescue map, scope modes, prediction tracking)
/plan <goal>           # Decompose into trackable issues
```

**Output:** Beads issues with dependencies

### Implement Phase

```bash
/implement <issue>     # Single issue execution
/crank <epic>          # Autonomous epic loop (uses swarm for waves)
/swarm                 # Parallel execution (fresh context per agent)
```

**Output:** Code changes, tests, documentation

### Validate Phase

```bash
/vibe [target]         # Code validation (finding classification + suppression + domain checklists)
/post-mortem           # Validation + streak tracking + prediction accuracy + retro history
/post-mortem --quick   # Quick-capture a single learning (folded the retired retro lane)
```

**Output:** `.agents/learnings/`, `.agents/patterns/`

## Phase-to-Skill Mapping

| Phase | Primary Skill | Supporting Skills |
|-------|---------------|-------------------|
| **Discovery** | `/discovery` | `/brainstorm`, `/research`, `/plan`, `/pre-mortem` |
| **Implement** | `/crank` | `/implement` (single issue), `/swarm` (parallel execution) |
| **Validate** | `/validate` | `/vibe`, `/post-mortem`, `/forge` |

**Choosing the skill:**
- Use `/implement` for **single issue** execution. **Now defaults to TDD-first** — writes failing tests before implementing. Skip with `--no-tdd`.
- Use `/crank` for **autonomous epic execution** (loops waves via swarm until done). Auto-generates file-ownership maps to prevent worker conflicts.
- Use `/discovery` for the **discovery phase only** (brainstorm → search → research → plan → pre-mortem).
- Use `/validate` for the **validation phase only** (vibe → post-mortem → forge).
- Use `rpi` for **full lifecycle** — delegates to `/discovery` → `/crank` → `/validate`.
- Use `/ratchet` to **gate/record progress** through RPI.

## Start Here (12 starters)

These are the skills every user needs first. Everything else is available when you need it.

| Skill | Purpose |
|-------|---------|
| `/quickstart` | Guided onboarding — run this first |
| `/bootstrap` | One-command full AgentOps setup — fills gaps only |
| `/research` | Deep codebase exploration |
| `/council` | Multi-model consensus review + finding auto-extraction |
| `/validate` | Canonical PASS/WARN/FAIL verdict over an artifact, plan, code change, PR, or gate |
| `/vibe` | Code validation (classification + suppression + domain checklists) |
| `rpi` | Full RPI lifecycle orchestrator (`/discovery` → `/crank` → `/validate`) |
| `/implement` | Execute single issue |
| `/post-mortem --quick` | Quick-capture a single learning into the flywheel |
| `/status` | Single-screen dashboard of current work and suggested next action |
| `/goals` | Maintain GOALS.yaml fitness specification |
| `/push` | Atomic test-commit-push workflow |

## Advanced Skills (when you need them)

| Skill | Purpose |
|-------|---------|
| `/compile`, `/flywheel` | Active knowledge intelligence and flywheel health — Mine → Grow → Defrag cycle |
| `/curate` | Canonical miner role for transcripts, `.agents/`, bd, git, skill diffs, and rare wiki entries |
| `/research-software` | External reading and research synthesis for software topics |
| `/curate --mode=harvest` | Cross-rig knowledge consolidation — sweep, dedup, promote to global hub (folded the retired harvest lane) |
| `/inject` | Operationalize a mature `.agents` corpus into beliefs, playbooks, briefings, and gap surfaces (folded the retired knowledge-activation lane) |
| `/bd-first-memory-migration` | Consolidate fragmented agent-memory layers onto a bd-canonical store, then GC/retire the rest |
| `/brainstorm` | Structured idea exploration before planning |
| `/discovery` | Full discovery phase orchestrator (brainstorm → search → research → plan → pre-mortem) |
| `/plan` | Epic decomposition into issues |
| `/design` | Product validation gate — goal alignment, persona fit, competitive differentiation |
| `/pre-mortem` | Failure simulation (error/rescue, scope modes, temporal, predictions) |
| `/post-mortem` | Validation + streak tracking + prediction accuracy + retro history |
| `/bug-hunt` | Root cause analysis |
| `/release` | Pre-flight, changelog, version bumps, tag |
| `/crank` | Autonomous epic loop (uses swarm for each wave) |
| `/swarm` | Fresh-context parallel execution (Ralph pattern) |
| `/using-ntm` | Run AgentOps loops out of session on an NTM tmux swarm (the NTM leg of the substrate) |
| `evolve` | Goal-driven fitness-scored improvement loop |
| `/burndown` | Bounded epic-completion loop — drive a finite target to all-merged, then stop |
| `/eval-outcomes` | Grade via Outcomes as a holdout-safe projection of the locked eval substrate — one bar, many runtimes |
| `/operating-loop-workflow` | Install + run the operating-loop multi-agent Workflow (seven-move loop) |
| `/autodev` | PROGRAM.md autonomous development contract setup and validation |
| `agy-rules-workflows` | Install AGY-native rules, loop workflow, and scheduled goal controls |
| `/curate --mode=dream` | Out-of-session knowledge compounding lane |
| `/doc` | Documentation generation — repo docs (default), gold-standard README (`--mode=readme`), OSS doc packs (`--mode=oss`) |
| `/post-mortem --quick` | Quick-capture a learning (folded the retired retro lane) |
| `/validate` | Full validation phase orchestrator (vibe → post-mortem → forge) |
| `/ratchet` | Brownian Ratchet progress gates for RPI workflow |
| `/forge` | Mine transcripts for knowledge — decisions, learnings, patterns |
| `/security` | Repository security scanning and release gating, plus the composable binary/prompt-surface suite — static analysis, dynamic tracing, offline redteam, policy gating |
| `/test` | Test generation, coverage analysis, and TDD workflow |
| `/cc-hooks` | Author and validate AgentOps runtime hook behavior |
| `/red-team` | Persona-based adversarial validation — probe docs and skills from constrained user perspectives |
| `/review` | Review incoming PRs, agent output, or diffs — SCORED checklist |
| `/refactor` | Safe, verified refactoring with regression testing at each step |
| `/deps` | Dependency audit, update, vulnerability scanning, and license compliance |
| `/perf` | Performance profiling, benchmarking, regression detection, and optimization |
| `/system-tuning` | Restore system responsiveness via safe, ordered process cleanup and agent-swarm hygiene |
| `/scaffold` | Project scaffolding, component generation, and boilerplate setup |
| `/scenario` | Author and manage holdout scenarios for behavioral validation |
| `/skill-auditor` | Two-pass audit of an existing SKILL.md against the unified template (15 checks) |
| `/skill-builder` | Scaffold or absorb new SKILL.md files against the unified template |
| `/automation-shape-routing` | Front door for building agent automation — decide the SHAPE (Workflow vs NTM swarm vs plain skill), then hand off to the right builder |
| `/workflow-builder` | Scaffold a new Claude Workflow script (`.claude/workflows/*.js`) — deterministic multi-agent orchestration |
| `/agent-native` | Make out-of-session agents AgentOps-native via skills + ao CLI + CI, not hooks |

## Expert Skills (specialized workflows)

| Skill | Purpose |
|-------|---------|
| `/swarm` | Parallel Codex agent execution (folded the retired codex-team lane) |
| `/external-search-triage` | External docs and source lookup with citations |
| `/reverse-engineer-rpi` | Reverse-engineer a product into feature catalog and specs |
| `/pr-research` | Upstream repository research before contribution |
| `/pr-implement` | Fork-based PR implementation |
| `/validate --mode=pr` | PR-specific validation and isolation checks (folded the retired pr-validate lane) |
| `/pr-prep` | PR preparation and structured body generation |
| `/ship-loop` | Bot-paired internal-PR fast-lane cycle |
| `/complexity` | Code complexity analysis |
| `/product` | Interactive PRODUCT.md generation |
| `/handoff` | Session handoff for continuation |
| `/recover` | Post-compaction context recovery |
| `/session-bootstrap` | Universal init prompt — every agent runs this first (soc-vuu6.25) |
| `/trace` | Trace design decisions through history |
| `/curate --mode=provenance` | Trace artifact lineage to sources |
| `/beads` | Issue tracking operations |
| `/heal-skill` | Detect and fix skill hygiene issues |
| `/converter` | Convert skills to Codex/Cursor formats |

**To update installed skills:** re-run the install one-liner — `bash <(curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install.sh)`. (There is no update skill; skill refresh is an install-script concern.)

## Knowledge Flywheel

Every `/post-mortem` promotes learnings and patterns into `.agents/` so future `/research` starts with better context instead of zero.

Inspect, lint, and triage the `.agents/` write surface contract via `ao agents inspect | lint | doctor` (`doctor` rolls up inspect + lint + orphan/stray-dir report; `--strict` fails on orphans).

## Runtime Modes

AgentOps has several runtime modes. Do not assume hook automation exists everywhere.

| Mode | When it applies | Start path | Closeout path | Guarantees |
|------|-----------------|------------|---------------|------------|
| `substrate` (out-of-session) | A swappable orchestration substrate available out-of-session: an NTM tmux swarm, MCP (`ao mcp serve`), or managed-agents (`ao agent`) | The operator or a lead agent runs `bd ready` and dispatches a whole loop per bead — an agent that runs the `rpi` skill; cron / managed triggers run maintenance | The substrate owns the merge gate (CI-green is the signal) and triggers the knowledge-flywheel feedback | The substrate orchestrates *whole* `rpi`/`evolve` loops (each an agent running the skill) — it never sees the loop's insides; the seam is substrate → agent-running-the-skill. There is no in-CLI `runtime=gc` executor. See [agent-native](../agent-native/SKILL.md) and [docs/3.0.md](https://github.com/boshu2/agentops/blob/main/docs/3.0.md). |
| `hook-capable` | Claude/OpenCode with lifecycle hooks installed (no gc) | Runtime hook or `ao inject` / `ao lookup` | Runtime hook or `ao forge transcript` + `ao flywheel close-loop` | Automatic startup/context injection and session-end maintenance when hooks are installed |
| `codex-native-hooks` | Codex CLI v0.115.0+ with native hook support (March 2026) | Runtime hooks (same as hook-capable) | Runtime hooks (same as hook-capable) | Native lifecycle hooks — same guarantees as hook-capable mode |
| `codex-hookless-fallback` | Codex Desktop / Codex CLI pre-v0.115.0 without hook surfaces | `ao codex start` | `ao codex stop` | Explicit startup context, citation tracking, transcript fallback, and close-loop metrics without hooks |
| `manual` | No hooks and no Codex-native runtime detection | `ao inject` / `ao lookup` | `ao forge transcript` + `ao flywheel close-loop` | Works everywhere, but lifecycle actions are operator-driven |

## Issue Tracking

This workflow uses beads for git-native issue tracking:

```bash
bd ready              # Unblocked issues
bd show <id>          # Issue details
bd close <id>         # Close issue
bd vc status          # Inspect Dolt state if needed (JSONL auto-sync is automatic)
```

## Examples

**Startup context loading.** AgentOps 3.0's default path is explicit: run `ao session bootstrap`, then pull prior context with `ao inject` / `ao lookup` or a phase-scoped packet. Optional hook-capable runtimes may run an authored `session-start.sh`, and Codex CLI can use opt-in native hooks via `install-codex.sh --with-hooks`; those are compatibility/adaptor paths, not the default. Either way the agent gets the RPI workflow, prior context, and a citation path.

**Workflow reference during planning.** When a user asks how to approach a feature, the agent uses this skill's RPI section to recommend Research → Plan → Implement → Validate — `/research` for exploration, `/plan` for decomposition, `/pre-mortem` for failure simulation — instead of an ad-hoc approach.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Skill not loaded | Startup path not run | Run `ao session bootstrap`, then `ao inject` / `ao lookup`; for Codex lifecycle recovery, run `ao codex start` explicitly |
| Outdated skill catalog | This file not synced with actual skills/ directory | Update skill list in this file after adding/removing skills |
| Wrong skill suggested | Natural language trigger ambiguous | User explicitly calls skill with `/skill-name` syntax |
| Workflow unclear | RPI phases not well-documented here | Read full workflow guide in README.md or docs/ARCHITECTURE.md |
