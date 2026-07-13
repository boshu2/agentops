# Documentation Index

> Master table of contents for AgentOps documentation.

## Getting Started

> **Pick your path:** evaluating the product → [README](https://github.com/boshu2/agentops/blob/main/README.md) then [FAQ](FAQ.md). Installing for real work → [Getting Started landing](getting-started/index.md) then [Create Your First Skill](create-your-first-skill.md). Orienting in the codebase as a contributor → [Newcomer Guide](newcomer-guide.md) then [CONTRIBUTING](CONTRIBUTING.md). Upgrading from an older version → [Upgrading](UPGRADING.md).

- [README](https://github.com/boshu2/agentops/blob/main/README.md) — Project overview and quick start
- [Getting Started](getting-started/index.md) — Install + first command landing page
- [PRACTICE-REGISTRY.md](https://github.com/boshu2/agentops/blob/main/PRACTICE-REGISTRY.md) — Practice lineage and canonical `practices: [slug]` registry
- [AgentOps 3.0 — the north star](3.0.md) — The single source of truth for what 3.0 is: the hookless-first CDLC loop, the four-practice waist (BDD/DDD/Hexagonal/TDD), and what "3.0-ready" means
- [Roadmap](ROADMAP.md) — Designed-but-not-built features (planned, not committed): CLI roadmap, curation pipeline later stages, hookless default-install
- [3.0-Readiness Level-Set](3.0-readiness.md) — Honest box-by-box status against the 3.0 acceptance criteria after the 2026-05-23 reconciliation: what's done, fitness snapshot, named remaining work
- [AgentOps 3.0 Explainer Kit](agentops-3-explainer-kit.md) — Public gist/launch copy for the council-first 3.0 story
- [AgentOps 3.0 First-Value Path](first-value-path.md) — First session via skills: `/plan` (Gherkin) → `/implement` (ATDD) → `/validate` (membrane vs that behavior)
- [Intent → Validated Code](architecture/intent-to-validated-code.md) — Full product flow from intent to membrane-proven acceptance
- [Skills Matrix](skills-matrix.md) — Every skill placed on operating-loop moves 1–7
- [AgentOps 3.0 YouTube Starter Series](agentops-3-youtube-starter-series.md) — Launch video plan, scripts, clip hooks, CTAs, and PMF measurement fields
- [AgentOps 3.0 PMF Evidence Loop](agentops-3-pmf-evidence-loop.md) — Content-led discovery loop and claim-gated evidence plan
- [Behavioral Discipline](behavioral-discipline.md) — Before/after examples of good coding-agent behavior
- [Driving Agents Reliably](driving-agents.md) — Operator's field guide: three laws, a copy-paste prompt pack, and the failure→mechanism table (companion to Behavioral Discipline)
- [Newcomer Guide](newcomer-guide.md) — Fast orientation to repo structure, architecture, and contribution path
- [Codebase Overview](architecture/codebase-overview.md) — Consolidated map of subsystems, active waist, registries, gates, footguns, and reading order (humans + agents)
- [Understanding the AgentOps Go CLI](architecture/go-cli-architecture-guide.md) — Contributor-oriented guide to the live Cobra bootstrap, command modules, services, ports/adapters, Go idioms, migration debt, tests, and code-reading exercises
- [FAQ](FAQ.md) — Comparisons, limitations, subagent nesting, uninstall
- [CONTRIBUTING](CONTRIBUTING.md) — How to contribute
- [Create Your First Skill](create-your-first-skill.md) — Fast path for authoring a first skill without tripping CI
- [Dependencies](dependencies.md) — Complete tool-dependency declaration (ao, git, br/bv, gh, go, and utilities) with purpose, required-vs-optional, and fallback-if-absent
- [Upgrading](UPGRADING.md) — Version-to-version migration notes and breaking changes
- [Migration Guide](MIGRATION.md) — Living map from every removed/retired surface (bd, hooks, daemon, `ao rpi`, corpus/flywheel, acfs) to what you use instead, plus the recommended open-source stack (br/bv, NTM, cass/cm, ubs, dcg, ACFS)
- [Migrating to AgentOps 3.0](MIGRATION-3.0.md) — What was removed in 3.0 (hooks, daemon, scheduler, factory) and what to use instead (in-session loop + an adopted substrate: NTM / MCP / managed-agents)
- [AGENTS.md](https://github.com/boshu2/agentops/blob/main/AGENTS.md) — Local agent instructions for this repo
- [Changelog](CHANGELOG.md) — Release history
- [Security](SECURITY.md) — Vulnerability reporting

## Four Product Layers

| Layer | What it does | Key surfaces |
|-------|-------------|-------------|
| **Bookkeeping** (L0) | Records agent work so attempts, decisions, verdicts, and handoffs leave evidence | `.agents/`, RPI packets, council verdicts, retros, postmortems |
| **Context Compiler** (L1) | Assembles the right context for the right phase | `ao inject`, `ao compile`, skills, execution packets |
| **Validation Gates** (L2) | Challenges plans and code before they ship | `/council`, `/vibe`, `/premortem`, `/postmortem` |
| **Knowledge Flywheel** (L3) | Extracts, scores, and resurfaces learnings | `/postmortem --quick`, `/curate --mode=forge`, `ao lookup`, `.agents/` |

Deep dives: [CDLC](cdlc.md) (AgentOps' context-native SDLC under token scarcity), [Knowledge Flywheel](knowledge-flywheel.md), [Context Lifecycle](context-lifecycle.md), [Assurance Profile](assurance-profile.md), [PRODUCT.md](https://github.com/boshu2/agentops/blob/main/PRODUCT.md)

Bridge / framing docs:

- [A wiki for your agents](wiki-for-agents.md) — `.agents/` as a markdown wiki agents read, traverse, and contribute to (deflationary framing for the busy buyer)
- [AgentOps as a Trust Factory](trust-factory.md) — Mapping AgentOps to the five-step trust-factory primitive (identity, reproducibility, evaluation, evidence, recovery)

## Architecture

- [Codebase Overview](architecture/codebase-overview.md) — Consolidated repo map: bounded contexts, directory ownership, active CLI waist, registries, gates, knowledge flywheel, footguns, reading order
- [Understanding the AgentOps Go CLI](architecture/go-cli-architecture-guide.md) — Source-pinned architecture and code-reading guide for the transitional Go CLI, including pure/effectful command traces, tracker/config policy, testing, known debt, and a worked capstone
- [Go CLI Production-Readiness Audit](audits/2026-07-12-go-cli-production-readiness.md) — Evidence-backed audit of the strangler program, proof gaps, tracker/context semantics, recursive contracts, CLI output conformance, integration risk, and goal-design inputs
- [Postmortem: Go CLI Goal Stall and Tracker-Layer Confusion](learnings/2026-07-12-go-cli-goal-stall-tracker-layer-confusion.md) — Why the run stopped before helper adjudication, why durable `br` decomposition mattered, and the exact boundary between this repo's beads_rust ledger and Gas City's `bd`/Dolt substrate store
- [How It Works](how-it-works.md) — Brownian Ratchet, Ralph Wiggum Pattern, agent backends, context windowing
- [Software Factory Surface](software-factory.md) — Explicit automation surface for briefings, RPI flows, and operator-controlled closeout
- [Assurance Profile](assurance-profile.md) — High-assurance operating posture, authority boundaries, and evidence artifact expectations for constrained environments
- [Architecture](ARCHITECTURE.md) — System design and component overview
- [Architecture Folder Index](architecture/index.md) — Architecture subdocs overview
- [Codex Hookless Lifecycle](architecture/codex-hookless-lifecycle.md) — Runtime-aware lifecycle fallback for Codex when hooks are unavailable
- [Codex Task Packet Contract](contracts/codex-task-packet.md) — Non-mutating Codex dispatch packet, auth guard, sandbox, stdin, timeout, resume, and run-receipt contract
- [Codex Fanout Approval Packet](contracts/codex-fanout-approval-packet.md) — PerspectivePlan, SynthesisPacket, and ApprovalEdge contract for Fable-gated Codex discovery before bead creation
- [Primitive Chains](architecture/primitive-chains.md) — Audited primitive set, lifecycle chains, and terminology drift ledger
- [Ports and Adapters](architecture/ports-and-adapters.md) — Hexagonal seam: inner-hexagon domain, driving/driven adapters, ports, and how to add a new adapter
- [Hexagon Port-Realness Audit](architecture/hexagon-port-realness-audit.md) — Empirical 2026-05-23 inventory of all 26 declared ports (real vs in-memory vs bypassed), direct-coupling hotspots (git/bd/loop/corpus) with file:line, and the recommended adapter build order for epic soc-zvhsl
- [Operating Loop](architecture/operating-loop.md) — Operational discipline every process skill executes: BDD intent → vertical slices → conflict-free wave → bead acceptance → evidence (cleanroom companion to ports-and-adapters)
- [The Canonical Loop Model](architecture/canonical-loop-model.md) — "One loop body, two drivers, one inner tick, one config": how rpi/evolve/factory/crank/swarm/autodev relate; the in-session loop is AgentOps-shipped, the out-of-session Factory driver is substrate-owned (reference: NTM / MCP / managed-agents)
- [Intent-to-Loop Hexagon](architecture/intent-to-loop-hexagon.md) — Process-level ports/adapters from BDD intent through beads, slices, validation, ratchet evidence, and loop steering
- [Fungibility Charter](architecture/fungibility-charter.md) — AgentOps 3.0's six doctrinal commitments (single-model RPI default, role-free claiming, stateless agents, universal init, automatic death recovery, opt-in specialization); fungible by default, specialized when you opt in
- [Behavior-Shaping Environment](architecture/behavior-shaping-environment.md) — The *why* beneath the loop: AgentOps as an operant-conditioning system (Antecedent → Behavior → Consequence); arrange the environment + reinforce/stop the behaviors you agree on
- [ADR-0001: Adopt DDD + Hexagonal Architecture](adr/ADR-0001-ddd-hexagonal-adoption.md) — Decision record for encoding DDD + Hexagonal with `ExecutionPacket` as the tracer-bullet aggregate
- [ADR-0002: AgentOps 3.0 Hookless-First CDLC Rearchitecture](adr/ADR-0002-agentops-3-hookless-cdlc-rearchitecture.md) — Proposed 3.0 direction: demote hooks to optional runtime adapters and center CDLC bounded contexts
- [ADR-0003: Executable-Spec Artifact Durability](adr/ADR-0003-executable-spec-artifact-durability.md) — Where executable-spec scenarios and domain manifests live: promoted spec scenarios in tracked `spec/scenarios/`, ad hoc holdout scenarios stay in `.agents/holdout/`
- [ADR-0013: Domain-Slice Manifest Contract](adr/ADR-0013-domain-slice-manifest-contract.md) — Schema and resolution rules for `docs/domains/<name>/manifest.yaml`, the domain-scoped boundary contract consumed by `ao rpi phased --domain` (formerly ADR-0004; renumbered to resolve a duplicate number)
- [ADR-0005: Trace-Link Convention](adr/ADR-0005-trace-link-convention.md) — The directive→scenario→bead→verdict→learning link grammar that `ao goals trace` renders and audits, including warning/error defect classes and `--strict` escalation
- [ADR-0006: Re-Steer Policy and Mutation Safety](adr/ADR-0006-re-steer-policy-and-mutation-safety.md) — The `docs/re-steer-policy.json` engine, human-gate, and non-lossy GOALS.md patcher that govern `ao goals steer recommend`/`apply`
- [ADR-0007: Deterministic /evolve Loop — Only the Operator Stops It](adr/ADR-0007-deterministic-loop-only-operator-stops.md) — Mechanical pre-cycle gate (`scripts/evolve/halt-check.sh`): operator-only markers, goal-regression halt, revert-on-red; ported from the mt-olympus unbounded-evolve substrate
- [ADR-0008: /evolve Operating Model — Intelligent-Agile, Not Waterfall](adr/ADR-0008-evolve-intelligent-agile-operating-model.md) — Three-layer loop contract (intent re-read each cycle / locked architecture / bounded shaping authority) + the scope-precondition audit that prevents building-the-wrong-thing drift
- [ADR-0009: Delete the Daemon — AgentOps Is In-Session Only](adr/ADR-0009-daemon-deletion-in-session-only.md) — Why the standalone daemon/scheduler/overnight-runner was deleted (not deprecated): AgentOps is a Gas City reference config with no core to protect, the in-session loop is the zero-dependency sovereignty floor, always-on opts into Gas City; names the rejected deprecate-keep-standalone alternative and the e2e-proof GC dispatch gap
- [ADR-0010: E6 Session-Log Miner Is Build-Native](adr/ADR-0010-e6-session-log-miner-build-native.md) — The E6.0 spike decision (cross-family): build the session-JSONL→typed-PROV-O miner native over `cli/internal/parser` + the ASSAY `--mine-cmd` slot; cass/`cm` stay adopted for search/memory only (neither owns the provenance graph). Steals cass's incremental-index discipline; no langchain core dep; scopes `ao provenance mine-session` (E6.1/`...6.2`)
- [ADR-0011: Escape-Corpus Compounding Unproven — Structural Data-Starvation](adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md) — The self-improvement *mechanism* is proven e2e (escape→check→block), but "the escape-corpus *compounds*" is demoted to an unproven hypothesis: a competent membrane catches at review, so it self-starves its own escape supply (self-improvement anti-correlated with membrane quality). Records revival conditions; the proven claim — independent verification, no verdict = not done — is unchanged
- [ADR-0012: Focus the Surface on Membrane + Bookkeeper; Archive the Satellites](adr/ADR-0012-focus-surface-on-membrane-bookkeeper-archive-satellites.md) — Re-headline the public surface on the two proven cores (validation membrane + hash-chained bookkeeper); archive the unproven corpus/flywheel (behind `//go:build flywheel`) and RPI/factory (behind `legacy`) satellites rather than delete them, because the ADR-0004/0009/0011 revival conditions require the code to stay buildable; targets a ~22-command / ~15-skill spine
- [PDC Framework](architecture/pdc-framework.md) — Prevent, Detect, Correct quality control approach
- [FAAFO Alignment](architecture/faafo-alignment.md) — FAAFO promise framework for vibe coding value
- [Failure Patterns](architecture/failure-patterns.md) — The 12 failure patterns reference guide

## Skills

- [Skills Reference](SKILLS.md) — Complete reference for all AgentOps skills
- [Skills Decision Tree](skills-decision-tree.md) — "Which skill do I need next?" — single source of truth linked from harvest, compile, knowledge-activation, and quickstart SKILL.md
- [Skill API](SKILL-API.md) — Frontmatter fields, context declarations, enforcement status
- [Critical Skills Policy](contracts/critical-skills.txt) — Human-supervised skill-edit denylist consumed by `ao skills edit seal`
- [Skill Quality Rubric](reference/skill-quality-rubric.md) — Scoring rubric for repo-runtime, export, and mega-skill readiness
- [AgentOps Domain Evolution BDD](reference/agentops-domain-evolution-bdd.md) — Gherkin acceptance contract for skill, CLI, and hook evolution
- [AgentOps Skill Domain Map](reference/agentops-skill-domain-map.md) — All 63 checked-in skills mapped to Corpus, Validation, Loop, Factory, and Runtime domains (drift-checked by `scripts/check-registry-drift.sh`)
- [AgentOps Hexagonal Architecture Map](reference/agentops-hexagonal-architecture-map.md) — Bounded contexts, ports, adapters, and proof gates for the evolution program
- [AgentOps Domain Evolution Plan](reference/agentops-domain-evolution-plan.md) — Sequenced bootstrap and evolution plan anchored to `soc-y5vh`
- [Skill Tiers](https://github.com/boshu2/agentops/blob/main/skills/SKILL-TIERS.md) — Taxonomy and dependency graph
- [skill-builder](https://github.com/boshu2/agentops/blob/main/skills/skill-builder/SKILL.md) — Scaffold or absorb new SKILL.md files against the unified template
- [heal-skill (deep audit mode)](https://github.com/boshu2/agentops/blob/main/skills/heal-skill/SKILL.md) — Two-pass audit of an existing SKILL.md against the unified template (absorbed from the retired skill-auditor)
- [Tier-S Audit Pilot 2026-05-06](https://github.com/boshu2/agentops/blob/main/.agents/audits/2026-05-06-tier-s-pilot.md) — Empirical baseline of 5 Tier-S skills against the auditor
- [Claude Code Skills Docs](https://code.claude.com/docs/en/skills) — Official Claude Code skills documentation (upstream)

## Workflows

- [Agent Workflow Reference](agent-workflow-reference.md) — On-demand deep detail behind the thin `CLAUDE.md` router: building the CLI, key scripts, CI-validation rules, testing rules, release pipeline, `ao goals` surface
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
- [Pattern Guide](workflows/meta-observer-pattern.md) — Autonomous multi-session coordination guide
- [Example Session](workflows/meta-observer/example-today.md) — Real example from 2025-11-09
- [Showcase](workflows/meta-observer/SHOWCASE.md) — Distributed intelligence for multi-session work

## Concepts

- [Philosophy](philosophy.md) — Five validated principles for building with coding agents, with evidence from five months of production use
- [Sovereignty Proof](sovereignty-proof/index.md) — Falsifiable case studies showing where independent-vendor review caught what same-vendor review missed (2026-05-15 RPI reframe, 2026-05-16 F6/F7 findings). CI gate `validate-sovereignty-proof-citations` keeps the cited file:line evidence honest.
- [Assurance Profile](assurance-profile.md) — High-assurance operating posture for local, auditable, constrained-environment agent work
- [Context Lifecycle Contract](context-lifecycle.md) — Internal proof contract behind the compounding product loop
- [Knowledge Flywheel](knowledge-flywheel.md) — How every session makes the next one smarter
- [The Science](the-science.md) — Research behind knowledge decay and compounding
- [Effective Feedback Compute](doctrine/effective-feedback-compute.md) — Imported doctrine: validation quality over raw token volume (η = EFC/C_raw); not locally proven
- [AgentOps Effectiveness Evidence](evals/agentops-effectiveness-evidence.md) — Honest audit: what we can and cannot claim about live-agent uplift
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
- [vs hosted AI code review](comparisons/vs-hosted-code-review.md) — AgentOps vs CodeRabbit, Qodo, and Copilot code review
- [vs Tons-of-Skills](comparisons/vs-tons-of-skills.md) — AgentOps vs `jeremylongshore/claude-code-plugins-plus-skills` (volume marketplace lane)
- [vs everything-claude-code](comparisons/vs-everything-claude-code.md) — AgentOps vs `affaan-m/everything-claude-code` (cross-harness lane)
- [Competitive Radar](comparisons/competitive-radar.md) — Current market read and improvement pressure

## Convergence

- [Convergence Overview](convergence/index.md) — The industry arriving at the structure AgentOps runs (vindication, not competition)
- [The Reading](convergence/the-reading.md) — Living thesis: the industry is converging, and here's the mechanism why
- [Convergence Ledger](convergence/ledger.md) — Dated receipts: external parties independently arriving at the AgentOps thesis
- [Google SRE](convergence/google-sre.md) — Encoding map: where Google's 2026 SRE AI whitepaper aligns with AgentOps doctrine, point by point

## Positioning

- [Positioning Overview](positioning/README.md) — Product and messaging foundations
- [DevOps for Vibe-Coding](positioning/devops-for-vibe-coding.md) — Strategic foundation document
- [12 Factors Validation Lens](positioning/12-factors-validation-lens.md) — Shift-left validation for coding agents

## Plans

- [Plans Overview](plans/README.md) — Time-stamped plans index
- [Validated Release Pipeline](plans/2026-01-28-validated-release-pipeline.md) — Release pipeline design (2026-01-28)
- [All Improvements](plans/2026-02-24-all-improvements.md) — Comprehensive improvement plan (2026-02-24)
- [AO Search as an Upstream CASS Wrapper](plans/2026-03-22-ao-search-cass-wrapper.md) — Make `ao search` broker to upstream `cass` plus AO-local fallback (2026-03-22)
- [AgentOps 3.0 Hookless CDLC Rearchitecture](plans/2026-05-15-agentops-3-hookless-cdlc-rearchitecture.md) — Hookless-first 3.0 plan with bounded contexts, hook lease disposition, and migration waves

## Templates

- [Templates Overview](templates/README.md) — Templates index
- [Intent Issue Template](templates/intent-issue.md) — BDD-shaped intent issue (Given/When/Then acceptance examples, bounded context, slice candidates) — produced by `/discovery`, consumed by `/plan`
- [Goal Design Intent Template](templates/goal-design-intent.md) — Schema-backed `.agents/goal-design/<slug>/intent.md` template for objective, BDD behavior, boundaries, evidence, stale inputs, and hard rules
- [Goal Design Driver Template](templates/goal-design-driver.md) — Schema-backed `.agents/goal-design/<slug>/driver.md` template for four-loop routing, candidate beads, route-back rules, digest integrity, and validation policy
- [Slice Validation Plan Template](templates/slice-validation.md) — Per-slice proof with first failing test, write-scope, wave-validity check, and roll-up acceptance — produced by `/plan`, executed by `/validate`
- [Workflow Template](templates/workflow.template.md) — Template for new workflows
- [Agent Template](templates/agent.template.md) — Template for new agents
- [Skill Template](templates/skill.template.md) — Template for new skills
- [Command Template](templates/command.template.md) — Template for new commands
- [Kernel Template](templates/kernel.template.md) — Template for new project kernels
- [AgentOps 3.0 Domain/Practice Packet](examples/agentops-3-domain-practice-packet.md) — Tracked launch-demo packet example
- [AgentOps 3.0 Council Demo Storyboard](examples/agentops-3-council-demo-storyboard.md) — Canonical council-first launch demo script
- [AgentOps 3.0 Council Verdict Example](examples/agentops-3-council-verdict-example.md) — Public sample verdict artifact for the explainer kit
- [Product Template](PRODUCT-TEMPLATE.md) — Template for writing a PRODUCT.md

## Reference

- [Agent Footguns](agent-footguns.md) — Common agent failure modes and mitigations
- [AgentOps Brief](agentops-brief.md) — Executive summary
- [AgentOps System Map](agentops-system-map.md) — Visual system map
- [Working with `.agents/`](agents-operator-guide.md) — Operator guide for `.agents/` state, write surfaces, and contributor flow
- [Glossary](GLOSSARY.md) — Definitions of domain-specific terms (Beads, Brownian Ratchet, RPI, etc.)
- [CLI Reference](https://github.com/boshu2/agentops/blob/main/cli/docs/COMMANDS.md) — Complete `ao` command reference (generated from source)
- [CLI Command Surface](cli-surface.md) — Generated classification of leaf commands by coverage and runtime safety
- [CLI ↔ Skills Map](cli-skills-map.md) — Which commands are called by which skills
- [Reference](reference.md) — Pipeline stages, execution-model table, and skill-selection matrix (deep-dive companion to SKILLS.md)
- [Releasing](RELEASING.md) — Release process for ao CLI and plugin
- [Environment Variables](ENV-VARS.md) — All configuration variables with defaults and precedence
- [Schemas](SCHEMAS.md) — JSON Schemas for manifests, runtime artifacts, and internal runtime contracts
- [Skill Router](SKILL-ROUTER.md) — Which skill to use for which task
- [Troubleshooting](troubleshooting.md) — Common issues and quick fixes
- [Incident Runbook](INCIDENT-RUNBOOK.md) — Operational runbook for incidents and recovery
- [Autonomy Runtime Cycle-1 Runbook](runbooks/autonomy-runtime-cycle-1.md) — Safe activation/rollback/evidence checks for RPI, evolve, and daemon-backed autonomy work
- [bd Server-Mode Tracker Closeout](runbooks/bd-server-mode-closeout.md) — Historical closeout note for retired bd/Dolt tracker deployments; not the current AgentOps tracker path
- [Release Process Runbook](runbooks/release-process.md) — Step-by-step release runbook for gates, version injection, goreleaser, and post-release checks; complements `RELEASING.md`
- [PR Creation From Linked Worktrees](runbooks/pr-creation-from-linked-worktrees.md) — Root-cause + verified fix for linked-worktree PR branch inference issues; retained as historical PR-flow guidance
- [AO Command Customization Matrix](architecture/ao-command-customization-matrix.md) — External command dependencies and customization policy tiers
- [Contracts Index](contracts/index.md) — Landing page for all inter-component contracts
- [Mortem Naming Migration](contracts/mortem-naming-migration.md) — Canonical `premortem`/`postmortem` identities, permanent legacy reads, and the schema-v3/S8 writer cutover boundary
- [Four-Umbrella Write Manifests](contracts/four-umbrella-write-manifests.json) — Per-slice write ownership and frozen S1 base for the validation-loop refactor
- [Four-Umbrella Examples](contracts/four-umbrella-examples.md) — Executable request, execution-packet, Learn-receipt, and plan-impact examples
- [Pawls — the one-way doors](contracts/pawls.md) — The ratchet's static map: the short list of irreversible actions (mutate-shared-trunk · delete · external-send/shared-state-mutation · schema/contract change · credential/authority change · spend) where the cross-family gate fires; everything else runs as ungated chaos
- [Operating Discipline (D1–D16)](doctrine/operating-discipline.md) — The general, substrate-neutral fleet-operating rules (admission-first · author≠judge · fail-closed · evidence-bound · single-writer · typed transitions) folded from the mt-olympus triangulated kernel; each rule marked embodied-in-gate (cited to pawls.md / pawl-verdict.sh / reconcile-pr.sh), advisory doctrine, or dropped-as-cathedral
- [Lesson Format](contracts/lesson-format.md) — Schema for `.agents/learnings/` entries with frontmatter (id/severity/trigger/verifiable/rule/falsified_by/practice/related) and graduation path (unassigned → proposed → accepted → encoded)
- [Corpus Learning Seam](contracts/corpus-learning-seam.md) — Field-level public/private boundary for learning records (epic ag-k7tq9 S3): the `sensitivity` + `publishable` promote-gate fields, what crosses the seam (the abstracted lesson) vs what never does (evidence/provenance/source_session), and the `ao corpus classify` migration; cites the cross-family council verdict
- [bd remember Migration Manifest](contracts/bd-remember-migration-manifest.md) — Lineage-preserving manifest contract for classifying `bd remember` notes into bead-scoped, pull-learning, or discard dispositions before migration
- [Bounded Contexts (yaml)](contracts/bounded-contexts.yaml) — Canonical BC1-BC6 definitions (id/name/responsibility/ports/center-of-gravity); registry doc prose must match this yaml (drift-checked by `scripts/check-bounded-contexts-drift.sh`, soc-zxia.2)
- [add-validate-job scaffolder](https://github.com/boshu2/agentops/blob/main/scripts/add-validate-job.sh) — CI integration scaffolder; emits all 5 touch-points (workflow + summary needs + summary echo + pre-push + bats stub + AGENTS table) atomically when adding a new `validate-*` job (soc-3oij)
- [CI Jobs Manifest (yaml)](contracts/ci-jobs.yaml) — Canonical reason+failure for every validate.yml CI job; AGENTS.md `### CI Jobs and What They Check` table is rendered from this yaml + workflow `summary.needs` via `scripts/generate-ci-jobs-table.sh` (golden-file gate enforced by `validate-ci-policy-parity`, soc-3oij)
- [Scenario → Test Linkage](contracts/scenario-test-linkage.md) — Every Gherkin scenario in `skills/*/references/*.feature` must declare its covering test via a `@covered-by:<test-path>` tag or be allowlisted as doc-only in `scripts/.scenario-linkage-allow`; gate is `scripts/check-scenario-test-linkage.sh` / `validate-scenario-test-linkage` (sibling to executable-spec-link-integrity; links scenarios→tests, soc-63xfx)
- [@claude Bot Delegation](contracts/claude-bot-delegation.md) — Operational runbook for the `@claude` GitHub App: permissions, triggers, status decoding, gotchas, when to delegate
- [Local Pre-Push Gate Retirement](contracts/local-pre-push-gate-retirement.md) — Historical ADR superseded by the local cockpit gate posture; retained for lineage, not current release authority
- [Skill Dispositions (yaml)](contracts/skill-dispositions.yaml) — Canonical per-skill domain/disposition/rationale data; source-of-truth for `agentops-skill-domain-map.md`. Hand-edits to the .md forbidden — edit yaml and run `scripts/generate-skill-domain-map.sh` (golden-file gate, soc-zxia.3)
- [Context Map](contracts/context-map.md) — Auto-generated bounded-context map of skills by hexagonal role with relationship and data-flow views (see ADR-0001)
- [Skill-Flow Connectivity](contracts/skill-flow.md) — Closed `consumes` vocabulary + cross-layer connectivity model (`consumes`/`context_rel`/`metadata.dependencies`); gate `scripts/validate-skill-flow.sh` (`validate-skill-flow`) fails on unresolved tokens or un-allowlisted orphans; standalone leaves in `scripts/skill-flow-standalone.txt`
- [PMF Evidence Gate](contracts/pmf-evidence.md) — Public docs (PRODUCT.md, README, launch artifacts) must promote `.agents/` evidence to `docs/evidence/<bead-id>/` via `scripts/export-evidence.sh`; `scripts/check-pmf-evidence.sh` is the gate (soc-m6v5.8)
- [Claim-Eval-Promote Policy](contracts/claim-eval-promote.md) — CEP policy overlay: 4-tier claim lifecycle (UNPROVEN/PILOT/NULL/PROVEN), curated registry at `docs/contracts/claim-registry.yaml`, additive regen from `agentops:claim:*` markers, drift gate + WARN-only PMF evidence gate in `ao gate check` (age-6sg)
- [Claim Registry (yaml)](contracts/claim-registry.yaml) — Curated claim policy overlay: per-claim tier/surfaces/owner/evidence/eval-binding; additive regen from `agentops:claim:*` markers via `scripts/regen-claim-registry.sh`; drift-gated by `claim.registry-drift` (age-6sg)
- [Skill Domain Map](contracts/skill-domain-map.md) — V0 DDD map assigning every shared skill to one explicit skill domain with ports, artifacts, and adapters
- [Registry as derived artifact](contracts/registry-as-derived.md) — Design contract (soc-jbea, status:design): move `registry.json` out of version control to eliminate sibling-PR conflict cascade (40-50% of waste in the 2026-05-20 PR-cleanup session per Council 220-240). Same pattern for `skills-codex/.agentops-manifest.json` and `skills-codex/*/.agentops-generated.json`. Implementation deferred to soc-jbea.1 through soc-jbea.7.
- [SKU Capability Catalog](contracts/sku-catalog.md) — `registry.json` schema_version 2: the generated SKU catalog (skills + CLI commands + gates + reference-impls) as a 4th derived JOIN projection. Defines the SKU entry schema, the `drives_commands` skill↔command join key (derived from skill bodies), `status` derivation, and the `validate-sku-catalog-drift` gate's three checks (drift + linkage-integrity + BC/loop-move coverage). Retires the bogus "163 cli_commands" count (ag-cbm).
- [Skill Ports and Adapters](contracts/skill-ports-and-adapters.md) — V0 skill-boundary vocabulary for inbound ports, outbound ports, adapters, context packets, and guard surfaces
- [Skill Lease Audit](contracts/skill-lease-audit.md) — V0 lease-on-life audit classifying all shared skills as keep, merge, split, retire, or unknown
- [Repo Execution Profile](contracts/repo-execution-profile.md) — Repo-local bootstrap, validation, tracker, and done-criteria contract for autonomous orchestration
- [Repo Execution Profile Example](contracts/repo-execution-profile.json) — Concrete repository execution profile used by local autonomous orchestration
- [Autodev Program Contract](contracts/autodev-program.md) — Repo-local operational contract for bounded autonomous development
- [AO / MTO Seam](contracts/ao-mto-seam.md) — Reduction contract separating the lean AO image from the outer MTO factory and routing RELOCATE surfaces through MTO, vendor-adapter, or defer-load-bearing seams
- [`.agents/` Write Surfaces](contracts/agents-write-surfaces.md) — Catalogued top-level subdirs that production code writes under `.agents/`, gated by `scripts/check-agents-write-surfaces.sh`
- [Goal Design Artifacts](contracts/goal-design-artifacts.md) — Two-artifact contract for `.agents/goal-design/<slug>/intent.md` and `driver.md`, including schemas, digest integrity, validation, lifecycle, and route-back rules
- [CI Path-Filter / Gate-Target Coverage Audit](contracts/ci-pathfilter-coverage-audit.md) — Repo-wide audit (ag-g9ex) of the invariant "a CI gate that reads a file must be triggered by a path-filter covering that file" (the #634/#638 class). Findings table for every file-reading gate in `validate.yml`, the two gaps fixed (AGENTS tiered-split siblings; wiring-closure GOALS.md de-wire), and the `--admin` self-merge governance policy. Guarded by `tests/scripts/test-pathfilter-gate-coverage.bats`.
- [Update Principles Contract](contracts/update-principles.md) — Five operator-exemplar properties every commit must demonstrate (single concern, drift-blocking test, sibling citation, fitness delta, clean branch point); sourced from commit 1b9d139c
- [Ubiquitous Language Contract](contracts/ubiquitous-language.md) — Canonical names per bounded context (BC1 Corpus, BC2 Validation, BC3 Loop, BC4 Factory, BC5 Runtime) for the 5 ranked drifts (Gate/Check, Cycle/Loop, Claim/Evidence, Skill/Pattern/Practice, Session); rename schedule bound to soc-5yuy children
- [BC1 Corpus Ports Contract](contracts/bc1-corpus-ports.md) — Core BC1 corpus ports scaffolded under `cli/internal/ports/`; semantics cheat-sheet, adapter triplet pattern, soc-pm5t wire-up order
- [BC Ports Inventory](contracts/bc-ports-inventory.md) — Roster of all 20 BC ports with per-port adapter contracts, the universal triplet construction pattern, and per-BC wire-up order.
- [Orchestration Ports](contracts/orchestration-ports.md) — `OrchestrationPort` dual-runtime selection seam: the 3-category model (Claude Workflow / NTM swarm / plain skill), the NTM → Claude-native → beads-floor degradation ladder, `AGENTOPS_ORCHESTRATION=off` opt-out, capability detection via `ntm --robot-capabilities`, output-contract parity (`orchestration-result.v1`), and the two-ladders distinction. Paired schemas `schemas/orchestration-backend.v1.schema.json` + `schemas/orchestration-result.v1.schema.json`.
- [Orchestration Profiles (yaml)](contracts/orchestration-profiles.yaml) — SOT for portable NTM worker and pawl-review role profiles plus structured spawn arguments
- [Orchestration Tools (yaml)](contracts/orchestration-tools.yaml) — Tool matrix contract for `ao orchestrate tools`; drift-gated via `scripts/check-orchestration-contracts.sh`
- [Orchestration Instrument Schema](https://github.com/boshu2/agentops/blob/main/schemas/orchestration-instrument.v1.schema.json) — JSON schema for `ao orchestrate` preflight/verify/tools/route/status JSON output (`orchestration-instrument.v1`)
- [Orchestration Backend Selection Contract](contracts/orchestration-backend.md) — wire shape of one `OrchestrationPort` selection decision (chosen/reason/considered/opt_out/pin); pairs `schemas/orchestration-backend.v1.schema.json` for structural-floor validation.
- [Orchestration Result Parity Contract](contracts/orchestration-result.md) — the output-contract parity shape every tier (NTM/Claude/beads) must emit; pairs `schemas/orchestration-result.v1.schema.json`; enforced by the degradation-conformance test.
- [Remote Compute Contract](contracts/remote-compute.md) — Product-neutral RemoteTarget, RemoteSession, command ledger, recovery, and GasCity-first remote execution contract
- [Rubric Schema](https://github.com/boshu2/agentops/blob/main/schemas/rubric.v1.schema.json) — JSON Schema for rubric files (outcome rubric → target → grader → retry loop)
- [Worker Spec Schema](https://github.com/boshu2/agentops/blob/main/schemas/worker-spec.v1.schema.json) — JSON Schema for per-worker model/tool/prompt isolation specs
- [Repo Execution Profile Schema](contracts/repo-execution-profile.schema.json) — Machine-readable schema for repo execution profiles
- [RPI Run Registry](contracts/rpi-run-registry.md) — RPI run registry specification
- [Eval Environment Contract](contracts/eval-environment.md) — Evaluation suite, run, scorecard, baseline, canary, and holdout contract
- [Eval Baseline-A/B Contract](contracts/eval-baseline-ab.md) — `ao eval run --baseline-mode` semantics, `DeltaScorecard` schema, hook-suppression scope
- [Context Usefulness Eval Contract](contracts/context-usefulness-eval.md) — Wave 0 deterministic `context_off` versus `context_on` evaluation, scorecard fields, hook-preservation boundaries
- [Eval Verdict Pipeline Contract](contracts/eval-verdict-pipeline.md) — Verdict compiler pipeline from eval run manifests to learning utility and retirement signals
- [Outcomes Rubric Projection Contract](contracts/outcomes-rubric-projection.md) — Holdout-safe projection of the locked eval substrate into an Outcomes-style grading payload (`schemas/outcomes-rubric.v1.schema.json`); `additionalProperties:false` at every level forbids target/ground_truth/expected_output (Managed Agents are not ZDR); validator `scripts/validate-outcomes-rubric.sh` + Go schema↔struct drift guard (ag-hguuf)
- [Agent-Native Mechanism Contract](contracts/agent-native-mechanism.md) — Decision doc mapping each old-hook *intent* (orientation, standards, scope guard, commit-review, holdout-isolation) to its hookless equivalent (skill + `ao` subcommand + gate job) across both runtimes; retained with historical substrate notes
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
- [Multi-Runtime Tier Charter](contracts/multi-runtime-tier-charter.md) — Explicit Tier S/I/E declaration: Tier S structural blocks CI; Tier E live execution is opt-in (Directive D1)
- [v2.39.0 README claim evidence manifest](releases/v2.39.0-claims/README.md) — Maps each `AOP-CLAIM-README-*` marker to its evidence file and verification gate (PG4)
- [AgentOps 3.0 PMF Scenario — evidence bundle](releases/v3.0/pmf-scenario.md) — Single-day autonomous /evolve drain record: 11 P1 closures, 11 commits, friction modes, durable artifacts (PG2)
- [Scope Escape Report](contracts/scope-escape-report.md) — Structured template for agent scope-escape reporting
- [Dream Report Contract](contracts/dream-report.md) — Canonical `summary.json` and `summary.md` schema for Dream outputs
- [Dreaming/Memory Writers Characterization](contracts/dreaming-writers-characterization.md) — Spike (ag-cj8mk): the three foreign Dreaming/memory writers (Anthropic Managed-Agents Dreaming `/v1/dreams`→memory store, OpenClaw memory-wiki indexer, local gc-dream/T3 synthesis), their output shapes/destinations, the normalized `.agents/learnings/*.md` target via `ao corpus capture`, the NOT-ZDR holdout/PII constraint enforced by the ag-onf37 leak guard, and GO on the Claude-side REST pull feeder
- [dispatch-checklist.md](contracts/dispatch-checklist.md) — Standard references for agent dispatch prompts
- [Headless Invocation Standards](contracts/headless-invocation-standards.md) — Required flags, tool allowlists, and timeout strategy for non-interactive Claude/Codex execution
- [Codex Skill API Contract](contracts/codex-skill-api.md) — Source of truth for Codex runtime skill structure, frontmatter, discovery paths, and multi-agent primitives
- [Context Assembly Interface](contracts/context-assembly-interface.md) — Interface contract for adaptive context assembly and mechanical token budgeting
- [Session Intelligence Trust Model](contracts/session-intelligence-trust-model.md) — Artifact eligibility contract for runtime context assembly, explainability, and startup suppression rules
- [Finding Registry Contract](contracts/finding-registry.md) — Canonical intake-ledger contract for reusable findings in `.agents/findings/registry.jsonl`
- [Producer-Defect Recurrence Contract](contracts/producer-defect-register.md) — Distinct-objective recurrence reduction from evidence-backed findings to advisory producer-rule candidates
- [Finding Registry Schema](contracts/finding-registry.schema.json) — Machine-readable schema for the finding intake ledger
- [Finding Artifact Schema](contracts/finding-artifact.schema.json) — Machine-readable schema for promoted finding artifacts under `.agents/findings/*.md`
- [Finding Item Schema](https://github.com/boshu2/agentops/blob/main/schemas/finding.json) — Canonical finding-item schema for validation skill outputs (compatible subset of finding-artifact)
- [Finding Compiler Contract](contracts/finding-compiler.md) — V2 promotion ladder, executable constraint index contract, and lifecycle rules for turning findings into prevention artifacts

## Migration Trackers

- [resolve-project-dir.md](migration-trackers/resolve-project-dir.md) — os.Getwd() → resolveProjectDir() migration status
