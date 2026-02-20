# Skills Reference

Complete reference for all 53 AgentOps skills (43 user-facing + 10 internal).

**Behavioral Contracts:** Most skills include `scripts/validate.sh` behavioral checks to verify key features remain documented. Run `skills/<name>/scripts/validate.sh` when present, or the GOALS.yaml `behavioral-skill-contracts` goal to validate the full covered set.

## Skill Router (Start Here)

Use this when you're not sure which skill to run. For a full workflow overview, run `/using-agentops`.

```text
What are you trying to do?
│
├─ "Not sure what to do yet"
│   └─ Generate options first ─────► /brainstorm
│
├─ "I have an idea"
│   └─ Understand code + context ──► /research
│
├─ "I know what I want to build"
│   └─ Break it into issues ───────► /plan
│
├─ "Now build it"
│   ├─ Small/single issue ─────────► /implement
│   ├─ Multi-issue epic ───────────► /crank <epic-id>
│   └─ Full flow in one command ───► /rpi "goal"
│
├─ "Fix a bug"
│   ├─ Already scoped? ────────────► /implement <issue-id>
│   └─ Need to investigate? ───────► /bug-hunt
│
├─ "Build a feature"
│   ├─ Small (1-2 files) ─────────► /implement
│   ├─ Medium (3-6 issues) ───────► /plan → /crank
│   └─ Large (7+ issues) ─────────► /rpi (full pipeline)
│
├─ "Validate something"
│   ├─ Code ready to ship? ───────► /vibe
│   ├─ Plan ready to build? ──────► /pre-mortem
│   ├─ Work ready to close? ──────► /post-mortem
│   └─ Quick sanity check? ───────► /council --quick validate
│
├─ "Explore or research"
│   ├─ Understand this codebase ──► /research
│   ├─ Compare approaches ────────► /council research <topic>
│   └─ Generate ideas ────────────► /brainstorm
│
├─ "Learn from past work"
│   ├─ What do we know about X? ──► /knowledge <query>
│   ├─ Save this insight ─────────► /learn "insight"
│   └─ Run a retrospective ───────► /retro
│
├─ "Parallelize work"
│   ├─ Multiple independent tasks ► /swarm
│   └─ Full epic with waves ──────► /crank <epic-id>
│
├─ "Ship a release"
│   └─ Changelog + tag ──────────► /release <version>
│
├─ "Session management"
│   ├─ Where was I? ──────────────► /status
│   ├─ Save for next session ─────► /handoff
│   └─ Recover after compaction ──► /recover
│
└─ "First time here" ────────────► /quickstart
```

## Core Workflow Skills

### /research

Deep codebase exploration using Explore agents.

```bash
/research authentication flows in services/auth
```

**Output:** `.agents/research/<topic>.md`

### /plan

Decompose goals into trackable beads issues with dependencies.

```bash
/plan "Add user authentication with OAuth2"
```

**Output:** Beads issues with parent/child relationships

### /implement

Execute a single beads issue with full lifecycle.

```bash
/implement ap-1234
```

**Phases:** Context → Tests → Code → Validation → Commit

### /brainstorm

Structured idea exploration. Four phases: assess clarity, understand idea, explore approaches, capture design.

```bash
/brainstorm "add user authentication"
```

**Output:** `.agents/brainstorm/YYYY-MM-DD-<slug>.md`

### /rpi

Full RPI lifecycle orchestrator. Discovery → Implementation → Validation in one command.

```bash
/rpi "Add user authentication"
/rpi --auto --max-cycles=1    # fully autonomous single cycle
```

**Phases:** Discovery (research + plan + pre-mortem) → Implementation (crank) → Validation (vibe + post-mortem)

### /crank

Autonomous multi-issue execution. Runs until epic is CLOSED.

```bash
/crank <epic-id>
```

**Modes:** Crew (sequential) or Mayor (parallel via polecats)

### /vibe

