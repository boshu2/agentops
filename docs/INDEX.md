# Documentation Index

> Master table of contents for AgentOps documentation.

## Getting Started

- [README](../README.md) — Project overview and quick start
- [Behavioral Discipline](behavioral-discipline.md) — Before/after examples of good coding-agent behavior
- [Newcomer Guide](newcomer-guide.md) — Fast orientation to repo structure, architecture, and contribution path
- [FAQ](FAQ.md) — Comparisons, limitations, subagent nesting, uninstall
- [CONTRIBUTING](CONTRIBUTING.md) — How to contribute
- [Create Your First Skill](create-your-first-skill.md) — Fast path for authoring a first skill without tripping CI
- [AGENTS.md](../AGENTS.md) — Local agent instructions for this repo
- [Changelog](CHANGELOG.md) — Release history
- [Security](SECURITY.md) — Vulnerability reporting

## Architecture

- [How It Works](how-it-works.md) — Brownian Ratchet, Ralph Wiggum Pattern, agent backends, hooks, context windowing
- [Software Factory Surface](software-factory.md) — Explicit automation surface for briefings, RPI flows, and operator-controlled closeout
- [Architecture](ARCHITECTURE.md) — System design and component overview
- [Architecture Folder Index](architecture/README.md) — Architecture subdocs overview
- [Codex Hookless Lifecycle](architecture/codex-hookless-lifecycle.md) — Runtime-aware lifecycle fallback for Codex when hooks are unavailable
- [Primitive Chains](architecture/primitive-chains.md) — Audited primitive set, lifecycle chains, and terminology drift ledger
- [AO-Olympus Ownership Matrix](architecture/ao-olympus-ownership-matrix.md) — Responsibility split for skills, runtime, and bridge contracts
- [PDC Framework](architecture/pdc-framework.md) — Prevent, Detect, Correct quality control approach
- [FAAFO Alignment](architecture/faafo-alignment.md) — FAAFO promise framework for vibe coding value
- [Failure Patterns](architecture/failure-patterns.md) — The 12 failure patterns reference guide

## Skills

- [Skills Reference](SKILLS.md) — Complete reference for all AgentOps skills
- [Skills Decision Tree](skills-decision-tree.md) — "Which skill do I need next?" — single source of truth linked from harvest, compile, knowledge-activation, and quickstart SKILL.md
- [Skill API](SKILL-API.md) — Frontmatter fields, context declarations, enforcement status
- [Skill Tiers](../skills/SKILL-TIERS.md) — Taxonomy and dependency graph
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
- [Context Lifecycle Contract](context-lifecycle.md) — Internal proof contract behind the public bookkeeping, validation, primitives, and flows story
- [Knowledge Flywheel](knowledge-flywheel.md) — How every session makes the next one smarter
- [The Science](the-science.md) — Research behind knowledge decay and compounding
- [Brownian Ratchet](brownian-ratchet.md) — AI-native development philosophy
- [Evolve Setup](evolve-setup.md) — GOALS.md, fitness loop, overnight runs
- [Seed Definition](seed-definition.md) — What `ao seed` creates and why
- [Scale Without Swarms](scale-without-swarms.md) — Single-agent scaling patterns
- [Curation Pipeline](curation-pipeline.md) — Six-stage knowledge curation lifecycle
- [Context Packet](context-packet.md) — Agent context assembly specification
- [Strategic Direction](strategic-direction.md) — Product strategy and roadmap
- [Leverage Points](leverage-points.md) — Meadows-inspired system intervention points

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

- [Levels Overview](levels/README.md) — Progressive learning path

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

- [Profiles Overview](profiles/README.md) — Role-based profile organization
- [Profile Comparison](profiles/COMPARISON.md) — Workspace profiles vs 12-Factor examples
- [Meta-Patterns](profiles/META_PATTERNS.md) — Patterns extracted from role-based taxonomy
- [Example: Software Dev](profiles/examples/software-dev-session.md) — Software development session
- [Example: Platform Ops](profiles/examples/platform-ops-session.md) — Platform operations session
- [Example: Content Creation](profiles/examples/content-creation-session.md) — Content creation session

## Comparisons

- [Comparisons Overview](comparisons/README.md) — AgentOps vs the competition
- [vs SDD](comparisons/vs-sdd.md) — AgentOps vs Spec-Driven Development
- [vs GSD](comparisons/vs-gsd.md) — AgentOps vs Get Shit Done
- [vs Superpowers](comparisons/vs-superpowers.md) — AgentOps vs Superpowers plugin
- [vs Claude-Flow](comparisons/vs-claude-flow.md) — AgentOps vs Claude-Flow orchestration
- [vs Compound Engineer](comparisons/vs-compound-engineer.md) — AgentOps vs Compound Engineering plugin

## Positioning

- [Positioning Overview](positioning/README.md) — Product and messaging foundations
- [DevOps for Vibe-Coding](positioning/devops-for-vibe-coding.md) — Strategic foundation document
- [12 Factors Validation Lens](positioning/12-factors-validation-lens.md) — Shift-left validation for coding agents

## Plans

