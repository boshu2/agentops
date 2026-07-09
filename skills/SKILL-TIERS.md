# Skill Tier Taxonomy

This document defines the internal `tier` field used in skill frontmatter. Publicly, AgentOps talks about bookkeeping, validation, primitives, and flows. The tier names below are the internal execution taxonomy behind that operating model.

## The Spine (Membrane + Bookkeeper)

Two skill sets lead the operating model. The **Membrane** proves each change is
actually done — no verdict, not done. The **Bookkeeper** tracks the work between
your head and *done*. Everything else (research, plan, build, ship) runs *through*
these two, and the router below leads with them.

**The Membrane — validation spine (no verdict = not done):**

- `/validate` — canonical PASS/WARN/FAIL verdict on artifacts, plans, code, PRs, and gates; absorbs the retired `/review` (`--mode=pr`) and adversarial `--debate` (ex-`/red-team`, retired).
- `/council` — multi-judge consensus; the core primitive under every validation skill.
- `/pre-mortem` — simulate failures before implementing; predictions tracked into validate.
- `/converge` — drive a fix → re-run-judge-panel loop to terminal agreement or a hard BLOCK.
- `/security` — repository security scans (vulns, dependency risk, secrets) plus release gating.
- `/reality-check` — mid-epic drift audit: code is ground truth, the plan is the measuring stick.
- `/pre-land-refuters` — fresh-context refuters attack the completion claim at the shared-trunk pawl before landing.

**The Bookkeeper — tracking + session spine:**

- `/beads-br` — local-first, git-native issue tracker (find ready work, update, close).
- `/status` — single-screen dashboard of project state.
- `/handoff` — compact session handoff so the next turn continues instead of restarting.
- `/discovery` — shape intent into a dense execution packet (ideate → search → research → plan → pre-mortem).
- `/goal-design` — create checked `.agents/goal-design/<slug>/` intent and driver packets before discovery or planning.
- `/plan` — decompose a goal into an acceptance-gated bead DAG with dependency waves.
- `/implement` — execute a single bead through its full TDD lifecycle.

## Behavioral Probe Ledger (MEASURED)

