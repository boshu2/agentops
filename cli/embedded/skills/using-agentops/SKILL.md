---
name: using-agentops
description: 'Meta skill explaining the RPI workflow. Hook-capable runtimes inject it at session start; Codex uses it through the explicit startup fallback. Covers Research-Plan-Implement workflow, Knowledge Flywheel, and skill catalog.'
skill_api_version: 1
user-invocable: false
context:
  window: isolated
  intent:
    mode: none
  sections:
    exclude: [HISTORY, INTEL, TASK]
  intel_scope: none
metadata:
  tier: meta
  dependencies: []
  internal: true
---

# RPI Workflow

You have access to workflow skills for structured development.

## The RPI Workflow

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
/retro                 # Quick-capture a single learning
```

**Output:** `.agents/learnings/`, `.agents/patterns/`

### Release Phase

```bash
/release [version]     # Full release: changelog + bump + commit + tag
/release --check       # Readiness validation only (GO/NO-GO)
/release --dry-run     # Preview without writing
```

**Output:** Updated CHANGELOG.md, version bumps, git tag, `.agents/releases/`

## Phase-to-Skill Mapping

| Phase | Primary Skill | Supporting Skills |
|-------|---------------|-------------------|
| **Discovery** | `/discovery` | `/brainstorm`, `/research`, `/plan`, `/pre-mortem` |
| **Implement** | `/crank` | `/implement` (single issue), `/swarm` (parallel execution) |
| **Validate** | `/validation` | `/vibe`, `/post-mortem`, `/retro`, `/forge` |
| **Release** | `/release` | — |

**Choosing the skill:**
- Use `/implement` for **single issue** execution. **Now defaults to TDD-first** — writes failing tests before implementing. Skip with `--no-tdd`.
- Use `/crank` for **autonomous epic execution** (loops waves via swarm until done). Auto-generates file-ownership maps to prevent worker conflicts.
- Use `/swarm` directly for **parallel execution** without beads (TaskList only).
- Use `/discovery` for the **discovery phase only** (brainstorm → search → research → plan → pre-mortem).
- Use `/validation` for the **validation phase only** (vibe → post-mortem → retro → forge).
- Use `/rpi` for **full lifecycle** — delegates to `/discovery` → `/crank` → `/validation`.
- Use `/ratchet` to **gate/record progress** through RPI.

## Available Skills

## Start Here (11 starters)

These are the skills every user needs first. Everything else is available when you need it.

| Skill | Purpose |
|-------|---------|
| `/quickstart` | Guided onboarding — run this first |
| `/research` | Deep codebase exploration |
| `/council` | Multi-model consensus review + finding auto-extraction |
| `/vibe` | Code validation (classification + suppression + domain checklists) |
| `/rpi` | Full RPI lifecycle orchestrator (research → plan → implement → validate) |
| `/implement` | Execute single issue |
| `/retro --quick` | Quick-capture a single learning into the flywheel |
| `/status` | Single-screen dashboard of current work and suggested next action |
| `/goals` | Maintain GOALS.yaml fitness specification |
| `/push` | Atomic test-commit-push workflow |
| `/flywheel` | Knowledge flywheel health monitoring (σ×ρ > δ/100) |

## Advanced Skills (when you need them)

| Skill | Purpose |
|-------|---------|
| `/athena` | Active knowledge intelligence — Mine → Grow → Defrag cycle |
| `/brainstorm` | Structured idea exploration before planning |
| `/discovery` | Full discovery phase orchestrator (brainstorm → search → research → plan → pre-mortem) |
| `/plan` | Epic decomposition into issues |
| `/pre-mortem` | Failure simulation (error/rescue, scope modes, temporal, predictions) |
| `/post-mortem` | Validation + streak tracking + prediction accuracy + retro history |
| `/bug-hunt` | Root cause analysis |
| `/release` | Pre-flight, changelog, version bumps, tag |
| `/crank` | Autonomous epic loop (uses swarm for each wave) |
| `/swarm` | Fresh-context parallel execution (Ralph pattern) |
| `/evolve` | Goal-driven fitness-scored improvement loop |
| `/doc` | Documentation generation |
| `/retro` | Quick-capture a learning (full retro → /post-mortem) |
| `/validation` | Full validation phase orchestrator (vibe → post-mortem → retro → forge) |
| `/ratchet` | Brownian Ratchet progress gates for RPI workflow |
| `/forge` | Mine transcripts for knowledge — decisions, learnings, patterns |
| `/readme` | Generate gold-standard README for any project |
| `/security` | Continuous repository security scanning and release gating |
| `/security-suite` | Binary and prompt-surface security suite — static analysis, dynamic tracing, offline redteam, policy gating |
| `/test` | Test generation, coverage analysis, and TDD workflow |
| `/review` | Review incoming PRs, agent output, or diffs — SCORED checklist |
| `/refactor` | Safe, verified refactoring with regression testing at each step |
| `/deps` | Dependency audit, update, vulnerability scanning, and license compliance |
| `/perf` | Performance profiling, benchmarking, regression detection, and optimization |
| `/scaffold` | Project scaffolding, component generation, and boilerplate setup |

## Expert Skills (specialized workflows)

| Skill | Purpose |
|-------|---------|
| `/grafana-platform-dashboard` | Build Grafana platform dashboards from templates/contracts |
| `/codex-team` | Parallel Codex agent execution |
| `/openai-docs` | Official OpenAI docs lookup with citations |
| `/oss-docs` | OSS documentation scaffold and audit |
| `/reverse-engineer-rpi` | Reverse-engineer a product into feature catalog and specs |
| `/pr-research` | Upstream repository research before contribution |
| `/pr-plan` | External contribution planning |
| `/pr-implement` | Fork-based PR implementation |
| `/pr-validate` | PR-specific validation and isolation checks |
| `/pr-prep` | PR preparation and structured body generation |
| `/pr-retro` | Learn from PR outcomes |
| `/complexity` | Code complexity analysis |
| `/product` | Interactive PRODUCT.md generation |
| `/handoff` | Session handoff for continuation |
| `/recover` | Post-compaction context recovery |
| `/trace` | Trace design decisions through history |
| `/provenance` | Trace artifact lineage to sources |
| `/beads` | Issue tracking operations |
| `/heal-skill` | Detect and fix skill hygiene issues |
| `/converter` | Convert skills to Codex/Cursor formats |
| `/update` | Reinstall all AgentOps skills from latest source |

## Knowledge Flywheel

Every `/post-mortem` feeds back to `/research`:

1. **Learnings** extracted → `.agents/learnings/`
2. **Patterns** discovered → `.agents/patterns/`
3. **Research** enriched → Future sessions benefit

## Runtime Modes

AgentOps has three runtime modes. Do not assume hook automation exists everywhere.

| Mode | When it applies | Start path | Closeout path | Guarantees |
|------|-----------------|------------|---------------|------------|
| `hook-capable` | Claude/OpenCode with lifecycle hooks installed | Runtime hook or `ao inject` / `ao lookup` | Runtime hook or `ao forge transcript` + `ao flywheel close-loop` | Automatic startup/context injection and session-end maintenance when hooks are installed |
| `codex-hookless-fallback` | Codex Desktop / Codex CLI without hook surfaces | `ao codex start` | `ao codex stop` | Explicit startup context, citation tracking, transcript fallback, and close-loop metrics without hooks |
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

### Startup Context Loading

**Hook-capable runtimes**
1. `session-start.sh` (or equivalent) can run at session start.
2. In `manual` mode, MEMORY.md is auto-loaded and the hook points to on-demand retrieval (`ao search`, `ao lookup`).
3. In `lean` mode, the hook extracts pending knowledge and injects prior learnings with a reduced token budget.
4. This skill can be injected automatically into session context.

**Codex hookless fallback**
1. Run `ao codex start`.
2. AgentOps inspects `.agents/`, runs safe close-loop maintenance, syncs MEMORY.md, and writes `.agents/ao/codex/startup-context.md`.
3. Surfaced learnings, patterns, and findings are cited as `retrieved`.
4. Use `ao lookup` for automatic citations during work, or `ao search --cite retrieved|reference|applied` when a search result is actually adopted.
5. End the session with `ao codex stop`, then verify loop health with `ao codex status`.

**Result:** The agent gets the RPI workflow, prior context, and a citation path in both modes. The difference is whether lifecycle work is hook-driven or command-driven.

### Workflow Reference During Planning

**User says:** "How should I approach this feature?"

**What happens:**
1. Agent references this skill's RPI workflow section
2. Agent recommends Research → Plan → Implement → Validate phases
3. Agent suggests `/research` for codebase exploration, `/plan` for decomposition
4. Agent explains `/pre-mortem` for failure simulation before implementation
5. User follows recommended workflow with agent guidance

**Result:** Agent provides structured workflow guidance based on this meta-skill, avoiding ad-hoc approaches.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Skill not auto-loaded | Hook runtime unavailable or startup path not run | Hook-capable runtimes: verify `hooks/session-start.sh` exists and is enabled. Codex: run `ao codex start` explicitly |
| Outdated skill catalog | This file not synced with actual skills/ directory | Update skill list in this file after adding/removing skills |
| Wrong skill suggested | Natural language trigger ambiguous | User explicitly calls skill with `/skill-name` syntax |
| Workflow unclear | RPI phases not well-documented here | Read full workflow guide in README.md or docs/ARCHITECTURE.md |
