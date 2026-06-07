# Skill Tier Taxonomy

This document defines the internal `tier` field used in skill frontmatter. Publicly, AgentOps talks about bookkeeping, validation, primitives, and flows. The tier names below are the internal execution taxonomy behind that operating model.

## Tier Values

Skills fall into three functional categories, plus infrastructure tiers for internal and library skills.

| Tier | Category | Description | Examples |
|------|----------|-------------|----------|
| **judgment** | Validation | Internal tier for validation, review, and quality gates — council is the foundation | council, vibe, pre-mortem, post-mortem, red-team |
| **execution** | Primitives + flows | Research, plan, build, and ship — the work itself | research, plan, implement, crank, swarm, rpi |
| **knowledge** | Bookkeeping | The flywheel — capture, store, query, inject, and promote learnings | retro (quick-capture), flywheel, forge |
| **product** | Execution | Define mission, goals, release, docs | product, goals, release, doc |
| **session** | Execution | Session continuity and status | handoff, recover, status, session-bootstrap |
| **utility** | Execution | Standalone tools | quickstart, brainstorm, bug-hunt, complexity |
| **contribute** | Execution | Upstream PR workflow | pr-research, pr-implement, pr-validate, pr-prep |
| **cross-vendor** | Execution | Multi-runtime orchestration | codex-team, converter |
| **library** | Internal | Reference skills loaded JIT by other skills | beads, standards, shared |
| **background** | Internal | Hook-triggered or automatic skills | inject, extract, forge, ratchet |
| **meta** | Internal | Skills about skills | using-agentops, heal-skill |

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
  │ pre-mortem │        │  vibe   │         │ post-mortem │
  │ (plans)    │        │ (code)  │         │ (full retro │
  └────────────┘        └────┬────┘         │ + knowledge)│
                             │              └─────────────┘
                             ▼
                       ┌────────────┐
                       │ complexity │
                       └────────────┘
```

### Primitives and flows — the work (tier: execution)

Skills that move work through the system. Swarm parallelizes them. Flows like RPI chain them into a repeatable delivery path.

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
                                        │           └───────────┘
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

### Bookkeeping — the flywheel (tier: knowledge)

Append-only ledger in `.agents/`. Every session writes. Freshness decay prunes. Next session injects the best. This is the bookkeeping layer that makes sessions compound instead of starting from scratch.

```
┌─────────┐     ┌─────────┐     ┌──────────┐     ┌──────────┐
│  retro  │────►│  forge  │────►│ compile  │────►│  inject  │
└─────────┘     └─────────┘     └──────────┘     └──────────┘
     ▲                                                 │
     │              ┌──────────┐                       │
     └──────────────│ flywheel │◄──────────────────────┘
                    └──────────┘

User-facing: /compile (query + grow), /post-mortem --quick (quick-capture), /post-mortem (full), /flywheel
Background:  inject, forge, ratchet
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
│   ├─ What do we know about X? ──► /compile <query>
│   ├─ Save this insight ─────────► /post-mortem --quick "insight"
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
│   └─ Full PR workflow ──────────► /pr-research → /plan → /pr-implement
│
├─ "Ship a release"
│   └─ Changelog + tag ──────────► /release <version>
│
├─ "Parallelize work"
│   ├─ Multiple independent tasks ► /swarm
│   ├─ Codex agents specifically ─► /swarm
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
| **PR contribution** | `/pr-research` → `/plan` → `/pr-implement` → `/validate --mode=pr` → `/pr-prep` | External repo |
| **Knowledge query** | `/compile` → `/research` (if gaps) | Understanding before building |
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

### User-Facing Skills (150)

**Judgment:**

| Skill | Tier | Description |
|-------|------|-------------|
| **council** | judgment | Multi-model validation (core primitive) — independent judges debate and converge |
| **validate** | judgment | Canonical validator role — produce PASS/WARN/FAIL verdicts for artifacts, plans, code, PRs, and gates |
| **vibe** | judgment | Complexity analysis + council — code quality review |
| **pre-mortem** | judgment | Council on plans — simulate failures before implementation |
| **post-mortem** | judgment | Council + knowledge lifecycle — validate completed work, extract/activate/retire learnings |
| **review** | judgment | Review incoming PRs, agent-generated changes, or diffs — SCORED checklist |
| **design** | judgment | Product validation gate — checks goal alignment, persona fit, competitive differentiation before discovery |
| **red-team** | judgment | Persona-based adversarial validation — probe docs and skills from constrained user perspectives |

