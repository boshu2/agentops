# Skills Reference

Complete reference for all 68 AgentOps skills (59 user-facing + 9 internal).

Skills are the primitive layer of AgentOps. Higher-level entry points like
`/implement`, `/validation`, `/rpi`, and `/evolve` compose those primitives
into repeatable flows.

**Behavioral Contracts:** Most skills include `scripts/validate.sh` behavioral checks to verify key features remain documented. Run `skills/<name>/scripts/validate.sh` when present, or the GOALS.yaml `behavioral-skill-contracts` goal to validate the full covered set.

## Skill Router (Start Here)

Use this when you're not sure which skill to run. For a full flow overview, run
`/using-agentops`.

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
│   ├─ Work ready to close? ──────► /validation
│   ├─ Code quality only? ───────► /vibe
│   ├─ Plan ready to build? ──────► /pre-mortem
│   └─ Quick sanity check? ───────► /council --quick validate
│
├─ "Explore or research"
│   ├─ Understand this codebase ──► /research
│   ├─ Compare approaches ────────► /council research <topic>
│   └─ Generate ideas ────────────► /brainstorm
│
├─ "Learn from past work"
│   ├─ Turn the corpus into operator surfaces ─► /knowledge-activation
│   ├─ What do we know about X? ──► /knowledge <query>
│   ├─ Save this insight ─────────► /retro --quick "insight"
│   └─ Full retrospective ────────► /post-mortem
│
├─ "Parallelize work"
│   ├─ Multiple independent tasks ► /swarm
│   └─ Full epic with waves ──────► /crank <epic-id>
│
├─ "Ship a release"
│   └─ Changelog + tag ──────────► /release <version>
│
├─ "Session management"
│   ├─ Prep or review Dream runs ─► /dream
│   ├─ Where was I? ──────────────► /status
│   ├─ Save for next session ─────► /handoff
│   └─ Recover after compaction ──► /recover
│
└─ "First time here" ────────────► /quickstart
```

## Core Flow Skills

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
/rpi --fast-path "fix typo in README"
/rpi --from=implementation ag-1234
```

**Phases:** Discovery (`/discovery`) → Implementation (`/crank`) → Validation (`/validation`)

### /crank

Autonomous multi-issue execution. Runs until epic is CLOSED.

```bash
/crank <epic-id>
```

**Execution model:** Wave-based orchestration via `/swarm` with runtime-native workers.

### /validation

Full validation close-out. Wraps `/vibe` + `/post-mortem` + `/retro` + `/forge`.

```bash
/validation
/validation ag-1234
```

**Use when:** The work is ready for final review, closeout, and learning capture.

### /vibe

Comprehensive code validation across 8 aspects with finding classification (CRITICAL vs INFORMATIONAL), suppression framework for known false positives, and domain-specific checklists (SQL safety, LLM trust boundary, race conditions) auto-loaded from `/standards`. Correlates findings against pre-mortem predictions.

```bash
/vibe services/auth/
```

**Checks:** Security, Quality, Architecture, Complexity, Testing, Accessibility, Performance, Documentation

### /retro

Quick-capture a learning. For full retrospectives, use `/post-mortem`.

```bash
/retro --quick "debugging memory leak"
```

**Output:** `.agents/learnings/`

### /post-mortem

Full validation + knowledge lifecycle. Council validates, extracts learnings, activates/retires knowledge, then synthesizes process improvement proposals and suggests the next `/rpi` command. The flywheel exit point. Now includes RPI session streak tracking, prediction accuracy scoring (HIT/MISS/SURPRISE against pre-mortem predictions), and persistent retro history to `.agents/retro/` for cross-epic trend analysis. Supports `--quick`, `--process-only`, and `--skip-activate` flags.

```bash
/post-mortem <epic-id>
/post-mortem --quick            # Lightweight post-mortem
/post-mortem --process-only     # Process improvements only
/post-mortem --skip-activate    # Skip knowledge activation
```

**Output:** Council report, learnings, knowledge activation/retirement, process improvement proposals, next-work queue (`.agents/rpi/next-work.jsonl`)

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

### /llm-wiki

Maintain a compounding wiki for external reading material.

```bash
/llm-wiki ingest raw/articles/example.md
/llm-wiki query "what do we know about this topic?"
/llm-wiki lint
```

**Status:** Proposal skill for external `raw/` to `wiki/` knowledge, separate from the internal `.agents/` compiler.

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

Simulate failures before implementing. Includes error/rescue mapping (tabular risk/mitigation), scope mode selection (Expand/Hold/Reduce with auto-detection), temporal interrogation (hour 1/2/4/6+ timeline), and prediction tracking with unique IDs (`pm-YYYYMMDD-NNN`) correlated through vibe and post-mortem.

```bash
/pre-mortem "add caching layer"
```

**Output:** Failure modes, error/rescue maps, predictions with IDs, mitigation strategies, spec improvements

---

## Orchestration Skills

### /council

Multi-model validation — the core primitive used by vibe, pre-mortem, and post-mortem. Auto-extracts significant findings from WARN/FAIL verdicts into the knowledge flywheel.

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

### /dream

Interactive Dream operator surface for setup, bedtime runs, and morning
reports on top of the shared `ao overnight` engine.

```bash
/dream setup
/dream start "close the loop on today's auth work"
/dream report
```

**Output:** Dream setup preview or `.agents/overnight/<run>/summary.{json,md}`

### /trace

Trace design decisions through knowledge artifacts.

```bash
/trace "why did we choose Redis?"
```

### /knowledge-activation

Operationalize a mature `.agents` corpus into reusable belief, playbook, briefing, and gap surfaces.

```bash
/knowledge-activation
ao knowledge activate --goal "productize knowledge activation"
ao knowledge gaps
```

### /recover

Post-compaction context recovery. Detects in-progress RPI and evolve sessions, loads knowledge, shows recent work and pending tasks.

```bash
/recover                     # Recover context after compaction
```

### /evolve

Autonomous fitness-scored improvement loop. Measures GOALS.yaml, fixes the worst gap, compounds via knowledge flywheel.

```bash
/evolve                      # Run until stopped or the full producer ladder is exhausted
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

Prepare structured PR bodies with validation evidence. Includes commit split advisor (Phase 4.5) suggesting bisectable commit ordering.

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
/update                      # Reinstall all 68 skills
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
| `shared` | Shared reference documents for multi-agent backends |
| `beads` | Issue tracking reference |
| `using-agentops` | Workflow guide (hook-capable silent state prep, explicit Codex startup fallback) |

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
| `/research` | `ao lookup`, `ao search`, `ao rpi phased` |
| `/retro` | `ao forge markdown`, `ao session close` |
| `/post-mortem` | `ao forge`, `ao flywheel close-loop`, `ao constraint activate` |
| `/implement` | `ao context assemble`, `ao lookup`, `ao ratchet record` |
| `/crank` | `ao rpi phased`, `ao ratchet`, `ao flywheel status` |
