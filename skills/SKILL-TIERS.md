# Skill Tier Taxonomy

This document defines the `tier` field used in skill frontmatter to categorize skills by their role in the AgentOps workflow.

## Tier Values

Skills fall into three functional categories, plus infrastructure tiers for internal and library skills.

| Tier | Category | Description | Examples |
|------|----------|-------------|----------|
| **judgment** | Judgment | Validation, review, and quality gates — council is the foundation | council, vibe, pre-mortem, post-mortem |
| **execution** | Execution | Research, plan, build, ship — the work itself | research, plan, implement, crank, swarm, rpi |
| **knowledge** | Knowledge | The flywheel — extract, store, query, inject learnings | retro (quick-capture), flywheel, forge |
| **product** | Execution | Define mission, goals, release, docs | product, goals, release, readme, doc |
| **session** | Execution | Session continuity and status | handoff, recover, status |
| **utility** | Execution | Standalone tools | quickstart, brainstorm, bug-hunt, complexity |
| **contribute** | Execution | Upstream PR workflow | pr-research, pr-plan, pr-implement, pr-validate, pr-prep, pr-retro, oss-docs |
| **cross-vendor** | Execution | Multi-runtime orchestration | codex-team, openai-docs, converter |
| **library** | Internal | Reference skills loaded JIT by other skills | beads, standards, shared |
| **background** | Internal | Hook-triggered or automatic skills | inject, extract, forge, provenance, ratchet |
| **meta** | Internal | Skills about skills | using-agentops, heal-skill, update |

## The Three Categories

### Judgment — the foundation

Council is the core primitive. Every validation skill depends on it. Remove council and all quality gates break.

```
                         ┌──────────┐
                         │ council  │  ← Core primitive: independent judges
                         └────┬─────┘     debate and converge
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
  ┌────────────┐        ┌─────────┐         ┌─────────────┐
  │ pre-mortem │        │  vibe   │         │ post-mortem │
  │ (plans)    │        │ (code)  │         │ (full retro │
  └────────────┘        └────┬────┘         │ + knowledge)│
                             │              └─────────────┘
                             ▼
                       ┌────────────┐
                       │ complexity │
                       └────────────┘
```

### Execution — the work

Skills that move work through the system. Swarm parallelizes any of them. RPI chains them into a pipeline.

```
RESEARCH          PLAN              IMPLEMENT           VALIDATE
────────          ────              ─────────           ────────

┌──────────┐    ┌──────────┐      ┌───────────┐      ┌──────────┐
│ research │───►│   plan   │─────►│ implement │─────►│   vibe   │
└──────────┘    └────┬─────┘      └─────┬─────┘      └────┬─────┘
                     │                  │                 │
                     ▼                  │                 │
               ┌────────────┐           │                 │
               │ pre-mortem │           │                 │
               │ (council)  │           │                 │
               └────────────┘           │                 │
                                        │                 │
                                        ▼                 ▼
                                   ┌─────────┐      ┌───────────┐
                                   │  swarm  │      │complexity │
                                   └────┬────┘      │ + council │
                                        │          └───────────┘
                                        ▼
                                   ┌─────────┐
                                   │  crank  │
                                   └─────────┘

POST-SHIP                             ONBOARDING / STATUS
─────────                             ───────────────────

┌─────────────┐                       ┌────────────┐
│ post-mortem │                       │ quickstart │ (first-time tour)
│ (council +  │                       └────────────┘
│ knowledge)  │                       ┌────────────┐
└──────┬──────┘                       │   status   │ (dashboard)
       │                              └────────────┘
       ▼
┌─────────────┐
│   release   │ (changelog, version bump, tag)
└─────────────┘
```

### Knowledge — the flywheel

Append-only ledger in `.agents/`. Every session writes. Freshness decay prunes. Next session injects the best. This is what makes sessions compound instead of starting from scratch.

```
┌─────────┐     ┌─────────┐     ┌──────────┐     ┌──────────┐
│ extract │────►│  forge  │────►│ knowledge│────►│  inject  │
└─────────┘     └─────────┘     └──────────┘     └──────────┘
     ▲                                                 │
     │              ┌──────────┐                       │
     └──────────────│ flywheel │◄──────────────────────┘
                    └──────────┘

User-facing: /knowledge, /retro (quick-capture), /post-mortem (full), /flywheel
Background:  inject, extract, forge, provenance, ratchet
CLI:         ao lookup, ao extract, ao forge, ao maturity
```

## Which Skill Should I Use?

Start here. Match your intent to a skill.

