# Skill Tier Taxonomy

This document defines the internal `tier` field used in skill frontmatter. Publicly, AgentOps talks about bookkeeping, validation, primitives, and flows. The tier names below are the internal execution taxonomy behind that operating model.

## The Spine (Membrane + Bookkeeper)

Two skill sets lead the operating model. The **Membrane** proves each change is
actually done — no verdict, not done. The **Bookkeeper** tracks the work between
your head and *done*. Everything else (research, plan, build, ship) runs *through*
these two, and the router below leads with them.

**The Membrane — validation spine (no verdict = not done):**

- `/validate` — canonical PASS/WARN/FAIL verdict on artifacts, plans, code, PRs, and gates.
- `/review` — structured review + root-cause bug-hunt over PRs, agent output, and diffs.
- `/council` — multi-judge consensus; the core primitive under every validation skill.
- `/pre-mortem` — simulate failures before implementing; predictions tracked into validate.
- `/red-team` — persona-based adversarial probe of a doc, skill, plan, or claim before it ships.
- `/converge` — drive a fix → re-run-judge-panel loop to terminal agreement or a hard BLOCK.
- `/security` — repository security scans (vulns, dependency risk, secrets) plus release gating.
- `/reality-check` — mid-epic drift audit: code is ground truth, the plan is the measuring stick.
- `/pre-land-refuters` — fresh-context refuters attack the completion claim at the shared-trunk pawl before landing.

**The Bookkeeper — tracking + session spine:**

- `/beads-br` — local-first, git-native issue tracker (find ready work, update, close).
- `/status` — single-screen dashboard of project state.
- `/handoff` — compact session handoff so the next turn continues instead of restarting.
- `/discovery` — shape intent into a dense execution packet (ideate → search → research → plan → pre-mortem).
- `/plan` — decompose a goal into an acceptance-gated bead DAG with dependency waves.
- `/implement` — execute a single bead through its full TDD lifecycle.

## Tier Values

Skills fall into three functional categories, plus infrastructure tiers for internal and library skills.

| Tier | Category | Description | Examples |
|------|----------|-------------|----------|
| **judgment** | Validation | Internal tier for validation, review, and quality gates — council is the foundation | council, validate, pre-mortem, post-mortem, red-team |
| **execution** | Primitives + flows | Research, plan, build, and ship — the work itself | research, plan, implement, crank, swarm, rpi |
| **knowledge** | Bookkeeping | The flywheel — capture, store, query, inject, and promote learnings | compile, flywheel, forge, operationalize |
| **product** | Execution | Define mission, goals, release, docs | product, goals, release, doc |
| **session** | Execution | Session continuity and status | handoff, recover, status |
| **utility** | Execution | Standalone tools | converter, scaffold, security, perf |
| **contribute** | Execution | Upstream PR workflow | pr-prep |
| **cross-vendor** | Execution | Multi-runtime orchestration | agent-native, converter, using-atm |
| **library** | Internal | Reference skills loaded JIT by other skills | standards, shared |
| **background** | Internal | Hook-triggered or automatic skills | inject, forge, flywheel |
| **meta** | Internal | Skills about skills | heal-skill, skill-auditor, skill-builder |

## The Three Categories

### Validation — the foundation (tier: judgment)

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
  │ pre-mortem │        │validate │         │ post-mortem │
  │ (plans)    │        │ (code)  │         │ (knowledge  │
  └────────────┘        └────┬────┘         │ + knowledge)│
                             │              └─────────────┘
                             ▼
                       ┌────────────┐
                       │  checks    │
                       └────────────┘
```

### Primitives and flows — the work (tier: execution)

Skills that move work through the system. Swarm parallelizes them. Flows like RPI chain them into a repeatable delivery path.

```
RESEARCH          PLAN              IMPLEMENT           VALIDATE
────────          ────              ─────────           ────────