- [Plans Overview](plans/README.md) — Time-stamped plans index
- [Validated Release Pipeline](plans/2026-01-28-validated-release-pipeline.md) — Release pipeline design (2026-01-28)
- [AO-Olympus Bridge Next Steps](plans/2026-02-13-ao-olympus-bridge-next-steps.md) — Follow-up work to make the AO↔OL bridge enforceable (2026-02-13)
- [All Improvements](plans/2026-02-24-all-improvements.md) — Comprehensive improvement plan (2026-02-24)
- [AO Search as an Upstream CASS Wrapper](plans/2026-03-22-ao-search-cass-wrapper.md) — Make `ao search` broker to upstream `cass` plus AO-local fallback (2026-03-22)

## Templates

- [Templates Overview](templates/README.md) — Templates index
- [Workflow Template](templates/workflow.template.md) — Template for new workflows
- [Agent Template](templates/agent.template.md) — Template for new agents
- [Skill Template](templates/skill.template.md) — Template for new skills
- [Command Template](templates/command.template.md) — Template for new commands
- [Kernel Template](templates/kernel.template.md) — Template for new project kernels
- [Product Template](PRODUCT-TEMPLATE.md) — Template for writing a PRODUCT.md

## Reference

- [Agent Footguns](agent-footguns.md) — Common agent failure modes and mitigations
- [AgentOps Brief](agentops-brief.md) — Executive summary
- [AgentOps System Map](agentops-system-map.md) — Visual system map
- [Glossary](GLOSSARY.md) — Definitions of domain-specific terms (Beads, Brownian Ratchet, RPI, etc.)
- [CLI Reference](../cli/docs/COMMANDS.md) — Complete `ao` command reference
- [CLI ↔ Skills/Hooks Map](cli-skills-map.md) — Which commands are called by which skills and hooks
- [Reference](reference.md) — Deep documentation and pipeline details
- [Releasing](RELEASING.md) — Release process for ao CLI and plugin
- [Environment Variables](ENV-VARS.md) — All configuration variables with defaults and precedence
- [Skill Router](SKILL-ROUTER.md) — Which skill to use for which task
- [Troubleshooting](troubleshooting.md) — Common issues and quick fixes
- [Incident Runbook](INCIDENT-RUNBOOK.md) — Operational runbook for incidents and recovery
- [AO Command Customization Matrix](architecture/ao-command-customization-matrix.md) — External command dependencies and customization policy tiers
- [OL-AO Bridge Contracts](ol-bridge-contracts.md) — Olympus-AgentOps interchange formats
- [MemRL Policy Integration](contracts/memrl-policy-integration.md) — AO-exported deterministic MemRL policy contract for Olympus hooks
- [MemRL Policy Schema](contracts/memrl-policy.schema.json) — Machine-readable schema for MemRL policy package
- [MemRL Policy Example Profile](contracts/memrl-policy.profile.example.json) — Example deterministic policy profile
- [Repo Execution Profile](contracts/repo-execution-profile.md) — Repo-local bootstrap, validation, tracker, and done-criteria contract for autonomous orchestration
- [Autodev Program Contract](contracts/autodev-program.md) — Repo-local operational contract for bounded autonomous development
- [Repo Execution Profile Schema](contracts/repo-execution-profile.schema.json) — Machine-readable schema for repo execution profiles
- [RPI Run Registry](contracts/rpi-run-registry.md) — RPI run registry specification
- [Next-Work Queue Schema](contracts/next-work.schema.md) — Contract for `.agents/rpi/next-work.jsonl`
- [RPI Phase Result Schema](contracts/rpi-phase-result.schema.json) — Machine-readable schema for RPI phase results
- [RPI C2 Events Schema](contracts/rpi-c2-events.schema.json) — Machine-readable schema for per-run `.agents/rpi/runs/<run-id>/events.jsonl`
- [RPI C2 Commands Schema](contracts/rpi-c2-commands.schema.json) — Machine-readable schema for per-run `.agents/rpi/runs/<run-id>/commands.jsonl`
- [Swarm Worker Result Schema](contracts/swarm-worker-result.schema.json) — Machine-readable schema for `.agents/swarm/results/<task-id>.json` worker artifacts (strict completion contract)
- [Swarm Evidence Contract](contracts/swarm-evidence.md) — Permissive shape covering all historical swarm result files; enforced by `scripts/validate-swarm-evidence.sh`
- [Swarm Evidence Schema](../schemas/swarm-evidence.schema.json) — JSON Schema for the permissive swarm evidence shape
- [Hook Runtime Contract](contracts/hook-runtime-contract.md) — Canonical event mapping across Claude, Codex, and manual runtimes
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
- [Finding Item Schema](../schemas/finding.json) — Canonical finding-item schema for validation skill outputs (compatible subset of finding-artifact)
- [Finding Compiler Contract](contracts/finding-compiler.md) — V2 promotion ladder, executable constraint index contract, and lifecycle rules for turning findings into prevention artifacts

## Migration Trackers

- [resolve-project-dir.md](migration-trackers/resolve-project-dir.md) — os.Getwd() → resolveProjectDir() migration status