```
What are you trying to do?
│
├─ "Fix a bug"
│   ├─ Know which file? ──────────► /implement <issue-id>
│   └─ Need to investigate? ──────► /bug-hunt
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
│   ├─ Save this insight ─────────► /retro --quick "insight"
│   ├─ Full retrospective ────────► /post-mortem
│   └─ Trace a decision ─────────► /trace <concept>
│
├─ "Write or improve tests"
│   ├─ Generate tests for code ───► /test <target>
│   ├─ Find coverage gaps ────────► /test --coverage <scope>
│   └─ TDD a new feature ────────► /test --tdd <feature>
│
├─ "Review someone's code"
│   ├─ Review a PR ───────────────► /review <PR-number>
│   ├─ Review agent output ───────► /review --agent <path>
│   └─ Review local diff ────────► /review --diff
│
├─ "Refactor code"
│   ├─ Refactor specific target ──► /refactor <file-or-function>
│   ├─ Sweep for complexity ──────► /refactor --sweep <scope>
│   └─ Extract method/module ─────► /refactor --extract <pattern>
│
├─ "Manage dependencies"
│   ├─ Full health check ────────► /deps audit
│   ├─ Update dependencies ──────► /deps update
│   ├─ Vulnerability scan ───────► /deps vuln
│   └─ License compliance ───────► /deps license
│
├─ "Performance work"
│   ├─ Profile hotspots ─────────► /perf profile <target>
│   ├─ Run benchmarks ───────────► /perf bench <target>
│   ├─ Compare runs ─────────────► /perf compare <baseline> <candidate>
│   └─ Optimize code ────────────► /perf optimize <target>
│
├─ "Start a new project"
│   ├─ Scaffold project ─────────► /scaffold <language> <name>
│   ├─ Add component ────────────► /scaffold component <type> <name>
│   └─ Generate CI config ───────► /scaffold ci <platform>
│
├─ "Contribute upstream"
│   └─ Full PR workflow ──────────► /pr-research → /pr-plan → /pr-implement
│
├─ "Ship a release"
│   └─ Changelog + tag ──────────► /release <version>
│
├─ "Parallelize work"
│   ├─ Multiple independent tasks ► /swarm
│   ├─ Codex agents specifically ─► /codex-team
│   └─ Full epic with waves ──────► /crank <epic-id>
│
├─ "Session management"
│   ├─ Where was I? ──────────────► /status
│   ├─ Save for next session ─────► /handoff
│   └─ Recover after compaction ──► /recover
│
└─ "First time here"
    └─ Interactive tour ──────────► /quickstart
```

### Composition patterns

These are how skills chain in practice:

| Pattern | Chain | When |
|---------|-------|------|
| **Quick fix** | `/implement` | One issue, clear scope |
| **Quick ship** | `/implement` → `/push` | Implement, test, and push |
| **Validated fix** | `/implement` → `/vibe` | One issue, want confidence |
| **Planned epic** | `/plan` → `/pre-mortem` → `/crank` → `/post-mortem` | Multi-issue, structured |
| **Full pipeline** | `/rpi` (chains all above) | End-to-end, autonomous |
| **Evolve loop** | `/evolve` (chains `/rpi` repeatedly) | Fitness-scored improvement |
| **PR contribution** | `/pr-research` → `/pr-plan` → `/pr-implement` → `/pr-validate` → `/pr-prep` | External repo |
| **Knowledge query** | `/knowledge` → `/research` (if gaps) | Understanding before building |
| **Standalone review** | `/council validate <target>` | Ad-hoc multi-judge review |
| **Time-boxed pipeline** | `/rpi --budget=research:180,plan:120` | Prevent research/plan stalls |
| **TDD feature** | `/implement <issue>` | TDD-first by default (skip with `--no-tdd`) |
| **Scoped parallel** | `/crank <epic>` | Auto file-ownership map prevents conflicts |
| **Test-first build** | `/test --tdd` → `/implement` | Write tests before code |
| **Reviewed PR** | `/review <PR>` → approve/request changes | Incoming PR review |
| **Safe refactor** | `/complexity` → `/refactor` → `/test` | Find hotspots, refactor, verify |
| **Dep hygiene** | `/deps audit` → `/deps update` → `/test` | Audit, update, verify |
| **Perf cycle** | `/perf profile` → `/perf optimize` → `/perf compare` | Profile, fix, verify |
| **New project** | `/scaffold` → `/test` → `/push` | Bootstrap, verify, ship |

---

## Current Skill Tiers

