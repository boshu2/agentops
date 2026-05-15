# Documentation Index

> Master table of contents for AgentOps documentation.

## Getting Started

> **Pick your path:** evaluating the product → [README](https://github.com/boshu2/agentops/blob/main/README.md) then [FAQ](FAQ.md). Installing for real work → [Getting Started landing](getting-started/index.md) then [Create Your First Skill](create-your-first-skill.md). Orienting in the codebase as a contributor → [Newcomer Guide](newcomer-guide.md) then [CONTRIBUTING](CONTRIBUTING.md). Upgrading from an older version → [Upgrading](UPGRADING.md).

- [README](https://github.com/boshu2/agentops/blob/main/README.md) — Project overview and quick start
- [Getting Started](getting-started/index.md) — Install + first command landing page
- [PRACTICE.md](https://github.com/boshu2/agentops/blob/main/PRACTICE.md) — Engineering doctrine and practice lineage for agent-context-limited artifacts
- [AgentOps 3.0 Explainer Kit](agentops-3-explainer-kit.md) — Public gist/launch copy for the council-first 3.0 story
- [AgentOps 3.0 First-Value Path](first-value-path.md) — First-session path from install to domain packet, council verdict, tracked work, and optional daemon lane
- [AgentOps 3.0 YouTube Starter Series](agentops-3-youtube-starter-series.md) — Launch video plan, scripts, clip hooks, CTAs, and PMF measurement fields
- [AgentOps 3.0 PMF Evidence Loop](agentops-3-pmf-evidence-loop.md) — Content-led discovery loop and claim-gated evidence plan
- [Behavioral Discipline](behavioral-discipline.md) — Before/after examples of good coding-agent behavior
- [Newcomer Guide](newcomer-guide.md) — Fast orientation to repo structure, architecture, and contribution path
- [FAQ](FAQ.md) — Comparisons, limitations, subagent nesting, uninstall
- [CONTRIBUTING](CONTRIBUTING.md) — How to contribute
- [Create Your First Skill](create-your-first-skill.md) — Fast path for authoring a first skill without tripping CI
- [Upgrading](UPGRADING.md) — Version-to-version migration notes and breaking changes
- [AGENTS.md](https://github.com/boshu2/agentops/blob/main/AGENTS.md) — Local agent instructions for this repo
- [Changelog](CHANGELOG.md) — Release history
- [Security](SECURITY.md) — Vulnerability reporting

## Four Product Layers

| Layer | What it does | Key surfaces |
|-------|-------------|-------------|
| **Bookkeeping** (L0) | Records agent work so attempts, decisions, verdicts, and handoffs leave evidence | `.agents/`, RPI packets, council verdicts, retros, post-mortems |
| **Context Compiler** (L1) | Assembles the right context for the right phase | `ao inject`, `ao compile`, skills, hooks |
| **Validation Gates** (L2) | Challenges plans and code before they ship | `/council`, `/vibe`, `/pre-mortem`, `/post-mortem` |
| **Knowledge Flywheel** (L3) | Extracts, scores, and resurfaces learnings | `/retro`, `/forge`, `ao lookup`, `.agents/` |

Deep dives: [CDLC](cdlc.md) (tier-to-layer mapping), [Knowledge Flywheel](knowledge-flywheel.md), [Context Lifecycle](context-lifecycle.md), [Assurance Profile](assurance-profile.md), [PRODUCT.md](https://github.com/boshu2/agentops/blob/main/PRODUCT.md)

Bridge / framing docs:

- [A wiki for your agents](wiki-for-agents.md) — `.agents/` as a markdown wiki agents read, traverse, and contribute to (deflationary framing for the busy buyer)
- [AgentOps as a Trust Factory](trust-factory.md) — Mapping AgentOps to the five-step trust-factory primitive (identity, reproducibility, evaluation, evidence, recovery)

## Architecture

- [How It Works](how-it-works.md) — Brownian Ratchet, Ralph Wiggum Pattern, agent backends, hooks, context windowing
- [Software Factory Surface](software-factory.md) — Explicit automation surface for briefings, RPI flows, and operator-controlled closeout
- [Assurance Profile](assurance-profile.md) — High-assurance operating posture, authority boundaries, and evidence artifact expectations for constrained environments
- [Architecture](ARCHITECTURE.md) — System design and component overview
- [Architecture Folder Index](architecture/index.md) — Architecture subdocs overview
- [Codex Hookless Lifecycle](architecture/codex-hookless-lifecycle.md) — Runtime-aware lifecycle fallback for Codex when hooks are unavailable
- [Primitive Chains](architecture/primitive-chains.md) — Audited primitive set, lifecycle chains, and terminology drift ledger
- [Ports and Adapters](architecture/ports-and-adapters.md) — Hexagonal seam: inner-hexagon domain, driving/driven adapters, ports, and how to add a new adapter
- [Operating Loop](architecture/operating-loop.md) — Operational discipline every process skill executes: BDD intent → vertical slices → conflict-free wave → bead acceptance → evidence (cleanroom companion to ports-and-adapters)
- [ADR-0001: Adopt DDD + Hexagonal Architecture](adr/ADR-0001-ddd-hexagonal-adoption.md) — Decision record for encoding DDD + Hexagonal with `ExecutionPacket` as the tracer-bullet aggregate
- [PDC Framework](architecture/pdc-framework.md) — Prevent, Detect, Correct quality control approach
- [FAAFO Alignment](architecture/faafo-alignment.md) — FAAFO promise framework for vibe coding value
- [Failure Patterns](architecture/failure-patterns.md) — The 12 failure patterns reference guide

## Skills

- [Skills Reference](SKILLS.md) — Complete reference for all AgentOps skills
- [Skills Decision Tree](skills-decision-tree.md) — "Which skill do I need next?" — single source of truth linked from harvest, compile, knowledge-activation, and quickstart SKILL.md
- [Skill API](SKILL-API.md) — Frontmatter fields, context declarations, enforcement status
- [JSM Skill Absorption Matrix](reference/jsm-skill-absorption.md) — Disposition table for the 2026-05-05 Bushido standalone JSM skill set
- [Skill Tiers](https://github.com/boshu2/agentops/blob/main/skills/SKILL-TIERS.md) — Taxonomy and dependency graph
- [skill-builder](https://github.com/boshu2/agentops/blob/main/skills/skill-builder/SKILL.md) — Scaffold or absorb new SKILL.md files against the unified template
- [skill-auditor](https://github.com/boshu2/agentops/blob/main/skills/skill-auditor/SKILL.md) — Two-pass audit of an existing SKILL.md against the unified template (15 checks)
- [Tier-S Audit Pilot 2026-05-06](https://github.com/boshu2/agentops/blob/main/.agents/audits/2026-05-06-tier-s-pilot.md) — Empirical baseline of 5 Tier-S skills against the auditor
- [Claude Code Skills Docs](https://code.claude.com/docs/en/skills) — Official Claude Code skills documentation (upstream)

## Workflows

- [Workflow Guide](workflows/README.md) — Decision matrix for choosing the right workflow
- [Complete Cycle](workflows/complete-cycle.md) — Full Research, Plan, Implement, Validate, Learn workflow
- [Session Lifecycle](workflows/session-lifecycle.md) — Runtime-aware session start and closeout across hook-capable and Codex hookless runtimes
- [Quick Fix](workflows/quick-fix.md) — Fast implementation for simple, low-risk changes
- [Debug Cycle](workflows/debug-cycle.md) — Systematic debugging from symptoms to root cause to fix
- [Knowledge Synthesis](workflows/knowledge-synthesis.md) — Extract and synthesize knowledge from multiple sources
- [Assumption Validation](workflows/assumption-validation.md) — Validate research assumptions before planning
- [Post-Work Retro](workflows/post-work-retro.md) — Systematic retrospective after completing work
- [Multi-Domain](workflows/multi-domain.md) — Coordinate work spanning multiple domains
- [Continuous Improvement](workflows/continuous-improvement.md) — Ongoing system optimization and pattern refinement
- [Infrastructure Deployment](workflows/infrastructure-deployment.md) — Orchestrate deployment with validation gates
- [Meta-Observer Pattern](workflows/meta-observer-pattern.md) — Autonomous multi-session coordination

### Meta-Observer

- [Meta-Observer README](workflows/meta-observer/README.md) — Complete workflow package overview
- [Pattern Guide](workflows/meta-observer/pattern-guide.md) — Autonomous multi-session coordination guide
- [Example Session](workflows/meta-observer/example-today.md) — Real example from 2025-11-09
- [Showcase](workflows/meta-observer/SHOWCASE.md) — Distributed intelligence for multi-session work

## Concepts

- [Philosophy](philosophy.md) — Five validated principles for building with coding agents, with evidence from five months of production use
- [Assurance Profile](assurance-profile.md) — High-assurance operating posture for local, auditable, constrained-environment agent work
- [Context Lifecycle Contract](context-lifecycle.md) — Internal proof contract behind the compounding product loop
- [Knowledge Flywheel](knowledge-flywheel.md) — How every session makes the next one smarter
- [The Science](the-science.md) — Research behind knowledge decay and compounding
- [Brownian Ratchet](brownian-ratchet.md) — AI-native development philosophy
- [Evolve Setup](evolve-setup.md) — GOALS.md, fitness loop, overnight runs
- [Seed Definition](seed-definition.md) — What `ao seed` creates and why
- [Scale Without Swarms](scale-without-swarms.md) — Single-agent scaling patterns
- [Curation Pipeline](curation-pipeline.md) — Six-stage knowledge curation lifecycle
- [Context Packet](context-packet.md) — Agent context assembly specification
- [Domain and Practice Packets](domain-practice-packets.md) — Product-facing contract for the shared engineering domain agents judge work against
- [Strategic Direction](strategic-direction.md) — Product strategy and roadmap
- [Leverage Points](leverage-points.md) — Meadows-inspired system intervention points

## Patterns

- [`.agents/` Hygiene Contract](patterns/agents-hygiene-contract.md) — Five-ring layering for taking native ownership of structural surfaces
- [Completion Notifications](patterns/completion-notifications.md) — Off-API webhook-equivalent patterns (GitHub Actions, post-commit hook, daemon log tail)

## Standards

- [Standards Overview](standards/README.md) — Coding standards index
- [Go Style Guide](standards/golang-style-guide.md) — Go coding conventions
- [TypeScript Standards](standards/typescript-standards.md) — TypeScript coding conventions
- [Python Style Guide](standards/python-style-guide.md) — Python coding conventions
- [Shell Script Standards](standards/shell-script-standards.md) — Shell script conventions
- [Markdown Style Guide](standards/markdown-style-guide.md) — Markdown formatting conventions
- [JSON/JSONL Standards](standards/json-jsonl-standards.md) — JSON and JSONL conventions
- [YAML/Helm Standards](standards/yaml-helm-standards.md) — YAML and Helm chart conventions
- [Tag Vocabulary](standards/tag-vocabulary.md) — Standard tag definitions

## Testing & CI

- [Testing Guide](TESTING.md) — Umbrella guide for all test types, tiers, and conventions
- [CI/CD Architecture](CI-CD.md) — Workflow map, job graph, blocking vs soft gates, local CI
- [Testing Skills](testing-skills.md) — Guide for writing and running skill integration tests
- [Release E2E Checklist](release-e2e-checklist.md) — Fast/full local gate commands and release smoke expectations

## Levels

- [Levels Overview](levels/index.md) — Progressive learning path

### L1 — Basics

- [L1 README](levels/L1-basics/README.md) — Single-session work with Claude Code
- [Research](levels/L1-basics/research.md) — Explore a codebase to understand how it works
- [Implement](levels/L1-basics/implement.md) — Make changes, validate, commit
- [Demo: Research Session](levels/L1-basics/demo/research-session.md) — Example research session
- [Demo: Implement Session](levels/L1-basics/demo/implement-session.md) — Example implement session

### L2 — Persistence

- [L2 README](levels/L2-persistence/README.md) — Cross-session bookkeeping with `.agents/`
- [Research](levels/L2-persistence/research.md) — Explore codebase and save findings
- [Retro](levels/L2-persistence/retro.md) — Extract session learnings
- [Demo: Research Session](levels/L2-persistence/demo/research-session.md) — Example persistent research
- [Demo: Retro Session](levels/L2-persistence/demo/retro-session.md) — Example retro session

### L3 — State Management

- [L3 README](levels/L3-state-management/README.md) — Issue tracking with beads
- [Plan](levels/L3-state-management/plan.md) — Decompose goals into tracked issues
- [Implement](levels/L3-state-management/implement.md) — Execute, validate, commit, close
- [Demo: Plan Session](levels/L3-state-management/demo/plan-session.md) — Example planning session
- [Demo: Implement Session](levels/L3-state-management/demo/implement-session.md) — Example implement session

### L4 — Parallelization

- [L4 README](levels/L4-parallelization/README.md) — Wave-based parallel execution
- [Implement Wave](levels/L4-parallelization/implement-wave.md) — Execute unblocked issues in parallel
- [Demo: Wave Session](levels/L4-parallelization/demo/wave-session.md) — Example wave execution

### L5 — Orchestration

- [L5 README](levels/L5-orchestration/README.md) — Full autonomous operation with /crank
- [Crank](levels/L5-orchestration/crank.md) — Execute epics to completion
- [Demo: Crank Session](levels/L5-orchestration/demo/crank-session.md) — Example crank session

## Profiles

- [Activation Profiles](activation-profiles.md) — 3.0 first-value workflow recipes with explicit inputs, commands, artifacts, and fallbacks
- [Profiles Overview](profiles/README.md) — Role-based profile organization
- [Profile Comparison](profiles/COMPARISON.md) — Workspace profiles vs 12-Factor examples
- [Meta-Patterns](profiles/META_PATTERNS.md) — Patterns extracted from role-based taxonomy
- [Example: Software Dev](profiles/examples/software-dev-session.md) — Software development session
- [Example: Platform Ops](profiles/examples/platform-ops-session.md) — Platform operations session
- [Example: Content Creation](profiles/examples/content-creation-session.md) — Content creation session

## Comparisons

- [Comparisons Overview](comparisons/README.md) — AgentOps vs the competition
- [Competition RPI: Memory, Learning, Wiki, Dream, and Pruning Pipelines](comparisons/competition-rpi-memory-pipelines.md) — Cross-product primitive and pipeline audit
- [vs SDD](comparisons/vs-sdd.md) — AgentOps vs Spec-Driven Development
- [vs GSD](comparisons/vs-gsd.md) — AgentOps vs Get Shit Done
- [vs Superpowers](comparisons/vs-superpowers.md) — AgentOps vs Superpowers plugin
- [vs Claude-Flow](comparisons/vs-claude-flow.md) — AgentOps vs Claude-Flow orchestration
- [vs Compound Engineer](comparisons/vs-compound-engineer.md) — AgentOps vs Compound Engineering plugin
- [vs Tons-of-Skills](comparisons/vs-tons-of-skills.md) — AgentOps vs `jeremylongshore/claude-code-plugins-plus-skills` (volume marketplace lane)
- [vs everything-claude-code](comparisons/vs-everything-claude-code.md) — AgentOps vs `affaan-m/everything-claude-code` (cross-harness lane)
- [Competitive Radar](comparisons/competitive-radar.md) — Current market read and improvement pressure

## Positioning

- [Positioning Overview](positioning/README.md) — Product and messaging foundations
- [DevOps for Vibe-Coding](positioning/devops-for-vibe-coding.md) — Strategic foundation document
- [12 Factors Validation Lens](positioning/12-factors-validation-lens.md) — Shift-left validation for coding agents

## Plans

- [Plans Overview](plans/README.md) — Time-stamped plans index
- [Validated Release Pipeline](plans/2026-01-28-validated-release-pipeline.md) — Release pipeline design (2026-01-28)
- [All Improvements](plans/2026-02-24-all-improvements.md) — Comprehensive improvement plan (2026-02-24)
- [AO Search as an Upstream CASS Wrapper](plans/2026-03-22-ao-search-cass-wrapper.md) — Make `ao search` broker to upstream `cass` plus AO-local fallback (2026-03-22)

## Templates

- [Templates Overview](templates/README.md) — Templates index
- [Intent Issue Template](templates/intent-issue.md) — BDD-shaped intent issue (Given/When/Then acceptance examples, bounded context, slice candidates) — produced by `/discovery`, consumed by `/plan`
- [Slice Validation Plan Template](templates/slice-validation.md) — Per-slice proof with first failing test, write-scope, wave-validity check, and roll-up acceptance — produced by `/plan`, executed by `/validation`
- [Workflow Template](templates/workflow.template.md) — Template for new workflows
- [Agent Template](templates/agent.template.md) — Template for new agents
- [Skill Template](templates/skill.template.md) — Template for new skills
- [Command Template](templates/command.template.md) — Template for new commands
- [Kernel Template](templates/kernel.template.md) — Template for new project kernels
- [AgentOps 3.0 Domain/Practice Packet](examples/agentops-3-domain-practice-packet.md) — Tracked launch-demo packet example
- [AgentOps 3.0 Council Demo Storyboard](examples/agentops-3-council-demo-storyboard.md) — Canonical council-first launch demo script
- [AgentOps 3.0 Council Verdict Example](examples/agentops-3-council-verdict-example.md) — Public sample verdict artifact for the explainer kit
- [Dark Factory Schedule Example](templates/dark-factory-schedule.yaml.example) — Disabled agentopsd schedule template for reviewed Dream and local factory pilots
- [Product Template](PRODUCT-TEMPLATE.md) — Template for writing a PRODUCT.md

## Reference

- [Agent Footguns](agent-footguns.md) — Common agent failure modes and mitigations
- [AgentOps Brief](agentops-brief.md) — Executive summary
- [AgentOps System Map](agentops-system-map.md) — Visual system map
- [Working with `.agents/`](agents-operator-guide.md) — Operator guide for `.agents/` state, write surfaces, and contributor flow
- [Glossary](GLOSSARY.md) — Definitions of domain-specific terms (Beads, Brownian Ratchet, RPI, etc.)
- [CLI Reference](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md) — Complete `ao` command reference (generated from source)
- [CLI Command Surface](cli-surface.md) — Generated classification of leaf commands by coverage and runtime safety
- [Hooks Reference](HOOKS.md) — Lifecycle events, what each hook does, how to customize
- [CLI ↔ Skills/Hooks Map](cli-skills-map.md) — Which commands are called by which skills and hooks
- [Reference](reference.md) — Pipeline stages, execution-model table, and skill-selection matrix (deep-dive companion to SKILLS.md)
- [Releasing](RELEASING.md) — Release process for ao CLI and plugin
- [Environment Variables](ENV-VARS.md) — All configuration variables with defaults and precedence
- [Schemas](SCHEMAS.md) — JSON Schemas for manifests, runtime artifacts, and internal runtime contracts
- [Skill Router](SKILL-ROUTER.md) — Which skill to use for which task
- [Troubleshooting](troubleshooting.md) — Common issues and quick fixes
- [Incident Runbook](INCIDENT-RUNBOOK.md) — Operational runbook for incidents and recovery
- [Autonomy Runtime Cycle-1 Runbook](runbooks/autonomy-runtime-cycle-1.md) — Safe activation/rollback/evidence checks for cycle-1 autonomy runtime work (ported from olympus)
- [bd Server-Mode Tracker Closeout](runbooks/bd-server-mode-closeout.md) — Distinguish Git push, local bd durability, and conditional Dolt remote push for server-mode trackers
- [Release Process Runbook](runbooks/release-process.md) — Step-by-step release runbook (gates, version bump, goreleaser, post-release; ported from olympus and complements `RELEASING.md`)
- [Factory Manual Merge Runbook](runbooks/factory-manual-merge.md) — Operator recovery and manual merge procedure for factory worktrees and validation evidence
- [Daemon Factory Admission Runbook](runbooks/daemon-factory-admission.md) — Rehearsal procedure for daemon-native factory admission, blocked decisions, RPI handoff, and schedule payloads
- [Cloud-Frontier Factory Pilot Runbook](runbooks/cloud-frontier-pilot.md) — Bounded one-versus-two worker pilot procedure using cloud/frontier coding lanes and manual merge
- [AO Command Customization Matrix](architecture/ao-command-customization-matrix.md) — External command dependencies and customization policy tiers
- [Contracts Index](contracts/index.md) — Landing page for all inter-component contracts
- [Context Map](contracts/context-map.md) — Auto-generated bounded-context map of skills by hexagonal role with relationship and data-flow views (see ADR-0001)
- [Repo Execution Profile](contracts/repo-execution-profile.md) — Repo-local bootstrap, validation, tracker, and done-criteria contract for autonomous orchestration
- [Repo Execution Profile Example](contracts/repo-execution-profile.json) — Concrete repository execution profile used by local autonomous orchestration
- [Autodev Program Contract](contracts/autodev-program.md) — Repo-local operational contract for bounded autonomous development
- [`.agents/` Write Surfaces](contracts/agents-write-surfaces.md) — Catalogued top-level subdirs that production code writes under `.agents/`, gated by `scripts/check-agents-write-surfaces.sh`
- [AgentOps Daemon Contract](contracts/agentops-daemon.md) — Always-on daemon ledger, job lifecycle, activation, readiness, projection, and threat model contract
- [AgentOpsd Control Plane Contract](contracts/agentopsd-control-plane.md) — Production control-plane contract for worker slots, worktree ownership, lifecycle telemetry, validation gates, yield, and operator status
- [Factory Admission Contract](contracts/factory-admission.md) — Daemon-owned work-order admission contract for fail-closed local factory pilots and RPI handoff
- [Factory Work Order Schema](https://github.com/boshu2/agentops/blob/main/schemas/factory-work-order.v1.schema.json) — JSON Schema for daemon-native factory work-order inputs
- [Factory Admission Decision Schema](https://github.com/boshu2/agentops/blob/main/schemas/factory-admission.v1.schema.json) — JSON Schema for daemon-native admission decisions
- [Routing Policy Contract](contracts/routing-policy.md) — Schema-backed model/provider/runtime lane policy, authority levels, and milestone-1 production-routing guardrails
- [Routing Policy Schema](https://github.com/boshu2/agentops/blob/main/schemas/routing-policy.v1.schema.json) — JSON Schema for `agentopsd` routing policy lane contracts
- [Factory Yield Ledger Contract](contracts/factory-yield-ledger.md) — Schema-backed baseline/treatment yield observations for routing, validation, merge, and artifact correlation
- [Factory Yield Ledger Schema](contracts/factory-yield-ledger.schema.json) — Contract-local JSON Schema used to validate the yield ledger fixture
- [Factory Yield Ledger Example](contracts/factory-yield-ledger.example.json) — Valid fixture for a `factory.yield_observation` event
- [Factory Claim Ledger Contract](contracts/factory-claim-ledger.md) — Public claim posture ledger mapping software-factory claims to evidence level, owner issue, closure gate, and safe wording
- [Factory Claim Ledger Schema](contracts/factory-claim-ledger.schema.json) — Contract-local JSON Schema for claim-ledger rows and posture enums
- [Factory Claim Ledger Example](contracts/factory-claim-ledger.example.json) — Current claim ledger fixture covering README, PRODUCT, GOALS, docs index, assurance, contracts, and comparison claims
- [Daemon Idempotency Contract](contracts/daemon-idempotency.md) — Submit retry contract defining `idempotency_key` as the dedup key and `request_id` as trace-only
- [AgentOps Daemon Scheduling Contract](contracts/agentopsd-schedule.md) — `.agents/schedule.yaml` schema, cron syntax, backpressure, mutation auth, ledger event vocabulary, executor idempotency, and migration recipe for native daemon scheduling
- [JobSpec OpenAPI v0](contracts/jobspec-openapi-v0.yaml) — Machine-readable current-behavior OpenAPI contract for `agentopsd` job submission, queue state, ledger replay, projections, and OpenClaw consumer routes
- [GasCity Integration Contract](contracts/gascity-integration.md) — Narrow handwritten GasCity adapter, fake/live split, compatibility matrix, and API/SSE expectations
- [Remote Compute Contract](contracts/remote-compute.md) — Product-neutral RemoteTarget, RemoteSession, command ledger, recovery, and GasCity-first remote execution contract
- [AgentWorker Runtime Contract](contracts/agent-worker.md) — Generic headless Codex/Claude worker and AgentSession lifecycle contract for daemon jobs
- [`ao outcomes` Contract](contracts/ao-outcomes.md) — Rubric → target → grader → retry loop spec for `ao outcomes run`; off-API analog of Anthropic Managed Agents Outcomes
- [`ao watch` Contract](contracts/ao-watch.md) — Live worker-event stream spec for `ao watch --follow`; off-API analog of Anthropic Managed Agents Console trace
- [Rubric Schema](https://github.com/boshu2/agentops/blob/main/schemas/rubric.v1.schema.json) — JSON Schema for `ao outcomes run` rubric files
- [Worker Spec Schema](https://github.com/boshu2/agentops/blob/main/schemas/worker-spec.v1.schema.json) — JSON Schema for per-worker model/tool/prompt isolation specs
- [OpenClaw Consumer API Contract](contracts/openclaw-consumer-api.md) — Read-only consumer snapshot API and authorized local trigger contract
- [Repo Execution Profile Schema](contracts/repo-execution-profile.schema.json) — Machine-readable schema for repo execution profiles
- [RPI Run Registry](contracts/rpi-run-registry.md) — RPI run registry specification
- [Eval Environment Contract](contracts/eval-environment.md) — Evaluation suite, run, scorecard, baseline, canary, and holdout contract
- [Eval Baseline-A/B Contract](contracts/eval-baseline-ab.md) — `ao eval run --baseline-mode` semantics, `DeltaScorecard` schema, hook-suppression scope
- [Context Usefulness Eval Contract](contracts/context-usefulness-eval.md) — Wave 0 deterministic `context_off` versus `context_on` evaluation, scorecard fields, hook-preservation boundaries
- [Eval Verdict Pipeline Contract](contracts/eval-verdict-pipeline.md) — Verdict compiler pipeline from eval run manifests to learning utility and retirement signals
- [Retrieval Comparison Contract](contracts/retrieval-comparison.md) — Deterministic search-eval backend comparison, promotion thresholds, optional rerank behavior, and deferred vector/graph-store policy
- [Release Readiness Contract](contracts/release-readiness.md) — 8/10 release readiness score, SIL/VIL/HIL evidence, artifact manifest requirements, and HIL waiver policy
- [MemRL Policy Schema](contracts/memrl-policy.schema.json) — Machine-readable retry/escalation policy profile for memory-reinforcement feedback loops
- [MemRL Policy Profile Example](contracts/memrl-policy.profile.example.json) — Example deterministic MemRL retry/escalation policy profile
- [Eval Workbench](https://github.com/boshu2/agentops/tree/main/evals/workbench) — Known-good fixture project (Go CLI, Python FastAPI, DevOps scripts) with 12 behavioral eval tasks and scoring scripts
- [Eval Suite Schema](https://github.com/boshu2/agentops/blob/main/schemas/eval-suite.v1.schema.json) — JSON Schema for public canary and private holdout evaluation suites
- [Eval Run Schema](https://github.com/boshu2/agentops/blob/main/schemas/eval-run.v1.schema.json) — JSON Schema for evaluation run records and scorecards
- [Remote Compute Target Schema](https://github.com/boshu2/agentops/blob/main/schemas/remote-compute-target.schema.json) — JSON Schema for product-neutral GasCity-backed remote compute targets
- [Remote Session Event Schema](https://github.com/boshu2/agentops/blob/main/schemas/remote-session-event.schema.json) — JSON Schema for remote session event and idempotent command ledger records
- [Next-Work Queue Schema](contracts/next-work.schema.md) — Contract for `.agents/rpi/next-work.jsonl`
- [RPI Phase Result Schema](contracts/rpi-phase-result.schema.json) — Machine-readable schema for RPI phase results
- [RPI C2 Events Schema](contracts/rpi-c2-events.schema.json) — Machine-readable schema for per-run `.agents/rpi/runs/<run-id>/events.jsonl`
- [RPI C2 Commands Schema](contracts/rpi-c2-commands.schema.json) — Machine-readable schema for per-run `.agents/rpi/runs/<run-id>/commands.jsonl`
- [Swarm Worker Result Schema](contracts/swarm-worker-result.schema.json) — Machine-readable schema for `.agents/swarm/results/<task-id>.json` worker artifacts (strict completion contract)
- [Swarm Evidence Contract](contracts/swarm-evidence.md) — Permissive shape covering all historical swarm result files; enforced by `scripts/validate-swarm-evidence.sh`
- [Swarm Evidence Schema](https://github.com/boshu2/agentops/blob/main/schemas/swarm-evidence.schema.json) — JSON Schema for the permissive swarm evidence shape
- [Hook Runtime Contract](contracts/hook-runtime-contract.md) — Canonical event mapping across Claude, Codex, and manual runtimes
- [Multi-Runtime Tier Charter](contracts/multi-runtime-tier-charter.md) — Explicit Tier S/I/E declaration: Tier S structural blocks CI; Tier E live execution is opt-in (Directive D1)
- [v2.39.0 README claim evidence manifest](releases/v2.39.0-claims/README.md) — Maps each `AOP-CLAIM-README-*` marker to its evidence file and verification gate (PG4)
- [AgentOps 3.0 PMF Scenario — exported evidence](releases/v3.0/pmf-scenario.md) — Single-day autonomous /evolve drain record: 11 P1 closures, 11 commits, friction modes, exported artifacts (PG2)
- [Scope Escape Report](contracts/scope-escape-report.md) — Structured template for agent scope-escape reporting
- [Dream Run Contract](contracts/dream-run-contract.md) — Process model, locking, keep-awake, and artifact floor for private overnight runs
- [Dream Report Contract](contracts/dream-report.md) — Canonical `summary.json` and `summary.md` schema for Dream outputs
- [dispatch-checklist.md](contracts/dispatch-checklist.md) — Standard references for agent dispatch prompts
- [Headless Invocation Standards](contracts/headless-invocation-standards.md) — Required flags, tool allowlists, and timeout strategy for non-interactive Claude/Codex execution
- [Codex Skill API Contract](contracts/codex-skill-api.md) — Source of truth for Codex runtime skill structure, frontmatter, discovery paths, and multi-agent primitives
- [Context Assembly Interface](contracts/context-assembly-interface.md) — Interface contract for adaptive context assembly and mechanical token budgeting
- [Session Intelligence Trust Model](contracts/session-intelligence-trust-model.md) — Artifact eligibility contract for runtime context assembly, explainability, and startup suppression rules
- [Finding Registry Contract](contracts/finding-registry.md) — Canonical intake-ledger contract for reusable findings in `.agents/findings/registry.jsonl`
- [Finding Registry Schema](contracts/finding-registry.schema.json) — Machine-readable schema for the finding intake ledger
- [Finding Artifact Schema](contracts/finding-artifact.schema.json) — Machine-readable schema for promoted finding artifacts under `.agents/findings/*.md`
- [Finding Item Schema](https://github.com/boshu2/agentops/blob/main/schemas/finding.json) — Canonical finding-item schema for validation skill outputs (compatible subset of finding-artifact)
- [Finding Compiler Contract](contracts/finding-compiler.md) — V2 promotion ladder, executable constraint index contract, and lifecycle rules for turning findings into prevention artifacts

## Migration Trackers

- [resolve-project-dir.md](migration-trackers/resolve-project-dir.md) — os.Getwd() → resolveProjectDir() migration status

## Reference: Olympus v4 Specs

> Verbatim port of `olympus/docs/specs/v4/` for cross-reference during the Olympus → agentopsd extraction. **NOT canonical for agentopsd.** Where these disagree with `.agents/design/2026-04-28-design-agentops-daemon-gascity-vertical-slices.md`, agentopsd canonical wins.

- [Architecture](design/olympus-v4-specs/architecture.md) — Olympus v4 architecture (reference)
- [CLI](design/olympus-v4-specs/cli.md) — `ol` CLI specification (reference)
- [Consensus](design/olympus-v4-specs/consensus.md) — Multi-model consensus before ratchet locks (reference)
- [Context](design/olympus-v4-specs/context.md) — Context as control plane (reference)
- [Daemon](design/olympus-v4-specs/daemon.md) — Daemon Phase 0 mechanical spec (reference)
- [Execution](design/olympus-v4-specs/execution.md) — Execution model (reference)
- [Knowledge](design/olympus-v4-specs/knowledge.md) — Knowledge compounding via constraint tests (reference)
- [Validation](design/olympus-v4-specs/validation.md) — Validation contract (reference)