┌──────────┐    ┌──────────┐      ┌───────────┐      ┌──────────┐
│ research │───►│   plan   │─────►│ implement │─────►│ validate │
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
                                   │  swarm  │      │ validate  │
                                   └────┬────┘      │ + council │
                                        │           └───────────┘
                                        ▼
                                   ┌─────────┐
                                   │  crank  │
                                   └─────────┘

POST-SHIP                             ONBOARDING / STATUS
─────────                             ───────────────────

┌─────────────┐                       ┌────────────┐
│ post-mortem │                       │ bootstrap  │ (first-time setup)
│ (council +  │                       └────────────┘
│ knowledge)  │                       ┌────────────┐
└──────┬──────┘                       │   status   │ (dashboard)
       │                              └────────────┘
       ▼
┌─────────────┐
│   release   │ (changelog, version bump, tag)
└─────────────┘
```

### Bookkeeping — the flywheel (tier: knowledge)

Append-only ledger in `.agents/`. Every session writes. Freshness decay prunes. Next session injects the best. This is the bookkeeping layer that makes sessions compound instead of starting from scratch.

```
┌─────────┐     ┌─────────┐     ┌──────────┐     ┌──────────┐
│post-mortem│──►│  forge  │────►│ compile  │────►│  inject  │
└─────────┘     └─────────┘     └──────────┘     └──────────┘
     ▲                                                 │
     │              ┌──────────┐                       │
     └──────────────│ flywheel │◄──────────────────────┘
                    └──────────┘

User-facing: /compile (query + grow), /post-mortem --quick (quick-capture), /post-mortem (full), /flywheel
Background:  inject, forge, flywheel
CLI:         ao lookup, ao extract, ao forge, ao maturity
```

## Which Skill Should I Use?

Start here. Match your intent to a skill.

```
What are you trying to do?
│
├─ "Prove it's done / validate" (the Membrane — no verdict = not done)
│   ├─ Code ready to ship? ───────► /validate
│   ├─ Plan ready to build? ──────► /pre-mortem
│   ├─ Independent judges ────────► /council --quick validate
│   ├─ Adversarial probe ─────────► /red-team
│   ├─ Root-cause a bug ──────────► /review
│   ├─ Drive fixes to agreement ──► /converge
│   ├─ Mid-epic drift check ──────► /reality-check
│   ├─ Security + release gate ───► /security audit
│   ├─ Landing 100+ files? ───────► /pre-land-refuters
│   └─ Work ready to close? ──────► /post-mortem
│
├─ "Track it / bookkeep it" (the Bookkeeper)
│   ├─ Break it into issues ──────► /plan
│   ├─ Manage/close issues ───────► /beads-br
│   ├─ Shape a fuzzy idea ────────► /discovery
│   ├─ Build a single issue ──────► /implement
│   ├─ Where was I? ──────────────► /status
│   ├─ Save for next session ─────► /handoff
│   └─ Recover after compaction ──► /recover
│
├─ "Build a feature"
│   ├─ Small (1-2 files) ─────────► /implement
│   ├─ Medium (3-6 issues) ───────► /plan → /crank
│   └─ Large (7+ issues) ─────────► /rpi (full pipeline)
│
├─ "Fix a bug"
│   ├─ Know which file? ──────────► /implement <issue-id>
│   └─ Need to investigate? ──────► /review
│
├─ "Explore or research"
│   ├─ Understand this codebase ──► /research
│   ├─ Compare approaches ────────► /council research <topic>
│   └─ Generate ideas ────────────► /discovery
│
├─ "Learn from past work"
│   ├─ What do we know about X? ──► /compile <query>
│   ├─ Save this insight ─────────► /post-mortem --quick "insight"
│   ├─ Full retrospective ────────► /post-mortem
│   └─ Trace a decision ─────────► /recover <concept>
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
│   ├─ Full health check ────────► /security audit
│   ├─ Update dependencies ──────► /security update
│   ├─ Vulnerability scan ───────► /security vuln
│   └─ License compliance ───────► /security license
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
│   └─ Full PR workflow ──────────► /pr-prep → /plan → /implement
│
├─ "Ship a release"
│   └─ Changelog + tag ──────────► /release <version>
│
├─ "Parallelize work"
│   ├─ Multiple independent tasks ► /swarm
│   ├─ Codex agents specifically ─► /swarm
│   └─ Full epic with waves ──────► /crank <epic-id>
│
└─ "First time here"
    └─ Interactive tour ──────────► /status