### User-Facing Skills (52)

**Judgment:**

| Skill | Tier | Description |
|-------|------|-------------|
| **council** | judgment | Multi-model validation (core primitive) — independent judges debate and converge |
| **vibe** | judgment | Complexity analysis + council — code quality review |
| **pre-mortem** | judgment | Council on plans — simulate failures before implementation |
| **post-mortem** | judgment | Council + knowledge lifecycle — validate completed work, extract/activate/retire learnings |
| **review** | judgment | Review incoming PRs, agent-generated changes, or diffs — SCORED checklist |

**Execution:**

| Skill | Tier | Description |
|-------|------|-------------|
| **research** | execution | Deep codebase exploration |
| **brainstorm** | execution | Structured idea exploration before planning |
| **plan** | execution | Decompose epics into issues with dependency waves |
| **implement** | execution | Full lifecycle for one task |
| **crank** | execution | Autonomous epic execution — parallel waves |
| **discovery** | meta | Discovery phase orchestrator — brainstorm → search → research → plan → pre-mortem |
| **validation** | meta | Validation phase orchestrator — vibe → post-mortem → retro → forge |
| **swarm** | execution | Parallelize any skill — fresh context per agent |
| **rpi** | meta | Thin wrapper: /discovery → /crank → /validation with complexity classification and loop |
| **evolve** | execution | Autonomous fitness-scored improvement loop |
| **bug-hunt** | execution | Investigate bugs with git archaeology |
| **complexity** | execution | Cyclomatic complexity analysis |
| **grafana-platform-dashboard** | execution | Build and validate platform operations dashboards with critical-first layout and PromQL gates |
| **push** | execution | Atomic test-commit-push workflow — tests, commits, rebases, pushes |
| **test** | execution | Test generation, coverage analysis, and TDD workflow |
| **refactor** | execution | Safe, verified refactoring with regression testing at each step |
| **deps** | execution | Dependency audit, update, vulnerability scanning, and license compliance |
| **perf** | execution | Performance profiling, benchmarking, regression detection, and optimization |
| **scaffold** | execution | Project scaffolding, component generation, and boilerplate setup |

**Knowledge:**

| Skill | Tier | Description |
|-------|------|-------------|
| **athena** | advanced | Active knowledge intelligence — Mine → Grow → Defrag cycle |
| **harvest** | knowledge | Cross-rig knowledge consolidation — sweep, dedup, promote |
| **retro** | knowledge | Quick-capture wrapper (full retro → /post-mortem) |
| **trace** | knowledge | Trace design decisions through history |

**Product & Release:**

| Skill | Tier | Description |
|-------|------|-------------|
| **product** | product | Interactive PRODUCT.md generation |
| **goals** | product | Maintain GOALS.yaml fitness specification |
| **release** | product | Pre-flight, changelog, version bumps, tag |
| **security** | product | Continuous security scanning and release gating |
| **security-suite** | execution | Composable security suite for binary and prompt-surface assurance, offline redteam, and policy gating |
| **readme** | product | Gold-standard README generation with council validation |
| **doc** | product | Generate documentation |

**Session & Status:**

| Skill | Tier | Description |
|-------|------|-------------|
| **handoff** | session | Session handoff — save context for next session |
| **recover** | session | Post-compaction context recovery |
| **status** | session | Single-screen dashboard |
| **quickstart** | session | Interactive onboarding |

**Upstream Contributions:**

| Skill | Tier | Description |
|-------|------|-------------|
| **pr-research** | contribute | Upstream repository research before contribution |
| **pr-plan** | contribute | Contribution planning for external PRs |
| **pr-implement** | contribute | Fork-based implementation for external PRs |
| **pr-validate** | contribute | PR-specific isolation and scope validation |
| **pr-prep** | contribute | PR preparation and structured PR body generation |
| **pr-retro** | contribute | Learn from accepted/rejected PR outcomes |
| **oss-docs** | contribute | Scaffold and audit OSS documentation packs |

**Cross-Vendor & Meta:**

| Skill | Tier | Description |
|-------|------|-------------|
| **codex-team** | cross-vendor | Spawn parallel Codex execution agents |
| **openai-docs** | cross-vendor | Authoritative OpenAI docs lookup with citations |
| **converter** | cross-vendor | Cross-platform skill converter (Codex, Cursor) |
| **reverse-engineer-rpi** | execution | Reverse-engineer a product into feature catalog + code map + specs |
| **heal-skill** | meta | Detect and fix skill hygiene issues |
| **update** | meta | Reinstall all AgentOps skills globally |

