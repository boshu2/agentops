# Skills Reference

Narrative reference for checked-in AgentOps skills. The current inventory is
generated from `skills/**/SKILL.md` into `registry.json` and the generated
domain maps; do not hard-code skill counts here.

Skills are the primitive layer of AgentOps. Higher-level entry points like
`/implement`, `/validate`, `/rpi`, and `/evolve` compose those primitives
into repeatable flows.

**Behavioral Contracts:** Most skills include `scripts/validate.sh` behavioral checks to verify key features remain documented. Run `skills/<name>/scripts/validate.sh` when present, or the GOALS.yaml `behavioral-skill-contracts` goal to validate the full covered set.

## Skill Router (Start Here)

Use this when you're not sure which skill to run. For a full flow overview, run
`ao session bootstrap`, then `ao lookup --query "<topic>"` when you need on-demand context loading.
To search skills by intent instead of reading this tree, use `ms search "<task>"`
(or `mcp__ms__search`) — the skill-search engine over both corpora ([`skills/ms/SKILL.md`](../skills/ms/SKILL.md)).

```text
What are you trying to do?
│
├─ "Prove it's done / validate" (the Membrane — no verdict = not done)
│   ├─ Code ready to ship? ───────► /validate
│   ├─ Deeper code audit? ────────► /validate --mode=post-impl
│   ├─ Plan ready to build? ──────► /pre-mortem
│   ├─ Independent judges ────────► /council validate recent
│   ├─ Adversarially probe it ────► /validate --debate
│   ├─ Need fresh pawl evidence? ──► /pawl-review → ao pawl
│   ├─ Drive fixes to agreement ──► /converge
│   ├─ Mid-epic drift check ──────► /reality-check
│   ├─ Security + release gate ───► /security
│   └─ Work ready to close? ──────► /validate, then /post-mortem
│
├─ "Track it / bookkeep it" (the Bookkeeper)
│   ├─ Break it into issues ──────► /plan
│   ├─ Manage/close issues ───────► /beads-br
│   ├─ Turn a goal into a loop-ready packet ─► /goal-design
│   ├─ Shape a fuzzy idea ────────► /discovery --ideate
│   ├─ Build a single issue ──────► /implement
│   ├─ Where was I? ──────────────► /status
│   └─ Save for next session ─────► /handoff
│
├─ "Build a feature"
│   ├─ Small (1-2 files) ─────────► /implement
│   ├─ Medium (3-6 issues) ───────► /plan → /crank
│   └─ Large (7+ issues) ─────────► /rpi (full pipeline)
│
├─ "Now build it"
│   ├─ Small/single issue ─────────► /implement
│   ├─ Multi-issue epic ───────────► /crank <epic-id>
│   └─ Full flow in one command ───► /rpi "goal"
│
├─ "Fix a bug"
│   ├─ Already scoped? ────────────► /implement <issue-id>
│   └─ Need to investigate? ───────► /validate --mode=pr
│
├─ "Explore or research"
│   ├─ Understand this codebase ──► /research
│   ├─ Compare approaches ────────► /council research <topic>
│   └─ Generate ideas ────────────► /discovery --ideate
│
├─ "Learn from past work"
│   ├─ Turn the corpus into operator surfaces ─► /operationalize
│   ├─ What do we know about X? ──► ao lookup "<query>" / ao search
│   ├─ Save this insight ─────────► /post-mortem --quick "insight"
│   └─ Full retrospective ────────► /post-mortem
│
├─ "Parallelize work"
│   ├─ Multiple independent tasks ► /swarm
│   └─ Full epic with waves ──────► /crank <epic-id>
│
├─ "City-shaped multi-quest work" (gas city — operator choice, coexists with NTM; never auto-routed)
│   ├─ Stand up / drive / admin / unstick a city ─► /using-gc
│   └─ The close door + pawl-verdict.v1 internals ► gc-membrane (reference)
│
├─ "Ship a release"
│   └─ Changelog + tag ──────────► /release <version>
│
├─ "Session management"
│   ├─ Compile knowledge ─────────► /post-mortem  or  ao compile
│   ├─ Where was I? ──────────────► /status
│   ├─ Save for next session ─────► /handoff
│   └─ Recover after compaction ──► /status --recover
│
└─ "First time here" ────────────► ao session bootstrap → /status
```

<!-- BEGIN:spine -->
## The Membrane — validation spine (no verdict = not done)

The verification skills, the load-bearing product. Every change reaches *done*
only with an independent verdict — a fresh-context, cross-family, or
deterministic check on the completion claim. Reach here first: no verdict, not
done.

### /validate

Final validation close-out. Use `/post-mortem` after validation when the work
should feed the knowledge flywheel.

```bash
/validate
/validate ag-1234
```

**Use when:** The work is ready for final review, closeout, and learning capture.

**Absorbed (retired 2026-07-07, folded here):** `/review` → `/validate --mode=pr` (diff/PR review); `/red-team` → `/validate --debate` (adversarial); `/eval-outcomes` → `/validate --mode=pre-impl --target=scenario` (CLI: `ao eval scenario`).