```

### Composition patterns

These are how skills chain in practice:

| Pattern | Chain | When |
|---------|-------|------|
| **Quick fix** | `/implement` | One issue, clear scope |
| **Quick ship** | `/implement` → `/push` | Implement, test, and push |
| **Validated fix** | `/implement` → `/validate` | One issue, want confidence |
| **Planned epic** | `/plan` → `/pre-mortem` → `/crank` → `/post-mortem` | Multi-issue, structured |
| **Full pipeline** | `/rpi` (chains all above) | End-to-end, autonomous |
| **Evolve loop** | `/evolve` (chains `/rpi` repeatedly) | Fitness-scored improvement |
| **PR contribution** | `/pr-prep` → `/plan` → `/implement` → `/validate --mode=pr` → `/pr-prep` | External repo |
| **Knowledge query** | `/compile` → `/research` (if gaps) | Understanding before building |
| **Standalone review** | `/council validate <target>` | Ad-hoc multi-judge review |
| **Time-boxed pipeline** | `/rpi --budget=research:180,plan:120` | Prevent research/plan stalls |
| **TDD feature** | `/implement <issue>` | TDD-first by default (skip with `--no-tdd`) |
| **Scoped parallel** | `/crank <epic>` | Auto file-ownership map prevents conflicts |
| **Test-first build** | `/test --tdd` → `/implement` | Write tests before code |
| **Reviewed PR** | `/review <PR>` → approve/request changes | Incoming PR review |
| **Safe refactor** | `/refactor` → `/refactor` → `/test` | Find hotspots, refactor, verify |
| **Dep hygiene** | `/security audit` → `/security update` → `/test` | Audit, update, verify |
| **Perf cycle** | `/perf profile` → `/perf optimize` → `/perf compare` | Profile, fix, verify |
| **New project** | `/scaffold` → `/test` → `/push` | Bootstrap, verify, ship |

---

## Current Skill Tiers

### User-Facing Skills (70)

**Judgment:**

| Skill | Tier | Description |
|-------|------|-------------|
| **council** | judgment | Multi-model validation (core primitive) — independent judges debate and converge |
| **validate** | judgment | Canonical validator role — produce PASS/WARN/FAIL verdicts for artifacts, plans, code, PRs, and gates |
| **pre-land-refuters** | judgment | Use before landing any 100+ file change: dispatch unbiased Fable + codex refuters to attack the completion claim. |
| **pre-mortem** | judgment | Council on plans — simulate failures before implementation |
| **post-mortem** | judgment | Council + knowledge lifecycle — validate completed work, extract/activate/retire learnings |
| **review** | judgment | Review incoming PRs, agent-generated changes, or diffs — SCORED checklist |
| **red-team** | judgment | Persona-based adversarial validation — probe docs and skills from constrained user perspectives |

**Execution:**

| Skill | Tier | Description |
|-------|------|-------------|
| **research** | execution | Deep codebase exploration |
| **plan** | execution | Decompose epics into issues with dependency waves |
| **implement** | execution | Full lifecycle for one task |
| **crank** | execution | Autonomous epic execution — parallel waves |
| **discovery** | meta | Discovery phase orchestrator — ideate → search → research → plan → pre-mortem |
| **swarm** | execution | Parallelize any skill — fresh context per agent |
| **using-atm** | execution | Run AgentOps loops out of session on an ATM tmux swarm — the ATM leg of the substrate |
| **rpi** | meta | Thin wrapper: /discovery → /crank → /validate with complexity classification and loop |
| **evolve** | execution | Autonomous fitness-scored improvement loop |
| **eval-outcomes** | execution | Grade via Outcomes as a holdout-safe projection of the locked eval substrate — one bar, many runtimes |
| **autodev** | execution | PROGRAM.md autonomous development contract setup and validation |
| **push** | execution | Atomic test-commit-push workflow — tests, commits, rebases, pushes |
| **test** | execution | Test generation, coverage analysis, and TDD workflow |
| **refactor** | execution | Safe, verified refactoring with regression testing at each step |
| **perf** | execution | Performance profiling, benchmarking, regression detection, and optimization |
| **scaffold** | execution | Project scaffolding, component generation, and boilerplate setup |
| **scope** | execution | Edit-scope guard — freeze/unfreeze directories with hard-block PreToolUse hook |

**Knowledge:**

| Skill | Tier | Description |
|-------|------|-------------|
| **compile** | knowledge | Active knowledge intelligence — Mine → Grow → Defrag cycle |
| **domain** | knowledge | Shared vocabulary for human-AI software building (tracer-bullet shape; loaded JIT when terms like vertical slice, tracer bullet, primitive need a canonical definition) |
| **curate** | knowledge | Canonical miner role — mine transcripts, `.agents/`, bd, and git for skill diffs, bd updates, and rare wiki entries |

**Product & Release:**

| Skill | Tier | Description |
|-------|------|-------------|
| **product** | product | Interactive PRODUCT.md generation |
| **goals** | product | Maintain GOALS.yaml fitness specification |
| **release** | product | Pre-flight, changelog, version bumps, tag |
| **security** | product | Continuous security scanning and release gating, plus the composable binary/prompt-surface suite (offline redteam, policy gating) |
| **doc** | product | Generate repo docs (default), gold-standard README (`--mode=readme`, council-validated), and OSS doc packs (`--mode=oss`) |

**Session & Status:**

| Skill | Tier | Description |
|-------|------|-------------|
| **handoff** | session | Session handoff — save context for next session |
| **recover** | session | Post-compaction context recovery |
| **status** | session | Single-screen dashboard |
| **bootstrap** | session | One-command full AgentOps setup — fills gaps only |

**Upstream Contributions:**

| Skill | Tier | Description |
|-------|------|-------------|
| **pr-prep** | contribute | PR preparation and structured PR body generation |

**Cross-Vendor & Meta:**

| Skill | Tier | Description |
|-------|------|-------------|
| **converter** | cross-vendor | Cross-platform skill converter (Codex, Cursor) |
| **heal-skill** | meta | Detect and fix skill hygiene issues |
| **skill-auditor** | meta | Two-pass audit of an existing SKILL.md against the unified template (15 checks) |
| **skill-builder** | meta | Scaffold or absorb new SKILL.md files against the unified template |
| **automation-shape-routing** | meta | Front door for building automation: route to Workflow vs NTM swarm vs plain skill, then hand off |
| **workflow-builder** | meta | Scaffold a new Claude Workflow script (.claude/workflows/*.js) from the operating-loop.js template |
| **agent-native** | meta | Make out-of-session agents (Managed/SDK/sandbox) AgentOps-native via skills + ao CLI + CI, not hooks |


**Factory-Built Operator And Pack Skills:**

| Skill | Tier | Description |
|-------|------|-------------|
| **acfs** | orchestration | Use when operating ACFS flywheel health checks, init, and agent loop tooling from ~/acfs/bin/acfs. |
| **agent-mail** | execution | Use when coordinating agents with Agent Mail locks, inboxes, threads, and conflict-prevention handoffs. |
| **agy-headless-evidence** | execution | Use when running Antigravity (AGY) headlessly and capturing durable, machine-checkable JSONL evidence of each run. |
| **agy-native** | cross-vendor | Use when driving AgentOps work natively in Google Antigravity with claims, validation, closeout, and persistence. |
| **beads-br** | execution | Local-first issue tracker (beads_rust) for AI agents. Use when tracking tasks, managing dependencies, finding ready work, or syncing issues to git via JSONL. |
| **beads-bv** | execution | Graph-aware task triage with bv and br. Use when prioritizing work, finding bottlenecks, tracking dependencies, or managing local issues across projects. |
| **beads-workflow** | execution | Use when converting markdown plans into br beads with dependencies for implementation or swarm execution. |
| **cass** | execution | Mine past agent sessions for working prompts, decisions, and patterns. Use when "what did I ask?", "find that prompt", session archaeology, or agent history. |
| **cc-hooks** | execution | Configure Claude Code hooks for PreToolUse, PostToolUse, Stop, Notification. Use when blocking commands, auto-formatting, custom permissions, or writing hooks. |
| **codex-approval** | execution | Use when Codex needs independent Claude/Fable approval for a plan, design, or high-risk change through an ATM/NTM interactive validator pane. |
| **codex-exec** | orchestration | Use when running Codex workers or validators non-interactively through codex exec with evidence. |
| **dcg** | execution | Handle blocked destructive commands. Use when dcg blocks rm -rf, git reset --hard, DROP DATABASE, kubectl delete, or when configuring agent safety guardrails. |
| **ntm** | execution | Orchestrates NTM tmux agent swarms and robot APIs. Use when spawning/sending panes, reading robot state, triaging work, locks/mail, safety, pipelines, serve, or NTM errors. |
| **rch** | execution | Use when offloading slow builds to remote workers or recovering RCH worker, hook, SSH, sync, or disk issues. |
| **sbh** | execution | Disk-pressure defense for AI coding workloads. Use when: disk full, low space, ballast, cleanup, scan artifacts, emergency, sbh daemon, sbh status. |
| **vibing-with-ntm** | execution | Use when tending NTM agent swarms, unsticking panes, handling rate limits, or coordinating convergence. |
| **account-rotation** | execution | "Use when you hit a usage/rate limit on a coding-agent subscription and need to switch accounts, or to spread swarm lanes across accounts. Routes by host+agent: macOS+Claude → claude-acct (Keychain swap); macOS+Codex/Gemini or any Linux/WSL → caam (file swap). One symptom, the right tool per host." |
| **continuity-loop** | execution | Own the unattended renewal spine: renewal ticks, the two-tick stall rule, escalation for NTM panes over MCP Agent Mail. Use when wiring or tuning a loop's continuity step. |
| **operationalize** | execution | Distill context (research, recon, learnings) into evidence-anchored rules routed to automation shapes. Use when a finished artifact should become skills, gates, or beads. |
| **reality-check** | execution | Mid-epic drift audit: code is ground truth; README/PRODUCT/plan are the measuring stick. Use when a wave boundary lands and bead counts look healthy but value feels absent. |
| **toil-mining** | execution | Mine usage history (cass, rtk, shell) for repeated toil, score frequency x pain, emit ranked candidates for automation-shape-routing. Use when rituals repeat by hand. |
| **converge** | execution | Drive a fix→re-run-judge-panel loop to terminal agreement or a 3-consecutive-fail BLOCK via the Go `ao converge` command. Thin memo over the CLI — the loop, the context-quorum floor, the LAW-0 cross-family dispatch table, and the canary entry gate all live in Go. |
| **dual-pane-atm** | execution | 'Repeatable Opus (Claude) + Codex dual-pane ATM collaboration. Triggers: "dual pane", "Opus and Codex together", "CEP duel/build", "two-pane ATM", "collaborative ATM".' |
| **orchestrate** | execution | 'Out-of-session orchestration instrument lane: route, preflight, verify before human atm/am procedure.' |
| **behavior-first-planning** | execution | 'Behavior-first planning discipline — intent → Gherkin behaviors → EXECUTED-red acceptance tests → spec → acceptance-gated bead DAG. No runnable acceptance test, no bead. Triggers: "plan behavior-first", "acceptance-first planning", "give these beads runnable done-criteria".' |
| **reverse-engineer** | execution | 'Reverse-engineer an external system you own or are authorized to analyze — repo, binary, or product — into a mechanically-verifiable feature inventory + spec set, then a steal-map (have/gap/steal/park/reject) onto our own surfaces. Use when evaluating a competitor, upstream, fork, or reference tool for what to adopt. Triggers: "reverse-engineer X", "tear down Y", "what should we steal from Z", "evaluate competitor/upstream", "should we fork/adopt/build-native".' |

### Internal Skills (5) — `metadata.internal: true`

Not auto-loaded — loaded JIT by other skills via Read or auto-triggered by hooks. Loaded JIT by other skills via Read or auto-triggered by hooks.

| Skill | Tier | Category | Purpose |
|-------|------|----------|---------|
| standards | library | Judgment | Coding standards (loaded by /validate, /implement, /doc) |
| shared | library | Execution | Shared reference documents (multi-agent backends) |
| inject | background | Knowledge | Load knowledge at session start (hook-triggered) |
| forge | background | Knowledge | Mine transcripts for knowledge (includes --promote for pending extraction) |
| flywheel | background | Knowledge | Knowledge health monitoring |

---

## Skill Dependency Graph

### Dependency Table

| Skill | Dependencies | Type |
|-------|--------------|------|
| **compile** | - | - (standalone, ao CLI optional) |
| **curate** | - | - (standalone knowledge miner) |
| **operationalize** | compile, flywheel | optional, optional |
| **council** | - | - (core primitive) |
| **validate** | - | - (standalone validator role) |
| **pre-mortem** | council | required |
| **post-mortem** | council, beads-br | required, optional |
| domain | - | - |
| **agent-native** | - | - (standalone runtime guide) |
| **crank** | swarm, validate, implement, beads-br, post-mortem | required, required, required, optional, optional |
| doc | standards | required |
| flywheel | - | - |
| forge | - | - |
| handoff | - | - |
| **implement** | beads-br, standards | optional, required |
| inject | - | - |
| **plan** | research, beads-br, pre-mortem, crank, implement | optional, optional, optional, optional, optional |
| **push** | - | - (standalone) |
| **product** | - | - (standalone) |
| **pr-prep** | validate | optional |
| **bootstrap** | goals, product, doc, shared | all optional (progressive — skips what exists) |
| **discovery** | research, plan, pre-mortem, shared | research+plan+pre-mortem required, shared optional |
| **rpi** | discovery, crank, validate | all required |
| **evolve** | rpi | required (rpi pulls in all sub-skills) |
| **autodev** | evolve, rpi | required |
| **release** | - | - (standalone) |
| **security** | - | - (standalone) |
| **recover** | - | - (standalone) |
| research | inject | optional |
| standards | - | - |
| **goals** | - | - (reads GOALS.yaml directly) |
| **status** | - | - (all CLIs optional) |
| **swarm** | implement, validate | required, optional |
| **test** | standards | required |
| **review** | standards, council | required, optional |
| **refactor** | standards, beads-br | required, optional |
| **perf** | standards | optional |
| **scaffold** | standards | required |

---

## CLI Integration

### Spawning Agents

| Vendor | CLI | Command |
|--------|-----|---------|
| Claude | `claude` | Interactive ATM/NTM pane only; print mode is forbidden |
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
| `/validate` | `.agents/council/YYYY-MM-DD-validate-<target>.md` |
| `/pre-mortem` | `.agents/council/YYYY-MM-DD-pre-mortem-<topic>.md` |
| `/post-mortem` | `.agents/council/YYYY-MM-DD-post-mortem-<topic>.md` |

Individual judge outputs also go to `.agents/council/`:
- `YYYY-MM-DD-<target>-claude-pragmatist.md`, `...-claude-skeptic.md`, `...-claude-visionary.md`
- `YYYY-MM-DD-<target>-codex-pragmatist.md`, `...-codex-skeptic.md`, `...-codex-visionary.md`

---

## Execution Modes

Skills follow an execution-isolation model based on visibility and context cost:

> **The Rule:** Lifecycle orchestration stays visible. Expensive phase execution
> and worker execution isolate behind declared skill contracts and return
> bounded artifacts.

### Tier 1: NO-FORK (stay in main context)

Lifecycle orchestrators and direct single-task executors stay in the main
session so the operator can see progress, phase transitions, and intervene.

| Skill | Role | Why |
|-------|------|-----|
| evolve | Orchestrator | Long loop, need cycle-by-cycle visibility |
| rpi | Orchestrator | Keeps phase order, objective, retries, and operator visibility |
| crank | Direct orchestrator | Wave reports visible when called directly |
| discovery | Direct orchestrator | Gate visibility when called directly |
| validate | Direct orchestrator | Verdict visibility when called directly |
| implement | Single-task | Single issue, medium duration |

### Tier 1.5: PHASE ISOLATION (declared phase contracts)

When `/rpi` calls lifecycle phases, the phase skill contract should run behind
isolated transport when the runtime supports it. `/rpi` stays visible; the
phase context receives only the objective plus bounded handoff artifact and
returns artifact path, verdict, and next action.

| Skill | Role | Why |
|-------|------|-----|
| discovery | Phase 1 contract | Research and planning context should not stay resident through implementation |
| crank | Phase 2 contract | Wave execution context should not stay resident through validation |
| validate | Phase 3 contract | Review and closeout context should not pollute the next lifecycle turn |

### Tier 2: FORK (discovery primitives)

Discovery skills that produce filesystem artifacts. User wants the output, not the process. Heavy codebase exploration and decomposition runs in a forked subagent; only the summary and artifact path return to the caller's context.

| Skill | Role | Why |
|-------|------|-----|
| research | Discovery | Massive codebase exploration → `.agents/research/*.md` |
| plan | Discovery | Decomposition + beads creation → `.agents/plans/*.md` + beads |
| post-mortem | Knowledge extraction | Extract learnings → `.agents/learnings/*.md` |

### Tier 3: FORK (judgment + worker spawners)

Judgment skills validate artifacts in isolation. Worker spawners fan out parallel work. Results merge back via filesystem.

| Skill | Role | Why |
|-------|------|-----|
| pre-mortem | Judgment | Plan validation, user wants verdict |
| post-mortem | Judgment | Validation close-out + knowledge extraction |
| council | Worker spawner | Parallel judges, merge verdicts |
| swarm | Worker spawner | Parallel runtime agents, merge results |

Note: `swarm` is an orchestrator (no `context: fork`) that spawns runtime workers via `TeamCreate`/`spawn_agent`. The workers it creates are runtime sub-agents, not SKILL.md skills.

### Dual-Role Skills

Some skills are orchestrators when called directly but workers when spawned by another skill. The caller determines the role:

- **implement**: Called directly → orchestrator (stays). Spawned by swarm → worker (already forked by swarm).
- **crank**: Called directly -> orchestrator (stays visible). Called by rpi -> phase contract (prefer phase-isolated transport; fallback stays visible but must keep artifact handoffs bounded).

### Mechanism

`context.window` is a declaration, not sufficient enforcement by itself. Use it
for discovery primitives, judgment skills, and worker spawners where the
runtime honors it. For `/rpi` phases, use the
[`phase skill isolation contract`](rpi/references/isolation-contract.md): the
orchestrator stays visible while the phase contract runs through isolated
transport and returns only bounded artifacts.

---

## See Also

- `skills/council/SKILL.md` — Core judgment primitive
- `skills/validate/SKILL.md` — Complexity + council for code
- `skills/pre-mortem/SKILL.md` — Council for plans
- `skills/post-mortem/SKILL.md` — Council + knowledge closeout for wrap-up
- `skills/swarm/SKILL.md` — Parallelize any skill
- `skills/rpi/SKILL.md` — Full pipeline orchestrator