### Internal Skills (9) — `metadata.internal: true`

Not auto-loaded — loaded JIT by other skills via Read or auto-triggered by hooks. Loaded JIT by other skills via Read or auto-triggered by hooks.

| Skill | Tier | Category | Purpose |
|-------|------|----------|---------|
| beads | library | Execution | Issue tracking reference (loaded by /implement, /plan) |
| standards | library | Judgment | Coding standards (loaded by /vibe, /implement, /doc) |
| shared | library | Execution | Shared reference documents (multi-agent backends) |
| inject | background | Knowledge | Load knowledge at session start (hook-triggered) |
| forge | background | Knowledge | Mine transcripts for knowledge (includes --promote for pending extraction) |
| provenance | background | Knowledge | Trace knowledge lineage |
| ratchet | background | Execution | Progress gates |
| flywheel | background | Knowledge | Knowledge health monitoring |
| using-agentops | meta | Meta | AgentOps workflow guide (auto-injected) |

---

## Skill Dependency Graph

### Dependency Table

| Skill | Dependencies | Type |
|-------|--------------|------|
| **athena** | - | - (standalone, ao CLI optional) |
| **harvest** | - | - (standalone, ao CLI required) |
| **council** | - | - (core primitive) |
| **vibe** | council, complexity, standards | required, optional (graceful skip), optional |
| **pre-mortem** | council | required |
| **post-mortem** | council, beads | required, optional |
| beads | - | - |
| bug-hunt | beads | optional |
| complexity | - | - |
| **codex-team** | - | - (standalone, fallback to swarm) |
| **crank** | swarm, vibe, implement, beads, post-mortem | required, required, required, optional, optional |
| doc | standards | required |
| flywheel | - | - |
| forge | - | - |
| handoff | - | - |
| **implement** | beads, standards | optional, required |
| inject | - | - |
| **openai-docs** | - | - (standalone) |
| **plan** | research, beads, pre-mortem, crank, implement | optional, optional, optional, optional, optional |
| **push** | - | - (standalone) |
| **product** | - | - (standalone) |
| **pr-research** | - | - (standalone) |
| **pr-plan** | pr-research | optional |
| **pr-implement** | pr-plan, pr-validate | optional, optional |
| **pr-validate** | - | - (standalone) |
| **pr-prep** | pr-validate | optional |
| **pr-retro** | pr-prep | optional |
| **oss-docs** | doc | optional |
| provenance | - | - |
| **quickstart** | - | - (zero dependencies) |
| **discovery** | brainstorm, research, plan, pre-mortem, shared | brainstorm optional, rest required |
| **validation** | vibe, post-mortem, retro, forge, shared | vibe+post-mortem required, retro+forge optional |
| **rpi** | discovery, crank, validation, ratchet | all required |
| **evolve** | rpi | required (rpi pulls in all sub-skills) |
| **release** | - | - (standalone) |
| **security** | - | - (standalone) |
| **security-suite** | - | - (standalone) |
| ratchet | - | - |
| **recover** | - | - (standalone) |
| **reverse-engineer-rpi** | - | - (standalone) |
| **grafana-platform-dashboard** | research, brainstorm | optional, optional |
| research | knowledge, inject | optional, optional |
| retro | - | - |
| standards | - | - |
| **goals** | - | - (reads GOALS.yaml directly) |
| **status** | - | - (all CLIs optional) |
| **swarm** | implement, vibe | required, optional |
| trace | provenance | alternative |
| **update** | - | - (standalone) |
| using-agentops | - | - |
| **test** | standards, complexity | required, optional |
| **review** | standards, council | required, optional |
| **refactor** | standards, complexity, beads | required, optional, optional |
| **deps** | standards | optional |
| **perf** | standards, complexity | optional, optional |
| **scaffold** | standards | required |

---

## CLI Integration

### Spawning Agents

| Vendor | CLI | Command |
|--------|-----|---------|
| Claude | `claude` | `claude --print "prompt" > output.md` |
| Codex | `codex` | `codex exec --full-auto -m gpt-5.3-codex -C "$(pwd)" -o output.md "prompt"` |
| OpenCode | `opencode` | (similar pattern) |

### Default Models

| Vendor | Model |
|--------|-------|
| Claude | Opus 4.6 |
| Codex/OpenAI | GPT-5.3-Codex |

### /council spawns both