### /validate --mode=post-impl (absorbs /vibe)

Comprehensive code validation across 8 aspects with finding classification (CRITICAL vs INFORMATIONAL), suppression framework for known false positives, and domain-specific checklists (SQL safety, LLM trust boundary, race conditions) auto-loaded from `/standards`. Correlates findings against pre-mortem predictions.

```bash
/validate --mode=post-impl services/auth/
```

**Checks:** Security, Quality, Architecture, Complexity, Testing, Accessibility, Performance, Documentation

### /council

Multi-model validation — the core primitive used by validate, pre-mortem, and post-mortem. Auto-extracts significant findings from WARN/FAIL verdicts into the knowledge flywheel.

```bash
/council validate recent
/council --deep recent
```

### /pre-mortem

Simulate failures before implementing. Includes error/rescue mapping (tabular risk/mitigation), scope mode selection (Expand/Hold/Reduce with auto-detection), temporal interrogation (hour 1/2/4/6+ timeline), and prediction tracking with unique IDs (`pm-YYYYMMDD-NNN`) correlated through validate and post-mortem.

```bash
/pre-mortem "add caching layer"
```

**Output:** Failure modes, error/rescue maps, predictions with IDs, mitigation strategies, spec improvements

### /converge

Drive a fix → re-run-judge-panel loop to terminal agreement or a 3-consecutive-fail BLOCK via the Go `ao converge` command. Thin memo over the CLI — the loop and gates live in Go.

```bash
ao converge --scope head
```

### /reality-check

Mid-epic drift audit: code is ground truth; README/PRODUCT/plan are the measuring stick. Use when a wave boundary lands and bead counts look healthy but value feels absent.

```bash
/reality-check
```

### /security

Run repository security scans for vulnerabilities, dependency risk, secrets, and release gates — plus the composable binary/prompt-surface suite (offline redteam, policy gating).

```bash
/security audit
```

### /pawl-review

Run one immutable, fresh, read-only reviewer lane and hand its contained evidence
to `ao pawl`. The skill does not decide or write the panel verdict.

```bash
/pawl-review
```

---

## The Bookkeeper — tracking + session spine

Where work lives between your head and *done*: the linked-intent packet, its
issues and dependency waves, and the session continuity that keeps the next
turn from starting from scratch.

### /beads-br

Git-native issue tracking operations.

```bash
BEADS_DIR="$(ao beads dir)" br ready      # Unblocked issues
BEADS_DIR="$(ao beads dir)" br show <id>  # Issue details
BEADS_DIR="$(ao beads dir)" br close <id> # Close issue
```

### /status

Single-screen dashboard of project state.

```bash
/status
```

**Absorbed (retired 2026-07-07, folded here):** `/recover` → `/status --recover` (post-compaction recovery; deep playbook at `skills/status/references/recovery-playbook.md`).

### /handoff

Session handoff — preserve context for continuation.

```bash
/handoff
```

### /discovery --ideate

Structured idea exploration. Four phases: assess clarity, understand idea, explore approaches, capture design.

```bash
/discovery --ideate "add user authentication"
```

**Output:** `.agents/discovery/YYYY-MM-DD-<slug>.md`

### /goal-design

Create a schema-backed `.agents/goal-design/<slug>/` packet with checked
`intent.md` and `driver.md` artifacts before discovery or planning.

```bash
/goal-design "harden packet identity checks"
```

**Output:** `.agents/goal-design/<slug>/intent.md` and `driver.md`

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
<!-- END:spine -->

---

## Flow Skills

The research → build → ship path. These move work through the system; the
Membrane proves each step and the Bookkeeper tracks it.

### /research

Deep codebase exploration using Explore agents.

```bash
/research authentication flows in services/auth
```

**Output:** `.agents/research/<topic>.md`

### /rpi

Full RPI lifecycle orchestrator. Discovery → Implementation → Validation in one command.

```bash
/rpi "Add user authentication"
/rpi --fast-path "fix typo in README"
/rpi --from=implementation ag-1234
```

**Phases:** Discovery (`/discovery`) → Implementation (`/crank`) → Validation (`/validate`)

### /crank

Autonomous multi-issue execution. Runs until epic is CLOSED.

```bash
/crank <epic-id>
```

**Execution model:** Wave-based orchestration via `/swarm` with runtime-native workers.

### /swarm

Parallel agent spawning for concurrent task execution.

```bash
/swarm <epic-id>
```

### Runtime-native multi-agent lanes

Spawn parallel execution agents through the current runtime/substrate. Use
`/swarm` for the skill-level entry point; use Codex subagents or NTM when
the active runtime owns that transport.

```bash
/swarm <epic-id>
```

### /post-mortem --quick

Quick-capture a learning. For full retrospectives, use `/post-mortem`.