**Execution:**

| Skill | Tier | Description |
|-------|------|-------------|
| **research** | execution | Deep codebase exploration |
| **brainstorm** | execution | Structured idea exploration before planning |
| **plan** | execution | Decompose epics into issues with dependency waves |
| **implement** | execution | Full lifecycle for one task |
| **crank** | execution | Autonomous epic execution — parallel waves |
| **discovery** | meta | Discovery phase orchestrator — brainstorm → search → research → plan → pre-mortem |
| **swarm** | execution | Parallelize any skill — fresh context per agent |
| **using-ntm** | execution | Run AgentOps loops out of session on an NTM tmux swarm — the NTM leg of the substrate |
| **rpi** | meta | Thin wrapper: /discovery → /crank → /validate with complexity classification and loop |
| **evolve** | execution | Autonomous fitness-scored improvement loop |
| **burndown** | execution | Bounded epic-completion loop — drive a finite target to all-merged, then stop |
| **eval-outcomes** | execution | Grade via Outcomes as a holdout-safe projection of the locked eval substrate — one bar, many runtimes |
| **operating-loop-workflow** | execution | Install + run the operating-loop multi-agent Workflow (seven-move loop) for plugin users |
| **autodev** | execution | PROGRAM.md autonomous development contract setup and validation |
| **bug-hunt** | execution | Investigate bugs with git archaeology |
| **complexity** | execution | Cyclomatic complexity analysis |
| **push** | execution | Atomic test-commit-push workflow — tests, commits, rebases, pushes |
| **ship-loop** | execution | Bot-paired fast lane PR cycle — single-scenario internal PR through auto-merge |
| **test** | execution | Test generation, coverage analysis, and TDD workflow |
| **refactor** | execution | Safe, verified refactoring with regression testing at each step |
| **deps** | execution | Dependency audit, update, vulnerability scanning, and license compliance |
| **perf** | execution | Performance profiling, benchmarking, regression detection, and optimization |
| **scaffold** | execution | Project scaffolding, component generation, and boilerplate setup |
| **scenario** | execution | Author and manage holdout scenarios for behavioral validation |
| **scope** | execution | Edit-scope guard — freeze/unfreeze directories with hard-block PreToolUse hook |
| **system-tuning** | utility | Restore system responsiveness via safe, ordered process cleanup and agent-swarm hygiene |

**Knowledge:**

| Skill | Tier | Description |
|-------|------|-------------|
| **compile** | knowledge | Active knowledge intelligence — Mine → Grow → Defrag cycle |
| **domain** | knowledge | Shared vocabulary for human-AI software building (tracer-bullet shape; loaded JIT when terms like vertical slice, tracer bullet, primitive need a canonical definition) |
| **curate** | knowledge | Canonical miner role — mine transcripts, `.agents/`, bd, and git for skill diffs, bd updates, and rare wiki entries |
| **trace** | knowledge | Trace design decisions through history |

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
| **quickstart** | session | Interactive onboarding |
| **bd-first-memory-migration** | knowledge | Consolidate fragmented memory layers onto a bd-canonical store |
| **bootstrap** | session | One-command full AgentOps setup — fills gaps only |
| **session-bootstrap** | session | Universal init prompt — every agent runs this first (soc-vuu6.25) |

**Upstream Contributions:**

| Skill | Tier | Description |
|-------|------|-------------|
| **pr-research** | contribute | Upstream repository research before contribution |
| **pr-implement** | contribute | Fork-based implementation for external PRs |
| **pr-prep** | contribute | PR preparation and structured PR body generation |

**Cross-Vendor & Meta:**