```bash
# Runtime-native judges (spawn via whatever multi-agent primitive your runtime provides)
# Each judge receives a prompt, writes output to .agents/council/, signals completion

# Codex CLI judges (--mixed mode, via shell)
codex exec --full-auto -m gpt-5.3-codex -C "$(pwd)" -o .agents/council/codex-output.md "..."
```

### Consolidated Output

All council-based skills write to `.agents/council/`:

| Skill / Mode | Output Pattern |
|--------------|----------------|
| `/council validate` | `.agents/council/YYYY-MM-DD-<target>-report.md` |
| `/council brainstorm` | `.agents/council/YYYY-MM-DD-brainstorm-<topic>.md` |
| `/council research` | `.agents/council/YYYY-MM-DD-research-<topic>.md` |
| `/vibe` | `.agents/council/YYYY-MM-DD-vibe-<target>.md` |
| `/pre-mortem` | `.agents/council/YYYY-MM-DD-pre-mortem-<topic>.md` |
| `/post-mortem` | `.agents/council/YYYY-MM-DD-post-mortem-<topic>.md` |

Individual judge outputs also go to `.agents/council/`:
- `YYYY-MM-DD-<target>-claude-pragmatist.md`, `...-claude-skeptic.md`, `...-claude-visionary.md`
- `YYYY-MM-DD-<target>-codex-pragmatist.md`, `...-codex-skeptic.md`, `...-codex-visionary.md`

---

## Execution Modes

Skills follow a two-tier execution model based on visibility needs:

> **The Rule:** Orchestrators stay inline for visibility. Discovery primitives, judgment skills, and worker spawners fork to keep the caller's context clean.

### Tier 1: NO-FORK (stay in main context)

Orchestrators, single-task executors, and investigative skills stay in the main session so the operator can see progress, phase transitions, and intervene.

| Skill | Role | Why |
|-------|------|-----|
| evolve | Orchestrator | Long loop, need cycle-by-cycle visibility |
| rpi | Orchestrator | Sequential phases, need phase gates |
| crank | Orchestrator | Wave orchestrator, need wave reports |
| discovery | Orchestrator | Discovery phase orchestrator, need gate visibility |
| validation | Orchestrator | Validation phase orchestrator, need verdict visibility |
| implement | Single-task | Single issue, medium duration |
| bug-hunt | Investigator | Hypothesis loop, need to see reasoning |

### Tier 1.5: FORK (discovery primitives)

Discovery skills that produce filesystem artifacts. User wants the output, not the process. Heavy codebase exploration and decomposition runs in a forked subagent; only the summary and artifact path return to the caller's context.

| Skill | Role | Why |
|-------|------|-----|
| research | Discovery | Massive codebase exploration → `.agents/research/*.md` |
| plan | Discovery | Decomposition + beads creation → `.agents/plans/*.md` + beads |
| retro | Knowledge extraction | Extract learnings → `.agents/learnings/*.md` |

### Tier 2: FORK (judgment + worker spawners)

Judgment skills validate artifacts in isolation. Worker spawners fan out parallel work. Results merge back via filesystem.

| Skill | Role | Why |
|-------|------|-----|
| vibe | Judgment | Code validation, user wants verdict |
| pre-mortem | Judgment | Plan validation, user wants verdict |
| post-mortem | Judgment | Validation close-out + knowledge extraction |
| council | Worker spawner | Parallel judges, merge verdicts |
| codex-team | Worker spawner | Parallel Codex agents, merge results |

Note: `swarm` is an orchestrator (no `context: fork`) that spawns runtime workers via `TeamCreate`/`spawn_agent`. The workers it creates are runtime sub-agents, not SKILL.md skills.

### Dual-Role Skills

Some skills are orchestrators when called directly but workers when spawned by another skill. The caller determines the role:

- **implement**: Called directly → orchestrator (stays). Spawned by swarm → worker (already forked by swarm).
- **crank**: Called directly → orchestrator (stays). Called by rpi → still in context (rpi chains sequentially, doesn't fork).

### Mechanism

Set `context: { window: fork }` in skill frontmatter to fork into a subagent. The skill's markdown body becomes the subagent's task prompt. Set on discovery primitives, judgment skills, and worker spawners. Never on orchestrators that need visibility.

---

## See Also

- `skills/council/SKILL.md` — Core judgment primitive
- `skills/vibe/SKILL.md` — Complexity + council for code
- `skills/pre-mortem/SKILL.md` — Council for plans
- `skills/post-mortem/SKILL.md` — Council + retro for wrap-up
- `skills/swarm/SKILL.md` — Parallelize any skill
- `skills/rpi/SKILL.md` — Full pipeline orchestrator