```bash
/post-mortem --quick "debugging memory leak"
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

### Knowledge queries (no slash command)

Query knowledge artifacts across locations via the CLI. There is no standalone
knowledge skill — use `/operationalize` and `/post-mortem` (with the `ao compile`
CLI) for corpus promotion, or run the CLI below for ad-hoc lookup.

```bash
ao lookup "patterns for rate limiting"
ao search --all "patterns for rate limiting"
```

**Searches:** `.agents/learnings/`, `.agents/patterns/`, `.agents/research/`, `.agents/compiled/`

### Skill search (ms — no slash command)

Find a skill by intent across both corpora before hand-grepping the catalog.
Consume via MCP (`mcp__ms__search` / `mcp__ms__load full:true`); writes and admin
go through the `ms` CLI. See [`skills/ms/SKILL.md`](../skills/ms/SKILL.md).

```bash
ms search "switch accounts on a rate limit" -O json
ms load account-rotation --full -O json | jq -r '.data.content'
```

### /refactor (absorbs /complexity)

Code complexity analysis using radon (Python) or gocyclo (Go).

```bash
/refactor services/
```

**Threshold:** CC > 10 triggers refactoring issue

### /doc

Generate and validate repo documentation. `--mode` selects the artifact family:
default (code/API docs, code-maps), `--mode=readme` (gold-standard README via
interview + council validation), or `--mode=oss` (open-source doc pack:
CONTRIBUTING, CHANGELOG, AGENTS).

```bash
/doc services/auth/          # code/API docs (default)
/doc --mode=readme           # gold-standard README
/doc --mode=oss              # scaffold/audit OSS doc pack
```

### /release

Pre-flight checks, changelog generation, version bumps, and tagging.

```bash
/release
```

### /status (absorbs /quickstart)

Interactive onboarding — mini RPI cycle for new users.

```bash
/status
```

### Out-of-session compounding

Retirement pointer. The in-tree out-of-session compounding engine was removed
(soc-2rtm0); scheduled, between-session knowledge compounding now runs via an
adopted substrate, and AgentOps ships no out-of-session runner of its own.
In-session knowledge primitives stay on-demand: `/post-mortem`, the `ao compile`
CLI, and `ao lookup`. Daytime code compounding is `/evolve` via `/rpi`.

**Output:** none — this skill no longer drives an in-repo command.

### Knowledge operationalization

Operationalize a mature `.agents` corpus into reusable belief, playbook, briefing, and gap surfaces.

```bash
ao knowledge activate --goal "productize knowledge activation"
ao knowledge gaps
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

### /pr-prep

Prepare structured PR bodies with validation evidence. Includes commit split advisor (Phase 4.5) suggesting bisectable commit ordering.

```bash
/pr-prep
```

---

## Additional Skills

Single-purpose skills not listed above. See each skill's `SKILL.md` for triggers,
phases, and flags.

| Skill | Purpose |
|-------|---------|
| `/bootstrap` | One-command product-layer setup (`GOALS.md`, `PRODUCT.md`, `README.md`, `.agents/`, optional hooks) |
| `/security` (absorbs deps) | Dependency audit, updates, vulnerability scanning, license compliance |
| `/product` | Maintain `PRODUCT.md` so validation and planning share the same product contract |
| `/discovery` | Full discovery-phase orchestrator (ideation + search + research + plan + pre-mortem) |
| `/goal-design` | Create checked goal-design packets before discovery or planning |
| `/goals` | Maintain `GOALS.yaml`/`GOALS.md` fitness specs; measure drift; add/prune directives |
| `/push` | Atomic test-commit-push with conventional-commit message |
| `/refactor` | Safe, verified refactoring with regression tests at each step |
| `/scaffold` | Project scaffolding, component generation, boilerplate |
| `/test` | Test generation, coverage analysis, TDD workflow |

---

## Internal Skills

These are loaded by other skills or lifecycle hooks; they are not primary
user-facing entry points:

| Skill | Purpose |
|-------|---------|
| `standards` | Language-specific coding standards (auto-loaded by /validate, /implement) |
| `shared` | Shared reference documents for multi-agent backends |
| `beads-br` | Issue tracking reference (local-first beads_rust tracker) |

---

## Subagents

Subagent behaviors are defined inline within SKILL.md files (not as separate agent files). Skills that use subagents spawn them as Task agents during execution. 20 specialized roles are used across `/validate`, `/pre-mortem`, `/post-mortem`, and `/research`.

| Agent Role | Used By | Focus |
|------------|---------|-------|
| Code reviewer | /validate, /council | Quality, patterns, maintainability |
| Security reviewer | /validate, /council | Vulnerabilities, OWASP |
| Security expert | /validate, /council | Deep security analysis |
| Architecture expert | /validate, /council | System design review |
| Code quality expert | /validate, /council | Complexity and maintainability |
| UX expert | /validate, /council | Accessibility and UX validation |
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
| `/research` | `ao lookup`, `ao search` (the `/rpi` engine was removed in 3.0 — driven in-session by the operating loop) |
| `/post-mortem --quick` | `ao forge markdown`, `ao session close` |
| `/post-mortem` | `ao forge`, `ao flywheel close-loop`, `ao constraint activate` |
| `/implement` | `ao context assemble`, `ao lookup`, `ao ratchet record` |
| `/crank` | `ao ratchet`, `ao flywheel status` (the `/rpi` engine was removed in 3.0; waves now drive via the operating loop + NTM/Agent Mail substrate) |