| Skill | Tier | Description |
|-------|------|-------------|
| **converter** | cross-vendor | Cross-platform skill converter (Codex, Cursor) |
| **reverse-engineer-rpi** | execution | Reverse-engineer a product into feature catalog + code map + specs |
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
| **agy-mcp-plugins** | execution | Use when wiring MCP servers and packaging/installing plugins into the Antigravity (AGY) image so an AGY worker reaches the AgentOps tool substrate. |
| **agy-native** | cross-vendor | Use when driving AgentOps work natively in Google Antigravity with claims, validation, closeout, and persistence. |
| **agy-rules-workflows** | orchestration | Use when installing or validating AgentOps rules and workflows for Google Antigravity. |
| **artifact-clarity-pass** | judgment | Use when removing generic filler from code, docs, or handoffs while preserving every load-bearing fact. |
| **automation-loop-hardening** | execution | Use when turning repeated manual operations into safer, observable, reusable automation loops. |
| **bead-completion-audit** | judgment | Use when auditing closed beads for real shipped evidence, acceptance proof, and truthful closeout. |
| **bead-tracker-migration** | execution | Use when migrating an issue tracker workspace from bd to br with loss-free verification. |
| **beads-br** | execution | Local-first issue tracker (beads_rust) for AI agents. Use when tracking tasks, managing dependencies, finding ready work, or syncing issues to git via JSONL. |
| **beads-bv** | execution | Graph-aware task triage with bv and br. Use when prioritizing work, finding bottlenecks, tracking dependencies, or managing local issues across projects. |
| **beads-workflow** | execution | Use when converting markdown plans into br beads with dependencies for implementation or swarm execution. |
| **behavior-preserving-simplification** | execution | Use when simplifying code, reducing duplication, or clarifying flow while preserving behavior with tests. |
| **caam** | execution | Use when switching AI coding CLI accounts quickly to recover from subscription rate limits or OAuth friction. |
| **casr** | execution | Cross Agent Session Resumer. Convert and resume sessions across Claude Code, Codex, Gemini, and other providers. |
| **cass** | execution | Mine past agent sessions for working prompts, decisions, and patterns. Use when "what did I ask?", "find that prompt", session archaeology, or agent history. |
| **cass-memory** | execution | Use when starting non-trivial work, mining lessons, or preventing repeated mistakes with cm procedural memory. |
| **cc-cron-ticks** | orchestration | Use when scheduling autonomous in-session flywheel ticks with Claude Code cron routines. |
| **cc-hooks** | execution | Configure Claude Code hooks for PreToolUse, PostToolUse, Stop, Notification. Use when blocking commands, auto-formatting, custom permissions, or writing hooks. |
| **cc-loop-driver** | orchestration | Use when running a Claude-native control-plane tick loop with worker and separate-validator subagents. |
| **cc-subagents** | orchestration | Use when dispatching scoped Claude Code subagents with worktrees, roles, tools, memory, and evidence gates. |
| **cc-worktree-isolation** | orchestration | Use when isolating parallel Claude Code workers in separate git worktrees to prevent file collisions. |
| **changelog-quality-pass** | library | Use when writing or auditing changelogs and release notes for user-facing, semver-aware clarity. |
| **cli-agent-ux-audit** | judgment | Use when improving CLI ergonomics for agents: flags, help, JSON output, exit codes, and robot surfaces. |
| **cli-doctoring-workflow** | judgment | Use when designing or auditing CLI doctor commands, health checks, repair hints, and diagnostic UX. |
| **codebase-briefing-report** | knowledge | Use when producing a shareable architecture, module, metrics, and health report for a codebase. |
| **codebase-risk-audit** | execution | Use when auditing codebase risks with evidence and prioritized remediation. |
| **codex-exec** | orchestration | Use when running Codex workers or validators non-interactively through codex exec with evidence. |
| **codex-goals** | orchestration | Use when using Codex Goals to define an objective once and let Codex iterate until done. |
| **codex-mcp-plugins** | execution | Use when wiring MCP servers or plugins into Codex CLI and the AgentOps Codex skill bundle. |
| **codex-sandbox-evidence** | execution | Use when running codex exec in a least-privilege sandbox with machine-checkable proof. |
| **concurrency-deadlock-remediation** | judgment | Use when finding and fixing deadlocks with lock ordering, reproduction, timeouts, or lock-free alternatives. |
| **contract-conformance-testing** | execution | Use when building conformance tests from specs, contracts, examples, or compatibility matrices. |
| **dcg** | execution | Handle blocked destructive commands. Use when dcg blocks rm -rf, git reset --hard, DROP DATABASE, kubectl delete, or when configuring agent safety guardrails. |
| **dependency-update-safety** | library | Use when updating dependencies safely with changelog review, small batches, tests, and rollback. |
| **expertise-to-procedure** | knowledge | Use when turning tacit expert know-how into a durable skill, playbook, or checklist. |
| **external-search-triage** | library | Use when deciding whether external research is needed and turning cited findings into repo actions. |
| **filesystem-path-rationalization** | execution | Use when rationalizing file or directory layout and updating references without breaking builds. |
| **fuzz-test-design** | execution | Use when designing fuzz, property, randomized, or corpus-based tests and replaying failures. |
| **gcloud** | execution | Google Cloud Platform CLI - manage GCP resources. Use when working with Compute Engine, Cloud Run, GKE, Cloud Functions, Storage, BigQuery, or other GCP services. |
| **gh-actions** | execution | Use when creating GitHub Actions workflows, release automation, checksums, signing, or CI/CD. |
| **gh-cli** | execution | GitHub CLI (gh) for repos, issues, PRs, actions, releases. Use when working with GitHub or running gh commands. |
| **gh-triage-ru** | execution | GitHub issue/PR triage via ru and gh. Use when processing issues, closing PRs (no-contributions policy), or bulk triage. Independent verification required. |
| **golden-artifact-testing** | judgment | Use when designing or repairing golden-file, snapshot, fixture, or generated-artifact tests. |
| **idea-option-forge** | judgment | Use when generating, winnowing, and operationalizing many project improvement options. |
| **implementation-pattern-mining** | judgment | Use when mining repeated codebase patterns and turning them into reusable implementation guidance. |
| **installer-quality-audit** | judgment | Use when auditing install, setup, bootstrap, or update scripts for safe, idempotent behavior. |
| **layered-defect-hunt** | judgment | Use when running systematic multi-pass bug hunting across correctness, edges, concurrency, and failures. |
| **legacy-codebase-recon** | judgment | Investigate unfamiliar legacy code before edits. Triggers: legacy module, unknown repo, risky refactor, trace ownership. |
| **live-service-e2e-testing** | execution | Use when building real-service end-to-end tests with fixtures, cleanup, rate limits, and evidence. |
| **mcp-interface-design** | judgment | Use when designing MCP servers with clear tools, strict schemas, scoped resources, and useful errors. |
| **measured-performance-optimization** | execution | Use when optimizing a hot path from saved profiles, measurements, and verified improvements. |
| **metamorphic-test-design** | judgment | Use when designing metamorphic tests for oracle-poor behavior using invariants and input relations. |
| **multi-model-triangulation** | execution | Cross-validate decisions using multiple AI models (Codex, Gemini, Grok). Use when "get a second opinion", evaluating approaches, or high-stakes decisions. |
| **native-debugger-triage** | execution | Use when debugging native programs or ELF binaries with gdb breakpoints, backtraces, and memory inspection. |
| **ntm** | execution | Orchestrates NTM tmux agent swarms and robot APIs. Use when spawning/sending panes, reading robot state, triaging work, locks/mail, safety, pipelines, serve, or NTM errors. |
| **ntm-browser-test-coordination** | orchestration | Use when coordinating browser or UI tests through NTM panes with screenshots and handoffs. |
| **ntm-review-worker-orchestration** | orchestration | Use when operating an NTM review or analysis worker with bounded inputs and evidence-backed output. |
| **operating-loop-skill** | orchestration | Use when driving one bead end-to-end through claim, work, independent validation, closeout, and persistence. |
| **performance-profile-triage** | execution | Use when investigating slowness with baselines, profiler evidence, and ranked bottlenecks. |
| **planning-workflow** | execution | Comprehensive markdown planning methodology for software projects. Use when starting a new project, creating implementation plans, or refining architecture before coding. |
| **process-triage** | execution | Use when diagnosing runaway processes with the pt wrapper and choosing safe remediation. |
| **production-placeholder-audit** | execution | Use when finding mocks, stubs, fake paths, or placeholders leaking into production code. |
| **project-readme-craft** | library | Use when writing READMEs that help users install, run, test, troubleshoot, and adopt a project. |
| **project-reality-check** | judgment | Use when comparing a project vision to code reality and turning gaps into tracked work. |
| **project-reasoning-lens-analysis** | judgment | Use when analyzing a project through first-principles, systems, adversarial, cost, and user lenses. |
| **rch** | execution | Use when offloading slow builds to remote workers or recovering RCH worker, hook, SSH, sync, or disk issues. |
| **release-readiness-gate** | execution | Use when preparing releases with versioning, changelog, artifacts, smoke tests, tags, and go/no-go. |
| **repeatedly-apply-skill** | execution | Use when applying a skill repeatedly with progressive deepening for iterative improvement. |
| **repository-hygiene-sweep** | execution | Use when cleaning repository branches, worktrees, gc state, large objects, and ignore rules safely. |
| **research-software** | execution | Research software tools via source code, GitHub, web. Use when creating skills, learning new tools, finding undocumented features, or bleeding-edge patterns. |
| **ripgrep-search-discipline** | library | Use when searching code with rg using precise, fast flags instead of slow grep or find patterns. |
| **ru-multi-repo-workflow** | execution | Use when using ru for multi-repo commits, sync, GitHub review, or maintenance automation. |
| **rust-crate-release-readiness** | judgment | Use when preparing a Rust crate release with metadata, semver, docs, tests, packaging, and rollback notes. |
| **rust-port-validation-gauntlet** | library | Use when running a Rust port through build, test, clippy, fmt, miri, fuzz, and bench gates. |
| **rust-search-integration** | library | Use when adding fast lexical or semantic code search to a Rust project with an ergonomic query API. |
| **rust-sqlite-cli-architecture** | judgment | Use when designing Rust CLIs backed by SQLite with migrations, transactions, tests, and data safety. |
| **rust-ub-risk-audit** | judgment | Use when auditing Rust UB risks in unsafe, FFI, raw pointers, layout, or concurrency. |
| **rust-unsafe-boundary-audit** | library | Use when auditing Rust unsafe blocks and FFI boundaries, invariants, tests, and tooling. |
| **sbh** | execution | Disk-pressure defense for AI coding workloads. Use when: disk full, low space, ballast, cleanup, scan artifacts, emergency, sbh daemon, sbh status. |
| **spec-reliability-implementation** | execution | Use when implementing a written spec into a reliable service with acceptance examples and observability. |
| **ssh** | execution | Use when configuring SSH access, keys, tunnels, host diagnostics, or safe remote command workflows. |
| **stash-hygiene-sweep** | execution | Use when auditing git stashes, deciding keep/drop/apply/archive, and clearing confirmed stale entries. |
| **system-performance-remediation** | execution | Use when restoring machine responsiveness from high CPU, memory, IO, cache, or runaway process pressure. |
| **ubs** | execution | Use when reviewing code with UBS for bugs, security issues, AI-generated quality, or pre-commit checks. |
| **vibing-with-ntm** | execution | Use when tending NTM agent swarms, unsticking panes, handling rate limits, or coordinating convergence. |
| **work-contract-portability** | knowledge | Use when designing agent work contracts, handoffs, evidence, and role boundaries across runtimes. |
| **worktree-branch-rationalization** | execution | Use when rationalizing git worktrees and branches into a canonical line without losing useful work. |