Comprehensive code validation across 8 aspects.

```bash
/vibe services/auth/
```

**Checks:** Security, Quality, Architecture, Complexity, Testing, Accessibility, Performance, Documentation

### /retro

Extract learnings from completed work.

```bash
/retro "debugging memory leak"
```

**Output:** `.agents/retros/` and `.agents/learnings/`

### /post-mortem

Full validation + knowledge extraction. Council validates, retro extracts learnings, then synthesizes process improvement proposals and suggests the next `/rpi` command. The flywheel exit point.

```bash
/post-mortem <epic-id>
```

**Output:** Council report, retro, learnings, process improvement proposals, next-work queue (`.agents/rpi/next-work.jsonl`)

---

## Utility Skills

### /beads

Git-native issue tracking operations.

```bash
bd ready              # Unblocked issues
bd show <id>          # Issue details
bd close <id>         # Close issue
```

### /bug-hunt

Root cause analysis with git archaeology.

```bash
/bug-hunt "login fails after password reset"
```

### /knowledge

Query knowledge artifacts across locations.

```bash
/knowledge "patterns for rate limiting"
```

**Searches:** `.agents/learnings/`, `.agents/patterns/`, `.agents/research/`

### /complexity

Code complexity analysis using radon (Python) or gocyclo (Go).

```bash
/complexity services/
```

**Threshold:** CC > 10 triggers refactoring issue

### /doc

Generate documentation for code.

```bash
/doc services/auth/
```

### /pre-mortem

Simulate failures before implementing.

```bash
/pre-mortem "add caching layer"
```

**Output:** Failure modes, mitigation strategies, spec improvements

---

## Orchestration Skills

### /council

Multi-model validation — the core primitive used by vibe, pre-mortem, and post-mortem.

```bash
/council validate recent
/council --deep recent
```

### /swarm

Parallel agent spawning for concurrent task execution.

```bash
/swarm <epic-id>
```

### /codex-team

Spawn parallel Codex execution agents.

```bash
/codex-team <epic-id>
```

---

## Additional Utility Skills

### /handoff

Session handoff — preserve context for continuation.

```bash
/handoff
```

### /inbox

Agent mail monitoring for distributed mode.

```bash
/inbox
```

### /release

Pre-flight checks, changelog generation, version bumps, and tagging.

```bash
/release
```

### /status

Single-screen dashboard of project state.

```bash
/status
```

### /quickstart

Interactive onboarding — mini RPI cycle for new users.

```bash
/quickstart
```

### /trace

Trace design decisions through knowledge artifacts.

```bash
/trace "why did we choose Redis?"
```

### /recover

Post-compaction context recovery. Detects in-progress RPI and evolve sessions, loads knowledge, shows recent work and pending tasks.

```bash
/recover                     # Recover context after compaction
```

### /evolve

Autonomous fitness-scored improvement loop. Measures GOALS.yaml, fixes the worst gap, compounds via knowledge flywheel.

```bash
/evolve                      # Run until all goals met
/evolve --max-cycles=5       # Cap at 5 cycles
/evolve --dry-run            # Measure only, don't execute
```

### /product

Interactive PRODUCT.md generation. Interviews about mission, personas, value props, and competitive landscape.

```bash
/product
```

**Output:** `PRODUCT.md` in repo root

### /heal-skill

Detect and auto-fix skill hygiene issues (missing frontmatter, unlinked references, dead references).

```bash
/heal-skill --check                     # Report issues
/heal-skill --fix                       # Auto-fix what's safe
/heal-skill --check skills/council      # Check specific skill
```

**Checks:** MISSING_NAME, MISSING_DESC, NAME_MISMATCH, UNLINKED_REF, EMPTY_DIR, DEAD_REF

### /converter

Convert skills to other platforms (Codex, Cursor).

```bash
/converter skills/council codex          # Single skill to Codex format
/converter --all cursor                  # All skills to Cursor .mdc format
```