> Tiers below are the **editorial** taxonomy. This ledger is the **measured**
> column: for each probed skill, whether loading it actually **changed agent
> behavior** in a control-vs-treatment A/B — `BEHAVIORAL` (it did), `INERT` (it
> didn't), or `UNMEASURED` (no probe). **A probe measures behavior-change, not
> quality-uplift** (ADR-0011 discipline — do not overclaim). Harness:
> `scripts/probe-skill.sh` over `evals/skill-probes/<id>/`; the advisory gate
> `skill.probe-coverage` NAMES every product-/judgment-tier skill still absent
> here. Spine first (the workflow start set), ratchet does the rest — not all
> 100+ skills.

| Skill | Probe ID | Date | Verdict | Evidence |
|-------|----------|------|---------|----------|
| crank | crank | 2026-07-08 | INERT | `docs/evals/2026-07-08-skill-probe-crank.md` — frontier (gpt-5.5) separated the write-scope-colliding beads in BOTH arms (2/2 each); loading crank changed nothing at this task altitude. Needs a weaker producer or harder task to surface value. |
| graphify | graphify-tool-preference | 2026-07-08 | INERT | `docs/evals/2026-07-08-skill-probe-graphify-calibration.md` — calibration reproducing the 2026-06-30 A/B (0/2 treatment used the tool). |

## Tier Values

Skills fall into three functional categories, plus infrastructure tiers for internal and library skills.

| Tier | Category | Description | Examples |
|------|----------|-------------|----------|
| **judgment** | Validation | Internal tier for validation, review, and quality gates — council is the foundation | council, validate, pre-mortem, post-mortem |
| **execution** | Primitives + flows | Research, plan, build, and ship — the work itself | research, plan, implement, crank, swarm, rpi |
| **knowledge** | Bookkeeping | The flywheel — capture, store, query, inject, and promote learnings | domain |
| **product** | Execution | Define mission, goals, release, docs | product, goals, release, doc |
| **session** | Execution | Session continuity and status | handoff, status |
| **utility** | Execution | Standalone tools | converter, scaffold, security |
| **contribute** | Execution | Upstream PR workflow | pr-prep |
| **cross-vendor** | Execution | Multi-runtime orchestration | agent-native, converter, using-atm |
| **library** | Internal | Reference skills loaded JIT by other skills | standards, shared |
| **background** | Internal | Hook-triggered or automatic skills | (none active) |
| **meta** | Internal | Skills about skills | heal-skill, skill-builder |
| **experimental** | Internal | Heavy legacy loops kept but demoted (heavy rpi chains, no measured uplift) | evolve, operationalize |

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
┌───────────┐     ┌────────────┐     ┌───────────┐
│post-mortem│───► │ ao compile │───► │ ao lookup │
└───────────┘     └────────────┘     └───────────┘
      ▲                                     │
      │           ┌────────────────┐        │
      └───────────│ ao flywheel    │◄───────┘
                  │    status      │
                  └────────────────┘

User-facing: /post-mortem --quick (quick-capture), /post-mortem (full — mines + compiles the corpus)
CLI:         ao compile, ao flywheel status, ao lookup, ao extract, ao forge, ao maturity
```

## Which skill should I use?

One curated router exists: [docs/SKILLS.md](../docs/SKILLS.md) — the decision tree and
composition patterns live THERE (single owner; the duplicate tree formerly here drifted
and was removed 2026-07-07, age-skills-audit-fable-l6ic.6). This file owns the TIER
taxonomy and the tier tables below, nothing else.

## Current Skill Tiers

### User-Facing Skills (57)

**Judgment:**

| Skill | Tier | Description |
|-------|------|-------------|
| **council** | judgment | Multi-model validation (core primitive) — independent judges debate and converge |
| **validate** | judgment | Canonical validator role — produce PASS/WARN/FAIL verdicts for artifacts, plans, code, PRs, and gates |
| **pre-land-refuters** | judgment | Use before landing any 100+ file change: dispatch unbiased Fable + codex refuters to attack the completion claim. |
| **pre-mortem** | judgment | Council on plans — simulate failures before implementation |
| **post-mortem** | judgment | Council + knowledge lifecycle — validate completed work, extract/activate/retire learnings |

**Execution:**

| Skill | Tier | Description |
|-------|------|-------------|
| **research** | execution | Deep codebase exploration |
| **plan** | execution | Decompose epics into issues with dependency waves |
| **implement** | execution | Full lifecycle for one task |
| **crank** | execution | Autonomous epic execution — parallel waves |
| **discovery** | meta | Discovery phase orchestrator — ideate → search → research → plan → pre-mortem |
| **goal-design** | execution | Create checked goal-design intent + driver packets before discovery or planning |
| **swarm** | execution | Parallelize any skill — fresh context per agent |
| **using-atm** | execution | Run AgentOps loops out of session on an ATM tmux swarm — the ATM leg of the substrate |
| **rpi** | meta | Thin wrapper: /discovery → /crank → /validate with complexity classification and loop |
| **evolve** | experimental | Autonomous fitness-scored improvement loop |
| **push** | execution | Atomic test-commit-push workflow — tests, commits, rebases, pushes |
| **test** | execution | Test generation, coverage analysis, and TDD workflow |
| **refactor** | execution | Safe, verified refactoring with regression testing at each step |
| **scaffold** | execution | Project scaffolding, component generation, and boilerplate setup |
| **scope** | execution | Edit-scope guard — freeze/unfreeze directories with hard-block PreToolUse hook |

**Knowledge:**

| Skill | Tier | Description |
|-------|------|-------------|
| **domain** | knowledge | Shared vocabulary for human-AI software building (tracer-bullet shape; loaded JIT when terms like vertical slice, tracer bullet, primitive need a canonical definition) |

**Product & Release:**

| Skill | Tier | Description |
|-------|------|-------------|
| **product** | product | Interactive PRODUCT.md generation |
| **goals** | product | Maintain GOALS.md fitness specification |
| **release** | product | Pre-flight, changelog, version bumps, tag |
| **security** | product | Continuous security scanning and release gating, plus the composable binary/prompt-surface suite (offline redteam, policy gating) |
| **doc** | product | Generate repo docs (default), gold-standard README (`--mode=readme`, council-validated), and OSS doc packs (`--mode=oss`) |

**Session & Status:**

| Skill | Tier | Description |
|-------|------|-------------|
| **handoff** | session | Session handoff — save context for next session |
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
| **skill-builder** | meta | Scaffold or absorb new SKILL.md files against the unified template |
| **automation-shape-routing** | meta | Front door for building automation: route to Workflow vs NTM swarm vs plain skill, then hand off |
| **workflow-builder** | meta | Scaffold a new Claude Workflow script (.claude/workflows/*.js) from the operating-loop.js template |
| **agent-native** | meta | Make out-of-session agents (Managed/SDK/sandbox) AgentOps-native via skills + ao CLI + CI, not hooks |


**Factory-Built Operator And Pack Skills:**

| Skill | Tier | Description |
|-------|------|-------------|
| **agent-mail** | execution | Use when coordinating agents with Agent Mail locks, inboxes, threads, and conflict-prevention handoffs. |
| **agy-native** | cross-vendor | Use when driving AgentOps work natively in Google Antigravity with claims, validation, closeout, and persistence. |
| **beads-br** | execution | Local-first issue tracker (beads_rust) for AI agents. Use when tracking tasks, managing dependencies, finding ready work, or syncing issues to git via JSONL. |
| **beads-bv** | execution | Graph-aware task triage with bv and br. Use when prioritizing work, finding bottlenecks, tracking dependencies, or managing local issues across projects. |
| **cass** | execution | Mine past agent sessions for working prompts, decisions, and patterns. Use when "what did I ask?", "find that prompt", session archaeology, or agent history. |
| **cc-hooks** | execution | Configure Claude Code hooks for PreToolUse, PostToolUse, Stop, Notification. Use when blocking commands, auto-formatting, custom permissions, or writing hooks. |
| **codex-exec** | orchestration | Use when running Codex workers or validators non-interactively through codex exec with evidence. |
| **dcg** | execution | Handle blocked destructive commands. Use when dcg blocks rm -rf, git reset --hard, DROP DATABASE, kubectl delete, or when configuring agent safety guardrails. |
| **ms** | execution | meta_skill (ms) — the skill-search/load engine over both corpora (agentops + jsm). Use when you need to find a skill for a task, search skills, or load runnable skill guidance. Consume via MCP, write/admin via CLI. |
| **ntm** | execution | Orchestrates NTM tmux agent swarms and robot APIs. Use when spawning/sending panes, reading robot state, triaging work, locks/mail, safety, pipelines, serve, or NTM errors. |
| **using-gc** | cross-vendor | Drive a Gas City factory day-to-day: stand up a correct native city, sling quests, watch the membrane close gate, resolve the known stalls, read pawl-verdict.v1, converge. The vibing-with-ntm analog for gc — operator choice, coexists with NTM. |
| **gc-membrane** | library | Reference for the agentops-membrane Gas City pack: close-gate mechanics, finalize semantics (nonce, ≥2 families, DEGRADED), pawl-verdict.v1 anatomy, trinity RBAC. Loaded JIT by using-gc. |
| **rch** | execution | Use when offloading slow builds to remote workers or recovering RCH worker, hook, SSH, sync, or disk issues. |
| **sbh** | execution | Disk-pressure defense for AI coding workloads. Use when: disk full, low space, ballast, cleanup, scan artifacts, emergency, sbh daemon, sbh status. |
| **account-rotation** | execution | "Use when you hit a usage/rate limit on a coding-agent subscription and need to switch accounts, or to spread swarm lanes across accounts. Routes by host+agent: macOS+Claude → claude-acct (Keychain swap); macOS+Codex/Gemini or any Linux/WSL → caam (file swap). One symptom, the right tool per host." |
| **operationalize** | experimental | Distill context (research, recon, learnings) into evidence-anchored rules routed to automation shapes. Use when a finished artifact should become skills, gates, or beads. |
| **reality-check** | execution | Mid-epic drift audit: code is ground truth; README/PRODUCT/plan are the measuring stick. Use when a wave boundary lands and bead counts look healthy but value feels absent. |
| **toil-mining** | execution | Mine usage history (cass, rtk, shell) for repeated toil, score frequency x pain, emit ranked candidates for automation-shape-routing. Use when rituals repeat by hand. |
| **converge** | execution | Drive a fix→re-run-judge-panel loop to terminal agreement or a 3-consecutive-fail BLOCK via the Go `ao converge` command. Thin memo over the CLI — the loop, the context-quorum floor, the LAW-0 cross-family dispatch table, and the canary entry gate all live in Go. |
| **behavior-first-planning** | execution | 'Behavior-first planning discipline — intent → Gherkin behaviors → EXECUTED-red acceptance tests → spec → acceptance-gated bead DAG. No runnable acceptance test, no bead. Triggers: "plan behavior-first", "acceptance-first planning", "give these beads runnable done-criteria".' |
| **reverse-engineer** | execution | 'Reverse-engineer an external system you own or are authorized to analyze — repo, binary, or product — into a mechanically-verifiable feature inventory + spec set, then a steal-map (have/gap/steal/park/reject) onto our own surfaces. Use when evaluating a competitor, upstream, fork, or reference tool for what to adopt. Triggers: "reverse-engineer X", "tear down Y", "what should we steal from Z", "evaluate competitor/upstream", "should we fork/adopt/build-native".' |

### Internal Skills (2) — `metadata.internal: true`

Not auto-loaded — loaded JIT by other skills via Read or auto-triggered by hooks. Loaded JIT by other skills via Read or auto-triggered by hooks.

| Skill | Tier | Category | Purpose |
|-------|------|----------|---------|
| standards | library | Judgment | Coding standards (loaded by /validate, /implement, /doc) |
| shared | library | Execution | Shared reference documents (multi-agent backends) |

---

## Skill Dependency Graph

### Dependency Table

| Skill | Dependencies | Type |
|-------|--------------|------|
| **operationalize** | - | - (standalone; ao compile / ao flywheel CLIs optional) |
| **council** | - | - (core primitive) |
| **validate** | - | - (standalone validator role) |
| **pre-mortem** | council | required |
| **post-mortem** | council, beads-br | required, optional |
| domain | - | - |
| **agent-native** | - | - (standalone runtime guide) |
| **crank** | swarm, validate, implement, beads-br, post-mortem | required, required, required, optional, optional |
| doc | standards | required |
| handoff | - | - |
| **implement** | beads-br, standards | optional, required |
| **plan** | research, beads-br, pre-mortem, crank, implement | optional, optional, optional, optional, optional |
| **push** | - | - (standalone) |
| **product** | - | - (standalone) |
| **pr-prep** | validate | optional |
| **bootstrap** | goals, product, doc, shared | all optional (progressive — skips what exists) |
| **discovery** | research, plan, pre-mortem, shared | research+plan+pre-mortem required, shared optional |
| **goal-design** | validate, discovery, plan | validate required after checker; discovery/plan consume checked packets |
| **rpi** | discovery, crank, validate | all required |
| **evolve** | rpi | required (rpi pulls in all sub-skills) |
| **release** | - | - (standalone) |
| **security** | - | - (standalone) |
| research | - | - |
| standards | - | - |
| **goals** | - | - (reads GOALS.md directly) |
| **status** | - | - (all CLIs optional) |
| **swarm** | implement, validate | required, optional |
| **test** | standards | required |
| **refactor** | standards, beads-br | required, optional |
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
| Claude | Opus 4.8 |
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
| goal-design | Packet authoring | Digest refresh + checker before handoff |
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
| goal-design | Pre-discovery contract | Intent and driver identity should be checked before planning |
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