### Internal Skills (8) — `metadata.internal: true`

Not auto-loaded — loaded JIT by other skills via Read or auto-triggered by hooks. Loaded JIT by other skills via Read or auto-triggered by hooks.

| Skill | Tier | Category | Purpose |
|-------|------|----------|---------|
| beads | library | Execution | Issue tracking reference (loaded by /implement, /plan) |
| standards | library | Judgment | Coding standards (loaded by /vibe, /implement, /doc) |
| shared | library | Execution | Shared reference documents (multi-agent backends) |
| inject | background | Knowledge | Load knowledge at session start (hook-triggered) |
| forge | background | Knowledge | Mine transcripts for knowledge (includes --promote for pending extraction) |
| ratchet | background | Execution | Progress gates |
| flywheel | background | Knowledge | Knowledge health monitoring |
| using-agentops | meta | Meta | AgentOps workflow guide (auto-injected) |

---

## Skill Dependency Graph

### Dependency Table

| Skill | Dependencies | Type |
|-------|--------------|------|
| **compile** | - | - (standalone, ao CLI optional) |
| **curate** | - | - (standalone knowledge miner) |
| **harvest** | - | - (standalone, ao CLI required) |
| **knowledge-activation** | compile, harvest, flywheel | optional, optional, optional |
| **council** | - | - (core primitive) |
| **validate** | - | - (standalone validator role) |
| **vibe** | council, complexity, standards | required, optional (graceful skip), optional |
| **pre-mortem** | council | required |
| **post-mortem** | council, beads | required, optional |
| beads | - | - |
| domain | - | - |
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
| **plan** | research, beads, pre-mortem, crank, implement | optional, optional, optional, optional, optional |
| **push** | - | - (standalone) |
| **product** | - | - (standalone) |
| **pr-research** | - | - (standalone) |
| **pr-implement** | plan, pr-validate | optional, optional |
| **pr-validate** | - | - (standalone) |
| **pr-prep** | pr-validate | optional |
| **quickstart** | - | - (zero dependencies) |
| **bootstrap** | goals, product, doc, shared | all optional (progressive — skips what exists) |
| **discovery** | brainstorm, research, plan, pre-mortem, shared | brainstorm optional, rest required |
| **validation** | vibe, post-mortem, retro, forge, shared | vibe+post-mortem required, retro+forge optional |
| **rpi** | discovery, crank, validation, ratchet | all required |
| **evolve** | rpi | required (rpi pulls in all sub-skills) |
| **autodev** | evolve, rpi | required |
| **release** | - | - (standalone) |
| **security** | - | - (standalone) |
| ratchet | - | - |
| **recover** | - | - (standalone) |
| **reverse-engineer-rpi** | - | - (standalone) |
| research | knowledge, inject | optional, optional |
| retro | - | - |
| standards | - | - |
| **goals** | - | - (reads GOALS.yaml directly) |
| **status** | - | - (all CLIs optional) |
| **swarm** | implement, vibe | required, optional |
| trace | - | - (standalone) |
| **update** | - | - (standalone) |
| using-agentops | - | - |
| **test** | standards, complexity | required, optional |
| **review** | standards, council | required, optional |
| **design** | council, shared | required, optional |
| **refactor** | standards, complexity, beads | required, optional, optional |
| **deps** | standards | optional |
| **perf** | standards, complexity | optional, optional |
| **scaffold** | standards | required |
| **scenario** | - | - (standalone) |
| **system-tuning** | - | - (standalone) |

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
| validation | Direct orchestrator | Verdict visibility when called directly |
| implement | Single-task | Single issue, medium duration |
| bug-hunt | Investigator | Hypothesis loop, need to see reasoning |