**Targets:** codex (SKILL.md + prompt.md), cursor (.mdc + optional mcp.json), test (raw bundle)

### /openai-docs

Use official OpenAI docs MCP access for current API/platform guidance with citations.

```bash
/openai-docs "responses api tools"
```

### /oss-docs

Scaffold and audit open-source documentation packs (README, CONTRIBUTING, changelog, AGENTS).

```bash
/oss-docs
```

### /pr-research

Research upstream contribution conventions before implementing an external PR.

```bash
/pr-research https://github.com/org/repo
```

### /pr-plan

Create a scoped contribution plan from PR research artifacts.

```bash
/pr-plan
```

### /pr-implement

Execute fork-based contribution work with isolation checks.

```bash
/pr-implement
```

### /pr-validate

Run PR-specific validation (scope creep, isolation, upstream alignment).

```bash
/pr-validate
```

### /pr-prep

Prepare structured PR bodies with validation evidence.

```bash
/pr-prep
```

### /pr-retro

Capture lessons from accepted/rejected PR outcomes.

```bash
/pr-retro
```

### /update

Reinstall all AgentOps skills globally from the latest source.

```bash
/update                      # Reinstall all 51 skills
```

---

## Internal Skills

These fire automatically and are not directly invoked:

| Skill | Purpose |
|-------|---------|
| `inject` | Load knowledge at session start (`ao inject`) |
| `extract` | Extract decisions/learnings from transcripts |
| `forge` | Mine transcripts for knowledge artifacts |
| `ratchet` | Progress gates for RPI workflow |
| `flywheel` | Knowledge health monitoring |
| `provenance` | Trace knowledge artifact lineage |
| `standards` | Language-specific coding standards (auto-loaded by /vibe, /implement) |
| `shared` | Shared reference documents for distributed mode |
| `beads` | Issue tracking reference |
| `using-agentops` | Workflow guide (auto-injected on session start) |

---

## Subagents

Subagent behaviors are defined inline within SKILL.md files (not as separate agent files). Skills that use subagents spawn them as Task agents during execution. 20 specialized roles are used across `/vibe`, `/pre-mortem`, `/post-mortem`, and `/research`.

| Agent Role | Used By | Focus |
|------------|---------|-------|
| Code reviewer | /vibe, /council | Quality, patterns, maintainability |
| Security reviewer | /vibe, /council | Vulnerabilities, OWASP |
| Security expert | /vibe, /council | Deep security analysis |
| Architecture expert | /vibe, /council | System design review |
| Code quality expert | /vibe, /council | Complexity and maintainability |
| UX expert | /vibe, /council | Accessibility and UX validation |
| Plan compliance expert | /post-mortem | Compare implementation to plan |
| Goal achievement expert | /post-mortem | Did we solve the problem? |
| Ratchet validator | /post-mortem | Verify gates are locked |
| Flywheel feeder | /post-mortem | Extract learnings with provenance |
| Technical learnings expert | /post-mortem | Technical patterns |
| Process learnings expert | /post-mortem | Process improvements |
| Integration failure expert | /pre-mortem | Integration risks |
| Ops failure expert | /pre-mortem | Operational risks |
| Data failure expert | /pre-mortem | Data integrity risks |
| Edge case hunter | /pre-mortem | Edge cases and exceptions |
| Coverage expert | /research | Research completeness |
| Depth expert | /research | Depth of analysis |
| Gap identifier | /research | Missing areas |
| Assumption challenger | /research | Challenge assumptions |

---

## ao CLI Integration

Skills integrate with the ao CLI for orchestration:

| Skill | ao CLI Command |
|-------|----------------|
| `/research` | `ao forge search` |
| `/retro` | `ao forge index` |
| `/post-mortem` | `ao ratchet record` |
| `/implement` | `ao ratchet claim/record` |
| `/crank` | `ao ratchet verify` |