### Tier 1.5: PHASE ISOLATION (declared phase contracts)

When `/rpi` calls lifecycle phases, the phase skill contract should run behind
isolated transport when the runtime supports it. `/rpi` stays visible; the
phase context receives only the objective plus bounded handoff artifact and
returns artifact path, verdict, and next action.

| Skill | Role | Why |
|-------|------|-----|
| discovery | Phase 1 contract | Research and planning context should not stay resident through implementation |
| crank | Phase 2 contract | Wave execution context should not stay resident through validation |
| validation | Phase 3 contract | Review and closeout context should not pollute the next lifecycle turn |

### Tier 2: FORK (discovery primitives)

Discovery skills that produce filesystem artifacts. User wants the output, not the process. Heavy codebase exploration and decomposition runs in a forked subagent; only the summary and artifact path return to the caller's context.

| Skill | Role | Why |
|-------|------|-----|
| research | Discovery | Massive codebase exploration → `.agents/research/*.md` |
| plan | Discovery | Decomposition + beads creation → `.agents/plans/*.md` + beads |
| retro | Knowledge extraction | Extract learnings → `.agents/learnings/*.md` |

### Tier 3: FORK (judgment + worker spawners)

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
- `skills/vibe/SKILL.md` — Complexity + council for code
- `skills/pre-mortem/SKILL.md` — Council for plans
- `skills/post-mortem/SKILL.md` — Council + retro for wrap-up
- `skills/swarm/SKILL.md` — Parallelize any skill
- `skills/rpi/SKILL.md` — Full pipeline orchestrator
